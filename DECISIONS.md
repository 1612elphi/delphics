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
| delphics-bar, delphics-modd | this repo (`packages/delphics/build.sh`) | gotk4 pinned to v0.3.1: v0.4.x calls GLib > 2.84 (trixie); first gotk4 compile ~13 min on the X280 |

`bootstrap.sh` (phase 1) installs from a local dir of these .debs plus trixie, trixie-backports, and upstream repos (WezTerm, Tailscale, gh, NodeSource 24), then Helium, fonts, DELPHICS files, Flathub apps, greetd, and moves ifupdown Wi-Fi to NetworkManager.

## Own components (Go)

| Package | Purpose |
|---|---|
| `delphics-bar` | GTK4 layer-shell top bar. Left: active app + minimap of the niri strip. Right: plugin items. Also the notification daemon. |
| `delphics-modd` | system service; reads evdev keyboards, publishes modifier state only (super/shift/ctrl/alt), never other keys |
| `delphics` | CLI: `theme`, `update`, config helpers |
| `delphics-desktop` | meta-package depending on everything above |

### Bar

- Data: niri IPC event stream (JSON over `$NIRI_SOCKET`) for windows, columns, focus.
- Minimap: one tile per window (stacked windows included), the screen as a frame, off-screen tiles dimmed; true-to-screen scale with fixed 3 px gaps, panned past 240 px. niri's IPC has no scroll position for tiled windows (`tile_pos_in_workspace_view` is floating-only in 26.04), so the bar re-runs niri's `compute_new_view_offset` for `center-focused-column "never"` on every focus change. Actions that scroll without moving focus (`center-column`, `center-visible-columns`, touchpad scrolling) put the frame off until the next scroll the emulation sees; changing `center-focused-column` needs a matching change in `internal/niri`.
- Plugin API: D-Bus, bus name prefix `tools.delphi.Delphics`. Plugins register items and push updates; click callbacks supported.
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
