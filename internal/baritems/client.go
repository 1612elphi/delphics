package baritems

import (
	"log"
	"sync"

	"github.com/godbus/dbus/v5"
)

// Props are item properties for Client.Set: text, icon and tooltip (string), order (int32), urgent and bold
// (bool), menu ([]MenuEntry).
type Props map[string]any

// Client keeps one item on the bar for a plugin. It remembers the item's properties and sends them
// again whenever the bar (re)starts, so plugins can start before the bar and survive its restarts.
type Client struct {
	conn  *dbus.Conn
	id    string
	mu    sync.Mutex
	props map[string]dbus.Variant
	// warned is set after a failed send has been logged, so a missing bar logs once
	warned bool
	// Clicks receives the button number of every click on the item; clicks are dropped while it is full.
	Clicks chan uint32
	// Activations receives the ID of every chosen menu entry, dropped while full like Clicks.
	Activations chan string
	// Changes receives slider moves; when full the oldest is dropped, so the last value always arrives.
	Changes chan Change
	// Scrolls receives scroll deltas on the item, positive downwards; dropped while full.
	Scrolls chan float64
}

type Change struct {
	Entry string
	Value float64
}

func NewClient(conn *dbus.Conn, id string) (*Client, error) {
	c := &Client{conn: conn, id: id, props: map[string]dbus.Variant{}, Clicks: make(chan uint32, 4), Activations: make(chan string, 4),
		Changes: make(chan Change, 8), Scrolls: make(chan float64, 16)}
	for _, member := range []string{"Clicked", "Activated", "Changed", "Scrolled"} {
		if err := conn.AddMatchSignal(dbus.WithMatchInterface(Iface), dbus.WithMatchMember(member), dbus.WithMatchArg(0, id)); err != nil {
			return nil, err
		}
	}
	if err := conn.AddMatchSignal(dbus.WithMatchInterface("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, BusName)); err != nil {
		return nil, err
	}
	signals := make(chan *dbus.Signal, 16)
	conn.Signal(signals)
	go func() {
		for sig := range signals {
			switch {
			case sig.Name == "org.freedesktop.DBus.NameOwnerChanged" && len(sig.Body) == 3 && sig.Body[0] == BusName && sig.Body[2] != "":
				c.mu.Lock()
				c.send()
				c.mu.Unlock()
			case sig.Name == Iface+".Clicked" && len(sig.Body) == 2 && sig.Body[0] == id:
				if button, ok := sig.Body[1].(uint32); ok {
					select {
					case c.Clicks <- button:
					default:
					}
				}
			case sig.Name == Iface+".Changed" && len(sig.Body) == 3 && sig.Body[0] == id:
				entry, _ := sig.Body[1].(string)
				value, _ := sig.Body[2].(float64)
				for {
					select {
					case c.Changes <- Change{entry, value}:
					default:
						select {
						case <-c.Changes:
						default:
						}
						continue
					}
					break
				}
			case sig.Name == Iface+".Scrolled" && len(sig.Body) == 2 && sig.Body[0] == id:
				if delta, ok := sig.Body[1].(float64); ok {
					select {
					case c.Scrolls <- delta:
					default:
					}
				}
			case sig.Name == Iface+".Activated" && len(sig.Body) == 2 && sig.Body[0] == id:
				if entry, ok := sig.Body[1].(string); ok {
					select {
					case c.Activations <- entry:
					default:
					}
				}
			}
		}
	}()
	return c, nil
}

// Set merges p into the item's properties and sends them. A bar that is not running is not an
// error: the item appears when the bar starts.
func (c *Client) Set(p Props) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range p {
		if m, ok := v.([]MenuEntry); ok {
			v = Menu(m)
		}
		c.props[k] = dbus.MakeVariant(v)
	}
	return c.send()
}

func (c *Client) send() error {
	err := c.conn.Object(BusName, Path).Call(Iface+".Set", 0, c.id, c.props).Err
	if dbusErr, ok := err.(dbus.Error); ok && dbusErr.Name == "org.freedesktop.DBus.Error.ServiceUnknown" {
		if !c.warned {
			log.Printf("bar not running; item %q appears when it starts", c.id)
			c.warned = true
		}
		return nil
	}
	if err == nil {
		c.warned = false
	}
	return err
}

// ShowLevel flashes a level in the bar's HUD; value is 0 to 1, or negative for no meter.
// Without a running bar it does nothing.
func ShowLevel(conn *dbus.Conn, icon string, value float64, text string) error {
	err := conn.Object(BusName, Path).Call(Iface+".ShowLevel", 0, icon, value, text).Err
	if dbusErr, ok := err.(dbus.Error); ok && dbusErr.Name == "org.freedesktop.DBus.Error.ServiceUnknown" {
		return nil
	}
	return err
}
