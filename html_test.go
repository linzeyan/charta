package charta

import (
	"strings"
	"testing"

	"github.com/linzeyan/charta/html"
)

func twoLines() LineChart {
	return LineChart{
		Categories: []string{"Jan", "Feb"},
		Series:     []Series{{"Web", []float64{120, 135}}, {"iOS", []float64{80, 95}}},
	}
}

func TestHTMLCrosshairCarriesEverySeriesForTheRuntimeAndScreenReaders(t *testing.T) {
	page, err := RenderHTML(twoLines(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		// The runtime builds the multi-row tooltip from data-rows.
		`class="ch-m ch-x" data-key="x0" data-tip-label="Jan" data-rows="[{&quot;c&quot;:&quot;series-1&quot;,&quot;n&quot;:&quot;Web&quot;,&quot;v&quot;:&quot;120&quot;},{&quot;c&quot;:&quot;series-2&quot;,&quot;n&quot;:&quot;iOS&quot;,&quot;v&quot;:&quot;80&quot;}]"`,
		`aria-label="Jan: Web 120, iOS 80"`,
		`<div class="ch-rows"></div>`,
		// Hovering without a pointer is impossible; the table has every value.
		`<tr><th scope="row">Jan · iOS</th><td>80</td></tr>`,
		// The legend toggles its series, so it is a pressed button.
		`class="ch-m ch-leg" data-key="legend-1" data-series="iOS" tabindex="0" role="button" aria-pressed="true" aria-label="iOS"`,
		// Undecorated series marks still say which series they draw, so the
		// runtime can hide and dim them.
		`data-series="Web"/>`,
		"stroke-linejoin:round;stroke-linecap:round",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %s", want)
		}
	}
}

func TestHTMLSummaryMarksCarryUnkeyedRowsAndNoEmptyValue(t *testing.T) {
	page, err := RenderHTML(BoxPlotChart{Groups: []Sample{{"a", []float64{1, 2, 3, 4, 5, 40}}}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		// No value of its own: the runtime must not show an empty lead line,
		// and statistic rows have no series key to paint.
		`class="ch-m" data-key="0" data-tip-label="a" data-rows="[{&quot;n&quot;:&quot;upper whisker&quot;,&quot;v&quot;:&quot;5&quot;},`,
		`aria-label="a: upper whisker 5, Q3 4.75, median 3.5, Q1 2.25, lower whisker 1, outliers 1, values 6"`,
		// The table lists the statistics, without a blank row for the group.
		`<tr><th scope="row">a · median</th><td>3.5</td></tr>`,
		// A data mark keeps its value attribute and spoken value.
		`data-tip-label="a · outlier" data-tip-value="40" tabindex="0" role="img" aria-label="a · outlier: 40"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %s", want)
		}
	}
	if strings.Contains(page, `<tr><th scope="row">a</th><td></td></tr>`) {
		t.Error("summary mark left an empty table row")
	}
}

func TestSVGDropsCrosshairsButKeepsLegendAndTitles(t *testing.T) {
	svg, err := RenderHTML(twoLines(), Options{Format: html.FormatSVG})
	if err != nil {
		t.Fatal(err)
	}
	// A static file has no hover to reveal a crosshair with.
	if strings.Contains(svg, "ch-x") || strings.Contains(svg, "<script") || strings.Contains(svg, "aria-pressed") {
		t.Fatal(svg)
	}
	if !strings.Contains(svg, ">Web</text>") {
		t.Fatal("legend text missing from the SVG")
	}
}
