package charta

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

const (
	parallelSetsBarW = 8.0
	// parallelSetsCatGap is the room between categories on an axis: more
	// than a label's height, so labels of neighboring small categories
	// cannot collide.
	parallelSetsCatGap = 16.0
)

// Combination is how many items share one category on every dimension.
type Combination struct {
	Categories []string `json:"categories"`
	Count      float64  `json:"count"`
}

// ParallelSetsChart puts categorical dimensions side by side, each split by
// category share, with ribbons tracing how items in one dimension's
// categories divide across the next. Ribbons keep the first dimension's
// color all the way along.
type ParallelSetsChart struct {
	Title        string        `json:"title,omitempty"`
	Dimensions   []string      `json:"dimensions"`
	Combinations []Combination `json:"combinations"`
}

// index returns each dimension's categories in order of first appearance,
// and each combination's category index per dimension.
func (c ParallelSetsChart) index() ([][]string, [][]int, error) {
	d := len(c.Dimensions)
	if d < 2 {
		return nil, nil, invalid("%s, at least 2: there is nothing to trace across", count(d, "dimension", "dimensions"))
	}
	if len(c.Combinations) == 0 {
		return nil, nil, invalid("no combinations: there is nothing to trace")
	}
	cats := make([][]string, d)
	idx := make([][]int, 0, len(c.Combinations))
	for k, co := range c.Combinations {
		if len(co.Categories) != d {
			return nil, nil, invalid("combination %d has %d categories for %d dimensions", k, len(co.Categories), d)
		}
		if !(isFinite(co.Count) && co.Count > 0) {
			return nil, nil, invalid("combination %d needs a positive count", k)
		}
		for _, o := range c.Combinations[:k] {
			if slices.Equal(o.Categories, co.Categories) {
				quoted := make([]string, d)
				for j, s := range co.Categories {
					quoted[j] = fmt.Sprintf("%q", s)
				}
				return nil, nil, invalid("combination [%s] is given twice", strings.Join(quoted, ", "))
			}
		}
		row := make([]int, d)
		for j := range d {
			i := slices.Index(cats[j], co.Categories[j])
			if i < 0 {
				cats[j] = append(cats[j], co.Categories[j])
				i = len(cats[j]) - 1
			}
			row[j] = i
		}
		idx = append(idx, row)
	}
	// The first dimension's categories color every ribbon.
	if len(cats[0]) > maxSeries {
		return nil, nil, invalid(`%d categories in %q, at most %d: it colors the ribbons; fold the rest into "Other" or put another dimension first`,
			len(cats[0]), c.Dimensions[0], maxSeries)
	}
	return cats, idx, nil
}

func (c ParallelSetsChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	cats, idx, err := c.index()
	if err != nil {
		return nil, err
	}
	d := len(c.Dimensions)
	total := 0.0
	for _, co := range c.Combinations {
		total += co.Count
	}
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, cats[0], keySwatch)

	// Dimension names above, then the axes; first-axis labels to its left,
	// the rest to each axis's right.
	namesY := top
	plotTop := top + tickSize + gap
	bottom := math.Max(height-pad, plotTop+1)
	left := pad + maxWidth(m, cats[0]) + gap/2
	right := math.Max(width-pad-maxWidth(m, cats[d-1])-gap/2, left+parallelSetsBarW)
	axisX := func(j int) float64 {
		return left + float64(j)*(right-left-parallelSetsBarW)/float64(d-1)
	}
	most := 1
	if len(cats) > 0 {
		most = 0
		for _, cs := range cats {
			most = max(most, len(cs))
		}
	}
	ky := 0.0
	if total > 0 {
		ky = math.Max((bottom-plotTop-float64(float64(most-1)*parallelSetsCatGap))/total, 0)
	}

	// Category extents; each axis centered vertically.
	sums := make([][]float64, d)
	for j := range d {
		sums[j] = make([]float64, len(cats[j]))
	}
	for k, co := range c.Combinations {
		for j := range d {
			sums[j][idx[k][j]] += co.Count
		}
	}
	catY := make([][]float64, d)
	for j := range d {
		h := float64(total*ky) + float64(float64(len(cats[j])-1)*parallelSetsCatGap)
		y := plotTop + (bottom-plotTop-h)/2
		ys := make([]float64, 0, len(cats[j]))
		for _, s := range sums[j] {
			ys = append(ys, y)
			y += float64(s*ky) + parallelSetsCatGap
		}
		catY[j] = ys
	}

	// Ribbons between neighboring axes, one per (first category, left
	// category, right category). On the left they stack by right category,
	// on the right by left category, so ribbons from one category stay
	// together and cross as little as the data allows; the first
	// dimension's category orders within that.
	type ribbon struct {
		first, a, b int
		v           float64
	}
	for j := range d - 1 {
		var groups []ribbon
		for k, co := range c.Combinations {
			row := idx[k]
			at := slices.IndexFunc(groups, func(g ribbon) bool {
				return g.first == row[0] && g.a == row[j] && g.b == row[j+1]
			})
			if at >= 0 {
				groups[at].v += co.Count
			} else {
				groups = append(groups, ribbon{row[0], row[j], row[j+1], co.Count})
			}
		}
		byLeft := make([]int, len(groups))
		byRight := make([]int, len(groups))
		for g := range groups {
			byLeft[g], byRight[g] = g, g
		}
		slices.SortStableFunc(byLeft, func(p, q int) int {
			x, y := groups[p], groups[q]
			return cmp.Or(cmp.Compare(x.a, y.a), cmp.Compare(x.b, y.b), cmp.Compare(x.first, y.first))
		})
		slices.SortStableFunc(byRight, func(p, q int) int {
			x, y := groups[p], groups[q]
			return cmp.Or(cmp.Compare(x.b, y.b), cmp.Compare(x.a, y.a), cmp.Compare(x.first, y.first))
		})
		yLeft := make([]float64, len(groups))
		yRight := make([]float64, len(groups))
		fill := slices.Clone(catY[j])
		for _, g := range byLeft {
			a := groups[g].a
			yLeft[g] = fill[a]
			fill[a] += float64(groups[g].v * ky)
		}
		fill = slices.Clone(catY[j+1])
		for _, g := range byRight {
			b := groups[g].b
			yRight[g] = fill[b]
			fill[b] += float64(groups[g].v * ky)
		}
		for g, rb := range groups {
			w := math.Max(rb.v*ky, 1)
			marks = append(marks, scene.Mark{
				Shape:  scene.PathShape{Ops: flowBand(axisX(j)+parallelSetsBarW, yLeft[g], axisX(j+1), yRight[g], w)},
				Fill:   base.Link(rb.first),
				Series: cats[0][rb.first],
				Hit: &scene.Hit{
					Key:   fmt.Sprintf("ribbon-%d-%d", j, g),
					Label: cats[j][rb.a] + " → " + cats[j+1][rb.b],
					Value: base.Value(rb.v),
					Rows: []scene.TipRow{
						{Color: base.Series(rb.first), Name: c.Dimensions[0], Value: cats[0][rb.first]},
						{Name: "of all", Value: base.Approx(rb.v/total*100) + "%"},
					},
				},
			})
		}
	}

	for j := range d {
		x := axisX(j)
		anchor, nx := scene.Middle, x+parallelSetsBarW/2
		switch j {
		case 0:
			anchor, nx = scene.Start, x
		case d - 1:
			anchor, nx = scene.End, x+parallelSetsBarW
		}
		marks = append(marks, text(nx, namesY, c.Dimensions[j], tickSize, base.Semibold, anchor, scene.Top, base.TextPrimary))
		for i, cat := range cats[j] {
			y, h := catY[j][i], math.Max(sums[j][i]*ky, 1)
			// The first axis wears the ribbon colors, so its bars are the
			// color key; the others stay neutral.
			fillColor, series := base.TextMuted, ""
			if j == 0 {
				fillColor, series = base.Series(i), cat
			}
			marks = append(marks, scene.Mark{
				Shape:  scene.RectShape{Rect: scene.Rect{X: x, Y: y, W: parallelSetsBarW, H: h}},
				Fill:   fillColor,
				Series: series,
				Hit: &scene.Hit{
					Key:   fmt.Sprintf("cat-%d-%d", j, i),
					Label: c.Dimensions[j] + ": " + cat,
					Value: base.Value(sums[j][i]),
					Rows:  []scene.TipRow{{Name: "of all", Value: base.Approx(sums[j][i]/total*100) + "%"}},
				},
			})
			cy := y + h/2
			switch j {
			case 0:
				marks = append(marks, text(x-gap/2, cy, cat, tickSize, base.Regular, scene.End, scene.Center, base.TextSecondary))
			case d - 1:
				marks = append(marks, text(x+parallelSetsBarW+gap/2, cy, cat, tickSize, base.Regular, scene.Start, scene.Center, base.TextSecondary))
			default:
				haloLabel(&marks, m, x+parallelSetsBarW+gap/2, cy, cat, scene.Start, base.TextSecondary)
			}
		}
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(c.Title, fmt.Sprintf("Parallel sets of %s across %s.",
			base.Value(total), count(d, "dimension", "dimensions"))),
		Marks: marks,
	}, nil
}
