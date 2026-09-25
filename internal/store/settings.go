package store

import (
	"path/filepath"

	"github.com/nxck2005/surmise/internal/game"
)

// Settings is what the player has chosen, as opposed to what they have played.
// Native builds keep it at the root of the data dir; KVStore uses the same
// codec under its browser settings key. It is small, rewritten rarely and safe
// to lose: every field has a working zero value.
type Settings struct {
	// Schema is the save-format version this blob was written with, the same
	// tag every puzzle record carries. Zero means "written before the tag
	// existed" and stays valid; anything above what this build knows degrades
	// to the defaults rather than erroring — see codec.go.
	Schema int `json:"schema"`

	Theme string `json:"theme,omitempty"`
	// DisplayName is local presentation on the profile screen. It is not an
	// account ID, puzzle owner, authentication claim, or uniqueness promise;
	// future networking can add identity without inheriting this field's rules.
	DisplayName string `json:"display_name,omitempty"`
	// Length is the word length the app opens on. Zero means "no choice made",
	// which the UI reads as its own default rather than as an invalid mode.
	Length int `json:"length,omitempty"`
	// RememberLast makes playing a mode set Length, so the app reopens on
	// whatever was last played. Off by default: the zero value is the one that
	// leaves Length alone.
	RememberLast bool `json:"remember_last,omitempty"`

	// The splash screen's three preferences. All strings, because the zero value
	// has to mean "nothing chosen" and the default for the first of them is on:
	// a bool could not tell "never chosen" from "chosen off".
	//
	//	Splash        "" (on), "on", "off"
	//	SplashArt     "" (the default art), "random", or a banner's name
	//	SplashDismiss "" (key), "skip", "key", "fixed"
	//
	// The UI resolves each of them, and an unknown value is reported on its
	// error line rather than refused — art that stopped shipping must not cost
	// anyone a launch.
	Splash        string `json:"splash,omitempty"`
	SplashArt     string `json:"splash_art,omitempty"`
	SplashDismiss string `json:"splash_dismiss,omitempty"`

	// Motion is how much the board animates: tile reveals, the invalid-word
	// cue, keycap pulses and the win accent. A string for the same reason the
	// splash preferences are strings — the default is not the zero value, so a
	// bool could not tell "never chosen" from "chosen off".
	//
	//	Motion  "" (pronounced), "off", "restrained", "pronounced"
	//
	// An unset value is also what lets the environment answer instead:
	// $NO_MOTION natively, prefers-reduced-motion in a browser. Choosing here
	// is deliberate and overrides both.
	Motion string `json:"motion,omitempty"`

	// SplashMillis is how long a timed splash stays up. Zero is "nothing
	// chosen", which the UI reads as its own default — the same rule Length
	// follows, and the reason this is not a time.Duration: a duration's zero is
	// a legitimate value (no wait at all) and could not be told apart from an
	// older settings file that never had the field.
	SplashMillis int `json:"splash_ms,omitempty"`

	// PlaytimeMS is the lifetime play counter, in milliseconds — the one field
	// here that is not a preference. It lives with the preferences because both
	// stores already carry this struct through one codec, so the browser build
	// needs no extra method to keep it.
	//
	// It is a counter and not a figure derived from the saved puzzles, which is
	// what makes time played permanent: a deleted puzzle leaves a tombstone
	// with no ElapsedMS, so a total summed from the records would shrink when a
	// puzzle is deleted. This only ever grows. Zero means nothing played yet,
	// which is also what an older settings file says, so stats.Playtime floors
	// it with what the records can still prove.
	PlaytimeMS int64 `json:"playtime_ms,omitempty"`
}

// MaxPlaytimeMS is the largest play counter the settings may hold. The counter
// is milliseconds, and a time.Duration holds about 292 years of them: past that
// the conversion on the profile — and behind -playtime — overflows and the
// figure turns into nonsense. No install reaches the bound, so it is a validity
// rule for what may be imported or hand-edited, not a limit on play.
//
// It is game.MaxElapsedMS, not a second derivation of the same figure: a saved
// puzzle's elapsed time is held to exactly this bound by game.Validate, and
// the two must not be able to drift.
const MaxPlaytimeMS = game.MaxElapsedMS

// MaxSettingFieldBytes bounds one free-text preference: the theme name, the
// profile name, and the splash and motion choices. Every one of them is a short
// display string, and nothing this app writes comes near the figure — it is a
// theme name's worth of bytes, which is the longest free text the theme reader
// accepts either.
//
// It exists because the total-size cap is not a field cap. ValidateSettings
// refuses a blob over MaxRecordBytes, so a settings section may still devote
// 64 KiB to a single string, and every one of these fields is drawn inside a
// cell the terminal cannot widen. A name of a few thousand combining marks takes
// no space at all on screen and still widens the panel to hold it.
//
// internal/ui's textField holds the *same* bound, and must not hold a larger
// one: a field that accepted what the store would then refuse would be a field
// whose value could not be saved. That is why this is the one constant and the
// UI refers to it, in the way MaxPlaytimeMS is game.MaxElapsedMS rather than a
// second derivation of the same figure.
const MaxSettingFieldBytes = 128

// clampPlaytime is how every settings read and write saturates the counter: a
// value past the bound is corrupt, and one that wrapped negative would take the
// whole lifetime total with it. A negative value reads as "nothing played yet",
// which is what the zero value already means.
func clampPlaytime(ms int64) int64 {
	switch {
	case ms < 0:
		return 0
	case ms > MaxPlaytimeMS:
		return MaxPlaytimeMS
	default:
		return ms
	}
}

const settingsName = "settings.json"

func (s *JSON) settingsPath() string { return filepath.Join(s.dir, settingsName) }

// Settings reads the saved preferences. A missing or damaged file yields the
// defaults rather than an error: a bad settings file must never cost a puzzle.
// It is read through the same record limit as a puzzle, for the same reason —
// a hand-replaced settings file must not be read into memory whole.
func (s *JSON) Settings() Settings {
	b, err := readLimited(s.settingsPath())
	if err != nil {
		return Settings{}
	}
	return decodeSettings(b)
}

// SaveSettings persists preferences, atomically like every other write here.
func (s *JSON) SaveSettings(v Settings) error {
	b, err := encodeSettings(v)
	if err != nil {
		return err
	}
	return writeFileAtomic(s.settingsPath(), b)
}
