// Command wortle is Wordle for the terminal.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/wortle/internal/store"
	"github.com/nxck2005/wortle/internal/theme"
	"github.com/nxck2005/wortle/internal/ui"
)

func main() {
	// A custom data directory keeps test or demo play out of the real profile.
	dataDir := flag.String("data", "", "directory for saved puzzles (default: user config dir)")
	// -theme wins over the saved choice for one run, which is what makes
	// screenshotting and theme authoring bearable.
	themeName := flag.String("theme", os.Getenv("WORTLE_THEME"), "theme to start with (default: last used)")
	listThemes := flag.Bool("themes", false, "list available themes and exit")
	flag.Parse()

	if err := run(*dataDir, *themeName, *listThemes); err != nil {
		fmt.Fprintln(os.Stderr, "wortle:", err)
		os.Exit(1)
	}
}

func run(dataDir, themeName string, listThemes bool) error {
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

	_, err = tea.NewProgram(ui.New(s, lib, themeName)).Run()
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
