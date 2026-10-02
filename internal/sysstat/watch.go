package sysstat

import (
	"time"

	"github.com/godbus/dbus/v5"
)

// debounce collects bursts of signals (NetworkManager sends several per state change) into one read.
const debounce = 250 * time.Millisecond

// WatchBattery calls onChange after UPower's display device or the power profile changes, and when
// either service restarts.
func (s *Stat) WatchBattery(onChange func()) error {
	return s.watch([]string{upower, profiles}, func(sig *dbus.Signal) bool {
		return sig.Path == display || sig.Path == ppPath
	}, onChange)
}

// WatchNetwork calls onChange after NetworkManager's state, the primary connection, the Wi-Fi device
// or its active access point changes, and when NetworkManager restarts. current returns the paths to
// follow; signal strength changes of other access points are ignored, they only matter for the menu,
// which is refreshed by the device's scan results.
func (s *Stat) WatchNetwork(current func() Network, onChange func()) error {
	return s.watch([]string{nm}, func(sig *dbus.Signal) bool {
		n := current()
		switch sig.Path {
		case nmPath, n.wifiDev:
			return true
		case "":
			return false
		}
		return sig.Path == n.activeConn || sig.Path == n.activeAP
	}, onChange)
}

func (s *Stat) watch(services []string, relevant func(*dbus.Signal) bool, onChange func()) error {
	for _, svc := range services {
		if err := s.conn.AddMatchSignal(dbus.WithMatchSender(svc), dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
			dbus.WithMatchMember("PropertiesChanged")); err != nil {
			return err
		}
		if err := s.conn.AddMatchSignal(dbus.WithMatchInterface("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged"),
			dbus.WithMatchArg(0, svc)); err != nil {
			return err
		}
	}
	signals := make(chan *dbus.Signal, 64)
	s.conn.Signal(signals)
	go func() {
		var timer *time.Timer
		for sig := range signals {
			if sig.Name != "org.freedesktop.DBus.NameOwnerChanged" && !relevant(sig) {
				continue
			}
			if timer == nil {
				timer = time.AfterFunc(debounce, onChange)
			} else {
				timer.Reset(debounce)
			}
		}
	}()
	return nil
}
