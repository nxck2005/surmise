// Command surmise is a word-guessing game for the terminal, and — built for
// WebAssembly — for a browser.
//
// What the two builds share is here. What only one of them can do is in
// main_native.go and main_js.go: a browser has no flags, no environment and no
// config directory, and a terminal has no query string. The split is by what
// the platform can do, not by taste, and the option *names* are declared once
// so a flag and a URL parameter cannot drift apart.
package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/surmise/internal/brand"
	"github.com/nxck2005/surmise/internal/build"
	"github.com/nxck2005/surmise/internal/daily"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/theme"
	"github.com/nxck2005/surmise/internal/ui"
)

// The option names, used by the flags natively and by the URL query string in a
// browser. One declaration, so `-theme` and `?theme=` cannot diverge.
const (
	optData     = "data"
	optTheme    = "theme"
	optThemes   = "themes"
	optLength   = "length"
	optDay      = "day"
	optSplash   = "splash"
	optMotion   = "motion"
	optVersion  = "version"
	optPlaytime = "playtime"
)

// config is what the player asked for, however they asked. Every zero value
// means "nothing chosen", matching store.Settings, so an option that the
// platform cannot express simply stays zero.
type config struct {
	dataDir      string
	theme        string
	day          string
	splash       string
	motion       string
	length       int
	listThemes   bool
	showVersion  bool
	showPlaytime bool
}

func main() {
	cfg := loadConfig()

	// This needs no store and no theme directory, so it does not go through run:
	// printing a version must never create anything on disk.
	if cfg.showVersion {
		fmt.Println(build.Get())
		return
	}

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, brand.Name+":", err)
		os.Exit(1)
	}
}

// uiOptions is the one place ui.Options is built, so both platforms pass the
// same shape and a new field is added once.
//
// dataDir is display-only — the about screen shows it and nothing reads through
// it — which is why the browser can pass a description rather than a path.
func uiOptions(cfg config, dataDir string) ui.Options {
	// The daily's seeds come from here, which is the one place a future remote
	// source would be chosen; ui.Options carries it in so nothing below has to
	// know which one it got.
	return ui.Options{
		Theme:      cfg.theme,
		Length:     cfg.length,
		Day:        cfg.day,
		Splash:     cfg.splash,
		Motion:     cfg.motion,
		DailySeeds: daily.Local(),
		DataDir:    dataDir,
	}
}

// start runs the program. Every tea.NewProgram in this command goes through
// here: the native build passes no options and takes bubbletea's terminal
// defaults, and the browser build passes the several it needs to talk to
// xterm.js instead of a tty.
// attach is the platform's chance to hold the Program, which the browser needs
// so a resize can be sent in. It is a no-op natively.
func start(s store.Store, lib *theme.Library, opts ui.Options, popts ...tea.ProgramOption) error {
	p := tea.NewProgram(ui.New(s, lib, opts), popts...)
	attach(p)
	_, err := p.Run()
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
