<div align="center">

# wortle

**Wordle for the terminal.** A fast, themeable TUI in Go.

[![CI](https://github.com/nxck2005/wortle/actions/workflows/ci.yml/badge.svg)](https://github.com/nxck2005/wortle/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/nxck2005/wortle.svg)](https://pkg.go.dev/github.com/nxck2005/wortle)
[![Go Report Card](https://goreportcard.com/badge/github.com/nxck2005/wortle)](https://goreportcard.com/report/github.com/nxck2005/wortle)
![Go](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)
[![Built with Bubble Tea](https://img.shields.io/badge/built%20with-Bubble%20Tea-ff69b4)](https://charm.land)
![Themes](https://img.shields.io/badge/themes-13-e2b714)

</div>

```sh
go install github.com/nxck2005/wortle@latest
```

Then run `wortle`. It opens straight onto a puzzle — no menu to click through.

## What you get

- **4, 5 and 6-letter modes.** Length is the difficulty axis; nothing in the
  code hardcodes five. Pick the one you open on in the settings screen, or let
  it follow whatever you last played.
- **Resumable puzzles.** Every guess is written to disk, so quit mid-word and
  pick it back up. Each puzzle gets a number: `wortle #42`.
- **A profile screen** — win rate, average attempts, average solve time,
  current and best streak, guess distribution.
- **A colour legend on the board**, so a theme that does not use green and
  yellow is still readable at a glance. It steps aside on small terminals.
- **13 bundled themes** and a live-preview picker. Themes are plain text files;
  writing one is editing a handful of key/value lines.
- **Full mouse support.** Anything the keys can do, a click can do too — the
  on-screen keyboard types, list rows and menu entries are clickable, and the
  help line at the bottom doubles as a button bar.

## Playing

| Key | Action |
| --- | --- |
| letters | type a guess |
| <kbd>enter</kbd> | submit |
| <kbd>backspace</kbd> | delete a letter |
| <kbd>tab</kbd> then <kbd>enter</kbd> | new puzzle |
| <kbd>esc</kbd> | back to the menu |
| <kbd>↑</kbd>/<kbd>↓</kbd> · <kbd>enter</kbd> | navigate menus |
| <kbd>d</kbd> twice | delete the selected puzzle (in the puzzle list) |
| <kbd>q</kbd> / <kbd>ctrl+c</kbd> | quit (an in-progress puzzle is saved) |

Tiles score like Wordle: green for the right letter in the right place, yellow
for the right letter elsewhere, grey for absent. Duplicate letters behave the
way you'd expect them to — exact matches are claimed first, then whatever is
left over gets handed out.

## Settings

The settings screen holds two things: the mode new puzzles start in, and
whether playing a different mode makes *that* the default. Both are saved as
you change them.

```sh
wortle -length 6        # start in a mode for one run, without saving it
```

## Themes

```sh
wortle -themes          # list what's installed, and where the directory is
wortle -theme dracula   # play with a theme without changing your saved choice
```

Bundled: `serika-dark` (the default), `serika-light`, `catppuccin-mocha`,
`dracula`, `everforest-dark`, `gruvbox-dark`, `high-contrast`, `matrix`,
`nord`, `rose-pine`, `solarized-dark`, `terminal`, `tokyo-night`.

Your own go in `~/.config/wortle/themes`. A whole theme can be four lines:

```toml
name = "just the accent"
accent = "#ff0088"
```

Everything you leave out keeps its built-in value, and a bad line is a warning
naming the line — never a crash. Glyphs and spacing are themeable too, and
click targets are measured rather than hardcoded, so they follow along.
Full reference in [docs/THEMES.md](docs/THEMES.md).

## Where things live

Saved puzzles and settings go in your user config directory
(`~/.config/wortle` on Linux). `-data <dir>` points everything somewhere else,
which is handy for a scratch profile.

## Development

```sh
go run .                    # play from a clone
go build ./...
go test ./...
go test -race ./internal/...
go run ./tools/genwords     # regenerate the embedded word lists
```

`internal/game`, `store`, `words` and `stats` are the UI-agnostic core, fully
unit-tested; `internal/ui` is the Bubble Tea layer on top. The split is
deliberate — the same core is meant to be drivable by a server later. Word
lists are compiled into the binary with `go:embed`, so there is nothing to
download at runtime.

The UI has tests too: they drive the root model with synthetic key and mouse
events and assert on the rendered frame, so no TTY is involved.

See [internal/words/data/SOURCES.md](internal/words/data/SOURCES.md) for
word-list provenance.

---

Inspired by [monkeytype](https://monkeytype.com).
