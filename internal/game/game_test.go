package game

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/words"
)

// missOfThisLength is a real word of length n, distinct from the answer, so a
// record built from it is one play could have produced. Both come from the
// answer list, which every length carries in bulk.
func missOfThisLength(t *testing.T, n int, answer string, i int) string {
	t.Helper()
	for at := 0; ; at++ {
		w, err := words.AnswerAt(n, (i+at)%256)
		if err != nil {
			t.Fatal(err)
		}
		if w != answer {
			return w
		}
	}
}

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

// An id is a persistence key, so the set of shapes is closed: the two this app
// has ever written, and nothing else. Accepting more would let a crafted save
// or backup name itself in a way the store cannot safely hold — the id becomes
// a filename in the JSON store and a key suffix in the browser's.
func TestValidID(t *testing.T) {
	valid := []string{
		"7f3a1c0b9d2e4f56",                     // a pre-UUID id, as older saves hold
		"3f2a7b4c-5d6e-4f70-8123-456789abcdef", // a random puzzle's UUIDv4
		"0130405e-2c98-8e40-ba2c-dce569a50a05", // a derived puzzle's UUIDv8
		"0000000000000000",
	}
	for _, id := range valid {
		if !ValidID(id) {
			t.Errorf("ValidID(%q) = false, want true", id)
		}
	}

	invalid := []string{
		"",
		"x",
		"fuzz-seed",
		"../escape",
		`..\escape`,
		"a/b",
		".",
		"..",
		"./abc",
		"abc.",
		"with space",
		"3f2a7b4c-5d6e-4f70-8123-456789abcde",   // one character short
		"3f2a7b4c-5d6e-4f70-8123-456789abcdef0", // one character long
		// Uppercase is a different string on Linux and the same file on macOS
		// and Windows, so the store could only promise one of the two readings.
		"3F2A7B4C-5D6E-4F70-8123-456789ABCDEF",
		"7F3A1C0B9D2E4F56",
		"3f2a7b4c_5d6e_4f70_8123_456789abcdef",
		"3f2a7b4c-5d6e-4f70-8123-456789abcdeg", // not hex
		"con",                                  // a device name on Windows
		"\x1b]52;c;x\x07",
		"café",
	}
	for _, id := range invalid {
		if ValidID(id) {
			t.Errorf("ValidID(%q) = true, want false", id)
		}
	}

	for _, make := range []func() (*Game, error){
		func() (*Game, error) { return New(5) },
		func() (*Game, error) { return NewCustom("nishu", 5) },
	} {
		g, err := make()
		if err != nil {
			t.Fatal(err)
		}
		if !ValidID(g.ID) {
			t.Errorf("a constructor produced the invalid id %q", g.ID)
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
	const id = "3f2a7b4c-5d6e-4f70-8123-456789abcdef"
	cases := []struct {
		name       string
		id, answer string
		length     int
	}{
		{"no id", "", "about", 5},
		{"id that could escape a store", "../escape", "about", 5},
		{"unsupported length", id, "abouts", 7},
		{"answer does not match length", id, "about", 4},
		{"answer is not a word", id, "zzzzz", 5},
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

// A caller with a map of its own gets the same answer, and the fill clears it
// first: a letter left from the board it was last filled from would be a mark
// this game never made. The keyboard is the caller that does this every frame.
func TestFillLetterStatesClearsAndRefills(t *testing.T) {
	g := newFixed(t, "crane")
	if err := g.Guess("acorn"); err != nil {
		t.Fatal(err)
	}

	reused := map[byte]Mark{'z': Correct}
	before := g.LetterStates()
	g.FillLetterStates(reused)

	if len(reused) != len(before) {
		t.Fatalf("reused map holds %d letters, want %d", len(reused), len(before))
	}
	for c, want := range before {
		if reused[c] != want {
			t.Errorf("letter %c = %v, want %v", c, reused[c], want)
		}
	}
	if _, stale := reused['z']; stale {
		t.Error("a letter from the previous fill survived")
	}

	// Filling the very same map again is idempotent rather than accumulating.
	g.FillLetterStates(reused)
	if len(reused) != len(before) {
		t.Errorf("a second fill left %d letters, want %d", len(reused), len(before))
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
		// The two of these are one invariant: a board allows length+1 guesses,
		// and a record may not hold more than that whatever it claims. Both
		// matter because the composer's height ladder reads MaxAttempts while
		// the board itself draws one row per guess, so a record where the two
		// disagree sizes a frame it then overflows.
		{"too many guesses", func(g *Game) { g.MaxAttempts = 1 }},
		// The fixture is a five-letter board played twice over, so it holds two
		// guesses of six. Repeating the last one past the limit is the least
		// record that disagrees, and a player who lost a five-letter board does
		// hold six — so the shape is a real one, not a synthetic one.
		{"more guesses than the board allows", func(g *Game) {
			for len(g.Guesses) < g.MaxAttempts+1 {
				last := g.Guesses[len(g.Guesses)-1]
				g.Guesses = append(g.Guesses, last)
				g.Marks = append(g.Marks, Score(last, g.Answer))
			}
		}},
		// maxAttempts is derived, not stored state: every board has always
		// allowed length+1 guesses, and a record claiming otherwise would only
		// be a way to make the composer draw a board of the wrong size.
		{"maxAttempts too large", func(g *Game) { g.MaxAttempts = 99 }},
		{"unknown status", func(g *Game) { g.Status = "surrendered" }},
		{"mark out of range", func(g *Game) { g.Marks[0][0] = 7 }},
		// Words become what the board draws, tile by tile and byte by byte, so
		// anything but lowercase letters is a way to smuggle terminal control
		// bytes back out through a saved game or an imported backup.
		{"escape in the answer", func(g *Game) { g.Answer = "cr\x1bne" }},
		{"escape in a guess", func(g *Game) { g.Guesses[0] = "abo\x1bt" }},
		{"uppercase answer", func(g *Game) { g.Answer = "CRANE" }},
		{"digit in a guess", func(g *Game) { g.Guesses[1] = "ac0rn" }},
		{"daily is not a date", func(g *Game) { g.Daily = "2026-13-01" }},
		{"daily carries an escape", func(g *Game) { g.Daily = "2026-08-06\x1b" }},
		// ElapsedMS is multiplied by time.Millisecond, so a value past a
		// duration's range wraps Elapsed() negative and takes the profile's
		// totals and averages with it. See MaxElapsedMS.
		{"negative elapsed", func(g *Game) { g.ElapsedMS = -1 }},
		{"elapsed past a duration", func(g *Game) { g.ElapsedMS = math.MaxInt64 }},
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

// The guess bound is inclusive too, and in both directions: a record holding
// exactly as many guesses as the board allows is the last one that is valid,
// and the refusal must be the refusal and not an off-by-one either way. A board
// played to its last attempt is a real thing — a loss — so the boundary is
// where a player can actually get to.
func TestValidateGuessCountAtItsBound(t *testing.T) {
	for _, n := range words.Lengths {
		t.Run(fmt.Sprintf("%d letters", n), func(t *testing.T) {
			// A loss: every attempt used, which is the deepest a valid record
			// for this length can go. The guess is a real word of this length
			// so the only thing wrong with the record is how deep it is.
			answer, err := words.AnswerAt(n, 0)
			if err != nil {
				t.Fatal(err)
			}
			// The guesses are built directly rather than played: what is under
			// test is the shape of a record at the limit, not how a game is
			// played to it. Every word is a real one of this length and none is
			// the answer, so the only thing wrong with the record is its depth.
			g := newFixed(t, answer)
			for i := range attemptsFor(n) {
				miss := missOfThisLength(t, n, answer, i)
				g.Guesses = append(g.Guesses, miss)
				g.Marks = append(g.Marks, Score(miss, answer))
			}
			g.Status = Lost
			if len(g.Guesses) != g.MaxAttempts {
				t.Fatalf("the fixture holds %d guesses, not %d", len(g.Guesses), g.MaxAttempts)
			}
			if err := g.Validate(); err != nil {
				t.Errorf("a record at exactly the attempt limit was refused: %v", err)
			}
		})
	}
}

// The bound is inclusive: exactly a duration's worth of milliseconds is a
// value the profile can still render, and it is the largest such value.
func TestValidateAcceptsElapsedAtItsBound(t *testing.T) {
	g := newFixed(t, "crane")
	g.ElapsedMS = MaxElapsedMS
	if err := g.Validate(); err != nil {
		t.Errorf("Validate at MaxElapsedMS: %v", err)
	}
}

// A daily label is drawn on the board header, the browse list and the result
// card, so it is held to the one form the app writes: an exact calendar date.
func TestValidateAcceptsOnlyRealDailyDates(t *testing.T) {
	g := newFixed(t, "crane")
	g.Daily = "2026-08-06"
	if err := g.Validate(); err != nil {
		t.Errorf("Validate with a daily date: %v", err)
	}
	for _, bad := range []string{
		"2026-8-6", // unpadded
		"2026-13-01",
		"2026-02-30",
		"yesterday",
		"2026-08-06\x1b",
		"2026-08-06 ",
	} {
		g.Daily = bad
		if err := g.Validate(); err == nil {
			t.Errorf("Validate accepted daily %q", bad)
		}
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
	tomb = g.Tombstone()
	tomb.Status = "surrendered"
	if err := tomb.Validate(); err == nil {
		t.Error("Validate accepted a tombstone with an invented status")
	}
	for _, elapsed := range []int64{-1, MaxElapsedMS + 1} {
		tomb = g.Tombstone()
		tomb.ElapsedMS = elapsed
		if err := tomb.Validate(); err == nil {
			t.Errorf("Validate accepted a tombstone with elapsedMs %d", elapsed)
		}
	}
}

func TestAddElapsedIgnoresNonPositive(t *testing.T) {
	g := newFixed(t, "crane")
	g.AddElapsed(-5)
	if g.Elapsed() != 0 {
		t.Errorf("Elapsed() = %v after negative add, want 0", g.Elapsed())
	}
}

func TestAddElapsedSaturatesAtTheBound(t *testing.T) {
	g := newFixed(t, "crane")
	g.ElapsedMS = MaxElapsedMS - 1
	g.AddElapsed(2 * time.Millisecond)
	if g.ElapsedMS != MaxElapsedMS {
		t.Errorf("ElapsedMS = %d after an over-bound add, want %d", g.ElapsedMS, MaxElapsedMS)
	}
}

func TestNewCustomTakesAWordOutsideTheList(t *testing.T) {
	const secret = "nishu" // a name: a real length, and not in any list
	if words.IsValidGuess(5, secret) {
		t.Skipf("%q reached the word list; pick another non-word", secret)
	}

	if _, err := NewFrom("3f2a7b4c-5d6e-4f70-8123-456789abcdef", secret, 5); err == nil {
		t.Fatal("NewFrom accepted an off-list answer; it must stay strict")
	}

	g, err := NewCustom(secret, 5)
	if err != nil {
		t.Fatalf("NewCustom: %v", err)
	}
	if g.Answer != secret {
		t.Errorf("Answer = %q, want %q", g.Answer, secret)
	}
	if !g.Custom {
		t.Error("NewCustom did not mark the puzzle custom")
	}
	if g.MaxAttempts != attemptsFor(5) {
		t.Errorf("MaxAttempts = %d, want %d", g.MaxAttempts, attemptsFor(5))
	}
}

func TestNewCustomRefusesAWordNoBoardCouldHold(t *testing.T) {
	for _, tc := range []struct {
		name   string
		answer string
		length int
	}{
		{"too short", "cran", 5},
		{"too long", "cranes", 5},
		{"unsupported length", "no", 2},
		{"not letters", "cr4ne", 5},
		{"punctuation", "cr-ne", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewCustom(tc.answer, tc.length); err == nil {
				t.Errorf("NewCustom(%q, %d) was accepted", tc.answer, tc.length)
			}
		})
	}
}

func TestAnOffListAnswerCanStillBeTyped(t *testing.T) {
	const secret = "nishu"
	if words.IsValidGuess(5, secret) {
		t.Skipf("%q reached the word list; pick another non-word", secret)
	}
	g, err := NewCustom(secret, 5)
	if err != nil {
		t.Fatal(err)
	}

	// Every other off-list word is still refused: relaxing the answer must not
	// turn the board into a free-text field.
	if err := g.Guess("zzzzz"); !errors.Is(err, ErrNotAWord) {
		t.Errorf("Guess(zzzzz) = %v, want ErrNotAWord", err)
	}
	if g.Attempts() != 0 {
		t.Errorf("a refused guess cost an attempt: %d", g.Attempts())
	}

	if err := g.Guess(secret); err != nil {
		t.Fatalf("Guess(%q) = %v, want the answer to be playable", secret, err)
	}
	if g.Status != Won {
		t.Errorf("Status = %q after guessing the answer, want %q", g.Status, Won)
	}
}

func TestCustomPuzzlesAreLeftOutOfTheFigures(t *testing.T) {
	g := newFixed(t, "crane")
	if !g.CountsForStats() {
		t.Error("an ordinary puzzle does not count")
	}
	g.Custom = true
	if g.CountsForStats() {
		t.Error("a custom puzzle counts")
	}
}

func TestCustomIsOmittedFromAnOrdinarySave(t *testing.T) {
	g := newFixed(t, "about")
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte(`"custom"`)) {
		t.Errorf("ordinary save carries a custom key: %s", b)
	}

	g.Custom = true
	b, err = json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	var back Game
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if !back.Custom {
		t.Error("Custom was lost in a round trip")
	}
}

func TestTombstoneKeepsTheCustomMarker(t *testing.T) {
	g := newFixed(t, "crane")
	g.Custom = true
	if err := g.Guess("crane"); err != nil {
		t.Fatal(err)
	}

	tomb := g.Tombstone()
	if !tomb.Custom {
		t.Error("Tombstone dropped the custom marker: deleting one would move a streak")
	}
	if tomb.Answer != "" || len(tomb.Guesses) != 0 {
		t.Errorf("Tombstone kept the play record: %+v", tomb)
	}
}

func TestChallengeMetadataAndTombstone(t *testing.T) {
	g := newFixed(t, "crane")
	g.Challenge = &ChallengeInfo{Code: "4500-820C-20A1-G73J"}
	if !g.CountsForStats() {
		t.Error("challenge does not count for stats")
	}
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	var back Game
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Challenge == nil || back.Challenge.Code != g.Challenge.Code {
		t.Errorf("Challenge = %+v after round trip, want %+v", back.Challenge, g.Challenge)
	}

	tomb := g.Tombstone()
	if tomb.Challenge == nil {
		t.Fatal("challenge tombstone lost its origin marker")
	}
	if tomb.Challenge.Code != "" {
		t.Errorf("challenge tombstone retained code %q", tomb.Challenge.Code)
	}
	if err := tomb.Validate(); err != nil {
		t.Fatalf("Validate(tombstone): %v", err)
	}
}

func TestValidateRejectsInvalidChallengeMetadata(t *testing.T) {
	g := newFixed(t, "crane")
	g.Challenge = &ChallengeInfo{}
	if err := g.Validate(); err == nil {
		t.Error("Validate accepted a live challenge with no code")
	}

	g.Challenge.Code = "code"
	g.Custom = true
	if err := g.Validate(); err == nil {
		t.Error("Validate accepted a challenge that was also custom")
	}
}
