package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"

	"delphics.delphi.tools/internal/baritems"
	"delphics.delphi.tools/internal/sysstat"
)

// Built-in plugins sit right of third-party items (order 0 by default), clock last.
const (
	orderNetwork = 100
	orderBattery = 110
	orderClock   = 120
)

const pluginNames = "clock|network|battery|volume|brightness|bluetooth|wwan"

// plugin runs one built-in bar plugin until killed.
func plugin(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: delphics plugin "+pluginNames)
		return 2
	}
	run := map[string]func(*dbus.Conn, *baritems.Client, <-chan os.Signal) error{
		"clock": runClock, "network": runNetwork, "battery": runBattery,
		"volume": runVolume, "brightness": runBrightness, "bluetooth": runBluetooth, "wwan": runWWAN,
	}[args[0]]
	if run == nil {
		fmt.Fprintf(os.Stderr, "unknown plugin %q; known: %s\n", args[0], pluginNames)
		return 2
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		log.Printf("session bus: %v", err)
		return 1
	}
	defer conn.Close()
	item, err := baritems.NewClient(conn, args[0])
	if err != nil {
		log.Printf("bar: %v", err)
		return 1
	}
	if err := run(conn, item, stopSignals()); err != nil {
		log.Print(err)
		return 1
	}
	return 0
}

func stopSignals() <-chan os.Signal {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	return stop
}

func set(item *baritems.Client, p baritems.Props) {
	if err := item.Set(p); err != nil {
		log.Printf("bar: %v", err)
	}
}

func runClock(_ *dbus.Conn, item *baritems.Client, stop <-chan os.Signal) error {
	for {
		set(item, clockProps(time.Now()))
		// wake just after the minute turns
		select {
		case <-time.After(time.Until(time.Now().Truncate(time.Minute).Add(time.Minute)) + 50*time.Millisecond):
		case <-stop:
			return nil
		}
	}
}

func runNetwork(session *dbus.Conn, item *baritems.Client, stop <-chan os.Signal) error {
	stat, err := sysstat.New()
	if err != nil {
		return err
	}
	var mu sync.Mutex
	var cur sysstat.Network
	current := func() sysstat.Network {
		mu.Lock()
		defer mu.Unlock()
		return cur
	}
	refresh := func() {
		n := stat.Network()
		mu.Lock()
		cur = n
		mu.Unlock()
		p := networkProps(n)
		p["menu"] = networkMenu(n)
		set(item, p)
	}
	refresh()
	if err := stat.WatchNetwork(current, refresh); err != nil {
		return err
	}
	for {
		select {
		case button := <-item.Clicks:
			if button == 1 {
				// the menu is opening: fresh scan results show up in it as they arrive
				stat.RequestScan(current())
			}
		case entry := <-item.Activations:
			n := current()
			var err error
			switch {
			case entry == "wifi":
				err = stat.SetWifiEnabled(!n.WifiEnabled)
			case entry == "disconnect":
				err = stat.DisconnectWifi(n)
			case entry == "settings":
				err = spawn("wezterm", "start", "--", "nmtui")
			case strings.HasPrefix(entry, "ap:"):
				ssid := strings.TrimPrefix(entry, "ap:")
				err = stat.ConnectWifi(n, ssid)
				if errors.Is(err, sysstat.ErrNeedsSecret) {
					err = spawn("wezterm", "start", "--", "nmtui", "connect", ssid)
				}
			}
			if err != nil {
				notifyError(session, "Network", err)
			}
		case <-stop:
			return nil
		}
	}
}

func runBattery(session *dbus.Conn, item *baritems.Client, stop <-chan os.Signal) error {
	stat, err := sysstat.New()
	if err != nil {
		return err
	}
	refresh := func() {
		b := stat.Battery()
		active, available := stat.PowerProfiles()
		p := batteryProps(b)
		p["menu"] = batteryMenu(b, active, available)
		set(item, p)
	}
	refresh()
	if err := stat.WatchBattery(refresh); err != nil {
		return err
	}
	for {
		select {
		case entry := <-item.Activations:
			if name, ok := strings.CutPrefix(entry, "profile:"); ok {
				if err := stat.SetPowerProfile(name); err != nil {
					notifyError(session, "Power profile", err)
				}
			}
		case <-item.Clicks:
		case <-stop:
			return nil
		}
	}
}

func spawn(argv ...string) error {
	cmd := exec.Command(argv[0], argv[1:]...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// notifyError shows a failed action as a desktop notification, which the bar displays.
func notifyError(session *dbus.Conn, what string, err error) {
	log.Printf("%s: %v", what, err)
	msg := err.Error()
	var dbusErr dbus.Error
	if errors.As(err, &dbusErr) && len(dbusErr.Body) > 0 {
		msg = fmt.Sprint(dbusErr.Body[0])
	}
	session.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications").Call(
		"org.freedesktop.Notifications.Notify", 0, "DELPHICS", uint32(0), "", what+" failed", msg,
		[]string{}, map[string]dbus.Variant{}, int32(-1))
}

func clockProps(t time.Time) baritems.Props {
	return baritems.Props{"text": t.Format("15:04"), "tooltip": t.Format("Monday, 2 January 2006"), "bold": true, "order": int32(orderClock)}
}

// batteryProps shows the Adwaita level icon (steps of 10) and the percentage; an empty item hides itself.
func batteryProps(b sysstat.Battery) baritems.Props {
	p := baritems.Props{"order": int32(orderBattery), "text": "", "icon": "", "tooltip": "", "urgent": false}
	if !b.Present {
		return p
	}
	level := min(int(b.Percent)/10*10, 100)
	pct := fmt.Sprintf("%.0f%%", b.Percent)
	switch {
	case b.Full:
		p["icon"], p["tooltip"] = "battery-level-100-charged-symbolic", "full"
	case b.Charging && level == 100:
		// Adwaita has no battery-level-100-charging icon
		p["icon"], p["tooltip"] = "battery-level-100-charged-symbolic", pct+", charging"
	case b.Charging:
		p["icon"], p["tooltip"] = fmt.Sprintf("battery-level-%d-charging-symbolic", level), pct+", charging"
	default:
		p["icon"], p["tooltip"] = fmt.Sprintf("battery-level-%d-symbolic", level), pct+", on battery"
		p["urgent"] = b.Percent <= 10
	}
	p["text"] = pct
	return p
}

var profileNames = map[string]string{"power-saver": "Power saver", "balanced": "Balanced", "performance": "Performance"}

// batteryMenu is a status line and the power profiles.
func batteryMenu(b sysstat.Battery, active string, available []string) []baritems.MenuEntry {
	var status string
	switch {
	case !b.Present:
		status = "No battery"
	case b.Full:
		status = "Fully charged"
	case b.Charging && b.Seconds > 0:
		status = fmt.Sprintf("%.0f%%, full in %s", b.Percent, duration(b.Seconds))
	case b.Charging:
		status = fmt.Sprintf("%.0f%%, charging", b.Percent)
	case b.Seconds > 0:
		status = fmt.Sprintf("%.0f%%, %s left", b.Percent, duration(b.Seconds))
	default:
		status = fmt.Sprintf("%.0f%% on battery", b.Percent)
	}
	menu := []baritems.MenuEntry{{Label: status}}
	for i, name := range available {
		label := profileNames[name]
		if label == "" {
			label = name
		}
		menu = append(menu, baritems.MenuEntry{ID: "profile:" + name, Label: label, Checked: name == active, Section: i == 0})
	}
	return menu
}

func duration(seconds int64) string {
	m := (seconds + 30) / 60
	if m < 60 {
		return fmt.Sprintf("%d min", m)
	}
	return fmt.Sprintf("%d h %d min", m/60, m%60)
}

// networkProps shows the connection's icon, with the name for Wi-Fi and other non-wired links.
func networkProps(n sysstat.Network) baritems.Props {
	p := baritems.Props{"order": int32(orderNetwork), "text": "", "tooltip": n.Name}
	switch n.Kind {
	case sysstat.Offline:
		p["icon"], p["tooltip"] = "network-offline-symbolic", "offline"
	case sysstat.Wired:
		p["icon"] = "network-wired-symbolic"
	case sysstat.Wireless:
		p["icon"], p["text"] = wifiIcon(n.Strength), n.Name
		if n.Strength >= 0 {
			p["tooltip"] = fmt.Sprintf("%s, %d%%", n.Name, n.Strength)
		}
	default:
		p["icon"], p["text"] = "network-vpn-symbolic", n.Name
	}
	return p
}

// maxNetworks caps the Wi-Fi list in the menu.
const maxNetworks = 8

// networkMenu is a status line, the Wi-Fi switch and visible networks, and the settings.
func networkMenu(n sysstat.Network) []baritems.MenuEntry {
	status := "Not connected"
	switch n.Kind {
	case sysstat.Wireless:
		status = "Connected to " + n.Name
	case sysstat.Wired:
		status = "Wired: " + n.Name
	case sysstat.Other:
		status = n.Name
	}
	menu := []baritems.MenuEntry{{Label: status}}
	if n.HasWifi {
		menu = append(menu, baritems.MenuEntry{ID: "wifi", Label: "Wi-Fi", Checked: n.WifiEnabled, Section: true})
		if n.WifiEnabled {
			for i, ap := range n.AccessPoints {
				if i == maxNetworks {
					break
				}
				menu = append(menu, baritems.MenuEntry{ID: "ap:" + ap.SSID, Label: fmt.Sprintf("%s  %d%%", ap.SSID, ap.Strength), Checked: ap.Active})
			}
			if n.Kind == sysstat.Wireless {
				menu = append(menu, baritems.MenuEntry{ID: "disconnect", Label: "Disconnect"})
			}
		}
	}
	return append(menu, baritems.MenuEntry{ID: "settings", Label: "Network settings…", Section: true})
}

// wifiIcon uses GNOME Shell's signal thresholds.
func wifiIcon(strength int) string {
	switch {
	case strength < 0:
		return "network-wireless-symbolic"
	case strength < 20:
		return "network-wireless-signal-none-symbolic"
	case strength < 40:
		return "network-wireless-signal-weak-symbolic"
	case strength < 50:
		return "network-wireless-signal-ok-symbolic"
	case strength < 80:
		return "network-wireless-signal-good-symbolic"
	}
	return "network-wireless-signal-excellent-symbolic"
}
