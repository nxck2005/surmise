# Word list sources

Regenerate with `go run ./tools/genwords`. Do not edit the word lists by
hand — `blocked.txt` and `profanity.txt` below are the two files here that are
hand-maintained.

Both sources are pinned to a commit, not a branch: an answer list that shifts
under us changes the word of the day for every date not yet played, so a
regeneration is a deliberate act. Licences are reproduced in
`THIRD_PARTY_NOTICES.md` at the repo root.

## guesses{4,5,6}.txt — accepted input

ENABLE1, filtered to the given length.

- Source: https://raw.githubusercontent.com/dolph/dictionary/c65f04b0b5b2/enable1.txt
- Revision: c65f04b0b5b27a981f437b940cf62fe71320d5ec
- License: public domain

ENABLE is a Scrabble dictionary, so it contains no proper nouns. That is why it
is used instead of a general word list such as dwyl/english-words, which admits
"aaron" and "adams" as five-letter words.

## answers{4,5,6}.txt — puzzle solutions

The intersection of the corresponding guess list with the top 10000 entries of a
frequency-ranked list of common English, minus `profanity.txt`, so solutions are
words people actually know and would not mind seeing.

- Source: https://raw.githubusercontent.com/hermitdave/FrequencyWords/525f9b560de4/content/2018/en/en_50k.txt
- Revision: 525f9b560de45753a5ea01069454e72e9aa541c6
- License: MIT (Hermit Dave)
- Derived from the OpenSubtitles 2018 corpus.

This replaced first20hours/google-10000-english, which derives from an LDC
corpus and asks that commercial use be licensed separately — a restriction this
project's MIT licence cannot pass on. The vocabulary is conversational rather
than literary as a result, which is what `profanity.txt` exists to temper.

Every answer is by construction also a valid guess; `words` has a test asserting this.

## blocked.txt — words kept out of both

A hand-maintained list of slurs, edited by hand and read (never written) by
genwords, which drops every entry from the guess lists and therefore from the
answer lists too. ENABLE is a Scrabble dictionary and keeps a number of slurs,
so the source lists alone are not enough.

The list is deliberately length-agnostic and includes inflections and spellings
that no current mode can reach, so it keeps working if a word length is added.
`words` has a test asserting no shipped list contains a blocked word.

## profanity.txt — words kept out of answers only

Vulgar but unremarkable English: words a player may reasonably type and should
be told are real, but which should never be the word of the day. Unlike
`blocked.txt` these stay in the guess lists, so the "every answer is a valid
guess" invariant is untouched — the subtraction only ever runs one way.

The distinction is the point. A slur is not a word this game accepts; a swear is
a word it accepts but does not choose.
