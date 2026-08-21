package main

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The option names are declared once, in main.go's const block, so the native
// flag and the browser query parameter for the same idea cannot drift. Both
// platform files consume them through their identifiers, which is what makes a
// rename a one-edit change — but nothing stopped an option from being wired
// into one platform and forgotten on the other, or a literal string sneaking
// past the constants. These tests read the sources as text and hold the line.
//
// They are deliberately dumb: no AST, no build tags, four files. The js file is
// read as text rather than compiled into the test because the test itself must
// build for both targets.

const (
	mainFile    = "main.go"
	nativeFile  = "main_native.go"
	jsFile      = "main_js.go"
	browserNote = "if this test failed, an option was added to one platform only, " +
		"or a literal bypassed the opt* constants; declare the name once in main.go " +
		"and use it through its identifier on both sides"
)

// browserOptions lists what a URL query string can express. The rest of the
// const block is native-only by nature: a browser has no data directory to
// redirect, no themes flag to list, no version or playtime to print without
// opening the app, and no files to export or import (the backup screen is its
// way in). An option belongs here when main_js.go reads it with get().
var browserOptions = []string{
	optTheme,
	optDay,
	optSplash,
	optMotion,
	optLength,
}

func declaredOptions(t *testing.T) map[string]string {
	t.Helper()
	b, err := os.ReadFile(mainFile)
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(?m)^\s*(opt\w+)\s*=\s*"([^"]+)"\s*$`)
	opts := map[string]string{}
	for _, m := range re.FindAllStringSubmatch(string(b), -1) {
		opts[m[2]] = m[1]
	}
	if len(opts) == 0 {
		t.Fatalf("no opt* constants found in %s — has the const block moved?", mainFile)
	}
	return opts
}

func readFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestBrowserOptionsExistOnBothPlatforms fails when an option the browser can
// express is missing from either side.
func TestBrowserOptionsExistOnBothPlatforms(t *testing.T) {
	opts := declaredOptions(t)
	native := readFile(t, nativeFile)
	js := readFile(t, jsFile)

	for _, name := range browserOptions {
		id, ok := opts[name]
		if !ok {
			t.Errorf("browser option ?%s= has no opt* constant in %s", name, mainFile)
			continue
		}
		if !strings.Contains(native, id) {
			t.Errorf("-%s is not registered in %s (%s never appears); %s",
				name, nativeFile, id, browserNote)
		}
		if !strings.Contains(js, id) {
			t.Errorf("?%s= is not read in %s (%s never appears); %s",
				name, jsFile, id, browserNote)
		}
	}
}

// TestNativeOptionsStayNative checks the complement: every constant that is not
// browser-expressible still reaches the flags, so adding one cannot silently
// land nowhere.
func TestNativeOptionsStayNative(t *testing.T) {
	opts := declaredOptions(t)
	native := readFile(t, nativeFile)

	browser := map[string]bool{}
	for _, name := range browserOptions {
		browser[name] = true
	}
	var names []string
	for name := range opts {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if browser[name] {
			continue
		}
		if !strings.Contains(native, opts[name]) {
			t.Errorf("-%s has a constant (%s) that %s never uses; %s",
				name, opts[name], nativeFile, browserNote)
		}
	}
}

// TestPlatformFilesUseNoOptionLiterals keeps the constants load-bearing: a
// quoted option name in a platform file is somebody reaching past the one
// declaration, and the two spellings would drift apart from then on.
func TestPlatformFilesUseNoOptionLiterals(t *testing.T) {
	opts := declaredOptions(t)
	for _, f := range []string{nativeFile, jsFile} {
		src := readFile(t, f)
		for name, id := range opts {
			literal := `"` + name + `"`
			if strings.Contains(src, literal) {
				t.Errorf("%s quotes %s literally; use %s instead — %s",
					f, literal, id, browserNote)
			}
		}
	}
}
