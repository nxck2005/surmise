package backup

import (
	"fmt"
	"time"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/theme"
)

// Result is what a restore did, in the terms a player would ask about it.
//
// Settings and Themes are handed back rather than written: preferences are not
// on store.Store (only the concrete stores carry them), and a theme is a file on
// a disk the browser build does not have. Apply decides, the platform writes —
// the same split that keeps Build and Read testable without either.
type Result struct {
	// PuzzlesAdded is what the store did not already hold; PuzzlesKept is what
	// it did, and which the archive was therefore not allowed to touch.
	PuzzlesAdded int
	PuzzlesKept  int

	// Settings is the merged preferences, for the caller to save. It is the
	// caller's own value with the archive's answers filled into the questions
	// they had not answered, so saving it unconditionally is safe.
	Settings store.Settings
	// SettingsFilled names the preferences the archive supplied, in the order
	// this file lists them, so a caller can say what changed rather than
	// claiming that everything did.
	SettingsFilled []string
	// PlaytimeAdded is how much the lifetime counter went up by. A counter is
	// raised to the larger of the two figures and never lowered — see
	// store.Settings.PlaytimeMS for why that field is a counter at all.
	PlaytimeAdded time.Duration

	// Themes is what the archive carried, for the platform to write with
	// theme.WriteNew. Empty on a file that had none, and on a platform with no
	// theme directory the caller simply drops it.
	Themes []theme.File
}

// Any reports whether the restore changed anything Apply itself did. A player
// who imports the same file twice should be told the second time did nothing
// rather than be shown a report that reads like it worked.
//
// Themes are deliberately not counted: Apply hands them back rather than
// writing them, so whether any of them was new is the platform's answer to
// give, not this one's. A caller that writes them adds its own count to this.
func (r Result) Any() bool {
	return r.PuzzlesAdded > 0 || len(r.SettingsFilled) > 0 || r.PlaytimeAdded > 0
}

// Apply reads an archive and merges it into an install.
//
// It only ever adds. A puzzle the store already holds is left exactly as it is,
// a preference already chosen is left alone, and the play counter is raised to
// whichever figure is larger. So importing the wrong file costs nothing, and
// importing the same file twice changes nothing the second time.
//
// Keeping the local copy — rather than the newer of the two, or the more
// complete — is a deliberate choice and not laziness. "Newer" would need every
// record's clock to be trustworthy across two machines, and the failure it
// invites is silent: a restore that overwrote today's finished daily with a
// stale one would destroy the very history the file exists to protect. Refusing
// to overwrite has no such failure, and a player who really wants the archive's
// copy can delete theirs and import again.
//
// current is the caller's saved preferences, which Apply needs in order to tell
// a chosen value from an unset one.
func Apply(b []byte, s store.Store, current store.Settings) (Result, error) {
	a, games, err := Read(b)
	if err != nil {
		return Result{}, err
	}

	// One read of the whole store, not a Load per record: Load hides tombstones
	// (it reports one as ErrNotFound, because nothing may resume a deletion), so
	// asking it would read every deleted puzzle in the archive as absent and
	// write a tombstone back over a puzzle the player still has.
	existing, err := s.All()
	if err != nil {
		return Result{}, fmt.Errorf("backup: read the current history: %w", err)
	}
	have := make(map[string]bool, len(existing))
	for _, g := range existing {
		have[g.ID] = true
	}

	var out Result
	for _, g := range games {
		if have[g.ID] {
			out.PuzzlesKept++
			continue
		}
		// Save routes a tombstone to the marker encoding, so a deleted puzzle
		// arrives as the marker it was and not as a corrupt-looking Game. See
		// store.encodeRecord.
		if err := s.Save(g); err != nil {
			return out, fmt.Errorf("backup: restore %s: %w", game.Code(g.ID), err)
		}
		have[g.ID] = true
		out.PuzzlesAdded++
	}

	out.Settings, out.SettingsFilled, out.PlaytimeAdded = mergeSettings(current, a.Settings)
	out.Themes = a.Themes
	return out, nil
}

// mergeSettings fills in the preferences nobody has chosen and raises the play
// counter, leaving every answered question alone.
//
// Every field of store.Settings has a zero value meaning "nothing chosen" — the
// splash and motion preferences are strings for exactly that reason — which is
// what makes "fill only the unset" expressible at all.
func mergeSettings(current store.Settings, from *store.Settings) (store.Settings, []string, time.Duration) {
	if from == nil {
		return current, nil, 0
	}

	out := current
	var filled []string
	str := func(name string, dst *string, src string) {
		if *dst == "" && src != "" {
			*dst = src
			filled = append(filled, name)
		}
	}
	num := func(name string, dst *int, src int) {
		if *dst == 0 && src != 0 {
			*dst = src
			filled = append(filled, name)
		}
	}

	str("theme", &out.Theme, from.Theme)
	str("display name", &out.DisplayName, from.DisplayName)
	num("length", &out.Length, from.Length)
	str("splash", &out.Splash, from.Splash)
	str("splash art", &out.SplashArt, from.SplashArt)
	str("splash dismiss", &out.SplashDismiss, from.SplashDismiss)
	str("motion", &out.Motion, from.Motion)
	num("splash duration", &out.SplashMillis, from.SplashMillis)

	// RememberLast is the one preference whose zero value is also a real answer
	// ("off"), so there is no telling an unset one from a chosen one. Turning it
	// on is the only move that adds, so that is the only move made.
	if !out.RememberLast && from.RememberLast {
		out.RememberLast = true
		filled = append(filled, "remember last mode")
	}

	// The counter: raised, never lowered, whichever side is behind. Two installs
	// of the same history that were played apart both keep their own time, and
	// the larger figure is the honest floor for the whole of it.
	var gained time.Duration
	if from.PlaytimeMS > out.PlaytimeMS {
		gained = time.Duration(from.PlaytimeMS-out.PlaytimeMS) * time.Millisecond
		out.PlaytimeMS = from.PlaytimeMS
	}
	return out, filled, gained
}
