#!/usr/bin/env bash
# Build the browser bundle into web/dist.
#
#   scripts/build-web.sh [version]
#
# Serve the result with any static file server; there is nothing dynamic in it.
set -euo pipefail

cd "$(dirname "$0")/.."

version="${1:-${SURMISE_VERSION:-dev}}"
out=web/dist

rm -rf "$out"
mkdir -p "$out/vendor"

# The wasm and wasm_exec.js must come from the same toolchain — a mismatched
# pair fails to instantiate — which is why wasm_exec.js is copied here every
# time and never committed.
echo "building $version for js/wasm"
GOOS=js GOARCH=wasm go build -trimpath \
	-ldflags "-s -w -X github.com/nxck2005/surmise/internal/build.version=$version" \
	-o "$out/surmise.wasm" .

goroot=$(go env GOROOT)
if [ -f "$goroot/lib/wasm/wasm_exec.js" ]; then
	cp "$goroot/lib/wasm/wasm_exec.js" "$out/"
elif [ -f "$goroot/misc/wasm/wasm_exec.js" ]; then
	# Go moved this in 1.24; keep working on an older toolchain.
	cp "$goroot/misc/wasm/wasm_exec.js" "$out/"
else
	echo "cannot find wasm_exec.js under $goroot" >&2
	exit 1
fi

# xterm.js is pinned in web/package.json and copied out of node_modules rather
# than fetched from a CDN, so a build is reproducible and the page works with no
# third-party host involved.
npm ci --prefix web --silent
cp web/node_modules/@xterm/xterm/lib/xterm.js "$out/vendor/"
cp web/node_modules/@xterm/xterm/css/xterm.css "$out/vendor/"
cp web/node_modules/@xterm/addon-fit/lib/addon-fit.js "$out/vendor/"

cp web/index.html web/style.css web/boot.js "$out/"

# Cache-bust the wasm. index.html is served with no-cache and names a URL that
# changes with the build, so a deploy is picked up immediately while the wasm
# itself can be cached hard.
stamp=$(sha256sum "$out/surmise.wasm" | cut -c1-12)
sed -i.bak "s|return \"surmise.wasm\";|return \"surmise.wasm?v=$stamp\";|" "$out/boot.js"
rm -f "$out/boot.js.bak"

size=$(wc -c <"$out/surmise.wasm")
printf 'built %s\n  wasm: %.1f MB raw\n' "$out" "$(echo "$size" | awk '{print $1/1048576}')"
echo "  serve it with: python3 -m http.server -d $out"
