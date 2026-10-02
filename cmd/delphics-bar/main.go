// delphics-bar is the DELPHICS top bar: focused app and strip minimap on the left, notifications in the
// middle, system status on the right. It is also the session's notification daemon.
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/diamondburned/gotk4/pkg/cairo"
	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/godbus/dbus/v5"

	"delphics.delphi.tools/internal/hints"
	"delphics.delphi.tools/internal/layershell"
	"delphics.delphi.tools/internal/niri"
	"delphics.delphi.tools/internal/notify"
	"delphics.delphi.tools/internal/sysstat"
)

// niri layout gap from delphics-desktop/niri/config.kdl
const niriGap = 8

const minimapWidth = 160

// milliseconds Super must be held before the hint view replaces the bar; shorter taps are shortcuts
const hintDelay = 350

const moddSocket = "/run/delphics-modd/modd.sock"

// ponytail: dark DELPHICS palette hardcoded until darkman hooks switch palettes
var palette = struct{ card, fg, muted, border, primary [3]float64 }{
	card:    rgb(0x101f10),
	fg:      rgb(0xebe4d2),
	muted:   rgb(0x959074),
	border:  rgb(0x213321),
	primary: rgb(0xc2ad61),
}

const css = `
window.delphics-bar { background: #101f10; color: #ebe4d2; font-family: "Open Sans"; font-stretch: condensed; font-size: 13px; }
.delphics-bar box.bar { min-height: 28px; padding: 0 10px; }
.delphics-bar .app { font-weight: 700; }
.delphics-bar .status { color: #959074; }
.delphics-bar .clock { font-weight: 700; }
.delphics-bar .mods { color: #c2ad61; font-weight: 700; }
.delphics-bar .hints { color: #959074; }
.delphics-bar .notification.critical { color: #c2ad61; }
`

func rgb(hex int) [3]float64 {
	return [3]float64{float64(hex>>16&0xff) / 255, float64(hex>>8&0xff) / 255, float64(hex&0xff) / 255}
}

func setColor(cr *cairo.Context, c [3]float64) { cr.SetSourceRGB(c[0], c[1], c[2]) }

func main() {
	app := gtk.NewApplication("tools.delphi.Delphics.Bar", gio.ApplicationFlagsNone)
	app.ConnectActivate(func() { activate(app) })
	os.Exit(app.Run(os.Args))
}

func activate(app *gtk.Application) {
	if !layershell.Supported() {
		log.Fatal("compositor does not support wlr-layer-shell")
	}
	provider := gtk.NewCSSProvider()
	provider.LoadFromString(css)
	gtk.StyleContextAddProviderForDisplay(gdk.DisplayGetDefault(), provider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)

	win := gtk.NewApplicationWindow(app)
	win.AddCSSClass("delphics-bar")
	layershell.AnchorTop(unsafe.Pointer(coreglib.InternObject(win).Native()), "delphics-bar")

	bar := gtk.NewBox(gtk.OrientationHorizontal, 14)
	bar.AddCSSClass("bar")
	appLabel := gtk.NewLabel("")
	appLabel.AddCSSClass("app")
	minimap := gtk.NewDrawingArea()
	minimap.SetContentWidth(minimapWidth)
	minimap.SetContentHeight(28)
	noteLabel := gtk.NewLabel("")
	noteLabel.AddCSSClass("notification")
	noteLabel.SetHExpand(true)
	noteLabel.SetEllipsize(pango.EllipsizeEnd)
	dndLabel := gtk.NewLabel("dnd")
	dndLabel.AddCSSClass("status")
	dndLabel.SetVisible(false)
	netLabel := gtk.NewLabel("")
	netLabel.AddCSSClass("status")
	batLabel := gtk.NewLabel("")
	batLabel.AddCSSClass("status")
	clock := gtk.NewLabel("")
	clock.AddCSSClass("clock")
	for _, w := range []gtk.Widgetter{appLabel, minimap, noteLabel, dndLabel, netLabel, batLabel, clock} {
		bar.Append(w)
	}
	hintBox := gtk.NewBox(gtk.OrientationHorizontal, 14)
	hintBox.AddCSSClass("bar")
	modLabel := gtk.NewLabel("")
	modLabel.AddCSSClass("mods")
	hintLabel := gtk.NewLabel("")
	hintLabel.AddCSSClass("hints")
	hintLabel.SetEllipsize(pango.EllipsizeEnd)
	hintLabel.SetHExpand(true)
	hintLabel.SetXAlign(0)
	hintBox.Append(modLabel)
	hintBox.Append(hintLabel)

	stack := gtk.NewStack()
	stack.AddNamed(bar, "bar")
	stack.AddNamed(hintBox, "hints")
	stack.SetVisibleChildName("bar")
	win.SetChild(stack)

	var mu sync.Mutex
	state := niri.NewState()
	var view niri.View
	names := appNames()

	minimap.SetDrawFunc(func(_ *gtk.DrawingArea, cr *cairo.Context, width, height int) {
		mu.Lock()
		v := view
		mu.Unlock()
		drawMinimap(cr, v, float64(win.Width()), float64(width), float64(height))
	})

	go func() {
		for {
			err := niri.Stream(func(line []byte) {
				mu.Lock()
				if err := state.Apply(line); err != nil {
					log.Printf("niri event: %v", err)
				}
				view = state.View(niriGap)
				title := ""
				if view.Focused != nil {
					title = displayName(names, view.Focused.AppID)
				}
				mu.Unlock()
				glib.IdleAdd(func() {
					appLabel.SetText(title)
					minimap.QueueDraw()
				})
			})
			log.Printf("niri stream: %v; reconnecting", err)
			time.Sleep(2 * time.Second)
		}
	}()

	// hint view: shown while Super is held past hintDelay; follows modifier changes while shown
	var mask byte
	pending := false
	showHints := func() {
		modLabel.SetText(modNames(mask))
		hintLabel.SetMarkup(hintMarkup(hints.For(hints.Load(niriConfigPath()), mask)))
		stack.SetVisibleChildName("hints")
	}
	onMask := func(m byte) {
		mask = m
		switch {
		case m&hints.Super == 0:
			stack.SetVisibleChildName("bar")
		case stack.VisibleChildName() == "hints":
			showHints()
		case !pending:
			pending = true
			glib.TimeoutAdd(hintDelay, func() bool {
				pending = false
				if mask&hints.Super != 0 {
					showHints()
				}
				return false
			})
		}
	}
	go func() {
		for {
			err := readModifiers(func(m byte) { glib.IdleAdd(func() { onMask(m) }) })
			log.Printf("delphics-modd: %v; reconnecting", err)
			glib.IdleAdd(func() { onMask(0) })
			time.Sleep(2 * time.Second)
		}
	}()

	stat, err := sysstat.New()
	if err != nil {
		log.Printf("system bus: %v", err)
	}
	tick := func() bool {
		clock.SetText(time.Now().Format("15:04"))
		if stat != nil {
			netLabel.SetText(stat.Network())
			batLabel.SetText(stat.Battery())
		}
		return true
	}
	tick()
	// ponytail: polls every 5 s; switch to PropertiesChanged signals if the latency matters
	glib.TimeoutSecondsAdd(5, tick)

	setupNotifications(app, noteLabel, dndLabel)

	win.SetVisible(true)
}

// shownNote is a notification on screen; seq tells an expiry timer whether its notification was replaced since.
type shownNote struct {
	notify.Notification
	seq int
}

// setupNotifications makes the bar the notification daemon. The newest notification shows in noteLabel;
// clicking it runs its default action and dismisses it. The app action "dnd" toggles do-not-disturb,
// which hides everything but critical notifications.
func setupNotifications(app *gtk.Application, noteLabel, dndLabel *gtk.Label) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		log.Printf("session bus: %v", err)
		return
	}
	// GTK thread only from here on
	var shown []shownNote
	seq := 0
	dnd := false
	var srv *notify.Server

	render := func() {
		noteLabel.RemoveCSSClass("critical")
		if len(shown) == 0 {
			noteLabel.SetText("")
			return
		}
		n := shown[len(shown)-1]
		if n.Urgency == notify.Critical {
			noteLabel.AddCSSClass("critical")
		}
		noteLabel.SetMarkup(noteMarkup(n.Notification, len(shown)-1))
	}
	remove := func(id uint32) bool {
		for i, n := range shown {
			if n.ID == id {
				shown = append(shown[:i], shown[i+1:]...)
				render()
				return true
			}
		}
		return false
	}
	add := func(n notify.Notification) {
		if dnd && n.Urgency != notify.Critical {
			return
		}
		seq++
		s := shownNote{n, seq}
		replaced := false
		for i := range shown {
			if shown[i].ID == n.ID {
				shown[i], replaced = s, true
			}
		}
		if !replaced {
			shown = append(shown, s)
		}
		render()
		if n.Timeout > 0 {
			glib.TimeoutAdd(uint(n.Timeout.Milliseconds()), func() bool {
				for _, cur := range shown {
					if cur.ID == s.ID && cur.seq == s.seq {
						remove(s.ID)
						srv.Close(s.ID, notify.Expired)
					}
				}
				return false
			})
		}
	}

	srv, err = notify.Serve(conn,
		func(n notify.Notification) { glib.IdleAdd(func() { add(n) }) },
		func(id uint32) { glib.IdleAdd(func() { remove(id) }) })
	if err != nil {
		log.Printf("notifications: %v", err)
		return
	}

	click := gtk.NewGestureClick()
	click.ConnectReleased(func(int, float64, float64) {
		if len(shown) == 0 {
			return
		}
		n := shown[len(shown)-1]
		if n.HasAction("default") {
			srv.Invoke(n.ID, "default")
		}
		remove(n.ID)
		srv.Close(n.ID, notify.Dismissed)
	})
	noteLabel.AddController(click)

	toggle := gio.NewSimpleAction("dnd", nil)
	toggle.ConnectActivate(func(*glib.Variant) {
		dnd = !dnd
		dndLabel.SetVisible(dnd)
		if !dnd {
			return
		}
		kept := shown[:0]
		for _, n := range shown {
			if n.Urgency == notify.Critical {
				kept = append(kept, n)
			} else {
				srv.Close(n.ID, notify.Dismissed)
			}
		}
		shown = kept
		render()
	})
	app.AddAction(toggle)
}

// noteMarkup renders a notification as one line: app, summary, first body line, and how many more are queued.
func noteMarkup(n notify.Notification, more int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<span foreground=\"#c2ad61\" weight=\"bold\">%s</span> %s",
		glib.MarkupEscapeText(n.AppName), glib.MarkupEscapeText(n.Summary))
	if body, _, _ := strings.Cut(n.Body, "\n"); body != "" {
		fmt.Fprintf(&b, " <span foreground=\"#959074\">%s</span>", glib.MarkupEscapeText(body))
	}
	if more > 0 {
		fmt.Fprintf(&b, " <span foreground=\"#959074\">+%d</span>", more)
	}
	return b.String()
}

// drawMinimap scales the strip so the whole strip and the visible area fit in the widget.
func drawMinimap(cr *cairo.Context, v niri.View, screenW, width, height float64) {
	if len(v.Columns) == 0 {
		return
	}
	last := v.Columns[len(v.Columns)-1]
	stripW := last.X + last.Width
	left := min(0, v.ViewX)
	right := max(stripW, v.ViewX+screenW)
	scale := width / (right - left)
	const tileH = 12.0
	y := (height - tileH) / 2
	for _, c := range v.Columns {
		x := (c.X - left) * scale
		w := max(c.Width*scale-1, 2)
		if c.Focused {
			setColor(cr, palette.primary)
		} else {
			setColor(cr, palette.border)
		}
		cr.Rectangle(x, y, w, tileH)
		cr.Fill()
	}
	setColor(cr, palette.muted)
	cr.SetLineWidth(1)
	cr.Rectangle((v.ViewX-left)*scale+0.5, y-2.5, screenW*scale-1, tileH+5)
	cr.Stroke()
}

// appNames maps desktop file IDs without ".desktop" to their Name.
func appNames() map[string]string {
	names := map[string]string{}
	for _, info := range gio.AppInfoGetAll() {
		names[strings.TrimSuffix(info.ID(), ".desktop")] = info.Name()
	}
	return names
}

func displayName(names map[string]string, appID string) string {
	if n, ok := names[appID]; ok {
		return n
	}
	return appID
}

// readModifiers calls onMask with every mask byte delphics-modd sends until the connection fails.
func readModifiers(onMask func(byte)) error {
	conn, err := net.Dial("unix", moddSocket)
	if err != nil {
		return err
	}
	defer conn.Close()
	buf := make([]byte, 1)
	for {
		if _, err := conn.Read(buf); err != nil {
			return err
		}
		onMask(buf[0])
	}
}

func niriConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "/usr/share/delphics/niri/config.kdl"
	}
	return filepath.Join(dir, "niri", "config.kdl")
}

func modNames(m byte) string {
	var parts []string
	for _, mod := range []struct {
		bit  byte
		name string
	}{{hints.Super, "Super"}, {hints.Shift, "Shift"}, {hints.Ctrl, "Ctrl"}, {hints.Alt, "Alt"}} {
		if m&mod.bit != 0 {
			parts = append(parts, mod.name)
		}
	}
	return strings.Join(parts, "+")
}

func hintMarkup(hs []hints.Hint) string {
	var b strings.Builder
	for i, h := range hs {
		if i > 0 {
			b.WriteString("  ")
		}
		fmt.Fprintf(&b, "<span foreground=\"#ebe4d2\" weight=\"bold\">%s</span> %s",
			glib.MarkupEscapeText(strings.Join(h.Keys, "/")), glib.MarkupEscapeText(h.Action))
	}
	return b.String()
}
