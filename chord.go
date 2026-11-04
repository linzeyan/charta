package charta

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

const (
	chordRing = 12.0
	// chordPadAngle is the gap between neighboring groups on the ring, in
	// radians.
	chordPadAngle = 0.04
	// chordMaxStroke: link widths in the non-ribbon chord run from 1px to
	// this.
	chordMaxStroke = 4.0
)

// ChordChart shows flows between a handful of groups: each group an arc as
// long as all it sends and receives, each pair a ribbon as wide as their
// exchange.
type ChordChart struct {
	Title string `json:"title,omitempty"`
	Links []Link `json:"links"`
}

func (c ChordChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	g, err := graph(c.Links, true)
	if err != nil {
		return nil, err
	}
	n := len(g.names)
	// Ribbons are told apart by their group's color alone.
	if n > maxSeries {
		return nil, invalid(`%d groups, at most %d: fold the rest into "Other"`, n, maxSeries)
	}
	flow := make([][]float64, n)
	for i := range flow {
		flow[i] = make([]float64, n)
	}
	for _, e := range g.edges {
		flow[e.S][e.T] = e.V
	}
	// A group's arc covers everything it exchanges, both directions; a
	// pair's ribbon is as wide as their two flows together at both ends.
	pair := func(i, j int) float64 { return flow[i][j] + flow[j][i] }
	totals := make([]float64, n)
	grand := 0.0
	for i := range n {
		for j := range n {
			totals[i] += pair(i, j)
		}
		grand += totals[i]
	}

	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, g.legend(), keySwatch)
	roomX, roomY := labelRoom(m, g.names)
	cx := width / 2
	cy := (top + height - pad) / 2
	r := math.Max(math.Min(width/2-pad-roomX, (height-pad-top)/2-roomY), chordRing+1)
	rIn := r - chordRing
	rr := rIn - surfaceGap

	// Each group's arc is split among its partners, nearest counterclockwise
	// first, so ribbons to neighbors leave from the facing ends and the arcs
	// do not twist across each other.
	k := 0.0
	if grand > 0 {
		k = (tau - float64(chordPadAngle*float64(n))) / grand
	}
	group := make([][2]float64, n)
	end := make([][][2]float64, n)
	a := chordPadAngle / 2.0
	for i := range n {
		end[i] = make([][2]float64, n)
		a0 := a
		var partners []int
		for j := range n {
			if j != i {
				partners = append(partners, j)
			}
		}
		slices.SortStableFunc(partners, func(p, q int) int { return cmp.Compare((i+n-p)%n, (i+n-q)%n) })
		for _, j := range partners {
			end[i][j] = [2]float64{a, a + float64(pair(i, j)*k)}
			a += float64(pair(i, j) * k)
		}
		group[i] = [2]float64{a0, a}
		a += chordPadAngle
	}

	for i := range n {
		for j := i + 1; j < n; j++ {
			if pair(i, j) == 0 {
				continue
			}
			// Colored by whichever sends more; a tie goes to the first.
			lead := i
			if flow[j][i] > flow[i][j] {
				lead = j
			}
			s0, s1 := end[i][j][0], end[i][j][1]
			t0, t1 := end[j][i][0], end[j][i][1]
			sx, sy := polarAt(cx, cy, rr, s0)
			ops := []scene.PathOp{scene.Move(sx, sy)}
			ops = arc(ops, cx, cy, rr, s0, s1)
			fx, fy := polarAt(cx, cy, rr, s1)
			tx, ty := polarAt(cx, cy, rr, t0)
			ops = quad(ops, fx, fy, cx, cy, tx, ty)
			ops = arc(ops, cx, cy, rr, t0, t1)
			fx, fy = polarAt(cx, cy, rr, t1)
			ops = quad(ops, fx, fy, cx, cy, sx, sy)
			ops = append(ops, scene.Close())
			row := func(from, to int) scene.TipRow {
				return scene.TipRow{
					Color: base.Series(from),
					Name:  g.names[from] + " → " + g.names[to],
					Value: base.Value(flow[from][to]),
				}
			}
			marks = append(marks, scene.Mark{
				Shape:  scene.PathShape{Ops: ops},
				Fill:   base.Link(lead),
				Series: g.names[lead],
				Hit: &scene.Hit{
					Key:   fmt.Sprintf("ribbon-%d-%d", i, j),
					Label: g.names[i] + " ↔ " + g.names[j],
					Value: base.Value(pair(i, j)),
					Rows:  []scene.TipRow{row(i, j), row(j, i)},
				},
			})
		}
	}
	for i := range n {
		a0, a1 := group[i][0], group[i][1]
		out, inc := 0.0, 0.0
		for j := range n {
			out += flow[i][j]
		}
		for j := range n {
			inc += flow[j][i]
		}
		marks = append(marks, scene.Mark{
			Shape:  scene.PathShape{Ops: sector(cx, cy, rIn, r, a0, a1)},
			Fill:   base.Series(i),
			Series: g.names[i],
			Hit: &scene.Hit{
				Key:   fmt.Sprintf("group-%d", i),
				Label: g.names[i],
				Value: base.Value(totals[i]),
				Rows:  []scene.TipRow{{Name: "out", Value: base.Value(out)}, {Name: "in", Value: base.Value(inc)}},
			},
		})
		x, y, anchor, baseline := around(cx, cy, r+gap/2, (a0+a1)/2)
		marks = append(marks, text(x, y, g.names[i], tickSize, base.Regular, anchor, baseline, base.TextSecondary))
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(c.Title, fmt.Sprintf("Chord diagram of %s among %s.",
			count(len(g.edges), "flow", "flows"), count(n, "group", "groups"))),
		Marks: marks,
	}, nil
}

// NonRibbonChordChart shows connections between nodes on a circle, drawn
// as curves through the middle: which nodes connect, not how much flows.
type NonRibbonChordChart struct {
	Title string `json:"title,omitempty"`
	Links []Link `json:"links"`
}

func (c NonRibbonChordChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	g, err := graph(c.Links, false)
	if err != nil {
		return nil, err
	}
	n := len(g.names)
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	top = legend(&marks, m, top, width, g.legend(), keySwatch)
	// Labels run outward along the radius, so however many nodes there are
	// each needs only a line's width of the circumference.
	room := dotRadius + gap/2 + maxWidth(m, g.names)
	cx := width / 2
	cy := (top + height - pad) / 2
	r := math.Min(width/2-pad, (height-pad-top)/2) - room
	r = math.Max(r, dotRadius+1)
	angle := func(i int) float64 { return float64(i) * tau / float64(n) }
	strength := g.strength()
	maxV := 0.0
	for _, e := range g.edges {
		maxV = math.Max(maxV, e.V)
	}

	for k, e := range g.edges {
		px, py := polarAt(cx, cy, r, angle(e.S))
		qx, qy := polarAt(cx, cy, r, angle(e.T))
		ops := quad([]scene.PathOp{scene.Move(px, py)}, px, py, cx, cy, qx, qy)
		marks = append(marks, scene.Mark{
			Shape:  scene.PathShape{Ops: ops},
			Stroke: &scene.Stroke{Color: base.Link(g.color(e.S)), Width: 1 + (chordMaxStroke-1)*e.V/maxV},
			Series: g.series(e.S),
			Hit: &scene.Hit{
				Key:   fmt.Sprintf("link-%d", k),
				Label: g.names[e.S] + " – " + g.names[e.T],
				Value: base.Value(e.V),
			},
		})
	}
	for i, name := range g.names {
		a := angle(i)
		x, y := polarAt(cx, cy, r, a)
		marks = append(marks, scene.Mark{
			Shape:  scene.CircleShape{CX: x, CY: y, R: dotRadius},
			Fill:   base.Series(g.color(i)),
			Stroke: &scene.Stroke{Color: base.Surface, Width: surfaceGap},
			Series: g.series(i),
			Hit: &scene.Hit{
				Key:   fmt.Sprintf("node-%d", i),
				Label: name,
				Value: base.Value(strength[i]),
				Rows:  []scene.TipRow{{Name: "links", Value: strconv.Itoa(g.links(i))}},
			},
		})
		// Right half reads outward left to right; the left half is flipped
		// so no label is upside down.
		lx, ly := polarAt(cx, cy, r+dotRadius+gap/2, a)
		deg := a * 180 / math.Pi
		anchor, rot := scene.Start, deg-90
		if a > math.Pi {
			anchor, rot = scene.End, deg-270
		}
		t := text(lx, ly, name, tickSize, base.Regular, anchor, scene.Center, base.TextSecondary)
		ts := t.Shape.(scene.TextShape)
		ts.Angle = rot
		t.Shape = ts
		marks = append(marks, t)
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label: describe(c.Title, fmt.Sprintf("Non-ribbon chord diagram of %s among %s.",
			count(len(g.edges), "link", "links"), count(n, "node", "nodes"))),
		Marks: marks,
	}, nil
}
