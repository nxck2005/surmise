# Wortle — design notes & roadmap

This is the living design document. It records **why** things are the way they
are, the decisions that aren't obvious from reading the code, and what comes
next. `CLAUDE.md` is the short operational guide (commands, architecture map);
this file is the longer-form context. Append future plans here.

`IDEA.md` is the original brief and is intentionally loose — it warns that scope
will widen (eventual central server + global leaderboard) and asks not to lock
into a tight architecture. Honour that: keep the core UI-agnostic, keep storage
behind an interface, keep word length a variable.

---

## Current state (as of this writing)

Milestone 1 is **done and working**: a playable, persistent, typing-test-styled
Wordle TUI with a stats profile. All packages build, `go vet` is clean, and
`go test -race ./...` passes.

What exists:

- **4/5/6-letter modes**, chosen from the menu; the app opens straight onto a
  5-letter puzzle (menu is one `esc` away).
- **Resumable puzzles**, saved to disk after every guess, browsable in a list,
  reviewable when finished and resumable when not.
- **Profile screen**: win rate, avg attempts, avg solve time, current/max
  streak, guess-distribution histogram, and a per-mode breakdown.
- **Full mouse control**: every keybind has an on-screen target — the keyboard
  caps type (with `⏎`/`⌫`), rows and help-bar hints are buttons, and hover
  tracks the pointer. See "Mouse support" below.
- **btop-style UI**: a rounded, titled panel boxes the content and is centred in
  the terminal; near-black background; "serika" palette; on-screen
  keyboard rendered as filled keycaps; spaced, enlarged board tiles; a colour
  legend under the status line, drawn with the tile styles themselves and
  dropped when the terminal is too small for it.
- **Themes**: the whole look is data. 13 bundled themes, a picker with live
  preview, and user themes as one file each in `<data>/themes/*.toml`. See
  "Theme system" below and `docs/THEMES.md`.
- **Settings**: the mode the app opens on, and whether playing a mode makes it
  the default. `-length` overrides for one run. See "Settings" below.

---

## Environment / toolchain (verified)

- Go **1.26.5**. Module path `github.com/nxck2005/wortle`.
- Charm **v2** libraries under `charm.land/...` (NOT `github.com/charmbracelet/...`):
  - `charm.land/bubbletea/v2 v2.0.8`
  - `charm.land/lipgloss/v2 v2.0.5`
  - `bubbles` v2 was added early but is **not used** — `go mod tidy` drops it.
    Don't re-add it unless a component (textinput, viewport, list) actually
    earns its place.
- When unsure of a v2 API, **read the vendored source** under
  `~/go/pkg/mod/charm.land/` — v2 diverged from v1 and from most online
  examples. The traps that bit during milestone 1:
  - `View() tea.View` returns a **struct**, not a string. Content goes in
    `.Content`; alt-screen, bg/fg colour, window title are **fields on the
    `tea.View`** (see `internal/ui/app.go` `View()`), not program options.
  - Key messages are `tea.KeyPressMsg`, matched via `.String()`.
  - Module paths resolve only via `charm.land`; importing the github path fails.

---

## Architecture & the reasoning behind it

Layering is deliberate so a server can later drive the same core:

```
internal/game   rules, no I/O          ← pure, the eventual server shares this
internal/store  persistence, interface ← the seam for a remote backend
internal/words  embedded vocabulary
internal/stats  profile aggregation
internal/ui     Bubble Tea layer       ← the only package that knows about a terminal
```

### game — the rules
- `Game` is the **unit of persistence**: every field needed to resume is
  exported and JSON-tagged. Don't add un-exported state that a resumed puzzle
  would need.
- **Scoring (`score.go`) is the subtle part.** Duplicate letters force a
  **two-pass** algorithm: pass 1 claims exact matches and decrements a letter
  count; pass 2 hands out `Present` from what's left, so a letter is never
  marked more times than it occurs in the answer. This is why `ALLOY` vs `LOYAL`
  scores right. `score_test.go` has the densest, hand-verified table — **touch
  scoring only alongside those tests**, and hand-check expected values (several
  were wrong on the first pass from eyeballing).
- **Nothing hardcodes length 5.** `MaxAttempts = length + 1` (5/6/7). Keep new
  code length-agnostic.
- IDs are 8 random bytes from `crypto/rand` (hex) — no uuid dependency.

### store — the server seam
- `Store` is an **interface**; `JSON` is the only implementation. A remote
  backend for the leaderboard slots in here without the UI changing.
- One JSON file per puzzle under `os.UserConfigDir()/wortle/puzzles/`, plus
  `meta.json` for the number counter. **Atomic writes** (temp file + `os.Rename`
  in the same dir) so a crash mid-write can't corrupt a save.
- `All()` **skips unreadable files** rather than failing — one corrupt save must
  not lock the player out of their history. A corrupt `meta.json` rebuilds the
  counter from the puzzles on disk.
- Saved after **every guess**: a `kill -9` should cost nothing.

### words — embedded vocabulary
- Two lists per length via `go:embed`: `answers` (solution pool) and the larger
  `guesses` (accept list). **Every answer is also a valid guess** — enforced by
  a test; don't break it.
- Source of truth is `tools/genwords` (run by hand, not at build). Guesses come
  from **ENABLE1** (public-domain Scrabble dict → no proper nouns *by
  construction*, which a first-names blocklist failed to achieve). Answers are
  ENABLE ∩ a common-word frequency list. Provenance in `internal/words/data/SOURCES.md`.
- Counts: 4→3903/925, 5→8636/1178, 6→15232/1267 (guesses/answers).

### stats — recomputed, never stored
- `Compute` scans all games each time. Cheap at this scale and **can't drift**.
  Keep it that way rather than caching until scale actually demands it.
- Averages (attempts, time) cover **wins only** — a loss always uses every
  attempt and an abandoned game can sit open for hours, so including them makes
  the numbers meaningless. The `Summary` struct is **additive by design**;
  IDEA.md expects more metrics.
- Streaks ignore in-progress puzzles (having a game open doesn't break a streak).

### ui — the Bubble Tea layer
- One root `Model` (`app.go`) owns a `screen` enum and routes keys. Screens are
  **plain structs** (`gameScreen`, `listScreen`, `profileScreen`, inline menu)
  that render to strings and report intent back — **not** nested `tea.Model`s.
  This keeps all message plumbing in one place.
- `theme.go` turns a `theme.Theme` into lipgloss styles and holds `renderPanel`
  (the btop-style border). It is the **only** place a style is built from a
  colour. See "Theme system" below.
- `board.go` renders tiles and the keyboard. `format.go` has duration/percent
  helpers. `hit.go` holds mouse hit-testing (see below).

---

## UI layout model (read before touching the visuals)

Getting alignment right took several iterations. The rules that hold it
together:

- **Two levels of centring.** Each screen's `view()` centres its own sections
  relative to each other with `lipgloss.JoinVertical(lipgloss.Center, …)` (so
  the header and status line sit under the middle of the board). Then `app.go`
  `View()` wraps the whole thing in `renderPanel` and centres that panel in the
  terminal with `lipgloss.Place(width, height, Center, Center, …)`. If something
  looks left-aligned, it's almost always a `view()` still joining with
  `lipgloss.Left`.
- **`renderPanel` is hand-built**, not `lipgloss.Border()`, specifically so the
  title can be inlaid in the top rule (`╭─ wortle ─…╮`). It pads content, finds
  the widest line, pads the rest to match, then draws rounded border runes with
  the frame in muted grey and the title in accent.
- **Adjacent filled backgrounds must be separated by a gutter** or they merge
  into one bar. `joinTiles` (board) and the keyboard both insert a one-space
  gutter; `stackSpaced` puts a blank line between rows.
- **Height budget is real.** Tiles are **one row tall on purpose**. The tallest
  mode (6-letter → 7 attempts) with row gaps + keyboard + panel already
  approaches a 24-row terminal. Making tiles multi-line boxes overflows small
  terminals (`Place` will clip). If you want chunkier tiles, gate it on
  `m.height`.
- **The colour legend is the first thing to drop.** The board screen carries a
  one-line key (`A correct spot · A wrong spot · A not in word`) at the foot of
  the body, under the status line and spaced off it like every other section,
  because themes may repaint the three scored colours to anything and
  green/yellow is not a given. It is drawn with `st.tileCorrect` /
  `tilePresent` / `tileAbsent` themselves — not a parallel set of swatch styles
  — so a `[style.tile.*]` override or a wider `tile_width` moves the legend and
  the board together, and there is nothing to keep in sync. It is also the one
  optional row on the tallest screen, so `gameScreen.fits` drops it when the
  terminal cannot spare the height, or the width the themed tiles need. That is
  why the root pushes its size down via `gameScreen.resize`; an unmeasured size
  (before the first `WindowSizeMsg`) counts as unbounded.
- Colours: menu items and the panel title are accent; the selected menu row is
  flanked symmetrically (`› label ‹`) so the marker doesn't throw off centring.
  Both markers now come from the theme, and the centring width is measured from
  them rather than assuming two cells each.

---

## Theme system

The look used to be ~11 colour constants and ~25 styles derived from them at
package-init in `ui/theme.go`. That made restyling a rebuild, and — because the
styles were computed once from the colours — made a runtime palette change
impossible: reassigning `colorAccent` would not have updated `titleStyle`.

### Shape

- **`internal/theme`** owns the schema as data: a palette, glyphs, metrics, and
  per-element overrides. It has no Bubble Tea dependency (lipgloss only, for
  colour parsing), matching the rest of the UI-agnostic core.
- **`ui/theme.go`** builds a `styles` struct from a `theme.Theme`. The active
  one lives in a single package-level pointer, `st`, swapped wholesale by
  `setTheme`. One pointer rather than thirty variables because styles are
  *derived* — a palette change must rebuild all of them together — and because
  the swap is then atomic, which is what makes live preview possible.

### The file format, and why it is hand-rolled

Flat TOML: comments, `[section]` headers, `key = value`. Parsed by ~150 lines in
`parse.go` rather than by a TOML library. The schema is flat, so full TOML would
be unused, and this project has no non-Charm direct dependencies to spend (the
same reasoning that got bubbles removed). The readable subset is also what makes
a theme shareable: one file, obvious to edit, diffable.

Loading is **forgiving in the store's tradition** (a damaged counter is
recovered from, not fatal): a line that will not parse becomes a `Warning`
naming its line number and the rest of the theme still loads. `Parse` starts
from `Default()` and overlays, so a four-line theme file is valid.

Colour values take hex, an ANSI number 0-255, or the name of another palette
entry. The `terminal` bundled theme is built entirely from ANSI numbers, which
is what that third form is for.

Text-on-tile colours (`correct_text` and friends) are **resolved at read time,
not load time**, so a theme that sets only `bg` still moves the letters inside
filled tiles with it. That is the difference between light themes working and
not.

### Bundled themes

Embedded `themes/*.toml`, in exactly the format a user writes — so each is a
worked example, and `TestBundledThemesAreClean` holds them to it (no warnings, a
name, a complete palette). The colour-blind palette that used to be a roadmap
item is just `high-contrast.toml` now.

### Persistence and selection

`settings.json` sits beside `meta.json`, read/written through the narrow
`settingsStore` interface in `app.go` rather than by widening `store.Store` —
puzzles and preferences are different concerns, and a future remote store might
serve one without the other. Resolution order: `-theme` → `$WORTLE_THEME` →
`settings.json` → `serika dark`. A name that resolves to nothing is reported on
the error line, never swallowed.

Themes live under the data dir, so `-data` isolates the look as well as the
history. `EnsureDir` seeds an empty themes directory with `example.toml` — a
copy of the default with its `name` stripped (otherwise the copy would shadow
the built-in it came from, and editing it would look like the built-in had
changed).

### What themes may and may not do

Colours **must not move the layout**: hit regions are measured from the composed
frame, so anything that shifted a cell would move every click target with it.
`TestBundledThemesDoNotMoveTheLayout` strips ANSI and compares.

Glyphs and metrics *are* allowed to reshape things — that is the point of
`tile_width` — and because targets are measured rather than predicted, the mouse
follows for free (`TestClickTargetsFollowThemeMetrics` proves it at
`tile_width = 11`). Themes that change shape are exempted from the layout test
by comparing their `Glyphs`/`Metrics` against the default.

`st` is package state, so any test that calls `setTheme` must restore it —
`withTheme` in `theme_test.go` does.

---

## Settings

`settings.json` started as one field (the theme) written by the theme picker.
It now also carries the mode the app opens on, which needed somewhere to be
edited from — hence `settingsScreen`, the second thing on the menu that writes
preferences rather than puzzles.

### Two settings, not one

`Length` alone would have been ambiguous: is the default the mode you *set*, or
the mode you last *played*? Both are reasonable and neither is guessable, so
`RememberLast` makes the question explicit. With it on, choosing a mode from the
menu writes `Length` back (`applyChoice`), and the settings screen becomes a
readout of what you last played; with it off, the screen is the only thing that
moves the default. Off is the zero value, so a fresh install does the
predictable thing and nothing changes under a player who never opens the screen.

`Settings` is documented as safe to lose, and that now has to hold per field:
`Length == 0` means "nothing chosen" and resolves to `defaultLength`, so a
settings file written before the field existed is not an invalid mode. All
reads and writes go through `Model.settingsOf`/`saveSettings`, which
read-modify-write — two independent settings in one file must not clobber each
other, which is exactly the bug a `SaveSettings(Settings{Length: n})` would
have been.

### Why no commit step

The theme picker previews, so it needs enter to keep and esc to revert. Nothing
here is previewed — the mode you pick does not change what is on screen — so a
commit step would only be a way to lose a change. Every keypress saves, and esc
is just "back". `TestSettingsHaveNoCancel` pins that down, since the picker next
door sets the opposite expectation.

Resolution order matches the theme's — `-length` → `$WORTLE_LENGTH` →
`settings.json` → `defaultLength` — and an unsupported length is reported on the
error line rather than being fatal, the same as an unknown theme name. Both
overrides are for one run: they never write. The two now arrive as `ui.Options`
rather than positional arguments, so the next one is additive.

The step arrows are themeable glyphs (`value_prev` / `value_next`) rather than
literals at the call site, for the same reason every other glyph is. The value
and the `›` deliberately share one `action`, so the wide, easy target and the
arrow do the same thing — which is why the geometry test looks up the *last*
zone for that action to find the arrow itself.

## Mouse support (`hit.go`)

The rule is **parity**: anything the keyboard can do, a click can do. That is a
constraint on future work, not just a feature — a new keybind needs an on-screen
target, and `TestPlayingByClickingOnly` fails if the board stops being playable
with a mouse alone.

Parity needed new affordances, since several keybinds pointed at nothing on
screen. What was added: `⏎` and `⌫` caps on the on-screen keyboard (Wordle has
them too, and they fit — row 0 stays the widest at 59 cells, so the panel didn't
grow), an `×` inlaid at the right end of the panel's top rule, and the **bottom
help line reworked into the button bar** — each hint carrying an `action` is a
click target, which is why it can list every control without looking any
different from the plain hint line it replaced.

### Why marker-and-scan, and not geometry

Hit-testing has to answer "what is at cell (x, y)?", but by then the element has
been through **two levels of centring plus the panel** — and lipgloss rounds the
odd cell *left* when joining (`join.go`) and *right* when placing
(`position.go`). Re-deriving that arithmetic is possible, and would break
silently the first time a style changed.

So positions are measured, not predicted. `hitMap.mark` prefixes each clickable
atom with a zero-width APC escape carrying an id; `View` composes the frame as
before and then calls `hitMap.scan`, which finds where each marker actually
landed and strips them all before the string reaches Bubble Tea. The escape is
invisible to layout because `ansi.StringWidth` — what lipgloss measures with —
runs a full ANSI state machine and counts it as zero; terminals never see it
anyway. `TestMarkersDoNotAffectLayout` composes every screen with and without a
hit map and demands byte-identical output, so this can't silently drift.

The trade is one extra pass over the frame per render, which at this size is
nothing. It also means clickable regions cost nothing to maintain: mark the
atom, and moving it around the layout keeps working.

### Rules to keep

- **One behaviour, two inputs.** `Model.dispatch` never reimplements anything:
  it calls the same methods keys do (`typeLetter`/`deleteLetter`/`trimTo`,
  `submit`, `startNew`, `exit`, `applyChoice`, `openSelected`, `back`). Adding a
  click path means *extracting* an intent method, never copying a handler body.
  This is also why `applyChoice`/`openSelected` were split out of the key
  handlers.
- **Clicks obey the puzzle lifecycle** like keys do — the click path for a new
  puzzle goes through `startNew`, so `persisted` and the peek/commit split still
  hold (`TestHelpBarButtons` checks no phantom entry is saved).
- `action` is comparable so it doubles as the hover key; hover is stored on the
  root `Model` and carried into the next frame's `hitMap`.
- Hover uses `MouseModeAllMotion` (motion with no button held). On the menu and
  list the pointer moves the selection, which is why one click is enough to act;
  elsewhere `hoverStyle` (one line in `theme.go`) underlines the target.
- `View` writes `m.hits`, which `Update` reads. That is safe because Bubble Tea
  calls `View` synchronously right after `Update` on the same goroutine
  (`tea.go`), and `go test -race` covers it.

---

## Puzzle lifecycle (non-obvious, easy to break)

A fresh puzzle is **transient in memory until its first guess**:

- `newPuzzle` uses `store.PeekNumber()` (a non-committing peek) for a
  *prospective* `#N` shown in the header. It does **not** save.
- `gameScreen.persisted` starts false for a new puzzle, true for one loaded from
  the list. `leave()` **skips saving** when not persisted, so launching the app
  and quitting without playing leaves nothing behind and consumes no number.
- The **first guess** calls `store.NextNumber()` to commit the number, sets
  `persisted = true`, and saves. This keeps saved puzzles numbered contiguously
  (1, 2, 3…) with no gaps from abandoned puzzles, and keeps 0/6 entries out of
  the list.
- Elapsed time is banked per *session* (`AddElapsed` on leave / on the
  winning-or-losing guess), so idle time between sessions isn't counted.

If you add a new way to create or exit a puzzle, respect `persisted` and the
peek/commit split, or you'll get phantom 0/6 entries or number gaps.

---

## Testing approach

- **Core packages** are straightforwardly unit-tested. Scoring and the store's
  atomic/corruption behaviour have the most careful tests.
- **The UI is tested without a TTY** by driving the root `Model` with synthetic
  `KeyPressMsg`s and asserting on `View().Content` (`app_test.go`). The `key()`
  helper builds those messages; `TestKeyHelperMatchesFramework` guards that the
  synthetic keys match what the framework actually produces — **keep it passing
  if you extend the key helper.** `newModel` resets to the menu so tests can
  start games explicitly even though the app opens on a puzzle.
- **Mouse input is tested the same way** (`mouse_test.go`): `draw` renders at a
  fixed size, then `click`/`point` resolve a target through `hitMap.find` and
  send a synthetic `MouseClickMsg`/`MouseMotionMsg` at its centre, so no test
  hard-codes a coordinate. Two tests carry the weight:
  `TestMarkersDoNotAffectLayout` (marking changes no bytes) and
  `TestClickTargetsMatchGlyphs` (a recorded rect really covers the glyphs it
  claims — an off-by-one in `scan` fails it, which was checked by injecting one).
  To exercise the real input path, spawn the binary in a pty and write SGR mouse
  sequences: `\x1b[<0;COL;ROWM` + `…m` is a left click, `\x1b[<35;COL;ROWM` bare
  motion, both 1-based.
- **To eyeball real rendering**, drive the built binary in a sized pty, or write
  a throwaway Go test that sets `m.width/height` and prints `m.View().Content`.
  Strip ANSI with `sed -E 's/\x1b\[[0-9;]*m//g'`. The terminal bg/fg are set via
  OSC, not inline — grep the raw output for `\x1b]11;` / `\x1b]10;` to verify
  them. This is how the layout and colour work was checked during milestone 1.

---

## Roadmap / future work

Nothing below is committed to; it's the shape of where this goes.

### Near-term polish (small, safe)
- ~~`--length` / config for the default mode instead of the hardcoded 5~~ —
  done, as `Settings.Length` and the settings screen (see above).
- A confirm/delete action in the puzzle list (puzzles are currently only added).
- Hard/expert mode (revealed hints must be reused), per real Wordle.
- ~~Colour-blind palette toggle~~ — done, as the `high contrast` theme.
- Theme extras worth having once people write them: a `[style.*]` key for the
  panel title separate from the accent, and hot-reload of the themes directory
  while the picker is open.

### Larger (the IDEA.md ambitions)
- **Networked play / global leaderboard.** The `Store` interface is the seam:
  add a `RemoteStore` (HTTP) implementation, or a caching layer that syncs a
  local JSON store to a server. The `game` package is already server-safe (pure,
  no I/O) and can be shared by a backend. Keep scoring authoritative on the
  server if leaderboards ever matter competitively.
- **Lightweight "sign in" by ID** (IDEA.md deems full auth too heavy). A player
  ID could live in `meta.json` and tag uploaded puzzles.
- **Daily / seeded puzzles** (deterministic answer from a date seed) so players
  can compare the same board — a natural precursor to a leaderboard. Would need
  answer selection in `words` to accept a seed.

### Watch-outs when extending
- Don't hardcode length 5 anywhere.
- Don't move state the server would need out of the exported `Game`.
- Don't cache stats until scale demands it (recompute-from-truth is a feature).
- Don't let the UI reach past the `Store` interface to touch files directly.
