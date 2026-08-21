package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nxck2005/surmise/internal/game"
)

// FuzzDecodeRecord feeds arbitrary bytes to the codec that reads saves — files
// a crash may have truncated or a hand may have edited. Its contract is to
// refuse, never to panic: every input ends in either a valid game or an error,
// and which of the two is not the fuzz test's business.
func FuzzDecodeRecord(f *testing.F) {
	g, err := game.NewFrom("fuzz-seed", "crane", 5)
	if err != nil {
		f.Fatal(err)
	}
	full, err := EncodeRecord(g)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(full)
	f.Add(full[:len(full)/2])
	f.Add([]byte(`{"schema":1}`))
	f.Add([]byte(`{"schema":999,"id":"x"}`))
	f.Add([]byte(`{"id":"x","length":5,"answer":"crane","status":"won"}`))
	f.Add([]byte("\x00\x01\x02"))
	for _, name := range []string{"legacy-puzzle.json", "legacy-settings.json"} {
		b, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}

	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = DecodeRecord("fuzz", b)
	})
}
