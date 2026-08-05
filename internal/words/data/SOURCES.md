# Word list sources

Regenerate with `go run ./tools/genwords`. Do not edit the word lists by
hand — `blocked.txt` below is the one file here that is hand-maintained.

## guesses{4,5,6}.txt — accepted input

ENABLE1, filtered to the given length.

- Source: https://raw.githubusercontent.com/dolph/dictionary/master/enable1.txt
- License: public domain

ENABLE is a Scrabble dictionary, so it contains no proper nouns. That is why it
is used instead of a general word list such as dwyl/english-words, which admits
"aaron" and "adams" as five-letter words.

## answers{4,5,6}.txt — puzzle solutions

The intersection of the corresponding guess list with a frequency-ranked list of
common English, so solutions are words people actually know.

- Source: https://raw.githubusercontent.com/first20hours/google-10000-english/master/google-10000-english-usa-no-swears.txt
- Derived from the Google Web Trillion Word Corpus.

Every answer is by construction also a valid guess; `words` has a test asserting this.

## blocked.txt — words kept out of both

A hand-maintained list of slurs, edited by hand and read (never written) by
genwords, which drops every entry from the guess lists and therefore from the
answer lists too. ENABLE is a Scrabble dictionary and keeps a number of slurs,
so the source lists alone are not enough.

The list is deliberately length-agnostic and includes inflections and spellings
that no current mode can reach, so it keeps working if a word length is added.
`words` has a test asserting no shipped list contains a blocked word.
