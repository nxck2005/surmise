# surmise in a browser

The same game, compiled to WebAssembly and drawn by [xterm.js]. No install, no
account, no server: the whole thing runs in the tab.

<!-- The live URL goes here once the site is deployed. See the note at the end. -->

## What is different from the terminal build

Three things, and nothing else. The rules, the words, the daily and the profile
are the same code.

| | terminal | browser |
|---|---|---|
| Saved puzzles and settings | files under the config directory | `localStorage`, this browser only |
| Themes | bundled, plus your own `.toml` files | bundled only |
| Theme hot reload | yes | no — there is no directory to watch |

### Your puzzles live in this browser

Everything is kept in `localStorage`, under keys beginning `surmise/v1/`. That
means:

- Clearing site data for this origin erases your history.
- Another browser, another device or a private window is a different history.
- There is no sync and no account.

Private mode in Safari refuses `localStorage` outright. The game notices, falls
back to memory and stays playable — it just forgets everything when you leave.

Two tabs at once is last-write-wins. Play in one.

### Themes

The bundled themes are all there and the picker works as it does in the
terminal. Adding your own means a file on disk, which a browser has none of, so
that part is desktop-only. See [THEMES.md](THEMES.md).

## Options

The flags have query-string equivalents, under the same names:

| URL | same as |
|---|---|
| `?theme=dracula` | `-theme dracula` |
| `?length=6` | `-length 6` |
| `?day=2026-08-06` | `-day 2026-08-06` |
| `?splash=off` | `-splash off` |

Combine them with `&`: `?theme=nord&length=6`.

There is no `?data=`, no `?themes=` and no `?version=`. The first has nothing to
point at, and the other two print to a place nobody can see. The version is on
the about screen.

## Keys the browser keeps

Everything reaches the game except the chords the browser will not give up:
`Ctrl`/`Cmd`+`W`, `Ctrl`/`Cmd`+`T`, `Cmd`+`Q` and friends. Copy and paste are
deliberately left to the browser as well — `Ctrl`/`Cmd`+`C` copies when there is
a selection, and `Ctrl`/`Cmd`+`V` pastes.

The mouse works exactly as it does in a terminal, because xterm.js sends the
same sequences a terminal would.

## Small screens

It loads on a phone and the on-screen keyboard types, but a 6-letter board plus
the keyboard wants about 60 columns by 24 rows. Below that it is cramped. This
is a known limitation, not a bug being worked on.

## Building it yourself

```sh
scripts/build-web.sh              # writes web/dist
python3 -m http.server -d web/dist
```

Then open the address it prints. The build needs Go and npm; `npm ci` installs
the pinned xterm.js into `web/node_modules`.

To check it without a browser:

```sh
GOOS=js GOARCH=wasm go build -o /tmp/surmise.wasm .
node scripts/smoke-web.mjs /tmp/surmise.wasm
```

That drives the real binary against a stub xterm.js and asserts on what it
writes — the alternate screen, mouse tracking, 24-bit colour, typing, and a
resize. CI runs it. Colour, layout and hover still need eyes.

`web/dist` and `web/node_modules` are both ignored by git. `wasm_exec.js` is
copied from the Go toolchain on every build and is never committed: it has to
match the compiler that produced the `.wasm`, and a mismatched pair fails to
instantiate.

## How it fits together

```
xterm.js  --onData-->  reader  --> tea.WithInput  \
                                                   bubbletea Program
xterm.js  <--write--   writer  <-- tea.WithOutput /
xterm.js  --onResize-> channel --> Program.Send(WindowSizeMsg)
```

The bridge is a byte pipe, not a translation layer. xterm.js already turns key
presses and mouse movement into terminal escape sequences, and Bubble Tea
already parses them, so the browser build gets key and mouse parity with the
terminal build for free.

Two implementation notes worth knowing before changing `internal/web`:

- **A `js.Func` callback must never block.** Go on WebAssembly runs on the
  single JavaScript thread. A callback that waits for something another event
  would deliver deadlocks the runtime, and the symptom is a page that draws one
  frame and then freezes. Append and signal without blocking; call
  `Program.Send` from a goroutine only.
- **The Program is handed a stub tty.** Bubble Tea turns on newline mapping when
  standard input is not a tty, and that leaves fragments of a larger frame on
  screen when the frame shrinks. xterm.js behaves like a raw-mode terminal, so
  the stub tells the truth.
- **The colour profile is forced.** The output is not a tty, so detection would
  find no colour and every theme would collapse to monochrome.

Bubble Tea has no WebAssembly build upstream. `third_party/bubbletea` is
v2.0.8 plus two small additive files; see
[PATCHES.md](../third_party/bubbletea/PATCHES.md).

## Not deployed yet

There is no public URL. Everything above works locally today; hosting is the one
remaining step. When there is somewhere to put it, add the address at the top of
this file and in the README.

[xterm.js]: https://xtermjs.org
