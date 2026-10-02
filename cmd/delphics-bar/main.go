// delphics-bar is the DELPHICS top bar: focused app and strip minimap on the left, notifications in the
// middle, system status on the right. It is also the session's notification daemon.
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"reflect"
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

	"delphics.delphi.tools/internal/baritems"
	"delphics.delphi.tools/internal/hints"
	"delphics.delphi.tools/internal/layershell"
	"delphics.delphi.tools/internal/niri"
	"delphics.delphi.tools/internal/notify"
)

// niri layout gap from delphics-desktop/niri/config.kdl
const niriGap = 8

// milliseconds Super must be held before the hint view replaces the bar; shorter taps are shortcuts
const hintDelay = 600

// milliseconds of the crossfade between the bar and the hint view
const hintFade = 200

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
.delphics-bar .mods { color: #c2ad61; font-weight: 700; }
.delphics-bar .item { -gtk-icon-palette: success #c2ad61, warning #c2ad61, error #c2ad61; }
.delphics-bar .item.urgent { color: #c2ad61; font-weight: 700; }
.delphics-bar .item.bold { color: #ebe4d2; font-weight: 700; }
.delphics-bar .hints { color: #959074; }
.delphics-bar .notification.critical { color: #c2ad61; }
.delphics-bar popover.menu { font-family: "Open Sans"; font-stretch: condensed; font-size: 13px; }
.delphics-bar popover.menu > contents { background: #101f10; color: #ebe4d2; border: 1px solid #213321; border-radius: 0; box-shadow: none; padding: 4px 0; }
.delphics-bar popover.menu modelbutton { border-radius: 0; padding: 3px 14px; min-height: 22px; }
.delphics-bar popover.menu modelbutton:hover, .delphics-bar popover.menu modelbutton:selected,
.delphics-bar popover.menu modelbutton:hover label, .delphics-bar popover.menu modelbutton:selected label,
.delphics-bar popover.menu modelbutton:hover arrow, .delphics-bar popover.menu modelbutton:selected arrow { background: #213321; color: #ebe4d2; }
.delphics-bar popover.menu modelbutton:disabled { color: #959074; }
.delphics-bar popover.menu modelbutton check { color: #c2ad61; }
.delphics-bar popover.menu box.slider { padding: 2px 14px; }
.delphics-bar popover.menu scale trough { background: #213321; border: none; outline: none; box-shadow: none; border-radius: 0; min-height: 4px; }
.delphics-bar popover.menu scale highlight { background: #c2ad61; border: none; box-shadow: none; border-radius: 0; }
.delphics-bar popover.menu scale slider { background: #ebe4d2; border-radius: 0; min-width: 10px; min-height: 14px; margin: -6px 0; box-shadow: none; border: none; }
.delphics-bar popover.menu separator { background: #213321; }
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
	appLabel := gtk.NewLabel("Desktop")
	appLabel.AddCSSClass("app")
	haveWindow := false
	// the system menu hangs off a box around the label; see popupMenu
	appBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	appBox.Append(appLabel)
	var appMenu *openMenu
	var appClosed time.Time
	appClick := gtk.NewGestureClick()
	appClick.ConnectReleased(func(int, float64, float64) {
		switch {
		case appMenu != nil:
			appMenu.pop.Popdown()
		case time.Since(appClosed) > reopenGuard:
			appMenu = popupMenu(appBox, systemMenu(haveWindow), menuHandlers{activate: runSystem})
			appMenu.pop.ConnectClosed(func() { appMenu, appClosed = nil, time.Now() })
		}
	})
	appBox.AddController(appClick)
	mapArea := gtk.NewDrawingArea()
	mapArea.SetContentHeight(28)
	noteLabel := gtk.NewLabel("")
	noteLabel.AddCSSClass("notification")
	noteLabel.SetHExpand(true)
	noteLabel.SetEllipsize(pango.EllipsizeEnd)
	dndLabel := gtk.NewLabel("dnd")
	dndLabel.AddCSSClass("status")
	dndLabel.SetVisible(false)
	// plugin items, including the built-in network, battery and clock plugins (delphics plugin NAME)
	itemBox := gtk.NewBox(gtk.OrientationHorizontal, 14)
	for _, w := range []gtk.Widgetter{appBox, mapArea, noteLabel, dndLabel, itemBox} {
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
	stack.SetTransitionType(gtk.StackTransitionTypeCrossfade)
	stack.SetTransitionDuration(hintFade)
	win.SetChild(stack)

	var mu sync.Mutex
	state := niri.NewState()
	names := appNames()
	var layout minimap

	mapArea.SetDrawFunc(func(_ *gtk.DrawingArea, cr *cairo.Context, _, height int) {
		layout.draw(cr, height)
	})
	// refresh recomputes the view from the niri state; GTK thread only
	refresh := func() {
		surface := win.Surface()
		if surface == nil {
			return
		}
		mon := gdk.DisplayGetDefault().MonitorAtSurface(surface)
		if mon == nil {
			return
		}
		geo := mon.Geometry()
		// the bar's exclusive zone is the only strut, so the working area is the monitor minus the bar
		workH := float64(geo.Height() - win.Height())
		mu.Lock()
		view := state.View(niriGap, float64(geo.Width()))
		mu.Unlock()
		// with no window focused the label still names something, so the system menu stays reachable
		title := "Desktop"
		if view.Focused != nil {
			title = displayName(names, view.Focused.AppID)
		}
		appLabel.SetText(title)
		haveWindow = view.Focused != nil
		layout = layoutMinimap(view, workH, niriGap)
		mapArea.SetContentWidth(layout.width)
		mapArea.QueueDraw()
	}
	win.ConnectMap(func() { glib.IdleAdd(refresh) })

	go func() {
		for {
			err := niri.Stream(func(line []byte) {
				mu.Lock()
				if err := state.Apply(line); err != nil {
					log.Printf("niri event: %v", err)
				}
				mu.Unlock()
				glib.IdleAdd(refresh)
			})
			log.Printf("niri stream: %v; reconnecting", err)
			time.Sleep(2 * time.Second)
		}
	}()

	// hint view: shown while Super is held past hintDelay; follows modifier changes while shown
	var mask byte
	// pending is the running hintDelay timer, 0 when none; releasing Super cancels it,
	// so a quick tap followed by a new press starts the delay over
	var pending glib.SourceHandle
	showHints := func() {
		modLabel.SetText(modNames(mask))
		hintLabel.SetMarkup(hintMarkup(hints.For(hints.Load(niriConfigPath()), mask)))
		stack.SetVisibleChildName("hints")
	}
	onMask := func(m byte) {
		mask = m
		switch {
		case m&hints.Super == 0:
			if pending != 0 {
				glib.SourceRemove(pending)
				pending = 0
			}
			stack.SetVisibleChildName("bar")
		case stack.VisibleChildName() == "hints":
			showHints()
		case pending == 0:
			pending = glib.TimeoutAdd(hintDelay, func() bool {
				pending = 0
				showHints()
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

	if conn, err := dbus.ConnectSessionBus(); err != nil {
		log.Printf("session bus: %v", err)
	} else {
		setupNotifications(app, conn, noteLabel, dndLabel)
		setupItems(conn, itemBox)
	}

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
func setupNotifications(app *gtk.Application, conn *dbus.Conn, noteLabel, dndLabel *gtk.Label) {
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

	var err error
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

// reopenGuard: with a mouse, clicking the item of an open menu closes the menu on press (GTK's
// autohide) and would reopen it on release; a release this soon after the close keeps it closed.
const reopenGuard = 300 * time.Millisecond

// itemView is the widget of one plugin item; it lives as long as the item, so an open menu survives updates.
type itemView struct {
	box   *gtk.Box
	icon  *gtk.Image
	label *gtk.Label
	item  baritems.Item
	// menu is the open menu, nil when closed
	menu *openMenu
	// closed is when the menu last closed; see reopenGuard
	closed time.Time
}

// setupItems shows plugin items (internal/baritems) in box and reports clicks and menu choices back.
// A left click opens the item's menu when it has one; every click is also sent as Clicked, so a plugin
// can refresh what its menu shows.
func setupItems(conn *dbus.Conn, box *gtk.Box) {
	var srv *baritems.Server
	views := map[string]*itemView{}
	newView := func(id string) *itemView {
		v := &itemView{box: gtk.NewBox(gtk.OrientationHorizontal, 4), icon: gtk.NewImage(), label: gtk.NewLabel("")}
		v.icon.SetPixelSize(16)
		v.box.Append(v.icon)
		v.box.Append(v.label)
		click := gtk.NewGestureClick()
		click.SetButton(0)
		click.ConnectReleased(func(int, float64, float64) {
			button := click.CurrentButton()
			switch {
			case button != 1 || len(v.item.Menu) == 0:
			case v.menu != nil:
				// touch input does not close a popover on a tap outside, so the item closes its own menu
				v.menu.pop.Popdown()
			case time.Since(v.closed) > reopenGuard:
				v.menu = popupMenu(v.box, pluginMenu(v.item.Menu), menuHandlers{
					activate: func(entry string) { srv.Activate(id, entry) },
					change:   func(entry string, value float64) { srv.Change(id, entry, value) },
				})
				v.menu.pop.ConnectClosed(func() { v.menu, v.closed = nil, time.Now() })
			}
			srv.Click(id, uint32(button))
		})
		v.box.AddController(click)
		scroll := gtk.NewEventControllerScroll(gtk.EventControllerScrollVertical)
		scroll.ConnectScroll(func(_, dy float64) bool {
			srv.Scroll(id, dy)
			return true
		})
		v.box.AddController(scroll)
		return v
	}
	render := func(items []baritems.Item) {
		seen := map[string]bool{}
		var prev gtk.Widgetter
		for _, it := range items {
			seen[it.ID] = true
			v := views[it.ID]
			if v == nil {
				v = newView(it.ID)
				views[it.ID] = v
				box.Append(v.box)
			}
			menuChanged := v.menu != nil && !reflect.DeepEqual(v.item.Menu, it.Menu)
			v.item = it
			for class, on := range map[string]bool{"status": true, "item": true, "urgent": it.Urgent, "bold": it.Bold} {
				if on {
					v.box.AddCSSClass(class)
				} else {
					v.box.RemoveCSSClass(class)
				}
			}
			// symbolic icons take the item's text color
			v.icon.SetFromIconName(it.Icon)
			v.icon.SetVisible(it.Icon != "")
			v.label.SetText(it.Text)
			v.label.SetVisible(it.Text != "")
			v.box.SetTooltipText(it.Tooltip)
			v.box.SetVisible(it.Text != "" || it.Icon != "")
			if menuChanged {
				v.menu.update(pluginMenu(it.Menu))
			}
			box.ReorderChildAfter(v.box, prev)
			prev = v.box
		}
		for id, v := range views {
			if !seen[id] {
				if v.menu != nil {
					v.menu.pop.Popdown()
				}
				box.Remove(v.box)
				delete(views, id)
			}
		}
	}
	var err error
	srv, err = baritems.Serve(conn, func(items []baritems.Item) { glib.IdleAdd(func() { render(items) }) })
	if err != nil {
		log.Printf("bar items: %v", err)
	}
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
