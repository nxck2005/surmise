# contributing

This is a small project. The build is normal Go. Most of this page is about
the rules that the code follows, because these rules are not always clear from
the code.

## Building and testing

The [README](README.md#development) has the full steps. In short:

```sh
go run .          # play from a clone
go test ./...
go test -race ./internal/...
```

CI runs the tests on Linux, macOS and Windows. It also makes a WebAssembly
build and runs a headless smoke test of that build against xterm.js in Node.
CI checks `gofmt` and `go vet`. It also checks for known vulnerabilities in
the Go modules (`govulncheck`) and in the npm dependencies of the browser shell
(`npm audit`). Run gofmt and vet before you push.

## Common problems

Most first contributions have one of these problems. For all other topics, read
the package comments and the docs in [`docs/`](docs).

- **The Charm libraries are v2.** Their import paths start with
  `charm.land/...`, not `github.com/charmbracelet/...`. Examples written for v1
  do not work. In v2, `View()` returns a `tea.View` struct, not a string, and
  keys come in as `tea.KeyPressMsg`. If you are not sure about an API, read the
  module source. Do not trust old blog posts.
- **Do not edit `third_party/bubbletea`.** It is upstream v2.0.8 with two
  added patch files, kept byte for byte. The web build needs it.
  `scripts/check-bubbletea.sh` compares it with upstream, and CI fails if it
  changes. For details, read `third_party/bubbletea/PATCHES.md`.
- **Do not regenerate the word lists without a good reason.** The answer for
  each daily puzzle is an index into these lists. If the lists change, the word
  for every future date changes for all players. To add words to a blocklist,
  edit `internal/words/data/blocked.txt`, `profanity.txt` or `pruned.txt`. Do
  not edit a generated list. Then run `go run ./tools/genwords`. The three
  files do different things. A word in `blocked.txt` is removed from the game.
  A word in one of the other two files is still a valid guess, but it is never
  an answer. `internal/words/data/SOURCES.md` explains each file.
- **The keyboard and the mouse must do the same things.** If a key does
  something, a click must do it too. When you add a keybind, add its click
  target. Both paths must call one shared method. Do not copy the handler.
- **Do not hardcode a word length of 5.** Word length (4, 5 or 6) sets the
  difficulty. New code must work for all three lengths.
- **Do not find a puzzle by its code.** The code (for example `#042317`) is a
  label made from the id, and two puzzles can have the same code. Use the id
  for all lookups.
- **An id is a key for stored data, not free text.** Before an id becomes a
  filename or a storage key, it must pass `game.ValidID`. A valid id is a
  legacy id of 16 lowercase hex characters, or a canonical lowercase UUID.
  `game.Validate` checks this on each read and each write. An id from a save,
  a backup or a URL must never go into a path without this check.
- **Do not put stored or imported text into a frame without a check.** Words,
  daily dates, ids and theme values are checked where they come in
  (`game.Validate`, `theme.Parse` and the store codec). Errors and messages
  that can contain other text must go through `ui.safeText` before they are
  shown. The board draws a word one byte at a time, so a control byte becomes
  an escape sequence.
- **Limit the size of untrusted input.** Archives, records and theme files have
  size limits (`backup.MaxArchiveBytes`, `store.MaxRecordBytes`,
  `theme.MaxFileBytes`). The code applies each limit *during* the read, not
  after it. If you add code that reads a file, a stream or a browser `File`,
  give it a limit too. Theme files can be symlinks. The picker follows a
  symlink, but it stats and reads the file through the open descriptor, so the
  limit applies to the target file. A backup export skips symlinked names and
  lists them in its report. Writes use `O_EXCL`, and they never make or follow
  a symlink.
- **Explain why you need a new dependency.** The list of direct dependencies is
  small, and we want to keep it small. It has four Charm modules (bubbletea,
  lipgloss, colorprofile and x/ansi) and coder/websocket. Online rooms need
  websocket, and it also works in the browser build. The theme reader is our
  own code, and UUIDs come from `crypto/rand`. Before you add a module, explain
  why a smaller solution is not possible.

## Update the docs with the change

- Put the reasons for decisions that are not obvious in comments next to the
  code. Do not put them only in the commit. A comment that says why stops the
  next person from undoing the change.
- If a player can see the change, or if it changes a saved file, add an entry
  to [`docs/UPGRADING.md`](docs/UPGRADING.md) when it is released.
- If you change a keybind, update the two places where players read about it:
  the how-to-play controls page (`internal/ui/howtoscreen.go`) and the key
  table in the README.
- To rename something that players see, change it in `internal/brand`. Do not
  use find and replace.

## Pull requests

- Give the PR a clear title, and leave the body empty.
- Do not add `Co-Authored-By` or other co-author trailers.
- If possible, change one behaviour per PR. Reviews here read the full diff.
- If your change affects rendering, add a test or extend one in the same
  style. The UI tests run headless. They send synthetic key and mouse events to
  the root model and check the frame (`internal/ui/app_test.go`,
  `mouse_test.go`). You do not need a TTY.

## Reporting problems

Put bugs and feature ideas in the issue tracker. For a security problem, or
anything that must stay private until a fix is ready, use the process in
[SECURITY.md](SECURITY.md).

## Licence

When you contribute, you agree that your contributions use the MIT licence
that covers the project. See [LICENSE](LICENSE).
