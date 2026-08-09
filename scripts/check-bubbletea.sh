#!/usr/bin/env bash
# Prove third_party/bubbletea is upstream plus exactly the two js patch files.
# Runs in CI, so drift fails the build instead of reaching a release.
set -euo pipefail

cd "$(dirname "$0")/.."

dst=third_party/bubbletea
mod=charm.land/bubbletea/v2

version=$(GOWORK=off go list -m -f '{{.Version}}' "$mod")
GOWORK=off go mod download "$mod"
src="$(go env GOMODCACHE)/$mod@$version"
[ -d "$src" ] || { echo "not in the module cache: $src" >&2; exit 1; }

tmp=$(mktemp -d)
# The module cache is read-only, and so is everything unpacked from it.
trap 'chmod -R u+w "$tmp" 2>/dev/null; rm -rf "$tmp"' EXIT
tar -C "$src" -cf - \
	--exclude='*_test.go' --exclude='testdata' --exclude='Taskfile.yaml' \
	--exclude='.github' --exclude='examples' --exclude='tutorials' . |
	tar -C "$tmp" -xf -

# The patch is additive, so the only differences may be files we added.
if diff -r -q \
	--exclude=tty_js.go --exclude=signals_js.go --exclude=PATCHES.md \
	"$tmp" "$dst"; then
	echo "third_party/bubbletea matches $mod@$version plus the patch files"
else
	cat >&2 <<-EOF

		third_party/bubbletea has drifted from $mod@$version.

		The copy must be upstream plus exactly tty_js.go and signals_js.go.
		Run scripts/update-bubbletea.sh, or see third_party/bubbletea/PATCHES.md.
	EOF
	exit 1
fi

for f in tty_js.go signals_js.go; do
	[ -f "$dst/$f" ] || { echo "missing patch file: $dst/$f" >&2; exit 1; }
done
