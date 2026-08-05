package build

import (
	"strings"
	"testing"
)

// Every field the about screen prints unconditionally must be non-empty, since
// nothing downstream checks before rendering it.
func TestGetAlwaysDescribesTheBinary(t *testing.T) {
	i := Get()
	if i.Version == "" {
		t.Error("Version is empty; an unstamped build must fall back to " + devVersion)
	}
	if i.GoVersion == "" || i.OS == "" || i.Arch == "" {
		t.Errorf("toolchain incomplete: %+v", i)
	}
}

// A test binary has no stamped version and reports "(devel)" at most, so this
// is the fallback path in the only environment that exercises it.
func TestUnstampedBuildIsDev(t *testing.T) {
	if version != "" {
		t.Skipf("built with a stamped version %q", version)
	}
	if got := Get().Version; got != devVersion && !strings.HasPrefix(got, "v") {
		t.Errorf("Version = %q, want %q or a module version", got, devVersion)
	}
}

func TestStringIsOneLine(t *testing.T) {
	i := Info{Version: "1.2.0", Revision: "a1b2c3d", Modified: true, GoVersion: "go1.26.5", OS: "linux", Arch: "amd64"}
	got := i.String()
	want := "wortle 1.2.0 (a1b2c3d, dirty) go1.26.5 linux/amd64"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if strings.Contains(Get().String(), "\n") {
		t.Error("String() spans lines")
	}
}

// No VCS stamp is the normal case for a module download, and must not leave
// stray punctuation in the line.
func TestStringWithoutRevision(t *testing.T) {
	i := Info{Version: "1.2.0", GoVersion: "go1.26.5", OS: "darwin", Arch: "arm64"}
	if got, want := i.String(), "wortle 1.2.0 go1.26.5 darwin/arm64"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if i.Commit() != "" {
		t.Errorf("Commit() = %q, want empty", i.Commit())
	}
}
