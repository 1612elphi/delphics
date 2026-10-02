-- installed to /usr/share/delphics/wezterm/wezterm.lua; ~/.config/wezterm/wezterm.lua loads it with dofile
local wezterm = require 'wezterm'
local config = wezterm.config_builder()

config.font = wezterm.font 'Iosevka'
config.font_size = 11
config.window_decorations = 'NONE'
config.hide_tab_bar_if_only_one_tab = true
config.use_fancy_tab_bar = false
config.window_padding = { left = 10, right = 10, top = 8, bottom = 8 }
config.check_for_updates = false

-- ponytail: dark palette hardcoded until darkman hooks switch palettes; values from themes/delphics/palette.toml
config.colors = {
  foreground = '#ebe4d2',
  background = '#091509',
  cursor_bg = '#c2ad61',
  cursor_fg = '#091b0a',
  cursor_border = '#c2ad61',
  selection_bg = '#213321',
  selection_fg = '#ebe4d2',
  ansi = { '#182619', '#e18881', '#77b779', '#bfa14c', '#74a7e8', '#cf8ac0', '#36baba', '#d4cdbc' },
  brights = { '#959074', '#ffaba3', '#9bd69c', '#ddc276', '#9dc7fe', '#edacde', '#6cd9d8', '#f6f2e7' },
  tab_bar = {
    background = '#101f10',
    active_tab = { bg_color = '#091509', fg_color = '#ebe4d2' },
    inactive_tab = { bg_color = '#101f10', fg_color = '#959074' },
    inactive_tab_hover = { bg_color = '#182619', fg_color = '#ebe4d2' },
    new_tab = { bg_color = '#101f10', fg_color = '#959074' },
    new_tab_hover = { bg_color = '#182619', fg_color = '#ebe4d2' },
  },
}

return config
