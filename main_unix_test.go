//go:build !js && !windows

package main

import (
	"io"
	"strings"
	"syscall"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/ui"
)

// Closing the terminal sends SIGHUP, which bubbletea does not handle; attach
// turns it into a quit, so start returns and runs the exit path.
func TestAHangUpEndsTheProgram(t *testing.T) {
	s, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- start(s, nil, ui.Options{},
			tea.WithInput(strings.NewReader("")), tea.WithOutput(io.Discard), tea.WithoutSignalHandler())
	}()
	time.Sleep(200 * time.Millisecond) // let attach register before the signal
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the program kept running after SIGHUP")
	}
}
