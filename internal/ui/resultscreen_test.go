package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/brand"
	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
)

func TestShareResultIsSpoilerSafe(t *testing.T) {
	g, err := game.NewFrom("share-win", "crane", 5)
	if err != nil {
		t.Fatal(err)
	}
	g.Daily = "2026-08-11"
	for _, guess := range []string{"about", "crane"} {
		if err := g.Guess(guess); err != nil {
			t.Fatal(err)
		}
	}
	g.AddElapsed(83 * time.Second)

	want := fmt.Sprintf("%s #%s 2/6\n", brand.Name, game.Code(g.ID)) +
		"daily 2026-08-11 · 5 letters · 1:23\n" +
		"□····\n" +
		"■■■■■\n" +
		"■ correct  □ present  · absent"
	if got := shareResult(g); got != want {
		t.Errorf("shareResult() =\n%q\nwant\n%q", got, want)
	}
	for _, spoiler := range append([]string{g.Answer}, g.Guesses...) {
		if strings.Contains(shareResult(g), spoiler) {
			t.Errorf("share result contains spoiler %q", spoiler)
		}
	}
}

func TestShareResultMarksALoss(t *testing.T) {
	g, err := game.NewFrom("share-loss", "crane", 5)
	if err != nil {
		t.Fatal(err)
	}
	for range g.MaxAttempts {
		if err := g.Guess("about"); err != nil {
			t.Fatal(err)
		}
	}
	g.AddElapsed(9 * time.Second)

	got := shareResult(g)
	wantHead := fmt.Sprintf("%s #%s X/6\n5 letters · 9s\n",
		brand.Name, game.Code(g.ID))
	if !strings.HasPrefix(got, wantHead) {
		t.Errorf("loss share starts with %q, want %q", got, wantHead)
	}
	if rows := strings.Count(got, "□····\n"); rows != g.MaxAttempts {
		t.Errorf("loss share has %d scored rows, want %d\n%s", rows, g.MaxAttempts, got)
	}
	if strings.Contains(got, g.Answer) || strings.Contains(got, "about") {
		t.Errorf("loss share contains an answer or guess\n%s", got)
	}
}

func TestResultReviewAndNext(t *testing.T) {
	m := gameModel(t)
	first := m.game.g.ID
	send(t, m, "c", "r", "a", "n", "e", "enter")
	if m.screen != screenResult {
		t.Fatalf("screen after winning = %v, want result", m.screen)
	}

	view := send(t, m, "enter")
	if m.screen != screenGame {
		t.Fatalf("screen after review = %v, want game", m.screen)
	}
	if !strings.Contains(view, "Q") || !strings.Contains(view, "solved in 1") {
		t.Errorf("review did not restore the finished board\n%s", view)
	}

	send(t, m, "enter")
	if m.screen != screenResult {
		t.Fatalf("enter on a reviewed board opened %v, want result", m.screen)
	}
	send(t, m, "n")
	if m.screen != screenGame {
		t.Fatalf("screen after next = %v, want game", m.screen)
	}
	if m.game.g.ID == first || m.game.g.Status != game.InProgress {
		t.Errorf("next game = id %q status %v, want a new in-progress puzzle",
			m.game.g.ID, m.game.g.Status)
	}
}

func TestResultCopyAndMenu(t *testing.T) {
	m := gameModel(t)
	send(t, m, "c", "r", "a", "n", "e", "enter")

	_, cmd := m.Update(key("c"))
	if cmd == nil {
		t.Fatal("copy returned no clipboard command")
	}
	if !m.result.copyRequested {
		t.Fatal("copy did not record its acknowledgement")
	}
	if view := m.View().Content; !strings.Contains(view, "copy requested") {
		t.Errorf("copy acknowledgement is not visible\n%s", view)
	}

	send(t, m, "esc")
	if m.screen != screenMenu {
		t.Errorf("screen after esc = %v, want menu", m.screen)
	}
}

func TestDailyResultNextReturnsToDailyModes(t *testing.T) {
	m := dailyModel(t, Options{})
	playDaily(t, m, 5)
	m.game.g.Answer = "crane"
	send(t, m, "c", "r", "a", "n", "e", "enter")
	if m.screen != screenResult {
		t.Fatalf("screen after daily win = %v, want result", m.screen)
	}

	view := send(t, m, "n")
	if m.screen != screenDaily {
		t.Fatalf("screen after daily next = %v, want daily", m.screen)
	}
	if !strings.Contains(view, "5 letters") || !strings.Contains(view, "solved 1/6") {
		t.Errorf("daily list did not refresh the completed mode\n%s", view)
	}
}

type failCompletionSaveStore struct {
	store.Store
	failed bool
}

func (s *failCompletionSaveStore) Save(g *game.Game) error {
	if g.Status.Done() && !s.failed {
		s.failed = true
		return errors.New("disk full")
	}
	return s.Store.Save(g)
}

func TestResultRetriesAFailedCompletionSaveOnExit(t *testing.T) {
	base, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &failCompletionSaveStore{Store: base}
	m := New(s, nil, Options{Motion: motionOffName})
	m.screen = screenMenu
	send(t, m, "down", "enter")
	m.game.g.Answer = "crane"
	id := m.game.g.ID

	view := send(t, m, "c", "r", "a", "n", "e", "enter")
	if m.screen != screenResult || !strings.Contains(view, "could not save: disk full") {
		t.Fatalf("failed completion save was hidden\n%s", view)
	}
	if _, err := base.Load(id); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Load() after failed save = %v, want ErrNotFound", err)
	}

	send(t, m, "esc")
	if m.screen != screenMenu {
		t.Fatalf("screen after successful save retry = %v, want menu", m.screen)
	}
	if saved, err := base.Load(id); err != nil {
		t.Fatalf("Load() after save retry: %v", err)
	} else if saved.Status != game.Won {
		t.Errorf("saved status = %v, want won", saved.Status)
	}
}
