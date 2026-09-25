//go:build !js

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nxck2005/surmise/internal/backup"
)

// The desktop backup screen's file access. The screen itself is tested in
// internal/ui against an in-memory transfer; what is left to prove here is the
// part that touches a disk.

func TestFileTransferRoundTrip(t *testing.T) {
	dir := t.TempDir()
	f := fileTransfer{dir: dir}

	want := []byte(`{"format":"surmise.backup"}`)
	where, err := f.Save(want)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !strings.HasPrefix(where, filepath.Join(dir, backupDir)) {
		t.Errorf("saved to %q, want it under the data directory", where)
	}

	got, from, err := f.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("read back %q, want %q", got, want)
	}
	if from != filepath.Base(where) {
		t.Errorf("reported %q, want the file's name %q", from, filepath.Base(where))
	}
}

// A second save on the same day must not overwrite the first. The flag refuses
// an existing file instead; a button cannot, because pressing save has to leave
// a file — just never the one that was already there.
func TestFileTransferKeepsEveryBackup(t *testing.T) {
	dir := t.TempDir()
	f := fileTransfer{dir: dir}

	first, err := f.Save([]byte("one"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.Save([]byte("two"))
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("the second save wrote over the first at %s", first)
	}

	b, err := os.ReadFile(first)
	if err != nil || string(b) != "one" {
		t.Errorf("the first backup is now %q, %v; want it untouched", b, err)
	}
	if !strings.HasSuffix(second, "-2.json") {
		t.Errorf("the second backup is %q, want it numbered", filepath.Base(second))
	}
}

// Load takes the newest by name, because the names are dated and a file copied
// from another machine keeps its name while its timestamp becomes the copy's.
func TestFileTransferLoadsTheNewestByName(t *testing.T) {
	dir := t.TempDir()
	backups := filepath.Join(dir, backupDir)
	if err := os.MkdirAll(backups, 0o755); err != nil {
		t.Fatal(err)
	}
	// Written oldest last, and with the newest name given the oldest timestamp,
	// so a mtime-based answer would pick the wrong one.
	write := func(name, body string, mod time.Time) {
		t.Helper()
		path := filepath.Join(backups, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	write("surmise-backup-2026-08-18.json", "newest", now.Add(-72*time.Hour))
	write("surmise-backup-2026-08-01.json", "oldest", now)
	write("notes.txt", "not a backup", now)

	b, from, err := fileTransfer{dir: dir}.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(b) != "newest" {
		t.Errorf("loaded %q, want the newest date", b)
	}
	if from != "surmise-backup-2026-08-18.json" {
		t.Errorf("reported %q, want the newest file", from)
	}
}

// A foreign file must not shadow the real backups. The newest was chosen by
// name, so any .json dropped into the directory sorted above every dated one
// and "load a backup" read the stranger's file. Only the names backupName
// writes are candidates now.
func TestFileTransferIgnoresFilesThisAppDidNotWrite(t *testing.T) {
	dir := t.TempDir()
	backups := filepath.Join(dir, backupDir)
	if err := os.MkdirAll(backups, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"surmise-backup-2026-08-18.json":   "real",
		"zzz-drop.json":                    "foreign",
		"surmise-backup-notadate.json":     "foreign",
		"surmise-backup-2026-08-18-1.json": "foreign", // backupName never writes -1
		"other-backup-2026-09-01.json":     "foreign",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(backups, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	b, from, err := fileTransfer{dir: dir}.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(b) != "real" || from != "surmise-backup-2026-08-18.json" {
		t.Errorf("Load = %q from %q, want the real backup", b, from)
	}
}

// Nothing but foreign files is "no backups", not "the newest of them".
func TestFileTransferWithOnlyForeignFilesSaysThereAreNone(t *testing.T) {
	dir := t.TempDir()
	backups := filepath.Join(dir, backupDir)
	if err := os.MkdirAll(backups, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backups, "zzz-drop.json"), []byte("foreign"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err := fileTransfer{dir: dir}.Load()
	if err == nil || !strings.Contains(err.Error(), "no backups") {
		t.Errorf("Load = %v, want a no-backups refusal", err)
	}
}

// The numbering is read as a number, not as text: as text, the tenth backup of
// a day ("-10") sorts before the ninth ("-9").
func TestFileTransferNumbersBackupsNumerically(t *testing.T) {
	dir := t.TempDir()
	backups := filepath.Join(dir, backupDir)
	if err := os.MkdirAll(backups, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"surmise-backup-2026-08-18-9.json":  "nine",
		"surmise-backup-2026-08-18-10.json": "ten",
	} {
		if err := os.WriteFile(filepath.Join(backups, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	b, from, err := fileTransfer{dir: dir}.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(b) != "ten" || from != "surmise-backup-2026-08-18-10.json" {
		t.Errorf("Load = %q from %q, want the tenth save", b, from)
	}
}

// Nothing to load says where a backup would go, rather than reporting a missing
// directory the player has never been told about.
func TestFileTransferWithNoBackupsSaysWhereTheyGo(t *testing.T) {
	dir := t.TempDir()
	_, _, err := fileTransfer{dir: dir}.Load()
	if err == nil {
		t.Fatal("Load of an empty install returned no error")
	}
	if !strings.Contains(err.Error(), filepath.Join(dir, backupDir)) {
		t.Errorf("error = %v, want it to name where backups go", err)
	}
}

// The name is dated, so a directory of these reads as a list of days.
// Importing reads through a cap: a FIFO, a device or a swapped-in file cannot
// make the app allocate without bound before the archive is even parsed.
func TestReadCappedRefusesAnOversizedStream(t *testing.T) {
	b, err := readCapped(strings.NewReader("ok"))
	if err != nil || string(b) != "ok" {
		t.Fatalf("readCapped(small) = %q, %v", b, err)
	}
	if _, err := readCapped(strings.NewReader(strings.Repeat("a", backup.MaxArchiveBytes+1))); err == nil {
		t.Error("readCapped accepted a stream larger than MaxArchiveBytes")
	}
}

func TestBackupNameIsDated(t *testing.T) {
	day := time.Date(2026, 8, 18, 23, 59, 0, 0, time.UTC)
	if got := backupName(day, 1); got != "surmise-backup-2026-08-18.json" {
		t.Errorf("backupName = %q", got)
	}
	if got := backupName(day, 3); got != "surmise-backup-2026-08-18-3.json" {
		t.Errorf("backupName for a third save = %q", got)
	}
}

// parseBackupName is the boundary between "a file this app wrote" and "a file
// that happens to be here": it accepts exactly what backupName produces, so
// the filter and the writer cannot drift apart.
func TestParseBackupName(t *testing.T) {
	accept := map[string]struct {
		date string
		n    int
	}{
		"surmise-backup-2026-08-18.json":    {"2026-08-18", 1},
		"surmise-backup-2026-08-18-2.json":  {"2026-08-18", 2},
		"surmise-backup-2026-08-18-10.json": {"2026-08-18", 10},
	}
	for name, want := range accept {
		day, n, ok := parseBackupName(name)
		if !ok || day.Format(time.DateOnly) != want.date || n != want.n {
			t.Errorf("parseBackupName(%q) = %v, %d, %v; want %s, %d, true", name, day, n, ok, want.date, want.n)
		}
	}

	for _, name := range []string{
		"notes.txt",
		"zzz-drop.json",
		"surmise-backup-notadate.json",
		"surmise-backup-2026-13-01.json",
		"surmise-backup-2026-08-18-1.json", // backupName's first save has no number
		"surmise-backup-2026-08-18-0.json",
		"surmise-backup-2026-08-18-+2.json",
		"surmise-backup-2026-08-18-02.json",
		"surmise-backup-2026-08-18-0002.json",
		"surmise-backup-2026-08-18.JSON",
		"other-backup-2026-08-18.json",
		"surmise-backup-2026-08-18.json.bak",
	} {
		if _, _, ok := parseBackupName(name); ok {
			t.Errorf("parseBackupName(%q) accepted a name this app does not write", name)
		}
	}
}
