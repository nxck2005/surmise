# Writing a surmise theme

A theme is one file. Drop it in your themes directory, and it appears in the
picker (`esc` → `themes`). Sharing a theme means sending someone that file.

```sh
surmise -themes          # where the directory is, and what is in it
surmise -theme dracula   # start with a theme, without changing your saved choice
```

The directory is `themes/` inside your data dir — `~/.config/surmise/themes` by
default, or `<dir>/themes` when you pass `-data <dir>`. It is created on first
run with an `example.toml` in it, which is a copy of the default theme with a
header explaining itself: the fastest way to start is to edit that.

**Edit with the app running.** The directory is re-read about once a second, so
saving a file recolours what is on screen: open the picker on your theme and
every save shows up there, or keep the theme selected and edit while you play.
A new file appears in the list without moving your place in it. `r` in the
picker reloads at once, for the rare edit the check misses — one that changes no
byte count and lands in the same instant.

A theme that is deleted, or that stops being readable, falls back to the default
rather than leaving the screen mid-edit. A theme with a *mistake* in it is not
that: the bad line is skipped, the rest of the file still applies, and the
picker prints the warning (see "When something is wrong").

## The format

Comments start with `#`. Everything else is `key = value`, optionally grouped
under a `[section]` header. Every key is optional — whatever you leave out keeps
its built-in value, so a four-line theme is a perfectly good theme:

```toml
name = "just the accent"
accent = "#ff0088"
```

Colours accept:

| form | example | meaning |
|---|---|---|
| hex | `"#e2b714"`, `"#eb1"` | an exact colour |
| ANSI number | `3`, `202` | 0–255, resolved by your terminal's own palette |
| palette name | `"accent"` | whatever that entry is set to |

## Colours

Set these at the top level or under `[colors]`; the two are identical.

| key | what it colours |
|---|---|
| `bg` | the terminal background |
| `text` | primary text |
| `accent` | the one emphasis colour: titles, the cursor, the caret, wins |
| `muted` | secondary text, the panel border, the help bar |
| `error` | failures, the close box, out-of-guesses |
| `correct` | right letter, right place |
| `present` | right letter, wrong place |
| `absent` | letter not in the word |
| `slot` | an empty board cell |
| `key_face` | an untouched keycap |
| `key_spent` | a keycap for a letter known absent |

These follow the ones above unless you set them, which is what makes a light
theme work — on a pale ground the letters inside a filled tile want to stay dark
rather than match the background:

| key | follows |
|---|---|
| `correct_text`, `present_text` | `bg` |
| `absent_text` | `muted` |
| `key_correct_text`, `key_present_text` | `bg` |
| `key_absent_text` | `muted` |
| `key_unused_text` | `text` |

## Glyphs

Characters the UI draws. **These change the layout**, so check a theme that
touches them in a real terminal.

```toml
[glyphs]
border       = "rounded"  # rounded | normal | thick | double | hidden | block
caret        = "_"        # where the next letter lands
empty        = "·"        # an unfilled board cell
cursor       = "› "       # selection marker, left
cursor_right = " ‹"       # selection marker, right (menu rows are flanked)
separator    = " · "      # between hints on the help bar
value_prev   = "‹"        # settings row: step to the previous value
value_next   = "›"        # settings row: step to the next value
jump_first   = "⇱"        # scrolling list: jump to the first row
jump_last    = "⇲"        # scrolling list: jump to the last row
enter        = "⏎"
delete       = "⌫"
bar          = "█"        # the profile histogram
close        = "×"        # the panel's close box
```

## Metrics

Also layout-changing. Tile *height* is not themeable: the board draws one row
tall, and grows to three by itself when the terminal has room for the whole
screen at that size. Width is the knob you have.

```toml
[metrics]
tile_width  = 7   # display cells per board tile
key_pad_x   = 2   # padding inside a keycap
panel_pad_x = 3   # padding inside the panel border
panel_pad_y = 1
```

## Per-element styles

For when the palette is not enough. Each element takes any of `fg`, `bg`,
`bold`, `italic`, `underline`, `faint`; anything you omit keeps whatever the
palette already decided.

```toml
[style.title]
italic = true

[style.tile.correct]
bold = false
fg   = "text"      # instead of the usual bg-coloured letter

[style.hover]       # the "your pointer is on this" cue; underline by default
bold      = true
underline = false
```

Elements: `title`, `text`, `muted`, `accent`, `error`, `help`, `help_hover`,
`hover`, `border`, `panel_title`, `menu_selected`, `cursor`, `caret`, `bar`,
`splash`, `tile.correct`, `tile.present`, `tile.absent`, `tile.active`, `tile.empty`,
`key.unused`, `key.correct`, `key.present`, `key.absent`, `status.won`,
`status.lost`, `status.playing`.

`splash` is the ASCII art drawn at startup — bold and accent-coloured by
default, and worth quietening (`fg = "muted"`, `bold = false`) in a theme that
wants a calmer opening. Which
art it draws is a setting rather than part of the theme, so a theme colours
whatever banner the player picked.

The legend on the puzzle screen (`A correct spot · A wrong spot · A not in
word`) has no element of its own: it is drawn with `tile.correct`,
`tile.present` and `tile.absent`, so whatever you do to the tiles it explains,
it does too. Note that a large `tile_width` makes the legend the widest thing on
the screen — surmise hides it rather than overflow a narrow terminal.

Some of the frame is **blended** rather than filled: the panel's top and bottom
rules run from `accent` at the corners into `border` across the middle. Those
are your colours, mixed — a gradient never introduces a colour the theme did not
choose, so it needs no key of its own and every theme gets it. On a terminal
with fewer than 256 colours the blend is dropped and the rule is drawn flat in
`border`, which is what it always was.

The board's feedback animations likewise name nothing of their own. A submitted
row turns from `tile.active` into its scored tiles, a refused guess flashes
`error`, a struck keycap borrows `hover`, and a win eases `border` toward
`accent` and back — so colouring those colours the animations too, and a theme
never has to think about them.

Two effects brighten a colour instead of borrowing another one: the band of
light that crosses the startup art once, and the tile that lights as it walks a
solved word. Both are `splash` and the scored tiles themselves, lifted — so a
theme that quietened the art keeps it quiet, and neither can clash with a
palette it has never seen. How much of it happens, or whether it happens at all, is a
setting rather than part of the theme.

## When something is wrong

A line the parser cannot read becomes a warning naming its line number; the rest
of the theme still loads. The picker shows those warnings under the theme, and
`surmise -themes` prints them:

```
  broken                   ~/.config/surmise/themes/broken.toml
      line 2: bad hex colour "#nothex"
      line 3: unknown key "wut"
```

A theme whose `name` matches a built-in one replaces it, so you can adjust a
bundled theme by copying it out and editing the copy. Drop the `name` line and
the theme is called after its file.

## Crediting a theme

`author` is a free-text line beside `name`, shown under the theme in the picker:

```toml
name = "midnight"
author = "you"
```

It is worth setting on anything you did not invent. Every bundled theme adapted
from someone else's palette carries one, which is both a courtesy to whoever
designed the colours and how their licence notice stays attached to them — see
`THIRD_PARTY_NOTICES.md` at the repo root.

## The bundled themes

`tokyo night` (the default), `ember dark`, `ember light`, `dracula`, `nord`,
`gruvbox dark`, `catppuccin mocha`, `rose pine`, `solarized dark`,
`everforest dark`, `high contrast` (a colour-blind-friendly orange/blue scheme),
`terminal` (built entirely from ANSI numbers, so it follows
your terminal's own palette) and `matrix`. They live in
`internal/theme/themes/*.toml` and are written in exactly this format — every
one of them is a worked example.
