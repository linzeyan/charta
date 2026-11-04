package charta

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/linzeyan/charta/scene"
)

type wordBox struct {
	text string
	size float64
	box  scene.Rect
}

func wordBoxes(s *scene.Scene) []wordBox {
	var out []wordBox
	for _, m := range s.Marks {
		if tx, ok := m.Shape.(scene.TextShape); ok && m.Hit != nil {
			out = append(out, wordBox{tx.Text, tx.Size, *m.Hit.Area})
		}
	}
	return out
}

func noOverlaps(t *testing.T, b []wordBox) {
	t.Helper()
	for i, x := range b {
		for _, y := range b[i+1:] {
			if overlaps(x.box, y.box) {
				t.Fatalf("%s and %s overlap", x.text, y.text)
			}
		}
	}
}

func TestWordsNeverOverlapAndTheHeaviestTakesTheMiddle(t *testing.T) {
	var c WordCloudChart
	for i, w := range []string{"data", "chart", "axis", "scale", "color", "mark", "legend"} {
		c.Words = append(c.Words, Word{w, 70 - 10*float64(i)})
	}
	b := wordBoxes(build(t, c))
	if len(b) != 7 {
		t.Fatalf("%d words placed", len(b))
	}
	noOverlaps(t, b)
	first := b[0]
	if first.text != "data" {
		t.Fatalf("first placed %q", first.text)
	}
	mx, my := first.box.X+first.box.W/2, first.box.Y+first.box.H/2
	if math.Abs(mx-320) > 1e-9 || !(my > 100 && my < 300) {
		t.Fatalf("data centered at (%v, %v)", mx, my)
	}
	for k := 1; k < len(b); k++ {
		if b[k-1].size < b[k].size {
			t.Fatalf("sizes do not follow weight: %v then %v", b[k-1].size, b[k].size)
		}
	}
	// Area tracks weight: a quarter of the weight, half the font size, until
	// the floor.
	if b[0].size != wordMaxFont {
		t.Fatalf("the heaviest word is %v px", b[0].size)
	}
	// A short plot caps the biggest word at a quarter of its height.
	short := wordBoxes(buildAt(t, WordCloudChart{Words: []Word{{"a", 1}}}, 640, 120))
	if short[0].size != (120-2*pad)/4 {
		t.Fatalf("in a short plot the word is %v px", short[0].size)
	}
	q := wordBoxes(build(t, WordCloudChart{Words: []Word{{"a", 100}, {"b", 25}, {"c", 0.01}}}))
	if !near(q[1].size, wordMaxFont/2) || q[2].size != wordMinFont {
		t.Fatalf("sizes %v %v", q[1].size, q[2].size)
	}
}

func TestACrowdedCloudShrinksToKeepEveryWord(t *testing.T) {
	// At full size these words overflow the plot; the cloud is set smaller
	// instead, so no word is lost and the weights still rank.
	var c WordCloudChart
	for i := range 40 {
		c.Words = append(c.Words, Word{fmt.Sprintf("term%d", i), 100 - float64(i)*2})
	}
	s := buildAt(t, c, 320, 200)
	b := wordBoxes(s)
	if len(b) != 40 || strings.Contains(s.Label, "did not fit") {
		t.Fatalf("%d placed, label %q", len(b), s.Label)
	}
	// Alone, the heaviest word has room for its full size.
	full := wordBoxes(buildAt(t, WordCloudChart{Words: []Word{{"term0", 100}}}, 320, 200))[0].size
	if b[0].size >= full {
		t.Fatalf("the largest word kept its %v px", full)
	}
	for k := 1; k < len(b); k++ {
		if b[k-1].size < b[k].size {
			t.Fatalf("sizes do not follow weight: %v then %v", b[k-1].size, b[k].size)
		}
	}
	noOverlaps(t, b)
}

func TestWordsThatDoNotFitAreCountedNotOverlapped(t *testing.T) {
	var c WordCloudChart
	for i := range 120 {
		c.Words = append(c.Words, Word{fmt.Sprintf("word%d", i), 100 - float64(i)*0.5})
	}
	s := buildAt(t, c, 240, 160)
	b := wordBoxes(s)
	if len(b) >= 120 || !strings.Contains(s.Label, fmt.Sprintf("%d more did not fit", 120-len(b))) {
		t.Fatalf("%d placed, label %q", len(b), s.Label)
	}
	noOverlaps(t, b)
	for _, w := range b {
		if w.box.X < pad || w.box.X+w.box.W > 240-pad || w.box.Y < pad || w.box.Y+w.box.H > 160-pad {
			t.Fatalf("%s spills out of the plot: %+v", w.text, w.box)
		}
	}
}

func TestWordCloudRejectsBadWords(t *testing.T) {
	rejects(t, WordCloudChart{Words: []Word{{"a", 0}}})
	rejects(t, WordCloudChart{Words: []Word{{" \t", 1}}})
	rejects(t, WordCloudChart{Words: []Word{{"a", 1}, {"a", 2}}})
	rejects(t, WordCloudChart{Words: []Word{{"a", math.NaN()}}})
	var many WordCloudChart
	for i := range maxWords + 1 {
		many.Words = append(many.Words, Word{fmt.Sprint(i), 1})
	}
	rejects(t, many)
}
