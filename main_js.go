//go:build js && wasm

package main

import (
	"fmt"
	"strconv"

	"syscall/js"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/nxck2005/surmise/internal/brand"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/theme"
	"github.com/nxck2005/surmise/internal/web"
)

// browserTerm is the terminal run wired up, held for attach. There is exactly
// one page and one program, so a package-level value is honest here rather than
// a shortcut.
var browserTerm *web.Terminal

// loadConfig reads the same options the flags carry, from the page's query
// string: ?theme=dracula&length=6&day=2026-08-06&splash=off&motion=off
//
// There is no -data (a browser has no directories), and no -themes, -version or
// -playtime: the first has nothing to point at, and the others print to a stdout
// that nobody can see — playtime is on the profile screen here, which is where a
// browser can read it. Their zero values mean "not asked for", which is exactly
// what config wants.
//
// -export and -import are absent for the first reason (there is no path to name)
// but, unlike the rest, they are a gap and not a decision: backing up matters
// most here, where clearing site data destroys everything. internal/backup does
// no file I/O precisely so that a page-side download and file picker can call
// Build and Apply unchanged. See docs/WEB.md.
func loadConfig() config {
	get := func(string) string { return "" }

	// Nothing here may assume a browser. syscall/js panics on a Get against an
	// undefined value, and a host without a location — Node, which is how the
	// binary is smoke-tested — has to reach the ordinary "no options chosen"
	// path rather than a stack trace.
	loc := js.Global().Get("location")
	usp := js.Global().Get("URLSearchParams")
	if loc.Truthy() && usp.Truthy() {
		params := usp.New(loc.Get("search"))
		get = func(name string) string {
			v := params.Call("get", name)
			if !v.Truthy() {
				return ""
			}
			return v.String()
		}
	}

	cfg := config{
		theme:  get(optTheme),
		day:    get(optDay),
		splash: get(optSplash),
		motion: get(optMotion),
	}
	// An unreadable length is zero, "use whatever was saved" — the same
	// fallback $SURMISE_LENGTH gets natively.
	if n, err := strconv.Atoi(get(optLength)); err == nil {
		cfg.length = n
	}
	return cfg
}

func run(cfg config) error {
	term, err := web.Open()
	if err != nil {
		return err
	}
	browserTerm = term

	// A refused localStorage is not fatal: the game runs and forgets. The
	// message reaches the page's console so it is diagnosable.
	kv, storageErr := web.Storage()
	if storageErr != nil {
		fmt.Println(storageErr)
	}

	// theme.Bundled is the no-filesystem library. It also disables the theme
	// directory watcher by itself — watchThemes returns nothing for a library
	// with no directory — so there is no polling to turn off here.
	lib := theme.Bundled()

	cols, rows := term.Size()

	err = start(store.NewKV(kv), lib, uiOptions(cfg, browserDataDir),
		// Mandatory, not optional: without an input bubbletea falls through to
		// os.Stdin and OpenTTY, and there is neither.
		tea.WithInput(term.Reader()),
		tea.WithOutput(term.Writer()),
		// The output is not a tty, so colour detection would find no colour and
		// every theme would collapse to monochrome. Setting a profile skips the
		// detection entirely.
		tea.WithColorProfile(colorprofile.TrueColor),
		// os.Environ() is empty here, and both the input parser and the
		// renderer read TERM.
		tea.WithEnvironment([]string{"TERM=xterm-256color", "COLORTERM=truecolor"}),
		// Without this the first frame is laid out at zero by zero, and the app
		// reads an unmeasured size as unbounded.
		tea.WithWindowSize(cols, rows),
		// There are no signals in a browser.
		tea.WithoutSignalHandler(),
	)

	// Nothing above this can reach main's error path: a returned run would tear
	// the instance down. Report to the console, which is the only place a
	// browser has for this, then hand the page its overlay.
	if err != nil {
		fmt.Println(brandError(err))
	}
	term.Done()

	// A Go main that returns tears the instance down, and the page would be
	// left with a dead terminal. Done has revealed the overlay; block here so
	// the runtime stays alive behind it.
	select {}
}

func brandError(err error) string { return brand.Name + ": " + err.Error() }

// browserDataDir is what the about screen shows where a path would be. DataDir
// is display-only, so this is a description rather than a location.
const browserDataDir = "browser storage (localStorage)"

func attach(p *tea.Program) {
	if browserTerm != nil {
		browserTerm.Attach(p.Send)
	}
}
