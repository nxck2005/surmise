//go:build unix

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/store"
)

func TestBackupLoadRefusesFIFO(t *testing.T) {
	dir := t.TempDir()
	backups := filepath.Join(dir, backupDir)
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(backups, backupName(time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC), 1))
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatalf("Mkfifo: %v", err)
	}

	var loadErr error
	withinBackupRead(t, func() {
		_, _, loadErr = (fileTransfer{dir: dir}).Load()
	})
	if loadErr == nil || !strings.Contains(loadErr.Error(), "not a regular file") {
		t.Fatalf("Load = %v, want a non-regular-file refusal", loadErr)
	}
}

func TestImportBackupRefusesFIFO(t *testing.T) {
	dir := t.TempDir()
	data, err := store.NewJSON(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "fifo.json")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatalf("Mkfifo: %v", err)
	}

	var importErr error
	withinBackupRead(t, func() {
		importErr = importBackup(data, filepath.Join(dir, "themes"), path)
	})
	if importErr == nil || !strings.Contains(importErr.Error(), "not a regular file") {
		t.Fatalf("importBackup = %v, want a non-regular-file refusal", importErr)
	}
}

func withinBackupRead(t *testing.T, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the backup read blocked on a non-regular file")
	}
}
