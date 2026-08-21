package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/words"
)

// The sprint tests drive the root model exactly as the app tests do: synthetic
// keys, motion off, and assertions on the frame. Motion off is what makes a
// finishing guess settle in the same Update — under it, dealNext runs the
// moment submit returns, so every test here is also proof that nothing about a
// run waits on an animation.

// openSprintSetup raises the sprint screen from the menu.
func openSprintSetup(t *testing.T, m *Model) {
	t.Helper()
	m.menu.point(menuIndex(t, m, choiceSprint, 0))
	m.Update(key("enter"))
	if m.screen != screenSprint {
		t.Fatalf("screen = %v, want the sprint setup", m.screen)
	}
}

// startSprintRun goes from setup to a live board.
func startSprintRun(t *testing.T, m *Model) {
	t.Helper()
	openSprintSetup(t, m)
	m.Update(key("enter"))
	if m.screen != screenGame || !m.sprinting() {
		t.Fatalf("after enter: screen = %v, running = %v", m.screen, m.sprinting())
	}
}

// typeWord spells a word on the board.
func typeWord(t *testing.T, m *Model, word string) {
	t.Helper()
	for _, r := range word {
		m.Update(key(string(r)))
	}
	m.Update(key("enter"))
}

// solveCurrent finishes the open board with its own answer.
func solveCurrent(t *testing.T, m *Model) {
	t.Helper()
	typeWord(t, m, strings.ToLower(m.game.g.Answer))
}

// otherAnswer finds an answer of length n that is not the given one, so a test
// can lose a board honestly — real words, never the solution.
func otherAnswer(t *testing.T, n int, not string) string {
	t.Helper()
	count, err := words.AnswerCount(n)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		w, err := words.AnswerAt(n, i)
		if err != nil {
			t.Fatal(err)
		}
		if w != strings.ToLower(not) && w != strings.ToUpper(not) && words.IsValidGuess(n, w) {
			return strings.ToLower(w)
		}
	}
	t.Fatalf("no answer of length %d besides %q", n, not)
	return ""
}

// loseCurrent spends every attempt on wrong words. Under motion off the losing
// guess deals the next board in the same Update, so completion is read as "the
// model is holding a different game" rather than as a status on a board that
// has already been swapped away.
func loseCurrent(t *testing.T, m *Model) {
	t.Helper()
	g := m.game.g
	wrong := otherAnswer(t, g.Length, g.Answer)
	for range g.MaxAttempts {
		if m.game.g != g {
			break
		}
		typeWord(t, m, wrong)
	}
	if m.game.g == g && !g.Status.Done() {
		t.Fatal("board did not finish")
	}
}

func TestMenuOffersSprint(t *testing.T) {
	m := newModel(t)
	i := menuIndex(t, m, choiceSprint, 0)
	c := m.menu.choices[i]
	if got, want := m.menu.weight(c).GetForeground(), st.text.GetForeground(); got != want {
		t.Errorf("sprint row weight = %v, want the bright text colour %v", got, want)
	}
}

func TestSprintSetupDefaultsAndCycles(t *testing.T) {
	m := newModel(t)
	openSprintSetup(t, m)

	s := &m.sprints
	if s.length != defaultLength || s.duration != defaultSprintDuration {
		t.Fatalf("setup = %d letters @ %s, want %d @ %s",
			s.length, s.duration, defaultLength, defaultSprintDuration)
	}

	// The clock row cycles through every offered duration, wrapping at both
	// ends; enter starts from whichever row the cursor is on, so the arrows are
	// checked without disturbing the start path.
	m.Update(key("down")) // onto the clock row
	for _, want := range []time.Duration{10 * time.Minute, 10 * time.Second} {
		m.Update(key("right"))
		if s.duration != want {
			t.Fatalf("duration after right = %s, want %s", s.duration, want)
		}
	}
	frame := m.View().Content
	if !strings.Contains(frame, "10s") {
		t.Error("frame does not show the cycled duration")
	}

	// Back to setup's left edge: esc leaves for the menu without starting.
	m.Update(key("esc"))
	if m.screen != screenMenu {
		t.Fatalf("esc from setup went to %v, want the menu", m.screen)
	}
	if m.sprint != nil {
		t.Error("leaving setup started a session anyway")
	}
}

func TestSprintStartDealsABoardOfTheChosenLength(t *testing.T) {
	m := newModel(t)
	startSprintRun(t, m)

	if m.sprints.length != m.game.g.Length {
		t.Fatalf("dealt a %d-letter board for a %d-letter run",
			m.game.g.Length, m.sprints.length)
	}
	if m.game.g.Custom || m.game.g.Daily != "" {
		t.Error("a run deals ordinary random boards only")
	}
	if m.game.persisted {
		t.Error("a dealt board is transient until its first guess")
	}
}

func TestSprintWinDealsTheNextBoardAndCountsIt(t *testing.T) {
	m := newModel(t)
	startSprintRun(t, m)

	first := m.game.g.ID

	solveCurrent(t, m)

	if m.screen != screenGame {
		t.Fatalf("after solving: screen = %v, want the next board, not %v", m.screen, m.screen)
	}
	if m.game.g.ID == first {
		t.Fatal("a solved board was not replaced")
	}

	s := m.sprint
	if s.dealt != 1 || s.solved != 1 || s.missed != 0 {
		t.Fatalf("tally = %d/%d/%d, want 1 solved of 1 dealt", s.dealt, s.solved, s.missed)
	}
	if s.attempts != 1 {
		t.Errorf("attempts = %d, want 1 (the one-word solve)", s.attempts)
	}

	// The finished board is on disk like any other random puzzle.
	items, err := m.store.List()
	if err != nil || len(items) != 1 {
		t.Fatalf("saved history = %d items (%v), want the solved puzzle", len(items), err)
	}
}

func TestSprintLossAlsoDeals(t *testing.T) {
	m := newModel(t)
	startSprintRun(t, m)

	first := m.game.g.ID
	loseCurrent(t, m)

	if m.game.g.ID == first {
		t.Fatal("a lost board was not replaced")
	}
	s := m.sprint
	if s.dealt != 1 || s.missed != 1 || s.solved != 0 {
		t.Fatalf("tally = %d/%d/%d, want 1 missed of 1 dealt", s.dealt, s.solved, s.missed)
	}
}

func TestSprintExpiryRaisesTheSummaryAndKeepsTheBoardResumable(t *testing.T) {
	m := newModel(t)
	startSprintRun(t, m)

	// One accepted guess persists the board; expiry must leave it resumable,
	// not destroyed and not silently finished.
	typeWord(t, m, otherAnswer(t, m.game.g.Length, m.game.g.Answer))
	if !m.game.persisted {
		t.Fatal("a typed guess should have persisted the board")
	}

	m.sprint.deadline = timeNow().Add(-time.Second)
	m.Update(tickMsg{})

	if m.screen != screenSprint || m.sprints.phase != sprintSummary {
		t.Fatalf("expiry landed on %v phase %v, want the summary", m.screen, m.sprints.phase)
	}
	frame := m.View().Content
	if !strings.Contains(frame, "sprint over") {
		t.Error("summary does not say the run is over")
	}
	if m.pendingDeal || m.pendingResult {
		t.Error("expiry left a deferred screen armed")
	}

	items, err := m.store.List()
	if err != nil || len(items) != 1 {
		t.Fatalf("history after expiry = %d items (%v), want the interrupted board", len(items), err)
	}
	g, err := m.store.Load(items[0].ID)
	if err != nil || g.Status.Done() {
		t.Fatalf("interrupted board is not resumable: %v, status %v", err, g.Status)
	}
}

func TestSprintInputAfterExpiryBelongsToTheSummary(t *testing.T) {
	m := newModel(t)
	startSprintRun(t, m)
	m.sprint.deadline = timeNow().Add(-time.Second)

	// A keystroke arrives for a board that is no longer part of anything. It
	// must close the run rather than type into it.
	m.Update(key("a"))

	if m.screen != screenSprint || m.sprints.phase != sprintSummary {
		t.Fatalf("input after expiry landed on %v phase %v", m.screen, m.sprints.phase)
	}
	if len(m.game.typing) != 0 {
		t.Error("an expired session took a letter onto the old board")
	}
}

func TestSprintEscEndsTheRunEarly(t *testing.T) {
	m := newModel(t)
	startSprintRun(t, m)
	typeWord(t, m, otherAnswer(t, m.game.g.Length, m.game.g.Answer))

	m.Update(key("esc"))

	if m.screen != screenSprint || m.sprints.phase != sprintSummary {
		t.Fatalf("esc mid-run went to %v phase %v, want the summary", m.screen, m.sprints.phase)
	}
	items, _ := m.store.List()
	if len(items) != 1 {
		t.Fatalf("history = %d items, want the abandoned board saved", len(items))
	}
}

func TestSprintRestartIsNotOfferedMidRun(t *testing.T) {
	m := newModel(t)
	startSprintRun(t, m)

	m.Update(key("tab"))
	if m.game.confirmNew {
		t.Fatal("tab armed a restart during a timed run")
	}

	frame := draw(t, m)
	if strings.Contains(frame, "new puzzle") {
		t.Error("help bar still offers a restart the keys refuse to arm")
	}
	if !strings.Contains(frame, "end sprint") {
		t.Error("help bar does not say esc ends the run")
	}
}

func TestSprintSummaryAgainRestartsOnTheSameSettings(t *testing.T) {
	m := newModel(t)
	startSprintRun(t, m)
	solveCurrent(t, m)

	// End the run and take the again offer.
	m.sprint.deadline = timeNow().Add(-time.Second)
	m.Update(tickMsg{})
	old := m.sprint
	if m.sprints.session != old {
		t.Fatal("the summary lost the session it is summarising")
	}

	m.Update(key("enter"))

	if m.screen != screenGame || !m.sprinting() {
		t.Fatalf("again went to %v running=%v, want a live board", m.screen, m.sprinting())
	}
	if m.sprint == old {
		t.Fatal("again resumed the finished session instead of a fresh one")
	}
	if m.sprint.length != old.length || m.sprint.duration != old.duration {
		t.Error("again changed the settings the player chose")
	}
	if m.sprint.dealt != 0 || m.sprint.solved != 0 {
		t.Errorf("fresh tally = %d/%d, want zeros", m.sprint.dealt, m.sprint.solved)
	}

	// And back out retires everything.
	m.Update(key("esc"))
	m.Update(key("esc"))
	if m.screen != screenMenu || m.sprint != nil {
		t.Fatalf("leaving the summary ended at %v with sprint=%v", m.screen, m.sprint)
	}
}

func TestSprintMouseAgreesWithKeys(t *testing.T) {
	m := newModel(t)
	draw(t, m)
	openSprintSetup(t, m)

	// Clicking the value steps it, like the arrow does.
	before := m.sprints.duration
	click(t, m, action{kind: actSprintNext, index: sprintRowTime})
	if m.sprints.duration == before {
		t.Error("clicking the clock row did not step it")
	}
	click(t, m, action{kind: actSprintStart})
	if m.screen != screenGame || !m.sprinting() {
		t.Fatalf("clicking start went to %v running=%v", m.screen, m.sprinting())
	}

	solveCurrent(t, m)
	m.sprint.deadline = timeNow().Add(-time.Second)
	draw(t, m)
	m.Update(tickMsg{})
	if m.screen != screenSprint {
		t.Fatalf("expiry landed on %v", m.screen)
	}
	click(t, m, action{kind: actSprintAgain})
	if m.screen != screenGame || m.sprint.dealt != 0 {
		t.Fatalf("clicking again went to %v with dealt=%d", m.screen, m.sprint.dealt)
	}
}

func TestSprintStatusLineShowsTheClockAndTally(t *testing.T) {
	at := time.Unix(1_000_000, 0)
	withClock(t, &at)

	m := newModel(t)
	startSprintRun(t, m)
	m.sprint.deadline = at.Add(90 * time.Second)
	freezeClock(m)

	frame := draw(t, m)
	if !strings.Contains(frame, "1:30") {
		t.Error("status line does not show the remaining time")
	}
	if !strings.Contains(frame, "0 solved") {
		t.Error("status line does not show the tally")
	}
}

func TestSprintCountsNothingTwice(t *testing.T) {
	m := newModel(t)
	startSprintRun(t, m)
	solveCurrent(t, m)

	// The finished board is gone already, but record is idempotent by id: a
	// second record of the same game must move nothing.
	finished := &game.Game{ID: m.sprint.lastID, Status: game.Won, Length: m.sprints.length}
	finished.MaxAttempts = finished.Length + 1
	m.sprint.record(finished)

	if m.sprint.dealt != 1 || m.sprint.solved != 1 {
		t.Fatalf("tally = %d/%d, want 1/1 — the same board counted twice", m.sprint.dealt, m.sprint.solved)
	}
}

func TestSprintDurationsAreWellFormed(t *testing.T) {
	seen := map[time.Duration]bool{}
	for _, d := range sprintDurations {
		if d <= 0 || seen[d] {
			t.Fatalf("duration list holds %s twice or non-positive", d)
		}
		seen[d] = true
	}
	if !seen[defaultSprintDuration] {
		t.Fatalf("default duration %s is not one of the offered values", defaultSprintDuration)
	}
	for _, d := range []time.Duration{10 * time.Second, 15 * time.Second, 30 * time.Second, time.Minute} {
		if !seen[d] {
			t.Errorf("short duration %s missing from the list", d)
		}
	}
}
