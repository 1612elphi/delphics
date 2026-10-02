package main

import "testing"

func TestTrackerMask(t *testing.T) {
	tr := newTracker()
	tr.key("kbd0", 125, true) // left super
	tr.key("kbd0", 42, true)  // left shift
	if m := tr.mask(); m != Super|Shift {
		t.Fatalf("mask = %04b, want super+shift", m)
	}
	tr.key("kbd1", 126, true) // right super on a second keyboard
	tr.key("kbd0", 125, false)
	if m := tr.mask(); m != Super|Shift {
		t.Fatalf("mask = %04b, want super still held via kbd1", m)
	}
	tr.drop("kbd1")
	if m := tr.mask(); m != Shift {
		t.Fatalf("mask = %04b, want shift only after kbd1 unplugged", m)
	}
	tr.key("kbd0", 42, false)
	if m := tr.mask(); m != 0 {
		t.Fatalf("mask = %04b, want 0", m)
	}
}

func TestEviocgbit(t *testing.T) {
	// EVIOCGBIT(EV_KEY, 96) as computed by the C macro on x86_64
	if got := eviocgbitKey(96); got != 0x80604521 {
		t.Fatalf("ioctl = %#x", got)
	}
}
