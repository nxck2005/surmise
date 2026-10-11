# security

Surmise is a word game that runs offline unless the player turns network on.
There is no account and no telemetry. With network on it talks to one small API
at surmise.nxck.dev/api/v1, described below. The surface is small — but the app
does read text it did not write, and this page says how to report a problem
with that safely.

## Supported versions

Security fixes land on `main` and ship in the next tagged release. Only the
latest release is supported; there are no backports to older tags. The browser
build at <https://surmise.nxck.dev> is the latest release too: a release tag
deploys it, and a push to `main` goes only to a staging copy.

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
- A link to the browser build. Its query parameters (`?theme=`, `?day=`,
  `?challenge=` and the rest) are chosen by whoever wrote the link.
- The deployed site itself — serving content that was not built from this
  repository, or scripts injected beyond the app's own bundle.
- Anything that lets a puzzle record, settings file or backup escape the data
  directory without the player asking it to.
- The API under `https://surmise.nxck.dev/api/v1` — the Worker script in
  `worker/`: its input validation and rate limits, anything that lets a
  request read or change data it should not, and anything that makes the
  Worker serve code other than this repository's.
- How the game handles what the server sends back.

Not in scope:

- **Cheating at your own local game.** Answers live in plaintext inside saved
  puzzles by design — the format is documented as local history, not as an
  anti-cheat boundary. Reading your own saves is a feature, not a finding.
- A theme or save you wrote yourself, when its only effect is on your own
  install. What you write there is trusted input.
- The vendored Bubble Tea copy's upstream defects — those belong to
  [bubbletea](https://github.com/charmbracelet/bubbletea/security), though say so
  in the report and the local copy will be patched too.
- A flood of requests against the API. It has rate limits, but absorbing a
  determined flood is Cloudflare's job, not a defect in this code.

The line is the file you open, not the directory it ends up in. A backup or a
theme that came from somebody else is in scope even though restoring it puts it
in your data directory: so is one that leaves the app unusable, slow or
unrecoverable through its own screens, because sharing these files is what the
features invite.

## What the server stores

- **Daily counts.** For each day and word length: how many players solved it
  in each number of guesses, and how many did not. No name, no id, no IP
  address. Kept 90 days, then deleted by a daily job.

The Worker writes no logs of its own. Cloudflare's request logs are outside
this repository.

## Design notes that bound the risk

- The dependency set is deliberately four Charm modules; everything else,
  including the TOML-ish theme reader and UUID generation, is hand-rolled and in
  this repository.
- Saves are written atomically (temp file + rename) under the user config
  directory, and nothing leaves the machine.
- Backups may only ever add when imported: `internal/backup` never overwrites or
  removes a record, so a hostile archive cannot destroy existing history.
- Records carry a schema version and a reader refuses unknown ones rather than
  guessing.
