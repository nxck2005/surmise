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
