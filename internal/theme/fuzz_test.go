package theme

import (
	"strings"
	"testing"
)

// FuzzParse feeds arbitrary bytes to the reader that takes a file a player
// wrote or was sent. Its contract is forgiving-by-warning, never a panic: any
// input either parses over the default theme or produces warnings naming lines,
// and Parse always returns something renderable. Errors would be a bug here —
// the caller in library.go is the only place allowed to fail on an unreadable
// file, and it never hands one to Parse.
func FuzzParse(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("name = \"fuzz\"\n"))
	f.Add([]byte("[palette]\nbg = \"#1a1b26\"\nfg = \"#c0caf5\"\n"))
	f.Add([]byte("[style.tile]\nbg = \"not a colour\"\n"))
	f.Add([]byte("[metric]\ntile_width = \"x\"\n"))
	f.Add([]byte(strings.Repeat("[section]\n", 1000)))
	// The bundled themes are the real corpus: every shape the schema actually
	// sees, from full palettes to ANSI-number-only terminal.toml.
	seeds, err := bundled.ReadDir("themes")
	if err != nil {
		f.Fatal(err)
	}
	for _, s := range seeds {
		b, err := bundled.ReadFile("themes/" + s.Name())
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		th, warns := Parse("fuzz", data)
		if th == nil {
			t.Fatal("Parse returned no theme")
		}
		for _, w := range warns {
			if w.Line < 0 || w.Msg == "" {
				t.Fatalf("malformed warning %+v", w)
			}
		}
	})
}
