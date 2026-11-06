package play

import (
	"encoding/json"
	"errors"
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
)

// fakeClock only moves when a test says so.
type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

const fixedID = "3f2a7c1e-8b4d-4c6a-9e2f-1d5b8a7c3e90"

// newApp is an App over a fresh native store and a stopped clock.
func newApp(t *testing.T) (*App, *store.JSON, *fakeClock) {
	t.Helper()
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c := &fakeClock{t: time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)}
	return New(s, c), s, c
}

// fixed opens an unsaved board whose answer is crane.
func fixed(t *testing.T, a *App) *Board {
	t.Helper()
	g, err := game.NewFrom(fixedID, "crane", 5)
	if err != nil {
		t.Fatal(err)
	}
	// NewFrom stamps the wall clock; the calendar files a puzzle by its
	// start, so pin it to the fake clock's day or the test breaks on Nov 2.
	g.StartedAt = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	return a.Open(g, false)
}

func typeWord(b *Board, w string) {
	for _, c := range w {
		b.Type(c)
	}
}

func guess(t *testing.T, b *Board, w string) {
	t.Helper()
	typeWord(b, w)
	if err := b.Submit(); err != nil {
		t.Fatalf("Submit(%q): %v", w, err)
	}
}

func onDisk(t *testing.T, s store.Store) []string {
	t.Helper()
	ids, err := s.IDs()
	if err != nil {
		t.Fatal(err)
	}
	return ids
}

// A board nobody guessed on never becomes a record, even after typing and
// leaving.
func TestUnplayedPuzzleIsNotSaved(t *testing.T) {
	a, s, _ := newApp(t)
	b, err := a.NewRandom(5)
	if err != nil {
		t.Fatal(err)
	}
	typeWord(b, "cra")
	if err := b.Leave(); err != nil {
		t.Fatal(err)
	}
	if ids := onDisk(t, s); len(ids) != 0 {
		t.Errorf("saved %v, want nothing", ids)
	}
}

func TestPuzzleSavedOnFirstGuess(t *testing.T) {
	a, s, _ := newApp(t)
	b := fixed(t, a)
	guess(t, b, "about")
	got, err := s.Load(fixedID)
	if err != nil {
		t.Fatalf("Load after the first guess: %v", err)
	}
	if len(got.Guesses) != 1 || got.Guesses[0] != "about" {
		t.Errorf("saved guesses %v, want [about]", got.Guesses)
	}
}

// Every guess is written at once, so a killed app loses nothing.
func TestEveryGuessIsSaved(t *testing.T) {
	a, s, _ := newApp(t)
	b := fixed(t, a)
	guess(t, b, "about")
	guess(t, b, "slate")
	got, err := s.Load(fixedID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Guesses) != 2 {
		t.Errorf("saved %d guesses, want 2", len(got.Guesses))
	}
}

func TestTypingDeletingAndTrimming(t *testing.T) {
	a, _, _ := newApp(t)
	b := fixed(t, a)
	typeWord(b, "AbOu7t!x") // case folds, non-letters drop, the row caps at 5
	if b.Typing() != "about" {
		t.Fatalf("typing = %q, want about", b.Typing())
	}
	b.Delete()
	if b.Typing() != "abou" {
		t.Errorf("after Delete typing = %q, want abou", b.Typing())
	}
	b.Trim(2)
	if b.Typing() != "ab" {
		t.Errorf("after Trim(2) typing = %q, want ab", b.Typing())
	}
	b.Trim(5) // past the typing: nothing
	if b.Typing() != "ab" {
		t.Errorf("after Trim(5) typing = %q, want ab", b.Typing())
	}
	b.Delete()
	b.Delete()
	b.Delete() // on an empty row: nothing
	if b.Typing() != "" {
		t.Errorf("typing = %q, want empty", b.Typing())
	}
}

// A refused guess costs nothing: no attempt, no save, and the typing stays
// so the player can fix it.
func TestRefusedGuessesChangeNothing(t *testing.T) {
	a, s, _ := newApp(t)
	b := fixed(t, a)
	typeWord(b, "abo")
	if err := b.Submit(); !errors.Is(err, ErrTooShort) {
		t.Errorf("short guess: err = %v, want ErrTooShort", err)
	}
	b.Trim(0)
	typeWord(b, "zzzzz")
	if err := b.Submit(); !errors.Is(err, ErrNotAWord) {
		t.Errorf("non-word: err = %v, want ErrNotAWord", err)
	}
	if CodeOf(b.Submit()) != "not_a_word" {
		t.Error("the refusal does not carry its code")
	}
	if b.Typing() != "zzzzz" {
		t.Errorf("typing = %q after a refusal, want it kept", b.Typing())
	}
	if n := len(b.Game().Guesses); n != 0 {
		t.Errorf("%d guesses after refusals, want 0", n)
	}
	if ids := onDisk(t, s); len(ids) != 0 {
		t.Errorf("a refused guess saved %v", ids)
	}
}

func TestWinningAndLosing(t *testing.T) {
	a, _, _ := newApp(t)
	b := fixed(t, a)
	guess(t, b, "crane")
	v := b.View()
	if v.Status != string(game.Won) || v.Answer != "crane" {
		t.Errorf("won view: status %q answer %q", v.Status, v.Answer)
	}
	typeWord(b, "about")
	if b.Typing() != "" {
		t.Error("a finished board took letters")
	}
	if err := b.Submit(); !errors.Is(err, ErrFinished) {
		t.Errorf("submit on a won board: %v, want ErrFinished", err)
	}

	g, err := game.NewFrom("9b1d2c3e-4f5a-4b6c-8d7e-0f1a2b3c4d5e", "crane", 5)
	if err != nil {
		t.Fatal(err)
	}
	l := a.Open(g, false)
	for range g.MaxAttempts {
		guess(t, l, "about")
	}
	v = l.View()
	if v.Status != string(game.Lost) || v.Answer != "crane" {
		t.Errorf("lost view: status %q answer %q", v.Status, v.Answer)
	}
}

// The answer must not leave Go while the puzzle is live.
func TestViewHidesTheAnswerUntilTheEnd(t *testing.T) {
	a, _, _ := newApp(t)
	b := fixed(t, a)
	guess(t, b, "about")
	raw, err := json.Marshal(b.View())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "crane") {
		t.Errorf("a live board's view carries the answer: %s", raw)
	}
}

func TestViewRowsAndKeys(t *testing.T) {
	a, _, _ := newApp(t)
	b := fixed(t, a)
	guess(t, b, "react") // r present, e present, a correct, c present, t absent
	typeWord(b, "ca")
	v := b.View()
	if v.Code != game.Code(fixedID) || v.Length != 5 || v.Typing != "ca" {
		t.Errorf("view header: %+v", v)
	}
	want := []string{"present", "present", "correct", "present", "absent"}
	if len(v.Rows) != 1 || strings.Join(v.Rows[0].Marks, ",") != strings.Join(want, ",") {
		t.Errorf("rows = %+v, want react marked %v", v.Rows, want)
	}
	if v.Keys["a"] != "correct" || v.Keys["t"] != "absent" || v.Keys["r"] != "present" {
		t.Errorf("keys = %v", v.Keys)
	}
	if _, ok := v.Keys["z"]; ok {
		t.Error("an untouched letter has a key state")
	}
}

// A letter's key never downgrades: correct beats present beats absent.
func TestKeyboardKeepsBestLetterState(t *testing.T) {
	a, _, _ := newApp(t)
	b := fixed(t, a)
	guess(t, b, "cigar") // c correct
	guess(t, b, "occur") // c present then absent
	if got := b.View().Keys["c"]; got != "correct" {
		t.Errorf("c = %q, want correct", got)
	}
}

func TestKeyboardStateIsRebuiltForEachBoard(t *testing.T) {
	a, _, _ := newApp(t)
	b, err := a.NewRandom(5)
	if err != nil {
		t.Fatal(err)
	}
	typeWord(b, "about")
	if err := b.Submit(); err != nil && !errors.Is(err, ErrNotAWord) {
		t.Fatal(err)
	}
	if err := b.Restart(); err != nil {
		t.Fatal(err)
	}
	if k := b.View().Keys; len(k) != 0 {
		t.Errorf("a fresh board inherited keys %v", k)
	}
}

// The clock runs from the first letter, not from opening the board.
func TestClockStartsOnFirstLetter(t *testing.T) {
	a, _, c := newApp(t)
	b := fixed(t, a)
	c.advance(time.Minute)
	if v := b.View(); v.Running || v.ElapsedMS != 0 {
		t.Errorf("before typing: running %v elapsed %d", v.Running, v.ElapsedMS)
	}
	b.Type('a')
	c.advance(3 * time.Second)
	b.Type('b')
	c.advance(2 * time.Second)
	if v := b.View(); !v.Running || v.ElapsedMS != 5000 {
		t.Errorf("after typing: running %v elapsed %d, want true 5000", v.Running, v.ElapsedMS)
	}
}

func TestSolveTimeIsBankedFromTheFirstLetter(t *testing.T) {
	a, s, c := newApp(t)
	b := fixed(t, a)
	c.advance(time.Hour) // staring at the board is free
	b.Type('c')
	c.advance(7 * time.Second)
	typeWord(b, "rane")
	if err := b.Submit(); err != nil {
		t.Fatal(err)
	}
	c.advance(time.Hour) // and so is reviewing it
	if err := b.Leave(); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load(fixedID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ElapsedMS != 7000 {
		t.Errorf("solve time %d ms, want 7000", got.ElapsedMS)
	}
	if pt := s.Settings().PlaytimeMS; pt != 7000 {
		t.Errorf("playtime %d ms, want 7000", pt)
	}
}

// Leave banks the session, and leaving again banks nothing more.
func TestLeaveBanksOnce(t *testing.T) {
	a, s, c := newApp(t)
	b := fixed(t, a)
	guess(t, b, "about")
	b.Type('s')
	c.advance(4 * time.Second)
	for range 2 {
		if err := b.Leave(); err != nil {
			t.Fatal(err)
		}
		c.advance(time.Minute)
	}
	got, err := s.Load(fixedID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ElapsedMS != 4000 || s.Settings().PlaytimeMS != 4000 {
		t.Errorf("elapsed %d playtime %d, want 4000 both", got.ElapsedMS, s.Settings().PlaytimeMS)
	}
	// The board stays open, and its clock starts again on the next letter.
	b.Delete()
	c.advance(time.Minute)
	if b.View().Running {
		t.Error("the clock runs again without a letter")
	}
	b.Type('l')
	c.advance(time.Second)
	if v := b.View(); !v.Running || v.ElapsedMS != 5000 {
		t.Errorf("after a new letter: running %v elapsed %d, want true 5000", v.Running, v.ElapsedMS)
	}
}

func TestResumingWithoutTypingAddsNoTime(t *testing.T) {
	a, s, c := newApp(t)
	b := fixed(t, a)
	guess(t, b, "about")
	if err := b.Leave(); err != nil {
		t.Fatal(err)
	}
	before, err := s.Load(fixedID)
	if err != nil {
		t.Fatal(err)
	}
	r, err := a.Resume(fixedID)
	if err != nil {
		t.Fatal(err)
	}
	c.advance(time.Hour)
	if err := r.Leave(); err != nil {
		t.Fatal(err)
	}
	after, err := s.Load(fixedID)
	if err != nil {
		t.Fatal(err)
	}
	if after.ElapsedMS != before.ElapsedMS || !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Errorf("a look at a board changed it: elapsed %d→%d, updated %v→%v",
			before.ElapsedMS, after.ElapsedMS, before.UpdatedAt, after.UpdatedAt)
	}
}

// A board left before its first guess is discarded, but the minutes spent on
// it still count as time in the app.
func TestDiscardedBoardCountsPlaytimeOnly(t *testing.T) {
	a, s, c := newApp(t)
	b := fixed(t, a)
	b.Type('c')
	c.advance(9 * time.Second)
	if err := b.Leave(); err != nil {
		t.Fatal(err)
	}
	if pt := s.Settings().PlaytimeMS; pt != 9000 {
		t.Errorf("playtime %d ms, want 9000", pt)
	}
	if ids := onDisk(t, s); len(ids) != 0 {
		t.Errorf("a discarded board saved %v", ids)
	}
}

// Banking playtime is a read-modify-write: it keeps every other choice,
// consent to go online included.
func TestPlaytimeKeepsOtherSettings(t *testing.T) {
	a, s, c := newApp(t)
	if err := a.UpdateSettings(func(st *store.Settings) {
		st.Network = true
		st.DisplayName = "nick"
	}); err != nil {
		t.Fatal(err)
	}
	b := fixed(t, a)
	b.Type('c')
	c.advance(time.Second)
	if err := b.Leave(); err != nil {
		t.Fatal(err)
	}
	got := s.Settings()
	if !got.Network || got.DisplayName != "nick" || got.PlaytimeMS != 1000 {
		t.Errorf("settings after banking: %+v", got)
	}
}

func TestRestartRefusals(t *testing.T) {
	a, _, _ := newApp(t)
	mk := func(edit func(*game.Game)) *Board {
		g, err := game.NewFrom(fixedID, "crane", 5)
		if err != nil {
			t.Fatal(err)
		}
		edit(g)
		return a.Open(g, false)
	}
	cases := []struct {
		name string
		b    *Board
		want error
	}{
		{"daily", mk(func(g *game.Game) { g.Daily = "2026-11-01" }), ErrRestartDaily},
		{"custom", mk(func(g *game.Game) { g.Custom = true }), ErrRestartCustom},
		{"challenge", mk(func(g *game.Game) { g.Challenge = &game.ChallengeInfo{Code: "x"} }), ErrRestartChallenge},
		{"room", mk(func(g *game.Game) { g.Room = &game.RoomInfo{} }), ErrRestartRoom},
	}
	sprint := mk(func(*game.Game) {})
	sprint.SetSprint(true)
	cases = append(cases, struct {
		name string
		b    *Board
		want error
	}{"sprint", sprint, ErrRestartSprint})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := tc.b.Game().ID
			if tc.b.View().CanRestart {
				t.Error("the view offers a restart")
			}
			if err := tc.b.Restart(); !errors.Is(err, tc.want) {
				t.Errorf("Restart: %v, want %v", err, tc.want)
			}
			if tc.b.Game().ID != id {
				t.Error("a refused restart replaced the board")
			}
		})
	}
}

func TestRestartLeavesTheOldBoardFirst(t *testing.T) {
	a, s, c := newApp(t)
	b := fixed(t, a)
	guess(t, b, "about")
	b.Type('s')
	c.advance(2 * time.Second)
	if !b.View().CanRestart {
		t.Fatal("a random board offers no restart")
	}
	if err := b.Restart(); err != nil {
		t.Fatal(err)
	}
	if b.Game().ID == fixedID || b.Typing() != "" || b.View().Running {
		t.Errorf("restart kept the old board's state: %+v", b.View())
	}
	old, err := s.Load(fixedID)
	if err != nil {
		t.Fatal(err)
	}
	if old.ElapsedMS != 2000 {
		t.Errorf("old board banked %d ms, want 2000", old.ElapsedMS)
	}
	if ids := onDisk(t, s); len(ids) != 1 {
		t.Errorf("records after restart: %v, want only the old one", ids)
	}
}

func TestNewPuzzleRerollsACollidingCode(t *testing.T) {
	a, _, _ := newApp(t)
	guess(t, fixed(t, a), "about")
	draws := 0
	a.draw = func(length int) (*game.Game, error) {
		draws++
		g, err := game.New(length)
		if err != nil {
			return nil, err
		}
		if draws == 1 {
			g.ID = fixedID // same id, so necessarily the same code
		}
		return g, nil
	}
	b, err := a.NewRandom(5)
	if err != nil {
		t.Fatal(err)
	}
	if draws != 2 {
		t.Errorf("drew %d times, want 2", draws)
	}
	if game.Code(b.Game().ID) == game.Code(fixedID) {
		t.Error("kept the colliding code")
	}
}

func TestNewPuzzleGivesUpRerollingRatherThanFail(t *testing.T) {
	a, _, _ := newApp(t)
	guess(t, fixed(t, a), "about")
	draws := 0
	a.draw = func(length int) (*game.Game, error) {
		draws++
		g, err := game.New(length)
		if err != nil {
			return nil, err
		}
		g.ID = fixedID // every draw collides
		return g, nil
	}
	b, err := a.NewRandom(5)
	if err != nil || b == nil {
		t.Fatalf("NewRandom: %v", err)
	}
	if draws != 8 {
		t.Errorf("drew %d times, want 8", draws)
	}
}

func TestTakenCodesReserveDeletedAndUnreadableRecords(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := New(s, &fakeClock{})
	var ids []string
	for range 3 {
		g, err := game.New(5)
		if err != nil {
			t.Fatal(err)
		}
		b := a.Open(g, false)
		guess(t, b, g.Answer)
		ids = append(ids, g.ID)
	}
	if err := s.Delete(ids[1]); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "puzzles", ids[2]+".json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	taken, err := a.takenCodes()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if !taken[game.Code(id)] {
			t.Errorf("code of record %s is not reserved", id)
		}
	}
}

// writeCounter counts All, to prove the history copy is read only after a
// write.
type writeCounter struct {
	*store.JSON
	all int
}

func (w *writeCounter) All() ([]*game.Game, error) { w.all++; return w.JSON.All() }

func TestHistoryIsReadOnlyAfterAWrite(t *testing.T) {
	js, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &writeCounter{JSON: js}
	a := New(s, &fakeClock{})
	for range 3 {
		if _, err := a.History(); err != nil {
			t.Fatal(err)
		}
	}
	if s.all != 1 {
		t.Errorf("three reads with no write read the store %d times, want 1", s.all)
	}
	b := fixed(t, a)
	guess(t, b, "about")
	games, err := a.History()
	if err != nil {
		t.Fatal(err)
	}
	if s.all != 2 || len(games) != 1 {
		t.Errorf("after a save: %d reads, %d games; want 2 and 1", s.all, len(games))
	}
}

func TestKindLabels(t *testing.T) {
	cases := []struct {
		edit func(*game.Game)
		want string
	}{
		{func(*game.Game) {}, KindRandom},
		{func(g *game.Game) { g.Daily = "2026-11-01" }, KindDaily},
		{func(g *game.Game) { g.Custom = true }, KindCustom},
		{func(g *game.Game) { g.Challenge = &game.ChallengeInfo{} }, KindChallenge},
		{func(g *game.Game) { g.Room = &game.RoomInfo{} }, KindRoom},
	}
	for _, tc := range cases {
		g, err := game.NewFrom(fixedID, "crane", 5)
		if err != nil {
			t.Fatal(err)
		}
		tc.edit(g)
		if got := kindOf(g); got != tc.want {
			t.Errorf("kindOf = %q, want %q", got, tc.want)
		}
	}
}

// The package is the front-end-free half of the app: nothing it imports,
// however indirectly, may be Charm or the TUI.
func TestImportsNoFrontEnd(t *testing.T) {
	seen := map[string]bool{}
	var walk func(path string)
	walk = func(path string) {
		if seen[path] {
			return
		}
		seen[path] = true
		if strings.HasPrefix(path, "charm.land/") || strings.Contains(path, "charmbracelet") ||
			strings.HasSuffix(path, "/internal/ui") {
			t.Errorf("internal/play reaches %s", path)
			return
		}
		pkg, err := build.Import(path, ".", 0)
		if err != nil {
			return // standard library and anything outside the module
		}
		for _, imp := range pkg.Imports {
			walk(imp)
		}
	}
	walk("github.com/nxck2005/surmise/internal/play")
}
