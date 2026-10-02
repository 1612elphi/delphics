// Package backlight reads the screen backlight from sysfs and sets it through logind, which needs no root.
package backlight

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/godbus/dbus/v5"
)

const sysfs = "/sys/class/backlight"

type Backlight struct {
	dir  string
	conn *dbus.Conn
}

// typeRank prefers firmware over platform over raw interfaces, like systemd-backlight.
var typeRank = map[string]int{"firmware": 0, "platform": 1, "raw": 2}

// Find returns the preferred backlight, or nil when the machine has none.
func Find() (*Backlight, error) {
	return find(sysfs)
}

func find(root string) (*Backlight, error) {
	dirs, _ := filepath.Glob(filepath.Join(root, "*"))
	best, bestRank := "", 99
	for _, d := range dirs {
		t, err := os.ReadFile(filepath.Join(d, "type"))
		if err != nil {
			continue
		}
		rank, ok := typeRank[strings.TrimSpace(string(t))]
		if !ok {
			rank = 3
		}
		if rank < bestRank {
			best, bestRank = d, rank
		}
	}
	if best == "" {
		return nil, nil
	}
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, err
	}
	return &Backlight{dir: best, conn: conn}, nil
}

func readInt(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}

// Level returns the brightness from 0 to 1.
func (b *Backlight) Level() (float64, error) {
	cur, err := readInt(filepath.Join(b.dir, "brightness"))
	if err != nil {
		return 0, err
	}
	top, err := readInt(filepath.Join(b.dir, "max_brightness"))
	if err != nil || top == 0 {
		return 0, err
	}
	return float64(cur) / float64(top), nil
}

// minLevel keeps the screen from going fully dark from the bar.
const minLevel = 0.01

// Set sets the brightness from 0 to 1 through logind's Session.SetBrightness.
func (b *Backlight) Set(level float64) error {
	top, err := readInt(filepath.Join(b.dir, "max_brightness"))
	if err != nil {
		return err
	}
	level = min(max(level, minLevel), 1)
	v := uint32(level*float64(top) + 0.5)
	return b.conn.Object("org.freedesktop.login1", "/org/freedesktop/login1/session/auto").Call(
		"org.freedesktop.login1.Session.SetBrightness", 0, "backlight", filepath.Base(b.dir), v).Err
}
