// Package niri follows niri's IPC event stream and keeps the window and workspace state.
package niri

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
)

type Layout struct {
	PosInScrollingLayout   *[2]int     `json:"pos_in_scrolling_layout"`
	TileSize               [2]float64  `json:"tile_size"`
	TilePosInWorkspaceView *[2]float64 `json:"tile_pos_in_workspace_view"`
}

type Window struct {
	ID          uint64  `json:"id"`
	Title       string  `json:"title"`
	AppID       string  `json:"app_id"`
	WorkspaceID *uint64 `json:"workspace_id"`
	IsFocused   bool    `json:"is_focused"`
	IsFloating  bool    `json:"is_floating"`
	Layout      Layout  `json:"layout"`
}

type Workspace struct {
	ID        uint64 `json:"id"`
	IsFocused bool   `json:"is_focused"`
}

type State struct {
	Windows    map[uint64]Window
	Workspaces map[uint64]Workspace
}

func NewState() *State {
	return &State{Windows: map[uint64]Window{}, Workspaces: map[uint64]Workspace{}}
}

// Apply updates the state from one event line. Lines that are not events (the initial reply) are ignored.
func (s *State) Apply(line []byte) error {
	var ev map[string]json.RawMessage
	if err := json.Unmarshal(line, &ev); err != nil {
		return err
	}
	for name, body := range ev {
		switch name {
		case "WorkspacesChanged":
			var e struct{ Workspaces []Workspace }
			if err := json.Unmarshal(body, &e); err != nil {
				return err
			}
			s.Workspaces = map[uint64]Workspace{}
			for _, w := range e.Workspaces {
				s.Workspaces[w.ID] = w
			}
		case "WorkspaceActivated":
			var e struct {
				ID      uint64
				Focused bool
			}
			if err := json.Unmarshal(body, &e); err != nil {
				return err
			}
			if e.Focused {
				for id, w := range s.Workspaces {
					w.IsFocused = id == e.ID
					s.Workspaces[id] = w
				}
			}
		case "WindowsChanged":
			var e struct{ Windows []Window }
			if err := json.Unmarshal(body, &e); err != nil {
				return err
			}
			s.Windows = map[uint64]Window{}
			for _, w := range e.Windows {
				s.Windows[w.ID] = w
			}
		case "WindowOpenedOrChanged":
			var e struct{ Window Window }
			if err := json.Unmarshal(body, &e); err != nil {
				return err
			}
			if e.Window.IsFocused {
				s.focus(&e.Window.ID)
			}
			s.Windows[e.Window.ID] = e.Window
		case "WindowClosed":
			var e struct{ ID uint64 }
			if err := json.Unmarshal(body, &e); err != nil {
				return err
			}
			delete(s.Windows, e.ID)
		case "WindowFocusChanged":
			var e struct{ ID *uint64 }
			if err := json.Unmarshal(body, &e); err != nil {
				return err
			}
			s.focus(e.ID)
		case "WindowLayoutsChanged":
			var e struct{ Changes [][2]json.RawMessage }
			if err := json.Unmarshal(body, &e); err != nil {
				return err
			}
			for _, c := range e.Changes {
				var id uint64
				var l Layout
				if err := json.Unmarshal(c[0], &id); err != nil {
					return err
				}
				if err := json.Unmarshal(c[1], &l); err != nil {
					return err
				}
				if w, ok := s.Windows[id]; ok {
					w.Layout = l
					s.Windows[id] = w
				}
			}
		}
	}
	return nil
}

func (s *State) focus(id *uint64) {
	for wid, w := range s.Windows {
		w.IsFocused = id != nil && wid == *id
		s.Windows[wid] = w
	}
}

type Column struct {
	X, Width float64
	Focused  bool
}

// View is the scrolling strip of the focused workspace, in niri's logical pixels.
type View struct {
	Focused *Window
	Columns []Column
	// ViewX is the left edge of the visible screen area in strip coordinates.
	ViewX float64
}

func (s *State) View(gap float64) View {
	var v View
	for _, w := range s.Windows {
		if w.IsFocused {
			w := w
			v.Focused = &w
		}
	}
	var wsID uint64
	for id, ws := range s.Workspaces {
		if ws.IsFocused {
			wsID = id
		}
	}
	cols := map[int]*Column{}
	viewPos := map[int]float64{}
	for _, w := range s.Windows {
		p := w.Layout.PosInScrollingLayout
		if w.IsFloating || p == nil || w.WorkspaceID == nil || *w.WorkspaceID != wsID {
			continue
		}
		c := cols[p[0]]
		if c == nil {
			c = &Column{}
			cols[p[0]] = c
		}
		c.Width = max(c.Width, w.Layout.TileSize[0])
		c.Focused = c.Focused || w.IsFocused
		if t := w.Layout.TilePosInWorkspaceView; t != nil {
			viewPos[p[0]] = t[0]
		}
	}
	idx := make([]int, 0, len(cols))
	for i := range cols {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	x := 0.0
	for _, i := range idx {
		c := cols[i]
		c.X = x
		x += c.Width + gap
		if vx, ok := viewPos[i]; ok {
			v.ViewX = c.X - vx
		}
		v.Columns = append(v.Columns, *c)
	}
	return v
}

// Stream sends every event line to onEvent until the connection fails.
func Stream(onEvent func(line []byte)) error {
	path := os.Getenv("NIRI_SOCKET")
	if path == "" {
		return fmt.Errorf("NIRI_SOCKET not set")
	}
	conn, err := net.Dial("unix", path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("\"EventStream\"\n")); err != nil {
		return err
	}
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		onEvent(sc.Bytes())
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return fmt.Errorf("niri closed the event stream")
}
