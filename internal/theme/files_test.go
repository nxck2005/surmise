package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A theme file is prose the player wrote. Files hands back the bytes, comments
// and spacing included, rather than anything a parse would normalise away.
func TestFilesReadsBytesVerbatimAndSorted(t *testing.T) {
	dir := t.TempDir()
	body := "# my theme, spaced how I like it\n\nname  =  \"mine\"\n"
	write(t, dir, "mine.toml", body)
	write(t, dir, "another.toml", "name = \"another\"\n")
	write(t, dir, "notes.txt", "not a theme")
	if err := os.Mkdir(filepath.Join(dir, "sub.toml"), 0o755); err != nil {
		t.Fatal(err)
	}

	files, linked, err := Files(dir)
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if len(linked) != 0 {
		t.Errorf("linked = %v, want nothing — none of these is a symlink", linked)
	}
	if len(files) != 2 {
		t.Fatalf("read %d files, want the two themes: %+v", len(files), files)
	}
	if files[0].Name != "another.toml" || files[1].Name != "mine.toml" {
		t.Errorf("order = %s, %s; want them sorted by name", files[0].Name, files[1].Name)
	}
	if files[1].Body != body {
		t.Errorf("body = %q, want the file exactly as written", files[1].Body)
	}
}

// An install that never wrote a theme has no directory, and that is not a
// failure worth refusing a backup over.
func TestFilesAcceptsNoDirectory(t *testing.T) {
	files, linked, err := Files(filepath.Join(t.TempDir(), "never-created"))
	if err != nil || files != nil || linked != nil {
		t.Errorf("Files of a missing directory = %v, %v, %v; want nothing and no error", files, linked, err)
	}
	if files, linked, err := Files(""); err != nil || files != nil || linked != nil {
		t.Errorf("Files of no directory at all = %v, %v, %v; want nothing and no error", files, linked, err)
	}
}

// WriteNew adds and never replaces: a theme file is the player's own writing,
// and a restore that overwrote one would destroy the work it exists to protect.
func TestWriteNewKeepsWhatIsAlreadyThere(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "mine.toml", "mine, as I wrote it\n")

	added, skipped, err := WriteNew(dir, []File{
		{Name: "mine.toml", Body: "someone else's\n"},
		{Name: "theirs.toml", Body: "theirs\n"},
	})
	if err != nil {
		t.Fatalf("WriteNew: %v", err)
	}
	if added != 1 || skipped != 1 {
		t.Errorf("added %d and skipped %d, want 1 and 1", added, skipped)
	}

	b, err := os.ReadFile(filepath.Join(dir, "mine.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "mine, as I wrote it\n" {
		t.Errorf("mine.toml = %q, want it left alone", b)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "theirs.toml")); err != nil || string(b) != "theirs\n" {
		t.Errorf("theirs.toml = %q, %v; want it written", b, err)
	}
}

// The directory is created on demand, so a restore into an install that has
// never had a theme still lands.
func TestWriteNewCreatesTheDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "themes")
	added, _, err := WriteNew(dir, []File{{Name: "mine.toml", Body: "mine\n"}})
	if err != nil || added != 1 {
		t.Fatalf("WriteNew = %d, %v; want the file written", added, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "mine.toml")); err != nil {
		t.Errorf("the theme is not there: %v", err)
	}
}

// The name in an archive was written by whoever made the archive and is about
// to be joined onto a path. Anything that could leave the directory is refused
// — and the honest files around it still land, so a hostile entry cannot stop a
// restore.
func TestWriteNewRefusesNamesThatEscape(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "themes")
	added, skipped, err := WriteNew(dir, []File{
		{Name: "../escaped.toml", Body: "no"},
		{Name: "/etc/passwd.toml", Body: "no"},
		{Name: "sub/nested.toml", Body: "no"},
		{Name: ".hidden.toml", Body: "no"},
		{Name: "no-suffix", Body: "no"},
		{Name: "good.toml", Body: "yes\n"},
	})
	if err == nil || !strings.Contains(err.Error(), "refused") {
		t.Errorf("error = %v, want the refused names reported", err)
	}
	if added != 1 || skipped != 5 {
		t.Errorf("added %d and skipped %d, want 1 and 5", added, skipped)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "good.toml")); err != nil || string(b) != "yes\n" {
		t.Errorf("the safe file did not land: %q, %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escaped.toml")); err == nil {
		t.Error("a name with a parent reference wrote outside the themes directory")
	}
}

// The refusal error is printed to a terminal — the backup screen renders it —
// so a refused name has to arrive quoted, never as the control bytes it is made
// of. %q is what makes an escape read as text instead of as an escape.
func TestWriteNewQuotesRefusedNamesInItsError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "themes")
	_, _, err := WriteNew(dir, []File{{Name: "\x1b]0;owned\x07.toml", Body: "no"}})
	if err == nil {
		t.Fatal("WriteNew accepted a name full of control characters")
	}
	if strings.ContainsFunc(err.Error(), unicode.IsControl) {
		t.Errorf("the error carries raw control characters: %q", err.Error())
	}
}

// A theme body over the cap is skipped rather than written, and the honest
// files in the same archive still land.
func TestWriteNewRefusesAnOversizedTheme(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "themes")
	added, skipped, err := WriteNew(dir, []File{
		{Name: "big.toml", Body: strings.Repeat("a", MaxFileBytes+1)},
		{Name: "small.toml", Body: "small\n"},
	})
	if err == nil || !strings.Contains(err.Error(), "refused") {
		t.Errorf("error = %v, want the oversized theme refused", err)
	}
	if added != 1 || skipped != 1 {
		t.Errorf("added %d and skipped %d, want 1 and 1", added, skipped)
	}
	if _, err := os.Stat(filepath.Join(dir, "big.toml")); !os.IsNotExist(err) {
		t.Error("the oversized theme was written")
	}
}

// A restore never follows a link: a symlink already wearing a theme's name is
// left alone — dangling included — so a write cannot be steered out of the
// directory by one that a hostile archive could not name anyway.
func TestWriteNewNeverFollowsASymlink(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "themes")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(base, "outside.toml")
	if err := os.Symlink(target, filepath.Join(dir, "link.toml")); err != nil {
		t.Skipf("symlinks are unavailable here: %v", err)
	}

	added, skipped, err := WriteNew(dir, []File{{Name: "link.toml", Body: "no\n"}})
	if err != nil {
		t.Fatalf("WriteNew: %v", err)
	}
	if added != 0 || skipped != 1 {
		t.Errorf("added %d and skipped %d, want 0 and 1", added, skipped)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("a restore followed a symlink out of the themes directory")
	}
}

// A link is not followed into a backup. Loading a theme through one is the
// player's own business — Library still does — but a backup is a copy meant to
// leave the machine, and carrying whatever the link points at would copy files
// the player never chose into a file they may share. The name comes back so
// the caller can say what was left out.
func TestFilesSkipsASymlinkedTheme(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "themes")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(base, "outside.toml")
	if err := os.WriteFile(real, []byte("name = \"linked\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(dir, "link.toml")); err != nil {
		t.Skipf("symlinks are unavailable here: %v", err)
	}

	files, linked, err := Files(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("Files = %+v, want a linked theme left out of a backup", files)
	}
	if len(linked) != 1 || linked[0] != "link.toml" {
		t.Errorf("linked = %v, want the name reported so the caller can say it was left out", linked)
	}
}

// The cap has to be measured on the file that would be read, not on the
// directory entry: fs.DirEntry.Info reports a symlink's own size, a few bytes,
// so a link to a large file used to pass the check and then be read whole. A
// mode check alone would miss it too, because the target is a regular file.
func TestFilesDoesNotReadThroughAnOversizedLink(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "themes")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(base, "huge.toml")
	if err := os.WriteFile(target, []byte(strings.Repeat("a", MaxFileBytes+1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "huge.toml")); err != nil {
		t.Skipf("symlinks are unavailable here: %v", err)
	}

	files, linked, err := Files(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 || len(linked) != 1 {
		t.Errorf("Files = %d themes, %d links; want the oversized target left unread", len(files), len(linked))
	}
}

// Nothing to restore writes nothing, and does not create a directory an install
// with no themes never had.
func TestWriteNewOfNothingDoesNothing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "themes")
	added, skipped, err := WriteNew(dir, nil)
	if added != 0 || skipped != 0 || err != nil {
		t.Errorf("WriteNew(nil) = %d, %d, %v; want nothing done", added, skipped, err)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Error("WriteNew created a themes directory with nothing to put in it")
	}
}
