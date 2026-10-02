package main

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"

	"delphics.delphi.tools/internal/baritems"
	"delphics.delphi.tools/internal/sysstat"
)

const (
	orderBluetooth = 90
	orderWWAN      = 95
)

func runBluetooth(session *dbus.Conn, item *baritems.Client, stop <-chan os.Signal) error {
	stat, err := sysstat.New()
	if err != nil {
		return err
	}
	var mu sync.Mutex
	var cur sysstat.Bluetooth
	refresh := func() {
		b := stat.Bluetooth()
		mu.Lock()
		cur = b
		mu.Unlock()
		p := bluetoothProps(b)
		p["menu"] = bluetoothMenu(b)
		set(item, p)
	}
	refresh()
	if err := stat.WatchBluetooth(refresh); err != nil {
		return err
	}
	for {
		select {
		case entry := <-item.Activations:
			mu.Lock()
			b := cur
			mu.Unlock()
			// connecting takes seconds; the menu follows through BlueZ's signals meanwhile
			go func() {
				var err error
				switch {
				case entry == "power":
					err = stat.SetBluetoothPowered(b, !b.Powered)
				case entry == "settings":
					err = spawn("wezterm", "start", "--", "bluetoothctl")
				case strings.HasPrefix(entry, "dev:"):
					for _, d := range b.Devices {
						if d.Key == strings.TrimPrefix(entry, "dev:") {
							err = stat.ToggleBluetoothDevice(d)
						}
					}
				}
				if err != nil {
					notifyError(session, "Bluetooth", err)
				}
			}()
		case <-item.Clicks:
		case <-stop:
			return nil
		}
	}
}

func bluetoothProps(b sysstat.Bluetooth) baritems.Props {
	p := baritems.Props{"order": int32(orderBluetooth), "text": "", "icon": "", "tooltip": ""}
	if !b.HasAdapter {
		return p
	}
	if !b.Powered {
		p["icon"], p["tooltip"] = "bluetooth-disabled-symbolic", "Bluetooth off"
		return p
	}
	p["icon"], p["tooltip"] = "bluetooth-active-symbolic", "Bluetooth on"
	var connected []string
	for _, d := range b.Devices {
		if d.Connected {
			connected = append(connected, d.Name)
		}
	}
	switch len(connected) {
	case 0:
	case 1:
		p["text"] = connected[0]
	default:
		p["text"] = fmt.Sprintf("%d devices", len(connected))
	}
	if len(connected) > 0 {
		p["tooltip"] = "Connected to " + strings.Join(connected, ", ")
	}
	return p
}

func bluetoothMenu(b sysstat.Bluetooth) []baritems.MenuEntry {
	m := []baritems.MenuEntry{{ID: "power", Label: "Bluetooth", Checked: b.Powered}}
	if b.Powered {
		if len(b.Devices) == 0 {
			m = append(m, baritems.MenuEntry{Label: "No paired devices", Section: true})
		}
		for i, d := range b.Devices {
			m = append(m, baritems.MenuEntry{ID: "dev:" + d.Key, Label: d.Name, Checked: d.Connected, Section: i == 0})
		}
	}
	return append(m, baritems.MenuEntry{ID: "settings", Label: "Bluetooth settings…", Section: true})
}

func runWWAN(session *dbus.Conn, item *baritems.Client, stop <-chan os.Signal) error {
	stat, err := sysstat.New()
	if err != nil {
		return err
	}
	var mu sync.Mutex
	var cur sysstat.Modem
	refresh := func() {
		m := stat.Modem()
		mu.Lock()
		cur = m
		mu.Unlock()
		p := wwanProps(m)
		p["menu"] = wwanMenu(m)
		set(item, p)
	}
	refresh()
	if err := stat.WatchModem(refresh); err != nil {
		return err
	}
	for {
		select {
		case entry := <-item.Activations:
			mu.Lock()
			m := cur
			mu.Unlock()
			go func() {
				var err error
				switch entry {
				case "data":
					err = stat.SetMobileData(m, !m.DataOn)
				case "modem":
					err = stat.SetModemEnabled(m, m.State == sysstat.ModemOff)
				case "settings":
					err = spawn("wezterm", "start", "--", "nmtui")
				}
				if err != nil {
					notifyError(session, "Mobile broadband", err)
				}
			}()
		case <-item.Clicks:
		case <-stop:
			return nil
		}
	}
}

func wwanIcon(m sysstat.Modem) string {
	switch m.State {
	case sysstat.ModemFailed, sysstat.ModemLocked:
		return "network-cellular-no-route-symbolic"
	case sysstat.ModemOff:
		return "network-cellular-disabled-symbolic"
	case sysstat.ModemSearching:
		return "network-cellular-acquiring-symbolic"
	}
	switch {
	case m.Signal < 20:
		return "network-cellular-signal-none-symbolic"
	case m.Signal < 40:
		return "network-cellular-signal-weak-symbolic"
	case m.Signal < 50:
		return "network-cellular-signal-ok-symbolic"
	case m.Signal < 80:
		return "network-cellular-signal-good-symbolic"
	}
	return "network-cellular-signal-excellent-symbolic"
}

func wwanStatus(m sysstat.Modem) string {
	switch m.State {
	case sysstat.ModemFailed:
		return "Modem unavailable (no SIM?)"
	case sysstat.ModemLocked:
		return "SIM locked"
	case sysstat.ModemOff:
		return "Modem off"
	case sysstat.ModemSearching:
		return "Searching for a network"
	}
	parts := []string{}
	for _, s := range []string{m.Operator, m.Tech, fmt.Sprintf("%d%%", m.Signal)} {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " · ")
}

// wwanProps shows the signal icon, and the access technology while mobile data is up.
func wwanProps(m sysstat.Modem) baritems.Props {
	p := baritems.Props{"order": int32(orderWWAN), "text": "", "icon": "", "tooltip": ""}
	if m.State == sysstat.ModemNone {
		return p
	}
	p["icon"], p["tooltip"] = wwanIcon(m), wwanStatus(m)
	if m.DataOn {
		p["text"] = m.Tech
	}
	return p
}

func wwanMenu(m sysstat.Modem) []baritems.MenuEntry {
	return []baritems.MenuEntry{
		{Label: wwanStatus(m)},
		{ID: "data", Label: "Mobile data", Checked: m.DataOn, Disabled: m.State != sysstat.ModemRegistered, Section: true},
		{ID: "modem", Label: "Modem", Checked: m.State >= sysstat.ModemSearching, Disabled: m.State == sysstat.ModemFailed || m.State == sysstat.ModemLocked},
		{ID: "settings", Label: "Mobile settings…", Section: true},
	}
}
