// Package chart computes SVG geometry (bars, lines, bands, ticks) for the
// server-rendered charts. It is pure and imports nothing from the app.
// Coordinates are SVG user units; y grows downward.
package chart

import (
	"fmt"
	"math"
	"strings"
)

// Frame is the SVG viewport and the margins around the plot area.
type Frame struct{ W, H, Left, Right, Top, Bottom float64 }

// DefaultFrame is the frame used by all charts.
var DefaultFrame = Frame{W: 960, H: 320, Left: 56, Right: 8, Top: 16, Bottom: 32}

func (f Frame) PlotW() float64    { return f.W - f.Left - f.Right }
func (f Frame) PlotH() float64    { return f.H - f.Top - f.Bottom }
func (f Frame) Baseline() float64 { return f.H - f.Bottom }

const (
	colGap     = 0.2  // share of a column slot left empty
	qtyShare   = 0.35 // share of the plot height used by quantity bars
	qtyColor   = 3
	tickTarget = 4
)

type Series struct {
	Name   string
	Color  int
	Values []float64
}

type Rect struct {
	X, Y, W, H float64
	Color      int
	Name       string
	Value      float64
}

type Column struct {
	Label   string
	X       float64 // centre of the column slot
	Rects   []Rect
	Total   float64
	Current bool
}

type Tick struct{ Y, Value float64 }

type Bars struct {
	Frame   Frame
	Ticks   []Tick
	Columns []Column
	AvgY    float64
	Avg     float64
	HasAvg  bool
	Empty   bool

	maxTick float64
}

func clamp(v float64) float64 {
	if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

func val(s Series, i int) float64 {
	if i < len(s.Values) {
		return clamp(s.Values[i])
	}
	return 0
}

func slotGeom(f Frame, n, i int) (centre, width float64) {
	slot := f.PlotW() / float64(n)
	return f.Left + slot*(float64(i)+0.5), slot * (1 - colGap)
}

func newBars(f Frame, labels []string, maxTotal float64) Bars {
	b := Bars{Frame: f, Empty: maxTotal <= 0}
	vals := NiceTicks(maxTotal, tickTarget)
	b.maxTick = vals[len(vals)-1]
	for _, v := range vals {
		b.Ticks = append(b.Ticks, Tick{Y: b.yFor(v), Value: v})
	}
	for i, l := range labels {
		x, _ := slotGeom(f, len(labels), i)
		b.Columns = append(b.Columns, Column{Label: l, X: x})
	}
	return b
}

func (b *Bars) yFor(v float64) float64 {
	return b.Frame.Baseline() - clamp(v)/b.maxTick*b.Frame.PlotH()
}

// Stacked builds one stacked column per label; series stack bottom to top.
func Stacked(f Frame, labels []string, series []Series) Bars {
	maxTotal := 0.0
	totals := make([]float64, len(labels))
	for i := range labels {
		for _, s := range series {
			totals[i] += val(s, i)
		}
		if totals[i] > maxTotal {
			maxTotal = totals[i]
		}
	}
	b := newBars(f, labels, maxTotal)
	for i := range labels {
		_, w := slotGeom(f, len(labels), i)
		c := &b.Columns[i]
		c.Total = totals[i]
		acc := 0.0
		for _, s := range series {
			v := val(s, i)
			if v == 0 {
				continue
			}
			top := b.yFor(acc + v)
			c.Rects = append(c.Rects, Rect{
				X: c.X - w/2, Y: top, W: w, H: b.yFor(acc) - top,
				Color: s.Color, Name: s.Name, Value: v,
			})
			acc += v
		}
	}
	return b
}

// Grouped builds side-by-side bars (one per series) for each label.
func Grouped(f Frame, labels []string, series []Series) Bars {
	maxV := 0.0
	for i := range labels {
		for _, s := range series {
			if v := val(s, i); v > maxV {
				maxV = v
			}
		}
	}
	b := newBars(f, labels, maxV)
	for i := range labels {
		_, w := slotGeom(f, len(labels), i)
		c := &b.Columns[i]
		bw := w / float64(len(series))
		for j, s := range series {
			v := val(s, i)
			c.Total += v
			if v == 0 {
				continue
			}
			y := b.yFor(v)
			c.Rects = append(c.Rects, Rect{
				X: c.X - w/2 + bw*float64(j), Y: y, W: bw, H: f.Baseline() - y,
				Color: s.Color, Name: s.Name, Value: v,
			})
		}
	}
	return b
}

// SetAverage places the average line at v. Ignored when the chart is empty.
func (b *Bars) SetAverage(v float64) {
	if b.Empty {
		return
	}
	b.Avg, b.AvgY, b.HasAvg = v, b.yFor(v), true
}

type Point struct {
	X, Y, Value, Shelf float64
	Label              string
}

type Polyline []Point

// Points formats the polyline as an SVG "points" attribute.
func (p Polyline) Points() string {
	parts := make([]string, len(p))
	for i, pt := range p {
		parts[i] = fmt.Sprintf("%.1f,%.1f", pt.X, pt.Y)
	}
	return strings.Join(parts, " ")
}

type PriceChart struct {
	Frame  Frame
	Ticks  []Tick
	Labels []Column // only Label and X are set
	Avg    []Polyline
	Band   []string // one polygon per run of periods with lo and hi: hi left to right, then lo right to left
	Qty    []Rect
	QtyMax float64
	Low    *Point
	Empty  bool
}

// Price builds a price line chart with a lo/hi band and quantity bars.
// All slices are indexed by period; nil entries are gaps.
func Price(f Frame, labels []string, avg, lo, hi, shelf []*float64, qty []float64) PriceChart {
	at := func(s []*float64, i int) *float64 {
		if i < len(s) && s[i] != nil && !math.IsNaN(*s[i]) && !math.IsInf(*s[i], 0) {
			return s[i]
		}
		return nil
	}
	p := PriceChart{Frame: f, Empty: true}
	var minV, maxV float64
	seen := false
	note := func(v *float64) {
		if v == nil {
			return
		}
		if !seen {
			minV, maxV, seen = *v, *v, true
			return
		}
		if *v < minV {
			minV = *v
		}
		if *v > maxV {
			maxV = *v
		}
	}
	for i := range labels {
		if at(avg, i) != nil {
			p.Empty = false
		}
		note(at(avg, i))
		note(at(lo, i))
		note(at(hi, i))
	}
	if !seen {
		minV, maxV = 0, 1
	}
	vals := NiceRange(minV, maxV, tickTarget)
	axMin, axMax := vals[0], vals[len(vals)-1]
	y := func(v float64) float64 {
		return f.Baseline() - (v-axMin)/(axMax-axMin)*f.PlotH()
	}
	for _, v := range vals {
		p.Ticks = append(p.Ticks, Tick{Y: y(v), Value: v})
	}

	var cur Polyline
	var hiPts, loPts []string
	// endBand closes the current band run, so empty periods are not bridged.
	endBand := func() {
		if len(hiPts) == 0 {
			return
		}
		pts := hiPts
		for i := len(loPts) - 1; i >= 0; i-- {
			pts = append(pts, loPts[i])
		}
		p.Band = append(p.Band, strings.Join(pts, " "))
		hiPts, loPts = nil, nil
	}
	for i, l := range labels {
		x, _ := slotGeom(f, len(labels), i)
		p.Labels = append(p.Labels, Column{Label: l, X: x})

		if a := at(avg, i); a != nil {
			pt := Point{X: x, Y: y(*a), Value: *a, Label: l}
			if s := at(shelf, i); s != nil {
				pt.Shelf = *s
			}
			cur = append(cur, pt)
		} else if len(cur) > 0 {
			p.Avg = append(p.Avg, cur)
			cur = nil
		}

		lv, hv := at(lo, i), at(hi, i)
		if lv != nil && hv != nil {
			hiPts = append(hiPts, fmt.Sprintf("%.1f,%.1f", x, y(*hv)))
			loPts = append(loPts, fmt.Sprintf("%.1f,%.1f", x, y(*lv)))
		} else {
			endBand()
		}
		if lv != nil && (p.Low == nil || *lv < p.Low.Value) {
			p.Low = &Point{X: x, Y: y(*lv), Value: *lv, Label: l}
		}

		if i < len(qty) && clamp(qty[i]) > p.QtyMax {
			p.QtyMax = clamp(qty[i])
		}
	}
	if len(cur) > 0 {
		p.Avg = append(p.Avg, cur)
	}
	endBand()

	if p.QtyMax > 0 {
		maxH := qtyShare * f.PlotH()
		for i := range labels {
			if i >= len(qty) || clamp(qty[i]) == 0 {
				continue
			}
			x, w := slotGeom(f, len(labels), i)
			h := clamp(qty[i]) / p.QtyMax * maxH
			p.Qty = append(p.Qty, Rect{
				X: x - w/2, Y: f.Baseline() - h, W: w, H: h,
				Color: qtyColor, Name: labels[i], Value: qty[i],
			})
		}
	}
	return p
}
