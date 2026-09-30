#!/usr/bin/env bash
# Backport rofi from Debian sid to Debian 13 (trixie's 1.7.5 has no native Wayland). Usage: build.sh [sid-version] [outdir]
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

SRC_VERSION="${1:-2.0.0-0.2}"
OUT="$(realpath "${2:-.}")"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

UPSTREAM="${SRC_VERSION%-*}"
POOL=https://deb.debian.org/debian/pool/main/r/rofi
cd "$WORK"
for f in "rofi_${SRC_VERSION}.dsc" "rofi_${UPSTREAM}.orig.tar.xz" "rofi_${SRC_VERSION}.debian.tar.xz"; do
    curl -fsSLO "$POOL/$f"
done
dpkg-source -x "rofi_${SRC_VERSION}.dsc" src
cd src

# ~ sorts below the sid version, so a later upgrade to Debian's own package wins
BPO_VERSION="${SRC_VERSION}~delphics13+1"
{
    printf 'rofi (%s) trixie; urgency=medium\n\n  * Rebuild for Debian 13.\n\n' "$BPO_VERSION"
    printf ' -- Ruby <rmv@rmv.fyi>  %s\n\n' "$(date -R)"
    cat debian/changelog
} > debian/changelog.new
mv debian/changelog.new debian/changelog

sudo -E apt-get install -y --no-install-recommends build-essential dpkg-dev
sudo -E apt-get build-dep -y ./
dpkg-buildpackage -b -us -uc
cp ../rofi_"${BPO_VERSION}"_amd64.deb "$OUT/"
