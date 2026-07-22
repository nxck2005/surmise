package game

import (
	"errors"
	"testing"

	"github.com/nxck2005/wortle/internal/words"
)

// newFixed builds a game with a known answer, so tests do not depend on the
// random draw.
func newFixed(t *testing.T, answer string) *Game {
	t.Helper()
	g, err := New(len(answer), 1)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	g.Answer = answer
	return g
}

func TestNewUsesSupportedLengths(t *testing.T) {
	for _, n := range words.Lengths {
		g, err := New(n, 7)
		if err != nil {
			t.Fatalf("New(%d): %v", n, err)
		}
		if g.Length != n || len(g.Answer) != n {
			t.Errorf("New(%d): answer %q has wrong length", n, g.Answer)
		}
		if g.MaxAttempts != n+1 {
			t.Errorf("New(%d): MaxAttempts = %d, want %d", n, g.MaxAttempts, n+1)
		}
		if g.Status != InProgress || g.Number != 7 || g.ID == "" {
			t.Errorf("New(%d): unexpected initial state %+v", n, g)
		}
		if err := g.Validate(); err != nil {
			t.Errorf("New(%d): Validate: %v", n, err)
		}
	}
}

func TestNewIDsAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		g, err := New(5, 1)
		if err != nil {
			t.Fatal(err)
		}
		if seen[g.ID] {
			t.Fatalf("duplicate id %q", g.ID)
		}
		seen[g.ID] = true
	}
}

func TestGuessWinning(t *testing.T) {
	g := newFixed(t, "crane")
	if err := g.Guess("about"); err != nil {
		t.Fatalf("Guess: %v", err)
	}
	if g.Status != InProgress {
		t.Errorf("status after wrong guess = %v, want in progress", g.Status)
	}
	if err := g.Guess("CRANE"); err != nil { // casing must be accepted
		t.Fatalf("Guess: %v", err)
	}
	if g.Status != Won {
		t.Errorf("status = %v, want won", g.Status)
	}
	if g.Attempts() != 2 {
		t.Errorf("Attempts() = %d, want 2", g.Attempts())
	}
}

func TestGuessLosingAfterMaxAttempts(t *testing.T) {
	g := newFixed(t, "crane")
	for i := range g.MaxAttempts {
		if g.Status.Done() {
			t.Fatalf("finished early after %d guesses", i)
		}
		if err := g.Guess("about"); err != nil {
			t.Fatalf("guess %d: %v", i, err)
		}
	}
	if g.Status != Lost {
		t.Errorf("status = %v, want lost", g.Status)
	}
	if g.Remaining() != 0 {
		t.Errorf("Remaining() = %d, want 0", g.Remaining())
	}
	if !errors.Is(g.Guess("about"), ErrFinished) {
		t.Error("guessing after loss should return ErrFinished")
	}
}

// Rejected input must not cost the player an attempt.
func TestInvalidGuessDoesNotConsumeAttempt(t *testing.T) {
	g := newFixed(t, "crane")

	if !errors.Is(g.Guess("zzzzz"), ErrNotAWord) {
		t.Error("nonsense word should return ErrNotAWord")
	}
	if !errors.Is(g.Guess("cat"), ErrWrongLength) {
		t.Error("short word should return ErrWrongLength")
	}
	if g.Attempts() != 0 {
		t.Errorf("Attempts() = %d after rejected guesses, want 0", g.Attempts())
	}
	if g.Remaining() != g.MaxAttempts {
		t.Errorf("Remaining() = %d, want %d", g.Remaining(), g.MaxAttempts)
	}
}

func TestLetterStatesKeepsBestMark(t *testing.T) {
	g := newFixed(t, "crane")
	// "areas": a is present (pos 0), r present, e present, then... use two
	// guesses so a letter is seen as Present before being seen as Correct.
	if err := g.Guess("acorn"); err != nil {
		t.Fatal(err)
	}
	if err := g.Guess("crane"); err != nil {
		t.Fatal(err)
	}

	states := g.LetterStates()
	for _, c := range "crane" {
		if states[byte(c)] != Correct {
			t.Errorf("letter %c = %v, want correct", c, states[byte(c)])
		}
	}
	if states['o'] != Absent {
		t.Errorf("letter o = %v, want absent", states['o'])
	}
}

func TestValidateRejectsCorruptState(t *testing.T) {
	tests := []struct {
		name  string
		munge func(*Game)
	}{
		{"no id", func(g *Game) { g.ID = "" }},
		{"bad length", func(g *Game) { g.Length = 9 }},
		{"answer length mismatch", func(g *Game) { g.Answer = "toolong" }},
		{"marks out of sync", func(g *Game) { g.Marks = nil }},
		{"too many guesses", func(g *Game) { g.MaxAttempts = 1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newFixed(t, "crane")
			for _, w := range []string{"about", "acorn"} {
				if err := g.Guess(w); err != nil {
					t.Fatal(err)
				}
			}
			tt.munge(g)
			if err := g.Validate(); err == nil {
				t.Error("Validate accepted corrupt game")
			}
		})
	}
}

func TestAddElapsedIgnoresNonPositive(t *testing.T) {
	g := newFixed(t, "crane")
	g.AddElapsed(-5)
	if g.Elapsed() != 0 {
		t.Errorf("Elapsed() = %v after negative add, want 0", g.Elapsed())
	}
}
