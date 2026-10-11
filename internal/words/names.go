package words

import (
	"strings"
	"sync"
)

// Offensive reports whether a display name contains a blocked or profane
// word as one of its letter runs. It splits the lowercased name on every
// character that is not a–z and looks each run up whole, so "class" is not
// caught for containing a shorter word, and "Ass_hat" is caught for "ass" if
// that word is listed.
//
// It reads the same two files genwords subtracts from the lists:
// blocked.txt (slurs) and profanity.txt (vulgar words). Lines that are empty
// or start with # are skipped.
func Offensive(name string) bool {
	offensive()
	if len(offensiveWords) == 0 || name == "" {
		return false
	}
	for _, run := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool { return r < 'a' || r > 'z' }) {
		if _, ok := offensiveWords[run]; ok {
			return true
		}
	}
	return false
}

var (
	loadOffensiveOnce sync.Once
	offensiveWords    map[string]struct{}
)

// offensive loads blocked.txt and profanity.txt into one set, the first time
// a name needs checking. Both are embedded, so the read cannot fail and the
// words are one per line, none uppercase.
func offensive() {
	loadOffensiveOnce.Do(func() {
		words := map[string]struct{}{}
		for _, name := range []string{"data/blocked.txt", "data/profanity.txt"} {
			b, err := data.ReadFile(name)
			if err != nil {
				continue // the file is compiled in; a miss here is impossible
			}
			for _, line := range strings.Split(string(b), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				for _, word := range strings.Fields(line) {
					words[word] = struct{}{}
				}
			}
		}
		offensiveWords = words
	})
}
