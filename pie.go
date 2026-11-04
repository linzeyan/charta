package charta

import (
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/dmath"
	"github.com/linzeyan/charta/scene"
)

const (
	// pieElbow and pieRun are the leader line: a radial run past the rim,
	// then a horizontal run to the label.
	pieElbow = 12.0
	pieRun   = 8.0
	pieRow   = legendSize + 4
	// pieRMin: below this radius the outside labels cost the pie too much;
	// the legend alone names the slices.
	pieRMin = 60.0
	// donutHole is a donut's hole, as a share of its radius.
	donutHole = 0.6
	totalSize = 22.0
)

// PieChart shows shares of a whole as slices of a circle, clockwise from
// twelve o'clock in data order.
type PieChart struct {
	Title string `json:"title,omitempty"`
	Data  []Bar  `json:"data"`
}

func (c PieChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	return pie(c.Title, c.Data, false, width, height, m)
}

// DonutChart is a pie with its center cut out, where the total is shown.
type DonutChart struct {
	Title string `json:"title,omitempty"`
	Data  []Bar  `json:"data"`
}

func (c DonutChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	return pie(c.Title, c.Data, true, width, height, m)
}

// percent is a share as a label: whole percents, with slivers kept visible
// as "<1".
func percent(share float64) string {
	p := share * 100
	if p > 0 && p < 1 {
		return "<1%"
	}
	return strconv.FormatFloat(math.Round(p), 'f', -1, 64) + "%"
}

// pieLabel is a label beside the pie, joined to its slice by a leader line.
type pieLabel struct {
	slice  int
	mid, y float64
}

// spread spaces one side's labels (sorted by y) at least a row apart within
// [lo, hi], each as close to where its slice points as it can be.
func spread(labels []pieLabel, lo, hi float64) {
	prev := math.Inf(-1)
	for i := range labels {
		labels[i].y = math.Max(math.Max(labels[i].y, prev+pieRow), lo)
		prev = labels[i].y
	}
	next := math.Inf(1)
	for i := len(labels) - 1; i >= 0; i-- {
		labels[i].y = math.Min(math.Min(labels[i].y, next-pieRow), hi)
		next = labels[i].y
	}
}

func pie(title string, data []Bar, hole bool, width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	if len(data) > maxSeries {
		return nil, invalid(`%d slices, at most %d: fold the smallest into "Other"`, len(data), maxSeries)
	}
	for i, b := range data {
		if err := finite(fmt.Sprintf("slice %d (%q)", i, b.Label), b.Value); err != nil {
			return nil, err
		}
		if b.Value < 0 {
			return nil, invalid("slice %d (%q) is negative; a share cannot be", i, b.Label)
		}
		for _, o := range data[:i] {
			if o.Label == b.Label {
				return nil, invalid("slice label %q is used twice", b.Label)
			}
		}
	}
	total := 0.0
	for _, b := range data {
		total += b.Value
	}
	if len(data) > 0 && total <= 0 {
		return nil, invalid("every slice is zero: there is no whole to divide")
	}
	kind := "Pie"
	if hole {
		kind = "Donut"
	}

	names := make([]string, len(data))
	for i, b := range data {
		names[i] = b.Label
	}
	var marks []scene.Mark
	top := drawTitle(&marks, title)
	top = legend(&marks, m, top, width, names, keySwatch)
	bottom := height - pad
	plotW, plotH := math.Max(width-2*pad, 1), math.Max(bottom-top, 1)
	cx, cy := width/2, (top+bottom)/2
	texts := make([]string, len(data))
	widest := 0.0
	for i, b := range data {
		texts[i] = b.Label + " " + percent(b.Value/total)
		widest = math.Max(widest, m.Width(texts[i], legendSize, base.Regular))
	}
	side := widest + pieElbow + pieRun + gap/2
	withLabels := math.Min(plotW/2-side, plotH/2-pieElbow-pieRow/2)
	labelled := withLabels >= pieRMin
	r := math.Max(math.Min(plotW, plotH)/2, 1)
	if labelled {
		r = withLabels
	}
	r0 := 0.0
	if hole {
		r0 = r * donutHole
	}

	cum := 0.0
	var sides [2][]pieLabel
	for i, b := range data {
		a0, a1 := tau*cum/total, tau*(cum+b.Value)/total
		cum += b.Value
		if b.Value == 0 {
			continue
		}
		marks = append(marks, scene.Mark{
			Shape:  scene.PathShape{Ops: sector(cx, cy, r0, r, a0, a1)},
			Fill:   base.Series(i),
			Stroke: &scene.Stroke{Color: base.Surface, Width: surfaceGap},
			Series: b.Label,
			Hit: &scene.Hit{
				Key:   strconv.Itoa(i),
				Label: b.Label,
				Value: base.Value(b.Value),
				Rows:  []scene.TipRow{{Name: "share", Value: base.Approx(b.Value/total*100) + "%"}},
			},
		})
		mid := (a0 + a1) / 2
		sn, cs := dmath.Sincos(mid)
		s := 0
		if sn < 0 {
			s = 1
		}
		sides[s] = append(sides[s], pieLabel{slice: i, mid: mid, y: cy - float64((r+pieElbow)*cs)})
	}

	if labelled {
		// Labels always fit: at most maxSeries slices, and a pie big enough to
		// be labelled (pieRMin) leaves more rows than that on each side.
		for s, labels := range sides {
			sort.SliceStable(labels, func(a, b int) bool { return totalLess(labels[a].y, labels[b].y) })
			spread(labels, top+pieRow/2, bottom-pieRow/2)
			dir, anchor := 1.0, scene.Start
			if s == 1 {
				dir, anchor = -1, scene.End
			}
			tx := cx + float64(dir*(r+pieElbow+pieRun))
			for _, l := range labels {
				name := data[l.slice].Label
				x1, y1 := polarAt(cx, cy, r+2, l.mid)
				x2, y2 := polarAt(cx, cy, r+pieElbow, l.mid)
				marks = append(marks, scene.Mark{
					Shape:  scene.PolylineShape{Points: []scene.Point{{X: x1, Y: y1}, {X: x2, Y: y2}, {X: tx - dir*gap/2, Y: l.y}}},
					Stroke: &scene.Stroke{Color: base.Axis, Width: hairline},
					Series: name,
				})
				t := text(tx, l.y, texts[l.slice], legendSize, base.Regular, anchor, scene.Center, base.TextSecondary)
				t.Series = name
				marks = append(marks, t)
			}
		}
	}

	if hole && len(data) > 0 {
		label := base.Value(total)
		if m.Width(label, totalSize, base.Semibold) <= 1.6*r0 {
			marks = append(marks, text(cx, cy, label, totalSize, base.Semibold, scene.Middle, scene.Bottom, base.TextPrimary))
			marks = append(marks, text(cx, cy+gap, "Total", tickSize, base.Regular, scene.Middle, scene.Top, base.TextMuted))
		}
	}

	what := fmt.Sprintf("%s chart of %s.", kind, count(len(data), "slice", "slices"))
	if hole {
		what = fmt.Sprintf("%s chart of %s, total %s.", kind, count(len(data), "slice", "slices"), base.Value(total))
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(title, what),
		Marks:  marks,
	}, nil
}
