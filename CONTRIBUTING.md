# contributing

Thanks for wanting to. This is a small project with strong opinions, so this
page is mostly about the opinions — the build itself is ordinary Go.

## Building and testing

The quickstart lives in [the README](README.md#development); the short version:

```sh
go run .          # play from a clone
go test ./...
go test -race ./internal/...
```

CI runs the tests on Linux, macOS and Windows, plus a WebAssembly build and a
headless smoke test that runs the WebAssembly build against xterm.js in Node. `gofmt` and `go vet` are checked, along with
known vulnerabilities in the Go module and the browser shell's npm
dependencies (`govulncheck` and `npm audit`); run gofmt and vet before pushing.

## The traps

These are the things that bite a first contribution. The package comments and
the docs under [`docs/`](docs) cover the rest.

- **Charm libraries are v2**, under `charm.land/...`, not
  `github.com/charmbracelet/...`. v2 differs from v1 in ways that break copied
  examples: `View()` returns a `tea.View` struct, not a string; keys arrive as
  `tea.KeyPressMsg`. When unsure of an API, read the module source rather than
  guessing from a blog post.
- **Do not edit `third_party/bubbletea`.** It is upstream v2.0.8 plus two
  additive patch files, held byte-for-byte so the web build can exist;
  `scripts/check-bubbletea.sh` proves it has not drifted and will fail CI if you
  touch it. See `third_party/bubbletea/PATCHES.md`.
- **The word lists are load-bearing.** A daily puzzle's answer is an index into
  them, so regenerating moves every unplayed date's word for everyone. Do not
  regenerate casually; to extend the blocklists, edit
  `internal/words/data/blocked.txt`, `profanity.txt` or `pruned.txt`, never a
  generated list, then run `go run ./tools/genwords`. The three files differ
  in direction: `blocked.txt` removes a word from the game entirely, and the
  other two keep it as a guess but never choose it as an answer.
  `internal/words/data/SOURCES.md` explains each one.
- **Keep keyboard and mouse at parity.** Anything a key can do, a click must do
  too. Adding a keybind means adding its click target, and both paths should go
  through one shared method rather than two copies of the handler.
- **Nothing hardcodes length 5.** Word length 4/5/6 is the difficulty axis; new
  code stays length-agnostic.
- **Don't look a puzzle up by its code.** `#042317` is a display label derived
  from the id and not unique; everything keys on the id.
- **An id is a persistence key, not free text.** It has to satisfy
  `game.ValidID` (legacy 16-lowercase-hex or a canonical lowercase UUID) before
  it may become a filename or a storage key, and `game.Validate` enforces that
  on every read and write. Never join an id from a save, a backup or a URL onto
  a path without it.
- **Never interpolate stored or imported text into a frame raw.** Words, daily
  dates, ids and theme values are refused at their own boundary
  (`game.Validate`, `theme.Parse`, the store codec); errors and messages that
  may carry anything else go through `ui.safeText` before they are rendered.
  The board draws a word byte by byte, so a control byte is an escape.
- **Untrusted input has a size.** Archives, records and theme files are read
  through caps (`backup.MaxArchiveBytes`, `store.MaxRecordBytes`,
  `theme.MaxFileBytes`) applied *while* reading, not after. A new entry point
  that reads a file, stream or browser `File` needs one too. Theme files may be
  symlinks: the picker follows one, but the file is statted and read through
  the opened descriptor so the cap is the target's, a backup export skips
  linked names and reports them, and writes use `O_EXCL` and never create or
  cross one.
- **New dependencies need an argument.** The direct set is deliberately four
  Charm modules: bubbletea, lipgloss, colorprofile and x/ansi. The theme reader is hand-rolled, UUIDs come from
  `crypto/rand`, and that is on purpose — say why nothing smaller exists before
  adding a module.

## Docs move with the change

- The non-obvious decisions in a change belong in comments beside the code it
  touches, not only in the diff. A comment says why, so the next person does
  not undo it.
- A change that a player can see, or that alters a saved file, gets an entry in
  [`docs/UPGRADING.md`](docs/UPGRADING.md) when it is released.
- Changing a keybind means updating **both** places players read it: the
  how-to-play controls page (`internal/ui/howtoscreen.go`) and README's key
  table.
- Renaming anything user-facing happens through `internal/brand`, not with a
  find-and-replace.

## Pull requests

- A descriptive title; leave the body empty.
- No `Co-Authored-By` or other co-author trailers.
- One behaviour per PR where you can — the review habit here is reading the
  whole diff.
- If your change touches rendering, add or extend a test in the existing style:
  the UI is tested headlessly by driving the root model with synthetic key and
  mouse events and asserting on the frame (`internal/ui/app_test.go`,
  `mouse_test.go`). No TTY needed.

## Reporting problems

Bugs and feature ideas go in the issue tracker. Security matters — anything that
should not be public before a fix exists — go through the process in
[SECURITY.md](SECURITY.md) instead.

## Licence

By contributing you agree that your contributions are licensed under the MIT
licence that covers the project — see [LICENSE](LICENSE).
