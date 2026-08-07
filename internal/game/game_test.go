package game

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/words"
)

// newFixed builds a game with a known answer, so tests do not depend on the
// random draw.
func newFixed(t *testing.T, answer string) *Game {
	t.Helper()
	g, err := New(len(answer))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	g.Answer = answer
	return g
}

func TestNewUsesSupportedLengths(t *testing.T) {
	for _, n := range words.Lengths {
		g, err := New(n)
		if err != nil {
			t.Fatalf("New(%d): %v", n, err)
		}
		if g.Length != n || len(g.Answer) != n {
			t.Errorf("New(%d): answer %q has wrong length", n, g.Answer)
		}
		if g.MaxAttempts != n+1 {
			t.Errorf("New(%d): MaxAttempts = %d, want %d", n, g.MaxAttempts, n+1)
		}
		if g.Status != InProgress || g.ID == "" {
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
		g, err := New(5)
		if err != nil {
			t.Fatal(err)
		}
		if seen[g.ID] {
			t.Fatalf("duplicate id %q", g.ID)
		}
		seen[g.ID] = true
	}
}

func TestNewIDsAreUUIDv4(t *testing.T) {
	// 8-4-4-4-12 hex, with the version nibble 4 and the variant nibble in 8..b.
	shape := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	for range 50 {
		g, err := New(5)
		if err != nil {
			t.Fatal(err)
		}
		if !shape.MatchString(g.ID) {
			t.Fatalf("id %q is not a v4 UUID", g.ID)
		}
	}
}

func TestNewFromKeepsTheIdentityItIsGiven(t *testing.T) {
	const id = "13f0405e-2c98-8e40-ba2c-dce569a50a05"
	g, err := NewFrom(id, "  ABOUT ", 5)
	if err != nil {
		t.Fatal(err)
	}
	if g.ID != id {
		t.Errorf("ID = %q, want %q", g.ID, id)
	}
	// The answer is normalized, the way a guess is, so a hand-written source
	// cannot produce a puzzle that can never be matched.
	if g.Answer != "about" {
		t.Errorf("Answer = %q, want %q", g.Answer, "about")
	}
	if g.MaxAttempts != 6 || g.Status != InProgress {
		t.Errorf("unexpected initial state %+v", g)
	}
	if g.StartedAt.IsZero() || g.UpdatedAt.IsZero() {
		t.Error("timestamps were not set")
	}
	// game knows nothing about dates: whoever derived the puzzle labels it.
	if g.Daily != "" {
		t.Errorf("Daily = %q, want empty", g.Daily)
	}
	if err := g.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestNewFromRejectsUnplayableInput(t *testing.T) {
	cases := []struct {
		name       string
		id, answer string
		length     int
	}{
		{"no id", "", "about", 5},
		{"unsupported length", "id", "abouts", 7},
		{"answer does not match length", "id", "about", 4},
		{"answer is not a word", "id", "zzzzz", 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := NewFrom(c.id, c.answer, c.length); err == nil {
				t.Error("succeeded, want error")
			}
		})
	}
}

// Daily rides on the Deleted precedent: it must be absent from an ordinary
// save, so no existing file changes shape and older saves decode to "".
func TestDailyIsOmittedFromAnOrdinarySave(t *testing.T) {
	g := newFixed(t, "about")
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte(`"daily"`)) {
		t.Errorf("ordinary save carries a daily key: %s", b)
	}

	g.Daily = "2026-08-06"
	b, err = json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	var back Game
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Daily != "2026-08-06" {
		t.Errorf("Daily = %q after a round trip, want 2026-08-06", back.Daily)
	}
}

func TestCodeIsSixDigitsAndDeterministic(t *testing.T) {
	digits := regexp.MustCompile(`^[0-9]{6}$`)
	ids := []string{
		"",                                     // degenerate, must still format
		"7f3a1c0b9d2e4f56",                     // a pre-UUID id, as older saves hold
		"3f2a1b4c-5d6e-4f70-8123-456789abcdef", // a UUID
	}
	for range 200 {
		g, err := New(5)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, g.ID)
	}

	for _, id := range ids {
		code := Code(id)
		if !digits.MatchString(code) {
			t.Errorf("Code(%q) = %q, want six digits", id, code)
		}
		if again := Code(id); again != code {
			t.Errorf("Code(%q) is not deterministic: %q then %q", id, code, again)
		}
	}
}

// A code is six digits including the leading zeros, or the column it is drawn
// in stops lining up.
func TestCodeKeepsLeadingZeros(t *testing.T) {
	// Hunt for an id hashing below 100000, which is where the padding is the
	// only thing holding the width at six. One in ten ids qualifies.
	for i := range 1000 {
		id := fmt.Sprintf("id-%d", i)
		if code := Code(id); code[0] == '0' {
			if len(code) != 6 {
				t.Fatalf("Code(%q) = %q, want six digits", id, code)
			}
			return
		}
	}
	t.Fatal("no id hashed below 100000 in 1000 tries, which is implausible")
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

func TestTombstoneKeepsOnlyTheSequence(t *testing.T) {
	g := newFixed(t, "crane")
	for _, w := range []string{"about", "crane"} {
		if err := g.Guess(w); err != nil {
			t.Fatal(err)
		}
	}
	g.AddElapsed(time.Second)

	tomb := g.Tombstone()
	if !tomb.Deleted {
		t.Error("Tombstone is not marked deleted")
	}
	if tomb.ID != g.ID || tomb.Length != g.Length || tomb.Status != g.Status {
		t.Errorf("Tombstone = %+v, want id/length/status of %+v", tomb, g)
	}
	if !tomb.UpdatedAt.Equal(g.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want %v", tomb.UpdatedAt, g.UpdatedAt)
	}
	if tomb.Answer != "" || len(tomb.Guesses) != 0 || len(tomb.Marks) != 0 || tomb.ElapsedMS != 0 {
		t.Errorf("Tombstone kept the play record: %+v", tomb)
	}
	// The original must be untouched — Tombstone returns a copy.
	if g.Answer != "crane" || len(g.Guesses) != 2 {
		t.Errorf("Tombstone mutated its receiver: %+v", g)
	}
	// A stripped record still has to survive the store's round trip.
	if err := tomb.Validate(); err != nil {
		t.Errorf("Validate(tombstone) = %v, want nil", err)
	}
	// A casual puzzle has no date to keep.
	if tomb.Daily != "" {
		t.Errorf("Daily = %q on a casual tombstone, want empty", tomb.Daily)
	}
}

// A deleted daily has to remember which day it was, or the daily streak — which
// is indexed by date rather than by completion order — cannot tell it from a
// day never played, and the runs either side of a deleted loss merge.
func TestTombstoneKeepsTheDailyDate(t *testing.T) {
	g := newFixed(t, "crane")
	g.Daily = "2026-08-06"
	if err := g.Guess("crane"); err != nil {
		t.Fatal(err)
	}

	tomb := g.Tombstone()
	if tomb.Daily != "2026-08-06" {
		t.Errorf("Daily = %q, want 2026-08-06", tomb.Daily)
	}
	// The date is all it keeps: the day is on the record, not how it went.
	if tomb.Answer != "" || len(tomb.Guesses) != 0 {
		t.Errorf("Tombstone kept the play record: %+v", tomb)
	}
}

func TestValidateRejectsCorruptTombstone(t *testing.T) {
	g := newFixed(t, "crane")
	tomb := g.Tombstone()
	tomb.Length = 9
	if err := tomb.Validate(); err == nil {
		t.Error("Validate accepted a tombstone with an unsupported length")
	}
	tomb = g.Tombstone()
	tomb.ID = ""
	if err := tomb.Validate(); err == nil {
		t.Error("Validate accepted a tombstone with no id")
	}
}

func TestAddElapsedIgnoresNonPositive(t *testing.T) {
	g := newFixed(t, "crane")
	g.AddElapsed(-5)
	if g.Elapsed() != 0 {
		t.Errorf("Elapsed() = %v after negative add, want 0", g.Elapsed())
	}
}
