package sysstat

import (
	"path"
	"sort"

	"github.com/godbus/dbus/v5"
)

const bluez = "org.bluez"

type BTDevice struct {
	// Key identifies the device across reads, e.g. "dev_AA_BB_CC_DD_EE_FF"
	Key       string
	Name      string
	Connected bool
	path      dbus.ObjectPath
}

type Bluetooth struct {
	// HasAdapter is false without a Bluetooth controller or without bluetoothd
	HasAdapter bool
	Powered    bool
	// Devices are the paired devices, connected first, then by name
	Devices []BTDevice
	adapter dbus.ObjectPath
}

// Bluetooth reads the first adapter and its paired devices from BlueZ.
func (s *Stat) Bluetooth() Bluetooth {
	var objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	if err := s.conn.Object(bluez, "/").Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objects); err != nil {
		return Bluetooth{}
	}
	return parseBluetooth(objects)
}

func parseBluetooth(objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant) Bluetooth {
	var b Bluetooth
	paths := make([]dbus.ObjectPath, 0, len(objects))
	for p := range objects {
		paths = append(paths, p)
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i] < paths[j] })
	for _, p := range paths {
		if a, ok := objects[p]["org.bluez.Adapter1"]; ok && !b.HasAdapter {
			b.HasAdapter, b.adapter = true, p
			b.Powered, _ = a["Powered"].Value().(bool)
		}
	}
	for _, p := range paths {
		d, ok := objects[p]["org.bluez.Device1"]
		if !ok {
			continue
		}
		if adapter, _ := d["Adapter"].Value().(dbus.ObjectPath); adapter != b.adapter {
			continue
		}
		if paired, _ := d["Paired"].Value().(bool); !paired {
			continue
		}
		dev := BTDevice{Key: path.Base(string(p)), path: p}
		dev.Name, _ = d["Alias"].Value().(string)
		dev.Connected, _ = d["Connected"].Value().(bool)
		b.Devices = append(b.Devices, dev)
	}
	sort.SliceStable(b.Devices, func(i, j int) bool {
		if b.Devices[i].Connected != b.Devices[j].Connected {
			return b.Devices[i].Connected
		}
		return b.Devices[i].Name < b.Devices[j].Name
	})
	return b
}

func (s *Stat) SetBluetoothPowered(b Bluetooth, on bool) error {
	return s.conn.Object(bluez, b.adapter).SetProperty("org.bluez.Adapter1.Powered", dbus.MakeVariant(on))
}

// ToggleBluetoothDevice connects a paired device, or disconnects it when connected. Connecting can
// take seconds; it blocks until BlueZ answers.
func (s *Stat) ToggleBluetoothDevice(d BTDevice) error {
	method := "org.bluez.Device1.Connect"
	if d.Connected {
		method = "org.bluez.Device1.Disconnect"
	}
	return s.conn.Object(bluez, d.path).Call(method, 0).Err
}

// WatchBluetooth calls onChange after adapters or devices change, appear or go away, and when
// bluetoothd restarts. Device signal strength updates during discovery are folded in by the debounce.
func (s *Stat) WatchBluetooth(onChange func()) error {
	if err := s.conn.AddMatchSignal(dbus.WithMatchSender(bluez), dbus.WithMatchInterface("org.freedesktop.DBus.ObjectManager")); err != nil {
		return err
	}
	return s.watch([]string{bluez}, func(*dbus.Signal) bool { return true }, onChange)
}
