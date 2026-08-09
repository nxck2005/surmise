# bubbletea, patched for WebAssembly

Upstream: `charm.land/bubbletea/v2` **v2.0.8**, MIT licensed. `LICENSE` beside
this file is upstream's, unchanged.

## Why the copy exists

Bubble Tea ships terminal glue for Unix and Windows only. `tty_unix.go` and
`signals_unix.go` name their platforms in an explicit build tag
(`darwin || dragonfly || freebsd || …`), and the Windows files match on
`windows`. WebAssembly matches neither, so four symbols go undefined and the
package does not compile for `js` or `wasip1`:

```
tea.go:690  p.listenForResize undefined
tea.go:772  undefined: suspendSupported
tty.go:18   undefined: suspendProcess
tty.go:28   p.initInput undefined
```

## The patch

Two **added** files. No upstream file is edited, which is what keeps an upgrade
to a copy-and-drop:

| File | Defines |
|---|---|
| `tty_js.go` | `initInput`, `suspendSupported`, `suspendProcess` |
| `signals_js.go` | `listenForResize` |

Both are tagged `//go:build js || wasip1`.

## Why not a `replace` directive

`go help install` says the module named on a `go install <module>@latest` line
"must not contain directives (replace and exclude) that would cause it to be
interpreted differently than if it were the main module". The README documents
that command, so a `replace` in the root `go.mod` would break every install.

A workspace does the same redirection for anyone working inside the repository
and leaves the published `go.mod` alone. See `go.work`.

## Never run `go work sync`

It rewrites every member module's `go.mod` and `go.sum`, including this copy's,
which is drift by definition and fails `scripts/check-bubbletea.sh`. The
workspace resolves one build list across its members already, so the copy's
`go.mod` never needs to agree with the root's. Leave both files exactly as
upstream wrote them.

## What is not copied

Tests, `testdata`, `examples`, `tutorials`, `.github` and `Taskfile.yaml`. They
carry test-only dependencies and do not affect the build.

## Upgrading

```sh
# 1. change the version in the root go.mod, then:
scripts/update-bubbletea.sh
# 2. prove the copy is upstream plus exactly the two patch files:
scripts/check-bubbletea.sh
```

`scripts/check-bubbletea.sh` also runs in CI, so drift fails the build rather
than reaching a release.

## Removing the copy

The same patch is offered upstream. When it is released, delete
`third_party/bubbletea` and `go.work`, drop the `check-bubbletea` step from
`.github/workflows/ci.yml`, and raise the `charm.land/bubbletea/v2` version in
`go.mod`. Nothing else refers to the copy.
