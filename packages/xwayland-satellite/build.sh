#!/usr/bin/env bash
# Build an xwayland-satellite .deb on Debian 13. Usage: build.sh [tag] [outdir]
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

TAG="${1:-v0.8.3}"
OUT="$(realpath "${2:-.}")"
WORK="$(mktemp -d)"
# cargo output outside WORK so a failed packaging step does not cost a rebuild
export CARGO_TARGET_DIR="$HOME/.cache/delphics/xwayland-satellite-target"
trap 'rm -rf "$WORK"' EXIT

sudo -E apt-get install -y --no-install-recommends \
    git ca-certificates clang pkg-config dpkg-dev rustup \
    libxcb1-dev libxcb-cursor-dev
rustup set profile minimal
rustup default stable

git clone --depth 1 --branch "$TAG" https://github.com/Supreeeme/xwayland-satellite.git "$WORK/src"
cd "$WORK/src"
cargo build --release --locked -F systemd

VERSION="${TAG#v}"
PKG="$WORK/pkg"
install -s -Dm755 "$CARGO_TARGET_DIR/release/xwayland-satellite" "$PKG/usr/bin/xwayland-satellite"

# dpkg-shlibdeps only runs inside a source tree with debian/control
mkdir -p debian && printf 'Source: xwayland-satellite\n\nPackage: xwayland-satellite\nArchitecture: any\n' > debian/control
SHLIBS="$(dpkg-shlibdeps -O "$CARGO_TARGET_DIR/release/xwayland-satellite" | sed -n 's/^shlibs:Depends=//p')"

mkdir -p "$PKG/DEBIAN"
cat > "$PKG/DEBIAN/control" <<EOF
Package: xwayland-satellite
Version: ${VERSION}-1delphics1
Architecture: amd64
Maintainer: Ruby <rmv@rmv.fyi>
Depends: ${SHLIBS}, xwayland
Section: x11
Priority: optional
Homepage: https://github.com/Supreeeme/xwayland-satellite
Description: rootless Xwayland for Wayland compositors
EOF

dpkg-deb --build --root-owner-group "$PKG" "$OUT/xwayland-satellite_${VERSION}-1delphics1_amd64.deb"
