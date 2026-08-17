//go:build js && wasm

package web

import (
	"errors"
	"fmt"
	"time"

	"syscall/js"
)

// Moving a backup file in and out of a browser.
//
// A page cannot write to a path and cannot read one, so both directions go
// through the two functions boot.js publishes on globalThis.surmise: a download
// out, and a file picker in. This file is the whole of what the Go side knows
// about either.
//
// The rule this package documents at the top applies here more than anywhere: a
// js.Func callback must never block. The picker answers whenever the player
// gets round to choosing, so the callback drops its answer into a buffered
// channel and returns, and the *caller* blocks on that channel — from a
// bubbletea command's own goroutine, never from a callback.

// pickTimeout bounds how long one picker may stay open before the waiting
// goroutine gives up on it.
//
// It exists for the case a browser tells us nothing about: a picker dismissed
// in a way that fires no cancel event (older Safari does this). Without a bound,
// each such dismissal would leave a goroutine parked on a channel for the life
// of the page. Ten minutes is far longer than anybody takes to choose a file and
// far shorter than a session.
const pickTimeout = 10 * time.Minute

// Transfer is the browser's implementation of the UI's backup file transfer.
type Transfer struct{ host js.Value }

// NewTransfer wires up to the page. It returns an error when the page half is
// not there, which is what the Node smoke test sees: the game then runs with no
// backup row rather than with one that cannot work.
func NewTransfer() (*Transfer, error) {
	host := js.Global().Get("surmise")
	if !host.Truthy() {
		return nil, errors.New("web: globalThis.surmise is missing; boot.js did not run")
	}
	if !host.Get("saveFile").Truthy() || !host.Get("openFile").Truthy() {
		// A host that publishes a terminal but no file functions: an older
		// boot.js against a newer wasm, or a harness that only wanted the
		// terminal. Neither is fatal — the game runs without a backup row.
		return nil, errors.New("web: this host offers no file access; backing up is off")
	}
	return &Transfer{host: host}, nil
}

// Save hands the archive to the page, which offers it as a download. The name
// is the page's business — a browser puts it wherever downloads go — so what
// comes back is a file name and not a path.
func (t *Transfer) Save(b []byte) (where string, err error) {
	defer func() {
		if r := recover(); r != nil {
			where, err = "", fmt.Errorf("web: the download was refused: %v", r)
		}
	}()

	name := t.host.Call("saveFile", string(b))
	if !name.Truthy() {
		return "", errors.New("web: the browser refused the download")
	}
	return name.String(), nil
}

// Load opens the page's file picker and waits for the answer.
//
// It blocks, which is safe here and nowhere else in this package: the UI calls
// it from a bubbletea command, which runs on its own goroutine while the JS
// event loop keeps turning — so the callback that will wake it can still fire.
//
// A player who closes the picker without choosing gets no bytes and no error.
// Choosing nothing is not a failure, and reporting it as one would put an error
// on screen for pressing escape.
func (t *Transfer) Load() (b []byte, from string, err error) {
	defer func() {
		if r := recover(); r != nil {
			b, from, err = nil, "", fmt.Errorf("web: the file picker failed: %v", r)
		}
	}()

	type picked struct {
		name string
		body string
		err  string
	}
	// Buffered, so the callback can leave its answer and return even if nobody
	// is reading yet — which is the whole rule this package is built on.
	answer := make(chan picked, 1)

	var done js.Func
	done = js.FuncOf(func(_ js.Value, args []js.Value) any {
		// One shot: release the callback as soon as it has fired, so a picker
		// opened many times does not leak one of these per opening.
		defer done.Release()

		var p picked
		if len(args) > 0 && args[0].Truthy() {
			v := args[0]
			if e := v.Get("error"); e.Truthy() {
				p.err = e.String()
			}
			if n := v.Get("name"); n.Truthy() {
				p.name = n.String()
			}
			if body := v.Get("text"); body.Truthy() {
				p.body = body.String()
			}
		}
		select {
		case answer <- p:
		default: // already answered; nothing to do and nothing to block on
		}
		return nil
	})

	t.host.Call("openFile", done)

	select {
	case p := <-answer:
		if p.err != "" {
			return nil, "", errors.New("web: " + p.err)
		}
		if p.name == "" {
			return nil, "", nil // closed without choosing
		}
		return []byte(p.body), p.name, nil
	case <-time.After(pickTimeout):
		done.Release()
		return nil, "", nil
	}
}
