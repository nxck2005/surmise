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
#
# The three excluded names are ours and ours alone. PATCHES.md and PATCHES.sha256
# are documentation and a manifest, and the two .go files are the patch; every
# other file in the tree is compared byte for byte with upstream, which is what
# makes the copy auditable.
if diff -r -q \
	--exclude=tty_js.go --exclude=signals_js.go \
	--exclude=PATCHES.md --exclude=PATCHES.sha256 \
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

# Existence is not enough for the two files the copy exists for.
#
# Everything above this line is a diff against upstream, which by construction
# cannot say anything about the contents of a file upstream does not have — and
# these two are exactly that. They are also the only place in the vendored tree
# whose bytes are ours, and they are tagged js/wasip1, so a native build, go vet,
# the gofmt diff and this diff all pass with anything at all in them while it
# ships in every browser build. Checking that they exist leaves the one file
# class in the repository with no content integrity control.
#
# So their contents are pinned in PATCHES.sha256, beside the files, and a
# change to either is a change to the patch: it fails here, it fails in CI, and
# it has to be made deliberately in a review that says so.
cd "$dst"

# The manifest has to name both files before sha256sum is asked about anything.
#
# sha256sum --check exits 0 when the lines it could parse all match, and says
# nothing about a line it could not parse or about a file that is not listed at
# all. So a manifest that has lost a line verifies the other one and reports
# success — which would leave a file unpinned and the check green, the exact
# outcome this is here to prevent. The count and the names are asserted first,
# so the pin cannot be dropped by accident.
pinned=$(grep -c '^[0-9a-f]\{64\}  ' PATCHES.sha256 || true)
if [ "$pinned" -ne 2 ]; then
	echo "PATCHES.sha256 holds $pinned well-formed lines, want 2" >&2
	exit 1
fi
for f in tty_js.go signals_js.go; do
	grep -q "  $f\$" PATCHES.sha256 || {
		echo "PATCHES.sha256 does not pin $f" >&2
		exit 1
	}
done

sha256sum --check PATCHES.sha256 || {
	cat >&2 <<-EOF

		the js patch files have changed.

		tty_js.go and signals_js.go are pinned by digest in PATCHES.sha256, so
		an edit to either is caught here rather than shipped. If the change is
		intended — a new js symbol, a fix carried down from upstream — update the
		digest deliberately, in a commit that also says why, and note it in
		third_party/bubbletea/PATCHES.md.
	EOF
	exit 1
}
echo "the js patch files match their pinned digests"
