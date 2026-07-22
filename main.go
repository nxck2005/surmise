// Command wortle is Wordle for the terminal.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/wortle/internal/store"
	"github.com/nxck2005/wortle/internal/ui"
)

func main() {
	// A custom data directory keeps test or demo play out of the real profile.
	dataDir := flag.String("data", "", "directory for saved puzzles (default: user config dir)")
	flag.Parse()

	if err := run(*dataDir); err != nil {
		fmt.Fprintln(os.Stderr, "wortle:", err)
		os.Exit(1)
	}
}

func run(dataDir string) error {
	if dataDir == "" {
		var err error
		if dataDir, err = store.DefaultDir(); err != nil {
			return err
		}
	}

	s, err := store.NewJSON(dataDir)
	if err != nil {
		return err
	}

	_, err = tea.NewProgram(ui.New(s)).Run()
	return err
}
