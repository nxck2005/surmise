//go:build js || wasip1

package tea

// A WebAssembly host has no tty to put into raw mode, and no process to
// suspend. Upstream ships tty_unix.go and tty_windows.go only, whose build tags
// name their platforms explicitly, so on js and wasip1 these three symbols go
// undefined and the package does not compile. This file is additive: it defines
// them and edits nothing upstream, which is what keeps re-vendoring a copy.
//
// Leaving p.ttyInput and p.ttyOutput nil is a state the rest of the package
// already tolerates — checkResize returns early on a nil output, and
// restoreInput does nothing without a saved state.

func (p *Program) initInput() error { return nil }

// suspendSupported is false because there is no SIGTSTP to raise.
const suspendSupported = false

func suspendProcess() {}
