package main

import (
	"fmt"
	"log"
	"os/exec"
	"reflect"

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
	// slider makes the entry a 0 to 1 slider at value, label beside it
	slider bool
	value  float64
}

// menuHandlers receive what the user does in a menu.
type menuHandlers struct {
	activate func(id string)
	change   func(id string, value float64)
}

// openMenu is a popover menu on screen.
type openMenu struct {
	pop     *gtk.PopoverMenu
	h       menuHandlers
	entries []menuEntry
	// sliders by entry index; only flat menus (plugin menus) have sliders
	sliders map[int]*gtk.Scale
	// gen numbers rebuilds, so the custom ids of a rebuilt menu never collide with the old ones
	gen int
	// setting is true while the bar moves a slider itself, so that is not reported as a user change
	setting bool
}

type custom struct {
	name  string
	label string
	scale *gtk.Scale
}

// build turns entries into a menu model, the action group ("m.N") its items call, and the slider
// widgets to place at their custom ids.
func (m *openMenu) build(entries []menuEntry) (*gio.Menu, *gio.SimpleActionGroup, []custom) {
	group := gio.NewSimpleActionGroup()
	var customs []custom
	m.gen++
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
			name := fmt.Sprintf("e%d", n)
			n++
			switch {
			case e.sub != nil:
				section.AppendSubmenu(e.label, build(e.sub))
			case e.slider:
				cname := fmt.Sprintf("g%d%s", m.gen, name)
				item := gio.NewMenuItem("", "")
				item.SetAttributeValue("custom", glib.NewVariantString(cname))
				section.AppendItem(item)
				customs = append(customs, custom{name: cname, label: e.label, scale: m.slider(e)})
			default:
				var action *gio.SimpleAction
				if e.checked {
					action = gio.NewSimpleActionStateful(name, nil, glib.NewVariantBoolean(true))
				} else {
					action = gio.NewSimpleAction(name, nil)
				}
				// an entry without id is a status line: a disabled action greys it out and skips it on hover
				action.SetEnabled(!e.disabled && e.id != "")
				id := e.id
				action.ConnectActivate(func(*glib.Variant) { m.h.activate(id) })
				group.AddAction(action)
				section.Append(e.label, "m."+name)
			}
		}
		flush()
		return menu
	}
	return build(entries), group, customs
}

func (m *openMenu) slider(e menuEntry) *gtk.Scale {
	scale := gtk.NewScaleWithRange(gtk.OrientationHorizontal, 0, 1, 0.01)
	scale.SetDrawValue(false)
	scale.SetHExpand(true)
	scale.SetSizeRequest(160, -1)
	scale.SetValue(e.value)
	scale.SetSensitive(!e.disabled)
	id := e.id
	scale.ConnectValueChanged(func() {
		if !m.setting && m.h.change != nil {
			m.h.change(id, scale.Value())
		}
	})
	return scale
}

// set shows entries in the popover, adding the sliders' widgets.
func (m *openMenu) set(entries []menuEntry) {
	model, group, customs := m.build(entries)
	m.pop.SetMenuModel(model)
	m.pop.InsertActionGroup("m", group)
	m.sliders = map[int]*gtk.Scale{}
	for _, c := range customs {
		box := gtk.NewBox(gtk.OrientationHorizontal, 8)
		box.AddCSSClass("slider")
		label := gtk.NewLabel(c.label)
		label.SetXAlign(0)
		label.SetSizeRequest(70, -1)
		box.Append(label)
		box.Append(c.scale)
		if !m.pop.AddChild(box, c.name) {
			log.Printf("menu: no place for slider %q", c.label)
		}
	}
	for i, e := range entries {
		if e.slider {
			m.sliders[i] = customs[0].scale
			customs = customs[1:]
		}
	}
	m.entries = entries
}

// update follows a changed menu. When only slider values changed, the sliders move in place, so a
// slider being dragged is not rebuilt under the pointer; anything else rebuilds the menu.
func (m *openMenu) update(entries []menuEntry) {
	if !sameButValues(m.entries, entries) {
		m.set(entries)
		return
	}
	m.setting = true
	for i, e := range entries {
		if e.slider {
			if scale := m.sliders[i]; scale != nil && abs(scale.Value()-e.value) > 0.005 {
				scale.SetValue(e.value)
			}
		}
	}
	m.setting = false
	m.entries = entries
}

func sameButValues(a, b []menuEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		x.value, y.value = 0, 0
		if x.sub != nil || y.sub != nil || !reflect.DeepEqual(x, y) {
			return false
		}
	}
	return true
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// popupMenu shows entries below parent and removes the popover again when it closes. parent must
// have a layout manager (a Box, not a Label): only then does GTK resize the popover once the menu's
// final size is known; otherwise it stays at its first, too small size and scrolls.
func popupMenu(parent gtk.Widgetter, entries []menuEntry, h menuHandlers) *openMenu {
	m := &openMenu{pop: gtk.NewPopoverMenuFromModel(nil), h: h}
	m.set(entries)
	m.pop.SetHasArrow(false)
	m.pop.SetPosition(gtk.PosBottom)
	m.pop.SetParent(parent)
	m.pop.ConnectClosed(func() { glib.IdleAdd(m.pop.Unparent) })
	// While open, the popover grabs input inside the bar: GTK sends clicks on the bar to the popover,
	// and the compositor only dismisses it for clicks on other clients. A press outside the popover's
	// own area is such a bar click; it closes the menu, like a click anywhere else.
	outside := gtk.NewGestureClick()
	outside.SetButton(0)
	outside.SetPropagationPhase(gtk.PhaseCapture)
	outside.ConnectPressed(func(_ int, x, y float64) {
		if x < 0 || y < 0 || x >= float64(m.pop.Width()) || y >= float64(m.pop.Height()) {
			m.pop.Popdown()
		}
	})
	m.pop.AddController(outside)
	m.pop.Popup()
	return m
}

// pluginMenu converts a plugin's flat menu.
func pluginMenu(entries []baritems.MenuEntry) []menuEntry {
	out := make([]menuEntry, len(entries))
	for i, e := range entries {
		out[i] = menuEntry{label: e.Label, id: e.ID, disabled: e.Disabled, checked: e.Checked, section: e.Section,
			slider: e.Slider, value: e.Value}
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
