# security

Surmise is a single-player game that runs offline. There is no account, no
server and no telemetry, so the surface is small — but the app does read text it
did not write, and this page says how to report a problem with that safely.

## Supported versions

Security fixes land on `main` and ship in the next tagged release. Only the
latest release is supported; there are no backports to older tags. The browser
build at <https://surmise.nxck.dev> tracks `main` and is updated by deployment,
not by tag.

## Reporting a vulnerability

Use **GitHub's private vulnerability reporting** for anything that should not be
public before a fix exists: open the repository's *Security* tab and choose
*Report a vulnerability*. That reaches the maintainer privately; please do not
open an issue for it.

Include what you can of: the version (`surmise -version`) or commit, the
platform (native or browser), and a minimal file or sequence that shows the
problem. A theme file or save record that triggers it is worth its weight in
description.

You will get an acknowledgement within a few days and a follow-up when the
report is triaged. If you would like credit in the release notes, say so and how
you want to be named; otherwise report anonymously and it stays that way.

## What counts

In scope:

- The native binary parsing untrusted input — a theme `.toml` from someone else,
  a hand-edited or corrupt save record, or an imported backup file.
- The WebAssembly bundle and `web/boot.js`: the localStorage bridge, the OSC 52
  clipboard handler, and the functions published on `globalThis.surmise`.
- The deployed site itself — serving content that was not built from this
  repository, or scripts injected beyond the app's own bundle.
- Anything that lets a puzzle record, settings file or backup escape the data
  directory without the player asking it to.

Not in scope:

- **Cheating at your own local game.** Answers live in plaintext inside saved
  puzzles by design — the format is documented as local history, not as an
  anti-cheat boundary. Reading your own saves is a feature, not a finding.
- A malicious theme or save that damages only the install that loaded it. Files
  you chose to put in your own data directory are trusted input; hostile files
  are only interesting if they cross that line (escape the directory, execute
  code, persist past deletion).
- The vendored Bubble Tea copy's upstream defects — those belong to
  [bubbletea](https://github.com/charmbracelet/bubbletea/security), though say so
  in the report and the local copy will be patched too.

## Design notes that bound the risk

- The dependency set is deliberately three Charm modules; everything else,
  including the TOML-ish theme reader and UUID generation, is hand-rolled and in
  this repository.
- Saves are written atomically (temp file + rename) under the user config
  directory, and nothing leaves the machine.
- Backups may only ever add when imported: `internal/backup` never overwrites or
  removes a record, so a hostile archive cannot destroy existing history.
- Records carry a schema version and a reader refuses unknown ones rather than
  guessing.
