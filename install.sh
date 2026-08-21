#!/bin/sh
# Installs the latest surmise release for your platform.
#
#   curl -fsSL https://raw.githubusercontent.com/nxck2005/surmise/main/install.sh | sh
#
# The binary lands in $SURMISE_INSTALL_DIR (default: ~/.local/bin). A specific
# version can be pinned with SURMISE_VERSION=v0.5.1. Windows is not handled
# here — take an archive from https://github.com/nxck2005/surmise/releases
# (a Scoop manifest is planned; until then that page is the whole story).
set -eu

REPO=nxck2005/surmise
DEST=${SURMISE_INSTALL_DIR:-"$HOME/.local/bin"}
WANT=${SURMISE_VERSION:-}

need() {
    command -v "$1" >/dev/null 2>&1 || {
        echo "install.sh: $1 is required but was not found" >&2
        exit 1
    }
}

need uname
need grep
need sha256sum || need shasum
FETCH=""
for c in curl wget; do
    if command -v "$c" >/dev/null 2>&1; then FETCH=$c; break; fi
done
[ -n "$FETCH" ] || {
    echo "install.sh: curl or wget is required" >&2
    exit 1
}

get() {
    # get URL FILE — fetch through whichever client we found, quietly.
    if [ "$FETCH" = curl ]; then
        curl -fsSL "$1" -o "$2"
    else
        wget -qO "$2" "$1"
    fi
}

case "$(uname -s)" in
Linux) os=linux ;;
Darwin) os=darwin ;;
MINGW* | MSYS* | CYGWIN*)
    echo "install.sh: no windows installer yet — take a zip from" >&2
    echo "  https://github.com/$REPO/releases/latest" >&2
    exit 1
;;
*)
    echo "install.sh: unsupported operating system: $(uname -s)" >&2
    exit 1
;;
esac

case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*)
    echo "install.sh: unsupported architecture: $(uname -m)" >&2
    exit 1
;;
esac

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# resolve_latest prints the newest release tag, e.g. v0.5.1.
#
# The JSON API rate-limits unauthenticated clients by IP, so it cannot be the
# only way in; when it refuses, fall back to following the releases page's
# redirect, which lands on /releases/tag/<tag> and answers the same question.
resolve_latest() {
    t=""
    get "https://api.github.com/repos/$REPO/releases/latest" "$tmp/api.json" 2>/dev/null || :
    t=$(grep -m1 '"tag_name"' "$tmp/api.json" 2>/dev/null | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/') || t=""
    if [ -z "$t" ]; then
        get "https://github.com/$REPO/releases/latest" "$tmp/page.html"
        t=$(grep -om1 '/releases/tag/v[0-9][0-9.]*' "$tmp/page.html") || t=""
        t=${t#/releases/tag/}
    fi
    [ -n "$t" ] || {
        echo "install.sh: could not resolve the latest release" >&2
        exit 1
    }
    printf '%s\n' "$t"
}

if [ -z "$WANT" ]; then
    tag=$(resolve_latest)
else
    tag=$WANT
fi
version=${tag#v}

base="surmise_${version}_${os}_${arch}"
url="https://github.com/$REPO/releases/download/$tag"

echo "fetching surmise $tag ($os/$arch)"
get "$url/$base.tar.gz" "$tmp/pkg.tar.gz"
get "$url/checksums.txt" "$tmp/checksums.txt"

want_sum=$(grep " $base.tar.gz\$" "$tmp/checksums.txt" | awk '{print $1}')
[ -n "$want_sum" ] || {
    echo "install.sh: checksums.txt has no entry for $base.tar.gz" >&2
    exit 1
}
if command -v sha256sum >/dev/null 2>&1; then
    got_sum=$(sha256sum "$tmp/pkg.tar.gz" | awk '{print $1}')
else
    got_sum=$(shasum -a 256 "$tmp/pkg.tar.gz" | awk '{print $1}')
fi
[ "$got_sum" = "$want_sum" ] || {
    echo "install.sh: checksum mismatch for $base.tar.gz" >&2
    echo "  wanted $want_sum" >&2
    echo "  got    $got_sum" >&2
    exit 1
}

tar -xzf "$tmp/pkg.tar.gz" -C "$tmp"
if [ -f "$DEST/surmise" ] && [ "${SURMISE_FORCE:-0}" != 1 ]; then
    echo "install.sh: $DEST/surmise already exists; set SURMISE_FORCE=1 to replace it" >&2
    exit 1
fi

mkdir -p "$DEST"
mv "$tmp/surmise" "$DEST/surmise"
chmod +x "$DEST/surmise"

echo "installed $DEST/surmise"
case ":$PATH:" in
*":$DEST:"*) ;;
*)
    echo "note: $DEST is not on your PATH. add this to your shell profile:"
    echo "  export PATH=\"\$PATH:$DEST\""
;;
esac
