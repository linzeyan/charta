package charta_test

import (
	"fmt"
	"log"

	"github.com/linzeyan/charta"
	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/geo"
	"github.com/linzeyan/charta/geo/world"
)

// ExampleBarChart compares a handful of categories by a single value.
func ExampleBarChart() {
	chart := charta.BarChart{
		Title: "Fruit sold this week",
		Data: []charta.Bar{
			{Label: "Apples", Value: 42}, {Label: "Pears", Value: 18}, {Label: "Cherries", Value: 27.5},
			{Label: "Plums", Value: -6}, {Label: "Figs", Value: 33},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Fruit sold this week. Bar chart with 5 bars.
}

// ExampleGroupedBarChart compares several series side by side within each category.
func ExampleGroupedBarChart() {
	chart := charta.GroupedBarChart{
		Title:      "Revenue by region",
		Categories: []string{"Q1", "Q2", "Q3", "Q4"},
		Series: []charta.Series{
			{Name: "North", Values: []float64{12, 15, 14, 19}},
			{Name: "South", Values: []float64{9, 11, 16, 13}},
			{Name: "West", Values: []float64{6, 8, 7, 11}},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Revenue by region. Grouped bar chart with 3 series and 4 categories.
}

// ExampleStackedBarChart shows how parts add up to each category's total.
func ExampleStackedBarChart() {
	chart := charta.StackedBarChart{
		Title:      "Tickets by channel",
		Categories: []string{"Q1", "Q2", "Q3", "Q4"},
		Series: []charta.Series{
			{Name: "Email", Values: []float64{40, 35, 42, 38}},
			{Name: "Chat", Values: []float64{22, 30, 33, 41}},
			{Name: "Phone", Values: []float64{15, 12, 10, 8}},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Tickets by channel. Stacked bar chart with 3 series and 4 categories.
}

// ExampleLineChart follows one or more series as they change over ordered steps.
func ExampleLineChart() {
	chart := charta.LineChart{
		Title:      "Weekly signups",
		Categories: []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"},
		Series: []charta.Series{
			{Name: "Web", Values: []float64{120, 135, 128, 160, 172, 190}},
			{Name: "iOS", Values: []float64{80, 95, 110, 108, 125, 140}},
			{Name: "Android", Values: []float64{60, 70, 66, 85, 90, 104}},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Weekly signups. Line chart with 3 series and 6 points.
}

// ExampleAreaChart emphasizes the volume under a trend, not just its shape.
func ExampleAreaChart() {
	chart := charta.AreaChart{
		Title:      "Active users (k)",
		Categories: []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"},
		Series:     []charta.Series{{Name: "Users", Values: []float64{3.2, 4.1, 4.0, 5.6, 6.3, 7.1}}},
		Curve:      charta.CurveMonotone,
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Active users (k). Area chart with 1 series and 6 points.
}

// ExampleStackedAreaChart shows how each part contributes to a total that changes over time.
func ExampleStackedAreaChart() {
	chart := charta.StackedAreaChart{
		Title:      "Traffic by source (k)",
		Categories: []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"},
		Series: []charta.Series{
			{Name: "Search", Values: []float64{12, 14, 13, 16, 18, 21}},
			{Name: "Social", Values: []float64{5, 7, 9, 8, 10, 12}},
			{Name: "Direct", Values: []float64{4, 4, 5, 6, 6, 7}},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Traffic by source (k). Stacked area chart with 3 series and 6 points.
}

// ExampleScatterChart reveals the relationship between two numeric variables.
func ExampleScatterChart() {
	chart := charta.ScatterChart{
		Title:  "Height vs weight",
		XLabel: "Height (cm)",
		YLabel: "Weight (kg)",
		Series: []charta.PointSeries{
			{Name: "Adults", Points: []charta.Point{{X: 160, Y: 55}, {X: 168, Y: 62}, {X: 175, Y: 70}, {X: 182, Y: 81}, {X: 171, Y: 66}}},
			{Name: "Teens", Points: []charta.Point{{X: 150, Y: 42}, {X: 158, Y: 49}, {X: 165, Y: 54}}},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Height vs weight. Scatterplot with 2 series and 8 points.
}

// ExampleBubbleChart adds a third variable, as circle area, to a scatter plot.
func ExampleBubbleChart() {
	size := func(v float64) *float64 { return &v }
	chart := charta.BubbleChart{
		Title:  "Markets",
		XLabel: "Growth (%)",
		YLabel: "Margin (%)",
		Series: []charta.PointSeries{{Name: "Markets", Points: []charta.Point{
			{X: 2, Y: 12, Size: size(40), Label: "EU"},
			{X: 6.5, Y: 9, Size: size(90), Label: "Asia"},
			{X: 4, Y: 18, Size: size(25), Label: "US"},
			{X: 8, Y: 6, Size: size(10), Label: "LatAm"},
		}}},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Markets. Bubble chart with 1 series and 4 points.
}

// ExampleHeatmapChart spots patterns across two categorical dimensions by color.
func ExampleHeatmapChart() {
	chart := charta.HeatmapChart{
		Title: "Orders by time of day",
		X:     []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
		Y:     []string{"Morning", "Noon", "Evening"},
		Values: [][]float64{
			{12, 18, 15, 20, 9},
			{30, 34, 28, 41, 25},
			{22, 19, 26, 31, 38},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Orders by time of day. Heatmap with 3 rows and 5 columns.
}

// ExampleSpanChart shows the range between a low and a high for each category.
func ExampleSpanChart() {
	chart := charta.SpanChart{
		Title: "Daily temperature range (°C)",
		Data: []charta.Span{
			{Label: "Mon", Low: 12, High: 21}, {Label: "Tue", Low: 9, High: 18},
			{Label: "Wed", Low: 14, High: 25}, {Label: "Thu", Low: 11, High: 19},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Daily temperature range (°C). Span chart with 4 ranges.
}

// ExamplePyramidChart compares two groups back to back across ordered bands, such as age.
func ExamplePyramidChart() {
	chart := charta.PyramidChart{
		Title:      "Population by age (k)",
		Categories: []string{"0–19", "20–39", "40–59", "60–79", "80+"},
		Left:       charta.PyramidSide{Name: "Male", Values: []float64{210, 260, 240, 150, 40}},
		Right:      charta.PyramidSide{Name: "Female", Values: []float64{200, 255, 245, 170, 65}},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Population by age (k). Population pyramid of Male and Female across 5 groups.
}

// ExampleBulletChart measures a value against a target and qualitative ranges in little space.
func ExampleBulletChart() {
	target := func(v float64) *float64 { return &v }
	chart := charta.BulletChart{
		Title: "Q3 against target",
		Data: []charta.Bullet{
			{Label: "Revenue (k$)", Value: 270, Target: target(250), Ranges: []float64{150, 225, 300}},
			{Label: "Profit (%)", Value: 22.5, Target: target(26), Ranges: []float64{20, 25, 30}},
			{Label: "New customers", Value: 1650, Target: target(2100), Ranges: []float64{1400, 2000, 2500}},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Q3 against target. Bullet graph with 3 measures.
}

// ExampleErrorBarChart shows estimates together with their uncertainty.
func ExampleErrorBarChart() {
	chart := charta.ErrorBarChart{
		Title: "Response by treatment",
		Data: []charta.ErrorBar{
			{Label: "Control", Value: 4.2, Low: 3.6, High: 4.9},
			{Label: "Dose A", Value: 5.8, Low: 5.1, High: 6.4},
			{Label: "Dose B", Value: 7.1, Low: 6.0, High: 8.3},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Response by treatment. Error bar chart with 3 estimates.
}

// ExampleHistogramChart shows how a single sample is distributed across bins.
func ExampleHistogramChart() {
	chart := charta.HistogramChart{
		Title:  "Support wait times",
		XLabel: "Minutes",
		Values: []float64{
			30, 37, 30, 29, 24, 30, 42, 36, 41, 34, 36, 34, 17, 40, 37, 36,
			17, 16, 24, 28, 35, 32, 37, 26, 35, 36, 26, 47, 37, 43, 26, 25,
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Support wait times. Histogram of 32 values in 7 bins.
}

// ExampleDensityChart compares the smoothed distributions of several samples.
func ExampleDensityChart() {
	chart := charta.DensityChart{
		Title:  "Response time by region",
		XLabel: "ms",
		Series: []charta.Sample{
			{Name: "EU", Values: []float64{138, 117, 85, 145, 118, 109, 127, 127, 147, 102, 140, 147, 146, 117, 107, 138, 122, 122, 146, 115}},
			{Name: "US", Values: []float64{145, 119, 136, 177, 109, 135, 175, 170, 150, 170, 154, 121, 111, 134, 173, 136, 127, 131, 112, 147}},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Response time by region. Density plot of 2 series.
}

// ExampleBoxPlotChart summarizes and compares groups by median, quartiles and outliers.
func ExampleBoxPlotChart() {
	chart := charta.BoxPlotChart{
		Title:  "Yield by fertilizer",
		YLabel: "t/ha",
		Groups: []charta.Sample{
			{Name: "None", Values: []float64{5.7, 4.3, 4.5, 3.6, 3.9, 4.4, 6.4, 2.8, 3.4, 5.3, 6.6, 5.6, 2.9, 2.2, 5.4}},
			{Name: "Organic", Values: []float64{6.7, 6.9, 6.5, 7.3, 5.9, 6.1, 7.2, 6.4, 5.7, 7.2, 7.6, 6.0, 5.3, 6.3, 6.3}},
			{Name: "Mineral", Values: []float64{3.6, 4.7, 7.0, 2.3, 4.5, 4.7, 4.5, 3.4, 5.1, 4.6, 3.3, 8.0, 4.7, 3.2, 11.5}},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Yield by fertilizer. Box plot of 3 groups.
}

// ExampleViolinChart compares the full shape of each group's distribution.
func ExampleViolinChart() {
	chart := charta.ViolinChart{
		Title:  "Yield by fertilizer",
		YLabel: "t/ha",
		Groups: []charta.Sample{
			{Name: "None", Values: []float64{5.7, 4.3, 4.5, 3.6, 3.9, 4.4, 6.4, 2.8, 3.4, 5.3, 6.6, 5.6, 2.9, 2.2, 5.4}},
			{Name: "Organic", Values: []float64{6.7, 6.9, 6.5, 7.3, 5.9, 6.1, 7.2, 6.4, 5.7, 7.2, 7.6, 6.0, 5.3, 6.3, 6.3}},
			{Name: "Mineral", Values: []float64{3.6, 4.7, 7.0, 2.3, 4.5, 4.7, 4.5, 3.4, 5.1, 4.6, 3.3, 8.0, 4.7, 3.2, 11.5}},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Yield by fertilizer. Violin plot of 3 groups.
}

// ExampleStemLeafChart lists a small sample's distribution while keeping every value readable.
func ExampleStemLeafChart() {
	chart := charta.StemLeafChart{
		Title: "Exam scores",
		Values: []float64{
			53, 53, 53, 57, 57, 59, 59, 60, 60, 67, 68, 70, 71, 73, 73, 74,
			78, 79, 79, 80, 80, 81, 82, 84, 86, 89, 89, 90, 90, 99,
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Exam scores. Stem-and-leaf plot of 30 values.
}

// ExampleParallelChart compares items across many numeric dimensions at once.
func ExampleParallelChart() {
	chart := charta.ParallelChart{
		Title:      "Laptops",
		Dimensions: []string{"Price ($)", "Weight (kg)", "Battery (h)", "Rating"},
		Rows: []charta.ParallelRow{
			{Label: "Air 13", Group: "Ultrabook", Values: []float64{1099, 1.24, 18, 4.7}},
			{Label: "XPS 13", Group: "Ultrabook", Values: []float64{1199, 1.17, 12, 4.5}},
			{Label: "Legion 5", Group: "Gaming", Values: []float64{1299, 2.4, 5, 4.4}},
			{Label: "ROG 16", Group: "Gaming", Values: []float64{1799, 2.5, 6, 4.6}},
			{Label: "Chromebook", Group: "Budget", Values: []float64{299, 1.5, 10, 3.8}},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Laptops. Parallel coordinates plot of 5 rows across 4 dimensions.
}

// ExampleCandlestickChart shows each period's open, high, low and close, colored by direction.
func ExampleCandlestickChart() {
	day := func(d int) int64 { return base.ToMS(2026, 3, d, 0, 0, 0) }
	chart := charta.CandlestickChart{
		Title: "ACME daily",
		Data: []charta.Candle{
			{Time: day(2), Open: 100, High: 102.4, Low: 99.1, Close: 101.8},
			{Time: day(3), Open: 101.8, High: 103.0, Low: 100.6, Close: 100.9},
			{Time: day(4), Open: 100.9, High: 101.5, Low: 98.7, Close: 99.2},
			{Time: day(5), Open: 99.2, High: 101.9, Low: 98.9, Close: 101.6},
			{Time: day(6), Open: 101.6, High: 104.2, Low: 101.0, Close: 103.7},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: ACME daily. Candlestick chart of 5 periods.
}

// ExampleOHLCChart shows each period's open, high, low and close with lighter ink than candles.
func ExampleOHLCChart() {
	day := func(d int) int64 { return base.ToMS(2026, 3, d, 0, 0, 0) }
	chart := charta.OHLCChart{
		Title: "ACME daily",
		Data: []charta.Candle{
			{Time: day(2), Open: 100, High: 102.4, Low: 99.1, Close: 101.8},
			{Time: day(3), Open: 101.8, High: 103.0, Low: 100.6, Close: 100.9},
			{Time: day(4), Open: 100.9, High: 101.5, Low: 98.7, Close: 99.2},
			{Time: day(5), Open: 99.2, High: 101.9, Low: 98.9, Close: 101.6},
			{Time: day(6), Open: 101.6, High: 104.2, Low: 101.0, Close: 103.7},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: ACME daily. OHLC chart of 5 periods.
}

// ExampleKagiChart filters out small price moves to show only meaningful reversals.
func ExampleKagiChart() {
	start := base.ToMS(2026, 1, 1, 0, 0, 0)
	closes := []float64{50, 51.2, 52.5, 51.8, 53.6, 55.1, 54.0, 52.2, 50.9, 51.5, 49.8, 48.6, 50.3, 52.8, 54.9, 56.2}
	data := make([]charta.TimeValue, len(closes))
	for i, v := range closes {
		data[i] = charta.TimeValue{Time: start + int64(i)*base.Day, Value: v}
	}
	chart := charta.KagiChart{Title: "ACME close", Data: data}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: ACME close. Kagi chart of 16 prices with a 4% reversal.
}

// ExamplePointFigureChart tracks price reversals as columns of Xs and Os, ignoring time.
func ExamplePointFigureChart() {
	start := base.ToMS(2026, 1, 1, 0, 0, 0)
	closes := []float64{50, 51.2, 52.5, 51.8, 53.6, 55.1, 54.0, 52.2, 50.9, 51.5, 49.8, 48.6, 50.3, 52.8, 54.9, 56.2}
	data := make([]charta.TimeValue, len(closes))
	for i, v := range closes {
		data[i] = charta.TimeValue{Time: start + int64(i)*base.Day, Value: v}
	}
	chart := charta.PointFigureChart{Title: "ACME close", Data: data}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: ACME close. Point and figure chart with 3 columns, box size 0.5, 3-box reversal.
}

// ExampleGanttChart schedules tasks on a timeline so overlaps and gaps are visible.
func ExampleGanttChart() {
	day := func(m, d int) int64 { return base.ToMS(2026, m, d, 0, 0, 0) }
	chart := charta.GanttChart{
		Title: "Website relaunch",
		Tasks: []charta.Task{
			{Name: "Research", Start: day(1, 5), End: day(1, 17), Group: "Design"},
			{Name: "Wireframes", Start: day(1, 12), End: day(1, 31), Group: "Design"},
			{Name: "Backend", Start: day(2, 2), End: day(3, 14), Group: "Build"},
			{Name: "Frontend", Start: day(2, 16), End: day(3, 21), Group: "Build"},
			{Name: "QA", Start: day(3, 16), End: day(4, 1), Group: "Launch"},
			{Name: "Go live", Start: day(4, 1), End: day(4, 1), Group: "Launch"},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Website relaunch. Gantt chart of 6 tasks.
}

// ExampleTimelineChart places dated events in order along a single time axis.
func ExampleTimelineChart() {
	at := func(y int64, m, d int) int64 { return base.ToMS(y, m, d, 0, 0, 0) }
	chart := charta.TimelineChart{
		Title: "Company milestones",
		Events: []charta.Event{
			{Time: at(2019, 3, 14), Label: "Founded"},
			{Time: at(2020, 6, 20), Label: "Seed round"},
			{Time: at(2021, 9, 1), Label: "1.0 release"},
			{Time: at(2023, 8, 8), Label: "Series A"},
			{Time: at(2025, 10, 30), Label: "1M users"},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Company milestones. Timeline of 5 events.
}

// ExampleStreamChart shows how the mix of several series shifts over time around a centre line.
func ExampleStreamChart() {
	times := make([]int64, 6)
	for m := range times {
		times[m] = base.ToMS(2025, m+1, 1, 0, 0, 0)
	}
	chart := charta.StreamChart{
		Title: "Listening hours by genre (k)",
		Times: times,
		Series: []charta.Series{
			{Name: "Pop", Values: []float64{22, 24, 23, 27, 30, 36}},
			{Name: "Rock", Values: []float64{18, 17, 19, 18, 16, 15}},
			{Name: "Hip-hop", Values: []float64{9, 11, 13, 16, 18, 21}},
			{Name: "Jazz", Values: []float64{6, 6, 5, 5, 4, 4}},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Listening hours by genre (k). Stream graph of 4 series over 6 times.
}

// ExampleCalendarChart shows a daily value laid out by week and weekday, exposing weekly rhythm.
func ExampleCalendarChart() {
	start := base.ToMS(2025, 1, 1, 0, 0, 0)
	commits := []float64{4, 7, 2, 0, 1, 6, 8, 5, 9, 3, 1, 0, 7, 6, 4, 8, 5, 0, 2, 9, 7, 3, 6, 5, 1, 0, 8, 4}
	days := make([]charta.TimeValue, len(commits))
	for d, v := range commits {
		days[d] = charta.TimeValue{Time: start + int64(d)*base.Day, Value: v}
	}
	chart := charta.CalendarChart{Title: "Commits per day, January 2025", Days: days}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Commits per day, January 2025. Calendar heatmap of 28 days across 1 year.
}

// ExampleTimetableChart lays out recurring weekly slots by day and time of day.
func ExampleTimetableChart() {
	slot := func(day string, h0, m0, h1, m1 int, label, group string) charta.Slot {
		return charta.Slot{Day: day, Start: h0*60 + m0, End: h1*60 + m1, Label: label, Group: group}
	}
	chart := charta.TimetableChart{
		Title: "Class timetable",
		Days:  []string{"Mon", "Tue", "Wed"},
		Slots: []charta.Slot{
			slot("Mon", 9, 0, 10, 30, "Maths", "Lecture"),
			slot("Mon", 14, 0, 16, 0, "Chemistry", "Lab"),
			slot("Tue", 9, 0, 10, 0, "English", "Seminar"),
			slot("Wed", 13, 0, 15, 0, "Physics", "Lab"),
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Class timetable. Timetable of 4 slots across 3 days.
}

// ExampleSpiralChart wraps a long series around a cycle so seasons line up turn after turn.
func ExampleSpiralChart() {
	months := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	temps := []float64{
		3.9, 5.2, 8.4, 12.1, 16.0, 19.6, 22.1, 21.4, 18.0, 13.2, 7.9, 5.1,
		4.4, 5.5, 8.9, 12.5, 16.3, 19.8, 22.6, 21.9, 18.3, 13.4, 8.4, 5.4,
	}
	data := make([]charta.Bar, len(temps))
	for i, t := range temps {
		data[i] = charta.Bar{Label: fmt.Sprintf("%s %d", months[i%12], 2024+i/12), Value: t}
	}
	chart := charta.SpiralChart{Title: "Monthly mean temperature (°C)", Period: 12, AngleLabels: months, Data: data}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Monthly mean temperature (°C). Spiral plot of 24 values, 12 per turn.
}

// ExamplePieChart shows parts of a whole when there are only a few parts.
func ExamplePieChart() {
	chart := charta.PieChart{
		Title: "Browser share (%)",
		Data: []charta.Bar{
			{Label: "Chrome", Value: 64.7}, {Label: "Safari", Value: 18.6}, {Label: "Edge", Value: 5.2},
			{Label: "Firefox", Value: 2.8}, {Label: "Other", Value: 8.7},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Browser share (%). Pie chart of 5 slices.
}

// ExampleDonutChart shows parts of a whole, leaving the centre free for a total or caption.
func ExampleDonutChart() {
	chart := charta.DonutChart{
		Title: "Budget by department (k$)",
		Data: []charta.Bar{
			{Label: "Engineering", Value: 420}, {Label: "Sales", Value: 260}, {Label: "Marketing", Value: 180},
			{Label: "Support", Value: 90}, {Label: "Admin", Value: 50},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Budget by department (k$). Donut chart of 5 slices, total 1,000.
}

// ExampleRoseChart compares cyclic categories, such as months, as wedges of varying radius.
func ExampleRoseChart() {
	chart := charta.RoseChart{
		Title:      "Causes of mortality",
		Categories: []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
		Series: []charta.Series{
			{Name: "Disease", Values: []float64{120, 150, 180, 210, 260, 320, 380, 410, 350, 280, 200, 150}},
			{Name: "Wounds", Values: []float64{20, 25, 30, 40, 60, 80, 90, 85, 70, 50, 35, 25}},
			{Name: "Other", Values: []float64{30, 28, 32, 35, 40, 45, 50, 48, 42, 38, 34, 30}},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Causes of mortality. Nightingale rose chart of 3 series across 12 categories.
}

// ExampleRadarChart compares a few items across several attributes on a shared scale.
func ExampleRadarChart() {
	chart := charta.RadarChart{
		Title: "Model comparison",
		Axes:  []string{"Speed", "Range", "Comfort", "Safety", "Price", "Efficiency"},
		Series: []charta.Series{
			{Name: "Model A", Values: []float64{8, 6, 7, 9, 5, 7}},
			{Name: "Model B", Values: []float64{6, 9, 6, 7, 8, 8}},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Model comparison. Radar chart of 2 series on 6 axes.
}

// ExampleRadialBarChart shows progress toward a goal as concentric arcs.
func ExampleRadialBarChart() {
	chart := charta.RadialBarChart{
		Title: "Goal progress (%)",
		Data: []charta.Bar{
			{Label: "Running", Value: 82}, {Label: "Cycling", Value: 64},
			{Label: "Swimming", Value: 45}, {Label: "Yoga", Value: 30},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Goal progress (%). Radial bar chart with 4 bars.
}

// ExampleRadialColumnChart arranges a cyclic series, such as months, as columns around a circle.
func ExampleRadialColumnChart() {
	months := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	rain := []float64{78, 62, 55, 48, 61, 84, 102, 96, 73, 88, 95, 86}
	data := make([]charta.Bar, len(rain))
	for i, v := range rain {
		data[i] = charta.Bar{Label: months[i], Value: v}
	}
	chart := charta.RadialColumnChart{Title: "Rainfall by month (mm)", Data: data}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Rainfall by month (mm). Radial column chart with 12 columns.
}

// ExampleTreemapChart shows a hierarchy's sizes as nested rectangles that fill the space.
func ExampleTreemapChart() {
	leaf := func(name string, v float64) charta.Node { return charta.Node{Name: name, Value: &v} }
	chart := charta.TreemapChart{
		Title: "Revenue by product line (M$)",
		Root: charta.Node{Name: "Revenue", Children: []charta.Node{
			{Name: "Software", Children: []charta.Node{leaf("Cloud", 42), leaf("Licenses", 18), leaf("Support", 12)}},
			{Name: "Hardware", Children: []charta.Node{leaf("Servers", 28), leaf("Storage", 16)}},
			{Name: "Services", Children: []charta.Node{leaf("Consulting", 22), leaf("Managed", 14)}},
		}},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Revenue by product line (M$). Treemap of 7 items.
}

// ExampleCirclePackingChart shows a hierarchy as nested circles, favoring structure over exact size.
func ExampleCirclePackingChart() {
	leaf := func(name string, v float64) charta.Node { return charta.Node{Name: name, Value: &v} }
	chart := charta.CirclePackingChart{
		Title: "Team sizes",
		Root: charta.Node{Name: "Company", Children: []charta.Node{
			{Name: "Engineering", Children: []charta.Node{leaf("Frontend", 12), leaf("Backend", 18), leaf("Data", 10)}},
			{Name: "Product", Children: []charta.Node{leaf("PM", 6), leaf("Design", 9)}},
			{Name: "Go-to-market", Children: []charta.Node{leaf("Sales", 15), leaf("Support", 11)}},
		}},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Team sizes. Circle packing of 7 items.
}

// ExampleSunburstChart shows a hierarchy as rings, one per level, sized by value.
func ExampleSunburstChart() {
	leaf := func(name string, v float64) charta.Node { return charta.Node{Name: name, Value: &v} }
	chart := charta.SunburstChart{
		Title: "Disk usage (GB)",
		Root: charta.Node{Name: "Disk", Children: []charta.Node{
			{Name: "Documents", Children: []charta.Node{
				{Name: "Work", Children: []charta.Node{leaf("Reports", 12), leaf("Slides", 8)}},
				leaf("Personal", 6),
			}},
			{Name: "Media", Children: []charta.Node{leaf("Photos", 48), leaf("Videos", 72), leaf("Music", 20)}},
			{Name: "Apps", Children: []charta.Node{leaf("System", 35), leaf("Games", 40)}},
		}},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Disk usage (GB). Sunburst of 8 items in 3 levels.
}

// ExampleTreeChart shows who reports to whom: a hierarchy's structure, not its sizes.
func ExampleTreeChart() {
	chart := charta.TreeChart{
		Title: "Organization",
		Root: charta.Node{Name: "CEO", Children: []charta.Node{
			{Name: "CTO", Children: []charta.Node{{Name: "Platform"}, {Name: "Web"}, {Name: "Data"}}},
			{Name: "CFO", Children: []charta.Node{{Name: "Finance"}, {Name: "Legal"}}},
			{Name: "COO", Children: []charta.Node{{Name: "Sales"}, {Name: "Support"}}},
		}},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Organization. Tree diagram of 11 nodes in 3 levels.
}

// ExampleMarimekkoChart shows two-way proportions: column width is one share, segment height the other.
func ExampleMarimekkoChart() {
	chart := charta.MarimekkoChart{
		Title:      "Sales by region and vendor (M$)",
		Categories: []string{"North America", "Europe", "Asia", "Other"},
		Series: []charta.Series{
			{Name: "Vendor A", Values: []float64{120, 80, 60, 15}},
			{Name: "Vendor B", Values: []float64{60, 90, 110, 10}},
			{Name: "Vendor C", Values: []float64{40, 30, 90, 25}},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Sales by region and vendor (M$). Marimekko chart of 3 series across 4 categories.
}

// ExampleBrainstormChart maps ideas radiating out from a central topic.
func ExampleBrainstormChart() {
	chart := charta.BrainstormChart{
		Title: "Launch plan",
		Root: charta.Node{Name: "Launch plan", Children: []charta.Node{
			{Name: "Marketing", Children: []charta.Node{{Name: "Social"}, {Name: "Email"}, {Name: "Ads"}}},
			{Name: "Product", Children: []charta.Node{{Name: "Features"}, {Name: "Pricing"}}},
			{Name: "Support", Children: []charta.Node{{Name: "FAQ"}, {Name: "Onboarding"}}},
		}},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Launch plan. Mind map of 10 ideas around "Launch plan".
}

// ExampleNetworkChart reveals clusters and bridges among connected nodes.
func ExampleNetworkChart() {
	chart := charta.NetworkChart{
		Title: "Who works with whom",
		Links: []charta.Link{
			{Source: "Ana", Target: "Ben", Value: 3}, {Source: "Ana", Target: "Cleo", Value: 2},
			{Source: "Ben", Target: "Cleo", Value: 2}, {Source: "Cleo", Target: "Dev", Value: 1},
			{Source: "Dev", Target: "Eli", Value: 1}, {Source: "Eli", Target: "Fay", Value: 3},
			{Source: "Eli", Target: "Gus", Value: 2}, {Source: "Fay", Target: "Gus", Value: 1},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Who works with whom. Network diagram of 7 nodes and 8 links.
}

// ExampleArcChart shows connections between nodes laid out on a single line.
func ExampleArcChart() {
	chart := charta.ArcChart{
		Title: "Co-authored papers",
		Links: []charta.Link{
			{Source: "Kim", Target: "Lee", Value: 4}, {Source: "Kim", Target: "Park", Value: 2},
			{Source: "Lee", Target: "Park", Value: 3}, {Source: "Park", Target: "Choi", Value: 1},
			{Source: "Choi", Target: "Jung", Value: 2}, {Source: "Kim", Target: "Jung", Value: 1},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Co-authored papers. Arc diagram of 5 nodes and 6 links.
}

// ExampleChordChart shows weighted flows in both directions between a few groups.
func ExampleChordChart() {
	chart := charta.ChordChart{
		Title: "Migration between regions (k people)",
		Links: []charta.Link{
			{Source: "North", Target: "South", Value: 30}, {Source: "South", Target: "North", Value: 12},
			{Source: "North", Target: "East", Value: 18}, {Source: "East", Target: "North", Value: 9},
			{Source: "East", Target: "West", Value: 15}, {Source: "West", Target: "East", Value: 22},
			{Source: "West", Target: "South", Value: 25}, {Source: "South", Target: "West", Value: 8},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Migration between regions (k people). Chord diagram of 8 flows among 4 groups.
}

// ExampleNonRibbonChordChart shows which nodes connect around a circle when weights matter less than links.
func ExampleNonRibbonChordChart() {
	chart := charta.NonRibbonChordChart{
		Title: "Package dependencies",
		Links: []charta.Link{
			{Source: "core", Target: "utils", Value: 2}, {Source: "core", Target: "io", Value: 2},
			{Source: "io", Target: "net", Value: 1}, {Source: "net", Target: "http", Value: 2},
			{Source: "http", Target: "json", Value: 1}, {Source: "cli", Target: "core", Value: 2},
			{Source: "cli", Target: "json", Value: 1},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Package dependencies. Non-ribbon chord diagram of 7 links among 7 nodes.
}

// ExampleSankeyChart follows quantities as they flow and split through stages.
func ExampleSankeyChart() {
	chart := charta.SankeyChart{
		Title: "Energy flows (TWh)",
		Links: []charta.Link{
			{Source: "Coal", Target: "Electricity", Value: 120}, {Source: "Gas", Target: "Electricity", Value: 90},
			{Source: "Gas", Target: "Homes", Value: 60}, {Source: "Renewables", Target: "Electricity", Value: 70},
			{Source: "Electricity", Target: "Homes", Value: 110}, {Source: "Electricity", Target: "Industry", Value: 130},
			{Source: "Electricity", Target: "Losses", Value: 40},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Energy flows (TWh). Sankey diagram of 7 flows between 7 nodes.
}

// ExampleParallelSetsChart shows how counts split across several categorical dimensions in turn.
func ExampleParallelSetsChart() {
	chart := charta.ParallelSetsChart{
		Title:      "Titanic passengers",
		Dimensions: []string{"Class", "Sex", "Survived"},
		Combinations: []charta.Combination{
			{Categories: []string{"First", "Male", "No"}, Count: 118}, {Categories: []string{"First", "Male", "Yes"}, Count: 62},
			{Categories: []string{"First", "Female", "No"}, Count: 4}, {Categories: []string{"First", "Female", "Yes"}, Count: 141},
			{Categories: []string{"Third", "Male", "No"}, Count: 422}, {Categories: []string{"Third", "Male", "Yes"}, Count: 88},
			{Categories: []string{"Third", "Female", "No"}, Count: 106}, {Categories: []string{"Third", "Female", "Yes"}, Count: 90},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Titanic passengers. Parallel sets of 1,031 across 3 dimensions.
}

// ExampleFlowChart documents a process's steps, decisions and loops.
func ExampleFlowChart() {
	chart := charta.FlowChart{
		Title: "Order handling",
		Steps: []charta.Step{
			{ID: "order", Label: "Order placed", Kind: charta.Terminal},
			{ID: "stock", Label: "In stock?", Kind: charta.Decision},
			{ID: "pack", Label: "Pack", Kind: charta.Process},
			{ID: "wait", Label: "Wait for stock", Kind: charta.Process},
			{ID: "done", Label: "Shipped", Kind: charta.Terminal},
		},
		Arrows: []charta.Arrow{
			{From: "order", To: "stock"},
			{From: "stock", To: "pack", Label: "Yes"},
			{From: "stock", To: "wait", Label: "No"},
			{From: "wait", To: "stock"},
			{From: "pack", To: "done"},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Order handling. Flow chart of 5 steps.
}

// ExampleChoroplethChart compares a rate across regions by shading each one.
func ExampleChoroplethChart() {
	chart := charta.ChoroplethChart{
		Title:    "Life expectancy at birth (years)",
		Features: world.Features(),
		Values: []charta.Region{
			{ID: "USA", Value: 78.5}, {ID: "CAN", Value: 82.2}, {ID: "BRA", Value: 75.9},
			{ID: "FRA", Value: 82.5}, {ID: "NGA", Value: 62.6}, {ID: "IND", Value: 70.8},
			{ID: "CHN", Value: 77.4}, {ID: "JPN", Value: 84.3}, {ID: "AUS", Value: 83},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Life expectancy at birth (years). Choropleth map of 177 regions, 9 with data.
}

// ExampleBubbleMapChart sizes circles at locations to compare amounts, not rates, across a map.
func ExampleBubbleMapChart() {
	chart := charta.BubbleMapChart{
		Title:    "Largest urban areas (M people)",
		Features: world.Features(),
		Places: []charta.Place{
			{Name: "Tokyo", Lon: 139.69, Lat: 35.69, Value: 37.4},
			{Name: "Delhi", Lon: 77.21, Lat: 28.61, Value: 31},
			{Name: "São Paulo", Lon: -46.63, Lat: -23.55, Value: 22},
			{Name: "Cairo", Lon: 31.24, Lat: 30.04, Value: 21.3},
			{Name: "New York", Lon: -74.01, Lat: 40.71, Value: 18.8},
			{Name: "London", Lon: -0.13, Lat: 51.51, Value: 9.5},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Largest urban areas (M people). Bubble map of 6 places.
}

// ExampleDotMapChart plots individual occurrences so their geographic clustering shows.
func ExampleDotMapChart() {
	chart := charta.DotMapChart{
		Title:      "Strong earthquakes (sample)",
		Features:   world.Features(),
		Projection: geo.Projection{Kind: geo.Equirectangular},
		Dots: []charta.Dot{
			{Lon: 142.8, Lat: 36.4, Group: "Shallow"}, {Lon: 143.5, Lat: 41.9, Group: "Shallow"}, {Lon: 140.7, Lat: 36.7, Group: "Deep"},
			{Lon: 120.4, Lat: -5.4, Group: "Deep"}, {Lon: 121.1, Lat: -8.2, Group: "Shallow"},
			{Lon: -73.7, Lat: -33.2, Group: "Shallow"}, {Lon: -70.7, Lat: -30.8, Group: "Deep"},
			{Lon: -149.8, Lat: 56, Group: "Shallow"}, {Lon: -151.9, Lat: 58.1, Group: "Deep"},
			{Lon: 175.8, Lat: -37.1, Group: "Shallow"}, {Lon: 54.3, Lat: 33.8, Group: "Shallow"},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Strong earthquakes (sample). Dot map of 11 dots.
}

// ExampleConnectionMapChart draws routes between places on a map, weighted by volume.
func ExampleConnectionMapChart() {
	chart := charta.ConnectionMapChart{
		Title:      "Weekly nonstop flights from Singapore",
		Features:   world.Features(),
		Projection: geo.Projection{Kind: geo.Mercator},
		Places: []charta.Location{
			{Name: "Singapore", Lon: 103.82, Lat: 1.35},
			{Name: "London", Lon: -0.13, Lat: 51.51},
			{Name: "Sydney", Lon: 151.21, Lat: -33.87},
			{Name: "Tokyo", Lon: 139.69, Lat: 35.69},
			{Name: "Dubai", Lon: 55.27, Lat: 25.2},
		},
		Links: []charta.Link{
			{Source: "Singapore", Target: "London", Value: 28}, {Source: "Singapore", Target: "Sydney", Value: 35},
			{Source: "Singapore", Target: "Tokyo", Value: 42}, {Source: "Singapore", Target: "Dubai", Value: 21},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Weekly nonstop flights from Singapore. Connection map of 4 routes between 5 places.
}

// ExampleFlowMapChart shows directed movement between places, with width for quantity.
func ExampleFlowMapChart() {
	chart := charta.FlowMapChart{
		Title:    "Green coffee exports (kt)",
		Features: world.Features(),
		Places: []charta.Location{
			{Name: "Brazil", Lon: -51.9, Lat: -14.2},
			{Name: "Vietnam", Lon: 108.3, Lat: 14.1},
			{Name: "Colombia", Lon: -74.3, Lat: 4.6},
			{Name: "USA", Lon: -98.6, Lat: 39.8},
			{Name: "Germany", Lon: 10.5, Lat: 51.2},
		},
		Links: []charta.Link{
			{Source: "Brazil", Target: "USA", Value: 400}, {Source: "Brazil", Target: "Germany", Value: 300},
			{Source: "Vietnam", Target: "Germany", Value: 200}, {Source: "Vietnam", Target: "USA", Value: 150},
			{Source: "Colombia", Target: "USA", Value: 250},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Green coffee exports (kt). Flow map of 5 flows between 5 places.
}

// ExamplePictogramChart makes counts tangible by repeating an icon per unit.
func ExamplePictogramChart() {
	chart := charta.PictogramChart{
		Title: "Volunteers by city",
		Unit:  10,
		Data: []charta.Bar{
			{Label: "Lisbon", Value: 46}, {Label: "Porto", Value: 32},
			{Label: "Braga", Value: 18}, {Label: "Faro", Value: 7},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Volunteers by city. Pictogram of 4 rows, each icon 10.
}

// ExampleProportionalAreaChart compares values that span orders of magnitude by shape area.
func ExampleProportionalAreaChart() {
	chart := charta.ProportionalAreaChart{
		Title: "Land area (M km²)",
		Data: []charta.Bar{
			{Label: "Russia", Value: 17.1}, {Label: "Canada", Value: 9.98}, {Label: "China", Value: 9.6},
			{Label: "India", Value: 3.29}, {Label: "Japan", Value: 0.378},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Land area (M km²). Proportional area chart of 5 values.
}

// ExampleTallyChart shows small counts as tally marks, readable as they were recorded.
func ExampleTallyChart() {
	chart := charta.TallyChart{
		Title: "Birds spotted this morning",
		Data: []charta.Bar{
			{Label: "Robin", Value: 12}, {Label: "Sparrow", Value: 23}, {Label: "Blue tit", Value: 7},
			{Label: "Magpie", Value: 4}, {Label: "Wren", Value: 0},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Birds spotted this morning. Tally chart of 5 rows.
}

// ExampleDotMatrixChart shows each unit as one dot, so shares of a whole can be counted.
func ExampleDotMatrixChart() {
	chart := charta.DotMatrixChart{
		Title: "Seats won",
		Unit:  1,
		Data: []charta.Bar{
			{Label: "Greens", Value: 42}, {Label: "Reds", Value: 31}, {Label: "Blues", Value: 15},
			{Label: "Yellows", Value: 8}, {Label: "Independents", Value: 4},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Seats won. Dot matrix of 100 dots in 5 categories, each dot 1.
}

// ExampleWordCloudChart gives a quick impression of which terms dominate a text.
func ExampleWordCloudChart() {
	chart := charta.WordCloudChart{
		Title: "Words in chart reviews",
		Words: []charta.Word{
			{Text: "data", Weight: 90}, {Text: "chart", Weight: 72}, {Text: "color", Weight: 60},
			{Text: "scale", Weight: 55}, {Text: "axis", Weight: 50}, {Text: "legend", Weight: 44},
			{Text: "tooltip", Weight: 40}, {Text: "label", Weight: 38}, {Text: "grid", Weight: 33},
			{Text: "dark mode", Weight: 22}, {Text: "contrast", Weight: 20}, {Text: "palette", Weight: 19},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Words in chart reviews. Word cloud of 12 words.
}

// ExampleVennChart shows how sets overlap and what they share.
func ExampleVennChart() {
	chart := charta.VennChart{
		Title: "Languages spoken (people)",
		Areas: []charta.VennArea{
			{Sets: []string{"English"}, Size: 120}, {Sets: []string{"Spanish"}, Size: 70}, {Sets: []string{"French"}, Size: 45},
			{Sets: []string{"English", "Spanish"}, Size: 30}, {Sets: []string{"English", "French"}, Size: 20},
			{Sets: []string{"Spanish", "French"}, Size: 8}, {Sets: []string{"English", "Spanish", "French"}, Size: 5},
		},
	}
	sc, err := chart.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		log.Fatal(err)
	}
	// The scene's label is what a screen reader announces.
	fmt.Println(sc.Label)
	// Output: Languages spoken (people). Venn diagram of 3 sets.
}
