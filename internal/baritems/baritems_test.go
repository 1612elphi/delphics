package baritems

import (
	"bufio"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// privateBus starts a dbus-daemon for the test and returns its address.
func privateBus(t *testing.T) string {
	bin, err := exec.LookPath("dbus-daemon")
	if err != nil {
		t.Skip("dbus-daemon not installed")
	}
	cmd := exec.Command(bin, "--session", "--nofork", "--print-address=1")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	addr, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(addr)
}

func connect(t *testing.T, addr string) *dbus.Conn {
	conn, err := dbus.Connect(addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

type props = map[string]dbus.Variant

func TestItems(t *testing.T) {
	addr := privateBus(t)
	changes := make(chan []Item, 16)
	srv, err := Serve(connect(t, addr), func(items []Item) { changes <- items })
	if err != nil {
		t.Fatal(err)
	}
	next := func() []Item {
		t.Helper()
		select {
		case items := <-changes:
			return items
		case <-time.After(2 * time.Second):
			t.Fatal("no change")
			return nil
		}
	}
	a, b := connect(t, addr), connect(t, addr)
	set := func(c *dbus.Conn, id string, p props) error {
		return c.Object(BusName, Path).Call(Iface+".Set", 0, id, p).Err
	}

	if err := set(a, "vpn", props{"text": dbus.MakeVariant("vpn up"), "order": dbus.MakeVariant(int32(5))}); err != nil {
		t.Fatal(err)
	}
	next()
	if err := set(b, "mail", props{"text": dbus.MakeVariant("3 new"), "urgent": dbus.MakeVariant(true),
		"icon": dbus.MakeVariant("mail-unread-symbolic")}); err != nil {
		t.Fatal(err)
	}
	items := next()
	if len(items) != 2 || items[0].ID != "mail" || items[1].ID != "vpn" || !items[0].Urgent || items[0].Icon != "mail-unread-symbolic" {
		t.Fatalf("items = %+v, want mail (order 0) before vpn (order 5)", items)
	}

	// properties left out keep their value
	if err := set(a, "vpn", props{"tooltip": dbus.MakeVariant("wg0")}); err != nil {
		t.Fatal(err)
	}
	if items := next(); items[1].Text != "vpn up" || items[1].Tooltip != "wg0" || items[1].Order != 5 {
		t.Fatalf("merge lost properties: %+v", items[1])
	}

	for name, err := range map[string]error{
		"other owner":  set(b, "vpn", props{"text": dbus.MakeVariant("mine")}),
		"unknown prop": set(a, "vpn", props{"colour": dbus.MakeVariant("red")}),
		"wrong type":   set(a, "vpn", props{"order": dbus.MakeVariant("first")}),
		"empty id":     set(a, "", props{}),
		"icon path":    set(a, "vpn", props{"icon": dbus.MakeVariant("/etc/passwd")}),
		"long text":    set(a, "vpn", props{"text": dbus.MakeVariant(strings.Repeat("x", maxText+1))}),
	} {
		if err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := b.Object(BusName, Path).Call(Iface+".Remove", 0, "vpn").Err; err == nil {
		t.Error("removed another connection's item")
	}

	if err := a.AddMatchSignal(dbus.WithMatchInterface(Iface)); err != nil {
		t.Fatal(err)
	}
	signals := make(chan *dbus.Signal, 4)
	a.Signal(signals)
	srv.Click("vpn", 3)
	select {
	case sig := <-signals:
		if sig.Name != Iface+".Clicked" || sig.Body[0] != "vpn" || sig.Body[1] != uint32(3) {
			t.Errorf("click signal %+v", sig)
		}
	case <-time.After(2 * time.Second):
		t.Error("no click signal")
	}

	// b's items go away with its connection
	b.Close()
	if items := next(); len(items) != 1 || items[0].ID != "vpn" {
		t.Fatalf("after disconnect: %+v", items)
	}
	if err := a.Object(BusName, Path).Call(Iface+".Remove", 0, "vpn").Err; err != nil {
		t.Fatal(err)
	}
	if items := next(); len(items) != 0 {
		t.Fatalf("after remove: %+v", items)
	}
}
