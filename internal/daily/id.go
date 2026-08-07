package daily

import (
	"crypto/sha256"
	"fmt"

	"github.com/nxck2005/surmise/internal/game"
)

// idVersion names the id derivation. Unlike seedVersion it may never be
// changed: the id is the key a daily is stored under, so a new one would orphan
// every daily already on disk and hand two versions of the app different codes
// for the same day.
//
// It carries no product name on purpose. This is a wire-format tag, not
// branding, and the two must not be confused: a name that appears in a hash is
// a name that can never be changed again. Renaming the project must never reach
// this string, which is why internal/daily does not import internal/brand and
// why TestDerivationTagsAreFrozen pins the literal.
const idVersion = "daily-id-v1"

// ID is the puzzle id for a day's puzzle in a mode.
//
// It is derived from public inputs only — never from the seed. That is what
// lets every player name the same puzzle, lets the id survive a change of
// Source, and lets an already-played daily be found on disk without asking any
// Source for anything (which is the whole of the offline story: resuming and
// reviewing never touch the network).
//
// The result is UUID-shaped like every other id, using version 8 — the
// "custom" version — since that is honestly what a derived id is.
func ID(d Day, length int) string {
	sum := sha256.Sum256(fmt.Appendf(nil, "%s|%s|%d", idVersion, d, length))
	return game.FormatID([16]byte(sum[:16]), 8)
}
