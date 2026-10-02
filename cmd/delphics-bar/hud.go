package main

import (
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"delphics.delphi.tools/internal/baritems"
)

// hudShow is how long a level stays in the HUD after the last change.
const hudShow = 1500

// hudFade is the crossfade between the notification line and the HUD, in milliseconds.
const hudFade = 150

// hud flashes levels (volume, brightness) in the middle of the bar, in place of the notification line.
type hud struct {
	stack *gtk.Stack
	icon  *gtk.Image
	meter *gtk.ProgressBar
	text  *gtk.Label
	// hide is the pending timer that switches back to the notification line, 0 when none
	hide glib.SourceHandle
}

// newHUD wraps middle (the notification line) in a stack with the HUD.
func newHUD(middle gtk.Widgetter) *hud {
	h := &hud{stack: gtk.NewStack(), icon: gtk.NewImage(), meter: gtk.NewProgressBar(), text: gtk.NewLabel("")}
	h.stack.SetHExpand(true)
	h.stack.SetTransitionType(gtk.StackTransitionTypeCrossfade)
	h.stack.SetTransitionDuration(hudFade)
	box := gtk.NewBox(gtk.OrientationHorizontal, 8)
	box.AddCSSClass("hud")
	box.SetHAlign(gtk.AlignCenter)
	h.icon.SetPixelSize(16)
	h.meter.SetSizeRequest(120, -1)
	h.meter.SetVAlign(gtk.AlignCenter)
	h.text.SetWidthChars(6)
	h.text.SetXAlign(0)
	box.Append(h.icon)
	box.Append(h.meter)
	box.Append(h.text)
	h.stack.AddNamed(middle, "note")
	h.stack.AddNamed(box, "hud")
	h.stack.SetVisibleChildName("note")
	return h
}

// show flashes l; GTK thread only.
func (h *hud) show(l baritems.Level) {
	h.icon.SetFromIconName(l.Icon)
	h.icon.SetVisible(l.Icon != "")
	h.meter.SetVisible(l.Value >= 0)
	h.meter.SetFraction(max(l.Value, 0))
	h.text.SetText(l.Text)
	h.stack.SetVisibleChildName("hud")
	if h.hide != 0 {
		glib.SourceRemove(h.hide)
	}
	h.hide = glib.TimeoutAdd(hudShow, func() bool {
		h.hide = 0
		h.stack.SetVisibleChildName("note")
		return false
	})
}
