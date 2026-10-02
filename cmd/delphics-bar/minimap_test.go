package main

import (
	"testing"

	"delphics.delphi.tools/internal/niri"
)

func TestMinimapSpacing(t *testing.T) {
	// 1366x740 working area, first column stacked, view scrolled to the left edge
	v := niri.View{
		ViewX: -8, ViewW: 1366,
		Columns: []niri.Column{
			{X: 0, Width: 671, Tiles: []niri.Tile{{Height: 358, Focused: true}, {Height: 358}}},
			{X: 679, Width: 671, Tiles: []niri.Tile{{Height: 724}}},
			{X: 1358, Width: 671, Tiles: []niri.Tile{{Height: 724}}},
		},
	}
	m := layoutMinimap(v, 740, 8)
	if len(m.tiles) != 4 {
		t.Fatalf("tiles = %+v, want 4 (stacked windows drawn separately)", m.tiles)
	}
	a, b, c, d := m.tiles[0], m.tiles[1], m.tiles[2], m.tiles[3]
	f := m.frame
	gaps := map[string]int{
		"frame left to tile":   a.x - f.x - 1,
		"frame top to tile":    a.y - f.y - 1,
		"tile to frame bottom": f.y + f.h - 1 - (b.y + b.h),
		"stacked tiles":        b.y - (a.y + a.h),
		"columns":              c.x - (a.x + a.w),
		"tile to frame right":  f.x + f.w - 1 - (c.x + c.w),
	}
	for name, g := range gaps {
		want := 1
		if name == "stacked tiles" || name == "columns" {
			want = mapGap
		}
		if g != want {
			t.Errorf("%s: %d px, want %d", name, g, want)
		}
	}
	if !a.focused || b.focused {
		t.Error("focus belongs to the top tile of column 1")
	}
	if d.x <= f.x+f.w {
		t.Error("third column should lie outside the frame")
	}
	if m.width != d.x+d.w-f.x || f.x != 0 {
		t.Errorf("width %d, frame x %d: widget must span frame to last tile", m.width, f.x)
	}
	if m.width > 120 {
		t.Errorf("width %d px for three half-screen columns; too wide", m.width)
	}
}

func TestMinimapPansWhenLong(t *testing.T) {
	var v niri.View
	for i := 0; i < 30; i++ {
		v.Columns = append(v.Columns, niri.Column{X: float64(i) * 679, Width: 671, Tiles: []niri.Tile{{Height: 724}}})
	}
	v.ViewX, v.ViewW = 29*679+671+8-1366, 1366 // last column, right-aligned
	m := layoutMinimap(v, 740, 8)
	if m.width != mapMaxW {
		t.Fatalf("width = %d, want cap %d", m.width, mapMaxW)
	}
	if f := m.frame; f.x < 0 || f.x+f.w > mapMaxW {
		t.Errorf("frame %+v outside the visible %d px", f, mapMaxW)
	}
}
