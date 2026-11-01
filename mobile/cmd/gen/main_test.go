package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/nxck2005/surmise/mobile/engine"
)

func TestGenerateIsStable(t *testing.T) {
	a, err := generate(engine.Methods(), engine.WireTypes())
	if err != nil {
		t.Fatal(err)
	}
	b, err := generate(engine.Methods(), engine.WireTypes())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("two runs wrote different Kotlin")
	}
	src := string(a)
	for _, m := range engine.Methods() {
		if !strings.Contains(src, "suspend fun "+funcName(m.Name)+"(") {
			t.Errorf("no Kotlin function for %s", m.Name)
		}
	}
	if !strings.Contains(src, "data class ImportReport(") {
		t.Error("a wire type outside the registry was left out")
	}
}

func TestNames(t *testing.T) {
	for in, want := range map[string]string{
		"board.submit":   "boardSubmit",
		"settings.get":   "settingsGet",
		"themes.tokens":  "themesTokens",
		"history.delete": "historyDelete",
	} {
		if got := funcName(in); got != want {
			t.Errorf("funcName(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"display_name": "displayName",
		"length":       "length",
		"in":           "`in`",
	} {
		if got := property(in); got != want {
			t.Errorf("property(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTwoStructsOfOneNameAreRefused(t *testing.T) {
	type Settings struct{ X int }
	g := &gen{structs: map[string]reflect.Type{}}
	if _, err := g.kotlinType(reflect.TypeFor[engine.Settings]()); err != nil {
		t.Fatal(err)
	}
	if _, err := g.kotlinType(reflect.TypeFor[Settings]()); err == nil {
		t.Fatal("a second struct named Settings was accepted")
	}
}
