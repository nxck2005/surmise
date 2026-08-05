<div align="center">

# wortle

**the word game for the terminal.** fast, themeable, remembers where you left off.

[![ci](https://github.com/nxck2005/wortle/actions/workflows/ci.yml/badge.svg)](https://github.com/nxck2005/wortle/actions/workflows/ci.yml)
[![go reference](https://pkg.go.dev/badge/github.com/nxck2005/wortle.svg)](https://pkg.go.dev/github.com/nxck2005/wortle)
[![go report card](https://goreportcard.com/badge/github.com/nxck2005/wortle)](https://goreportcard.com/report/github.com/nxck2005/wortle)
![go](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)
[![built with bubble tea](https://img.shields.io/badge/built%20with-bubble%20tea-ff69b4)](https://charm.land)
![themes](https://img.shields.io/badge/themes-13-e2b714)

</div>

```sh
go install github.com/nxck2005/wortle@latest
```

then run `wortle`. it opens straight onto a puzzle — no menu, no splash, no
account.

```
╭─ wortle ───────────────────────────────────────────────────────── × ╮
│                                                                     │
│                 wortle #352083   5 letters   –   2/6                │
│                                                                     │
│                  S       L       A       T       E                  │
│                                                                     │
│                  C       H       A       I       R                  │
│                                                                     │
│                  C       R       A       _       ·                  │
│                                                                     │
│                  ·       ·       ·       ·       ·                  │
│                                                                     │
│                  ·       ·       ·       ·       ·                  │
│                                                                     │
│                  ·       ·       ·       ·       ·                  │
│                                                                     │
│       Q     W     E     R     T     Y     U     I     O     P       │
│                                                                     │
│          A     S     D     F     G     H     J     K     L          │
│                                                                     │
│          ⏎     Z     X     C     V     B     N     M     ⌫         │
│                                                                     │
│                            4 guesses left                           │
│                                                                     │
│      A    correct spot ·    A    wrong spot ·    A    not in word   │
│                                                                     │
│     type a word · enter submit · tab+enter new puzzle · esc menu    │
│                                                                     │
╰─────────────────────────────────────────────────────────────────────╯
```

## why you'd want it

**more options for word sizes.** play 4, 5 or 6 letters! shorter is
tighter, longer gives you a guess more room. pick which one you open on, or let
it just follow whatever you played last.

**quit whenever.** every guess is saved the moment you make it. come back whenever! clock doesn't tick while you're away.

**it keeps score.** win rate, average attempts, average solve time, current and
best streak, guess distribution, all broken down by mode.

**wide assortment of themes, live preview.**  serika, catppuccin, dracula, gruvbox,
nord and a lot more. write your own themes as well.

**built to be mouse first.** all of it.

**no network required.** plays offline forever.

## playing

| key | action |
| --- | --- |
| letters | type a guess |
| <kbd>enter</kbd> | submit |
| <kbd>backspace</kbd> | delete a letter |
| <kbd>tab</kbd> then <kbd>enter</kbd> | new puzzle (not on the daily — there's one a day) |
| <kbd>esc</kbd> | back to the menu |
| <kbd>↑</kbd>/<kbd>↓</kbd> · <kbd>enter</kbd> | navigate menus |
| <kbd>d</kbd> twice | delete the selected puzzle (in the puzzle list) |
| <kbd>q</kbd> / <kbd>ctrl+c</kbd> | quit (an in-progress puzzle is saved) |

## the daily

```
╭─ daily ─────────────────────────────── × ╮
│                                          │
│                  daily                   │
│      2026-08-06 · resets in 7h 21m       │
│                                          │
│    › #072140 4 letters  not started      │
│      #390265 5 letters  solved 1/6       │
│      #448851 6 letters  not started      │
│                                          │
│     ↑/↓ mode · enter play · esc back     │
│                                          │
╰──────────────────────────────────────────╯
```

one puzzle a day in each mode, the same board for everyone who plays it.

## your puzzles

```
╭─ puzzles ──────────────────────────────────── × ╮
│                                                 │
│                     puzzles                     │
│                                                 │
│      › #715287 5 letters  in play 2/6    –      │
│       #577098 5 letters  solved 2/6     40s     │
│       #360144 5 letters  solved 2/6     47s     │
│       #465966 5 letters  solved 2/6     54s     │
│       #864121 5 letters  solved 2/6     1:01    │
│       #462410 5 letters  solved 2/6     1:08    │
│                                                 │
│   ↑/↓ move · enter open · d delete · esc menu   │
│                                                 │
╰─────────────────────────────────────────────────╯
```

everything you've played, newest first. <kbd>enter</kbd> resumes, <kbd>d</kbd>
twice deletes. deleting a puzzle really does destroy it — the answer and your
guesses are gone — but your streak still knows a game happened there, so
erasing a loss won't quietly hand you a longer best streak.

## your profile

```
╭─ profile ───────────────────────────────────────────────────── × ╮
│                                                                  │
│   played              won                 win rate               │
│   5                   5                   100%                   │
│                                                                  │
│   avg attempts        avg time            streak                 │
│   2                   54s                 5 (max 5)              │
│                                                                  │
│   guess distribution                                             │
│    2 ████████████████████████ 5                                  │
│                                                                  │
│   by mode                                                        │
│   5 letters  5 played      100%        avg 2 in 54s              │
│                                                                  │
╰──────────────────────────────────────────────────────────────────╯
```

averages cover wins only, so a bad day doesn't get to inflate your solve time.

## themes

```sh
wortle -themes          # list what's installed, and where the directory is
wortle -theme dracula   # play with a theme without changing your saved choice
```

bundled: `serika-dark` (the default), `serika-light`, `catppuccin-mocha`,
`dracula`, `everforest-dark`, `gruvbox-dark`, `high-contrast`, `matrix`,
`nord`, `rose-pine`, `solarized-dark`, `terminal`, `tokyo-night`.

yours go in `~/.config/wortle/themes`. a whole theme can be two lines:

```toml
name = "just the accent"
accent = "#ff0088"
```

anything you leave out keeps its built-in value. glyphs and spacing are themeable too. full
reference in [docs/THEMES.md](docs/THEMES.md).

## settings

the settings screen holds two things: the mode new puzzles start in, and
whether playing a different mode makes *that* the new default. both save as you
change them.

```sh
wortle -length 6        # start in a mode for one run, without saving it
wortle -day 2026-08-06  # play another date's daily, without waiting for it
wortle -data ./scratch  # keep saves and settings somewhere else
```

saves and settings otherwise live in your user config directory
(`~/.config/wortle` on linux).

## development

```sh
go run .                    # play from a clone
go test ./...
go test -race ./internal/...
go run ./tools/genwords     # regenerate the embedded word lists
```

`internal/game`, `store`, `words` and `stats` are the ui-agnostic core;
`internal/ui` is the bubble tea layer on top. the split is deliberate — the same
core is meant to be drivable by a server later. the ui has tests too: they drive
the root model with synthetic key and mouse events and assert on the rendered
frame, so no tty is involved.

word-list provenance in
[internal/words/data/SOURCES.md](internal/words/data/SOURCES.md).

---

<div align="center">

inspired by [monkeytype](https://monkeytype.com) · mit licensed

</div>
