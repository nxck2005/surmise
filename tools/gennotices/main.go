// Command gennotices writes THIRD_PARTY_NOTICES.md.
//
// A Go binary statically contains the code of every module it links, plus the
// runtime and standard library. The MIT and BSD-3-Clause licences those carry
// all require their notice to travel with a binary distribution, so shipping a
// release archive without this file is a licence violation regardless of how
// permissive the terms are.
//
// This is generated rather than hand-written because Dependabot changes the
// dependency set on its own schedule, and a hand-written notices file goes
// stale silently — the failure mode is a release that quietly stops complying.
// Run it after any dependency change:
//
//	go run ./tools/gennotices
//
// The module licences are read out of the local module cache, so the text is
// whatever the build actually links, not what a registry claims. The two
// hand-maintained sections at the bottom (themes, word lists) live in this file
// because nothing in the dependency graph knows about them.
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nxck2005/surmise/internal/brand"
)

const outFile = "THIRD_PARTY_NOTICES.md"

// selfPath is this module, which is covered by the root LICENSE and must not be
// listed as a third party. Taken from internal/brand so that a rename cannot
// leave it stale — a wrong value here silently emits the project's own module
// as somebody else's dependency.
const selfPath = brand.Repo

func main() {
	log.SetFlags(0)

	mods, err := linkedModules()
	if err != nil {
		log.Fatalf("list modules: %v", err)
	}
	if len(mods) == 0 {
		log.Fatal("no linked modules found; is this being run from the repo root?")
	}
	log.Printf("modules: %d", len(mods))

	var b strings.Builder
	b.WriteString(preamble)

	b.WriteString("## The Go programming language\n\n")
	b.WriteString("The Go runtime and standard library are linked into every binary.\n\n")
	b.WriteString(fence(goLicense))
	b.WriteString("\nGo's additional patent grant:\n\n")
	b.WriteString(fence(goPatents))

	b.WriteString("\n## Go modules\n\n")
	b.WriteString("Read from the module cache at generation time, so this is the text of the\ncode actually linked.\n\n")
	for _, m := range mods {
		text, err := licenseText(m.Dir)
		if err != nil {
			// Never emit a partial notices file: a module whose licence cannot
			// be found is exactly the case a human has to look at.
			log.Fatalf("%s: %v", m.Path, err)
		}
		fmt.Fprintf(&b, "### %s\n\n%s\n", m.Path, versionLine(m.Version))
		b.WriteString(fence(text))
		if extra, err := os.ReadFile(filepath.Join(m.Dir, "PATENTS")); err == nil {
			b.WriteString("\nAdditional patent grant:\n\n")
			b.WriteString(fence(string(extra)))
		}
		b.WriteString("\n")
	}

	b.WriteString(handWritten)

	if err := os.WriteFile(outFile, []byte(b.String()), 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s", outFile)
}

type module struct {
	Path    string
	Version string
	Dir     string
}

// linkedModules returns the modules whose code reaches the binary. It asks the
// build rather than parsing go.mod, so a module listed as indirect but never
// actually linked does not appear, and one pulled in transitively does.
func linkedModules() ([]module, error) {
	out, err := exec.Command("go", "list", "-deps",
		"-f", "{{if .Module}}{{.Module.Path}}\t{{.Module.Version}}\t{{.Module.Dir}}{{end}}", ".").Output()
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var mods []module
	for line := range strings.Lines(string(out)) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		path, rest, ok := strings.Cut(line, "\t")
		if !ok || path == selfPath || seen[path] {
			continue
		}
		version, dir, _ := strings.Cut(rest, "\t")
		if dir == "" {
			return nil, fmt.Errorf("%s has no local directory; run `go mod download` first", path)
		}
		seen[path] = true
		mods = append(mods, module{Path: path, Version: version, Dir: dir})
	}

	slices.SortFunc(mods, func(a, b module) int { return strings.Compare(a.Path, b.Path) })
	return mods, nil
}

// licenseText finds a module's licence file. The name is not standardised —
// uniseg ships LICENSE.txt — so several spellings are tried before giving up.
func licenseText(dir string) (string, error) {
	for _, name := range []string{"LICENSE", "LICENSE.txt", "LICENSE.md", "COPYING", "COPYING.txt", "LICENCE"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil {
			return string(b), nil
		}
	}
	return "", fmt.Errorf("no licence file in %s", dir)
}

func versionLine(v string) string {
	if v == "" {
		return "_Vendored or replaced; version not reported by the build._\n"
	}
	return "Version `" + v + "`\n"
}

// fence wraps licence text in a code fence so Markdown renderers leave the
// wrapping and indentation of a legal notice exactly as written.
func fence(text string) string {
	return "```text\n" + strings.TrimRight(text, "\n") + "\n```\n"
}
