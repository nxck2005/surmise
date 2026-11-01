#!/usr/bin/env bash
# Bind the Go engine (mobile/core) into android/core/libs/core.aar.
#
#   ANDROID_HOME=~/Android/Sdk scripts/build-android-core.sh
#
# Gradle never runs Go, so run this before the first Android build and again
# after any change under mobile/ or internal/. The .aar is a build output and
# is never committed.
set -euo pipefail

cd "$(dirname "$0")/.."

sdk="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}"
if [ -z "$sdk" ]; then
	echo "build-android-core.sh: set ANDROID_HOME to the Android SDK" >&2
	exit 1
fi

# One pin for the NDK: the app's ndkVersion. r28 or newer aligns the .so for
# 16 KB pages.
ndk_version=$(sed -n 's/^ *ndkVersion = "\(.*\)"/\1/p' android/app/build.gradle.kts)
if [ -z "$ndk_version" ]; then
	echo "build-android-core.sh: no ndkVersion in android/app/build.gradle.kts" >&2
	exit 1
fi
export ANDROID_NDK_HOME="$sdk/ndk/$ndk_version"
if [ ! -d "$ANDROID_NDK_HOME" ]; then
	echo "build-android-core.sh: NDK $ndk_version is not installed; run:" >&2
	echo "  sdkmanager \"ndk;$ndk_version\"" >&2
	exit 1
fi
export ANDROID_HOME="$sdk"

# mobile/ is its own module, outside go.work: the root module's workspace
# would otherwise pull the patched bubbletea and the root's own versions in.
export GOWORK=off
cd mobile

# gomobile and gobind at the version mobile/go.mod pins, never whatever is on
# PATH: gomobile runs gobind by name, and the two must match.
bin=$(mktemp -d)
trap 'rm -rf "$bin"' EXIT
go build -o "$bin/" golang.org/x/mobile/cmd/gomobile golang.org/x/mobile/cmd/gobind
export PATH="$bin:$PATH"

out=../android/core/libs/core.aar
mkdir -p "$(dirname "$out")"
echo "binding mobile/core with NDK $ndk_version"
gomobile bind \
	-target=android/arm64,android/arm,android/amd64 \
	-androidapi 26 \
	-javapkg com.nxck.surmise.engine \
	-ldflags="-s -w" \
	-trimpath \
	-o "$out" \
	./core
# gomobile also writes a sources jar next to the .aar; nothing reads it.
rm -f ../android/core/libs/core-sources.jar

printf 'built android/core/libs/core.aar (%s)\n' "$(du -h "$out" | cut -f1)"
