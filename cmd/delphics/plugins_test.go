package main

import (
	"os"
	"path/filepath"
	"strings"
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

func TestMenus(t *testing.T) {
	n := sysstat.Network{Kind: sysstat.Wireless, Name: "Home", Strength: 70, HasWifi: true, WifiEnabled: true,
		AccessPoints: []sysstat.AccessPoint{{SSID: "Home", Strength: 70, Active: true}, {SSID: "Cafe", Strength: 40, Secure: true}}}
	m := networkMenu(n)
	labels := []string{}
	for _, e := range m {
		labels = append(labels, e.ID+"="+e.Label)
	}
	want := "=Connected to Home|wifi=Wi-Fi|ap:Home=Home  70%|ap:Cafe=Cafe  40%|disconnect=Disconnect|settings=Network settings…"
	if got := strings.Join(labels, "|"); got != want {
		t.Errorf("network menu\n got %s\nwant %s", got, want)
	}
	if !m[1].Checked || !m[2].Checked || m[3].Checked || !m[1].Section || !m[5].Section {
		t.Errorf("checks/sections wrong: %+v", m)
	}
	// Wi-Fi off: no network list, no disconnect
	n.WifiEnabled, n.Kind = false, sysstat.Offline
	if m := networkMenu(n); len(m) != 3 || m[1].Checked {
		t.Errorf("wifi off menu: %+v", m)
	}

	b := batteryMenu(sysstat.Battery{Present: true, Percent: 64, Seconds: 7800}, "balanced", []string{"power-saver", "balanced", "performance"})
	if b[0].Label != "64%, 2 h 10 min left" || b[2].Label != "Balanced" || !b[2].Checked || b[1].Checked || !b[1].Section {
		t.Errorf("battery menu: %+v", b)
	}
	if b := batteryMenu(sysstat.Battery{Present: true, Percent: 97, Charging: true, Seconds: 958}, "", nil); b[0].Label != "97%, full in 16 min" {
		t.Errorf("charging status: %q", b[0].Label)
	}
}
