//go:build js || wasip1

package tea

import (
	"io"

	"github.com/charmbracelet/x/term"
)

// A WebAssembly host has no tty to put into raw mode, and no process to
// suspend. Upstream ships tty_unix.go and tty_windows.go only, whose build tags
// name their platforms explicitly, so on js and wasip1 these symbols go
// undefined and the package does not compile. This file is additive: it defines
// them and edits nothing upstream, which is what keeps re-vendoring a copy.

func (p *Program) initInput() error {
	// p.ttyInput is set to a stub rather than left nil, and that is not
	// cosmetic. tea.go computes
	//
	//	mapNl := runtime.GOOS != "windows" && p.ttyInput == nil
	//
	// and a true mapNl leaves stale cells on screen whenever a frame shrinks —
	// a smaller screen drawn over a larger one keeps fragments of the old
	// border. The comment above that line says as much: the behaviour exists
	// for an emulated-pty workaround and "breaks many things especially when we
	// want the output to be compatible with terminals that are not necessarily
	// a TTY".
	//
	// A browser terminal is not a tty, but it behaves exactly like one in raw
	// mode: xterm.js defaults to convertEol false, so a bare newline moves down
	// without returning to column zero, which is what mapNl false assumes. The
	// stub says so.
	//
	// Nothing else reads ttyInput. restoreInput is guarded on
	// previousTtyInputState, which stays nil here because there is no terminal
	// state to save or restore.
	p.ttyInput = notATTY{}
	return nil
}

// notATTY satisfies term.File without being one. Its methods are never called:
// bubbletea reads and writes through p.input and p.output, and only ever
// compares this field against nil.
type notATTY struct{}

func (notATTY) Read([]byte) (int, error)    { return 0, io.EOF }
func (notATTY) Write(b []byte) (int, error) { return len(b), nil }
func (notATTY) Close() error                { return nil }
func (notATTY) Fd() uintptr                 { return 0 }

var _ term.File = notATTY{}

// suspendSupported is false because there is no SIGTSTP to raise.
const suspendSupported = false

func suspendProcess() {}
