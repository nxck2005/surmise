package ui

import (
	"strings"
	"unicode"
)

// safeText replaces control characters in text that reaches the frame without
// having been validated at its source: an error from a store, a file or a URL,
// a clipboard failure, a warning.
//
// Bytes that are persisted — words, daily dates, ids, theme values — are
// refused at their own boundary instead (game.Validate, theme.Parse,
// store.decodeRecord), so this is the backstop for prose, not the first line of
// defence. It repairs rather than refuses because an error is the reader's,
// not the author's: losing a whole message over one bad character would hide
// the thing it is reporting. U+FFFD rather than dropping the rune, so a
// tampered message looks wrong instead of looking fine.
//
// The rule is theme.safeText's deliberately: one idea of what a terminal must
// never receive, whether it is a filename in the picker or an error on the
// status line. The two are kept apart rather than shared because each belongs
// with the layer that knows why it exists — this one never sanitises layout,
// only the strings handed to it.
func safeText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '�'
		}
		return r
	}, s)
}
