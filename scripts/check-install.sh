#!/bin/sh
# Exercise install.sh's install step, end to end, against a local release.
#
# install.sh is the one script a player runs before they have the app, so the
# states it has to get right are filesystem states somebody else may have left
# behind. This drives the real script — a stub `curl` on PATH serves an archive
# and a checksums file from a scratch directory, and SURMISE_INSTALL_DIR points
# at a scratch directory per case — rather than a restatement of its logic.
#
# Run from anywhere: sh scripts/check-install.sh
set -u

cd "$(dirname "$0")/.." || exit 1

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

pass=0
fail=0
ok() {
    pass=$((pass + 1))
    echo "PASS  $1"
}
no() {
    fail=$((fail + 1))
    echo "FAIL  $1${2:+ — $2}"
}

# ---------------------------------------------------------------- a release --

serve=$work/serve
mkdir -p "$serve/pkg"
printf '#!/bin/sh\necho stub\n' >"$serve/pkg/surmise"
chmod 0755 "$serve/pkg/surmise"

# The platform this host is, so the served asset names are the ones the script
# builds out of uname. The names are the contract: the script asks for
# surmise_<version>_<os>_<arch>.tar.gz and looks that name up in checksums.txt.
os=$(uname -s)
case "$os" in Linux) os=linux ;; Darwin) os=darwin ;; esac
case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
*) arch=arm64 ;;
esac
asset="surmise_1.0.0_${os}_${arch}.tar.gz"

# publish DIR [MEMBERS...] — lay the archive out under its release name with a
# matching checksums file, as a release does.
publish() {
    dir=$1
    shift
    rm -f "$dir/$asset"
    tar -czf "$dir/$asset" -C "$dir/pkg" "$@"
    if command -v sha256sum >/dev/null 2>&1; then
        (cd "$dir" && sha256sum "$asset" >checksums.txt)
    else
        (cd "$dir" && shasum -a 256 "$asset" >checksums.txt)
    fi
}
publish "$serve" surmise

# A stub curl that ignores the URL and serves from $SERVE_DIR, laid out under the
# release's own names. It exits 22 on a miss, as curl does on a 404, so the
# script's own error handling is what is being exercised.
bin=$work/bin
mkdir -p "$bin"
cat >"$bin/curl" <<'STUB'
#!/bin/sh
out=""
url=""
while [ $# -gt 0 ]; do
    case "$1" in
        -o) out=$2; shift 2 ;;
        -*) shift ;;
        *) url=$1; shift ;;
    esac
done
name=${url##*/}
[ -f "$SERVE_DIR/$name" ] || exit 22
cp "$SERVE_DIR/$name" "$out"
STUB
chmod 0755 "$bin/curl"

# install NAME [SETUP...] — a fresh destination per case, SETUP run first. The
# setup is given the destination as $1 and is expected to leave a filesystem
# state behind for the script to meet.
install_into() {
    name=$1
    shift
    dest=$work/dest-$name
    mkdir -p "$dest"
    if [ $# -gt 0 ]; then
        "$@" "$dest"
    fi
    out=$(SURMISE_INSTALL_DIR="$dest" run_install 2>&1) && code=0 || code=$?
    dest=$dest
}

# The common invocation: the stub on PATH, this host's release served, one
# version pinned. Every case goes through here, so a case cannot pass or fail by
# resolving a different version than the one the fixture publishes.
run_install() {
    [ -n "${SURMISE_FORCE:-}" ] && export SURMISE_FORCE
    PATH="$bin:$PATH" SERVE_DIR="${SERVE_DIR:-$serve}" SURMISE_VERSION=v1.0.0 \
        sh install.sh
}

# ------------------------------------------------------------- the ordinary --

install_into clean
if [ "$code" -eq 0 ] && [ -x "$work/dest-clean/surmise" ] &&
    grep -q stub "$work/dest-clean/surmise"; then
    ok "installs into an empty directory"
else
    no "installs into an empty directory" "exit $code: $out"
fi

# The mode is set outright rather than added to, so a 0777 member does not
# survive into the destination.
mode=$(ls -l "$work/dest-clean/surmise" 2>/dev/null | cut -c1-10)
if [ "$mode" = "-rwxr-xr-x" ]; then
    ok "the installed binary is 0755"
else
    no "the installed binary is 0755" "mode is $mode"
fi

# And the temporary file it renames from is not left behind.
if [ ! -e "$work/dest-clean/surmise.new" ]; then
    ok "leaves no temporary file behind on success"
else
    no "leaves no temporary file behind on success" "surmise.new is still there"
fi

# --------------------------------------------------------- what is already there --

seed_file() {
    mkdir -p "$1"
    printf '#!/bin/sh\necho MINE\n' >"$1/surmise"
    chmod 0755 "$1/surmise"
}

install_into existing seed_file
if [ "$code" -eq 0 ]; then
    no "refuses to replace an existing binary without SURMISE_FORCE" "it installed anyway: $out"
elif grep -q MINE "$work/dest-existing/surmise" 2>/dev/null; then
    ok "refuses to replace an existing binary without SURMISE_FORCE"
else
    no "refuses to replace an existing binary without SURMISE_FORCE" "the old binary is gone: $out"
fi

SURMISE_FORCE=1 install_into force
if [ "$code" -eq 0 ] && grep -q stub "$work/dest-force/surmise" 2>/dev/null; then
    ok "SURMISE_FORCE replaces an existing binary"
else
    no "SURMISE_FORCE replaces an existing binary" "exit $code: $out"
fi
unset SURMISE_FORCE

# ------------------------------------------------------------------ symlinks --

# A link is refused outright, whatever SURMISE_FORCE says. Whether something is a
# link is a separate question from what it points at, and that is the question
# the -L test asks; `[ -f ]` follows, so a dangling link is not a file and the
# old guard walked straight past it.

seed_dangling() {
    mkdir -p "$1"
    ln -s "$1/../../escape-me" "$1/surmise"
}
install_into dangling seed_dangling
if [ "$code" -eq 0 ]; then
    no "a dangling symlink is refused" "it installed anyway: $out"
elif [ -L "$work/dest-dangling/surmise" ]; then
    ok "a dangling symlink is refused and left alone"
else
    no "a dangling symlink is refused and left alone" "the link was replaced: $out"
fi

# The one that writes somewhere else: `mv file dest` treats a directory at dest
# as somewhere to move the file *into*, so the binary lands inside the linked
# directory and the link stays — an install that reports success and puts nothing
# where the user will run it from.
seed_linkdir() {
    mkdir -p "$1/elsewhere"
    ln -s "$1/elsewhere" "$1/surmise"
}
install_into linkdir seed_linkdir
if [ "$code" -eq 0 ]; then
    no "a symlink to a directory is refused" "it installed anyway: $out"
elif [ -e "$work/dest-linkdir/elsewhere/surmise" ]; then
    no "a symlink to a directory is refused" "the binary landed inside the linked directory"
else
    ok "a symlink to a directory is refused, so nothing lands inside it"
fi

# Force means "replace what is there", not "dereference it and replace that".
seed_linkfile() {
    mkdir -p "$1/elsewhere"
    printf 'TARGET\n' >"$1/elsewhere/victim"
    ln -s "$1/elsewhere/victim" "$1/surmise"
}
SURMISE_FORCE=1 install_into linkfile seed_linkfile
unset SURMISE_FORCE
if [ "$code" -eq 0 ]; then
    no "SURMISE_FORCE does not write through a symlink" "it installed anyway: $out"
elif [ "$(cat "$work/dest-linkfile/elsewhere/victim" 2>/dev/null)" = "TARGET" ]; then
    ok "SURMISE_FORCE does not write through a symlink"
else
    no "SURMISE_FORCE does not write through a symlink" \
        "the link's target is now: $(cat "$work/dest-linkfile/elsewhere/victim" 2>/dev/null)"
fi

# --------------------------------------------------------------- the archive --

# What came out of the archive has to be a plain file. An archive member can be
# a symlink, and both mv and chmod dereference one.
serve2=$work/serve2
mkdir -p "$serve2/pkg"
printf 'elsewhere\n' >"$serve2/pkg/elsewhere"
ln -s elsewhere "$serve2/pkg/surmise"
publish "$serve2" surmise elsewhere

SERVE_DIR=$serve2 install_into membersym
if [ "$code" -eq 0 ]; then
    no "an archive member that is a symlink is refused" "it installed anyway: $out"
elif [ -e "$work/dest-membersym/surmise" ]; then
    no "an archive member that is a symlink is refused" "something was installed"
else
    ok "an archive member that is a symlink is refused"
fi

# ----------------------------------------------------------------- the digest --

# The checksum is mandatory, and the install stops before extracting.
serve3=$work/serve3
mkdir -p "$serve3"
cp "$serve/$asset" "$serve3/$asset"
printf 'deadbeef  %s\n' "$asset" >"$serve3/checksums.txt"
SERVE_DIR=$serve3 install_into badsum
if [ "$code" -eq 0 ]; then
    no "a checksum mismatch still stops the install" "it installed anyway: $out"
elif [ ! -e "$work/dest-badsum/surmise" ]; then
    ok "a checksum mismatch still stops the install"
else
    no "a checksum mismatch still stops the install" "something was installed"
fi

# A checksums file with no line for this asset is refused too, rather than
# matching nothing and continuing.
serve4=$work/serve4
mkdir -p "$serve4"
cp "$serve/$asset" "$serve4/$asset"
printf '%s  some_other_asset.tar.gz\n' "$(grep -oE '^[0-9a-f]{64}' "$serve/checksums.txt")" \
    >"$serve4/checksums.txt"
SERVE_DIR=$serve4 install_into noline
if [ "$code" -eq 0 ]; then
    no "a checksums file with no line for this asset is refused" "it installed anyway: $out"
else
    ok "a checksums file with no line for this asset is refused"
fi

echo
echo "$((pass + fail)) checks, $fail failed"
[ "$fail" -eq 0 ]
