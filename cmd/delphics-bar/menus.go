package main

import (
	"fmt"
	"log"
	"os/exec"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"delphics.delphi.tools/internal/baritems"
)

// menuEntry is one line of a popover menu built by the bar.
type menuEntry struct {
	label string
	// id is passed to the menu's activate function; empty makes a plain, greyed label
	id       string
	disabled bool
	checked  bool
	// section starts a new group with a separator above it
	section bool
	// sub makes the entry open a submenu
	sub []menuEntry
}

// buildMenu turns entries into a menu model and the action group ("m.N") its items call.
func buildMenu(entries []menuEntry, activate func(id string)) (*gio.Menu, *gio.SimpleActionGroup) {
	group := gio.NewSimpleActionGroup()
	n := 0
	var build func([]menuEntry) *gio.Menu
	build = func(entries []menuEntry) *gio.Menu {
		menu := gio.NewMenu()
		section := gio.NewMenu()
		flush := func() {
			if section.NItems() > 0 {
				menu.AppendSection("", section)
			}
			section = gio.NewMenu()
		}
		for _, e := range entries {
			if e.section {
				flush()
			}
			if e.sub != nil {
				section.AppendSubmenu(e.label, build(e.sub))
				continue
			}
			name := fmt.Sprintf("e%d", n)
			n++
			var action *gio.SimpleAction
			if e.checked {
				action = gio.NewSimpleActionStateful(name, nil, glib.NewVariantBoolean(true))
			} else {
				action = gio.NewSimpleAction(name, nil)
			}
			// an entry without id is a status line: a disabled action greys it out and skips it on hover
			action.SetEnabled(!e.disabled && e.id != "")
			id := e.id
			action.ConnectActivate(func(*glib.Variant) { activate(id) })
			group.AddAction(action)
			section.Append(e.label, "m."+name)
		}
		flush()
		return menu
	}
	return build(entries), group
}

// popupMenu shows entries below parent and removes the popover again when it closes. parent must
// have a layout manager (a Box, not a Label): only then does GTK resize the popover once the menu's
// final size is known; otherwise it stays at its first, too small size and scrolls.
func popupMenu(parent gtk.Widgetter, entries []menuEntry, activate func(id string)) *gtk.PopoverMenu {
	model, group := buildMenu(entries, activate)
	pop := gtk.NewPopoverMenuFromModel(model)
	pop.SetHasArrow(false)
	pop.SetPosition(gtk.PosBottom)
	pop.InsertActionGroup("m", group)
	pop.SetParent(parent)
	pop.ConnectClosed(func() { glib.IdleAdd(pop.Unparent) })
	pop.Popup()
	return pop
}

// pluginMenu converts a plugin's flat menu.
func pluginMenu(entries []baritems.MenuEntry) []menuEntry {
	out := make([]menuEntry, len(entries))
	for i, e := range entries {
		out[i] = menuEntry{label: e.Label, id: e.ID, disabled: e.Disabled, checked: e.Checked, section: e.Section}
	}
	return out
}

// systemMenu is the menu behind the app name: the focused window, then the session.
// Log out, restart and power off sit in submenus, so each takes a second, deliberate click.
func systemMenu(haveWindow bool) []menuEntry {
	return []menuEntry{
		{label: "Close window", id: "close", disabled: !haveWindow},
		{label: "Lock", id: "lock", section: true},
		{label: "Suspend", id: "suspend"},
		{label: "Log out…", section: true, sub: []menuEntry{{label: "Log out now", id: "logout"}}},
		{label: "Restart…", sub: []menuEntry{{label: "Restart now", id: "reboot"}}},
		{label: "Power off…", sub: []menuEntry{{label: "Power off now", id: "poweroff"}}},
	}
}

var systemCommands = map[string][]string{
	"close":    {"niri", "msg", "action", "close-window"},
	"lock":     {"gtklock", "-d"},
	"suspend":  {"systemctl", "suspend"},
	"logout":   {"niri", "msg", "action", "quit", "--skip-confirmation"},
	"reboot":   {"systemctl", "reboot"},
	"poweroff": {"systemctl", "poweroff"},
}

func runSystem(id string) {
	argv, ok := systemCommands[id]
	if !ok {
		return
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	if err := cmd.Start(); err != nil {
		log.Printf("%s: %v", id, err)
		return
	}
	go cmd.Wait()
}
