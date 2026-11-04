package raster

import (
	"image/color"
	"testing"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/scene"
)

func near(a color.NRGBA, b base.Rgba, tol int) bool {
	d := func(x, y uint8) bool { return max(int(x)-int(y), int(y)-int(x)) <= tol }
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B)
}

func goFonts(t *testing.T) *Fonts {
	t.Helper()
	parse := func(b []byte) *sfnt.Font {
		f, err := sfnt.Parse(b)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	return NewFonts(parse(goregular.TTF), parse(gobold.TTF), nil)
}

// TestTooltipRowsLeadWithTheValueOnlyWhenThereIsOne pins the shared
// tooltip rule: an empty value must not leave a blank first line, only
// series rows carry a key, and hidden series drop out of the rows.
func TestTooltipRowsLeadWithTheValueOnlyWhenThereIsOne(t *testing.T) {
	summary := &scene.Hit{Label: "a", Rows: []scene.TipRow{{Name: "median", Value: "2"}, {Name: "values", Value: "3"}}}
	got := tipRows(summary, nil)
	if len(got) != 3 || got[0].weak != "a" || got[0].strong != "" || got[1].key != base.NoColor || got[1].strong != "2" {
		t.Fatalf("summary rows %+v", got)
	}
	mark := &scene.Hit{Label: "Pears", Value: "18"}
	if got := tipRows(mark, nil); len(got) != 2 || got[0].strong != "18" || got[1].weak != "Pears" {
		t.Fatalf("mark rows %+v", got)
	}
	cross := &scene.Hit{Label: "Apr", Kind: scene.HitCrosshair, Rows: []scene.TipRow{{Color: base.Series(0), Name: "Web", Value: "1"}, {Color: base.Series(1), Name: "iOS", Value: "2"}}}
	if got := tipRows(cross, []string{"iOS"}); len(got) != 2 || got[1].key != base.Series(0) || got[1].weak != "Web" {
		t.Fatalf("crosshair rows %+v", got)
	}
}

// TestRectOutlinesAreDrawn: treemap cells and mind-map pills are outlined
// rects; an outline the rasterizer dropped would leave them unbounded.
func TestRectOutlinesAreDrawn(t *testing.T) {
	sc := &scene.Scene{Width: 40, Height: 40, Marks: []scene.Mark{{
		Shape:  scene.RectShape{Rect: scene.Rect{X: 10, Y: 10, W: 20, H: 20}},
		Stroke: &scene.Stroke{Color: base.Series(1), Width: 2},
	}}}
	img := Paint(sc, NewFonts(nil, nil, nil), -1, Pointer{}, nil, 40, 40, 1, false)
	edge := base.Resolve(base.Series(1), false)
	if got := color.NRGBAModel.Convert(img.RGBAAt(10, 20)).(color.NRGBA); !near(got, edge, 1) {
		t.Fatalf("left edge is %v, want the outline %v", got, edge)
	}
	if got := color.NRGBAModel.Convert(img.RGBAAt(20, 20)).(color.NRGBA); near(got, edge, 1) {
		t.Fatalf("an unfilled rect's interior is %v, the outline color", got)
	}
}

// TestFontsMeasureWhatTheyDraw: the scene is laid out with Width and the
// painter places glyphs with the same advances; a semibold face that fell
// back to regular, or glyphs left as gaps, would show here.
func TestFontsMeasureWhatTheyDraw(t *testing.T) {
	f := goFonts(t)
	if i, w := f.Width("i", 12, base.Regular), f.Width("W", 12, base.Regular); w < 1.5*i {
		t.Fatalf("i=%v W=%v: not a proportional font", i, w)
	}
	if r, b := f.Width("Revenue", 12, base.Regular), f.Width("Revenue", 12, base.Semibold); b <= r {
		t.Fatalf("semibold %v is not wider than regular %v", b, r)
	}
	if got := f.Width("ab", 24, base.Regular); got != 2*f.Width("ab", 12, base.Regular) {
		t.Fatalf("width does not scale with size: %v", got)
	}
	gaps := NewFonts(nil, nil, nil)
	if got := gaps.Width("abc", 10, base.Regular); got != 18 {
		t.Fatalf("three missing glyphs at 10px are %v wide, want 3 gaps of 6", got)
	}
	if f.capHeight == base.CapHeight || f.capHeight <= 0.5 || f.capHeight >= 1 {
		t.Fatalf("cap height %v did not come from the face", f.capHeight)
	}
}
