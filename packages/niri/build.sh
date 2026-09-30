#!/usr/bin/env bash
# Build a niri .deb on Debian 13. Usage: build.sh [tag] [outdir]
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

TAG="${1:-v26.04}"
OUT="$(realpath "${2:-.}")"
WORK="$(mktemp -d)"
# cargo output outside WORK so a failed packaging step does not cost a rebuild
export CARGO_TARGET_DIR="$HOME/.cache/delphics/niri-target"
trap 'rm -rf "$WORK"' EXIT

sudo -E apt-get install -y --no-install-recommends \
    git curl ca-certificates gcc clang pkg-config dpkg-dev \
    libudev-dev libgbm-dev libxkbcommon-dev libegl1-mesa-dev libwayland-dev \
    libinput-dev libdbus-1-dev libsystemd-dev libseat-dev libpipewire-0.3-dev \
    libpango1.0-dev libdisplay-info-dev rustup

# niri needs rustc >= 1.87; trixie's rustc is 1.85, so use rustup's stable
rustup set profile minimal
rustup default stable

git clone --depth 1 --branch "$TAG" https://github.com/niri-wm/niri.git "$WORK/src"
cd "$WORK/src"
cargo build --release --locked

VERSION="${TAG#v}"
PKG="$WORK/pkg"
install -s -Dm755 "$CARGO_TARGET_DIR/release/niri"             "$PKG/usr/bin/niri"
install -Dm755 resources/niri-session          "$PKG/usr/bin/niri-session"
install -Dm644 resources/niri.desktop          "$PKG/usr/share/wayland-sessions/niri.desktop"
install -Dm644 resources/niri-portals.conf     "$PKG/usr/share/xdg-desktop-portal/niri-portals.conf"
install -Dm644 resources/niri.service          "$PKG/usr/lib/systemd/user/niri.service"
install -Dm644 resources/niri-shutdown.target  "$PKG/usr/lib/systemd/user/niri-shutdown.target"

# dpkg-shlibdeps only runs inside a source tree with debian/control
mkdir -p debian && printf 'Source: niri\n\nPackage: niri\nArchitecture: any\n' > debian/control
SHLIBS="$(dpkg-shlibdeps -O "$CARGO_TARGET_DIR/release/niri" | sed -n 's/^shlibs:Depends=//p')"

mkdir -p "$PKG/DEBIAN"
cat > "$PKG/DEBIAN/control" <<EOF
Package: niri
Version: ${VERSION}-1delphics1
Architecture: amd64
Maintainer: Ruby <rmv@rmv.fyi>
Depends: ${SHLIBS}, libwayland-server0
Recommends: xwayland-satellite, xdg-desktop-portal-gnome, xdg-desktop-portal-gtk, gnome-keyring
Section: x11
Priority: optional
Homepage: https://github.com/niri-wm/niri
Description: scrollable-tiling Wayland compositor
EOF

dpkg-deb --build --root-owner-group "$PKG" "$OUT/niri_${VERSION}-1delphics1_amd64.deb"
