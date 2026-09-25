package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/nxck2005/surmise/internal/store"
)

// A combining mark is printable, so every text filter admits it, and it is zero
// cells wide, so it never advances a width counter. That pair is what makes a
// cell cap on its own no cap at all, and it is why the value these are named
// after is the one a settings file somebody else wrote can carry.
const combining = "́"

func repeated(r string, n int) string { return strings.Repeat(r, n) }

// A settings file on disk is repaired by this package rather than refused by
// the store, so the repair is worth a test of its own here rather than only
// through a drawn frame: a name made of a few thousand combining marks renders
// as nothing at all, so the only way to see that it was caught is to measure
// what came back.
func TestAnOverLongStoredNameIsRepairedNotDropped(t *testing.T) {
	hostile := repeated(combining, 20_000)
	if got := sanitizeDisplayName(hostile); len(got) > store.MaxSettingFieldBytes {
		t.Fatalf("repaired name is %d bytes against a cap of %d", len(got), store.MaxSettingFieldBytes)
	}
	// And the player's other preferences are not collateral: the repair is
	// per-field, which is the whole reason it lives in the text field.
	if got := sanitizeDisplayName("nick"); got != "nick" {
		t.Fatalf("an ordinary name was affected by the repair: %q", got)
	}
}

// The cell cap and the byte cap have to agree, and in this direction: a field
// that accepted more than the store would keep is a field whose value cannot be
// saved, which is a worse failure than one refused at the boundary.
func TestFieldByteCapIsNoLooserThanTheStore(t *testing.T) {
	if store.MaxSettingFieldBytes <= 0 {
		t.Fatalf("store.MaxSettingFieldBytes = %d, want a real bound", store.MaxSettingFieldBytes)
	}
	f := newTextField(1000, newDisplayNameField().filter)
	got := f.sanitize(repeated(combining, 10_000))
	if len(got) > store.MaxSettingFieldBytes {
		t.Fatalf("a field accepted %d bytes against the store's cap of %d",
			len(got), store.MaxSettingFieldBytes)
	}
}

// The write path, which is a different loop from the read path and has to hold
// the same bound: a paste is the fastest way to put a megabyte of nothing into
// a field, and it arrives as one keypress.
func TestTypeTextIsBoundedByBytesToo(t *testing.T) {
	f := newTextField(1000, newDisplayNameField().filter)
	f.begin()
	f.typeText(repeated(combining, 10_000))
	if len(f.value) > store.MaxSettingFieldBytes {
		t.Fatalf("a paste of zero-width runes left %d bytes in the field, cap is %d",
			len(f.value), store.MaxSettingFieldBytes)
	}
	if width := lipgloss.Width(f.value); width > 1000 {
		t.Fatalf("the value is %d cells wide, over the field's 1000", width)
	}
}

// A bound that truncates everything to nothing would pass both tests above, so
// the ordinary cases are asserted alongside them.
func TestFieldCapsLeaveOrdinaryTextAlone(t *testing.T) {
	f := newDisplayNameField()
	for _, in := range []string{"nick", "abcdefghijklmnopqrs", "ni ck"} {
		if got := f.sanitize(in); got != in {
			t.Errorf("sanitize(%q) = %q, want it unchanged", in, got)
		}
	}

	g := newTextField(19, newDisplayNameField().filter)
	g.begin()
	g.typeText("abcdefghijklmnopqrstuvwxyz")
	if g.value != "abcdefghijklmnopqrs" {
		t.Errorf("typing into a 19-cell field left %q, want the first 19 cells", g.value)
	}
}

// A cell cap still does its own job: a wide rune must be refused on width even
// when there is byte room to spare, or the byte cap would have replaced it
// rather than sat beside it.
func TestFieldStillCapsByWidth(t *testing.T) {
	// A CJK ideograph is one rune and two cells.
	f := newTextField(6, func(r rune) (rune, bool) { return r, true })
	got := f.sanitize(repeated("漢", 10))
	if width := lipgloss.Width(got); width > 6 {
		t.Fatalf("a 6-cell field took %d cells: %q", width, got)
	}
	if width := lipgloss.Width(got); width != 6 {
		t.Fatalf("a 6-cell field took %d cells, want it filled: %q", width, got)
	}
}
