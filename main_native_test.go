//go:build !js

package main

import (
	"errors"
	"fmt"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/theme"
)

// The flags that only answer a question must not create the install they
// were asked about.
func TestReadOnlyFlagsCreateNothing(t *testing.T) {
	for name, cfg := range map[string]config{
		"-playtime": {showPlaytime: true},
		"-themes":   {listThemes: true},
	} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "never-opened")
			cfg.dataDir = dir
			if err := run(cfg); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("%s created %s (stat: %v)", name, dir, err)
			}
		})
	}
}

// An opened install still answers -playtime.
func TestPlaytimeReadsAnOpenedInstall(t *testing.T) {
	dir := t.TempDir()
	if _, err := store.NewJSON(dir); err != nil {
		t.Fatal(err)
	}
	if err := run(config{dataDir: dir, showPlaytime: true}); err != nil {
		t.Fatal(err)
	}
}

// The page's first paint uses the default theme's colours, so it does not
// flash another palette before the game sends its own.
func TestThePageStartsInTheDefaultColours(t *testing.T) {
	css, err := os.ReadFile(filepath.Join("web", "style.css"))
	if err != nil {
		t.Fatal(err)
	}
	hex := func(c color.Color) string {
		r, g, b, _ := c.RGBA()
		return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
	}
	d := theme.Default()
	for name, want := range map[string]string{
		"--surmise-bg": hex(d.Color(theme.Bg)),
		"--surmise-fg": hex(d.Color(theme.Text)),
	} {
		if line := name + ": " + want + ";"; !strings.Contains(string(css), line) {
			t.Errorf("style.css does not set %q", line)
		}
	}
}
