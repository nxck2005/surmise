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
headless browser smoke test. `gofmt` and `go vet` are checked; run them before
pushing.

## The traps

These are the things that bite a first contribution. The full operational guide
is [`AGENTS.md`](AGENTS.md) — written for coding agents, but the most precise
map of the repository there is.

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
  `internal/words/data/blocked.txt` or `profanity.txt`, never a generated list,
  then run `go run ./tools/genwords`.
- **Keep keyboard and mouse at parity.** Anything a key can do, a click must do
  too. Adding a keybind means adding its click target, and both paths should go
  through one shared method rather than two copies of the handler.
- **Nothing hardcodes length 5.** Word length 4/5/6 is the difficulty axis; new
  code stays length-agnostic.
- **Don't look a puzzle up by its code.** `#042317` is a display label derived
  from the id and not unique; everything keys on the id.
- **New dependencies need an argument.** The direct set is deliberately three
  Charm modules. The theme reader is hand-rolled, UUIDs come from
  `crypto/rand`, and that is on purpose — say why nothing smaller exists before
  adding a module.

## Docs move with the change

- A substantial change appends to [`notes/PLAN.md`](notes/PLAN.md) (the living
  design doc) and gets a write-up in `notes/plans/`. The non-obvious decisions
  belong there, not just in the diff.
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
