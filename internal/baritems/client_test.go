package baritems

import (
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
