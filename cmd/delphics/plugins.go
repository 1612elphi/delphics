package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
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

// statusPoll is how often network and battery are read.
// ponytail: polls; switch to PropertiesChanged signals if the latency matters
const statusPoll = 5 * time.Second

// plugin runs one built-in bar plugin until killed.
func plugin(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: delphics plugin clock|network|battery")
		return 2
	}
	var update func() baritems.Props
	var next func() time.Duration
	switch args[0] {
	case "clock":
		update = func() baritems.Props { return clockProps(time.Now()) }
		// wake just after the minute turns
		next = func() time.Duration {
			return time.Until(time.Now().Truncate(time.Minute).Add(time.Minute)) + 50*time.Millisecond
		}
	case "network", "battery":
		stat, err := sysstat.New()
		if err != nil {
			log.Printf("system bus: %v", err)
			return 1
		}
		if args[0] == "network" {
			update = func() baritems.Props { return networkProps(stat.Network()) }
		} else {
			update = func() baritems.Props { return batteryProps(stat.Battery()) }
		}
		next = func() time.Duration { return statusPoll }
	default:
		fmt.Fprintf(os.Stderr, "unknown plugin %q; known: clock, network, battery\n", args[0])
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
	stop := stopSignals()
	for {
		if err := item.Set(update()); err != nil {
			log.Printf("bar: %v", err)
		}
		select {
		case <-time.After(next()):
		case <-stop:
			return 0
		}
	}
}

func stopSignals() <-chan os.Signal {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	return stop
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
