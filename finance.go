package charta

import (
	"fmt"
	"math"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

// Price bars over time: candlesticks and open-high-low-close bars.

const (
	// priceOutline is the stroke of wicks, OHLC bars and hollow bodies.
	priceOutline = 1.5
	// priceFill is the share of the time between periods a candle takes.
	priceFill = 0.6
)

// Rising and falling periods: the palette's first two slots (a CVD-safe
// pair), never the reserved status red/green. Shape repeats the cue: rising
// candles are hollow, falling ones solid.
var (
	priceRise = base.Series(0)
	priceFall = base.Series(1)
)

// Candle is one period's prices. Time is the period's start, UTC epoch ms.
type Candle struct {
	Time  int64   `json:"time"`
	Open  float64 `json:"open"`
	High  float64 `json:"high"`
	Low   float64 `json:"low"`
	Close float64 `json:"close"`
}

func (c Candle) rising() bool { return c.Close >= c.Open }

// CandlestickChart draws each period as a body from open to close and wicks
// to the high and low. UTCOffset is the minutes the displayed clock is ahead
// of UTC (dates and ticks only).
type CandlestickChart struct {
	Title     string   `json:"title,omitempty"`
	UTCOffset int      `json:"utcOffset,omitempty"`
	Data      []Candle `json:"data"`
}

// OHLCChart draws each period's range as a line, with open and close as
// ticks to the left and right. UTCOffset is as in CandlestickChart.
type OHLCChart struct {
	Title     string   `json:"title,omitempty"`
	UTCOffset int      `json:"utcOffset,omitempty"`
	Data      []Candle `json:"data"`
}

func (c CandlestickChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	s, _, err := c.laidOut(width, height, m, nil)
	return s, err
}

func (c CandlestickChart) PlotScene(width, height float64, m base.TextMeasure) (*scene.Scene, scene.Plot, error) {
	return c.laidOut(width, height, m, nil)
}

func (c CandlestickChart) View(x [2]float64, y *[2]float64) Viewable {
	return inView[CandlestickChart]{c, ranges{x, y}}
}

func (c CandlestickChart) laidOut(width, height float64, m base.TextMeasure, v *ranges) (*scene.Scene, scene.Plot, error) {
	return priceScene(true, c.Title, c.UTCOffset, c.Data, width, height, m, v)
}

func (c OHLCChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	s, _, err := c.laidOut(width, height, m, nil)
	return s, err
}

func (c OHLCChart) PlotScene(width, height float64, m base.TextMeasure) (*scene.Scene, scene.Plot, error) {
	return c.laidOut(width, height, m, nil)
}

func (c OHLCChart) View(x [2]float64, y *[2]float64) Viewable {
	return inView[OHLCChart]{c, ranges{x, y}}
}

func (c OHLCChart) laidOut(width, height float64, m base.TextMeasure, v *ranges) (*scene.Scene, scene.Plot, error) {
	return priceScene(false, c.Title, c.UTCOffset, c.Data, width, height, m, v)
}

func checkCandles(data []Candle) error {
	for i, c := range data {
		for _, f := range []struct {
			what string
			v    float64
		}{{"open", c.Open}, {"high", c.High}, {"low", c.Low}, {"close", c.Close}} {
			if err := finite(fmt.Sprintf("candle %d %s", i, f.what), f.v); err != nil {
				return err
			}
		}
		if c.Low > math.Min(c.Open, c.Close) || c.High < math.Max(c.Open, c.Close) {
			return invalid("candle %d: low and high must bracket open and close", i)
		}
		if i > 0 && c.Time <= data[i-1].Time {
			return invalid("candle %d: times must increase", i)
		}
	}
	return nil
}

// priceScene lays the bars out over all of data, or over the times in v
// (epoch ms) showing the periods whose whole bar fits.
func priceScene(candles bool, title string, offset int, data []Candle, width, height float64, m base.TextMeasure, v *ranges) (*scene.Scene, scene.Plot, error) {
	if err := checkSize(width, height); err != nil {
		return nil, scene.Plot{}, err
	}
	if err := checkCandles(data); err != nil {
		return nil, scene.Plot{}, err
	}
	// Each period owns the time up to halfway to its neighbours; the domain
	// pads half a period at both ends so the first and last bars fit.
	step := base.Day
	for i := 1; i < len(data); i++ {
		d := data[i].Time - data[i-1].Time
		if i == 1 || d < step {
			step = d
		}
	}
	t0, t1 := int64(0), base.Day
	switch {
	case v != nil:
		t0, t1 = int64(math.Round(v.x[0])), int64(math.Round(v.x[1]))
	case len(data) > 0:
		t0, t1 = data[0].Time-step/2, data[len(data)-1].Time+step/2
	}
	reach := float64(float64(step)*priceFill) / 2
	var shown []int
	for i, c := range data {
		if v == nil || (float64(c.Time)-reach >= float64(t0) && float64(c.Time)+reach <= float64(t1)) {
			shown = append(shown, i)
		}
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, i := range shown {
		lo, hi = math.Min(lo, data[i].Low), math.Max(hi, data[i].High)
	}
	if !isFinite(lo) {
		lo, hi = 0, 1
	}

	var marks []scene.Mark
	top := drawTitle(&marks, title)
	key := []keyItem{
		{keySample{kind: sampleLine, color: priceRise, width: 2}, "Rising"},
		{keySample{kind: sampleLine, color: priceFall, width: 2}, "Falling"},
	}
	if candles {
		key = []keyItem{
			{keySample{kind: sampleHollow, color: priceRise}, "Rising"},
			{keySample{kind: sampleSolid, color: priceFall}, "Falling"},
		}
	}
	if len(data) > 0 {
		top = staticKey(&marks, m, top, key)
	}
	f := newTimeFrame(m, width, height, top, t0, t1, lo, hi, offset)
	f.grid(&marks)
	slot := f.x.pos(t0+step) - f.x.pos(t0)
	w := math.Max(1, math.Min(barMax, slot*priceFill))
	y := f.y.scale.Map
	for _, i := range shown {
		c := data[i]
		cx := f.x.pos(c.Time)
		color := priceFall
		if c.rising() {
			color = priceRise
		}
		hit := &scene.Hit{
			Key:   strconv.Itoa(i),
			Label: base.FormatDatetime(c.Time, offset),
			Rows: []scene.TipRow{
				{Name: "open", Value: base.Value(c.Open)},
				{Name: "high", Value: base.Value(c.High)},
				{Name: "low", Value: base.Value(c.Low)},
				{Name: "close", Value: base.Value(c.Close)},
			},
			Area: &scene.Rect{X: cx - slot/2, Y: f.top, W: slot, H: f.bottom - f.top},
		}
		line := func(x1, y1, x2, y2 float64) scene.Mark {
			return scene.Mark{Shape: scene.LineShape{X1: x1, Y1: y1, X2: x2, Y2: y2}, Stroke: &scene.Stroke{Color: color, Width: priceOutline}}
		}
		if candles {
			marks = append(marks, line(cx, y(c.High), cx, y(c.Low)))
			body := scene.Rect{
				X: cx - w/2,
				Y: y(math.Max(c.Open, c.Close)),
				W: w,
				H: y(math.Min(c.Open, c.Close)) - y(math.Max(c.Open, c.Close)),
			}
			// The surface fill hides the wick inside a hollow body.
			mk := scene.Mark{Shape: scene.RectShape{Rect: body}, Fill: color, Hit: hit}
			if c.rising() {
				mk = scene.Mark{
					Shape:  scene.PathShape{Ops: rectOps(body)},
					Fill:   base.Surface,
					Stroke: &scene.Stroke{Color: color, Width: priceOutline},
					Hit:    hit,
				}
			}
			marks = append(marks, mk)
		} else {
			marks = append(marks, line(cx-w/2, y(c.Open), cx, y(c.Open)))
			marks = append(marks, line(cx, y(c.Close), cx+w/2, y(c.Close)))
			l := line(cx, y(c.High), cx, y(c.Low))
			l.Hit = hit
			marks = append(marks, l)
		}
	}
	f.baseline(&marks)
	f.labels(&marks)
	what := "OHLC chart"
	if candles {
		what = "Candlestick chart"
	}
	plot := scene.Plot{
		Rect: scene.Rect{X: f.left, Y: f.top, W: f.right - f.left, H: f.bottom - f.top},
		X:    [2]float64{float64(t0), float64(t1)},
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(title, fmt.Sprintf("%s of %s.", what, count(len(shown), "period", "periods"))),
		Marks:  marks,
	}, plot, nil
}
