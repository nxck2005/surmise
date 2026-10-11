package online

import (
	"strings"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/words"
)

// RoomName is a display name made fit for a room: the Part 2 rules of the
// plan, the same ones the server applies to a hello (its cleanName). The two
// must reach the same answer so what the player typed is what everyone sees.
//
//  1. Keep only A–Z, a–z, 0–9, space, '_' and '-'.
//  2. Trim leading and trailing spaces.
//  3. Cut to the first 12 characters, then trim the end again.
//  4. A name whose letter runs are blocked or profane becomes empty.
//
// Empty means "show player N", which is what the server sends for a name it
// cannot use either.
func RoomName(display string) string {
	name := strings.Map(func(r rune) rune {
		if notARoomNameRune(r) {
			return -1
		}
		return r
	}, display)
	name = strings.TrimSpace(name)
	if len(name) > 12 {
		name = name[:12]
	}
	name = strings.TrimRight(name, " ")
	for _, run := range strings.FieldsFunc(strings.ToLower(name), isNotLetter) {
		if words.Offensive(run) {
			return ""
		}
	}
	return name
}

// isNotLetter is the split shape of a room name's letter runs.
func isNotLetter(r rune) bool { return r < 'a' || r > 'z' }

// notARoomNameRune reports whether r is not one of the characters a room
// name may carry.
func notARoomNameRune(r rune) bool {
	switch {
	case r >= 'A' && r <= 'Z':
		return false
	case r >= 'a' && r <= 'z':
		return false
	case r >= '0' && r <= '9':
		return false
	case r == ' ' || r == '_' || r == '.' || r == '-':
		return false
	default:
		return true
	}
}

// ValidMarks reports whether marks is one row for a word of length letters.
func ValidMarks(marks []game.Mark, length int) bool {
	if len(marks) != length {
		return false
	}
	for _, m := range marks {
		if m < game.Absent || m > game.Correct {
			return false
		}
	}
	return true
}

// ValidWord reports whether w is length lowercase letters a–z.
func ValidWord(w string, length int) bool {
	if len(w) != length {
		return false
	}
	for i := 0; i < len(w); i++ {
		if w[i] < 'a' || w[i] > 'z' {
			return false
		}
	}
	return true
}
