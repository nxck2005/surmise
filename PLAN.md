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

Milestone 1 is **done and working**: a playable, persistent, monkeytype-styled
Wordle TUI with a stats profile. All packages build, `go vet` is clean, and
`go test -race ./...` passes.

What exists:

- **4/5/6-letter modes**, chosen from the menu; the app opens straight onto a
  5-letter puzzle (menu is one `esc` away).
- **Resumable puzzles**, saved to disk after every guess, browsable in a list,
  reviewable when finished and resumable when not.
- **Profile screen**: win rate, avg attempts, avg solve time, current/max
  streak, guess-distribution histogram, and a per-mode breakdown.
- **btop-style UI**: a rounded, titled panel boxes the content and is centred in
  the terminal; near-black background; monkeytype "serika" palette; on-screen
  keyboard rendered as filled keycaps; spaced, enlarged board tiles.

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
- `theme.go` holds the **entire palette** and shared style helpers, plus
  `renderPanel` (the btop-style border). Restyling is a one-file change.
- `board.go` renders tiles and the keyboard. `format.go` has duration/percent
  helpers.

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
- Colours: menu items and the panel title are accent; the selected menu row is
  flanked symmetrically (`› label ‹`) so the marker doesn't throw off centring.

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
- **To eyeball real rendering**, drive the built binary in a sized pty, or write
  a throwaway Go test that sets `m.width/height` and prints `m.View().Content`.
  Strip ANSI with `sed -E 's/\x1b\[[0-9;]*m//g'`. The terminal bg/fg are set via
  OSC, not inline — grep the raw output for `\x1b]11;` / `\x1b]10;` to verify
  them. This is how the layout and colour work was checked during milestone 1.

---

## Roadmap / future work

Nothing below is committed to; it's the shape of where this goes.

### Near-term polish (small, safe)
- Optional `--length` / config for the default mode instead of the hardcoded 5
  (`defaultLength` in `app.go`).
- A confirm/delete action in the puzzle list (puzzles are currently only added).
- Hard/expert mode (revealed hints must be reused), per real Wordle.
- Colour-blind palette toggle (Wordle's blue/orange scheme).

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
