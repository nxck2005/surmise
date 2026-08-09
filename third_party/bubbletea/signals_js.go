//go:build js || wasip1

package tea

// There is no SIGWINCH in a browser. The host learns about a resize from its
// own event (xterm.js's onResize) and sends a WindowSizeMsg through the
// Program, so this watcher has nothing to watch.
//
// Closing done immediately is what the caller expects: handleResize waits on
// that channel during shutdown, and a channel that is never closed would hang
// the program on quit.

func (p *Program) listenForResize(done chan struct{}) { close(done) }
