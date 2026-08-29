package challenge

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/words"
)

func TestGoldenVectors(t *testing.T) {
	tests := []struct {
		length int
		seed   []byte
		code   string
		answer string
		id     string
	}{
		{4, []byte{0, 0, 0, 0, 0, 0, 0}, "4400-0000-0000-00QW", "able", "35aadf27-4645-885e-b17e-ee7d0ec06856"},
		{5, []byte{1, 2, 3, 4, 5, 6, 7}, "4500-820C-20A1-G73J", "faint", "1ec76f89-8da7-826a-9732-908286fd820c"},
		{6, []byte{0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x99}, "461Z-ZEVQ-6BQA-MSAS", "campus", "ebd89ebf-e3f8-84c3-9d5a-01364116844a"},
	}
	for _, tt := range tests {
		c, err := generate(tt.length, bytes.NewReader(tt.seed))
		if err != nil {
			t.Fatal(err)
		}
		// Empty expectations are filled after the format's first implementation;
		// printing all three in one failure makes freezing the vector deliberate.
		if tt.code == "" {
			t.Fatalf("freeze vector: length %d code=%q answer=%q id=%q",
				tt.length, c.String(), mustAnswer(t, c), c.ID())
		}
		if c.String() != tt.code || mustAnswer(t, c) != tt.answer || c.ID() != tt.id {
			t.Errorf("length %d = %q, %q, %q; want %q, %q, %q",
				tt.length, c.String(), mustAnswer(t, c), c.ID(), tt.code, tt.answer, tt.id)
		}
	}
}

func mustAnswer(t *testing.T, c Code) string {
	t.Helper()
	a, err := c.Answer()
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestRoundTripAndNormalization(t *testing.T) {
	c, err := generate(5, bytes.NewReader([]byte{1, 2, 3, 4, 5, 6, 7}))
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{
		c.String(),
		strings.ToLower(c.String()),
		strings.ReplaceAll(c.String(), "-", ""),
		strings.ReplaceAll(c.String(), "-", " "),
	} {
		got, err := Parse(text)
		if err != nil {
			t.Fatalf("Parse(%q): %v", text, err)
		}
		if got.String() != c.String() || got.ID() != c.ID() {
			t.Errorf("Parse(%q) = %q/%q, want %q/%q", text, got.String(), got.ID(), c.String(), c.ID())
		}
	}
}

func TestSameAnswerCanHaveDistinctChallengeIdentity(t *testing.T) {
	first := Code{listVersion: 1, length: 5, seed: 0}
	count, err := words.AnswerCountAt(1, 5)
	if err != nil {
		t.Fatal(err)
	}
	second := Code{listVersion: 1, length: 5, seed: uint64(count)}
	if mustAnswer(t, first) != mustAnswer(t, second) {
		t.Fatal("test seeds did not select the same answer")
	}
	if first.String() == second.String() || first.ID() == second.ID() {
		t.Error("two seeded challenges for the same answer collapsed into one identity")
	}
}

func TestParseRejectsBadInput(t *testing.T) {
	c, err := generate(5, bytes.NewReader(make([]byte, 7)))
	if err != nil {
		t.Fatal(err)
	}
	badChecksum := []byte(strings.ReplaceAll(c.String(), "-", ""))
	badChecksum[len(badChecksum)-1] = '0'
	if badChecksum[len(badChecksum)-1] == c.String()[len(c.String())-1] {
		badChecksum[len(badChecksum)-1] = '1'
	}
	for _, text := range []string{
		"", "short", strings.Repeat("A", 65),
		"IIII-IIII-IIII-IIII", "OOOO-OOOO-OOOO-OOOO",
		string(badChecksum),
	} {
		if _, err := Parse(text); err == nil {
			t.Errorf("Parse(%q) succeeded", text)
		}
	}
}

func TestParseRejectsUnknownProtocolFields(t *testing.T) {
	c, err := generate(5, bytes.NewReader(make([]byte, 7)))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name          string
		offset, width int
		value         uint64
	}{
		{"format", 0, 3, 2},
		{"answer version", 3, 5, 2},
		{"length", 8, 2, 3},
		{"flags", 10, 4, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packed := c.payload()
			putBits(packed[:], tt.offset, tt.width, tt.value)
			putBits(packed[:], payloadBits, checksumBits, uint64(checksum(packed)))
			if _, err := Parse(renderPacked(packed)); err == nil {
				t.Error("Parse accepted an unknown protocol field")
			}
		})
	}
}

func renderPacked(packed [10]byte) string {
	var out [encodedLen]byte
	for i := range out {
		out[i] = alphabet[getBits(packed[:], i*5, 5)]
	}
	return string(out[:])
}

func TestGenerateRejectsUnsupportedLengthAndReaderFailure(t *testing.T) {
	if _, err := generate(7, bytes.NewReader(make([]byte, 7))); err == nil {
		t.Error("generate accepted length 7")
	}
	if _, err := generate(5, bytes.NewReader(nil)); err == nil {
		t.Error("generate accepted an empty random source")
	}
}

func TestNewGameCarriesChallenge(t *testing.T) {
	c, err := generate(5, bytes.NewReader([]byte{1, 2, 3, 4, 5, 6, 7}))
	if err != nil {
		t.Fatal(err)
	}
	g, err := c.NewGame()
	if err != nil {
		t.Fatal(err)
	}
	if g.ID != c.ID() || g.Answer != mustAnswer(t, c) || g.Challenge == nil || g.Challenge.Code != c.String() {
		t.Errorf("game = %+v, want challenge %q", g, c.String())
	}
}

func TestValidateGameRejectsMismatchedMetadata(t *testing.T) {
	c, err := Parse("4500-820C-20A1-G73J")
	if err != nil {
		t.Fatal(err)
	}
	valid, err := c.NewGame()
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateGame(valid); err != nil {
		t.Fatalf("ValidateGame(valid): %v", err)
	}

	for _, mutate := range []func(*game.Game){
		func(g *game.Game) { g.Challenge.Code = strings.ToLower(g.Challenge.Code) },
		func(g *game.Game) { g.ID = "another-id" },
		func(g *game.Game) { g.Length = 4 },
		func(g *game.Game) { g.Answer = "about" },
	} {
		g, err := c.NewGame()
		if err != nil {
			t.Fatal(err)
		}
		mutate(g)
		if err := ValidateGame(g); err == nil {
			t.Errorf("ValidateGame accepted mismatched game: %+v", g)
		}
	}
}

func TestChallengeProtocolDoesNotDependOnBrand(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "/internal/brand") {
			t.Errorf("%s imports brand; protocol tags must survive a product rename", entry.Name())
		}
	}
}
