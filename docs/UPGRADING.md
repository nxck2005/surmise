# upgrading surmise

Almost every release is safe to install over the last one: saved puzzles,
settings, themes and stats carry forward untouched, and nothing here needs
reading. This file records the exceptions — the releases where something you can
see changed meaning — newest first.

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
