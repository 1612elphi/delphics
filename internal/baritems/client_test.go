package baritems

import (
	"reflect"
	"testing"
	"time"
)

func TestClientSurvivesBarRestarts(t *testing.T) {
	addr := privateBus(t)
	c, err := NewClient(connect(t, addr), "clock")
	if err != nil {
		t.Fatal(err)
	}
	// no bar yet: not an error, the item waits
	if err := c.Set(Props{"text": "12:00", "bold": true}); err != nil {
		t.Fatalf("set without bar: %v", err)
	}

	wait := func(changes chan []Item, want string) {
		t.Helper()
		deadline := time.After(2 * time.Second)
		for {
			select {
			case items := <-changes:
				if len(items) == 1 && items[0].Text == want && items[0].Bold {
					return
				}
			case <-deadline:
				t.Fatalf("item %q never arrived", want)
			}
		}
	}

	changes := make(chan []Item, 16)
	barConn := connect(t, addr)
	srv, err := Serve(barConn, func(items []Item) { changes <- items })
	if err != nil {
		t.Fatal(err)
	}
	wait(changes, "12:00")

	srv.Click("clock", 1)
	srv.Click("other", 2)
	select {
	case b := <-c.Clicks:
		if b != 1 {
			t.Errorf("click button %d, want 1", b)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no click")
	}
	select {
	case b := <-c.Clicks:
		t.Errorf("got click %d meant for another item", b)
	case <-time.After(100 * time.Millisecond):
	}

	// menus round-trip, and activations reach the client
	menu := []MenuEntry{{Label: "status"}, {ID: "wifi", Label: "Wi-Fi", Checked: true, Section: true}, {ID: "off", Label: "Off", Disabled: true}}
	if err := c.Set(Props{"menu": menu}); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(2 * time.Second)
	for got := false; !got; {
		select {
		case items := <-changes:
			if len(items) == 1 && len(items[0].Menu) == 3 {
				if !reflect.DeepEqual(items[0].Menu, menu) {
					t.Fatalf("menu = %+v, want %+v", items[0].Menu, menu)
				}
				got = true
			}
		case <-deadline:
			t.Fatal("menu never arrived")
		}
	}
	srv.Activate("clock", "wifi")
	select {
	case e := <-c.Activations:
		if e != "wifi" {
			t.Errorf("activation %q, want wifi", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no activation")
	}

	// sliders: the last of a burst always arrives; scrolls arrive
	if err := c.Set(Props{"menu": []MenuEntry{{ID: "vol", Label: "Output", Slider: true, Value: 0.4}}}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 20; i++ {
		srv.Change("clock", "vol", float64(i)/20)
	}
	srv.Scroll("clock", -1)
	var last Change
	timeout := time.After(2 * time.Second)
	for last.Value != 1 {
		select {
		case last = <-c.Changes:
		case <-timeout:
			t.Fatalf("last slider value never arrived, got %+v", last)
		}
	}
	if last.Entry != "vol" {
		t.Errorf("change entry %q", last.Entry)
	}
	select {
	case d := <-c.Scrolls:
		if d != -1 {
			t.Errorf("scroll %v", d)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no scroll")
	}

	// bar restarts: the client sends its merged properties again
	if err := c.Set(Props{"text": "12:01"}); err != nil {
		t.Fatal(err)
	}
	barConn.Close()
	changes2 := make(chan []Item, 16)
	if _, err := Serve(connect(t, addr), func(items []Item) { changes2 <- items }); err != nil {
		t.Fatal(err)
	}
	wait(changes2, "12:01")
}
