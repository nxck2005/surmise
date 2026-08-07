// Package brand is the single place the product's name lives.
//
// The name reaches the outside world in more places than it looks: the binary,
// the window title, the menu, the board header, the `-version` line, the
// directory under the user's config dir, and the prefix on every environment
// variable. Before this package each of those was its own string literal, which
// made renaming the project a sweep across thirty-odd files with a real chance
// of missing one — or, worse, of catching one that must never change.
//
// So: renaming the project should mean editing this file and the module path,
// and nothing else.
//
// The deliberate exception is internal/daily, which must NOT import this
// package. Its derivation tags are wire format, not branding — see the comments
// on idVersion and seedVersion there. A name in a hash is a name you can never
// change, so those two ideas are kept apart on purpose.
package brand

import "strings"

const (
	// Name is the product, and with it the binary, the window title, the
	// directory under the user config dir, and the prefix on the environment
	// variables. Lowercase: it is a command name first and a proper noun
	// second, and the UI renders it as typed.
	Name = "surmise"

	// Repo is where the project lives, shown on the about screen. It is written
	// out rather than derived from the module path because there is no honest
	// way to read that at runtime — debug.ReadBuildInfo reports it only for a
	// binary built as a module, not for `go run`.
	Repo = "github.com/nxck2005/" + Name
)

// Env names one of the product's environment variables: Env("THEME") is
// "SURMISE_THEME". Callers pass the bare setting rather than the whole name, so
// the prefix cannot drift between them and a rename moves all of them at once.
func Env(setting string) string {
	return strings.ToUpper(Name) + "_" + setting
}
