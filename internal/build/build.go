// Package build reports what this binary is: its version, the commit it came
// from and the toolchain that built it.
//
// It is the only place a version number lives. Nothing here affects play — it
// is display data — which is why a release may stamp it and a plain `go build`
// may leave it blank without either build behaving differently.
package build

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// version is stamped at release time:
//
//	go build -ldflags "-X github.com/nxck2005/wortle/internal/build.version=1.2.0"
//
// Empty means "nobody said", which falls back to the module version and then to
// devVersion. Unlike the daily's pepper (see internal/daily/local.go), this is
// safe to inject: two builds disagreeing about it changes a line of text, not a
// word anyone has to guess.
var version = ""

// devVersion is what an unstamped build calls itself. `go install …@latest`
// fills the module version in, so this is mostly the working copy.
const devVersion = "dev"

// shortRevision is how much of a commit hash is worth showing.
const shortRevision = 7

// Info is a snapshot of the build, ready to display.
type Info struct {
	// Version is the stamped version, the module version, or devVersion. Never
	// empty, so a caller can print it without checking.
	Version string
	// Revision is the commit, shortened. Empty when the binary was built
	// outside a checkout, which is normal for a module download.
	Revision string
	// Modified reports that the tree was dirty when this was built.
	Modified bool
	// Time is when it was built, as VCS reported it (RFC 3339), or empty.
	Time string

	GoVersion string
	OS, Arch  string
}

// Get reads the build stamps the toolchain embedded. It never fails: anything
// missing is simply an empty field.
func Get() Info {
	i := Info{
		Version:   version,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}

	if bi, ok := debug.ReadBuildInfo(); ok {
		if i.Version == "" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			i.Version = bi.Main.Version
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				i.Revision = s.Value
				if len(i.Revision) > shortRevision {
					i.Revision = i.Revision[:shortRevision]
				}
			case "vcs.modified":
				i.Modified = s.Value == "true"
			case "vcs.time":
				i.Time = s.Value
			}
		}
	}

	if i.Version == "" {
		i.Version = devVersion
	}
	return i
}

// Commit describes where the code came from, in one phrase: the revision, noted
// as dirty if it was. Empty when there is no VCS stamp at all.
func (i Info) Commit() string {
	if i.Revision == "" {
		return ""
	}
	if i.Modified {
		return i.Revision + ", dirty"
	}
	return i.Revision
}

// Toolchain is the Go version and platform, as one phrase.
func (i Info) Toolchain() string {
	return fmt.Sprintf("%s %s/%s", i.GoVersion, i.OS, i.Arch)
}

// String is the one-line form behind `wortle -version`.
func (i Info) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "wortle %s", i.Version)
	if c := i.Commit(); c != "" {
		fmt.Fprintf(&b, " (%s)", c)
	}
	fmt.Fprintf(&b, " %s", i.Toolchain())
	return b.String()
}
