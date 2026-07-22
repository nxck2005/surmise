// Command genwords regenerates the embedded word lists in internal/words/data.
//
// It is run by hand, not as part of the build:
//
//	go run ./tools/genwords
//
// Guess lists (words accepted as input) come from ENABLE1, a public-domain
// Scrabble dictionary. Scrabble dictionaries omit proper nouns by construction,
// which keeps names like "aaron" out of the game without a blocklist.
//
// Answer lists (words chosen as solutions) are the intersection of ENABLE1 with
// a frequency-ranked list of common English, so solutions stay guessable while
// the accept list stays permissive.
package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	enableURL = "https://raw.githubusercontent.com/dolph/dictionary/master/enable1.txt"
	commonURL = "https://raw.githubusercontent.com/first20hours/google-10000-english/master/google-10000-english-usa-no-swears.txt"
	outDir    = "internal/words/data"
)

// lengths mirrors words.Lengths; the game modes are 4, 6 and 6 letters.
var lengths = []int{4, 5, 6}

var alphaOnly = regexp.MustCompile(`^[a-z]+$`)

func main() {
	log.SetFlags(0)

	enable, err := fetchWords(enableURL)
	if err != nil {
		log.Fatalf("fetch enable1: %v", err)
	}
	log.Printf("enable1: %d words", len(enable))

	common, err := fetchWords(commonURL)
	if err != nil {
		log.Fatalf("fetch common: %v", err)
	}
	log.Printf("common:  %d words", len(common))

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	for _, n := range lengths {
		guesses := filterLen(enable, n)

		// Answers must also be valid guesses, so intersect with the guess list
		// rather than filtering the common list independently.
		answers := make([]string, 0, len(guesses))
		for _, w := range guesses {
			if _, ok := common[w]; ok {
				answers = append(answers, w)
			}
		}

		if len(answers) == 0 {
			log.Fatalf("length %d produced no answers", n)
		}

		writeList(fmt.Sprintf("guesses%d.txt", n), guesses)
		writeList(fmt.Sprintf("answers%d.txt", n), answers)
		log.Printf("length %d: %d guesses, %d answers", n, len(guesses), len(answers))
	}

	writeSources()
}

// fetchWords downloads a newline-delimited word list and returns the set of
// lowercase, alphabetic-only entries.
func fetchWords(url string) (map[string]struct{}, error) {
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}

	words := make(map[string]struct{})
	sc := bufio.NewScanner(io.LimitReader(resp.Body, 64<<20))
	for sc.Scan() {
		w := strings.ToLower(strings.TrimSpace(sc.Text()))
		if alphaOnly.MatchString(w) {
			words[w] = struct{}{}
		}
	}
	return words, sc.Err()
}

func filterLen(set map[string]struct{}, n int) []string {
	out := make([]string, 0, 4096)
	for w := range set {
		if len(w) == n {
			out = append(out, w)
		}
	}
	slices.Sort(out)
	return out
}

// writeList writes one word per line, sorted, so the files stay diff-friendly
// and words.load can binary-search them without re-sorting.
func writeList(name string, words []string) {
	path := filepath.Join(outDir, name)
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, word := range words {
		fmt.Fprintln(w, word)
	}
	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}
}

func writeSources() {
	const doc = `# Word list sources

Regenerate with ` + "`go run ./tools/genwords`" + `. Do not edit these files by hand.

## guesses{4,5,6}.txt — accepted input

ENABLE1, filtered to the given length.

- Source: https://raw.githubusercontent.com/dolph/dictionary/master/enable1.txt
- License: public domain

ENABLE is a Scrabble dictionary, so it contains no proper nouns. That is why it
is used instead of a general word list such as dwyl/english-words, which admits
"aaron" and "adams" as five-letter words.

## answers{4,5,6}.txt — puzzle solutions

The intersection of the corresponding guess list with a frequency-ranked list of
common English, so solutions are words people actually know.

- Source: https://raw.githubusercontent.com/first20hours/google-10000-english/master/google-10000-english-usa-no-swears.txt
- Derived from the Google Web Trillion Word Corpus.

Every answer is by construction also a valid guess; ` + "`words`" + ` has a test asserting this.
`
	if err := os.WriteFile(filepath.Join(outDir, "SOURCES.md"), []byte(doc), 0o644); err != nil {
		log.Fatal(err)
	}
}
