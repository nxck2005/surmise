package backup

import (
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/game"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/theme"
)

// FuzzRead holds Read to the contract every parser in this app carries: refuse
// anything it cannot fully check, and never panic doing it. The three
// per-record parsers have their own targets (store's FuzzDecodeRecord, theme's
// FuzzParse and challenge's FuzzParse); this one covers the wrapper's own
// gates — the header, the count caps that now fire while the arrays decode,
// the theme caps and the settings rules — which is what a whole archive
// arrives through.
func FuzzRead(f *testing.F) {
	s := store.NewKV(store.NewMemoryKV())
	g, err := game.New(5)
	if err != nil {
		f.Fatalf("game.New: %v", err)
	}
	if err := g.Guess(g.Answer); err != nil {
		f.Fatalf("Guess: %v", err)
	}
	if err := s.Save(g); err != nil {
		f.Fatalf("Save: %v", err)
	}
	valid, err := Build(s, store.Settings{Theme: "dracula"},
		[]theme.File{{Name: "mine.toml", Body: "# mine\n"}}, "fuzz", time.Unix(0, 0).UTC())
	if err != nil {
		f.Fatalf("Build: %v", err)
	}

	f.Add(valid)
	f.Add(valid[:len(valid)/2]) // truncated mid-file
	f.Add([]byte(`{"format":"surmise.backup","version":1,"puzzles":[]}`))
	f.Add([]byte(`{"format":"surmise.backup","version":1,"puzzles":[null,null,null]}`))
	f.Add([]byte(`{"format":"surmise.backup","version":1,"themes":[null]}`))
	f.Add([]byte(`{"format":"surmise.backup","version":99,"puzzles":[]}`))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, b []byte) {
		a, games, err := Read(b)
		if err != nil {
			return // a refusal is the common, correct answer
		}
		if len(a.Puzzles) > maxPuzzles || len(a.Themes) > maxThemes {
			t.Fatalf("Read returned %d records and %d themes, past the caps", len(a.Puzzles), len(a.Themes))
		}
		for _, g := range games {
			if !game.ValidID(g.ID) {
				t.Fatalf("Read returned a puzzle with an invalid id %q", g.ID)
			}
		}
	})
}
