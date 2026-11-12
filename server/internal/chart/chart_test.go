package chart

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fp(v float64) *float64 { return &v }

func noNaN(t *testing.T, vals ...float64) {
	t.Helper()
	for _, v := range vals {
		assert.False(t, math.IsNaN(v) || math.IsInf(v, 0), "bad coordinate %v", v)
	}
}

func TestNiceTicks(t *testing.T) {
	assert.Equal(t, []float64{0, 200, 400, 600, 800}, NiceTicks(690, 4))
	assert.Equal(t, []float64{0, 1}, NiceTicks(0, 4))
	assert.Equal(t, []float64{0, 25, 50, 75, 100}, NiceTicks(95, 4))
}

func TestNiceRange(t *testing.T) {
	r := NiceRange(1.1, 1.9, 4)
	assert.LessOrEqual(t, r[0], 1.1)
	assert.GreaterOrEqual(t, r[len(r)-1], 1.9)
	same := NiceRange(2, 2, 4)
	require.GreaterOrEqual(t, len(same), 2)
	assert.Less(t, same[0], same[len(same)-1])
	for _, v := range append(r, same...) {
		noNaN(t, v)
	}
}

func TestFrame(t *testing.T) {
	f := DefaultFrame
	assert.InDelta(t, 584, f.PlotW(), 1e-9)
	assert.InDelta(t, 220, f.PlotH(), 1e-9)
	assert.InDelta(t, 232, f.Baseline(), 1e-9)
}

func TestStacked(t *testing.T) {
	f := DefaultFrame
	b := Stacked(f, []string{"a", "b"}, []Series{
		{Name: "A", Color: 1, Values: []float64{10, 0}},
		{Name: "B", Color: 2, Values: []float64{30, 20}},
	})
	assert.False(t, b.Empty)
	require.Len(t, b.Columns, 2)
	c0 := b.Columns[0]
	assert.InDelta(t, 40, c0.Total, 1e-9)
	require.Len(t, c0.Rects, 2)
	a, bb := c0.Rects[0], c0.Rects[1]
	assert.InDelta(t, f.Baseline(), a.Y+a.H, 1e-9)
	assert.InDelta(t, a.Y, bb.Y+bb.H, 1e-9)
	assert.InDelta(t, 10.0/30.0, a.H/bb.H, 1e-9)
	assert.Equal(t, "A", a.Name)
	assert.Equal(t, 2, bb.Color)
	assert.InDelta(t, 30, bb.Value, 1e-9)
	assert.Len(t, b.Columns[1].Rects, 1)
	assert.InDelta(t, f.Left+f.PlotW()/4, c0.X, 1e-9)
	assert.Equal(t, "a", c0.Label)
	top := b.Ticks[len(b.Ticks)-1]
	assert.InDelta(t, f.Top, top.Y, 1e-9)
	b.SetAverage(20)
	assert.True(t, b.HasAvg)
	assert.InDelta(t, f.Baseline()-20/top.Value*f.PlotH(), b.AvgY, 1e-9)
}

func TestStackedAllZero(t *testing.T) {
	for _, labels := range [][]string{{"a", "b"}, {"a"}, {}} {
		b := Stacked(DefaultFrame, labels, []Series{{Name: "A", Values: make([]float64, len(labels))}})
		assert.True(t, b.Empty)
		b.SetAverage(0)
		assert.False(t, b.HasAvg)
		noNaN(t, b.AvgY)
		for _, tk := range b.Ticks {
			noNaN(t, tk.Y, tk.Value)
		}
		for _, c := range b.Columns {
			noNaN(t, c.X, c.Total)
			for _, r := range c.Rects {
				noNaN(t, r.X, r.Y, r.W, r.H)
			}
		}
	}
}

func TestGrouped(t *testing.T) {
	b := Grouped(DefaultFrame, []string{"a", "b"}, []Series{
		{Name: "A", Color: 1, Values: []float64{10, 5}},
		{Name: "B", Color: 2, Values: []float64{30, 20}},
	})
	require.Len(t, b.Columns, 2)
	for _, c := range b.Columns {
		require.Len(t, c.Rects, 2)
		assert.LessOrEqual(t, c.Rects[0].X+c.Rects[0].W, c.Rects[1].X+1e-9)
		assert.InDelta(t, DefaultFrame.Baseline(), c.Rects[0].Y+c.Rects[0].H, 1e-9)
	}
}

func TestPriceGaps(t *testing.T) {
	p := Price(DefaultFrame, []string{"a", "b", "c"},
		[]*float64{fp(1.2), nil, fp(1.3)},
		[]*float64{fp(1.0), nil, fp(0.9)},
		[]*float64{fp(1.5), nil, fp(1.6)},
		[]*float64{fp(1.4), nil, nil},
		[]float64{3, 0, 6})
	assert.False(t, p.Empty)
	require.Len(t, p.Avg, 2)
	assert.InDelta(t, 1.4, p.Avg[0][0].Shelf, 1e-9)
	assert.InDelta(t, 0, p.Avg[1][0].Shelf, 1e-9)
	require.NotNil(t, p.Low)
	assert.InDelta(t, 0.9, p.Low.Value, 1e-9)
	assert.Equal(t, "c", p.Low.Label)
	assert.Equal(t, 3, len(p.Labels))
	assert.NotEmpty(t, p.Band)
	assert.InDelta(t, 6, p.QtyMax, 1e-9)
	require.Len(t, p.Qty, 2)
	assert.Equal(t, 3, p.Qty[0].Color)
	assert.InDelta(t, 0.35*DefaultFrame.PlotH(), p.Qty[1].H, 1e-9)
	assert.InDelta(t, DefaultFrame.Baseline(), p.Qty[1].Y+p.Qty[1].H, 1e-9)

	e := Price(DefaultFrame, []string{"a"}, []*float64{nil}, []*float64{nil}, []*float64{nil}, []*float64{nil}, []float64{0})
	assert.True(t, e.Empty)
	assert.Nil(t, e.Low)
}

func TestPriceSingleValue(t *testing.T) {
	p := Price(DefaultFrame, []string{"a"}, []*float64{fp(2)}, []*float64{fp(2)}, []*float64{fp(2)}, []*float64{nil}, []float64{0})
	require.Len(t, p.Avg, 1)
	noNaN(t, p.Avg[0][0].X, p.Avg[0][0].Y)
	for _, tk := range p.Ticks {
		noNaN(t, tk.Y, tk.Value)
	}
}

func TestPolylinePoints(t *testing.T) {
	p := Polyline{{X: 10, Y: 20.5}, {X: 30, Y: 40}}
	assert.Equal(t, "10.0,20.5 30.0,40.0", p.Points())
}
