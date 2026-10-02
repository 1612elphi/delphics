package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"delphics.delphi.tools/internal/sysstat"
)

const adwaita = "/usr/share/icons/Adwaita/symbolic"

func iconExists(t *testing.T, name string) bool {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join(adwaita, "*", name+".svg"))
	return len(matches) > 0
}

func TestPluginIconsExist(t *testing.T) {
	if _, err := os.Stat(adwaita); err != nil {
		t.Skip("Adwaita icon theme not installed")
	}
	var icons []string
	for pct := 0.0; pct <= 100; pct += 1 {
		for _, b := range []sysstat.Battery{
			{Present: true, Percent: pct},
			{Present: true, Percent: pct, Charging: true},
			{Present: true, Percent: pct, Full: true},
		} {
			icons = append(icons, batteryProps(b)["icon"].(string))
		}
	}
	for s := -1; s <= 100; s++ {
		icons = append(icons, networkProps(sysstat.Network{Kind: sysstat.Wireless, Name: "x", Strength: s})["icon"].(string))
	}
	for _, k := range []sysstat.NetKind{sysstat.Offline, sysstat.Wired, sysstat.Other} {
		icons = append(icons, networkProps(sysstat.Network{Kind: k})["icon"].(string))
	}
	seen := map[string]bool{}
	for _, icon := range icons {
		if !seen[icon] && !iconExists(t, icon) {
			t.Errorf("icon %q not in Adwaita", icon)
		}
		seen[icon] = true
	}
}

func TestBatteryProps(t *testing.T) {
	for _, tc := range []struct {
		b      sysstat.Battery
		icon   string
		text   string
		urgent bool
	}{
		{sysstat.Battery{}, "", "", false},
		{sysstat.Battery{Present: true, Percent: 64}, "battery-level-60-symbolic", "64%", false},
		{sysstat.Battery{Present: true, Percent: 64, Charging: true}, "battery-level-60-charging-symbolic", "64%", false},
		{sysstat.Battery{Present: true, Percent: 100, Full: true}, "battery-level-100-charged-symbolic", "100%", false},
		{sysstat.Battery{Present: true, Percent: 9}, "battery-level-0-symbolic", "9%", true},
		{sysstat.Battery{Present: true, Percent: 9, Charging: true}, "battery-level-0-charging-symbolic", "9%", false},
	} {
		p := batteryProps(tc.b)
		if p["icon"] != tc.icon || p["text"] != tc.text || p["urgent"] != tc.urgent {
			t.Errorf("%+v: got %v", tc.b, p)
		}
	}
}

func TestNetworkAndClockProps(t *testing.T) {
	p := networkProps(sysstat.Network{Kind: sysstat.Wireless, Name: "HomeNet", Strength: 72})
	if p["icon"] != "network-wireless-signal-good-symbolic" || p["text"] != "HomeNet" || p["tooltip"] != "HomeNet, 72%" {
		t.Errorf("wifi: %v", p)
	}
	if p := networkProps(sysstat.Network{Kind: sysstat.Wired, Name: "Wired 1"}); p["text"] != "" || p["tooltip"] != "Wired 1" {
		t.Errorf("wired shows an icon only: %v", p)
	}
	c := clockProps(time.Date(2026, 10, 3, 9, 5, 0, 0, time.UTC))
	if c["text"] != "09:05" || c["tooltip"] != "Saturday, 3 October 2026" || c["bold"] != true {
		t.Errorf("clock: %v", c)
	}
}
