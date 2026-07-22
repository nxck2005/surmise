# wortle

Wordle for the terminal — a monkeytype-styled TUI in Go, built with
[Bubble Tea](https://charm.land).

- **4, 5 and 6-letter modes**, the way monkeytype has 15/30/60.
- **Resumable puzzles.** Every puzzle is saved as you play; quit any time and
  pick it back up. Each gets a number, `wortle #42`.
- **A profile** with win rate, average attempts, average solve time, streaks and
  a guess distribution.

## Install

```sh
go install github.com/nxck2005/wortle@latest
```

Or from a clone:

```sh
go run .
```

Saved puzzles live in your user config directory (`~/.config/wortle` on Linux).
Pass `-data <dir>` to use somewhere else.

## Playing

| Key | Action |
| --- | --- |
| letters | type a guess |
| enter | submit |
| backspace | delete a letter |
| tab then enter | start a new puzzle (monkeytype-style restart) |
| esc | back to the menu |
| ↑/↓ · enter | navigate menus |
| q / ctrl+c | quit (an in-progress puzzle is saved) |

Tiles score like Wordle: green for the right letter in the right place, yellow
for the right letter elsewhere, grey for absent.

## Development

```sh
go test ./...        # all tests
go build ./...
```

Word lists are embedded in the binary. Regenerate them with:

```sh
go run ./tools/genwords
```

See [PLAN.md](PLAN.md) for the design and roadmap, and
[internal/words/data/SOURCES.md](internal/words/data/SOURCES.md) for word-list
provenance.
