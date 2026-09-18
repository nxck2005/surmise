package theme

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// MaxFileBytes bounds one theme file, on the way in and on the way out. A
// theme is a page of settings; the cap is far above any honest file while
// keeping a hand-edited or imported one from being read or carried whole.
const MaxFileBytes = 64 << 10

// A theme a player wrote is a file, and a backup has to carry it as one.
//
// The rest of this package hands out parsed themes, which is what the app
// needs to draw with. It is not what a backup needs: a theme file is prose the
// player wrote, with their comments and their spacing in it, and a round trip
// through Parse and back would return something they did not write. So these
// two functions deal in bytes and never look inside them.
//
// They live here rather than in internal/backup because this package already
// owns that directory — Open reads it, EnsureDir creates and seeds it — and the
// UI is not allowed to reach for a path of its own.

// File is one theme exactly as it sits on disk.
type File struct {
	Name string `json:"name"` // the base name, including the .toml suffix
	Body string `json:"body"`
}

// safeName is what a theme file may be called. It is deliberately strict: the
// name in a backup was written by whoever made the backup, and it is about to
// be joined onto a path. A name with a separator or a parent reference in it
// would write outside the themes directory entirely.
var safeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.toml$`)

// validName reports whether a name from outside may be written into a theme
// directory. filepath.Base alone is not enough — it would silently accept
// "../../evil.toml" by rewriting it, which turns a refusal into a surprise.
func validName(name string) bool {
	if !safeName.MatchString(name) || strings.Contains(name, "..") {
		return false
	}
	return filepath.Base(name) == name
}

// Files reads every theme in dir, sorted by name so two reads of an unchanged
// directory are identical. A directory that is not there is not an error: an
// install that never wrote a theme simply has none.
//
// A name that is a symlink is not read at all: it is returned in linked so the
// caller can say what was left out. That is deliberately narrower than the
// picker, which follows a link — see Library — because a backup is a copy meant
// to leave the machine, and silently carrying whatever a link points at would
// put files the player never chose into a file they may hand to somebody else.
// Loading a dotfiles theme and copying an arbitrary out-of-tree file into a
// portable archive are different trust decisions.
func Files(dir string) (files []File, linked []string, err error) {
	if dir == "" {
		return nil, nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("theme: read %s: %w", dir, err)
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		if e.Type()&fs.ModeSymlink != 0 {
			linked = append(linked, e.Name())
			continue
		}
		b, err := readThemeFile(filepath.Join(dir, e.Name()))
		if err != nil {
			// One unreadable theme is not worth failing a whole backup over;
			// the player still has every other one. This follows JSON.All,
			// which skips a corrupt puzzle for the same reason.
			continue
		}
		files = append(files, File{Name: e.Name(), Body: string(b)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, linked, nil
}

// readThemeFile reads one theme, with MaxFileBytes applied while it is read.
//
// The cap has to be measured on the file the reads will come from, which is not
// the same as the directory entry: a theme may be a symlink, and
// fs.DirEntry.Info reports the link itself — a few bytes — while the target can
// be any size. So the descriptor is opened once and statted, and the bytes come
// from that same descriptor through a LimitReader, which is also what makes a
// file that grows under the stat, or a special file like /dev/zero, stop at the
// cap rather than being read whole.
//
// A target that is not a regular file is refused, and statted once before the
// open as well: opening a FIFO for reading blocks until a writer appears, so a
// planted one must be recognised by its mode before the open, not after. The
// check is repeated on the descriptor because a swap between the two is the
// only thing the first check cannot see.
func readThemeFile(path string) ([]byte, error) {
	before, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if err := checkThemeSize(before); err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if err := checkThemeSize(info); err != nil {
		return nil, err
	}

	b, err := io.ReadAll(io.LimitReader(f, MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > MaxFileBytes {
		return nil, fmt.Errorf("theme: file is larger than %d bytes", MaxFileBytes)
	}
	return b, nil
}

// checkThemeSize refuses a file that is not a plain one, or is too large to be
// a theme. It takes fs.FileInfo rather than a path so the caller decides
// whether that info came from before or after the open.
func checkThemeSize(info fs.FileInfo) error {
	if !info.Mode().IsRegular() {
		return errors.New("theme: not a regular file")
	}
	if info.Size() > MaxFileBytes {
		return fmt.Errorf("theme: file is larger than %d bytes", MaxFileBytes)
	}
	return nil
}

// WriteNew writes the themes that are not already there and leaves the rest
// alone, reporting how many it did each of. It never overwrites: a theme file
// is the player's own writing, and a restore that replaced one would destroy
// work that the backup was supposed to protect.
//
// A name that could escape the directory is skipped and named in the error,
// after everything safe has been written — a hostile entry must not stop the
// honest ones landing.
func WriteNew(dir string, files []File) (added, skipped int, err error) {
	if len(files) == 0 {
		return 0, 0, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return 0, 0, fmt.Errorf("theme: create %s: %w", dir, err)
	}

	var refused, oversized []string
	for _, f := range files {
		switch {
		case !validName(f.Name):
			refused = append(refused, f.Name)
			skipped++
			continue
		case len(f.Body) > MaxFileBytes:
			oversized = append(oversized, f.Name)
			skipped++
			continue
		}

		// O_EXCL rather than Stat-then-WriteFile: the check and the create are
		// one step, so nothing can appear between them, and a pre-existing
		// symlink — even a dangling one — is an EEXIST rather than a path out
		// of the directory. A theme that is already there is left alone, which
		// is the same rule as before; reads deliberately follow symlinks, so a
		// theme linked in from elsewhere stays usable, but a restore never
		// creates or crosses one.
		path := filepath.Join(dir, f.Name)
		fh, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			skipped++
			continue
		}
		if err != nil {
			return added, skipped, fmt.Errorf("theme: write %s: %w", f.Name, err)
		}
		if _, err := fh.Write([]byte(f.Body)); err != nil {
			fh.Close()
			return added, skipped, fmt.Errorf("theme: write %s: %w", f.Name, err)
		}
		if err := fh.Close(); err != nil {
			return added, skipped, fmt.Errorf("theme: write %s: %w", f.Name, err)
		}
		added++
	}

	if len(refused) > 0 || len(oversized) > 0 {
		// quoted, not spliced in raw: this error is rendered on the backup
		// screen, and a refused name is exactly where terminal control
		// characters would be coming from. %q turns an escape into text.
		var parts []string
		if len(refused) > 0 {
			parts = append(parts, fmt.Sprintf("%d name(s) that are not a plain *.toml: %s",
				len(refused), quoteNames(refused)))
		}
		if len(oversized) > 0 {
			parts = append(parts, fmt.Sprintf("%d theme(s) larger than %d bytes: %s",
				len(oversized), MaxFileBytes, quoteNames(oversized)))
		}
		return added, skipped, errors.New("theme: refused " + strings.Join(parts, "; "))
	}
	return added, skipped, nil
}

func quoteNames(names []string) string {
	quoted := make([]string, len(names))
	for i, name := range names {
		quoted[i] = fmt.Sprintf("%q", name)
	}
	return strings.Join(quoted, ", ")
}
