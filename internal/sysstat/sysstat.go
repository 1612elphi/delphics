// Package sysstat reads battery (UPower) and network (NetworkManager) state over the system bus.
package sysstat

import (
	"fmt"

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

// Battery returns e.g. "bat 64%", "chg 64%", or "" when there is no battery.
func (s *Stat) Battery() string {
	const dev = "/org/freedesktop/UPower/devices/DisplayDevice"
	const iface = "org.freedesktop.UPower.Device"
	present, err := s.prop("org.freedesktop.UPower", dev, iface, "IsPresent")
	if err != nil || !present.Value().(bool) {
		return ""
	}
	pct, err := s.prop("org.freedesktop.UPower", dev, iface, "Percentage")
	if err != nil {
		return ""
	}
	state, _ := s.prop("org.freedesktop.UPower", dev, iface, "State")
	label := "bat"
	if st, ok := state.Value().(uint32); ok && (st == upCharging || st == upFull) {
		label = "chg"
	}
	return fmt.Sprintf("%s %.0f%%", label, pct.Value().(float64))
}

// Network returns the primary connection, e.g. "HomeNet 72%", "wired", or "offline".
func (s *Stat) Network() string {
	const nm = "org.freedesktop.NetworkManager"
	primary, err := s.prop(nm, "/org/freedesktop/NetworkManager", nm, "PrimaryConnection")
	if err != nil {
		return "offline"
	}
	conn := primary.Value().(dbus.ObjectPath)
	if conn == "/" {
		return "offline"
	}
	id, err := s.prop(nm, conn, nm+".Connection.Active", "Id")
	if err != nil {
		return "offline"
	}
	typ, _ := s.prop(nm, conn, nm+".Connection.Active", "Type")
	if t, _ := typ.Value().(string); t != "802-11-wireless" {
		return "wired"
	}
	ap, err := s.prop(nm, conn, nm+".Connection.Active", "SpecificObject")
	if err != nil {
		return id.Value().(string)
	}
	strength, err := s.prop(nm, ap.Value().(dbus.ObjectPath), nm+".AccessPoint", "Strength")
	if err != nil {
		return id.Value().(string)
	}
	return fmt.Sprintf("%s %d%%", id.Value().(string), strength.Value().(byte))
}
