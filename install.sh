#!/bin/sh
# Installs the latest surmise release for your platform.
#
#   curl -fsSL https://raw.githubusercontent.com/nxck2005/surmise/main/install.sh | sh
#
# The binary lands in $SURMISE_INSTALL_DIR (default: ~/.local/bin). A specific
# version can be pinned with SURMISE_VERSION=v0.5.1. Windows is not handled
# here: use Scoop — https://github.com/nxck2005/scoop-bucket has the manifest,
# and the releases page still carries the raw zips.
set -eu

REPO=nxck2005/surmise
# ${HOME:?} rather than a bare $HOME, so a shell with no HOME says so instead of
# failing on an unset variable.
DEST=${SURMISE_INSTALL_DIR:-"${HOME:?install.sh: HOME is not set}/.local/bin"}
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
    echo "install.sh: windows goes through Scoop, not this script:" >&2
    echo "  scoop bucket add nxck2005 https://github.com/nxck2005/scoop-bucket" >&2
    echo "  scoop install surmise" >&2
    echo "or take a zip from https://github.com/$REPO/releases/latest" >&2
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

# What came out of the archive has to be the binary, and a plain one. An archive
# member can be a symlink, and `mv` and `chmod` both follow one.
[ -f "$tmp/surmise" ] && [ ! -L "$tmp/surmise" ] || {
    echo "install.sh: the archive holds no regular surmise binary" >&2
    exit 1
}

# A symlink already at the destination is refused outright, whatever
# SURMISE_FORCE says.
#
# The check below used to be `[ -f ... ]`, which follows a link, so what it asked
# was "is there a file there" and not "is a link there". Three things followed,
# each verified against the old script:
#
#   - a *dangling* link is not a file, so the guard passed with no
#     SURMISE_FORCE needed and the install replaced the link the player had put
#     there;
#   - a link to a *directory* is a directory to `mv file dest`, so the binary
#     landed inside the linked directory and the link stayed — an install that
#     reported success and put nothing where the user would run it from;
#   - with SURMISE_FORCE, `mv` and `chmod` both dereferenced, so a link to a real
#     file had that file replaced and chmodded rather than the link.
#
# Whether something is a link is a separate question from what it points at, and
# that is the question this asks.
[ -L "$DEST/surmise" ] && {
    echo "install.sh: $DEST/surmise is a symlink; refusing to write through it" >&2
    exit 1
}
if [ -e "$DEST/surmise" ] && [ "${SURMISE_FORCE:-0}" != 1 ]; then
    echo "install.sh: $DEST/surmise already exists; set SURMISE_FORCE=1 to replace it" >&2
    exit 1
fi

mkdir -p "$DEST"
# Write beside the destination and rename over it, rather than moving onto it.
# `mv file dest` treats a directory at dest as somewhere to move the file *into*,
# so the binary can end up one level down from where the user will run it; a
# rename is a single step that replaces whatever is there and is not a move into
# a directory at all.
#
# install(1) rather than cp + chmod, where it exists, because it sets the mode
# outright instead of adding to whatever the archive carried.
if command -v install >/dev/null 2>&1; then
    install -m 0755 "$tmp/surmise" "$DEST/surmise.new" || {
        rm -f "$DEST/surmise.new" 2>/dev/null || true
        echo "install.sh: could not write $DEST/surmise" >&2
        exit 1
    }
else
    cp "$tmp/surmise" "$DEST/surmise.new" || {
        rm -f "$DEST/surmise.new" 2>/dev/null || true
        echo "install.sh: could not write $DEST/surmise" >&2
        exit 1
    }
    chmod 0755 "$DEST/surmise.new"
fi
mv -f "$DEST/surmise.new" "$DEST/surmise" || {
    rm -f "$DEST/surmise.new" 2>/dev/null || true
    echo "install.sh: could not put the binary in place" >&2
    exit 1
}

echo "installed $DEST/surmise"
case ":$PATH:" in
*":$DEST:"*) ;;
*)
    echo "note: $DEST is not on your PATH. add this to your shell profile:"
    echo "  export PATH=\"\$PATH:$DEST\""
;;
esac
