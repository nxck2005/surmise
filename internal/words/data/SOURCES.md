# Word list sources

Regenerate with `go run ./tools/genwords`. Do not edit these files by hand.

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
