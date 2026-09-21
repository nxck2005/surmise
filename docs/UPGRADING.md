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
