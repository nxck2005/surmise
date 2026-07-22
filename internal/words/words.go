// Package words provides the puzzle vocabulary, compiled into the binary.
//
// Two lists exist per word length: answers, the pool puzzles are drawn from,
// and guesses, the larger set accepted as input. Every answer is also a valid
// guess. See data/SOURCES.md for provenance and tools/genwords to regenerate.
package words

import (
	"embed"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
)

//go:embed data/answers4.txt data/answers5.txt data/answers6.txt
//go:embed data/guesses4.txt data/guesses5.txt data/guesses6.txt
var data embed.FS

// Lengths are the supported word lengths, in menu order. These are the game's
// difficulty modes, analogous to monkeytype's 15/30/60 second tests.
var Lengths = []int{4, 5, 6}

type lists struct {
	answers []string            // sorted, for indexed random selection
	guesses map[string]struct{} // membership only
}

var (
	once    sync.Once
	byLen   map[int]*lists
	loadErr error
)

// load parses the embedded lists on first use. A failure here means the binary
// was built with corrupt data, so callers surface it rather than retrying.
func load() {
	byLen = make(map[int]*lists, len(Lengths))
	for _, n := range Lengths {
		answers, err := readList(fmt.Sprintf("data/answers%d.txt", n))
		if err != nil {
			loadErr = err
			return
		}
		guessList, err := readList(fmt.Sprintf("data/guesses%d.txt", n))
		if err != nil {
			loadErr = err
			return
		}
		if len(answers) == 0 || len(guessList) == 0 {
			loadErr = fmt.Errorf("words: empty list for length %d", n)
			return
		}

		guesses := make(map[string]struct{}, len(guessList))
		for _, w := range guessList {
			guesses[w] = struct{}{}
		}
		byLen[n] = &lists{answers: answers, guesses: guesses}
	}
}

func readList(name string) ([]string, error) {
	b, err := data.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("words: read %s: %w", name, err)
	}
	return strings.Fields(string(b)), nil
}

func get(n int) (*lists, error) {
	once.Do(load)
	if loadErr != nil {
		return nil, loadErr
	}
	l, ok := byLen[n]
	if !ok {
		return nil, fmt.Errorf("words: unsupported length %d", n)
	}
	return l, nil
}

// SupportedLength reports whether n is a playable word length.
func SupportedLength(n int) bool {
	for _, l := range Lengths {
		if l == n {
			return true
		}
	}
	return false
}

// Random returns a random answer of the given length.
func Random(n int) (string, error) {
	l, err := get(n)
	if err != nil {
		return "", err
	}
	return l.answers[rand.IntN(len(l.answers))], nil
}

// IsValidGuess reports whether word is accepted as a guess. Input is
// normalized, so callers may pass any casing or surrounding space.
func IsValidGuess(n int, word string) bool {
	l, err := get(n)
	if err != nil {
		return false
	}
	_, ok := l.guesses[Normalize(word)]
	return ok
}

// Normalize puts a word into the canonical form used by the lists.
func Normalize(word string) string {
	return strings.ToLower(strings.TrimSpace(word))
}

// Counts returns the number of answers and guesses for a length, for stats and
// diagnostics.
func Counts(n int) (answers, guesses int, err error) {
	l, err := get(n)
	if err != nil {
		return 0, 0, err
	}
	return len(l.answers), len(l.guesses), nil
}
