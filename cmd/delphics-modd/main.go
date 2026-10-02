// delphics-modd reads keyboards and publishes which modifier keys are held. Every other key code is
// discarded where it is read, so no client can use this daemon as a keylogger.
//
// Protocol: connect to the unix socket; the daemon sends one byte with the current mask, then one
// byte per change. Bits: 1 super, 2 shift, 4 ctrl, 8 alt.
package main

import (
	"encoding/binary"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const socketPath = "/run/delphics-modd/modd.sock"

const (
	Super byte = 1 << iota
	Shift
	Ctrl
	Alt
)

// linux/input-event-codes.h
var modBits = map[uint16]byte{
	29: Ctrl, 97: Ctrl, // KEY_LEFTCTRL, KEY_RIGHTCTRL
	42: Shift, 54: Shift, // KEY_LEFTSHIFT, KEY_RIGHTSHIFT
	56: Alt, 100: Alt, // KEY_LEFTALT, KEY_RIGHTALT
	125: Super, 126: Super, // KEY_LEFTMETA, KEY_RIGHTMETA
}

// tracker keeps held modifiers per device, so a keyboard unplugged mid-press does not leave a key stuck.
type tracker struct {
	held map[string]map[uint16]bool
}

func newTracker() *tracker { return &tracker{held: map[string]map[uint16]bool{}} }

func (t *tracker) key(dev string, code uint16, pressed bool) {
	if t.held[dev] == nil {
		t.held[dev] = map[uint16]bool{}
	}
	if pressed {
		t.held[dev][code] = true
	} else {
		delete(t.held[dev], code)
	}
}

func (t *tracker) drop(dev string) { delete(t.held, dev) }

func (t *tracker) mask() byte {
	var m byte
	for _, codes := range t.held {
		for c := range codes {
			m |= modBits[c]
		}
	}
	return m
}

type keyEvent struct {
	dev     string
	code    uint16
	pressed bool
	gone    bool
}

// struct input_event on 64-bit Linux: timeval (16 bytes), type u16, code u16, value s32
const eventSize = 24

const (
	evKey  = 0x01
	keyMax = 0x2ff
)

// EVIOCGBIT(EV_KEY, len) from linux/input.h
func eviocgbitKey(n int) uintptr { return uintptr(2<<30 | n<<16 | 'E'<<8 | (0x20 + evKey)) }

func isKeyboard(fd int) bool {
	bits := make([]byte, keyMax/8+1)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), eviocgbitKey(len(bits)), uintptr(unsafe.Pointer(&bits[0])))
	if errno != 0 {
		return false
	}
	has := func(code int) bool { return bits[code/8]&(1<<(code%8)) != 0 }
	return has(125) && has(30) // KEY_LEFTMETA and KEY_A: excludes power buttons, lid switches, mice
}

func readDevice(path string, events chan<- keyEvent) {
	defer func() { events <- keyEvent{dev: path, gone: true} }()
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	// non-keyboards are closed and re-checked on every rescan; that keeps a keyboard that later
	// reuses the same eventN path from being skipped
	if !isKeyboard(int(f.Fd())) {
		return
	}
	buf := make([]byte, eventSize*64)
	for {
		n, err := f.Read(buf)
		if err != nil {
			return
		}
		for off := 0; off+eventSize <= n; off += eventSize {
			ev := buf[off : off+eventSize]
			typ := binary.LittleEndian.Uint16(ev[16:])
			code := binary.LittleEndian.Uint16(ev[18:])
			value := int32(binary.LittleEndian.Uint32(ev[20:]))
			if typ != evKey || modBits[code] == 0 || value == 2 { // 2 = autorepeat
				continue
			}
			events <- keyEvent{dev: path, code: code, pressed: value == 1}
		}
	}
}

func main() {
	events := make(chan keyEvent, 64)
	var mu sync.Mutex
	open := map[string]bool{}

	// ponytail: rescans /dev/input every 3 s for hotplugged keyboards; switch to udev/inotify if the delay matters
	go func() {
		for {
			paths, _ := filepath.Glob("/dev/input/event*")
			mu.Lock()
			for _, p := range paths {
				if !open[p] {
					open[p] = true
					go readDevice(p, events)
				}
			}
			mu.Unlock()
			time.Sleep(3 * time.Second)
		}
	}()

	os.Remove(socketPath)
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.Chmod(socketPath, 0o666); err != nil {
		log.Fatal(err)
	}

	clients := map[net.Conn]bool{}
	var current byte
	conns := make(chan net.Conn)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				log.Fatal(err)
			}
			conns <- c
		}
	}()

	t := newTracker()
	send := func(c net.Conn, m byte) {
		c.SetWriteDeadline(time.Now().Add(time.Second))
		if _, err := c.Write([]byte{m}); err != nil {
			c.Close()
			delete(clients, c)
		}
	}
	for {
		select {
		case c := <-conns:
			clients[c] = true
			send(c, current)
		case ev := <-events:
			if ev.gone {
				t.drop(ev.dev)
				mu.Lock()
				delete(open, ev.dev)
				mu.Unlock()
			} else {
				t.key(ev.dev, ev.code, ev.pressed)
			}
			if m := t.mask(); m != current {
				current = m
				for c := range clients {
					send(c, m)
				}
			}
		}
	}
}
