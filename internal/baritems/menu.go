package baritems

import (
	"unicode/utf8"

	"github.com/godbus/dbus/v5"
)

const (
	maxMenu      = 40
	maxMenuLabel = 100
)

// MenuEntry is one line of an item's menu. The bar shows the menu when the item is clicked and
// emits Activated(item, ID) when an entry is chosen.
type MenuEntry struct {
	// ID is sent back on activation; an entry without ID is a plain label, such as a status line
	ID    string
	Label string
	// Disabled greys the entry out
	Disabled bool
	// Checked shows a check mark
	Checked bool
	// Section starts a new group, drawn with a separator above it
	Section bool
}

// Menu converts entries to the D-Bus form of the "menu" property, aa{sv}.
func Menu(entries []MenuEntry) []map[string]dbus.Variant {
	out := make([]map[string]dbus.Variant, 0, len(entries))
	for _, e := range entries {
		m := map[string]dbus.Variant{"label": dbus.MakeVariant(e.Label)}
		if e.ID != "" {
			m["id"] = dbus.MakeVariant(e.ID)
		}
		if e.Disabled {
			m["enabled"] = dbus.MakeVariant(false)
		}
		if e.Checked {
			m["checked"] = dbus.MakeVariant(true)
		}
		if e.Section {
			m["section"] = dbus.MakeVariant(true)
		}
		out = append(out, m)
	}
	return out
}

// parseMenu validates the D-Bus form of the "menu" property.
func parseMenu(v dbus.Variant) ([]MenuEntry, *dbus.Error) {
	raw, ok := v.Value().([]map[string]dbus.Variant)
	if !ok {
		return nil, invalid("property \"menu\" has type %s, want aa{sv}", v.Signature())
	}
	if len(raw) > maxMenu {
		return nil, invalid("menu has more than %d entries", maxMenu)
	}
	entries := make([]MenuEntry, 0, len(raw))
	for i, m := range raw {
		var e MenuEntry
		for k, v := range m {
			var ok bool
			switch k {
			case "id":
				e.ID, ok = v.Value().(string)
				ok = ok && len(e.ID) <= maxID
			case "label":
				e.Label, ok = v.Value().(string)
				ok = ok && utf8.RuneCountInString(e.Label) <= maxMenuLabel
			case "enabled":
				var enabled bool
				enabled, ok = v.Value().(bool)
				e.Disabled = !enabled
			case "checked":
				e.Checked, ok = v.Value().(bool)
			case "section":
				e.Section, ok = v.Value().(bool)
			default:
				return nil, invalid("menu entry %d: unknown key %q; known: id, label, enabled, checked, section", i, k)
			}
			if !ok {
				return nil, invalid("menu entry %d: bad %q", i, k)
			}
		}
		entries = append(entries, e)
	}
	return entries, nil
}
