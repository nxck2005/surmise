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
//
// Both lists are then filtered through internal/words/data/blocked.txt, a
// hand-maintained list of slurs. ENABLE is a Scrabble dictionary and keeps a
// number of them, so this file is the one part of the pipeline that is edited
// by hand; it is length-agnostic, so it keeps covering any word length added
// later.
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
	"strconv"
	"strings"
	"time"
)

// Both sources are pinned to a commit rather than a branch. A word list that
// moves under us is a silent change to every unplayed daily, so a regeneration
// has to be a deliberate bump of these constants and nothing else.
const (
	enableRev = "c65f04b0b5b27a981f437b940cf62fe71320d5ec"
	enableURL = "https://raw.githubusercontent.com/dolph/dictionary/" + enableRev + "/enable1.txt"

	// The frequency list is MIT-licensed, which the previous source
	// (first20hours/google-10000-english) was not: it derives from an LDC
	// corpus and asks that commercial use be licensed separately, which does
	// not sit inside this project's MIT grant. See THIRD_PARTY_NOTICES.md.
	commonRev = "525f9b560de45753a5ea01069454e72e9aa541c6"
	commonURL = "https://raw.githubusercontent.com/hermitdave/FrequencyWords/" + commonRev + "/content/2018/en/en_50k.txt"

	// commonRank is how far down the frequency list an answer may come from.
	// The file is ordered by descending count, so this is a "how obscure may a
	// solution be" dial and nothing else.
	commonRank = 10000

	outDir = "internal/words/data"

	// blockedFile and profaneFile live in outDir but, unlike the lists beside
	// them, are inputs: they are written by hand and read on every run.
	blockedFile = "blocked.txt"
	profaneFile = "profanity.txt"
)

// lengths mirrors words.Lengths; the game modes are 4, 6 and 6 letters.
var lengths = []int{4, 5, 6}

var alphaOnly = regexp.MustCompile(`^[a-z]+$`)

func main() {
	log.SetFlags(0)

	// Read the blocklist first: a missing or unreadable file must stop the run
	// rather than quietly regenerate the lists without it.
	blocked, err := readWordFile(blockedFile)
	if err != nil {
		log.Fatalf("read blocklist: %v", err)
	}
	log.Printf("blocked: %d words", len(blocked))

	// Profanity is a separate input from the blocklist because it gets separate
	// treatment: these stay typeable, they just never become the solution.
	profane, err := readWordFile(profaneFile)
	if err != nil {
		log.Fatalf("read profanity list: %v", err)
	}
	log.Printf("profane: %d words", len(profane))

	enable, err := fetchWords(enableURL)
	if err != nil {
		log.Fatalf("fetch enable1: %v", err)
	}
	log.Printf("enable1: %d words", len(enable))

	common, err := fetchRanked(commonURL, commonRank)
	if err != nil {
		log.Fatalf("fetch common: %v", err)
	}
	log.Printf("common:  %d words", len(common))

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	for _, n := range lengths {
		// Filtering the guess list is enough: answers are a subset of it.
		guesses := filterLen(enable, n, blocked)

		// Answers must also be valid guesses, so intersect with the guess list
		// rather than filtering the common list independently. Profanity is
		// subtracted only here: a player who types one should still be told it
		// is a real word, it just must never be the word of the day.
		answers := make([]string, 0, len(guesses))
		for _, w := range guesses {
			if _, ok := common[w]; !ok {
				continue
			}
			if _, bad := profane[w]; bad {
				continue
			}
			answers = append(answers, w)
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

// fetch opens a source list, failing loudly on anything but a 200 so a moved
// URL cannot be mistaken for a short list.
func fetch(url string) (io.ReadCloser, error) {
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return resp.Body, nil
}

// fetchWords downloads a newline-delimited word list and returns the set of
// lowercase, alphabetic-only entries.
func fetchWords(url string) (map[string]struct{}, error) {
	body, err := fetch(url)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	words := make(map[string]struct{})
	sc := bufio.NewScanner(io.LimitReader(body, 64<<20))
	for sc.Scan() {
		w := strings.ToLower(strings.TrimSpace(sc.Text()))
		if alphaOnly.MatchString(w) {
			words[w] = struct{}{}
		}
	}
	return words, sc.Err()
}

// fetchRanked downloads a frequency-ordered "word count" list and returns the
// set of the first limit usable words. Unlike fetchWords it cannot match the
// whole line — every line here carries a trailing count — so it takes the first
// field, and it counts only what it keeps, so a run of unusable lines cannot
// quietly shorten the result.
func fetchRanked(url string, limit int) (map[string]struct{}, error) {
	body, err := fetch(url)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	words := make(map[string]struct{}, limit)
	sc := bufio.NewScanner(io.LimitReader(body, 64<<20))
	for sc.Scan() && len(words) < limit {
		field, _, _ := strings.Cut(strings.TrimSpace(sc.Text()), " ")
		w := strings.ToLower(field)
		if alphaOnly.MatchString(w) {
			words[w] = struct{}{}
		}
	}
	return words, sc.Err()
}

// readWordFile returns one of the hand-maintained sets in outDir. An empty file
// is an error rather than an empty set: silently regenerating without a
// blocklist is exactly the failure these files exist to prevent.
func readWordFile(name string) (map[string]struct{}, error) {
	b, err := os.ReadFile(filepath.Join(outDir, name))
	if err != nil {
		return nil, err
	}
	words := make(map[string]struct{})
	for line := range strings.Lines(string(b)) {
		// These files carry section comments, so a # runs to end of line.
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		for _, w := range strings.Fields(line) {
			words[strings.ToLower(w)] = struct{}{}
		}
	}
	if len(words) == 0 {
		return nil, fmt.Errorf("%s is empty", name)
	}
	return words, nil
}

func filterLen(set map[string]struct{}, n int, blocked map[string]struct{}) []string {
	out := make([]string, 0, 4096)
	for w := range set {
		if _, bad := blocked[w]; bad {
			continue
		}
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
	doc := `# Word list sources

Regenerate with ` + "`go run ./tools/genwords`" + `. Do not edit the word lists by
hand — ` + "`blocked.txt`" + ` and ` + "`profanity.txt`" + ` below are the two files here that are
hand-maintained.

Both sources are pinned to a commit, not a branch: an answer list that shifts
under us changes the word of the day for every date not yet played, so a
regeneration is a deliberate act. Licences are reproduced in
` + "`THIRD_PARTY_NOTICES.md`" + ` at the repo root.

## guesses{4,5,6}.txt — accepted input

ENABLE1, filtered to the given length.

- Source: https://raw.githubusercontent.com/dolph/dictionary/` + enableRev[:12] + `/enable1.txt
- Revision: ` + enableRev + `
- License: public domain

ENABLE is a Scrabble dictionary, so it contains no proper nouns. That is why it
is used instead of a general word list such as dwyl/english-words, which admits
"aaron" and "adams" as five-letter words.

## answers{4,5,6}.txt — puzzle solutions

The intersection of the corresponding guess list with the top ` + strconv.Itoa(commonRank) + ` entries of a
frequency-ranked list of common English, minus ` + "`profanity.txt`" + `, so solutions are
words people actually know and would not mind seeing.

- Source: https://raw.githubusercontent.com/hermitdave/FrequencyWords/` + commonRev[:12] + `/content/2018/en/en_50k.txt
- Revision: ` + commonRev + `
- License: MIT (Hermit Dave)
- Derived from the OpenSubtitles 2018 corpus.

This replaced first20hours/google-10000-english, which derives from an LDC
corpus and asks that commercial use be licensed separately — a restriction this
project's MIT licence cannot pass on. The vocabulary is conversational rather
than literary as a result, which is what ` + "`profanity.txt`" + ` exists to temper.

Every answer is by construction also a valid guess; ` + "`words`" + ` has a test asserting this.

## blocked.txt — words kept out of both

A hand-maintained list of slurs, edited by hand and read (never written) by
genwords, which drops every entry from the guess lists and therefore from the
answer lists too. ENABLE is a Scrabble dictionary and keeps a number of slurs,
so the source lists alone are not enough.

The list is deliberately length-agnostic and includes inflections and spellings
that no current mode can reach, so it keeps working if a word length is added.
` + "`words`" + ` has a test asserting no shipped list contains a blocked word.

## profanity.txt — words kept out of answers only

Vulgar but unremarkable English: words a player may reasonably type and should
be told are real, but which should never be the word of the day. Unlike
` + "`blocked.txt`" + ` these stay in the guess lists, so the "every answer is a valid
guess" invariant is untouched — the subtraction only ever runs one way.

The distinction is the point. A slur is not a word this game accepts; a swear is
a word it accepts but does not choose.
`
	if err := os.WriteFile(filepath.Join(outDir, "SOURCES.md"), []byte(doc), 0o644); err != nil {
		log.Fatal(err)
	}
}
