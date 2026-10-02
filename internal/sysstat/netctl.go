package sysstat

import (
	"errors"

	"github.com/godbus/dbus/v5"
)

// ErrNeedsSecret means the Wi-Fi network is secured and has no saved connection, so connecting
// needs a password prompt.
var ErrNeedsSecret = errors.New("network needs a password")

func (s *Stat) SetWifiEnabled(on bool) error {
	return s.conn.Object(nm, nmPath).SetProperty(nm+".WirelessEnabled", dbus.MakeVariant(on))
}

// RequestScan asks the Wi-Fi device for fresh scan results; NetworkManager rate-limits it.
func (s *Stat) RequestScan(n Network) error {
	if n.wifiDev == "" {
		return nil
	}
	return s.conn.Object(nm, n.wifiDev).Call(nm+".Device.Wireless.RequestScan", 0, map[string]dbus.Variant{}).Err
}

// ConnectWifi activates a saved connection for ssid, or connects to an open network directly.
// A secured network without a saved connection returns ErrNeedsSecret.
func (s *Stat) ConnectWifi(n Network, ssid string) error {
	if saved := s.savedWifi(ssid); saved != "" {
		return s.conn.Object(nm, nmPath).Call(nm+".ActivateConnection", 0, saved, n.wifiDev, dbus.ObjectPath("/")).Err
	}
	for _, ap := range n.AccessPoints {
		if ap.SSID != ssid {
			continue
		}
		if ap.Secure {
			return ErrNeedsSecret
		}
		return s.conn.Object(nm, nmPath).Call(nm+".AddAndActivateConnection", 0,
			map[string]map[string]dbus.Variant{}, n.wifiDev, ap.path).Err
	}
	return errors.New("network " + ssid + " is out of range")
}

// DisconnectWifi disconnects the Wi-Fi device until the user connects again.
func (s *Stat) DisconnectWifi(n Network) error {
	if n.wifiDev == "" {
		return nil
	}
	return s.conn.Object(nm, n.wifiDev).Call(nm+".Device.Disconnect", 0).Err
}

// savedWifi returns the saved connection for ssid, or "".
func (s *Stat) savedWifi(ssid string) dbus.ObjectPath {
	const settings = "/org/freedesktop/NetworkManager/Settings"
	var conns []dbus.ObjectPath
	if err := s.conn.Object(nm, settings).Call(nm+".Settings.ListConnections", 0).Store(&conns); err != nil {
		return ""
	}
	for _, c := range conns {
		var cfg map[string]map[string]dbus.Variant
		if err := s.conn.Object(nm, c).Call(nm+".Settings.Connection.GetSettings", 0).Store(&cfg); err != nil {
			continue
		}
		if raw, ok := cfg["802-11-wireless"]["ssid"].Value().([]byte); ok && string(raw) == ssid {
			return c
		}
	}
	return ""
}
