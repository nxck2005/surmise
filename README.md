<div align="center">

# surmise

**the word game for the terminal.** fast, themeable, remembers where you left off.

[![ci](https://github.com/nxck2005/surmise/actions/workflows/ci.yml/badge.svg)](https://github.com/nxck2005/surmise/actions/workflows/ci.yml)
[![go reference](https://pkg.go.dev/badge/github.com/nxck2005/surmise.svg)](https://pkg.go.dev/github.com/nxck2005/surmise)
![go](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)
[![built with bubble tea](https://img.shields.io/badge/built%20with-bubble%20tea-ff69b4)](https://charm.land)
![themes](https://img.shields.io/badge/themes-13-e2b714)
![Surmise main Screenshot](assets/demo/1.png)
</div>

```sh
go install github.com/nxck2005/surmise@latest
surmise

# shell can't find it? add go's bin directory to your PATH:
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc && source ~/.zshrc    # zsh
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc && source ~/.bashrc  # bash
fish_add_path (go env GOPATH)/bin                                                 # fish
```

no go toolchain? every release carries a prebuilt binary:

| platform | file |
| --- | --- |
| linux | `surmise_<version>_linux_amd64.tar.gz` · `..._linux_arm64.tar.gz` |
| macos | `surmise_<version>_darwin_arm64.tar.gz` (apple silicon) · `..._darwin_amd64.tar.gz` (intel) |
| windows | `surmise_<version>_windows_amd64.zip` · `..._windows_arm64.zip` |

each release also ships a `checksums.txt`:

```sh
sha256sum -c checksums.txt --ignore-missing
```

## why you'd want it

**more options for word sizes.** play 4, 5 or 6 letters! shorter is
tighter, longer gives you a guess more room. pick which one you open on, or let
it just follow whatever you played last.

**quit whenever.** every guess is saved the moment you make it. come back whenever!

**it keeps score.** win rate, average attempts, average solve time, current and
best streak, guess distribution, all broken down by mode.

**wide assortment of themes, live preview.**  ember, catppuccin, dracula, gruvbox,
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

![Dailies Screenshot](assets/demo/2.png)

one puzzle a day in each mode, the same board for everyone.

## your puzzles

![Puzzles Screenshot](assets/demo/3.png)

everything you've played, newest first. <kbd>enter</kbd> resumes, <kbd>d</kbd>
twice deletes.

## your profile

![Profile Screenshot](assets/demo/4.png)

averages cover wins only, so a bad day doesn't get to inflate your solve time.

## themes

```sh
surmise -themes          # list what's installed, and where the directory is
surmise -theme dracula   # play with a theme without changing your saved choice
```

bundled: `ember-dark` (the default), `ember-light`, `catppuccin-mocha`,
`dracula`, `everforest-dark`, `gruvbox-dark`, `high-contrast`, `matrix`,
`nord`, `rose-pine`, `solarized-dark`, `terminal`, `tokyo-night`.

yours go in `~/.config/surmise/themes`. a whole theme can be two lines:

```toml
name = "just the accent"
accent = "#ff0088"
```

anything you leave out keeps its built-in value. glyphs and spacing are themeable too. full
reference in [docs/THEMES.md](docs/THEMES.md).

## settings

the settings screen holds the mode new puzzles start in, whether playing a
different mode makes *that* the new default, and the splash: whether you get one,
which ascii art it draws, and how it goes away (timed, timed but skippable, or
waiting for a key). everything saves as you change it.

```sh
surmise -length 6        # start in a mode for one run, without saving it
surmise -splash off      # skip the startup art for one run
surmise -splash random   # a different banner each launch
surmise -day 2026-08-06  # play another date's daily, without waiting for it
surmise -data ./scratch  # keep saves and settings somewhere else
```

saves and settings otherwise live in your user config directory
(`~/.config/surmise` on linux).

## about

the **about** row in the menu shows app info. for getting the version without opening the app:

```sh
surmise -version         # surmise 0.1.0 (a1b2c3d) go1.26.5 linux/amd64
```

## development

```sh
go run .                    # play from a clone
go test ./...
go test -race ./internal/...
go run ./tools/genwords     # regenerate the embedded word lists
go run ./tools/gennotices   # regenerate THIRD_PARTY_NOTICES.md after a dep change
```

`internal/game`, `store`, `words` and `stats` are the ui-agnostic core;
`internal/ui` is the bubble tea layer on top. the ui has tests too: they drive
the root model with synthetic key and mouse events and assert on the rendered
frame, so no tty is involved.

word-list provenance in
[internal/words/data/SOURCES.md](internal/words/data/SOURCES.md).

## licence

surmise's own code, docs and data are mit licensed — see
[LICENSE](LICENSE).

the linked go modules, the themes adapted from other people's palettes, and the
word lists stay under their own terms. all of them are permissive, all of them
are reproduced in full in
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md), and that file ships inside
every release archive. regenerate it with `go run ./tools/gennotices` after any
dependency change.

not affiliated with the new york times company or any other rights holder in a
similar game.

---

<div align="center">

inspired by [monkeytype](https://monkeytype.com) · mit licensed

</div>
