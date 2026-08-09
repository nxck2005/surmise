//go:build js && wasm

// Package web bridges a bubbletea Program to an xterm.js terminal in a page.
//
// The design rests on one fact: xterm.js already speaks the terminal protocol.
// It turns DOM key events into escape sequences and, once bubbletea asks for
// mouse tracking, emits SGR mouse sequences — and bubbletea already parses
// both. So this is a byte pipe, not a translation layer, and key and mouse
// parity with the terminal build comes for free rather than being maintained.
//
// # The rule that will break everything if ignored
//
// A js.Func callback must never block. Go on WebAssembly runs on the single
// JavaScript thread: while a Go callback is running, no other JS event can
// fire. A callback that blocks waiting for something another event would
// deliver deadlocks the whole runtime, and the symptom is a page that draws one
// frame and then freezes with nothing in the console.
//
// So every callback here does the same three things and returns: take the lock,
// append, signal without blocking. Nothing calls Program.Send from inside a
// callback either — Send blocks until the event loop reads it.
package web

import (
	"errors"
	"io"
	"sync"

	"syscall/js"

	tea "charm.land/bubbletea/v2"
)

// Terminal is the xterm.js instance the page created for us.
type Terminal struct {
	term js.Value
	host js.Value

	in *reader
}

// Open finds the terminal the page set up. boot.js publishes it as
// globalThis.surmise = { term, … } before starting the Go program, so the
// contract between the two halves is that one object.
func Open() (*Terminal, error) {
	host := js.Global().Get("surmise")
	if !host.Truthy() {
		return nil, errors.New("web: globalThis.surmise is missing; boot.js did not run")
	}
	term := host.Get("term")
	if !term.Truthy() {
		return nil, errors.New("web: globalThis.surmise.term is missing")
	}

	t := &Terminal{term: term, host: host, in: newReader()}

	// Keys and mouse arrive as terminal bytes. onData carries text, onBinary the
	// rare non-UTF-8 case; both go into the same pipe.
	t.term.Call("onData", js.FuncOf(func(_ js.Value, args []js.Value) any {
		t.in.push([]byte(args[0].String()))
		return nil
	}))
	t.term.Call("onBinary", js.FuncOf(func(_ js.Value, args []js.Value) any {
		// A binary string: one byte per code unit, so a rune-wise conversion
		// would corrupt it.
		s := args[0].String()
		b := make([]byte, 0, len(s))
		for _, r := range s {
			b = append(b, byte(r))
		}
		t.in.push(b)
		return nil
	}))

	// The window title bubbletea sets (OSC 0/2) becomes the tab's.
	t.term.Call("onTitleChange", js.FuncOf(func(_ js.Value, args []js.Value) any {
		js.Global().Get("document").Set("title", args[0].String())
		return nil
	}))

	return t, nil
}

// Size is the terminal's current size in cells. The page has already fitted it
// to the window before starting us, so this is the size of the first frame.
func (t *Terminal) Size() (cols, rows int) {
	return t.term.Get("cols").Int(), t.term.Get("rows").Int()
}

// Attach wires the resize event to the running Program.
//
// send is Program.Send, which blocks until the event loop reads the message —
// so it is called from a goroutine, never from the callback. The channel holds
// one pending size and drops the rest: a drag across the screen produces
// hundreds of these, and only the last one is true.
func (t *Terminal) Attach(send func(tea.Msg)) {
	sizes := make(chan [2]int, 1)

	t.term.Call("onResize", js.FuncOf(func(_ js.Value, args []js.Value) any {
		sz := [2]int{args[0].Get("cols").Int(), args[0].Get("rows").Int()}
		select {
		case sizes <- sz:
		default:
			// A size is already queued. Replace it, still without blocking:
			// the newest size is the only one that matters.
			select {
			case <-sizes:
			default:
			}
			select {
			case sizes <- sz:
			default:
			}
		}
		return nil
	}))

	go func() {
		for sz := range sizes {
			send(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		}
	}()
}

// Done reveals the page's "that's all" overlay. A returned main tears the Go
// instance down, so without this the player is left looking at a dead terminal.
func (t *Terminal) Done() {
	if fn := t.host.Get("onExit"); fn.Truthy() {
		fn.Invoke()
	}
}

// Writer is where the renderer's bytes go.
func (t *Terminal) Writer() io.Writer { return writer{t.term} }

// Reader is where the player's keystrokes come from.
func (t *Terminal) Reader() io.Reader { return t.in }

// writer hands whole byte slices to xterm.js.
//
// Bytes rather than a Go string, deliberately: a write can split a UTF-8
// sequence or an escape sequence, and xterm.js buffers a partial sequence until
// the rest arrives only when it is given bytes. write() is queued inside
// xterm.js, so returning immediately is correct rather than optimistic.
type writer struct{ term js.Value }

func (w writer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	buf := js.Global().Get("Uint8Array").New(len(p))
	js.CopyBytesToJS(buf, p)
	w.term.Call("write", buf)
	return len(p), nil
}

// reader turns the push of an event callback into the pull of an io.Reader.
type reader struct {
	mu   sync.Mutex
	buf  []byte
	wake chan struct{}
}

func newReader() *reader {
	return &reader{wake: make(chan struct{}, 1)}
}

// push is called from a JS callback, so it must not block. The signal is a
// buffered channel written with a default case: a wake-up already pending is as
// good as a new one.
func (r *reader) push(b []byte) {
	if len(b) == 0 {
		return
	}
	r.mu.Lock()
	r.buf = append(r.buf, b...)
	r.mu.Unlock()

	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *reader) Read(p []byte) (int, error) {
	for {
		r.mu.Lock()
		if len(r.buf) > 0 {
			n := copy(p, r.buf)
			r.buf = r.buf[n:]
			r.mu.Unlock()
			return n, nil
		}
		r.mu.Unlock()

		// Blocking here is safe and necessary: this runs on bubbletea's input
		// goroutine, not on a callback, so the JS event loop keeps turning and
		// push can still be called.
		<-r.wake
	}
}
