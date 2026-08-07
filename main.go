// Command surmise is a word-guessing game for the terminal.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/surmise/internal/brand"
	"github.com/nxck2005/surmise/internal/build"
	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/theme"
	"github.com/nxck2005/surmise/internal/ui"
)

func main() {
	// A custom data directory keeps test or demo play out of the real profile.
	dataDir := flag.String("data", "", "directory for saved puzzles (default: user config dir)")
	// -theme wins over the saved choice for one run, which is what makes
	// screenshotting and theme authoring bearable.
	themeName := flag.String("theme", os.Getenv(brand.Env("THEME")), "theme to start with (default: last used)")
	listThemes := flag.Bool("themes", false, "list available themes and exit")
	// -length likewise overrides the saved default mode for one run. A value
	// the game has no words for is reported by the UI on its error line rather
	// than refused here, so a typo costs a note, not a launch.
	length := flag.Int("length", envLength(), "word length to start with: 4, 5 or 6 (default: last used)")
	// -day plays another date's daily. Handy for looking at a board without
	// waiting for it, and, like the rest of this family, it never writes.
	day := flag.String("day", os.Getenv(brand.Env("DAY")), "date whose daily to play, YYYY-MM-DD (default: today, UTC)")
	// -version answers "which build is this" without opening the app, where the
	// same information is on the about screen.
	showVersion := flag.Bool("version", false, "print version information and exit")
	flag.Parse()

	// Unlike -themes, this needs no store and no theme directory, so it does not
	// go through run: printing a version must never create anything on disk.
	if *showVersion {
		fmt.Println(build.Get())
		return
	}

	if err := run(*dataDir, *themeName, *day, *length, *listThemes); err != nil {
		fmt.Fprintln(os.Stderr, brand.Name+":", err)
		os.Exit(1)
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

func run(dataDir, themeName, day string, length int, listThemes bool) error {
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

	if listThemes {
		printThemes(lib, themeDir)
		return nil
	}

	s, err := store.NewJSON(dataDir)
	if err != nil {
		return err
	}

	// The daily's seeds come from here, which is the one place a future remote
	// source would be chosen; ui.Options carries it in so nothing below has to
	// know which one it got.
	opts := ui.Options{
		Theme:      themeName,
		Length:     length,
		Day:        day,
		DailySeeds: daily.Local(),
		DataDir:    dataDir,
	}
	_, err = tea.NewProgram(ui.New(s, lib, opts)).Run()
	return err
}

// printThemes backs the -themes flag: the fastest way to find out what a theme
// file is called and whether it parsed.
func printThemes(lib *theme.Library, dir string) {
	fmt.Printf("themes directory: %s\n\n", dir)
	for _, e := range lib.Entries() {
		fmt.Printf("  %-24s %s\n", e.Name, e.Source)
		if e.Err != nil {
			fmt.Printf("      error: %v\n", e.Err)
		}
		for _, w := range e.Warnings {
			fmt.Printf("      %s\n", w)
		}
	}
}
