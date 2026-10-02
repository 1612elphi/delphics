package sysstat

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestParseBluetooth(t *testing.T) {
	v := dbus.MakeVariant
	objects := map[dbus.ObjectPath]map[string]map[string]dbus.Variant{
		"/org/bluez/hci0": {"org.bluez.Adapter1": {"Powered": v(true)}},
		"/org/bluez/hci0/dev_AA": {"org.bluez.Device1": {"Adapter": v(dbus.ObjectPath("/org/bluez/hci0")), "Alias": v("Mouse"),
			"Paired": v(true), "Connected": v(false)}},
		"/org/bluez/hci0/dev_BB": {"org.bluez.Device1": {"Adapter": v(dbus.ObjectPath("/org/bluez/hci0")), "Alias": v("Buds"),
			"Paired": v(true), "Connected": v(true)}},
		// seen during discovery, not paired: not listed
		"/org/bluez/hci0/dev_CC": {"org.bluez.Device1": {"Adapter": v(dbus.ObjectPath("/org/bluez/hci0")), "Alias": v("TV"),
			"Paired": v(false)}},
	}
	b := parseBluetooth(objects)
	if !b.HasAdapter || !b.Powered || len(b.Devices) != 2 {
		t.Fatalf("%+v", b)
	}
	if b.Devices[0].Name != "Buds" || !b.Devices[0].Connected || b.Devices[1].Key != "dev_AA" {
		t.Errorf("connected first, then by name: %+v", b.Devices)
	}
	if b := parseBluetooth(nil); b.HasAdapter {
		t.Error("no adapter expected")
	}
}

func TestTechName(t *testing.T) {
	if techName(1<<14) != "LTE" || techName(1<<15|1<<14) != "5G" || techName(1<<7) != "3G" || techName(0) != "" {
		t.Error("tech names")
	}
}
