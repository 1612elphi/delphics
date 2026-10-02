#!/usr/bin/env bash
# Build the delphics-bar, delphics-modd and delphics .debs from this repo on Debian 13. Usage: build.sh [outdir]
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT="$(realpath "${1:-.}")"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
VERSION="0.0.$(date -u +%Y%m%d%H%M)"

sudo -E apt-get install -y --no-install-recommends \
    dpkg-dev pkg-config gcc libgtk-4-dev libgtk4-layer-shell-dev libgirepository1.0-dev
sudo -E apt-get install -y --no-install-recommends -t trixie-backports golang-go

cd "$REPO"
go build -trimpath -ldflags=-s -o "$WORK/delphics-bar" ./cmd/delphics-bar
CGO_ENABLED=0 go build -trimpath -ldflags=-s -o "$WORK/delphics-modd" ./cmd/delphics-modd
CGO_ENABLED=0 go build -trimpath -ldflags=-s -o "$WORK/delphics" ./cmd/delphics

# dpkg-shlibdeps only runs inside a source tree with debian/control
mkdir -p "$WORK/debian"
printf 'Source: delphics\n\nPackage: delphics\nArchitecture: any\n' > "$WORK/debian/control"
shlibs() { (cd "$WORK" && dpkg-shlibdeps -O "$1" | sed -n 's/^shlibs:Depends=//p'); }

package() { # name depends pkgdir
    mkdir -p "$3/DEBIAN"
    cat > "$3/DEBIAN/control" <<EOF
Package: $1
Version: $VERSION
Architecture: amd64
Maintainer: Ruby <rmv@rmv.fyi>
${2:+Depends: $2
}Section: x11
Priority: optional
Description: $1
EOF
    chmod 755 "$3"/DEBIAN/post* "$3"/DEBIAN/pre* 2>/dev/null || true
    dpkg-deb --build --root-owner-group "$3" "$OUT/${1}_${VERSION}_amd64.deb"
}

BAR="$WORK/pkg-bar"
install -Dm755 "$WORK/delphics-bar" "$BAR/usr/bin/delphics-bar"
install -Dm644 "$REPO/cmd/delphics-bar/delphics-bar.service" "$BAR/usr/lib/systemd/user/delphics-bar.service"
mkdir -p "$BAR/DEBIAN"
printf '#!/bin/sh\nset -e\nsystemctl --global enable delphics-bar.service\n' > "$BAR/DEBIAN/postinst"
printf '#!/bin/sh\nset -e\n[ "$1" = remove ] && systemctl --global disable delphics-bar.service || true\n' > "$BAR/DEBIAN/prerm"
package delphics-bar "$(shlibs delphics-bar), upower, network-manager, niri, libglib2.0-bin" "$BAR"

MODD="$WORK/pkg-modd"
install -Dm755 "$WORK/delphics-modd" "$MODD/usr/bin/delphics-modd"
install -Dm644 "$REPO/cmd/delphics-modd/delphics-modd.service" "$MODD/usr/lib/systemd/system/delphics-modd.service"
mkdir -p "$MODD/DEBIAN"
printf '#!/bin/sh\nset -e\nsystemctl daemon-reload\nsystemctl enable delphics-modd.service\nsystemctl restart delphics-modd.service\n' > "$MODD/DEBIAN/postinst"
printf '#!/bin/sh\nset -e\n[ "$1" = remove ] && systemctl disable --now delphics-modd.service || true\n' > "$MODD/DEBIAN/prerm"
package delphics-modd "" "$MODD"

CLI="$WORK/pkg-cli"
install -Dm755 "$WORK/delphics" "$CLI/usr/bin/delphics"
package delphics "" "$CLI"
