package notify

import (
	"bufio"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		name    string
		hints   map[string]dbus.Variant
		timeout int32
		want    time.Duration
		urgency byte
	}{
		{"default", nil, -1, DefaultTimeout, 0},
		{"explicit", nil, 1500, 1500 * time.Millisecond, 0},
		{"never", nil, 0, 0, 0},
		{"critical stays", map[string]dbus.Variant{"urgency": dbus.MakeVariant(byte(2))}, -1, 0, Critical},
		{"critical explicit", map[string]dbus.Variant{"urgency": dbus.MakeVariant(byte(2))}, 3000, 3 * time.Second, Critical},
		{"int32 urgency", map[string]dbus.Variant{"urgency": dbus.MakeVariant(int32(2))}, -1, 0, Critical},
	} {
		n := Parse(1, "app", "s", "b", nil, tc.hints, tc.timeout)
		if n.Timeout != tc.want || n.Urgency != tc.urgency {
			t.Errorf("%s: timeout %v urgency %d, want %v %d", tc.name, n.Timeout, n.Urgency, tc.want, tc.urgency)
		}
	}
}

func TestHasAction(t *testing.T) {
	n := Notification{Actions: []string{"default", "Open", "reply", "Reply"}}
	if !n.HasAction("default") || !n.HasAction("reply") || n.HasAction("Open") {
		t.Error("HasAction must match keys only")
	}
}

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

func TestServe(t *testing.T) {
	addr := privateBus(t)
	got := make(chan Notification, 4)
	closed := make(chan uint32, 4)
	s, err := Serve(connect(t, addr), func(n Notification) { got <- n }, func(id uint32) { closed <- id })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Serve(connect(t, addr), func(Notification) {}, func(uint32) {}); err == nil {
		t.Error("second server took the name")
	}

	client := connect(t, addr)
	if err := client.AddMatchSignal(dbus.WithMatchInterface(iface)); err != nil {
		t.Fatal(err)
	}
	signals := make(chan *dbus.Signal, 4)
	client.Signal(signals)
	obj := client.Object(busName, path)
	notify := func(replaces uint32) uint32 {
		var id uint32
		err := obj.Call(iface+".Notify", 0, "test", replaces, "", "hello", "world",
			[]string{"default", "Open"}, map[string]dbus.Variant{}, int32(-1)).Store(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}

	id := notify(0)
	n := <-got
	if n.ID != id || n.Summary != "hello" || n.Body != "world" || !n.HasAction("default") {
		t.Errorf("got %+v for id %d", n, id)
	}
	if notify(0) == id {
		t.Error("new notification reused an id")
	}
	<-got
	if r := notify(id); r != id {
		t.Errorf("replace returned %d, want %d", r, id)
	}
	<-got

	s.Invoke(id, "default")
	sig := <-signals
	if sig.Name != iface+".ActionInvoked" || sig.Body[0] != id || sig.Body[1] != "default" {
		t.Errorf("ActionInvoked signal %+v", sig)
	}
	if err := obj.Call(iface+".CloseNotification", 0, id).Err; err != nil {
		t.Fatal(err)
	}
	if c := <-closed; c != id {
		t.Errorf("onClose got %d, want %d", c, id)
	}
	sig = <-signals
	if sig.Name != iface+".NotificationClosed" || sig.Body[0] != id || sig.Body[1] != Closed {
		t.Errorf("NotificationClosed signal %+v", sig)
	}
}
