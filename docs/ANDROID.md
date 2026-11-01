# surmise on android

The same game as a native Android app: the same rules, the same words, the
same dailies, the same save format and the same server. A backup made on the
phone restores in the terminal, and the other way round.

## Install

Download `surmise_<version>_android.apk` from the [latest
release](https://github.com/nxck2005/surmise/releases/latest) and open it on
the phone. It needs Android 8 or newer.

The app is distributed on GitHub only for now. There is no Play Store listing.

A newer APK installs over the old one and keeps your saves. Do not uninstall
first unless you have a backup: uninstalling deletes the app's private files.
Updates must be signed with the same key as the installed app; a build signed
with another key is refused.

## What is different from the terminal

The rules and the data are the same. These are the parts that are terminal
constraints rather than game rules:

| terminal | android |
|---|---|
| startup splash art and its settings | not ported; a phone uses the system launch screen |
| a theme folder with live reload, and `r` to reload | not needed; themes come from the bundle and from restored backups |
| the `terminal` theme | hidden; a phone has no terminal palette for ANSI numbers to mean |
| mouse hover and the hit map | touch |
| flags and a command line | the features that make sense on a phone are screens: version in about, backup and restore in settings, and so on. `-day`, `-splash` and `-data` have no phone equivalent. |
| OSC 52 copy | the system clipboard and the share sheet |
| tile outlines | not shown; tiles are drawn shapes, not terminal cells |
| every keybind | every action is a tap; the board also accepts a hardware keyboard |

Every terminal-only setting is still kept untouched in `settings.json` and in
a backup, so a phone → terminal backup loses nothing.

## Your data

Everything is in the app's private files directory, under `surmise/`, in the
same layout as the terminal:

- `puzzles/` — one JSON file per puzzle.
- `settings.json` — preferences and the playtime counter.
- `themes/` — themes a backup restored.
- `rooms.json` — the rooms this install is in.

**Android Auto Backup** carries the puzzle records and the themes, and leaves
out `settings.json` and `rooms.json`. The first holds the network consent, and
only the settings screen may turn that on; the second holds this device's seat
keys for online rooms. Playtime re-floors from the records if the counter is
lost (`stats.Playtime`).

**The backup row in settings** writes the same archive as the terminal's
`-export`, and loads one the same way. It uses Android's file picker, so the
file can go anywhere the phone can write. An import only ever adds: a puzzle
you already have is left as it is, a setting you have already chosen is not
overwritten, and playtime only goes up.

Uninstalling the app deletes its private files. Save a backup first.

## Network

Off until you turn it on in settings. With it on, the app uses the same server
as the terminal, at <https://surmise.nxck.dev>:

- finishing a daily sends whether you solved it and in how many guesses — no
  name and no id;
- the daily screen shows how everyone did on the modes you have finished;
- the menu shows how many dailies everyone has played today;
- rooms work as they do in the terminal.

A failed call shows nothing. If the server turns this version away (`410`), the
app says so once and stops using the network for the rest of the run. Nothing
needs the network to play.

## Themes

The app draws the bundled themes, including `night` and `day`. It hides
`terminal`, which is built from ANSI numbers and means "use the terminal's
colours".

A theme can set the app's own colours under `[app]` and `[app.colors]`; the
rest are derived from the terminal palette. See [THEMES.md](THEMES.md) for the
keys, the contrast rules and the accessibility tile palettes.

The phone's look preferences — theme, light/dark/system, tile palette and
haptics — live in the app's own preferences, not in `settings.json`, so a
backup does not carry them. The app's colour-blind and high-contrast tile
settings replace every theme's tiles, so the board stays readable whatever
theme you pick.

## Building it yourself

The engine is the same Go code as the terminal, bound with gomobile. `mobile/`
is its own Go module: run every Go command there with `GOWORK=off`, never add it
to `go.work`, and never run `go work sync`.

```sh
export ANDROID_HOME=~/Android/Sdk
scripts/build-android-core.sh      # writes android/core/libs/core.aar
cd android
./gradlew assembleDebug
```

Gradle never runs Go. Run `scripts/build-android-core.sh` before the first
Android build and again after any change under `mobile/` or `internal/`. It
uses the NDK version pinned in `android/app/build.gradle.kts`; r28 or newer is
required for 16 KB pages.

The generated files are committed:

- `mobile/cmd/gen` writes
  `android/core/src/main/java/com/nxck/surmise/core/gen/Api.kt` from the
  engine's method registry.
- `tools/genandroid` writes `android/app/src/main/res/values/brand.xml` and
  the bundled theme tokens under `android/design/src/main/assets/tokens/`.

CI checks both are current. The app name comes from `internal/brand`; never
type it into the Android project.

Tests:

```sh
cd android
./gradlew check assembleDebug verifyRoborazziDebug
```

Screenshot baselines are under `app/src/test/screenshots`. After a deliberate
visual change, run `./gradlew recordRoborazziDebug` and look at every changed
image before committing it. The Go engine tests run separately:

```sh
cd mobile
GOWORK=off go test ./...
```

The Android CI job also checks every `.so` for 16 KB LOAD alignment and the
APK with `zipalign -c -P 16`.

## Releasing

The repository tag is the app's version. `v0.12.0` gives `versionName=0.12.0`;
`v0.12.0-rc1` gives `versionName=0.12.0-rc1`. The `versionCode` is
`major*10,000,000 + minor*10,000 + patch*10 + n`, where `n` is N for `-rcN`
(1–8) and 9 for a final, so an rc always sorts below its release.

`release.yml` builds one universal APK named
`surmise_<version>_android.apk`, signs it, and adds it to `checksums.txt` and
the build provenance attestation. The signing key comes from four repository
secrets:

- `ANDROID_KEYSTORE_BASE64`
- `ANDROID_KEYSTORE_PASSWORD`
- `ANDROID_KEY_ALIAS`
- `ANDROID_KEY_PASSWORD`

The key is permanent. **If it is lost, no update can reach existing installs.**
Keep the keystore and its passwords offline, in two places, and never commit
them. The same key is intended for Play App Signing if a Play release is added
later.

There is no Play Store build in this project: no AAB, store listing, data
safety form or closed test. Distribution is the GitHub APK only.

## Known gaps

These are known and are planned for a later release:

- A saved room round opened from history shows the board, not the ranking.
- The motion setting's `calm` and `full` choices draw the same.
- Back during a sprint ends the run without the summary.
- The history has no filter by day.

## Licence

surmise's own code is MIT. The Geist fonts are under the SIL Open Font
Licence; the licence text ships in the app and is on its about screen. The
Android libraries are under their own licences, reproduced in
[THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md) for the Go side and in the
app's dependency metadata for the Android side.
