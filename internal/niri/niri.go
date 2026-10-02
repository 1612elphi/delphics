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
	// views follows each workspace's scroll position, which niri's IPC does not report for tiled windows
	views map[uint64]*scroll
}

// scroll is the emulated view position of one workspace's strip.
type scroll struct {
	x float64
	// window that was focused, and the view position relative to its column
	window uint64
	off    float64
}

func NewState() *State {
	return &State{Windows: map[uint64]Window{}, Workspaces: map[uint64]Workspace{}, views: map[uint64]*scroll{}}
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

type Tile struct {
	Height  float64
	Focused bool
}

type Column struct {
	X, Width float64
	Focused  bool
	// Tiles top to bottom
	Tiles []Tile
}

// View is the scrolling strip of the focused workspace, in niri's logical pixels.
type View struct {
	Focused *Window
	Columns []Column
	// ViewX is the left edge of the visible screen area in strip coordinates, ViewW its width.
	ViewX, ViewW float64
}

// View lays out the focused workspace's strip. viewW is the output's working area width; the scroll
// position follows niri's center-focused-column "never": the view moves only as far as needed to show
// the focused column fully, by the gap on either side.
func (s *State) View(gap, viewW float64) View {
	v := View{ViewW: viewW}
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
	type tile struct {
		row int
		Tile
	}
	cols := map[int][]tile{}
	widths := map[int]float64{}
	// column index of every tiled window, to tell whether focus stayed in the same column
	colOf := map[uint64]int{}
	for _, w := range s.Windows {
		p := w.Layout.PosInScrollingLayout
		if w.IsFloating || p == nil || w.WorkspaceID == nil || *w.WorkspaceID != wsID {
			continue
		}
		cols[p[0]] = append(cols[p[0]], tile{p[1], Tile{w.Layout.TileSize[1], w.IsFocused}})
		widths[p[0]] = max(widths[p[0]], w.Layout.TileSize[0])
		colOf[w.ID] = p[0]
	}
	idx := make([]int, 0, len(cols))
	for i := range cols {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	x := 0.0
	focused := -1
	for _, i := range idx {
		ts := cols[i]
		sort.Slice(ts, func(a, b int) bool { return ts[a].row < ts[b].row })
		c := Column{X: x, Width: widths[i]}
		for _, t := range ts {
			c.Tiles = append(c.Tiles, t.Tile)
			c.Focused = c.Focused || t.Focused
		}
		if c.Focused {
			focused = len(v.Columns)
		}
		x += c.Width + gap
		v.Columns = append(v.Columns, c)
	}

	sc := s.views[wsID]
	if sc == nil {
		// a new strip starts with its first column one gap from the left edge
		sc = &scroll{x: -gap}
		s.views[wsID] = sc
	}
	if focused >= 0 && viewW > 0 {
		c := v.Columns[focused]
		cur := sc.x
		// niri keeps the view relative to the active column, so it moves with the column when columns
		// to its left open, close or resize
		if prev, ok := colOf[sc.window]; ok && prev == idx[focused] {
			cur = c.X + sc.off
		}
		sc.x = fitView(cur, viewW, c.X, c.Width, gap)
		sc.window, sc.off = v.Focused.ID, sc.x-c.X
	}
	v.ViewX = sc.x
	return v
}

// fitView is niri's compute_new_view_offset (src/layout/scrolling.rs, v26.04) in absolute coordinates:
// the new left edge of the view after focusing the column at colX.
func fitView(cur, viewW, colX, colW, gap float64) float64 {
	if viewW <= colW {
		return colX
	}
	pad := min(max((viewW-colW)/2, 0), gap)
	left, right := colX-pad, colX+colW+pad
	if cur <= left && right <= cur+viewW {
		return cur
	}
	if abs(cur-left) <= abs(cur+viewW-right) {
		return left
	}
	return right - viewW
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
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
