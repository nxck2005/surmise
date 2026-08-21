package brand

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNameIsACommandName(t *testing.T) {
	if Name == "" {
		t.Fatal("Name is empty")
	}
	if Name != strings.ToLower(Name) {
		t.Errorf("Name %q is not lowercase; it is a command name first", Name)
	}
	if strings.TrimSpace(Name) != Name || strings.ContainsAny(Name, " \t") {
		t.Errorf("Name %q has surrounding or embedded whitespace", Name)
	}
}

func TestRepoIsDerivedFromName(t *testing.T) {
	want := "github.com/nxck2005/" + Name
	if Repo != want {
		t.Errorf("Repo = %q, want %q — it is built from Name, so a rename moves both", Repo, want)
	}
}

func TestEnvPrefixesTheSetting(t *testing.T) {
	cases := []struct{ setting, want string }{
		{"THEME", "SURMISE_THEME"},
		{"LENGTH", "SURMISE_LENGTH"},
		{"DAY", "SURMISE_DAY"},
	}
	for _, c := range cases {
		if got := Env(c.setting); got != c.want {
			t.Errorf("Env(%q) = %q, want %q", c.setting, got, c.want)
		}
	}
}

// TestDailyDoesNotImportBrand pins the separation AGENTS.md states: daily's
// derivation tags are wire format, and a product name inside them is a name
// that can never change. The check is textual because the rule is about the
// import graph, which a package cannot assert about itself.
func TestDailyDoesNotImportBrand(t *testing.T) {
	files, err := filepath.Glob("../daily/*.go")
	if err != nil || files == nil {
		t.Fatalf("cannot read internal/daily: %v", err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), `"github.com/nxck2005/surmise/internal/brand"`) {
			t.Errorf("%s imports internal/brand; internal/daily must not know the product name", f)
		}
	}
}
