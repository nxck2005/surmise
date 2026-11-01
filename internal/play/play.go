// Package play is the app's flows as plain Go: the rules that decide when a
// puzzle is saved, when its clock runs, what a restart may replace and what
// the keyboard shows, with no terminal and no phone in sight.
//
// internal/ui keeps its own copy of these rules for now; the Android app
// drives this package through the mobile module. The rules are the TUI's,
// ported with its test cases, so the two front ends cannot learn different
// lessons about the same save file. Nothing here imports Charm or
// internal/ui, and a test holds it to that.
//
// Time is injected (Clock) and nothing here starts a goroutine, so every rule
// can be tested with a clock that only moves when the test says.
package play

import (
	"time"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
)

// Clock is where the package reads the time. Production passes SystemClock;
// tests pass a clock they move by hand.
type Clock interface {
	Now() time.Time
}

// SystemClock is the wall clock.
type SystemClock struct{}

// Now returns time.Now.
func (SystemClock) Now() time.Time { return time.Now() }

// settingsStore is the half of a store that keeps the player's choices. Both
// shipped stores have it; a store without it keeps no settings and counts no
// playtime, the same reading internal/ui gives it.
type settingsStore interface {
	Settings() store.Settings
	SaveSettings(store.Settings) error
}

// App is one install: its store, its clock and the copy of its history that
// is read again only after a write. A front end holds one for its whole run.
// It is not safe for concurrent use; the mobile engine serialises its calls.
type App struct {
	store store.Store
	clock Clock

	hist       []*game.Game
	histWrites uint64
	histOK     bool

	// draw makes a random puzzle. It is game.New outside the tests, which
	// replace it because random ids cannot be made to collide on demand.
	draw func(int) (*game.Game, error)
}

// New makes an App over a store. A nil clock is the system clock.
func New(s store.Store, c Clock) *App {
	if c == nil {
		c = SystemClock{}
	}
	return &App{store: s, clock: c, draw: game.New}
}

// Store is the store the App was made with.
func (a *App) Store() store.Store { return a.store }

// Settings returns the saved choices, or the zero value for a store that keeps
// none. Every zero field means "nothing chosen".
func (a *App) Settings() store.Settings {
	if ss, ok := a.store.(settingsStore); ok {
		return ss.Settings()
	}
	return store.Settings{}
}

// UpdateSettings is the read-modify-write every settings change goes through,
// so a change to one preference can never lose another written meanwhile. It
// is a no-op for a store that keeps no settings.
//
// It is also the only path that may change Network, the consent to go online.
// Nothing else in this package writes that field: bankPlaytime reads it back
// unchanged, and that is the whole of the rule.
func (a *App) UpdateSettings(change func(*store.Settings)) error {
	ss, ok := a.store.(settingsStore)
	if !ok {
		return nil
	}
	s := ss.Settings()
	change(&s)
	return ss.SaveSettings(s)
}

// bankPlaytime adds a played session to the lifetime counter, the only thing
// that ever increases it. A failed write loses those seconds and nothing else;
// the counter re-floors from the records (stats.Playtime) on the profile.
func (a *App) bankPlaytime(d time.Duration) {
	if d <= 0 {
		return
	}
	ss, ok := a.store.(settingsStore)
	if !ok {
		return
	}
	s := ss.Settings()
	s.PlaytimeMS += d.Milliseconds()
	_ = ss.SaveSettings(s)
}

// History is every saved puzzle, as All returns it, read again only when the
// store has written since the last read (store.Writes). A store that does not
// count its writes is read every time. The games are shared: read them, never
// change them, and sort a copy.
func (a *App) History() ([]*game.Game, error) {
	counter, counts := a.store.(store.Writes)
	var writes uint64
	if counts {
		writes = counter.Writes()
		if a.histOK && writes == a.histWrites {
			return a.hist, nil
		}
	}
	// The count is taken before the read, so a write that lands during it
	// makes the next call read again.
	games, err := a.store.All()
	if err != nil {
		a.histOK = false
		return nil, err
	}
	a.hist, a.histWrites, a.histOK = games, writes, counts
	return games, nil
}

// newPuzzle draws a fresh random puzzle in memory. It is not saved until its
// first guess, but its code is final from the start.
//
// Codes are six digits, so a long history will eventually draw one already
// saved. That is harmless but reads as a glitch, so a random puzzle re-rolls a
// few times to avoid it. Only this random path re-rolls: a seeded puzzle (a
// daily, a challenge) keeps the id its seed decides. Falling out of the loop
// keeps the last draw, because a duplicate code is cosmetic and refusing to
// start a game over it would be far worse.
func (a *App) newPuzzle(length int) (*game.Game, error) {
	const attempts = 8
	taken, err := a.takenCodes()
	if err != nil {
		return nil, err
	}
	var g *game.Game
	for range attempts {
		if g, err = a.draw(length); err != nil {
			return nil, err
		}
		if !taken[game.Code(g.ID)] {
			break
		}
	}
	return g, nil
}

// takenCodes is the set of codes already on disk, from ids alone. A deleted
// finished puzzle's tombstone and an unreadable record both keep their codes:
// reusing either would make two puzzles look like one.
func (a *App) takenCodes() (map[string]bool, error) {
	ids, err := a.store.IDs()
	if err != nil {
		return nil, err
	}
	taken := make(map[string]bool, len(ids))
	for _, id := range ids {
		taken[game.Code(id)] = true
	}
	return taken, nil
}
