# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Wortle: a Wordle TUI in Go, styled after minimal typing-test UIs, built with
Bubble Tea.
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
go run . -themes         # list themes (and their parse warnings) and exit
go run . -theme dracula  # start with a theme, without changing the saved choice
go run . -length 6       # start in a mode, without changing the saved default
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
- Key messages are **`tea.KeyPressMsg`**, matched via `.String()`. Mouse
  messages are `tea.MouseClickMsg` / `MouseMotionMsg` / `MouseWheelMsg`, and the
  mouse is enabled by the **`MouseMode` field on `tea.View`**, not a program
  option.

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
  (temp file + rename). The game is saved after every guess. Deleting a
  **finished** puzzle overwrites its file with a **tombstone** (`game.Tombstone`:
  id, length, status, updatedAt, `deleted:true` — the answer and guesses are
  destroyed) so streaks still see the break; an unfinished one is unlinked.
  `Load` reports a tombstone as `ErrNotFound` and `List` omits it; **`All`
  returns them**, because stats need them. There is
  deliberately **no index and no counter**: a puzzle's displayed code comes from
  its own id, so the store allocates nothing that `Delete` could leave a hole in.
  An old install may still have a `meta.json` holding the retired counter; it is
  never read.

- **`internal/words`** — vocabulary compiled in via `go:embed`. Two lists per
  length: `answers` (solution pool) and the larger `guesses` (accept list);
  every answer is also a valid guess (enforced by a test). Regenerate with
  `tools/genwords`; provenance in `data/SOURCES.md`.

- **`internal/stats`** — recomputes the profile from all saved games each time
  (cheap at this scale, never drifts). Averages cover wins only. Deleted records
  are skipped by every counter and used **only** by the streak walk, where a
  deleted loss still breaks a run and a deleted win neither extends nor breaks
  one — that is what stops deleting a loss from raising `MaxStreak`.

- **`internal/theme`** — the styleable surface **as data**: palette, glyphs,
  metrics and per-element overrides, with a hand-rolled reader for the flat
  TOML-ish theme file format (no TOML dependency). Bundled themes are embedded
  `themes/*.toml` in exactly the format users write, so each doubles as a worked
  example. Bad input is a `Warning` naming the line, never fatal — the picker
  and `-themes` show them.

- **`internal/ui`** — one root `Model` (`app.go`) owns a screen enum and routes
  keys to screen structs (`gameScreen`, `listScreen`, `profileScreen`,
  `themeScreen`, `settingsScreen`, and an inline menu). Screens are plain structs that render to
  strings and report intent back to the root, not nested `tea.Model`s.
  `theme.go` turns a `theme.Theme` into lipgloss styles and holds `renderPanel`
  (the btop-style rounded, titled border); `board.go` renders tiles, the
  keyboard and the colour legend. Per `IDEA.md`, a new puzzle is **tab then
  enter**. `hit.go` carries
  mouse support (below).

## Settings

`store.Settings` (`settings.json`) is the whole of the persisted preferences:
`Theme`, `Length` (the mode the app opens on) and `RememberLast`. **Every field's
zero value means "nothing chosen"** — `Length == 0` resolves to `defaultLength`,
never to an invalid mode — so an older settings file stays readable. Resolution
for the mode mirrors the theme's: `-length` → `$WORTLE_LENGTH` → `settings.json`
→ `defaultLength`, and an unsupported value is reported on the error line rather
than being fatal (`applyStartupLength`).

Overrides reach the UI through **`ui.Options`**, whose zero value means "use what
was saved"; add a field there rather than another argument to `ui.New`. Read and
write preferences through `Model.settingsOf`/`saveSettings` (read-modify-write,
so the theme and the mode never clobber each other), not by reaching for the
`settingsStore` interface directly.

## Theming

Themes are data, not code. `internal/theme` owns the schema; `ui/theme.go` is
the only place that builds a lipgloss style from a colour. Adding a themeable
element means adding a field to `styles` and a name to `theme.Elements` — never
a new colour constant at a call site.

The active look is **one package-level pointer**, `st`, swapped wholesale by
`setTheme`. Styles are derived from the palette, so a palette change has to
rebuild all of them at once; that is also what makes live preview in the picker
work. Read `st` at render time, never capture a style at init.

- Persisted in `settings.json` beside `meta.json`, via the narrow
  `settingsStore` interface in `app.go` (`store.Store` stays about puzzles).
  `-theme` and `$WORTLE_THEME` override for one run.
- Colours must not move the layout: `TestBundledThemesDoNotMoveTheLayout` strips
  ANSI and compares. Glyph and metric changes *are* allowed to reshape things —
  hit regions are measured, so click targets follow
  (`TestClickTargetsFollowThemeMetrics`).
- `st` is package state, so a test that calls `setTheme` must restore it;
  use `withTheme` (`theme_test.go`).
- User-facing reference: `docs/THEMES.md`.

## Mouse

**Anything the keys can do, a click can do too** — keep it that way when adding
a keybind. The parity pieces: the on-screen keyboard types (its `⏎`/`⌫` caps
submit and delete), menu choices and list rows are clicked directly, a typed
tile is clicked to erase back to it, theme rows preview on hover and commit on
click, the `×` in the panel border quits, and the
**bottom help line doubles as the button bar** — each hint with an `action` is
a click target.

Positions can't be predicted through the two levels of centring, so `hit.go`
**measures them**: `hitMap.mark` prefixes each clickable atom with a zero-width
APC escape, and `View` calls `hitMap.scan` on the finished frame to record where
the markers landed, stripping them before the string reaches Bubble Tea. Hence
`view()`/`help()` take a `*hitMap` (nil-safe — nil marks nothing).

- A click resolves to an `action`, and `Model.dispatch` routes it into the same
  intent methods the key handlers use (`typeLetter`, `submit`, `applyChoice`,
  `openSelected`, `deleteSelected`, `back`…). **Never let the two paths
  diverge**: add the behaviour as a method, then call it from both.
- **Destructive actions confirm on both paths.** A click skips the board's
  tab-then-enter confirm (`actNewPuzzle`) because starting a puzzle is undone by
  starting another. Deleting is not undoable, so `actDeletePuzzle` arms the same
  prompt a key does and the prompt's own target — elsewhere on screen — carries
  it out.
- `action` is comparable and doubles as the hover key; hover highlighting is one
  cue in one place (`st.hover` in `theme.go`, themeable as `[style.hover]`).
- Marking must never move anything, which
  `TestMarkersDoNotAffectLayout` enforces by composing each screen with and
  without a `hitMap` and comparing bytes.

## UI layout

Centring happens at **two levels**: each screen's `view()` centres its own
sections with `lipgloss.JoinVertical(lipgloss.Center, …)`, then `app.go`
`View()` boxes the result in `renderPanel` and centres that panel in the
terminal with `lipgloss.Place`. If something looks left-aligned, a `view()` is
probably still joining with `lipgloss.Left`. Adjacent filled backgrounds need a
gutter or they merge (`joinTiles`, `stackSpaced`). Board tiles are **one row
tall on purpose** — the tallest mode plus keyboard and panel already nears a
24-row terminal, so don't make them multi-line without gating on `m.height`.

The **colour legend** at the foot of the board is the worked example of that
gating, and the pattern to copy for any further optional row: the root pushes
the terminal size down with `gameScreen.resize` (from `New`, `openGame` and
every `WindowSizeMsg`), and `gameScreen.fits` drops the legend when the height
or the width the themed tiles need is not there. An unmeasured size (before the
first `WindowSizeMsg`) counts as unbounded, which is what keeps the headless
tests rendering the whole screen. The legend renders through `st.tileCorrect` /
`tilePresent` / `tileAbsent` themselves — never a parallel set of swatch styles,
or the key and the board it explains can disagree.

More detail and rationale in `PLAN.md`.

## Puzzle identity

A puzzle is identified by a **UUIDv4** (`game.newID`, hand-rolled — no
dependency). What the player sees is `game.Code(id)`: **six digits derived by
hashing the id**, e.g. `#042317`. The code is a **label, never a key** — six
digits is a space of a million, so `Code` is not injective; everything looks a
puzzle up by `ID`. `Code` accepts any id string, so saves written before UUIDs
existed still render (no migration).

Deriving the code from the id rather than from a counter is what makes deletion
safe: nothing is allocated, so removing a puzzle disturbs no other puzzle. It is
also the groundwork for daily puzzles — a puzzle whose id is derived
deterministically from a date shows every player the same code, with nothing to
coordinate.

`newPuzzle` re-rolls the id a few times if the code is already in the local list,
then gives up and accepts a duplicate (cosmetic, never fatal). **Only the random
path re-rolls**: a seeded/daily constructor must keep the id its seed determines.

## Puzzle lifecycle

A new puzzle is **transient in memory until its first guess**: `newPuzzle` does
not save, and `gameScreen.persisted` is false until the first guess, which sets
it and saves. So launch-and-quit saves nothing and 0/6 entries never reach the
list. Respect `persisted` in any new create/exit path. Its code, unlike the old
`#N`, is final from creation — there is nothing prospective about it.

At the other end, deleting a finished puzzle **leaves a tombstone rather than
nothing** (see the store bullet above). The profile still recomputes from what is
on disk; the tombstone is what keeps deleting a loss from merging the win runs
either side of it. Any new path that removes a puzzle must go through
`store.Delete`, never `os.Remove`.

## Testing note

The UI is tested by driving the root `Model` with synthetic `KeyPressMsg`s and
asserting on `View().Content` (`internal/ui/app_test.go`) — no TTY needed.
`TestKeyHelperMatchesFramework` guards that the synthetic keys match what the
framework actually produces; keep it passing if you extend the key helper.
`newModel` resets to the menu so tests start games explicitly, even though the
app itself opens on a puzzle.

Mouse tests (`internal/ui/mouse_test.go`) work the same way: `draw` sizes the
terminal and renders, then `click`/`point` look the target up with
`hitMap.find` and send a `MouseClickMsg`/`MouseMotionMsg` at its centre — so no
test hard-codes coordinates. `TestClickTargetsMatchGlyphs` is the geometry
guard: it checks a recorded rect really covers the glyphs it claims (an
off-by-one in `scan` fails it).

To drive real mouse input end to end, spawn the binary in a pty and write SGR
sequences to it: `\x1b[<0;COL;ROWM` then `…m` is a left click, `\x1b[<35;COL;ROWM`
is bare motion (coordinates 1-based).

To eyeball real rendering, set `m.width/height` and print `m.View().Content`
from a throwaway test, or drive the built binary in a sized pty; strip ANSI with
`sed -E 's/\x1b\[[0-9;]*m//g'`. Background/foreground are emitted as OSC
sequences (`\x1b]11;` / `\x1b]10;`), not inline, so grep the raw output to check
them.
