//go:build !js

package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nxck2005/surmise/internal/backup"
	"github.com/nxck2005/surmise/internal/brand"
	"github.com/nxck2005/surmise/internal/build"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/theme"
	"github.com/nxck2005/surmise/internal/ui"
)

// -export and -import, the desktop half of backup and restore.
//
// They are flags rather than a screen because this is the one feature a player
// may need when the app will not open: a save directory being copied off a
// machine that is about to be wiped, or a history being moved into the browser
// build. A command that works from a script and can be piped is worth more here
// than a menu entry, and internal/backup keeps the format itself platform-free
// so the browser build can grow its own way in without a second implementation.

// stdio is what both flags read as "not a file": standard output for -export,
// standard input for -import.
const stdio = "-"

// backupDir is where the backup screen keeps its files, under the data
// directory beside `puzzles` and `themes`.
const backupDir = "backups"

// fileTransfer is the backup screen's way in and out on a desktop: a directory
// of dated files under the data dir.
//
// The screen names no path, and neither does the player. A text field in a TUI
// is a poor way to type a path — no completion, no globbing, nothing the shell
// gives for free — so the button writes somewhere predictable and says where it
// went, and `-export` / `-import` stay for anybody who wants a path of their
// own.
type fileTransfer struct{ dir string }

// The screen's contract, checked here rather than discovered at the call site.
var _ ui.Transfer = fileTransfer{}

// Save writes today's backup, and does not overwrite: a second save on the same
// day becomes -2, a third -3. The flag refuses an existing file instead, which
// is right for a command somebody typed a name into and wrong for a button —
// pressing save must always leave a file, and it must never be the one that was
// already there.
func (f fileTransfer) Save(b []byte) (string, error) {
	dir := filepath.Join(f.dir, backupDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}

	// 0600 like the saved puzzles: a history carries the answer to every puzzle
	// in it.
	for n := 1; ; n++ {
		path := filepath.Join(dir, backupName(time.Now(), n))
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("write %s: %w", path, err)
		}
		if _, err := file.Write(b); err != nil {
			file.Close()
			return "", fmt.Errorf("write %s: %w", path, err)
		}
		if err := file.Close(); err != nil {
			return "", fmt.Errorf("write %s: %w", path, err)
		}
		return path, nil
	}
}

// backupName is the file name for the nth backup of a day: the first is plain,
// and the rest carry their number. Dated rather than stamped to the second, so
// a directory of these reads as a list of days.
func backupName(now time.Time, n int) string {
	if n <= 1 {
		return fmt.Sprintf("%s-backup-%s.json", brand.Name, now.Format(time.DateOnly))
	}
	return fmt.Sprintf("%s-backup-%s-%d.json", brand.Name, now.Format(time.DateOnly), n)
}

// Load reads the newest backup in the directory.
//
// Newest by name, not by modification time: the names are dated and sort in
// date order, and a file copied in from another machine keeps its name while
// its timestamp becomes the copy's. The name is what the player sees, so the
// name is what decides.
func (f fileTransfer) Load() ([]byte, string, error) {
	dir := filepath.Join(f.dir, backupDir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, "", fmt.Errorf("no backups yet — save one first, and it goes in %s", dir)
	}
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", dir, err)
	}

	newest := ""
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if e.Name() > newest {
			newest = e.Name()
		}
	}
	if newest == "" {
		return nil, "", fmt.Errorf("no backups in %s", dir)
	}

	path := filepath.Join(dir, newest)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", path, err)
	}
	return b, newest, nil
}

// exportBackup writes the whole install to path.
//
// It refuses to overwrite an existing file. Everything else about this feature
// is built on "only ever add", and a backup command that silently replaced last
// month's backup would be the one part of it that destroys history.
func exportBackup(s *store.JSON, themeDir, path string) error {
	// A theme directory that cannot be read costs the themes, not the backup:
	// the puzzles are what nothing else can replace.
	themes, err := theme.Files(themeDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	b, err := backup.Build(s, s.Settings(), themes, build.Get().String(), time.Now())
	if err != nil {
		return err
	}

	if path == stdio {
		_, err := os.Stdout.Write(b)
		return err
	}

	// O_EXCL rather than a Stat first: the check and the write are then one
	// step, and nothing can appear in between them.
	// 0600, matching the saved puzzles — a history carries the answers to every
	// puzzle in it, and a backup is the one copy that tends to end up in a
	// shared directory.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("%s already exists; back up to a new file rather than over an old one", path)
	}
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	fmt.Printf("wrote %s\n", path)
	return nil
}

// importBackup merges a backup file into this install and reports what it did.
//
// There is no confirmation to give, which is the point of the merge rule: the
// worst an import of the wrong file can do is add puzzles somebody else played.
// Nothing already here is replaced or removed.
func importBackup(s *store.JSON, themeDir, path string) error {
	var (
		b   []byte
		err error
	)
	if path == stdio {
		b, err = io.ReadAll(os.Stdin)
	} else {
		b, err = os.ReadFile(path)
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	res, err := backup.Apply(b, s, s.Settings())
	if err != nil {
		return err
	}

	// Preferences are saved before the themes are written: a chosen theme that
	// came in with the file names one of them, and the app should not open on a
	// theme whose file failed to land.
	if len(res.SettingsFilled) > 0 || res.PlaytimeAdded > 0 {
		if err := s.SaveSettings(res.Settings); err != nil {
			return err
		}
	}

	added, skipped, themeErr := theme.WriteNew(themeDir, res.Themes)

	report(res, added, skipped)
	// Reported last and as a warning: a refused theme name must not make a
	// restore that moved a history look like it failed.
	if themeErr != nil {
		fmt.Fprintln(os.Stderr, themeErr)
	}
	return nil
}

// report says what the import did, in one line per thing that changed. An
// import that changed nothing says so rather than printing a list of zeroes:
// importing the same file twice is meant to be uneventful, and the player
// should be able to see that it was.
func report(res backup.Result, themesAdded, themesSkipped int) {
	if !res.Any() && themesAdded == 0 {
		fmt.Println("nothing new in that backup; this install already has all of it")
		return
	}

	if res.PuzzlesAdded > 0 {
		fmt.Printf("restored %s", plural(res.PuzzlesAdded, "puzzle"))
		if res.PuzzlesKept > 0 {
			fmt.Printf(", kept %s already here", plural(res.PuzzlesKept, "puzzle"))
		}
		fmt.Println()
	}

	if len(res.SettingsFilled) > 0 {
		fmt.Printf("filled in %s\n", strings.Join(res.SettingsFilled, ", "))
	}
	if res.PlaytimeAdded > 0 {
		fmt.Printf("time played is now the larger figure, up by %v\n", res.PlaytimeAdded.Round(time.Second))
	}
	if themesAdded > 0 {
		fmt.Printf("added %s", plural(themesAdded, "theme"))
		if themesSkipped > 0 {
			fmt.Printf(", left %s alone", plural(themesSkipped, "theme"))
		}
		fmt.Println()
	}
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
