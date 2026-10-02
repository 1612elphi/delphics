# DELPHICS keybindings (draft)

Base: niri `resources/default-config.kdl` (main, 2026-09-30). Status: accepted 2026-09-30.

Hint groups (what the bar shows while the modifiers are held):

| Held | Group |
|---|---|
| Super | navigate, layout, open/close |
| Super+Shift | move, resize, capture |
| Super+Ctrl | system |
| Super+Alt | monitors |

## Super

| # | Key | Action | niri default | Change |
|---|---|---|---|---|
| 1 | Super+←/→, Super+H/L | focus column left/right | same | keep |
| 2 | Super+↑/↓, Super+K/J | focus window up/down in column | same | keep |
| 3 | Super+Home/End | focus first/last column | same | keep |
| 4 | Super+1…9 | `focus-column N` | focus workspace N | no workspaces; numbers follow minimap order |
| 5 | Super+Tab | `focus-window-previous` | unbound | new |
| 6 | Super+Wheel←/→, Super+Shift+Wheel↑/↓ | focus column | same | keep; Super+Wheel↑/↓ (workspaces) removed |
| 7 | Super+O | overview | same | keep |
| 8 | Super+F | maximize column | same | keep |
| 9 | Super+M | maximize window to edges | same | keep |
| 10 | Super+E | expand column to available width | Super+Ctrl+F | moved out of system group |
| 11 | Super+C | center column | same | keep |
| 12 | Super+T | toggle tabbed column | Super+W | W now closes |
| 13 | Super+R | cycle preset column width | same | keep |
| 14 | Super+Minus/Equal | column width −/+10% | same | keep |
| 15 | Super+[ / ] | consume or expel window left/right | same | keep |
| 16 | Super+, / . | consume into / expel from column | same | keep |
| 17 | Super+Return | WezTerm | Super+T: alacritty | changed |
| 18 | Super+Space | rofi app launcher | Super+D: fuzzel | changed; Super+D unbound |
| 19 | Super+V | clipboard history (cliphist → rofi) | toggle floating | changed |
| 20 | Super+B | Helium | unbound | new |
| 21 | Super+W | close window | Super+Q | changed; Super+Q unbound |
| 22 | Super+Escape | toggle shortcut inhibit | same | keep |

## Super+Shift

| # | Key | Action | niri default | Change |
|---|---|---|---|---|
| 23 | Super+Shift+←/→, H/L | move column left/right | Super+Ctrl+… | moved |
| 24 | Super+Shift+↑/↓, K/J | move window up/down | Super+Ctrl+… | moved |
| 25 | Super+Shift+Home/End | move column to first/last | Super+Ctrl+… | moved |
| 26 | Super+Shift+1…9 | unbound | Super+Ctrl+1…9: to workspace N | removed; 3/4/5 are screenshots |
| 27 | Super+Shift+Minus/Equal | window height −/+10% | same | keep |
| 28 | Super+Shift+R | cycle preset window height | Super+Ctrl+Shift+R | moved; width-back (was Super+Shift+R) removed |
| 29 | Super+Shift+F | fullscreen window | same | keep |
| 30 | Super+Shift+C | center visible columns | Super+Ctrl+C | moved |
| 31 | Super+Shift+V | toggle floating | Super+V | moved |
| 32 | Super+Shift+Tab | switch focus floating/tiling | Super+Shift+V | moved |
| 33 | Super+Shift+3 / 4 / 5 | `screenshot-screen` / `screenshot` (area) / `screenshot-window` | Print keys | macOS Cmd+Shift+3/4/5 layout; Print keys stay |
| 34 | Super+Shift+A | open newest screenshot in swappy | unbound | new |

## Super+Ctrl

| # | Key | Action | niri default | Change |
|---|---|---|---|---|
| 35 | Super+Ctrl+L | lock (gtklock) | Super+Alt+L: swaylock | changed |
| 36 | Super+Ctrl+E | quit niri | Super+Shift+E | moved |
| 37 | Super+Ctrl+P | power off monitors | Super+Shift+P | moved |
| 38 | Super+Ctrl+T | toggle light/dark (`darkman toggle`) | unbound | new |
| 39 | Super+Ctrl+N | notifications do-not-disturb (bar) | unbound | new |

## Super+Alt

| # | Key | Action | niri default | Change |
|---|---|---|---|---|
| 40 | Super+Alt+←/→/↑/↓, HJKL | focus monitor | Super+Shift+… | moved |
| 41 | Super+Alt+Shift+←/→/↑/↓, HJKL | move column to monitor | Super+Shift+Ctrl+… | moved |
| 42 | Super+Alt+S | toggle orca screen reader | same | keep |

## No modifier

| # | Key | Action | niri default | Change |
|---|---|---|---|---|
| 43 | Print / Ctrl+Print / Alt+Print | screenshot region / screen / window | same | keep |
| 44 | volume, mic mute, media keys | wpctl, playerctl | same | keep |
| 45 | brightness keys | `delphics brightness up` / `down` (±10 %, shown in the bar HUD) | brightnessctl | changed |
| 46 | Ctrl+Alt+Delete | quit niri | same | keep |

## Removed

| # | niri default | Reason |
|---|---|---|
| 47 | Super+Shift+/ hotkey overlay (and startup overlay) | the hint bar replaces it; set `hotkey-overlay { skip-at-startup }` |
| 48 | Super+U/I, Super+PageUp/PageDown, Super+Wheel↑/↓ | workspace focus |
| 49 | Super+Ctrl+U/I/PageUp/PageDown, Super+Ctrl+Wheel | move to workspace |
| 50 | Super+Shift+U/I/PageUp/PageDown | move workspace |
| 51 | Super+Ctrl+Wheel←/→, Super+Ctrl+Shift+Wheel↑/↓ | move column by wheel; Ctrl is the system group |
| 52 | Super+Ctrl+R | reset window height |
