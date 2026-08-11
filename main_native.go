//go:build !js

package main

import (
	"flag"
	"os"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/surmise/internal/brand"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/theme"
)

// loadConfig reads the flags, each defaulting to its environment variable.
func loadConfig() config {
	// A custom data directory keeps test or demo play out of the real profile.
	dataDir := flag.String(optData, "", "directory for saved puzzles (default: user config dir)")
	// -theme wins over the saved choice for one run, which is what makes
	// screenshotting and theme authoring bearable.
	themeName := flag.String(optTheme, os.Getenv(brand.Env("THEME")), "theme to start with (default: last used)")
	listThemes := flag.Bool(optThemes, false, "list available themes and exit")
	// -length likewise overrides the saved default mode for one run. A value
	// the game has no words for is reported by the UI on its error line rather
	// than refused here, so a typo costs a note, not a launch.
	length := flag.Int(optLength, envLength(), "word length to start with: 4, 5 or 6 (default: last used)")
	// -day plays another date's daily. Handy for looking at a board without
	// waiting for it, and, like the rest of this family, it never writes.
	day := flag.String(optDay, os.Getenv(brand.Env("DAY")), "date whose daily to play, YYYY-MM-DD (default: today, UTC)")
	// -splash picks the startup art for one run, or turns it off: "off",
	// "random", or a banner's name. Like the rest of this family it never writes,
	// and an unknown name is reported on the UI's error line rather than here.
	splash := flag.String(optSplash, os.Getenv(brand.Env("SPLASH")), "startup art: off, random, or a banner's name (default: last used)")
	// -motion sets how much the board animates for one run, which is what makes
	// a recording or a screenshot reproducible. $NO_MOTION is read separately,
	// by the UI, and only when nobody has chosen: a variable in a shell profile
	// is a preference, and a preference must not overrule a choice.
	motion := flag.String(optMotion, os.Getenv(brand.Env("MOTION")), "board feedback: off, restrained or pronounced (default: last used)")
	// -version answers "which build is this" without opening the app, where the
	// same information is on the about screen.
	showVersion := flag.Bool(optVersion, false, "print version information and exit")
	flag.Parse()

	return config{
		dataDir:     *dataDir,
		theme:       *themeName,
		day:         *day,
		splash:      *splash,
		motion:      *motion,
		length:      *length,
		listThemes:  *listThemes,
		showVersion: *showVersion,
	}
}

// envLength is the default for -length: $SURMISE_LENGTH, matching how
// $SURMISE_THEME defaults -theme. Unset or unreadable means zero — "use
// whatever was saved" — which is the same fallback an unsupported value gets.
func envLength() int {
	n, err := strconv.Atoi(os.Getenv(brand.Env("LENGTH")))
	if err != nil {
		return 0
	}
	return n
}

func run(cfg config) error {
	dataDir := cfg.dataDir
	if dataDir == "" {
		var err error
		if dataDir, err = store.DefaultDir(); err != nil {
			return err
		}
	}

	// Themes live beside the puzzles, so -data isolates the look as well as the
	// history. Seeding the directory means "write your own theme" starts from a
	// file that is already there.
	themeDir := theme.Dir(dataDir)
	if err := theme.EnsureDir(themeDir); err != nil {
		return err
	}
	lib := theme.Open(themeDir)

	if cfg.listThemes {
		printThemes(lib, themeDir)
		return nil
	}

	s, err := store.NewJSON(dataDir)
	if err != nil {
		return err
	}

	return start(s, lib, uiOptions(cfg, dataDir))
}

// attach has nothing to do here: a terminal reports a resize with SIGWINCH, and
// bubbletea listens for that itself.
func attach(*tea.Program) {}
