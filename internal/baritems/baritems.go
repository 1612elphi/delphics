// Package baritems is the bar's plugin API: other processes put items on the right side of the bar
// over D-Bus and hear about clicks. An item lives as long as the D-Bus connection that set it.
package baritems

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

const (
	BusName = "tools.delphi.Delphics.BarItems"
	Path    = "/tools/delphi/Delphics/BarItems"
	Iface   = "tools.delphi.Delphics.BarItems1"
)

// iconName allows theme names only; empty clears the icon
var iconName = regexp.MustCompile(`^[A-Za-z0-9_.+-]{0,100}$`)

const (
	maxID   = 64
	maxText = 200
	// maxPerOwner keeps one runaway plugin from filling the bar
	maxPerOwner = 16
)

type Item struct {
	ID   string
	Text string
	// Icon is an icon theme name such as "battery-level-60-symbolic", shown left of the text
	Icon    string
	Tooltip string
	// Order sorts items left to right; ties sort by ID
	Order int32
	// Urgent is amber bold, Bold is bright bold text
	Urgent, Bold bool
	owner        string
}

// Server holds the items. onChange gets the full sorted item list after every change, on a D-Bus goroutine.
type Server struct {
	conn     *dbus.Conn
	onChange func([]Item)
	mu       sync.Mutex
	items    map[string]*Item
}

// Serve exports the API on conn and takes the bus name; it fails if another bar holds it.
func Serve(conn *dbus.Conn, onChange func([]Item)) (*Server, error) {
	s := &Server{conn: conn, onChange: onChange, items: map[string]*Item{}}
	if err := conn.Export(methods{s}, Path, Iface); err != nil {
		return nil, err
	}
	if err := conn.Export(introspect.Introspectable(introspection), Path, "org.freedesktop.DBus.Introspectable"); err != nil {
		return nil, err
	}
	// items go away with their owner's connection
	if err := conn.AddMatchSignal(dbus.WithMatchInterface("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged")); err != nil {
		return nil, err
	}
	signals := make(chan *dbus.Signal, 16)
	conn.Signal(signals)
	go func() {
		for sig := range signals {
			if sig.Name != "org.freedesktop.DBus.NameOwnerChanged" || len(sig.Body) != 3 {
				continue
			}
			if name, _ := sig.Body[0].(string); strings.HasPrefix(name, ":") && sig.Body[2] == "" {
				s.dropOwner(name)
			}
		}
	}()
	reply, err := conn.RequestName(BusName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return nil, err
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return nil, fmt.Errorf("%s is owned by another bar", BusName)
	}
	return s, nil
}

// Click tells the item's plugin that the user clicked it; button is the GDK button number.
func (s *Server) Click(id string, button uint32) {
	s.conn.Emit(Path, Iface+".Clicked", id, button)
}

func (s *Server) dropOwner(owner string) {
	s.mu.Lock()
	changed := false
	for id, it := range s.items {
		if it.owner == owner {
			delete(s.items, id)
			changed = true
		}
	}
	s.mu.Unlock()
	if changed {
		s.changed()
	}
}

func (s *Server) changed() {
	s.mu.Lock()
	list := make([]Item, 0, len(s.items))
	for _, it := range s.items {
		list = append(list, *it)
	}
	s.mu.Unlock()
	sort.Slice(list, func(i, j int) bool {
		if list[i].Order != list[j].Order {
			return list[i].Order < list[j].Order
		}
		return list[i].ID < list[j].ID
	})
	s.onChange(list)
}

// set applies props to the item, creating it for owner when new.
func (s *Server) set(owner, id string, props map[string]dbus.Variant) *dbus.Error {
	if id == "" || len(id) > maxID {
		return invalid("id must be 1 to %d bytes", maxID)
	}
	// validate everything before changing anything
	var it Item
	s.mu.Lock()
	cur, exists := s.items[id]
	if exists {
		it = *cur
	}
	n := 0
	for _, other := range s.items {
		if other.owner == owner {
			n++
		}
	}
	s.mu.Unlock()
	if exists && it.owner != owner {
		return invalid("item %q belongs to another connection", id)
	}
	if !exists && n >= maxPerOwner {
		return invalid("at most %d items per connection", maxPerOwner)
	}
	for k, v := range props {
		var ok bool
		switch k {
		case "text":
			it.Text, ok = v.Value().(string)
			if ok && utf8.RuneCountInString(it.Text) > maxText {
				return invalid("text longer than %d characters", maxText)
			}
		case "icon":
			it.Icon, ok = v.Value().(string)
			if ok && !iconName.MatchString(it.Icon) {
				return invalid("icon must be an icon theme name, not a path")
			}
		case "tooltip":
			it.Tooltip, ok = v.Value().(string)
		case "order":
			it.Order, ok = v.Value().(int32)
		case "urgent":
			it.Urgent, ok = v.Value().(bool)
		case "bold":
			it.Bold, ok = v.Value().(bool)
		default:
			return invalid("unknown property %q; known: text, icon, tooltip, order, urgent, bold", k)
		}
		if !ok {
			return invalid("property %q has type %s", k, v.Signature())
		}
	}
	it.ID, it.owner = id, owner

	s.mu.Lock()
	// another call may have taken the id meanwhile
	if cur, ok := s.items[id]; ok && cur.owner != owner {
		s.mu.Unlock()
		return invalid("item %q belongs to another connection", id)
	}
	s.items[id] = &it
	s.mu.Unlock()
	s.changed()
	return nil
}

func (s *Server) remove(owner, id string) *dbus.Error {
	s.mu.Lock()
	it, ok := s.items[id]
	if ok && it.owner == owner {
		delete(s.items, id)
	}
	s.mu.Unlock()
	if !ok {
		return nil
	}
	if it.owner != owner {
		return invalid("item %q belongs to another connection", id)
	}
	s.changed()
	return nil
}

func invalid(format string, args ...any) *dbus.Error {
	return dbus.NewError("org.freedesktop.DBus.Error.InvalidArgs", []any{fmt.Sprintf(format, args...)})
}

// methods keeps the exported method set to the API's names only.
type methods struct{ s *Server }

// Set creates or updates an item. props may hold text (s), icon (s), tooltip (s), order (i), urgent (b) and bold (b);
// properties left out keep their current value.
func (m methods) Set(sender dbus.Sender, id string, props map[string]dbus.Variant) *dbus.Error {
	return m.s.set(string(sender), id, props)
}

// Remove deletes one of the caller's items; removing an unknown id is not an error.
func (m methods) Remove(sender dbus.Sender, id string) *dbus.Error {
	return m.s.remove(string(sender), id)
}

const introspection = introspect.IntrospectDeclarationString + `<node>
 <interface name="` + Iface + `">
  <method name="Set"><arg name="id" direction="in" type="s"/><arg name="props" direction="in" type="a{sv}"/></method>
  <method name="Remove"><arg name="id" direction="in" type="s"/></method>
  <signal name="Clicked"><arg name="id" type="s"/><arg name="button" type="u"/></signal>
 </interface>` + introspect.IntrospectDataString + `</node>`
