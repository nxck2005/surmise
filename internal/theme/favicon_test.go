package theme

import (
	"fmt"
	"image/color"
	"os"
	"strings"
	"testing"
)

// The site's favicon is one tile in the default theme: a letter in the
// background colour on the correct colour, which is how the board draws it. The
// SVG is a hand-written copy of those two values, so a change to Default has to
// change it too.
func TestFaviconUsesTheDefaultTileColours(t *testing.T) {
	svg, err := os.ReadFile("../../web/favicon.svg")
	if err != nil {
		t.Fatal(err)
	}
	d := Default()
	for _, key := range []string{Correct, Bg} {
		want := hexOf(d.Color(key))
		if !strings.Contains(strings.ToLower(string(svg)), want) {
			t.Errorf("web/favicon.svg does not use the default %s colour %s", key, want)
		}
	}
}

func hexOf(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}
