// Package notify implements the org.freedesktop.Notifications server side for the bar.
package notify

import (
	"errors"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
)

const (
	busName = "org.freedesktop.Notifications"
	path    = "/org/freedesktop/Notifications"
	iface   = "org.freedesktop.Notifications"
)

// DefaultTimeout applies when the sender passes expire_timeout -1.
const DefaultTimeout = 5 * time.Second

// Close reasons from the spec.
const (
	Expired   uint32 = 1
	Dismissed uint32 = 2
	Closed    uint32 = 3
)

const Critical byte = 2

type Notification struct {
	ID      uint32
	AppName string
	Summary string
	Body    string
	// Actions holds the spec's flat key/label pairs.
	Actions []string
	Urgency byte
	// Timeout is 0 when the notification stays until dismissed.
	Timeout time.Duration
}

// HasAction reports whether key is one of the notification's action keys.
func (n Notification) HasAction(key string) bool {
	for i := 0; i+1 < len(n.Actions); i += 2 {
		if n.Actions[i] == key {
			return true
		}
	}
	return false
}

// Server owns the notification bus name. Callbacks run on D-Bus goroutines.
type Server struct {
	conn     *dbus.Conn
	onNotify func(Notification)
	onClose  func(id uint32)
	mu       sync.Mutex
	lastID   uint32
}

// Serve exports the server on conn and takes the bus name; it fails if another daemon holds it.
func Serve(conn *dbus.Conn, onNotify func(Notification), onClose func(id uint32)) (*Server, error) {
	s := &Server{conn: conn, onNotify: onNotify, onClose: onClose}
	if err := conn.Export(dbusMethods{s}, path, iface); err != nil {
		return nil, err
	}
	if err := conn.Export(introspect.Introspectable(introspection), path, "org.freedesktop.DBus.Introspectable"); err != nil {
		return nil, err
	}
	reply, err := conn.RequestName(busName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return nil, err
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return nil, errNameTaken
	}
	return s, nil
}

var errNameTaken = errors.New(busName + " is owned by another daemon")

// Close tells the sender the notification is gone.
func (s *Server) Close(id, reason uint32) {
	s.conn.Emit(path, iface+".NotificationClosed", id, reason)
}

// Invoke tells the sender the user picked an action.
func (s *Server) Invoke(id uint32, action string) {
	s.conn.Emit(path, iface+".ActionInvoked", id, action)
}

func (s *Server) nextID(replaces uint32) uint32 {
	if replaces != 0 {
		return replaces
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastID++
	if s.lastID == 0 {
		s.lastID++
	}
	return s.lastID
}

// Parse builds a Notification from the Notify call's arguments.
func Parse(id uint32, appName, summary, body string, actions []string, hints map[string]dbus.Variant, timeoutMs int32) Notification {
	n := Notification{ID: id, AppName: appName, Summary: summary, Body: body, Actions: actions}
	if v, ok := hints["urgency"]; ok {
		// senders disagree on the type; byte is the spec, some send int32 or uint32
		switch u := v.Value().(type) {
		case byte:
			n.Urgency = u
		case int32:
			n.Urgency = byte(u)
		case uint32:
			n.Urgency = byte(u)
		}
	}
	switch {
	case timeoutMs > 0:
		n.Timeout = time.Duration(timeoutMs) * time.Millisecond
	case timeoutMs < 0 && n.Urgency != Critical:
		n.Timeout = DefaultTimeout
	}
	return n
}

// dbusMethods keeps the exported method set to the spec's names only.
type dbusMethods struct{ s *Server }

func (m dbusMethods) Notify(appName string, replacesID uint32, _ string, summary, body string,
	actions []string, hints map[string]dbus.Variant, timeout int32) (uint32, *dbus.Error) {
	n := Parse(m.s.nextID(replacesID), appName, summary, body, actions, hints, timeout)
	m.s.onNotify(n)
	return n.ID, nil
}

func (m dbusMethods) CloseNotification(id uint32) *dbus.Error {
	m.s.onClose(id)
	m.s.Close(id, Closed)
	return nil
}

func (m dbusMethods) GetCapabilities() ([]string, *dbus.Error) {
	// actions: only the "default" action is reachable, by clicking the notification
	return []string{"body", "actions"}, nil
}

func (m dbusMethods) GetServerInformation() (string, string, string, string, *dbus.Error) {
	return "delphics-bar", "DELPHICS", "0", "1.2", nil
}

const introspection = introspect.IntrospectDeclarationString + `<node>
 <interface name="org.freedesktop.Notifications">
  <method name="Notify">
   <arg direction="in" type="s"/><arg direction="in" type="u"/><arg direction="in" type="s"/>
   <arg direction="in" type="s"/><arg direction="in" type="s"/><arg direction="in" type="as"/>
   <arg direction="in" type="a{sv}"/><arg direction="in" type="i"/><arg direction="out" type="u"/>
  </method>
  <method name="CloseNotification"><arg direction="in" type="u"/></method>
  <method name="GetCapabilities"><arg direction="out" type="as"/></method>
  <method name="GetServerInformation">
   <arg direction="out" type="s"/><arg direction="out" type="s"/><arg direction="out" type="s"/><arg direction="out" type="s"/>
  </method>
  <signal name="NotificationClosed"><arg type="u"/><arg type="u"/></signal>
  <signal name="ActionInvoked"><arg type="u"/><arg type="s"/></signal>
 </interface>` + introspect.IntrospectDataString + `</node>`
