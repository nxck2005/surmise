package words

import "testing"

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
