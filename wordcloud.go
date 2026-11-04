package charta

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

const (
	// Font sizes run from wordMinFont up to wordMaxFont, or a quarter of the
	// plot's height when that is smaller.
	wordMinFont = 11.0
	wordMaxFont = 48.0
	// wordPad is the air around each word's box.
	wordPad = 2.0
	// spiralStep is the spiral's vertical step; the horizontal one is scaled
	// by the plot's aspect so the cloud fills a wide plot instead of a circle
	// in its middle.
	spiralStep = 4.0
	maxWords   = 200
	// wordShrink scales every size down when words do not fit, and the cloud
	// is laid out again, until all fit or the largest word is at wordMinFont.
	wordShrink = 0.85
)

// Word is a word and how much it counts.
type Word struct {
	Text   string  `json:"text"`
	Weight float64 `json:"weight"`
}

// WordCloudChart sizes words by weight and packs them around the center:
// heaviest first, each at the first free spot on a spiral out from the
// middle.
type WordCloudChart struct {
	Title string `json:"title,omitempty"`
	Words []Word `json:"words"`
}

func overlaps(a, b scene.Rect) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}

// placedWord is one word placed: its index, font size and box.
type placedWord struct {
	i    int
	size float64
	box  scene.Rect
}

// placeWords places the words in order with the largest at top px, each at
// the first free spot on the spiral; the words that found none are counted.
func placeWords(words []Word, order []int, top float64, plot scene.Rect, m base.TextMeasure) ([]placedWord, int) {
	cx, cy := plot.X+plot.W/2, plot.Y+plot.H/2
	heaviest := 0.0
	for _, w := range words {
		heaviest = math.Max(heaviest, w.Weight)
	}
	// A square spiral of legs 1, 1, 2, 2, 3, 3… steps (right, down, left,
	// up): integer step counts, so both ports visit exactly the same spots,
	// with no trigonometry to differ in the last bit.
	dx, dy := spiralStep*plot.W/plot.H, spiralStep
	// A leg of n steps reaches about n/2 steps from the center, and the plot
	// is h/dy steps across either way (dx scales with the aspect).
	legs := int64(math.Ceil(plot.H/dy)) + 2
	dirs := [4][2]int64{{1, 0}, {0, 1}, {-1, 0}, {0, -1}}
	var placed []placedWord
	dropped := 0
	for _, i := range order {
		w := words[i]
		// Area tracks weight: the side of a word's box grows with the square
		// root.
		size := math.Max(float64(top*math.Sqrt(w.Weight/heaviest)), wordMinFont)
		bw, bh := m.Width(w.Text, size, base.Semibold)+2*wordPad, size+2*wordPad
		fits := func(sx, sy int64) (scene.Rect, bool) {
			r := scene.Rect{
				X: cx + float64(float64(sx)*dx) - bw/2,
				Y: cy + float64(float64(sy)*dy) - bh/2,
				W: bw,
				H: bh,
			}
			inside := r.X >= plot.X && r.Y >= plot.Y && r.X+r.W <= plot.X+plot.W && r.Y+r.H <= plot.Y+plot.H
			if !inside {
				return r, false
			}
			for _, p := range placed {
				if overlaps(p.box, r) {
					return r, false
				}
			}
			return r, true
		}
		spot, ok := fits(0, 0)
		var sx, sy int64
	spiral:
		for leg := int64(1); leg <= 2*legs && !ok; leg++ {
			u := dirs[(leg-1)%4]
			for range (leg + 1) / 2 {
				sx += u[0]
				sy += u[1]
				if spot, ok = fits(sx, sy); ok {
					break spiral
				}
			}
		}
		if ok {
			placed = append(placed, placedWord{i, size, spot})
		} else {
			// Smaller words come later and may still fit.
			dropped++
		}
	}
	return placed, dropped
}

func (c WordCloudChart) Scene(width, height float64, m base.TextMeasure) (*scene.Scene, error) {
	if err := checkSize(width, height); err != nil {
		return nil, err
	}
	if len(c.Words) > maxWords {
		return nil, invalid("%d words, at most %d: a cloud of more is noise", len(c.Words), maxWords)
	}
	for i, w := range c.Words {
		if err := finite(fmt.Sprintf("%q's weight", w.Text), w.Weight); err != nil {
			return nil, err
		}
		if w.Weight <= 0 || strings.TrimSpace(w.Text) == "" {
			return nil, invalid("word %d needs text and a positive weight", i)
		}
		for _, o := range c.Words[:i] {
			if o.Text == w.Text {
				return nil, invalid("%q is listed twice", w.Text)
			}
		}
	}
	var marks []scene.Mark
	top := drawTitle(&marks, c.Title)
	plot := scene.Rect{X: pad, Y: top, W: math.Max(width-2*pad, 1), H: math.Max(height-pad-top, 1)}
	maxFont := math.Max(math.Min(wordMaxFont, plot.H/4), wordMinFont)
	order := make([]int, len(c.Words))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return totalLess(c.Words[order[b]].Weight, c.Words[order[a]].Weight) })

	// Shrink the whole cloud rather than lose words: sizes keep their ratios
	// down to the wordMinFont floor. Of the tries, the one that drops fewest
	// wins, the larger on a tie; what still does not fit is left to the count
	// in the accessible name.
	largest := maxFont
	placed, dropped := placeWords(c.Words, order, largest, plot, m)
	for dropped > 0 && largest > wordMinFont {
		largest *= wordShrink
		if p, d := placeWords(c.Words, order, largest, plot, m); d < dropped {
			placed, dropped = p, d
		}
	}
	for _, p := range placed {
		w := c.Words[p.i]
		mk := text(p.box.X+p.box.W/2, p.box.Y+p.box.H/2, w.Text, p.size, base.Semibold, scene.Middle, scene.Center, base.Series(0))
		area := p.box
		mk.Hit = &scene.Hit{Key: strconv.Itoa(p.i), Label: w.Text, Value: base.Value(w.Weight), Area: &area}
		marks = append(marks, mk)
	}
	what := "Word cloud of " + count(len(c.Words)-dropped, "word", "words")
	if dropped > 0 {
		what += fmt.Sprintf("; %d more did not fit", dropped)
	}
	return &scene.Scene{
		Width:  width,
		Height: height,
		Label:  describe(c.Title, what+"."),
		Marks:  marks,
	}, nil
}
