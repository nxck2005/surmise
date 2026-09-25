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

## The two files are pinned by digest

`PATCHES.sha256` holds a sha256 for each, and `scripts/check-bubbletea.sh`
verifies it. This is worth a paragraph of its own because of where the gap
otherwise is.

Everything the drift check does is a `diff` against upstream, and a diff cannot
say anything about the contents of a file upstream does not have. These two are
exactly that. They are also the only bytes in the tree that are ours, and being
`js || wasip1` they are excluded from a native build, from `go vet`, from the
`gofmt` diff over tracked files, and from every test the three-OS matrix runs.
Checking that they *exist* therefore left the one file class in the repository
where an edit of any kind — including one nobody intended — reached a browser
build without a single check objecting.

An upgrade does not change them, so the digest survives one. If the patch is
ever edited — a new js symbol, a fix carried down from upstream — the check
fails, and that is the intended outcome: update `PATCHES.sha256` in the same
commit, and say there why.

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
# 2. prove the copy is upstream plus exactly the two patch files, and that the
#    patch files are still the ones that were reviewed:
scripts/check-bubbletea.sh
```

`scripts/check-bubbletea.sh` also runs in CI, so drift fails the build rather
than reaching a release. It checks two things: that every file upstream has is
byte-identical here, and that the two files upstream does not have match their
pinned digests.

## Removing the copy

The same patch is offered upstream. When it is released, delete
`third_party/bubbletea` and `go.work`, drop the `check-bubbletea` step from
`.github/workflows/ci.yml`, and raise the `charm.land/bubbletea/v2` version in
`go.mod`. Nothing else refers to the copy.
