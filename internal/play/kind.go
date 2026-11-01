package play

import "github.com/nxck2005/surmise/internal/game"

// KindOf names where a puzzle came from (KindDaily, KindCustom and so on), by
// the same rule the history list labels a record with.
func KindOf(g *game.Game) string { return kindOf(g) }
