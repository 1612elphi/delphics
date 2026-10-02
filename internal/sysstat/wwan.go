package sysstat

import (
	"errors"
	"strings"

	"github.com/godbus/dbus/v5"
)

const (
	mm     = "org.freedesktop.ModemManager1"
	mmPath = "/org/freedesktop/ModemManager1"
	mmIf   = mm + ".Modem"
)

// ModemManager modem states (MMModemState).
const (
	mmFailed     = -1
	mmLocked     = 2
	mmDisabled   = 3
	mmEnabled    = 6
	mmSearching  = 7
	mmRegistered = 8
)

type ModemState int

const (
	ModemNone ModemState = iota
	// ModemFailed covers a missing or unusable SIM and other failures
	ModemFailed
	ModemLocked
	ModemOff
	ModemSearching
	ModemRegistered
)

type Modem struct {
	State    ModemState
	Operator string
	// Tech is the access technology, e.g. "LTE", "5G"
	Tech string
	// Signal is the signal quality in percent
	Signal int
	// DataOn is true while NetworkManager has a mobile broadband connection up
	DataOn bool
	path   dbus.ObjectPath
	nmDev  dbus.ObjectPath
}

// access technology bits (MMModemAccessTechnology), fastest first
var techs = []struct {
	bit  uint32
	name string
}{{1 << 15, "5G"}, {1 << 14, "LTE"}, {1<<5 | 1<<6 | 1<<7 | 1<<8 | 1<<9, "3G"}, {1 << 3, "EDGE"}, {1 << 2, "GPRS"}}

func techName(bits uint32) string {
	for _, t := range techs {
		if bits&t.bit != 0 {
			return t.name
		}
	}
	return ""
}

// Modem reads the first modem from ModemManager and whether mobile data is up.
func (s *Stat) Modem() Modem {
	var objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	if err := s.conn.Object(mm, mmPath).Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objects); err != nil {
		return Modem{}
	}
	var m Modem
	for p, ifs := range objects {
		props, ok := ifs[mmIf]
		if !ok || (m.path != "" && p > m.path) {
			continue
		}
		m = Modem{path: p}
		state, _ := props["State"].Value().(int32)
		switch {
		case state == mmFailed:
			m.State = ModemFailed
		case state == mmLocked:
			m.State = ModemLocked
		case state <= mmDisabled:
			m.State = ModemOff
		case state < mmRegistered:
			m.State = ModemSearching
		default:
			m.State = ModemRegistered
		}
		if q, ok := props["SignalQuality"].Value().([]any); ok && len(q) == 2 {
			sig, _ := q[0].(uint32)
			m.Signal = int(sig)
		}
		tech, _ := props["AccessTechnologies"].Value().(uint32)
		m.Tech = techName(tech)
		if g, ok := ifs[mmIf+".Modem3gpp"]; ok {
			m.Operator, _ = g["OperatorName"].Value().(string)
		}
	}
	if m.path != "" {
		m.nmDev, m.DataOn = s.modemDevice()
	}
	return m
}

// NetworkManager device type for modems.
const nmDeviceModem = 8

// modemDevice finds NetworkManager's modem device and whether it has an active connection.
func (s *Stat) modemDevice() (dbus.ObjectPath, bool) {
	var devices []dbus.ObjectPath
	if err := s.conn.Object(nm, nmPath).Call(nm+".GetDevices", 0).Store(&devices); err != nil {
		return "", false
	}
	for _, d := range devices {
		if v, err := s.prop(nm, d, nm+".Device", "DeviceType"); err != nil || v.Value() != uint32(nmDeviceModem) {
			continue
		}
		active, _ := s.prop(nm, d, nm+".Device", "ActiveConnection")
		path, _ := active.Value().(dbus.ObjectPath)
		return d, path != "" && path != "/"
	}
	return "", false
}

// SetModemEnabled turns the modem's radio on or off.
func (s *Stat) SetModemEnabled(m Modem, on bool) error {
	return s.conn.Object(mm, m.path).Call(mmIf+".Enable", 0, on).Err
}

// SetMobileData brings mobile broadband up through a saved NetworkManager connection, creating one
// with automatic APN detection if there is none, or takes it down.
func (s *Stat) SetMobileData(m Modem, on bool) error {
	if m.nmDev == "" {
		return errors.New("NetworkManager does not manage the modem")
	}
	if !on {
		return s.conn.Object(nm, m.nmDev).Call(nm+".Device.Disconnect", 0).Err
	}
	if saved := s.savedOfType("gsm"); saved != "" {
		return s.conn.Object(nm, nmPath).Call(nm+".ActivateConnection", 0, saved, m.nmDev, dbus.ObjectPath("/")).Err
	}
	settings := map[string]map[string]dbus.Variant{
		"connection": {"type": dbus.MakeVariant("gsm"), "id": dbus.MakeVariant("Mobile broadband"),
			"autoconnect": dbus.MakeVariant(false)},
		"gsm": {"auto-config": dbus.MakeVariant(true)},
	}
	return s.conn.Object(nm, nmPath).Call(nm+".AddAndActivateConnection", 0, settings, m.nmDev, dbus.ObjectPath("/")).Err
}

func (s *Stat) savedOfType(typ string) dbus.ObjectPath {
	for _, c := range s.savedConnections() {
		if t, _ := c.settings["connection"]["type"].Value().(string); t == typ {
			return c.path
		}
	}
	return ""
}

// WatchModem calls onChange after the modem or NetworkManager's devices change, and when either
// service restarts.
func (s *Stat) WatchModem(onChange func()) error {
	if err := s.conn.AddMatchSignal(dbus.WithMatchSender(mm), dbus.WithMatchInterface("org.freedesktop.DBus.ObjectManager")); err != nil {
		return err
	}
	return s.watch([]string{mm, nm}, func(sig *dbus.Signal) bool {
		// access point updates are Wi-Fi chatter, not about the modem
		return !strings.HasPrefix(string(sig.Path), nmPath+"/AccessPoint/")
	}, onChange)
}
