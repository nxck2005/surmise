# Puzzle identity rework + delete from the list

## Context

`PLAN.md` has long listed "a confirm/delete action in the puzzle list (puzzles are
currently only added)". Planning it surfaced the real blocker: **puzzle numbers are a
committed, sequential resource.** `#N` comes from a counter in `meta.json`
(`NextNumber`/`PeekNumber`), and the entire "transient until first guess" lifecycle —
peek a prospective number, commit it on the first guess — exists *only* to keep that
sequence gap-free. Deleting a puzzle punches a hole in it, and every way of patching
that up is bad: leave gaps (the invariant the lifecycle was built to protect is now
broken anyway), renumber (a puzzle's identity changes under it), or reuse (two puzzles
share a number over time).

So the fix is upstream of delete. Each puzzle gets a **UUID**, and the visible ID
becomes a **6-digit code derived from that UUID by hashing** — a pure function of
identity, not a position in a sequence. Nothing is allocated, so nothing can be left
with a hole in it, and delete becomes `os.Remove`.

Two things fall out of this:

- **The counter subsystem disappears.** `meta.json`, `metaFile`,
  `highestNumberOnDisk`, the counter mutex, `NextNumber`, `PeekNumber`,
  `Game.Number`, and the peek/commit half of the puzzle lifecycle all go. That is a
  net deletion across `internal/store`, `internal/game` and `internal/ui`.
- **Daily puzzles get their groundwork.** Because the code is `hash(id)` and nothing
  else, a future daily whose UUID is derived deterministically from a date yields the
  *same* 6-digit code for every player, with no server and no coordination. The
  hashing step is what makes that work — a date-derived UUID sliced directly would
  expose its structure; hashed, it is well-distributed and opaque.

Trade-off worth naming: `#7` told you this was your seventh puzzle. `#042317` does not.
The list is already ordered by recency and shows elapsed time, so the ordinal was
carrying little weight, but it is a real loss and it is deliberate.

Scope confirmed with the user: **deleting is possible from the puzzle list only** —
not the game or review screen.

---

## Phase 1 — identity: UUID + derived code

### `internal/game/game.go`

Replace `newID` (currently 8 random bytes hex) with a canonical **UUIDv4**: 16 bytes
from `crypto/rand`, version and variant bits set, formatted `8-4-4-4-12`. Written
inline — about a dozen lines — so `PLAN.md`'s "no uuid dependency" stance holds, in the
same spirit as the hand-rolled TOML reader in `internal/theme`.

Add the code derivation beside it:

```go
// Code is the short, human-facing form of a puzzle id: six digits derived by
// hashing, e.g. "042317". It is a label, never a key — Code is not injective,
// so nothing may look a puzzle up by it. The id is the only identity.
//
// Deriving it by hash rather than by counting means it depends on nothing but
// the puzzle itself, so puzzles can be deleted without disturbing any other
// puzzle's code — and a puzzle whose id is derived deterministically (a daily,
// seeded from its date) yields the same code for every player.
func Code(id string) string {
    sum := sha256.Sum256([]byte(id))
    return fmt.Sprintf("%06d", binary.BigEndian.Uint64(sum[:8])%1_000_000)
}
```

It takes any `string`, so **existing saves need no migration** — their current 16-hex
ids hash to codes exactly as UUIDs do.

Drop the `Number int` field from `Game`. `New(length, number int)` loses its second
parameter and its comment about the store supplying it. Old JSON files keep a stray
`"number"` key, which `encoding/json` ignores on decode — that is the whole migration.
`Validate` is unaffected (it never checked `Number`).

#### Collisions — decided, not open

Six digits is a space of 1,000,000. By the birthday bound that is ~0.5% chance of a
collision within a 100-puzzle history and ~39% within 1,000. Two mitigations, both
needed:

1. **The code is a label, never a key.** Every lookup path — `pathFor`, `Load`,
   `Save`, the new `Delete`, the list rows — already keys on `ID`, and must stay that
   way. The code is only ever rendered.
2. **Re-roll on local collision at creation.** `newPuzzle` already makes one store call
   (`PeekNumber`), so swapping it for a `List()` collision check is like-for-like: if
   the new id's code matches an existing puzzle's, generate another id, capped at a
   handful of attempts, then accept a duplicate rather than failing. Cheap, and it
   keeps *your own* list unambiguous.

This guarantees local uniqueness only. If a future leaderboard ever makes codes
cross-player identifiers, the code stops being sufficient and the UUID has to travel
with it — worth a note in `PLAN.md`. The re-roll belongs in the **random** creation
path only: a future seeded/daily constructor must not re-roll, since its whole point is
that the id is determined by the date.

### `internal/store`

- `store.go` — drop `NextNumber` and `PeekNumber` from the `Store` interface, drop
  `Number` from `Summary` and from `summarize`. Add `Delete` (Phase 2).
- `json.go` — delete `metaFile`, `metaName`, `readMeta`, `highestNumberOnDisk`,
  `NextNumber`, `PeekNumber`, and the counter mutex `s.mu` (its only job was guarding
  the counter's read-modify-write). `NewJSON` no longer reads meta at open.

An existing install's `meta.json` is simply left on disk, unread. Deleting a user's
file to tidy up buys nothing and is the sort of destructive step that should not ride
along in an unrelated change.

### `internal/ui`

- `newPuzzle` (`gamescreen.go:244`) — drop the `PeekNumber` call; it becomes
  `game.New(length)` plus the collision re-roll.
- `submit` (`gamescreen.go:203-213`) — the "reserve its number now" block collapses to
  `m.persisted = true`. **`persisted` itself stays**: it still decides whether `leave()`
  saves, so launching the app and quitting still leaves nothing behind and no 0/6 entry
  reaches the list. What goes away is only the peek/commit half.
- Header (`gamescreen.go:258`) — `wortle #%d` on `g.Number` becomes `wortle #%s` on
  `game.Code(g.ID)`. It is no longer "prospective": a new puzzle shows its final code
  from the moment it exists.
- Row (`listscreen.go:135`) — `#%-5d` becomes the code. Codes are **fixed-width**, so
  the column is now exactly aligned rather than padded to fit a growing number.

### Docs

`CLAUDE.md` and `PLAN.md` both document the peek/commit split as a load-bearing
invariant ("Puzzle lifecycle", in both files) and `PLAN.md` describes the counter under
"store — the server seam". Both need rewriting to describe identity-by-hash, or the
next change will be planned against a lifecycle that no longer exists. `README.md`'s
feature list mentions saved puzzles but not numbering — check it in passing.

---

## Phase 2 — delete from the puzzle list

Trivial once Phase 1 lands, because there is no longer any allocated state to reconcile.

### Store

`Delete(id string) error` on the interface, implemented as `os.Remove(s.pathFor(id))`,
mapping `fs.ErrNotExist` to `ErrNotFound` to match `Load`'s contract
(`json.go:139-141`) and wrapping other errors in the file's `"store: verb noun: %w"`
house style. No atomic-write analogue is needed — `os.Remove` is already atomic. No
cache to invalidate: `List`/`All` re-read the directory every call.

### Confirm UX

The codebase has exactly one confirm precedent — `gameScreen.confirmNew`, an inline
arm-then-confirm on the status line (`gamescreen.go:30-32`, `113-123`, `307-317`).
Follow it: a `confirmDelete bool` on `listScreen`, a pre-switch at the top of
`update` that consumes the next key, and an inline prompt rendered in `view()` with
two marked halves.

**One deliberate divergence.** For a new puzzle, a click bypasses the confirm entirely
(`app.go:322-328`: "a click is already deliberate"). Delete is destructive and
irreversible, so the click path **also** arms the prompt, and the prompt's own target
completes it. Mis-clicking a row in a list is easy; mis-clicking twice, on two
different targets, is not. Worth a comment at the divergence so it doesn't read as an
inconsistency.

### UI wiring

| Change | Location |
| --- | --- |
| `confirmDelete` field + pre-switch; bind `d` | `listscreen.go:21-26`, `43-62` |
| Prompt line in `view()` (guard the empty-list early return at `:106`) | `listscreen.go:97-125` |
| `helpItem{keys: "d", label: "delete", act: …}` → click target for free | `listscreen.go:163-169` |
| Append `actDeletePuzzle`, `actCancelDelete` (append, don't insert — `iota` ordering) | `hit.go:40-55` |
| `dispatch` branches, mirroring `actListRow` | `app.go:280-285` |
| `updateList` — `update` returns `(open, back)` today; widen to `(open, del, back)`, matching the established two-bool tuple shape | `app.go:451-460` |
| `deleteSelected()` on `Model`, mirroring `openSelected` | `app.go:502-515` |
| Hover on `actDeletePuzzle` should move the list cursor, like `actListRow` | `app.go:246-261` |

Per `CLAUDE.md`'s mouse rule, `dispatch` calls `deleteSelected()` — the same method the
key path calls. Never two implementations.

**Cursor preservation:** `reload` resets `cursor, offset = 0, 0`
(`listscreen.go:28-31`), which would fling the selection to the top after every delete.
After deleting, reload and then restore the cursor clamped to the new length, followed
by `clampOffset()`.

**Clear `confirmDelete`** on any cursor movement, on `esc`, and on leaving the screen —
an armed prompt must never survive the row it was armed on.

### Stats

**No work required, by design.** `stats.Compute` is recomputed from `All()` on every
profile open (`profilescreen.go:25-33`), so a deleted puzzle simply stops counting.

One surprising-but-correct effect to note in a comment: `streaks` (`stats.go:126-149`)
counts runs of wins over finished puzzles ordered by `UpdatedAt`, so **deleting a loss
from the middle of history merges two win runs** — `MaxStreak` can go *up* after a
delete. Correct by construction, but it will look like a bug if nobody wrote it down.

---

## Verification

**Automated**

- `go build ./...`, `go vet ./...`, `go test ./...`, `go test -race ./internal/...`
- New in `internal/game`: `Code` is deterministic and always 6 digits (including
  leading-zero cases); ids are well-formed UUIDv4; the creation re-roll avoids a
  planted local collision.
- New in `internal/store` (in-package, `NewJSON(t.TempDir())` per `json_test.go:12-32`):
  delete removes the file, delete of an unknown id returns `ErrNotFound` (template at
  `json_test.go:70-75`), delete leaves other puzzles and `List` intact.
- New in `internal/ui`: delete by key (`d` then confirm), delete by click, cancel
  leaves the puzzle present, delete on an empty list is a no-op, and the cursor holds
  its position after a delete.
- **Existing tests that must be updated, not worked around:** everything referencing
  `Number`, `NextNumber` or `PeekNumber` — `json_test.go`
  (`TestNextNumberIncrementsAndSurvivesReopen`, `TestCorruptMetaRebuildsFromPuzzles`,
  which test subsystems being removed), plus `app_test.go` / `mouse_test.go` fixtures
  that construct games with a number.
- **`TestMarkersDoNotAffectLayout`** (`mouse_test.go:101-145`) — add a
  `"list with delete prompt"` row to the table, exactly as `"game with restart prompt"`
  exists for `confirmNew`. Also keep `TestClickTargetsMatchGlyphs` green; the row
  layout changes in Phase 1.
- `d` is a plain letter, so the `key()` helper and `TestKeyHelperMatchesFramework`
  (`app_test.go:15-41`, `82-88`) need no change. They would if the bind were `delete`.

**By hand**

- `go run . -data /tmp/wortle-scratch` — play a few puzzles, confirm codes render as
  six digits on both the board header and the list, and that they are stable across a
  restart.
- **Migration check, the one worth doing carefully:** point `-data` at a *copy* of the
  real save dir (never the original) and confirm old puzzles load, show codes derived
  from their pre-existing ids, and that the stale `meta.json` is ignored.
- Delete a puzzle, then open the profile and confirm the stats moved accordingly.

---

## Sequencing

Land as **two commits**: Phase 1 (identity + counter removal + docs) then Phase 2
(delete). Phase 1 is the larger and riskier change and is independently verifiable;
Phase 2 is small once it is in. Per this repo's convention, no `Co-Authored-By`
trailer.
