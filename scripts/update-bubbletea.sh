#!/usr/bin/env bash
# Refresh third_party/bubbletea from the version the root go.mod requires,
# keeping the two js patch files. See third_party/bubbletea/PATCHES.md.
set -euo pipefail

cd "$(dirname "$0")/.."

dst=third_party/bubbletea
mod=charm.land/bubbletea/v2

# The version comes from go.mod so there is one place it lives, the same rule
# the workflows follow with go-version-file.
version=$(GOWORK=off go list -m -f '{{.Version}}' "$mod")
echo "upstream: $mod@$version"

GOWORK=off go mod download "$mod"
src="$(go env GOMODCACHE)/$mod@$version"
[ -d "$src" ] || { echo "not in the module cache: $src" >&2; exit 1; }

# Keep the patch files across the refresh.
tmp=$(mktemp -d)
trap 'chmod -R u+w "$tmp" 2>/dev/null; rm -rf "$tmp"' EXIT
cp "$dst"/tty_js.go "$dst"/signals_js.go "$dst"/PATCHES.md "$tmp/"

rm -rf "$dst"
mkdir -p "$dst"
# tar rather than rsync: rsync is not installed everywhere, tar is.
tar -C "$src" -cf - \
	--exclude='*_test.go' --exclude='testdata' --exclude='Taskfile.yaml' \
	--exclude='.github' --exclude='examples' --exclude='tutorials' . |
	tar -C "$dst" -xf -

# The module cache is read-only.
chmod -R u+w "$dst"
cp "$tmp"/tty_js.go "$tmp"/signals_js.go "$tmp"/PATCHES.md "$dst/"

echo "updated $dst to $version; now run scripts/check-bubbletea.sh"
