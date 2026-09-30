#!/usr/bin/env bash
# Turn a minimal Debian 13 install into DELPHICS.
# Usage: sudo ./bootstrap.sh <dir with DELPHICS .debs>
# ponytail: .debs come from a local dir until the R2 apt repo exists
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

DEBS="$(realpath "${1:?usage: bootstrap.sh <dir with DELPHICS .debs>}")"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
USER_NAME="${SUDO_USER:?run with sudo from the target user}"
USER_HOME="$(getent passwd "$USER_NAME" | cut -d: -f6)"

log() { printf '\033[1m==> %s\033[0m\n' "$*"; }

[ "$(. /etc/os-release && echo "$VERSION_CODENAME")" = trixie ] || { echo "needs Debian 13 (trixie)" >&2; exit 1; }

log "apt sources"
cat > /etc/apt/sources.list.d/trixie-backports.sources <<'EOF'
Types: deb
URIs: http://deb.debian.org/debian
Suites: trixie-backports
Components: main non-free-firmware
Signed-By: /usr/share/keyrings/debian-archive-keyring.gpg
EOF

# xdg-desktop-portal-gnome recommends gnome-shell, which recommends gdm3; -1 keeps both out
cat > /etc/apt/preferences.d/delphics-no-gnome-shell <<'EOF'
Package: gnome-shell gdm3
Pin: release *
Pin-Priority: -1
EOF

add_repo() { # name key-url repo-line
    curl -fsSL "$2" | gpg --dearmor --yes -o "/usr/share/keyrings/$1.gpg"
    echo "deb [signed-by=/usr/share/keyrings/$1.gpg] $3" > "/etc/apt/sources.list.d/$1.list"
}
apt-get install -y --no-install-recommends curl gpg ca-certificates jq unzip
add_repo wezterm    https://apt.fury.io/wez/gpg.key                               "https://apt.fury.io/wez/ * *"
add_repo tailscale  https://pkgs.tailscale.com/stable/debian/trixie.noarmor.gpg   "https://pkgs.tailscale.com/stable/debian trixie main"
add_repo githubcli  https://cli.github.com/packages/githubcli-archive-keyring.gpg "https://cli.github.com/packages stable main"
add_repo nodesource https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key     "https://deb.nodesource.com/node_24.x nodistro main"
dpkg --add-architecture i386
apt-get update

log "packages"
apt-get install -y \
    greetd tuigreet gtklock swayidle swaybg swappy cliphist wl-clipboard \
    brightnessctl playerctl pipewire-audio wireplumber \
    xdg-desktop-portal-gnome xdg-desktop-portal-gtk gnome-keyring \
    xwayland libgl1-mesa-dri libegl-mesa0 mesa-vulkan-drivers intel-media-va-driver \
    network-manager power-profiles-daemon \
    fonts-open-sans fonts-noto-color-emoji orca \
    nautilus flatpak wezterm-nightly tailscale gh nodejs \
    wine wine64 wine32:i386 \
    fish starship \
    fzf bat eza ripgrep fd-find btop fastfetch zoxide tealdeer du-dust duf procs \
    glow jq xh broot tokei hyperfine git lazygit nano w3m wget lsof \
    gcc make rustup
apt-get install -y -t trixie-backports golang-go
apt-get install -y "$DEBS"/*.deb

log "helium"
HELIUM_URL="$(curl -fsSL https://api.github.com/repos/imputnet/helium-linux/releases/latest \
    | jq -r '.assets[] | select(.name | test("amd64\\.deb$")) | .browser_download_url')"
curl -fsSL -o /tmp/helium.deb "$HELIUM_URL"
apt-get install -y /tmp/helium.deb
rm /tmp/helium.deb

log "fonts"
FONT_DIR=/usr/local/share/fonts/delphics
mkdir -p "$FONT_DIR"
IOSEVKA_TAG="$(curl -fsSL https://api.github.com/repos/be5invis/Iosevka/releases/latest | jq -r .tag_name)"
curl -fsSL -o /tmp/iosevka.zip \
    "https://github.com/be5invis/Iosevka/releases/download/${IOSEVKA_TAG}/PkgTTC-Iosevka-${IOSEVKA_TAG#v}.zip"
unzip -o -q /tmp/iosevka.zip -d "$FONT_DIR"
rm /tmp/iosevka.zip
# the variable font has the wdth axis (75-100); Debian's fonts-open-sans is the static 2011 set
curl -fsSL -o "$FONT_DIR/OpenSans[wdth,wght].ttf" \
    "https://raw.githubusercontent.com/google/fonts/main/ofl/opensans/OpenSans%5Bwdth%2Cwght%5D.ttf"
fc-cache -f >/dev/null

log "delphics files"
install -Dm644 "$HERE/delphics-desktop/niri/config.kdl" /usr/share/delphics/niri/config.kdl
install -Dm644 "$HERE/delphics-cli/fish/delphics.fish" /usr/share/fish/vendor_conf.d/delphics.fish
mkdir -p /usr/lib/delphics/bin
ln -sf /usr/bin/batcat /usr/lib/delphics/bin/bat
ln -sf /usr/bin/fdfind /usr/lib/delphics/bin/fd

USER_NIRI="$USER_HOME/.config/niri/config.kdl"
if [ ! -e "$USER_NIRI" ]; then
    install -d -o "$USER_NAME" -g "$USER_NAME" "$USER_HOME/.config" "$USER_HOME/.config/niri"
    echo 'include "/usr/share/delphics/niri/config.kdl"' > "$USER_NIRI"
    chown "$USER_NAME:" "$USER_NIRI"
fi
install -d -o "$USER_NAME" -g "$USER_NAME" "$USER_HOME/Pictures" "$USER_HOME/Pictures/Screenshots"
chsh -s /usr/bin/fish "$USER_NAME"

log "flatpak"
flatpak remote-add --if-not-exists flathub https://dl.flathub.org/repo/flathub.flatpakrepo
flatpak install -y --noninteractive flathub \
    io.github.kolunmi.Bazaar org.gnome.Calculator org.gnome.TextEditor org.gnome.Firmware \
    io.mpv.Mpv io.gitlab.news_flash.NewsFlash io.github.fastrizwaan.WineCharm \
    io.github.flattool.Ignition de.haeckerfelix.Fragments net.codelogistics.webapps \
    org.gnome.gitlab.cheywood.Buffer com.toolstack.Folio dev.zed.Zed

log "login"
cat > /etc/greetd/config.toml <<'EOF'
[terminal]
vt = 7

[default_session]
command = "tuigreet --time --remember --asterisks --cmd niri-session"
user = "_greetd"
EOF
systemctl enable greetd power-profiles-daemon

log "network"
# installing systemd-resolved points /etc/resolv.conf at its stub, and ifupdown's DHCP does not
# feed resolved; DNS works again once NetworkManager owns the interfaces (below)
apt-get install -y systemd-resolved
systemctl reload dbus

# NetworkManager ignores interfaces listed in /etc/network/interfaces; hand all of them over.
# Ethernet gets an automatic NM profile; Wi-Fi needs one built from the wpa-* lines.
migrate_ifupdown() {
    local ifaces iface ssid psk
    ifaces="$(awk '$1=="iface" && $2!="lo" {print $2}' /etc/network/interfaces)"
    [ -n "$ifaces" ] || return 0
    cp /etc/network/interfaces /etc/network/interfaces.pre-delphics
    for iface in $ifaces; do
        ssid="$(awk -v i="$iface" '$1=="iface"{on=($2==i)} on && $1=="wpa-ssid"{sub(/^[ \t]*wpa-ssid[ \t]+/,""); print; exit}' /etc/network/interfaces)"
        psk="$(awk -v i="$iface" '$1=="iface"{on=($2==i)} on && $1=="wpa-psk"{sub(/^[ \t]*wpa-psk[ \t]+/,""); print; exit}' /etc/network/interfaces)"
        [ -n "$ssid" ] && nmcli connection add type wifi ifname "$iface" con-name "$ssid" ssid "$ssid" \
            ${psk:+wifi-sec.key-mgmt wpa-psk wifi-sec.psk "$psk"} >/dev/null
    done
    awk '($1=="allow-hotplug" || $1=="auto") && $2!="lo" {next}
         $1=="iface" {skip=($2!="lo")}
         !skip' /etc/network/interfaces.pre-delphics > /etc/network/interfaces
    # detached: this drops the current connection when run over SSH.
    # ifdown reads the backup, because the rewritten file no longer knows how these interfaces came up
    systemd-run --on-active=3 --unit=delphics-nm-switch \
        sh -c "for i in $ifaces; do ifdown -i /etc/network/interfaces.pre-delphics --force \$i; done; systemctl restart NetworkManager"
    log "$(echo $ifaces) move to NetworkManager in 3s; SSH sessions over them will drop"
}
migrate_ifupdown

log "done; reboot to reach the tuigreet login"
