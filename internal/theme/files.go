package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

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
func Files(dir string) ([]File, error) {
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("theme: read %s: %w", dir, err)
	}

	var files []File
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			// One unreadable theme is not worth failing a whole backup over;
			// the player still has every other one. This follows JSON.All,
			// which skips a corrupt puzzle for the same reason.
			continue
		}
		files = append(files, File{Name: e.Name(), Body: string(b)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files, nil
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
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, 0, fmt.Errorf("theme: create %s: %w", dir, err)
	}

	var refused []string
	for _, f := range files {
		if !validName(f.Name) {
			refused = append(refused, f.Name)
			skipped++
			continue
		}
		path := filepath.Join(dir, f.Name)
		if _, err := os.Stat(path); err == nil {
			skipped++
			continue
		}
		if err := os.WriteFile(path, []byte(f.Body), 0o644); err != nil {
			return added, skipped, fmt.Errorf("theme: write %s: %w", f.Name, err)
		}
		added++
	}
	if len(refused) > 0 {
		return added, skipped, fmt.Errorf("theme: refused %d name(s) that are not a plain *.toml: %s",
			len(refused), strings.Join(refused, ", "))
	}
	return added, skipped, nil
}
