package words

import (
	_ "embed"
	"strings"
	"testing"
)

// blocked is the same hand-maintained list genwords filters with. It is
// embedded here rather than in words.go because nothing at runtime needs it:
// the shipped lists are already clean, and this test is what keeps them that
// way after a regeneration.
//
//go:embed data/blocked.txt
var blockedList string

// TestListsLoad guards against a broken go:embed or a bad genwords run: the
// lists must be present, correctly sized, and self-consistent.
func TestListsLoad(t *testing.T) {
	for _, n := range Lengths {
		answers, guesses, err := Counts(n)
		if err != nil {
			t.Fatalf("length %d: %v", n, err)
		}
		if answers == 0 || guesses == 0 {
			t.Errorf("length %d: empty lists (%d answers, %d guesses)", n, answers, guesses)
		}
		if answers > guesses {
			t.Errorf("length %d: more answers (%d) than guesses (%d)", n, answers, guesses)
		}
	}
}

// TestEveryAnswerIsGuessable checks the invariant the whole game depends on:
// a puzzle can never have an answer the player is not allowed to type.
func TestEveryAnswerIsGuessable(t *testing.T) {
	for _, n := range Lengths {
		l, err := get(n)
		if err != nil {
			t.Fatalf("length %d: %v", n, err)
		}
		for _, a := range l.answers {
			if len(a) != n {
				t.Fatalf("length %d: answer %q has wrong length", n, a)
			}
			if _, ok := l.guesses[a]; !ok {
				t.Errorf("length %d: answer %q is not a valid guess", n, a)
			}
		}
	}
}

// TestNoBlockedWords checks that no slur is shippable, as an answer or as an
// accepted guess. A regeneration that skipped the blocklist fails here.
func TestNoBlockedWords(t *testing.T) {
	words := strings.Fields(blockedList)
	if len(words) == 0 {
		t.Fatal("data/blocked.txt is empty")
	}

	for _, n := range Lengths {
		l, err := get(n)
		if err != nil {
			t.Fatalf("length %d: %v", n, err)
		}
		for _, w := range words {
			if len(w) != n {
				continue
			}
			if _, ok := l.guesses[w]; ok {
				t.Errorf("length %d: blocked word is an accepted guess", n)
			}
		}
		// Answers are a subset of guesses, so the check above covers them; walk
		// them anyway so a broken subset invariant cannot hide a slur.
		blockedSet := make(map[string]struct{}, len(words))
		for _, w := range words {
			blockedSet[w] = struct{}{}
		}
		for _, a := range l.answers {
			if _, bad := blockedSet[a]; bad {
				t.Errorf("length %d: blocked word is a puzzle answer", n)
			}
		}
	}
}

func TestRandomReturnsPlayableWord(t *testing.T) {
	for _, n := range Lengths {
		for range 50 {
			w, err := Random(n)
			if err != nil {
				t.Fatalf("length %d: %v", n, err)
			}
			if len(w) != n {
				t.Fatalf("Random(%d) = %q, wrong length", n, w)
			}
			if !IsValidGuess(n, w) {
				t.Fatalf("Random(%d) = %q, not a valid guess", n, w)
			}
		}
	}
}

func TestAnswerAtIsStableAndPlayable(t *testing.T) {
	for _, n := range Lengths {
		count, err := AnswerCount(n)
		if err != nil {
			t.Fatalf("length %d: %v", n, err)
		}
		if count == 0 {
			t.Fatalf("length %d: empty answer pool", n)
		}

		// Every index must name a playable word, and name the same one twice:
		// the daily depends on an index meaning the same thing everywhere.
		for _, i := range []int{0, count / 2, count - 1} {
			w, err := AnswerAt(n, i)
			if err != nil {
				t.Fatalf("AnswerAt(%d, %d): %v", n, i, err)
			}
			if len(w) != n || !IsValidGuess(n, w) {
				t.Fatalf("AnswerAt(%d, %d) = %q, not a playable answer", n, i, w)
			}
			if again, _ := AnswerAt(n, i); again != w {
				t.Fatalf("AnswerAt(%d, %d) = %q then %q, not stable", n, i, w, again)
			}
		}
	}
}

func TestAnswerAtRejectsOutOfRange(t *testing.T) {
	count, err := AnswerCount(5)
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range []int{-1, count, count + 1} {
		if _, err := AnswerAt(5, i); err == nil {
			t.Errorf("AnswerAt(5, %d) succeeded, want error", i)
		}
	}
}

func TestIsValidGuessNormalizes(t *testing.T) {
	if !IsValidGuess(5, "  ABOUT ") {
		t.Error(`IsValidGuess(5, "  ABOUT ") = false, want true`)
	}
	if IsValidGuess(5, "zzzzz") {
		t.Error(`IsValidGuess(5, "zzzzz") = true, want false`)
	}
	if IsValidGuess(5, "about!") {
		t.Error(`IsValidGuess(5, "about!") = true, want false`)
	}
}

func TestUnsupportedLength(t *testing.T) {
	if SupportedLength(7) {
		t.Error("SupportedLength(7) = true, want false")
	}
	if _, err := Random(7); err == nil {
		t.Error("Random(7) succeeded, want error")
	}
}
