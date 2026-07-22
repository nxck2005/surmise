package game

// Mark is the result for a single letter of a guess.
type Mark int

const (
	// Absent means the letter does not appear in the answer, or every
	// occurrence of it is already accounted for by other marks in this guess.
	Absent Mark = iota
	// Present means the letter appears in the answer at a different position.
	Present
	// Correct means the letter appears at this position in the answer.
	Correct
)

func (m Mark) String() string {
	switch m {
	case Correct:
		return "correct"
	case Present:
		return "present"
	default:
		return "absent"
	}
}

// Score compares a guess against the answer, returning one Mark per letter.
// Both must be the same length and already normalized.
//
// Duplicate letters are the subtle part, and the reason this runs in two
// passes. Each letter of the answer can only justify one mark. Exact matches
// are claimed first, then leftover letters are handed out left to right; once
// a letter is used up, further occurrences in the guess score Absent.
//
// So guessing "allot" against "lotto" marks the first L Absent and the second
// Present: the answer has one L, and the second position claims it first.
func Score(guess, answer string) []Mark {
	if len(guess) != len(answer) {
		panic("game: Score called with mismatched lengths")
	}

	g, a := []byte(guess), []byte(answer)
	marks := make([]Mark, len(g))

	// Count the answer's letters, so the second pass knows how many of each
	// remain to be claimed.
	var remaining [26]int
	for _, c := range a {
		if idx := c - 'a'; idx < 26 {
			remaining[idx]++
		}
	}

	// Pass 1: exact positional matches take priority over everything else.
	for i := range g {
		if g[i] == a[i] {
			marks[i] = Correct
			if idx := g[i] - 'a'; idx < 26 {
				remaining[idx]--
			}
		}
	}

	// Pass 2: distribute whatever letters are left over.
	for i := range g {
		if marks[i] == Correct {
			continue
		}
		idx := g[i] - 'a'
		if idx < 26 && remaining[idx] > 0 {
			marks[i] = Present
			remaining[idx]--
		}
	}

	return marks
}
