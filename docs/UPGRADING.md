# upgrading surmise

Almost every release is safe to install over the last one: saved puzzles,
settings, themes and stats carry forward untouched, and nothing here needs
reading. This file records the exceptions — the releases where something you can
see changed meaning — newest first.

## The save-format promise

Every puzzle record, every tombstone and the settings file carry a `schema`
number saying what format they were written in. The rules that keep your history
safe across upgrades:

- **A record with no `schema` (or `"schema": 0`) is valid forever.** That is
  every file written before the tag existed, and it will always read.
- **Adding a field is never a breaking change.** New fields appear; old readers
  ignore them, new readers take their zero value as "unset".
- **A field is never renamed or repurposed.** A meaning that changes gets a new
  name, and the old one stays put.
- **`schema` moves only for a breaking change**, and a reader that meets a
  number it does not know refuses the record with `schema version mismatch`
  rather than guessing. A mismatch means either a file from a newer app or a
  corrupt one — both deserve an error, not a wrong answer.

The fixtures under `internal/store/testdata/` and the tests beside them pin all
of this.

## What a save survives

The game saves after every accepted guess, when a puzzle ends, and when you
leave a board. Each save is written to a temporary file and renamed into place,
so a file is always either the old version or the new one, never half of each.

- **The app crashing, being killed, or the terminal closing costs nothing**
  that was already saved. That includes `kill -9`.
- **A power cut or an operating-system crash can cost the last few seconds of
  saves.** The game does not force each save onto the disk (`fsync`); the
  operating system writes it out a few seconds later, as it does for most
  files. What was on the disk before stays readable.

This is a choice, made on 2026-10-03 for v0.7.0. Until then every save was
synced. That made each guess wait about 4.5 ms for the disk on Linux, longer on
macOS, and made a restore of 10,000 puzzles spend about 45 seconds waiting —
and it still did not fully protect a power cut, because the folder itself was
not synced, so a newly created puzzle could still vanish.

The alternative was to make the stronger promise properly: sync the folder as
well as the file, and move every save to a background writer so play never
waits for the disk. It was not taken. It would have changed when a failed save
is reported (after the guess rather than on it) and how the app has to shut
down, and losing a few seconds of a word game to a power cut did not justify
that. If that trade ever changes, it is that design, not putting the old sync
back.

The browser build is unaffected either way: a browser decides for itself when
its storage reaches the disk.

## v0.7.4 → v0.7.5: plainer how-to-play text

The how-to-play pages say the same rules in shorter, plainer sentences. Nothing
else changed: `schema` does not move, no daily answer moves, and no theme or
word list changed.

## v0.7.3 → v0.7.4: steadier panels, two more rows

This release changes only how screens are laid out: `schema` does not move, no
daily answer moves, and no theme or word list changed.

- **A screen no longer repeats its name under the panel's title.** The rule at
  the top of the panel already names the screen. Each screen gains two rows, so
  the puzzle list, the about screen and how to play fit more on a short
  terminal, and the splash art shows on a terminal two rows shorter than before.
  A heading that says something else stays, such as "new challenge". Your
  profile name, if you set one, is now on the right of the profile's rule.
- **Panels come in two widths.** Before, each screen sized its panel to its own
  content, so the frame grew and shrank as you moved between screens. The menu
  and the list-like screens now share one width, and the board, the result and
  the setups share a wider one. The profile, which is wider than both, keeps
  its own. On a terminal too narrow for a width, a panel fits the terminal as
  before.

## v0.7.2 → v0.7.3: clearer colours, and a tidier look

This release changes how the game looks, not what it saves: `schema` does not
move, no daily answer moves, and no word list changed.

### Bundled themes changed colour

- **An untouched key no longer looks like a ruled-out letter.** In every bundled
  theme the untouched keycap was close to the colour of the "not in word" tile,
  and in dracula and solarized it was the same colour. Each theme now draws it
  lighter, in a colour from its own palette.
- **A letter on a "not in word" tile is easier to read.** Matrix, solarized and
  ember light changed their `absent_text`. The terminal theme now draws an
  untouched key light grey with a black letter.

### A custom theme can look different

`absent_text` now follows `text`, not `muted`. If your theme does not set
`absent_text`, the letter on a "not in word" tile is now your text colour. To
keep the old look, add `absent_text = "muted"` to your theme. A theme that
already sets it does not change.

### The screens

- A tall board draws the tiles you have not played yet as outlines, in your
  theme's `border` glyphs. A one-row board does not change.
- The board's panel title names the kind of board: puzzle, daily, custom,
  challenge or sprint. The kind no longer starts the status at the right of
  that rule.
- A sprint board shows one clock, the countdown.
- The help bars agree: `esc menu` when esc goes to the menu, `esc back` when it
  goes to the screen before, and `← previous` / `→ next` wherever a value steps.
- The theme list lines up a custom theme with the built-in ones, the backup
  screen has its own title, and the about screen fits a long data path to the
  terminal.

## v0.7.0 → v0.7.1: a network setting, off

Settings has a new row, **network**. It is off, and it stays off until you turn
it on. In this release no feature uses it: the game still makes no network
calls, whatever the row says. It is there so that later online features have
your consent to ask for, and so that they do nothing until you give it.

- **A backup does not carry it.** Restoring an archive never turns the network
  on, even one made with it on. You choose it on each install.
- **Downgrading forgets it.** v0.7.0 does not know the field, so the next time
  it saves your settings the choice is lost, and it reads as off again.

Nothing else changed: `schema` does not move, and no daily answer moves.

## v0.6.7 → v0.7.0: smaller saves, faster saves, and the 2026-10-02 audit

This release changes how saves are written, though not what they say: `schema`
does not move, and nothing an earlier build wrote changes meaning. No word list,
challenge snapshot or bundled theme changed, so no daily answer moves.

### Saves are no longer forced onto the disk

**What changed.** Each save used to wait for the disk (`fsync`). It no longer
does. A crash, a kill or a closed terminal still costs nothing that was saved;
a power cut or an operating-system crash can now cost the last few seconds of
saves. The full reasoning, and the design that was not chosen, is in
[What a save survives](#what-a-save-survives) above.

**What you will notice.** Probably nothing while playing — the wait was about
4.5 ms a guess on Linux, more on macOS. A large restore is much faster: 10,000
puzzles used to spend about 45 seconds waiting for the disk.

### Saves and backups are written as compact JSON

**What changed.** Puzzle records, `settings.json` and backup files are written
on one line, without indentation. That makes a record about half the size.

**Why.** For the browser build. A browser gives a site about five million
characters of storage, which held 7,000–9,000 indented puzzles and holds
12,000–14,000 compact ones — enough for a full backup to fit.

**What you will notice.** Nothing in the game. Whitespace is not part of the
format, so this build reads indented files, and every earlier build reads the
compact ones; a backup made here restores into v0.6.7 and the other way round.
Existing files keep their indentation until the game next writes them. To read
one, `jq . file.json` puts the indentation back.

### The browser build shows how full its storage is

The backup screen now shows how much of the browser's storage is used, in red
from 90%. When it is full, the board says `storage is full — not saved (see
backup)` instead of passing on the browser's own error. Play continues, but
nothing new is kept until there is room. Save a backup before then. See
[`docs/WEB.md`](WEB.md).

### Fixes from the 2026-10-02 audit

- **A theme glyph is capped at 128 bytes as well as 16 cells.** v0.6.6 replaced
  the 16-rune cap with a cell count and lost the byte bound with it, so a glyph
  of thousands of zero-width marks passed. Such a glyph is now refused with a
  warning. No bundled theme comes near either cap.
- **The error line clears** on your next key press or click, instead of
  staying on every screen until you quit. A value echoed back in an error — a
  theme name from a link, for example — is cut to 40 characters.
- **One backup load at a time.** A second press of "load a backup" while the
  file picker was open used to start a second load, and a second merge beside
  the first.
- **A restored theme cannot be named after a Windows device** (`con`, `nul`,
  `com1` and the rest), which Windows treats as the device whatever follows the
  dot.
- **`$NO_MOTION` is read**, as the README always said. Before, only
  `$SURMISE_NO_MOTION` turned the animations off.
- **The browser grants a clipboard write from the copy action**, not from the
  bytes being written. See the correction in the v0.6.5 section below.
- `install.sh` uses HTTPS only, checks the shape of the version it resolved,
  and matches the checksum line exactly. The web deploy checks the value of
  each security header, not only that it is there.

## v0.6.5 → v0.6.6: the rest of the 2026-09-26 audit

Small fixes from the same audit. None changes the save format, and `schema`
does not move. No word list, challenge snapshot or bundled theme changed, so no
daily answer moves.

- **A theme glyph is capped at 16 cells wide, not 16 runes.** A glyph made of
  wide characters (most emoji, CJK) now counts two cells for each one, so a
  glyph of nine or more of them is refused with a warning, as an over-long
  glyph always was. No bundled theme comes near the cap.
- **A bar glyph wider than one cell draws the histogram at its proper width.**
  Before, a two-cell `bar` made every bar twice as long and pushed the profile
  out of its panel. It is now drawn half as many times.
- The about screen and the backup screen now filter the `-data` path they show,
  as every other screen already filtered outside text. Only your own command
  line could put an escape there.
- The web deploy now checks the security headers on the live site, not only on
  staging, and the release workflows refuse a tag that is not a version number.
  Nothing you run changes.

## v0.6.4 → v0.6.5: hardening from the 2026-09-26 audit

Two bounds closed, both found by the 2026-09-26 security audit and both
reachable by importing a file somebody else wrote. Neither changes the save
format, and `schema` does not move.

This is the first release since v0.6.2 with anything you could notice, and for
almost nobody that is anything at all: both changes refuse a file that was never
a real one. If you keep ordinary backups you will see no difference.

### A preferences field is bounded on its own

**What changed.** Each free-text preference — the theme name, the display name,
and the splash and motion choices — is now held to 128 bytes of its own, and an
archive carrying one past that is refused by name (`display name is longer than
128 bytes`) rather than filling it in. The 64 KiB bound on the settings blob
itself is unchanged and still checked; this is a bound per field, not instead of
one. A value the app wrote is far below either figure, so nothing this build
produces is affected.

Nothing about the format changed. `schema` does not move, and no record, setting
or archive written by a released build changes meaning.

**Why.** From the 2026-09-26 security audit. A total-size bound is not a
per-field bound: an archive could spend most of its 64 KiB on one string, and
the display name is drawn in a fixed 20-cell row. A name made of a few thousand
combining marks is printable, so it passes every text filter, and it is zero
cells wide, so a cell count never notices it either — the row measured thousands
of columns for a name that rendered as nothing, and since nothing in the app
truncates horizontally, that widened the whole panel. The name was then saved,
so it came back on every launch, and the editor could not remove it either: the
marks are invisible and backspace erases one at a time.

**What you will notice.** Nothing, unless you import an archive carrying an
absurdly long preference. If you do, the import is refused and says which field.
A `settings.json` already on disk is repaired when it is read: the offending
value is shortened to what fits, so the app is usable and the profile can be
edited normally again. A `display_name` of 128 bytes is far longer than the
19 cells the row shows, so if you did have one, the visible part is unchanged.

### A saved puzzle cannot hold more guesses than its board allows

Alongside the preferences bound above, a puzzle record is now held to its own
attempt limit: a record claiming more guesses than `length + 1` is refused,
wherever it is read — a save, a backup, an import.

The save format has not changed and `schema` does not move. No record written by
a released build is affected, because `Guess` has always stopped at the limit
and ended the game; only a hand-edited or crafted file can disagree.

**Why.** From the same audit. The composer's height ladder sizes a frame from
the board's attempt limit while the board itself draws one row per guess, so a
record where the two disagree was sized as though it would fit and drawn as
though it would not: a frame thousands of rows tall, of which a terminal shows
the bottom. The rows that go are the panel's title rule and its ✕ close box, and
the cost of redrawing that frame every message is high enough to leave the app
unresponsive. The safeguard built for exactly this — refuse to draw a board
nobody can see — was reading the same number as the frame, so it never fired.

This is also the root cause of #81's per-record size cap: a valid record could
be made arbitrarily large out of surplus guesses, which is why the cap was
needed. With the guess count bounded, every field of a record is bounded and the
largest valid record is a few hundred bytes, so that cap is now a backstop with
nothing behind it rather than a limit doing real work.

**What you will notice.** Nothing. If a save or an archive is refused for this,
the message names the record and the two counts, and importing it again after
deleting that puzzle from the file works.

### The installer will not write through a symlink

`install.sh` now refuses outright if a symlink is sitting where the binary goes,
whatever `SURMISE_FORCE` says, and refuses an archive whose `surmise` is one. The
binary is written beside the destination and renamed over it, and is installed
`0755` outright rather than having `+x` added to whatever mode the archive
carried.

**Why.** From the same audit. The old guard was `[ -f "$DEST/surmise" ]`, which
follows a link, so it asked whether there was a *file* there rather than whether
there was a *link* — three separate consequences, each reproduced against the old
script before it was fixed. A **dangling** link is not a file, so the guard passed
with no `SURMISE_FORCE` needed and the install replaced the link you had put
there. A link to a **directory** is a directory to `mv file dest`, so the binary
landed *inside* it and the link stayed: an install that reported success and put
nothing where you would run it from. And with `SURMISE_FORCE`, `mv` and `chmod`
both dereferenced, so a link to a real file had that file replaced instead of the
link.

**What you will notice.** Only if you had a symlink at the destination, which
almost nobody does. If you did — a version-manager link, a link into a dotfiles
repo — the installer now stops and says so rather than replacing or writing
through it. Remove the link and run it again, or point `SURMISE_INSTALL_DIR`
somewhere else. `SURMISE_FORCE` still replaces a *file* that is there; it just no
longer follows a link to somewhere else.

### A copied result will only reach the clipboard when you asked for it

In the browser build, the result screen's copy action still works exactly as
before. What changed is the other direction: the page no longer writes to your
clipboard for an OSC 52 sequence it did not ask for.

**Why.** Also from the same audit, and this one is defence in depth rather than a
live bug. The handler that bridges to the browser's Clipboard API is registered
for the life of the page, and its only condition on a write was that the base64
was well-formed — Go's half of that contract was a comment in the source rather
than anything the page checked. If any of the app's text filters ever regressed,
the result would have been a silent clipboard overwrite, which is the poisoning
primitive and has no visible symptom: the app says `copy requested` and never
learns whether it worked.

**What you will notice.** Nothing. A clipboard write now needs the game to have
asked for one, and the game only asks when you press the copy key or click the
button.

**Correction (v0.7.0).** The paragraph above claimed more than v0.6.5 did. The
page armed its one-shot permission whenever the game's output *contained* a
clipboard sequence, not when the game asked for a copy — so the gate trusted
the very bytes it was meant to check. Nothing could exploit it, because the
renderer drops such sequences from anything drawn on screen, but that was luck
rather than the gate. From v0.7.0 the permission is granted by the copy action
itself, and a test pins the renderer's behaviour.

### Smaller things

- The board's click-target parser no longer trusts the id in a marker it finds in
  a frame, and drops an unterminated one rather than passing it to the terminal. No
  accepted value can contain an escape byte today, so this closes a latent panic
  rather than a live one.
- The vendored Bubble Tea copy's two WebAssembly patch files are now pinned by
  digest and checked in CI, so an edit to either fails the build instead of
  reaching a browser release. See `third_party/bubbletea/PATCHES.md`.

## Challenge-code answer snapshots

A challenge code carries an answer-list version. Version 1 is the exact ordered
4-, 5-, and 6-letter answer pool shipped when codes were introduced, pinned by
full-file hashes in `internal/words/words_test.go`.

Future word-list work must add a new snapshot and advance
`CurrentAnswerVersion`; it must not overwrite a snapshot an older challenge
code names. Old decoders stay available for retained snapshots. This promises
the same challenge identity and answer, not that every non-answer guess remains
accepted forever. A safety removal may explicitly retire an affected old code;
silently mapping it to another answer is never allowed.

## v0.6.1 → v0.6.2: hardening from the 2026-09-21 audit

**An archive's arrays are counted while they are decoded.** A backup names its
puzzles and themes in two JSON arrays, and the count limits were only applied
after the whole array had been read into memory: a hostile file could name two
million empty puzzles in a few megabytes and spend roughly twenty-seven times
its own size in allocations before the limits were consulted. The counts are
now enforced element by element, as the file is decoded, so a file past a limit
is refused at the limit rather than after it. The refusal names the same
figure it always did, and every archive written by a released build still reads
unchanged.

**A backup is only written if this build could read it back.** The reader
refuses an archive over 64 MiB, with more than 10,000 records, more than 256
themes, a theme body or settings blob over its cap — but `Build` applied none
of those, so a long history or a couple of imported theme packs could produce
an export that its own import then refused. Export and the backup screen now
check the same limits and say which one was hit instead of writing a file that
cannot be restored. The limits themselves are unchanged; raising them stays a
deliberate decision rather than something a write path quietly works around.

**A daily record has to carry its own date.** A puzzle saved as the daily for a
day is now held to the id that date and mode derive, wherever a record is read
or written: an imported or hand-edited record that claims a real daily's id
under a chosen date is refused, and so is a custom puzzle carrying a daily
date. The daily streak walk also skips custom puzzles instead of counting a day
they never played. No save written by a released build is affected — every
daily has carried its derived id since dailies shipped, and the check needs no
schema change.

**A puzzle file has to be a regular file.** Opening a FIFO for reading blocks
until a writer appears, and one planted at a puzzle's path hung every scan that
touched the history — startup, the menu, the profile, the list. The store now
refuses a file that is not a plain one by its mode, before opening it, so a
planted pipe is skipped like any other unreadable record and a planted
`settings.json` falls back to the defaults. Nothing this app writes is
affected.

**A puzzle's play time is bounded.** The per-puzzle `elapsedMs` is multiplied
by a millisecond to render, so a hand-edited or imported value past what a
`time.Duration` can hold wrapped `Elapsed()` negative and moved the profile's
solve-time totals and averages. A saved puzzle's elapsed time is now held to
the same bound the settings play counter has always had, on every read and
write; no session this app records can come near it. The two figures are one
constant now, so they cannot drift apart.

**The backup button loads only this app's own dated files.** The newest backup
was chosen by name, so anything ending in `.json` that sorted after the dated
names was treated as the one to restore — a dropped file named `zzz.json`
shadowed every real backup. A candidate now has to be a name this app writes
(`surmise-backup-YYYY-MM-DD`, then `-2`, `-3`, …): a foreign name is left
alone, and a directory holding only foreign files says there are no backups
rather than loading one. The numbering is compared as a number too, so the
tenth save of a day is newer than the ninth. `-import <path>` still takes any
file the player names.

## v0.6.0 → v0.6.1: imported settings are bounded, theme links leave backups

**What changed.** The settings file is now held to the same 64 KiB bound on
write that it is read under: an import that fills in a preference can no longer
make the app write a `settings.json` its own reader would refuse. An archive's
settings section is checked before anything is applied — the schema tag, the
size, and the lifetime play counter, which is refused past the point a
`time.Duration` can hold. A hand-edited counter past that point is clamped when
the file is read, so the profile cannot show a negative total, and a giant saved
splash length is reported out of range instead of wrapping into a small one.

A theme loaded through a symlink is now statted and read through the opened
file, so the 64 KiB cap applies to the link's target and holds while it is read.
A backup's export no longer follows a symlinked theme: the name is left out and
the export says so. The picker still loads a linked theme, and a restore still
never creates or crosses one.

**Why.** Both come from the 2026-09-18 security audit's release gate. A link to
a large file was measured by the link's own few bytes, so its target could be
read whole and carried into a portable backup. An archive could devote most of
its 64 MiB to one preferences string, and a play counter past a duration's range
turns every later figure into nonsense.

**What you will notice.** Nothing, unless you keep a theme by symlink and back
up: that theme is no longer in the archive (the export names it), and restoring
it on another machine means copying the link's target. No setting, save, theme
or archive written by a released build changes meaning, and nothing needs
migrating.

## v0.5.4 → v0.6.0: saved records are validated

**What changed.** A puzzle id must now be one of the two shapes this app has
ever written: sixteen lowercase hex characters (saves from before puzzles
carried UUIDs) or a canonical lowercase UUID (version 4 for a random puzzle,
version 8 for a derived one). Anything else is refused wherever an id is read
or written, and an archive carrying such a record is refused whole, before any
of it is imported. A record is also held to the id it is stored under: one that
answers for a different id is ignored rather than trusted.

**Why.** A puzzle id becomes a filename on the desktop and a storage key in a
browser, and an imported backup supplies ids. Without the check, an archive
could name an id like `../settings` and have the import write outside the
puzzle directory.

**What you will notice.** Nothing, unless you hand-edited a save or import a
backup that was not written by this app. That file now reports an error instead
of being written. Every puzzle, tombstone, setting and theme written by any
released build still reads unchanged — no migration and no schema bump, because
the bytes are the same and only what is accepted narrowed.

**The words are validated too.** A saved answer, guess or daily label must be
lowercase letters or an exact `YYYY-MM-DD` date; anything else is refused
wherever a record is read, and an archive carrying one is refused whole. An
import error that names a refused theme file now quotes that name instead of
printing it raw. This is the same class of change, closing the second finding
of the 2026-09-05 audit: those fields are drawn on the board, where an embedded
terminal escape could reach the terminal — or, in a browser, the clipboard. No
released build ever wrote such a record, so no legitimate save is affected.

**The rest of the audit's hardening lands here as well.** An imported archive
is bounded — 64 MiB for the file, at most 10,000 records of 64 KiB, 256 themes
of 64 KiB — and a record or theme over its cap is refused or shown with an
error rather than read whole. A decoded puzzle's `maxAttempts` must be its
length plus one, its status a real one, and its marks in range. New data
directories are created `0700`; directories that already exist keep whatever
mode they have. A restore never follows a symlink when writing a theme file,
though reading one through a link still works. For a browser deploy the site
now sends a strict Content-Security-Policy and the usual companion headers.
None of this changes a record written by a released build.

## v0.3.1 → v0.3.2: every daily answer moved

**What changed.** 981 words were taken out of the three answer lists: proper
nouns, plurals and third-person verbs ending in `-s`, crude words,
interjections, slang and clipped forms, foreign words that are not naturalized,
and British-only spellings. They made poor solutions, and the lists are much
better for their absence.

**Why that is breaking.** A daily answer is not stored in a calendar. The date
is reduced to a number and that number indexes the sorted answer list, so
removing a word from the middle of the list shifts every word after it. There is
no way to take a word out of the pool and leave the calendar where it was.

**What you will notice.**

- Every date after the upgrade has a different daily answer than v0.3.1 would
  have given it. **Two people on different versions no longer share a board on
  the same day.** If you compare results with someone, upgrade together.
- Dailies you already finished are unaffected. A finished puzzle keeps the
  answer it was played with; history, stats and streaks do not move.
- A daily you have **in progress** keeps its original answer too, because the
  board was saved with it. Only days you have not started yet are drawn fresh.

**What did not change.** No word was removed from the guess lists, so everything
you could type before is still accepted — the removed words simply can no longer
be the solution. Random puzzles were never reproducible from a date, so they
have nothing to preserve. The save format, the settings and the daily derivation
itself are all untouched.

**If you want the old answers back**, stay on
[v0.3.1](https://github.com/nxck2005/surmise/releases/tag/v0.3.1). There is no
setting for it: the answer pool is compiled into the binary.
