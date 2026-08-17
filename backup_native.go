//go:build !js

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/nxck2005/surmise/internal/backup"
	"github.com/nxck2005/surmise/internal/build"
	"github.com/nxck2005/surmise/internal/store"
	"github.com/nxck2005/surmise/internal/theme"
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
