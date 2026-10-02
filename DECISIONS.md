# DELPHICS decisions

Debian-based Extremely Lightweight Pretty and Harmoniously Integrated Computing System.

## Base

| Area | Decision |
|---|---|
| Base | Debian 13 (trixie) stable + trixie-backports |
| Architecture | amd64 only |
| Delivery, phase 1 | minimal Debian netinst, then a bootstrap script that adds the DELPHICS apt repo and installs `delphics-desktop` |
| Delivery, phase 2 | ISO built from the same packages |
| Missing or old packages | built in GitHub Actions, published as a signed apt repo on Cloudflare R2 |
| Domain | `delphics.delphi.tools` (apt repo URL, D-Bus names) |
| Network | NetworkManager |
| Power | power-profiles-daemon |
| DNS | systemd-resolved (DNS cache; Tailscale MagicDNS) |
| Config ownership | defaults in `/usr/share/delphics`, updated by the package; `~/.config` holds user overrides only. niri >= 25.11 supports `include`, so `~/.config/niri/config.kdl` includes the system default, then user settings |
| Language for own code | Go |

## Session

| Component | Choice | Source |
|---|---|---|
| Compositor | niri (scrolling tiling), one workspace in the UX | DELPHICS repo |
| X11 apps | xwayland-satellite | DELPHICS repo |
| Login | greetd + tuigreet | trixie |
| Icons | Adwaita symbolic (`adwaita-icon-theme` 48) in the bar | trixie |
| Lock / idle | gtklock + swayidle | trixie |
| Launcher | rofi 2.0 (native Wayland) | DELPHICS repo (backport from sid) |
| Notifications | implemented in `delphics-bar` (org.freedesktop.Notifications) | own |
| Portals | xdg-desktop-portal-gtk, xdg-desktop-portal-gnome (screencast) | trixie |
| Screenshots | niri built-in UI, swappy for annotation | trixie (swappy) |
| Clipboard history | cliphist, picked through rofi | trixie |
| Wallpaper | swaybg, set by theme pack | trixie |

## Apps

| Role | Choice | Source |
|---|---|---|
| Terminal | WezTerm nightly (`wezterm-nightly`); the 20240203 release opens no window under niri 26.04 | upstream apt repo (apt.fury.io/wez) |
| Multiplexer | none; the niri strip replaces it | |
| Shell | fish | trixie (4.0.2) |
| Prompt | starship | trixie (1.22) |
| `$EDITOR` | fresh | DELPHICS repo |
| Browser | Helium | upstream `helium-bin` .deb, mirrored |
| UI font | Open Sans Condensed (Open Sans variable, `wdth` axis) | `delphics-fonts` |
| Mono font | Iosevka | `delphics-fonts` (sid has 34.4, trixie has none) |
| Code editor | Zed | Flathub `dev.zed.Zed` |
| File manager | Nautilus | trixie (48.3) |
| VPN | Tailscale | upstream apt repo (pkgs.tailscale.com) |
| Wine | Wine 10.0 + i386 multiarch; WineHQ repo if Affinity needs newer | trixie |

### GUI apps (Flatpak)

Flatpak (trixie 1.16) + Flathub are in the base install. Every GUI app below comes from Flathub, updated through Bazaar.

| App | Flathub ID |
|---|---|
| Bazaar | io.github.kolunmi.Bazaar |
| GNOME Calculator | org.gnome.Calculator |
| GNOME Text Editor | org.gnome.TextEditor |
| GNOME Firmware | org.gnome.Firmware |
| mpv | io.mpv.Mpv |
| Newsflash | io.gitlab.news_flash.NewsFlash |
| WineCharm | io.github.fastrizwaan.WineCharm |
| Ignition | io.github.flattool.Ignition |
| Fragments | de.haeckerfelix.Fragments |
| Web Apps | net.codelogistics.webapps |
| Buffer | org.gnome.gitlab.cheywood.Buffer |
| Folio | com.toolstack.Folio |
| Zed | dev.zed.Zed |

delphitools-cli and delphitools-lnx: planned, packaging undecided.

### CLI apps

| Source | Tools |
|---|---|
| trixie | fzf, bat, eza, ripgrep, fd-find, btop, fastfetch, zoxide, tealdeer, du-dust, duf, procs, glow, jq, xh, broot, tokei, hyperfine, git, lazygit, nano, w3m, curl, wget, gcc, make, rustup, lsof |
| trixie-backports | golang-go (1.26) |
| DELPHICS repo | doggo, navi, superfile, gopass, bun, yq (mikefarah, Go), fresh |
| upstream apt repos | gh (cli.github.com), nodejs 24 LTS (NodeSource) |

`delphics-cli` ships `/usr/lib/delphics/bin/{bat,fd}` symlinks to `batcat`/`fdfind` and `delphics-cli/fish/delphics.fish` as `/usr/share/fish/vendor_conf.d/delphics.fish`: `$EDITOR`, prompt, zoxide, aliases for command replacements and utilities, abbreviations for shortcuts.

## Package builds

`packages/<name>/build.sh` builds a .deb on a trixie host; the same scripts are meant to run in CI.

| Package | Source | Notes |
|---|---|---|
| niri | github.com/niri-wm/niri tag | rustc >= 1.87 via Debian `rustup`; ~6.5 min on the X280; 6.2 MB stripped |
| xwayland-satellite | github.com/Supreeeme/xwayland-satellite tag | `-F systemd` |
| rofi | sid source package, rebuilt | version `<sid>~delphics13+1` |
| delphics-bar, delphics-modd, delphics | this repo (`packages/delphics/build.sh`) | gotk4 pinned to v0.3.1: v0.4.x calls GLib > 2.84 (trixie); first gotk4 compile ~13 min on the X280 |

`bootstrap.sh` (phase 1) installs from a local dir of these .debs plus trixie, trixie-backports, and upstream repos (WezTerm, Tailscale, gh, NodeSource 24), then Helium, fonts, DELPHICS files, Flathub apps, greetd, and moves ifupdown Wi-Fi to NetworkManager.

## Own components (Go)

| Package | Purpose |
|---|---|
| `delphics-bar` | GTK4 layer-shell top bar. Left: active app + minimap of the niri strip. Right: plugin items. Also the notification daemon. |
| `delphics-modd` | system service; reads evdev keyboards, publishes modifier state only (super/shift/ctrl/alt), never other keys |
| `delphics` | CLI: `theme`, `update`, config helpers |
| `delphics-desktop` | meta-package depending on everything above |

### Bar

- System menu: clicking the app name (or "Desktop" with nothing focused) opens Close window, Lock (`gtklock -d`), Suspend, and Log out… / Restart… / Power off…, each of the last three behind a one-entry confirm submenu.
- HUD: a level change (volume, brightness) crossfades the middle of the bar, where notifications show, to icon + meter + text for 1.5 s. The volume plugin flashes it on every change of level or mute it sees (keys, other apps), except right after its own menu slider moved; the brightness keys run `delphics brightness up|down`, which sets the backlight through logind and flashes it, since the plugin only polls brightness once a second.
- Data: niri IPC event stream (JSON over `$NIRI_SOCKET`) for windows, columns, focus.
- Minimap: one tile per window (stacked windows included), the screen as a frame, off-screen tiles dimmed; true-to-screen scale with fixed 3 px gaps, panned past 240 px. niri's IPC has no scroll position for tiled windows (`tile_pos_in_workspace_view` is floating-only in 26.04), so the bar re-runs niri's `compute_new_view_offset` for `center-focused-column "never"` on every focus change. Actions that scroll without moving focus (`center-column`, `center-visible-columns`, touchpad scrolling) put the frame off until the next scroll the emulation sees; changing `center-focused-column` needs a matching change in `internal/niri`.
- Plugin API (`internal/baritems`): bus name `tools.delphi.Delphics.BarItems` (GTK holds `tools.delphi.Delphics.Bar`), object `/tools/delphi/Delphics/BarItems`, interface `tools.delphi.Delphics.BarItems1`.
  - `Set(s id, a{sv} props)` creates or updates an item; props `text` (s, ≤ 200 chars), `icon` (s, an icon theme name, no paths), `tooltip` (s), `order` (i, lower is further left, ties by id), `urgent` (b, amber bold), `bold` (b, bright bold), `menu` (aa{sv}, entries with `id` s, `label` s, `enabled` b, `checked` b, `section` b, `slider` d: a 0–1 slider). Props left out keep their value; unknown or mistyped props are errors. An item with neither text nor icon is hidden.
  - `ShowLevel(s icon, d value, s text)`: flashes a level in the bar's HUD (any caller, not tied to an item); value 0–1, negative for no meter.
  - `Remove(s id)`; signals `Clicked(s id, u button)`, `Activated(s id, s entry)`, `Changed(s id, s entry, d value)` (slider moved) and `Scrolled(s id, d delta)` (positive downwards, 1 per wheel notch).
  - Menus: a left click on an item with a menu opens it as a GTK popover menu drawn by the bar (palette colors, square corners, condensed font); plugins stay GTK-free. Entries without `id` are greyed status lines; `checked` shows a check; `section` starts a group. An open menu follows updates live; when only slider values change, the sliders move in place so a drag is not interrupted. Every click is still sent as `Clicked`, so a plugin can refresh before or while its menu shows. Clicking the item of an open menu closes it; clicking elsewhere on the bar closes it too (the popover's GTK grab routes bar clicks to the popover, which closes on presses outside itself; niri dismisses it for clicks on other clients).
  - An item belongs to the connection that created it: only it can change or remove it, and the item goes when that connection closes. At most 16 items per connection.
  - Icons are drawn at 16 px in the item's text color; Adwaita's success/warning/error colors (e.g. the green charge fill) are remapped to the palette's primary.
  - Network, battery and clock are plugins too: `delphics plugin brightness|volume|bluetooth|wwan|network|battery|clock`, one `delphics-plugin@NAME.service` user unit each, enabled globally by the `delphics` package (disable one with `systemctl --user disable --now delphics-plugin@battery`). Orders (brightness 80, volume 85, bluetooth 90, wwan 95, network 100, battery 110, clock 120) keep them right of third-party items (default order 0). Network and battery follow `PropertiesChanged` signals from NetworkManager, UPower and power-profiles-daemon (debounced 250 ms, re-read when a service restarts); the clock wakes on the minute and shows the date as tooltip.
    - Network menu: status, Wi-Fi switch, up to 8 visible networks (one per SSID, strongest first; click connects a saved or open network, a secured unknown one opens `nmtui connect SSID` in WezTerm), Disconnect, "Network settings…" (`nmtui`). Opening the menu requests a Wi-Fi scan.
    - Battery menu: charge and time left/to full, power profiles (power-saver/balanced/performance).
    - Volume (`wpctl`, changes from `pw-dump --monitor`): icon only; menu with output and microphone sliders, mute toggles, and device choice when there are several. Scroll: ±5 %; middle click: mute. Moving the slider of a muted device unmutes it.
    - Brightness (sysfs read, logind `Session.SetBrightness`): slider, scroll ±5 %, never below 1 %. Polled every second, since the kernel only announces firmware-driven backlight changes, not brightnessctl or logind writes.
    - Bluetooth (BlueZ): on/off, paired devices (click connects/disconnects), "Bluetooth settings…" (`bluetoothctl` in WezTerm); the bar shows the connected device's name. Hidden without an adapter. Pairing is not in the menu yet.
    - WWAN (ModemManager + NetworkManager): signal icon, plus the access technology while data is up; menu with operator · technology · signal, "Mobile data" (activates the saved gsm connection or creates one with `gsm.auto-config`, autoconnect off), "Modem" (ModemManager Enable), "Mobile settings…" (`nmtui`). Hidden without a modem.
    - Failed actions show as a notification.
  - Plugins use `baritems.Client`, which re-sends the item when the bar (re)starts, so start order does not matter.
  - Shell plugins: `delphics bar item [--icon NAME] [--tooltip T] [--order N] [--bold] [--on-click CMD] [--urgent-prefix P] ID` shows each stdin line as the item's text, runs CMD on click with `$DELPHICS_BUTTON`, keeps the item after stdin ends until killed, and re-sends it when the bar restarts.
- Notifications: the bar owns `org.freedesktop.Notifications` (`internal/notify`). The newest one shows in one line in the middle of the bar, with a `+N` count of the others; clicking runs its `default` action and dismisses it. Default timeout 5 s; critical urgency stays until dismissed. No popups, no history.
- Do-not-disturb: GApplication action `dnd` (`gapplication action tools.delphi.Delphics.Bar dnd`, Super+Ctrl+N). It hides all but critical notifications, drops the ones on screen, and shows `dnd` on the right.
- GTK bindings: `github.com/diamondburned/gotk4` v0.3.1 (newer versions need a newer GLib than trixie's 2.84). `gotk4-layer-shell` is GTK3-only and unmaintained since 2024-01, so the bar carries a small cgo binding to `libgtk4-layer-shell` (trixie 1.0.4).

### Key hints

- Holding super past 350 ms replaces the bar contents with key hints; releasing restores the bar.
- Transport: `delphics-modd` (system service, DynamicUser in group `input`) sends a 1-byte modifier mask on `/run/delphics-modd/modd.sock`.
- Hint set = held modifier combination + focused app-id.
  - `Super`: window/strip binds
  - `Super+Shift`: move/resize binds
  - `Super+Ctrl`: system binds
  - `Super+Alt`: monitor binds
  - apps register extra hints per app-id
- WM hints are generated from the niri config, not maintained by hand.
- Keybindings: `KEYBINDS.md`.

## Theming

- Switchable theme packs, `delphics theme set <name>`.
- First pack: DELPHICS, palette taken from delphitools web (`themes/delphics/palette.toml`). Light: cream + forest green. Dark: dark green + amber. Square corners (radius 0) everywhere DELPHICS draws: bar, rofi, niri borders, gtklock.
- Light/dark follows sunrise/sunset: darkman (Go; not in Debian, DELPHICS repo) sets the portal `color-scheme` and runs hooks that switch bar, WezTerm, rofi, niri, gtklock, starship, and the libadwaita accent. Location from geoclue (trixie 2.7) or fixed coordinates.
- ANSI 16 colors: derived in OKLCH (hues 25/145/90/255/335/195; light L 0.50 normal / 0.42 bright, dark L 0.72 / 0.82), in `palette.toml` under `[light.ansi]`/`[dark.ansi]`.
- libadwaita accent: `green` in light, `yellow` in dark (GNOME's accent is a fixed enum, so the nearest value).
- A pack is a palette plus templates for: bar, rofi, WezTerm, niri borders, gtklock, starship, fish, fresh.
- libadwaita apps get only the pack's accent color and light/dark scheme (gsettings `accent-color`, `color-scheme`, exposed to Flatpaks via the settings portal). No `gtk.css` overrides.

## Open questions

- `/usr/lib/delphics/bin` is on PATH for fish only; bash scripts calling `bat`/`fd` need `/etc/profile.d` too.
- Zed Flatpak: host LSP/terminal access setup (`flatpak-spawn --host`), unverified.
- Affinity under Wine: untested.
- Apt repo URL layout under `delphics.delphi.tools`.
- delphitools-cli / delphitools-lnx packaging.
