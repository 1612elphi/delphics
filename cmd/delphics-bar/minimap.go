package main

import (
	"math"

	"github.com/diamondburned/gotk4/pkg/cairo"

	"delphics.delphi.tools/internal/niri"
)

// Minimap geometry in widget pixels. The screen frame is mapH tall; inside it, and between tiles,
// everything is spaced by mapGap with the 1 px frame line centered in the gap.
const (
	mapH      = 22
	mapGap    = 3
	mapMaxW   = 240
	mapTileIn = 2 // frame line + 1 px space
)

type pxRect struct{ x, y, w, h int }

type mapTile struct {
	pxRect
	focused bool
}

type minimap struct {
	width int
	tiles []mapTile
	// frame is the visible screen area, outer edge of its 1 px line
	frame pxRect
}

// layoutMinimap scales the strip so the working area is mapH tall. Window sizes keep their proportions,
// but niri's gaps become a fixed mapGap so the spacing is even at any scale.
func layoutMinimap(v niri.View, workH, gap float64) minimap {
	inner := mapH - 2*mapTileIn
	s := float64(inner) / max(workH-2*gap, 1)

	px := make([]int, len(v.Columns))
	pw := make([]int, len(v.Columns))
	x := 0
	for i, c := range v.Columns {
		px[i], pw[i] = x, max(int(math.Round(c.Width*s)), 2)
		x += pw[i] + mapGap
	}
	// f maps strip x to widget x: columns scale to their pixel widths, gaps to mapGap, the rest by s
	f := func(sx float64) float64 {
		if len(v.Columns) == 0 {
			return sx * s
		}
		for i, c := range v.Columns {
			if sx < c.X {
				prevEnd, mPrevEnd := c.X-gap, float64(px[i]-mapGap)
				if i > 0 {
					prevEnd, mPrevEnd = v.Columns[i-1].X+v.Columns[i-1].Width, float64(px[i-1]+pw[i-1])
				}
				if sx >= prevEnd {
					return mPrevEnd + (sx-prevEnd)/(c.X-prevEnd)*(float64(px[i])-mPrevEnd)
				}
				return mPrevEnd + (sx-prevEnd)*s
			}
			if sx <= c.X+c.Width {
				return float64(px[i]) + (sx-c.X)/c.Width*float64(pw[i])
			}
		}
		last := len(v.Columns) - 1
		end, mEnd := v.Columns[last].X+v.Columns[last].Width, float64(px[last]+pw[last])
		if sx <= end+gap {
			return mEnd + (sx-end)/gap*mapGap
		}
		return mEnd + mapGap + (sx-end-gap)*s
	}

	// a view edge one gap from a column lands on the gap's middle pixel
	left := int(math.Round(f(v.ViewX))) + 1
	right := int(math.Round(f(v.ViewX+v.ViewW))) - 2
	m := minimap{frame: pxRect{left, 0, right - left + 1, mapH}}
	for i, c := range v.Columns {
		total := 0.0
		for _, t := range c.Tiles {
			total += t.Height
		}
		avail := float64(inner - (len(c.Tiles)-1)*mapGap)
		cum := 0.0
		for j, t := range c.Tiles {
			y0 := mapTileIn + j*mapGap + int(math.Round(cum/total*avail))
			cum += t.Height
			y1 := mapTileIn + j*mapGap + int(math.Round(cum/total*avail))
			m.tiles = append(m.tiles, mapTile{pxRect{px[i], y0, pw[i], y1 - y0}, t.Focused})
		}
	}

	lo, hi := left, right+1
	if len(px) > 0 {
		lo, hi = min(lo, 0), max(hi, x-mapGap)
	}
	shift := -lo
	m.width = hi - lo
	if m.width > mapMaxW {
		// pan to keep the frame centered as far as the strip allows
		shift = min(max(mapMaxW/2-(left+right)/2, mapMaxW-hi), -lo)
		m.width = mapMaxW
	}
	m.frame.x += shift
	for i := range m.tiles {
		m.tiles[i].x += shift
	}
	return m
}

// draw paints tiles dimmed outside the frame, fully inside it, and the frame on top.
func (m minimap) draw(cr *cairo.Context, height int) {
	y0 := float64((height - mapH) / 2)
	rect := func(r pxRect) { cr.Rectangle(float64(r.x), y0+float64(r.y), float64(r.w), float64(r.h)) }
	paint := func(alpha float64) {
		for _, t := range m.tiles {
			c := palette.muted
			if t.focused {
				c = palette.primary
			}
			cr.SetSourceRGBA(c[0], c[1], c[2], alpha)
			rect(t.pxRect)
			cr.Fill()
		}
	}
	paint(0.35)
	cr.Save()
	rect(pxRect{m.frame.x + 1, 0, m.frame.w - 2, mapH})
	cr.Clip()
	paint(1)
	cr.Restore()

	setColor(cr, palette.fg)
	f := m.frame
	for _, r := range []pxRect{{f.x, 0, f.w, 1}, {f.x, mapH - 1, f.w, 1}, {f.x, 0, 1, mapH}, {f.x + f.w - 1, 0, 1, mapH}} {
		rect(r)
	}
	cr.Fill()
}
