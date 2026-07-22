package game

import (
	"strings"
	"testing"
)

// marks renders a score compactly so failures are readable:
// '.' absent, '?' present, '#' correct.
func marks(ms []Mark) string {
	var b strings.Builder
	for _, m := range ms {
		switch m {
		case Correct:
			b.WriteByte('#')
		case Present:
			b.WriteByte('?')
		default:
			b.WriteByte('.')
		}
	}
	return b.String()
}

func TestScore(t *testing.T) {
	tests := []struct {
		name   string
		guess  string
		answer string
		want   string
	}{
		{"all correct", "crane", "crane", "#####"},
		{"all absent", "boils", "raged", "....."},
		{"mixed, no duplicates", "crane", "cargo", "#??.."},

		// Duplicates: each letter of the answer justifies exactly one mark,
		// and exact matches claim theirs before leftovers are handed out.

		// "lotto" has one L; the guess has two. The earlier one claims it.
		{"double in guess, single in answer", "allot", "lotto", ".?.??"},
		// The only E is claimed by the exact match at the end, so the two
		// earlier Es score absent.
		{"exact match consumes the only copy", "geese", "abide", "....#"},
		// "sassy" has three S; one is exact, so a later S can still be present.
		{"triple in answer", "loses", "sassy", "..#.?"},
		// Second A finds nothing left after the first claims the only A.
		{"repeat exhausts the letter", "kayak", "khaki", "#?..?"},
		// Trailing E matches exactly; of the two leading Es only one is left.
		{"later exact match wins over earlier", "eerie", "there", "?.?.#"},

		// Other lengths must behave identically; nothing may assume 5.
		{"four letters", "cool", "loco", "?#??"},
		{"six letters", "batter", "better", "#.####"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := marks(Score(tt.guess, tt.answer))
			if got != tt.want {
				t.Errorf("Score(%q, %q) = %s, want %s", tt.guess, tt.answer, got, tt.want)
			}
		})
	}
}

// TestScoreNeverOverCountsLetter asserts the invariant that makes the two-pass
// algorithm correct: a letter is never marked more times than it occurs in the
// answer. This is what a naive one-pass implementation gets wrong.
func TestScoreNeverOverCountsLetter(t *testing.T) {
	cases := [][2]string{
		{"allot", "lotto"}, {"geese", "abide"}, {"eerie", "there"},
		{"kayak", "khaki"}, {"loses", "sassy"}, {"aaaaa", "banal"},
	}
	for _, c := range cases {
		guess, answer := c[0], c[1]
		var inAnswer, marked [26]int
		for i := range answer {
			inAnswer[answer[i]-'a']++
		}
		for i, m := range Score(guess, answer) {
			if m != Absent {
				marked[guess[i]-'a']++
			}
		}
		for i := range marked {
			if marked[i] > inAnswer[i] {
				t.Errorf("Score(%q, %q): letter %c marked %d times but occurs %d times",
					guess, answer, 'a'+rune(i), marked[i], inAnswer[i])
			}
		}
	}
}

// TestScoreOfAnswerIsAllCorrect guards the win condition: whatever the
// duplicates, guessing the answer must score every position Correct.
func TestScoreOfAnswerIsAllCorrect(t *testing.T) {
	for _, w := range []string{"crane", "lotto", "geese", "aaaaa", "batter", "cool"} {
		if got := marks(Score(w, w)); got != strings.Repeat("#", len(w)) {
			t.Errorf("Score(%q, %q) = %s, want all correct", w, w, got)
		}
	}
}

func TestScorePanicsOnLengthMismatch(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Score with mismatched lengths did not panic")
		}
	}()
	Score("abcd", "abcde")
}
