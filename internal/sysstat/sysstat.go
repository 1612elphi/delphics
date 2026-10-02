// Package sysstat reads battery (UPower) and network (NetworkManager) state over the system bus.
package sysstat

import (
	"github.com/godbus/dbus/v5"
)

type Stat struct {
	conn *dbus.Conn
}

func New() (*Stat, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, err
	}
	return &Stat{conn: conn}, nil
}

func (s *Stat) prop(dest string, path dbus.ObjectPath, iface, name string) (dbus.Variant, error) {
	return s.conn.Object(dest, path).GetProperty(iface + "." + name)
}

// UPower.Device State values: 1 charging, 4 fully charged.
const (
	upCharging = 1
	upFull     = 4
)

type Battery struct {
	Present  bool
	Percent  float64
	Charging bool
	Full     bool
}

// Battery reads UPower's display device, the combined state of all batteries.
func (s *Stat) Battery() Battery {
	const dev = "/org/freedesktop/UPower/devices/DisplayDevice"
	const iface = "org.freedesktop.UPower.Device"
	present, err := s.prop("org.freedesktop.UPower", dev, iface, "IsPresent")
	if err != nil || !present.Value().(bool) {
		return Battery{}
	}
	pct, err := s.prop("org.freedesktop.UPower", dev, iface, "Percentage")
	if err != nil {
		return Battery{}
	}
	b := Battery{Present: true, Percent: pct.Value().(float64)}
	state, _ := s.prop("org.freedesktop.UPower", dev, iface, "State")
	st, _ := state.Value().(uint32)
	b.Charging, b.Full = st == upCharging, st == upFull
	return b
}

type NetKind int

const (
	Offline NetKind = iota
	Wired
	Wireless
	// Other is any other primary connection type, such as a VPN or a modem
	Other
)

type Network struct {
	Kind NetKind
	// Name is the connection name, the SSID for Wi-Fi
	Name string
	// Strength is the Wi-Fi signal in percent, -1 when unknown
	Strength int
}

// Network reads NetworkManager's primary connection.
func (s *Stat) Network() Network {
	const nm = "org.freedesktop.NetworkManager"
	primary, err := s.prop(nm, "/org/freedesktop/NetworkManager", nm, "PrimaryConnection")
	if err != nil {
		return Network{}
	}
	conn := primary.Value().(dbus.ObjectPath)
	if conn == "/" {
		return Network{}
	}
	id, err := s.prop(nm, conn, nm+".Connection.Active", "Id")
	if err != nil {
		return Network{}
	}
	n := Network{Kind: Other, Name: id.Value().(string), Strength: -1}
	typ, _ := s.prop(nm, conn, nm+".Connection.Active", "Type")
	switch t, _ := typ.Value().(string); t {
	case "802-3-ethernet":
		n.Kind = Wired
	case "802-11-wireless":
		n.Kind = Wireless
		if ap, err := s.prop(nm, conn, nm+".Connection.Active", "SpecificObject"); err == nil {
			if st, err := s.prop(nm, ap.Value().(dbus.ObjectPath), nm+".AccessPoint", "Strength"); err == nil {
				n.Strength = int(st.Value().(byte))
			}
		}
	}
	return n
}
