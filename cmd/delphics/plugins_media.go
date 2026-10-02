package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"

	"delphics.delphi.tools/internal/audio"
	"delphics.delphi.tools/internal/backlight"
	"delphics.delphi.tools/internal/baritems"
)

const (
	orderBrightness = 80
	orderVolume     = 85
)

// scrollStep is the volume or brightness change per wheel notch.
const scrollStep = 0.05

// brightnessPoll: the kernel announces only firmware-driven backlight changes, not writes from
// brightnessctl or logind, so the level is read from sysfs this often.
const brightnessPoll = time.Second

func runVolume(session *dbus.Conn, item *baritems.Client, stop <-chan os.Signal) error {
	var cur audio.State
	refresh := func() {
		cur = audio.Read()
		p := volumeProps(cur)
		p["menu"] = volumeMenu(cur)
		set(item, p)
	}
	updates := make(chan struct{}, 1)
	audio.Watch(func() {
		select {
		case updates <- struct{}{}:
		default:
		}
	})
	refresh()
	fail := func(err error) {
		if err != nil {
			notifyError(session, "Volume", err)
		}
	}
	for {
		select {
		case <-updates:
			refresh()
		case c := <-item.Changes:
			input := c.Entry == "in"
			fail(audio.SetVolume(input, c.Value))
			// dragging the slider of a muted device unmutes it
			if (input && cur.InputMuted) || (!input && cur.Muted) {
				fail(audio.ToggleMute(input))
				if input {
					cur.InputMuted = false
				} else {
					cur.Muted = false
				}
			}
		case d := <-item.Scrolls:
			cur.Volume = min(max(cur.Volume-d*scrollStep, 0), 1)
			fail(audio.SetVolume(false, cur.Volume))
		case button := <-item.Clicks:
			if button == 2 {
				fail(audio.ToggleMute(false))
			}
		case entry := <-item.Activations:
			switch {
			case entry == "mute":
				fail(audio.ToggleMute(false))
			case entry == "micmute":
				fail(audio.ToggleMute(true))
			case strings.HasPrefix(entry, "dev:"):
				id, _ := strconv.Atoi(strings.TrimPrefix(entry, "dev:"))
				fail(audio.SetDefault(id))
			}
		case <-stop:
			return nil
		}
	}
}

func volumeIcon(s audio.State) string {
	switch {
	case s.Muted || s.Volume == 0:
		return "audio-volume-muted-symbolic"
	case s.Volume < 0.34:
		return "audio-volume-low-symbolic"
	case s.Volume < 0.67:
		return "audio-volume-medium-symbolic"
	case s.Volume <= 1:
		return "audio-volume-high-symbolic"
	}
	return "audio-volume-overamplified-symbolic"
}

// volumeProps is icon-only; the level is in the tooltip and the menu.
func volumeProps(s audio.State) baritems.Props {
	p := baritems.Props{"order": int32(orderVolume), "text": "", "icon": "", "tooltip": ""}
	if !s.HasOutput {
		return p
	}
	p["icon"] = volumeIcon(s)
	p["tooltip"] = fmt.Sprintf("Volume %.0f%%", s.Volume*100)
	if s.Muted {
		p["tooltip"] = "Muted"
	}
	return p
}

func volumeMenu(s audio.State) []baritems.MenuEntry {
	m := []baritems.MenuEntry{
		{ID: "out", Label: "Output", Slider: true, Value: s.Volume},
		{ID: "mute", Label: "Mute", Checked: s.Muted},
	}
	if s.HasInput {
		m = append(m,
			baritems.MenuEntry{ID: "in", Label: "Microphone", Slider: true, Value: s.InputVolume, Section: true},
			baritems.MenuEntry{ID: "micmute", Label: "Mute microphone", Checked: s.InputMuted})
	}
	// device choice only when there is a choice
	for _, group := range [][]audio.Device{s.Outputs, s.Inputs} {
		if len(group) < 2 {
			continue
		}
		for i, d := range group {
			m = append(m, baritems.MenuEntry{ID: fmt.Sprintf("dev:%d", d.ID), Label: d.Description, Checked: d.Default, Section: i == 0})
		}
	}
	return m
}

func runBrightness(session *dbus.Conn, item *baritems.Client, stop <-chan os.Signal) error {
	bl, err := backlight.Find()
	if err != nil {
		return err
	}
	if bl == nil {
		// no backlight: nothing to show, but stay up so systemd does not restart the unit forever
		<-stop
		return nil
	}
	level := -1.0
	refresh := func() {
		l, err := bl.Level()
		if err != nil || abs(l-level) < 0.001 {
			return
		}
		level = l
		set(item, brightnessProps(level))
	}
	setLevel := func(l float64) {
		if err := bl.Set(l); err != nil {
			notifyError(session, "Brightness", err)
		}
		refresh()
	}
	refresh()
	tick := time.NewTicker(brightnessPoll)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			refresh()
		case c := <-item.Changes:
			setLevel(c.Value)
		case d := <-item.Scrolls:
			setLevel(min(max(level-d*scrollStep, 0), 1))
		case <-item.Clicks:
		case <-item.Activations:
		case <-stop:
			return nil
		}
	}
}

func brightnessProps(level float64) baritems.Props {
	return baritems.Props{
		"order": int32(orderBrightness), "icon": "display-brightness-symbolic", "text": "",
		"tooltip": fmt.Sprintf("Brightness %.0f%%", level*100),
		"menu":    []baritems.MenuEntry{{ID: "level", Label: "Brightness", Slider: true, Value: level}},
	}
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
