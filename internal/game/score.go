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
// So guessing "allot" against "lotto" marks the first L Present and the second
// Absent: the answer has one L, neither position matches it exactly, and the
// leftover pass hands it to the earlier of the two.
func Score(guess, answer string) []Mark {
	if len(guess) != len(answer) {
		panic("game: Score called with mismatched lengths")
	}

	marks := make([]Mark, len(guess))

	// Index the strings directly. []byte(guess) copies both words onto the
	// stack for no gain, and this runs once per guess on every save and every
	// record read (Validate rescores the board it is checking).
	var remaining [26]int
	for i := 0; i < len(answer); i++ {
		if idx := answer[i] - 'a'; idx < 26 {
			remaining[idx]++
		}
	}

	// Pass 1: exact positional matches take priority over everything else.
	for i := 0; i < len(guess); i++ {
		if guess[i] == answer[i] {
			marks[i] = Correct
			if idx := guess[i] - 'a'; idx < 26 {
				remaining[idx]--
			}
		}
	}

	// Pass 2: distribute whatever letters are left over.
	for i := 0; i < len(guess); i++ {
		if marks[i] == Correct {
			continue
		}
		idx := guess[i] - 'a'
		if idx < 26 && remaining[idx] > 0 {
			marks[i] = Present
			remaining[idx]--
		}
	}

	return marks
}
