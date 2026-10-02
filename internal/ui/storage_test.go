package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
)

// The meter appears only where the platform can say how full the store is, it
// is read when the screen opens rather than remembered from the last visit,
// and it says plainly when storage is nearly or wholly spent.
func TestBackupScreenShowsHowFullStorageIs(t *testing.T) {
	plain := backupModel(t, &fakeTransfer{})
	plain.screen = screenMenu
	plain.menu.point(menuIndex(t, plain, choiceBackup, 0))
	if view := send(t, plain, "enter"); strings.Contains(view, "storage") {
		t.Errorf("a build with no Storage shows a meter\n%s", view)
	}

	used := 0
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := New(s, nil, Options{
		Motion:   motionOffName,
		Transfer: &fakeTransfer{},
		Storage:  func() (int, int) { return used, 1000 },
	})
	m.screen = screenMenu
	row := menuIndex(t, m, choiceBackup, 0)

	for _, c := range []struct {
		used int
		want []string
	}{
		{310, []string{"storage: about 31% used"}},
		{935, []string{"storage is 93% full", "save a backup while you can"}},
		{1000, []string{"storage is full", "new progress is not being saved"}},
		{1200, []string{"storage is full"}},
	} {
		used = c.used
		m.screen = screenMenu
		m.menu.point(row)
		view := send(t, m, "enter")
		if m.screen != screenBackup {
			t.Fatalf("enter on the backup row opened %v", m.screen)
		}
		for _, w := range c.want {
			if !strings.Contains(view, w) {
				t.Errorf("at %d of 1000 the screen does not say %q\n%s", c.used, w, view)
			}
		}
	}
}

// fullStore refuses every puzzle save the way a browser out of room does.
type fullStore struct{ store.Store }

func (fullStore) Save(*game.Game) error { return fmt.Errorf("browser %w", store.ErrFull) }

// A save refused for lack of room says so on the board, in words that point
// at the backup screen, rather than in a browser's exception name.
func TestFullStorageSaysSoOnTheBoard(t *testing.T) {
	base, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := New(fullStore{base}, nil, Options{Motion: motionOffName})
	m.screen = screenMenu
	send(t, m, "down", "enter")
	m.game.g.Answer = "crane"

	view := send(t, m, "s", "l", "a", "t", "e", "enter")
	if !strings.Contains(view, "storage is full — not saved (see backup)") {
		t.Errorf("a full store is not reported on the board\n%s", view)
	}

	if got := saveFailed(fmt.Errorf("disk on fire")); got != "could not save: disk on fire" {
		t.Errorf("saveFailed(other) = %q", got)
	}
}
