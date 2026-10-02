// Package sysstat reads and changes battery (UPower), power profile (power-profiles-daemon) and network
// (NetworkManager) state over the system bus, and tells plugins when any of it changes.
package sysstat

import (
	"sort"

	"github.com/godbus/dbus/v5"
)

const (
	upower   = "org.freedesktop.UPower"
	display  = "/org/freedesktop/UPower/devices/DisplayDevice"
	profiles = "org.freedesktop.UPower.PowerProfiles"
	ppPath   = "/org/freedesktop/UPower/PowerProfiles"
	nm       = "org.freedesktop.NetworkManager"
	nmPath   = "/org/freedesktop/NetworkManager"
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
	// Seconds is the time to full while charging and to empty otherwise, 0 when unknown
	Seconds int64
}

// Battery reads UPower's display device, the combined state of all batteries.
func (s *Stat) Battery() Battery {
	const iface = upower + ".Device"
	present, err := s.prop(upower, display, iface, "IsPresent")
	if err != nil || !present.Value().(bool) {
		return Battery{}
	}
	pct, err := s.prop(upower, display, iface, "Percentage")
	if err != nil {
		return Battery{}
	}
	b := Battery{Present: true, Percent: pct.Value().(float64)}
	state, _ := s.prop(upower, display, iface, "State")
	st, _ := state.Value().(uint32)
	b.Charging, b.Full = st == upCharging, st == upFull
	timeProp := "TimeToEmpty"
	if b.Charging {
		timeProp = "TimeToFull"
	}
	if t, err := s.prop(upower, display, iface, timeProp); err == nil {
		b.Seconds, _ = t.Value().(int64)
	}
	return b
}

// PowerProfiles returns the active power profile and the available ones, e.g. "balanced" and
// [power-saver balanced performance]; empty without power-profiles-daemon.
func (s *Stat) PowerProfiles() (active string, available []string) {
	if v, err := s.prop(profiles, ppPath, profiles, "ActiveProfile"); err == nil {
		active, _ = v.Value().(string)
	}
	if v, err := s.prop(profiles, ppPath, profiles, "Profiles"); err == nil {
		list, _ := v.Value().([]map[string]dbus.Variant)
		for _, p := range list {
			if name, ok := p["Profile"].Value().(string); ok {
				available = append(available, name)
			}
		}
	}
	return active, available
}

func (s *Stat) SetPowerProfile(name string) error {
	return s.conn.Object(profiles, ppPath).SetProperty(profiles+".ActiveProfile", dbus.MakeVariant(name))
}

type NetKind int

const (
	Offline NetKind = iota
	Wired
	Wireless
	// Other is any other primary connection type, such as a VPN or a modem
	Other
)

type AccessPoint struct {
	SSID     string
	Strength int
	Secure   bool
	Active   bool
	path     dbus.ObjectPath
}

type Network struct {
	Kind NetKind
	// Name is the primary connection's name, the SSID for Wi-Fi
	Name string
	// Strength is the Wi-Fi signal in percent, -1 when unknown
	Strength int
	// WifiEnabled is NetworkManager's Wi-Fi radio switch; HasWifi is false without a Wi-Fi device
	HasWifi, WifiEnabled bool
	// AccessPoints are the visible Wi-Fi networks, one per SSID, strongest first
	AccessPoints []AccessPoint

	wifiDev, activeConn, activeAP dbus.ObjectPath
}

// Network reads NetworkManager's primary connection and the Wi-Fi device's view.
func (s *Stat) Network() Network {
	n := Network{Strength: -1}
	if v, err := s.prop(nm, nmPath, nm, "WirelessEnabled"); err == nil {
		n.WifiEnabled, _ = v.Value().(bool)
	}
	n.wifiDev = s.wifiDevice()
	n.HasWifi = n.wifiDev != ""
	if n.HasWifi {
		if v, err := s.prop(nm, n.wifiDev, nm+".Device.Wireless", "ActiveAccessPoint"); err == nil {
			n.activeAP, _ = v.Value().(dbus.ObjectPath)
		}
		n.AccessPoints = s.accessPoints(n.wifiDev, n.activeAP)
	}

	primary, err := s.prop(nm, nmPath, nm, "PrimaryConnection")
	if err != nil {
		return n
	}
	n.activeConn, _ = primary.Value().(dbus.ObjectPath)
	if n.activeConn == "/" || n.activeConn == "" {
		n.activeConn = ""
		return n
	}
	id, err := s.prop(nm, n.activeConn, nm+".Connection.Active", "Id")
	if err != nil {
		return n
	}
	n.Kind, n.Name = Other, id.Value().(string)
	typ, _ := s.prop(nm, n.activeConn, nm+".Connection.Active", "Type")
	switch t, _ := typ.Value().(string); t {
	case "802-3-ethernet":
		n.Kind = Wired
	case "802-11-wireless":
		n.Kind = Wireless
		for _, ap := range n.AccessPoints {
			if ap.Active {
				n.Strength = ap.Strength
			}
		}
	}
	return n
}

// NetworkManager device type for Wi-Fi.
const nmDeviceWifi = 2

func (s *Stat) wifiDevice() dbus.ObjectPath {
	var devices []dbus.ObjectPath
	if err := s.conn.Object(nm, nmPath).Call(nm+".GetDevices", 0).Store(&devices); err != nil {
		return ""
	}
	for _, d := range devices {
		if v, err := s.prop(nm, d, nm+".Device", "DeviceType"); err == nil && v.Value() == uint32(nmDeviceWifi) {
			return d
		}
	}
	return ""
}

func (s *Stat) accessPoints(dev, active dbus.ObjectPath) []AccessPoint {
	var paths []dbus.ObjectPath
	if v, err := s.prop(nm, dev, nm+".Device.Wireless", "AccessPoints"); err == nil {
		paths, _ = v.Value().([]dbus.ObjectPath)
	}
	best := map[string]AccessPoint{}
	for _, p := range paths {
		obj := s.conn.Object(nm, p)
		var props map[string]dbus.Variant
		if err := obj.Call("org.freedesktop.DBus.Properties.GetAll", 0, nm+".AccessPoint").Store(&props); err != nil {
			continue
		}
		ssid, _ := props["Ssid"].Value().([]byte)
		if len(ssid) == 0 {
			continue // hidden network
		}
		strength, _ := props["Strength"].Value().(byte)
		flags, _ := props["Flags"].Value().(uint32)
		wpa, _ := props["WpaFlags"].Value().(uint32)
		rsn, _ := props["RsnFlags"].Value().(uint32)
		ap := AccessPoint{SSID: string(ssid), Strength: int(strength), Secure: flags&1 != 0 || wpa != 0 || rsn != 0,
			Active: p == active, path: p}
		// one entry per SSID: the active access point, else the strongest
		if cur, ok := best[ap.SSID]; !ok || ap.Active || (!cur.Active && ap.Strength > cur.Strength) {
			best[ap.SSID] = ap
		}
	}
	out := make([]AccessPoint, 0, len(best))
	for _, ap := range best {
		out = append(out, ap)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Strength != out[j].Strength {
			return out[i].Strength > out[j].Strength
		}
		return out[i].SSID < out[j].SSID
	})
	return out
}
