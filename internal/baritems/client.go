package baritems

import (
	"log"
	"sync"

	"github.com/godbus/dbus/v5"
)

// Props are item properties for Client.Set: text, icon and tooltip (string), order (int32), urgent and bold (bool).
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
}

func NewClient(conn *dbus.Conn, id string) (*Client, error) {
	c := &Client{conn: conn, id: id, props: map[string]dbus.Variant{}, Clicks: make(chan uint32, 4)}
	if err := conn.AddMatchSignal(dbus.WithMatchInterface(Iface), dbus.WithMatchMember("Clicked"), dbus.WithMatchArg(0, id)); err != nil {
		return nil, err
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
