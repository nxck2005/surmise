# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Wortle: a Wordle TUI in Go, styled after monkeytype, built with Bubble Tea.
Word-length modes (4/5/6) are the difficulty axis. Puzzles are resumable and
saved to disk; there is a stats/profile screen. The app opens straight onto a
5-letter puzzle (menu is one `esc` away).

`IDEA.md` is the original brief. **`PLAN.md` is the living design + roadmap doc**
— the longer-form "why", the non-obvious decisions, and future work; read it
before a substantial change and append plans to it. Both note that scope is
expected to widen (eventual central server + global leaderboard), so avoid
locking into a tight design.

## Commands

```sh
go run .                 # play; add -data <dir> to use a scratch save location
go build ./...
go test ./...
go test -race ./internal/...
go run ./tools/genwords  # regenerate the embedded word lists (writes internal/words/data)
```

Run a single scoring case: `go test ./internal/game -run TestScore/double_in_guess -v`

## Dependencies — Charm v2

Uses the **v2** Charm libraries, whose module paths are **`charm.land/...`**, not
`github.com/charmbracelet/...` (the go.mod for v2 declares the charm.land path;
importing the github path fails to resolve). v2 differs from v1 in ways that
break copied v1 examples:

- `Model.Init() tea.Cmd`, `Update(Msg) (Model, Cmd)`, and **`View() tea.View`** —
  `View` returns a struct, not a string. Build content with a `tea.View` value
  and set `.Content`; alt-screen, background/foreground colour and window title
  are **fields on `tea.View`** (see `internal/ui/app.go`), not program options.
- Key messages are **`tea.KeyPressMsg`**, matched via `.String()`.

When unsure of a v2 API, read the module source under
`~/go/pkg/mod/charm.land/` rather than guessing from memory.

## Architecture

The core (`internal/game`, `store`, `words`, `stats`) is UI-agnostic and fully
unit-tested; `internal/ui` is the Bubble Tea layer on top. This separation is
deliberate — the same game/store can later be driven by a server.

- **`internal/game`** — the rules, with no I/O. `Game` is the unit of
  persistence (all state exported + JSON-tagged). `score.go` is the subtle part:
  duplicate-letter scoring is a **two-pass** algorithm (claim exact matches
  first, then hand out remaining letters), and it has the densest tests — touch
  it only alongside `score_test.go`. Nothing hardcodes length 5.

- **`internal/store`** — `Store` is an **interface**; `JSON` is the only
  implementation today. This interface is the seam for a future remote backend.
  One file per puzzle under the user config dir, written atomically
  (temp file + rename). The game is saved after every guess. `NextNumber`
  commits the puzzle counter; `PeekNumber` reads it without committing (see the
  puzzle lifecycle below).

- **`internal/words`** — vocabulary compiled in via `go:embed`. Two lists per
  length: `answers` (solution pool) and the larger `guesses` (accept list);
  every answer is also a valid guess (enforced by a test). Regenerate with
  `tools/genwords`; provenance in `data/SOURCES.md`.

- **`internal/stats`** — recomputes the profile from all saved games each time
  (cheap at this scale, never drifts). Averages cover wins only.

- **`internal/ui`** — one root `Model` (`app.go`) owns a screen enum and routes
  keys to screen structs (`gameScreen`, `listScreen`, `profileScreen`, and an
  inline menu). Screens are plain structs that render to strings and report
  intent back to the root, not nested `tea.Model`s. `theme.go` holds the entire
  palette plus `renderPanel` (the btop-style rounded, titled border);
  `board.go` renders tiles and the keyboard. Per `IDEA.md`, a new puzzle is
  **tab then enter**.

## UI layout

Centring happens at **two levels**: each screen's `view()` centres its own
sections with `lipgloss.JoinVertical(lipgloss.Center, …)`, then `app.go`
`View()` boxes the result in `renderPanel` and centres that panel in the
terminal with `lipgloss.Place`. If something looks left-aligned, a `view()` is
probably still joining with `lipgloss.Left`. Adjacent filled backgrounds need a
gutter or they merge (`joinTiles`, `stackSpaced`). Board tiles are **one row
tall on purpose** — the tallest mode plus keyboard and panel already nears a
24-row terminal, so don't make them multi-line without gating on `m.height`.
More detail and rationale in `PLAN.md`.

## Puzzle lifecycle

A new puzzle is **transient in memory until its first guess**: `newPuzzle` peeks
a prospective `#N` (`PeekNumber`) and does not save; `gameScreen.persisted` is
false until the first guess, which commits the number (`NextNumber`) and saves.
So launch-and-quit saves nothing, saved puzzles are numbered contiguously, and
0/6 entries never reach the list. Respect `persisted` and the peek/commit split
in any new create/exit path.

## Testing note

The UI is tested by driving the root `Model` with synthetic `KeyPressMsg`s and
asserting on `View().Content` (`internal/ui/app_test.go`) — no TTY needed.
`TestKeyHelperMatchesFramework` guards that the synthetic keys match what the
framework actually produces; keep it passing if you extend the key helper.
`newModel` resets to the menu so tests start games explicitly, even though the
app itself opens on a puzzle.

To eyeball real rendering, set `m.width/height` and print `m.View().Content`
from a throwaway test, or drive the built binary in a sized pty; strip ANSI with
`sed -E 's/\x1b\[[0-9;]*m//g'`. Background/foreground are emitted as OSC
sequences (`\x1b]11;` / `\x1b]10;`), not inline, so grep the raw output to check
them.
