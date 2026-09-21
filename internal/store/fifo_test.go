//go:build unix

package store

import (
	"syscall"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/game"
)

// A planted FIFO used to hang every scan that read the history: os.Open on a
// FIFO with no writer blocks forever, and All runs on startup (the menu
// status), the profile, the list and the daily guard. The refusal has to come
// from the mode, before the open. The timeout is what makes a regression a
// failure rather than a job that never finishes.
func TestAFIFOCannotHangAHistoryScan(t *testing.T) {
	s := newStore(t)
	g := newGame(t, 5)
	if err := syscall.Mkfifo(mustPath(t, s, g.ID), 0o600); err != nil {
		t.Fatalf("Mkfifo: %v", err)
	}

	var (
		loadErr error
		games   []*game.Game
		allErr  error
	)
	within(t, func() { _, loadErr = s.Load(g.ID) })
	if loadErr == nil {
		t.Error("Load read a FIFO as a puzzle")
	}
	within(t, func() { games, allErr = s.All() })
	if allErr != nil {
		t.Errorf("All: %v", allErr)
	}
	if len(games) != 0 {
		t.Errorf("All = %v, want an empty history", games)
	}
}

// Settings is read on every startup, so the same plant at settings.json is the
// same hang; a refused settings file degrades to the defaults rather than
// costing the player anything.
func TestAFIFOCannotHangSettings(t *testing.T) {
	s := newStore(t)
	if err := syscall.Mkfifo(s.settingsPath(), 0o600); err != nil {
		t.Fatalf("Mkfifo: %v", err)
	}
	var got Settings
	within(t, func() { got = s.Settings() })
	if got != (Settings{}) {
		t.Errorf("Settings = %+v, want the defaults", got)
	}
}

// within runs fn on its own goroutine and fails if it does not finish. A
// blocked open never returns, and a failing test is a better signal than a CI
// job that hangs until its timeout.
func within(t *testing.T, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the read blocked: a non-regular file was opened")
	}
}
