// Package demo holds the charts every Go gallery shows, shared by the Gio
// gallery and its headless tests so the tests exercise exactly what a person
// sees. The data matches the Rust examples/demos crate.
package demo

import (
	"fmt"
	"math"

	"github.com/linzeyan/charta"
	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/geo"
	"github.com/linzeyan/charta/geo/world"
)

// Demo is a named chart.
type Demo struct {
	Name  string
	Chart charta.Chart
}

var (
	quarters = []string{"Q1", "Q2", "Q3", "Q4"}
	months   = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"}
)

// The statistical demos' samples, the same literals as the Rust crate's.
var (
	waits = []float64{
		30, 37, 30, 29, 24, 30, 42, 36, 41, 34, 36, 34, 17, 40, 37, 36,
		17, 16, 24, 28, 35, 32, 37, 26, 35, 36, 26, 47, 37, 43, 26, 25,
		29, 31, 38, 34, 28, 23, 27, 43, 25, 34, 36, 19, 32, 44, 14, 29,
		31, 25, 36, 31, 19, 39, 38, 41, 45, 35, 33, 20,
	}
	euMs = []float64{
		138, 117, 85, 145, 118, 109, 127, 127, 147, 102, 140, 147, 146,
		117, 107, 138, 122, 122, 146, 115, 79, 113, 87, 135, 126, 109, 120,
		135, 121, 144, 119, 139, 147, 149, 108, 136, 86, 100, 85, 139, 98,
		120, 117, 119, 109, 124, 152, 121, 130, 138,
	}
	usMs = []float64{
		145, 119, 136, 177, 109, 135, 175, 170, 150, 170, 154, 121, 111,
		134, 173, 136, 127, 131, 112, 147, 121, 159, 91, 158, 134, 101,
		168, 143, 94, 128, 157, 139, 169, 169, 167, 158, 183, 166, 161, 98,
		172, 183, 143, 138, 199, 106, 162, 211, 127, 167,
	}
	yieldNone = []float64{
		5.7, 4.3, 4.5, 3.6, 3.9, 4.4, 6.4, 2.8, 3.4, 5.3, 6.6, 5.6, 2.9, 2.2, 5.4, 4.2, 3.8, 6.1, 6.2,
		5.2, 5.3, 5.5, 6.8, 5.7, 5.6, 5.6, 3.3, 6.4, 6.1, 5.6, 2.8, 4.3, 5.9, 3.0, 4.8, 6.1, 3.6, 6.8,
		5.6, 4.8,
	}
	yieldOrganic = []float64{
		6.7, 6.9, 6.5, 7.3, 5.9, 6.1, 7.2, 6.4, 5.7, 7.2, 7.6, 6.0, 5.3, 6.3, 6.3, 6.2, 7.5, 5.6, 7.4,
		5.4, 5.8, 6.9, 7.3, 7.1, 6.7, 6.5, 6.5, 6.9, 6.3, 6.6, 6.9, 6.4, 7.0, 6.9, 8.0, 6.7, 6.1, 6.1,
		6.4, 7.1,
	}
	// The Rust data has a -0.0; a Go constant -0.0 is +0, so spell it out.
	yieldMineral = []float64{
		3.6, 4.7, 7.0, math.Copysign(0, -1), 2.3, 4.5, 4.7, 4.5, 3.4, 5.1, 4.6, 3.3, 8.0, 4.7, 3.2, 3.9, 3.7, 4.0,
		-0.3, 3.3, 5.7, 2.2, 4.0, 5.6, 5.5, 6.5, 1.4, 3.5, 3.6, 5.1, 5.8, -0.2, 5.8, 1.8, 5.2, 1.7,
		4.4, 6.0, 3.9, 4.4, 11.5,
	}
	scores = []float64{
		53, 53, 53, 57, 57, 59, 59, 60, 60, 67, 68, 70, 71, 73, 73, 74,
		78, 79, 79, 80, 80, 81, 82, 84, 86, 89, 89, 90, 90, 99,
	}
	yieldGroups = []charta.Sample{{Name: "None", Values: yieldNone}, {Name: "Organic", Values: yieldOrganic}, {Name: "Mineral", Values: yieldMineral}}
)

func size(v float64) *float64 { return &v }

// All returns every demo, one per chart type.
func All() []Demo {
	return []Demo{
		{"Bar", charta.BarChart{
			Title: "Fruit sold this week",
			Data: []charta.Bar{
				{Label: "Apples", Value: 42}, {Label: "Pears", Value: 18}, {Label: "Cherries", Value: 27.5},
				{Label: "Plums", Value: -6}, {Label: "Figs", Value: 33},
			},
		}},
		{"Grouped bar", charta.GroupedBarChart{
			Title:      "Revenue by region",
			Categories: quarters,
			Series: []charta.Series{
				{Name: "North", Values: []float64{12, 15, 14, 19}},
				{Name: "South", Values: []float64{9, 11, 16, 13}},
				{Name: "West", Values: []float64{6, 8, 7, 11}},
			},
		}},
		{"Stacked bar", charta.StackedBarChart{
			Title:      "Tickets by channel",
			Categories: quarters,
			Series: []charta.Series{
				{Name: "Email", Values: []float64{40, 35, 42, 38}},
				{Name: "Chat", Values: []float64{22, 30, 33, 41}},
				{Name: "Phone", Values: []float64{15, 12, 10, 8}},
			},
		}},
		{"Line", Line()},
		{"Area", charta.AreaChart{
			Title:      "Active users (k)",
			Categories: months,
			Series:     []charta.Series{{Name: "Users", Values: []float64{3.2, 4.1, 4.0, 5.6, 6.3, 7.1}}},
			Curve:      charta.CurveMonotone,
		}},
		{"Stacked area", charta.StackedAreaChart{
			Title:      "Traffic by source (k)",
			Categories: months,
			Series: []charta.Series{
				{Name: "Search", Values: []float64{12, 14, 13, 16, 18, 21}},
				{Name: "Social", Values: []float64{5, 7, 9, 8, 10, 12}},
				{Name: "Direct", Values: []float64{4, 4, 5, 6, 6, 7}},
			},
		}},
		{"Scatter", charta.ScatterChart{
			Title:  "Height vs weight",
			XLabel: "Height (cm)",
			YLabel: "Weight (kg)",
			Series: []charta.PointSeries{
				{Name: "Adults", Points: []charta.Point{{X: 160, Y: 55}, {X: 168, Y: 62}, {X: 175, Y: 70}, {X: 182, Y: 81}, {X: 171, Y: 66}}},
				{Name: "Teens", Points: []charta.Point{{X: 150, Y: 42}, {X: 158, Y: 49}, {X: 165, Y: 54}}},
			},
		}},
		{"Bubble", charta.BubbleChart{
			Title:  "Markets",
			XLabel: "Growth (%)",
			YLabel: "Margin (%)",
			Series: []charta.PointSeries{{Name: "Markets", Points: []charta.Point{
				{X: 2, Y: 12, Size: size(40), Label: "EU"},
				{X: 6.5, Y: 9, Size: size(90), Label: "Asia"},
				{X: 4, Y: 18, Size: size(25), Label: "US"},
				{X: 8, Y: 6, Size: size(10), Label: "LatAm"},
			}}},
		}},
		{"Heatmap", charta.HeatmapChart{
			Title: "Orders by time of day",
			X:     []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
			Y:     []string{"Morning", "Noon", "Evening"},
			Values: [][]float64{
				{12, 18, 15, 20, 9},
				{30, 34, 28, 41, 25},
				{22, 19, 26, 31, 38},
			},
		}},
		{"Span", charta.SpanChart{
			Title: "Daily temperature range (°C)",
			Data:  []charta.Span{{Label: "Mon", Low: 12, High: 21}, {Label: "Tue", Low: 9, High: 18}, {Label: "Wed", Low: 14, High: 25}, {Label: "Thu", Low: 11, High: 19}},
		}},
		{"Population pyramid", charta.PyramidChart{
			Title:      "Population by age (k)",
			Categories: []string{"0–19", "20–39", "40–59", "60–79", "80+"},
			Left:       charta.PyramidSide{Name: "Male", Values: []float64{210, 260, 240, 150, 40}},
			Right:      charta.PyramidSide{Name: "Female", Values: []float64{200, 255, 245, 170, 65}},
		}},
		{"Bullet", charta.BulletChart{
			Title: "Q3 against target",
			Data: []charta.Bullet{
				{Label: "Revenue (k$)", Value: 270, Target: size(250), Ranges: []float64{150, 225, 300}},
				{Label: "Profit (%)", Value: 22.5, Target: size(26), Ranges: []float64{20, 25, 30}},
				{Label: "New customers", Value: 1650, Target: size(2100), Ranges: []float64{1400, 2000, 2500}},
			},
		}},
		{"Error bars", charta.ErrorBarChart{
			Title: "Response by treatment",
			Data:  []charta.ErrorBar{{Label: "Control", Value: 4.2, Low: 3.6, High: 4.9}, {Label: "Dose A", Value: 5.8, Low: 5.1, High: 6.4}, {Label: "Dose B", Value: 7.1, Low: 6.0, High: 8.3}},
		}},
		{"Histogram", charta.HistogramChart{Title: "Support wait times", XLabel: "Minutes", Values: waits}},
		{"Density", charta.DensityChart{
			Title:  "Response time by region",
			XLabel: "ms",
			Series: []charta.Sample{{Name: "EU", Values: euMs}, {Name: "US", Values: usMs}},
		}},
		{"Box plot", charta.BoxPlotChart{Title: "Yield by fertilizer", YLabel: "t/ha", Groups: yieldGroups}},
		{"Violin", charta.ViolinChart{Title: "Yield by fertilizer", YLabel: "t/ha", Groups: yieldGroups}},
		{"Stem and leaf", charta.StemLeafChart{Title: "Exam scores", Values: scores}},
		{"Parallel coordinates", laptops()},
		{"Candlestick", charta.CandlestickChart{Title: "ACME daily", Data: candles()}},
		{"OHLC", charta.OHLCChart{Title: "ACME daily", Data: candles()}},
		{"Kagi", charta.KagiChart{Title: "ACME close", Data: closes()}},
		{"Point and figure", charta.PointFigureChart{Title: "ACME close", Data: closes()}},
		{"Gantt", gantt()},
		{"Timeline", timeline()},
		{"Stream graph", stream()},
		{"Calendar", calendar()},
		{"Timetable", timetable()},
		{"Spiral", spiral()},
		{"Pie", charta.PieChart{
			Title: "Browser share (%)",
			Data: []charta.Bar{
				{Label: "Chrome", Value: 64.7}, {Label: "Safari", Value: 18.6}, {Label: "Edge", Value: 5.2},
				{Label: "Firefox", Value: 2.8}, {Label: "Other", Value: 8.7},
			},
		}},
		{"Donut", charta.DonutChart{
			Title: "Budget by department (k$)",
			Data: []charta.Bar{
				{Label: "Engineering", Value: 420}, {Label: "Sales", Value: 260}, {Label: "Marketing", Value: 180},
				{Label: "Support", Value: 90}, {Label: "Admin", Value: 50},
			},
		}},
		{"Nightingale rose", charta.RoseChart{
			Title:      "Causes of mortality",
			Categories: monthNames,
			Series: []charta.Series{
				{Name: "Disease", Values: []float64{120, 150, 180, 210, 260, 320, 380, 410, 350, 280, 200, 150}},
				{Name: "Wounds", Values: []float64{20, 25, 30, 40, 60, 80, 90, 85, 70, 50, 35, 25}},
				{Name: "Other", Values: []float64{30, 28, 32, 35, 40, 45, 50, 48, 42, 38, 34, 30}},
			},
		}},
		{"Radar", charta.RadarChart{
			Title: "Model comparison",
			Axes:  []string{"Speed", "Range", "Comfort", "Safety", "Price", "Efficiency"},
			Series: []charta.Series{
				{Name: "Model A", Values: []float64{8, 6, 7, 9, 5, 7}},
				{Name: "Model B", Values: []float64{6, 9, 6, 7, 8, 8}},
			},
		}},
		{"Radial bar", charta.RadialBarChart{
			Title: "Goal progress (%)",
			Data:  []charta.Bar{{Label: "Running", Value: 82}, {Label: "Cycling", Value: 64}, {Label: "Swimming", Value: 45}, {Label: "Yoga", Value: 30}},
		}},
		{"Radial column", radialColumn()},
		{"Treemap", charta.TreemapChart{Title: "Revenue by product line (M$)", Root: revenue()}},
		{"Circle packing", charta.CirclePackingChart{Title: "Team sizes", Root: teams()}},
		{"Sunburst", charta.SunburstChart{Title: "Disk usage (GB)", Root: disk()}},
		{"Tree diagram", charta.TreeChart{Title: "Organization", Root: org()}},
		{"Marimekko", charta.MarimekkoChart{
			Title:      "Sales by region and vendor (M$)",
			Categories: []string{"North America", "Europe", "Asia", "Other"},
			Series: []charta.Series{
				{Name: "Vendor A", Values: []float64{120, 80, 60, 15}},
				{Name: "Vendor B", Values: []float64{60, 90, 110, 10}},
				{Name: "Vendor C", Values: []float64{40, 30, 90, 25}},
			},
		}},
		{"Brainstorm", charta.BrainstormChart{Title: "Launch plan", Root: launchPlan()}},
		{"Network", charta.NetworkChart{Title: "Who works with whom", Links: network}},
		{"Arc", charta.ArcChart{Title: "Co-authored papers", Links: papers}},
		{"Chord", charta.ChordChart{Title: "Migration between regions (k people)", Links: migration}},
		{"Non-ribbon chord", charta.NonRibbonChordChart{Title: "Package dependencies", Links: deps}},
		{"Sankey", charta.SankeyChart{Title: "Energy flows (TWh)", Links: energy}},
		{"Parallel sets", titanic()},
		{"Flow chart", orderFlow()},
		{"Choropleth", charta.ChoroplethChart{Title: "Life expectancy at birth (years)", Features: world.Features(), Values: life}},
		{"Bubble map", charta.BubbleMapChart{Title: "Largest urban areas (M people)", Features: world.Features(), Places: cities}},
		{"Dot map", charta.DotMapChart{
			Title: "Strong earthquakes (sample)", Features: world.Features(),
			Projection: geo.Projection{Kind: geo.Equirectangular}, Dots: quakes,
		}},
		{"Connection map", charta.ConnectionMapChart{
			Title: "Weekly nonstop flights from Singapore", Features: world.Features(),
			Projection: geo.Projection{Kind: geo.Mercator}, Places: airports, Links: flights,
		}},
		{"Flow map", charta.FlowMapChart{Title: "Green coffee exports (kt)", Features: world.Features(), Places: coffeePlaces, Links: coffee}},
		{"Pictogram", charta.PictogramChart{
			Title: "Volunteers by city", Unit: 10,
			Data: []charta.Bar{{Label: "Lisbon", Value: 46}, {Label: "Porto", Value: 32}, {Label: "Braga", Value: 18}, {Label: "Faro", Value: 7}},
		}},
		{"Proportional area", charta.ProportionalAreaChart{
			Title: "Land area (M km²)",
			Data: []charta.Bar{
				{Label: "Russia", Value: 17.1}, {Label: "Canada", Value: 9.98}, {Label: "China", Value: 9.6},
				{Label: "India", Value: 3.29}, {Label: "Japan", Value: 0.378},
			},
		}},
		{"Tally", charta.TallyChart{
			Title: "Birds spotted this morning",
			Data: []charta.Bar{
				{Label: "Robin", Value: 12}, {Label: "Sparrow", Value: 23}, {Label: "Blue tit", Value: 7},
				{Label: "Magpie", Value: 4}, {Label: "Wren", Value: 0},
			},
		}},
		{"Dot matrix", charta.DotMatrixChart{
			Title: "Seats won", Unit: 1,
			Data: []charta.Bar{
				{Label: "Greens", Value: 42}, {Label: "Reds", Value: 31}, {Label: "Blues", Value: 15},
				{Label: "Yellows", Value: 8}, {Label: "Independents", Value: 4},
			},
		}},
		{"Word cloud", charta.WordCloudChart{Title: "Words in chart reviews", Words: words}},
		{"Venn", charta.VennChart{
			Title: "Languages spoken (people)",
			Areas: []charta.VennArea{
				{Sets: []string{"English"}, Size: 120}, {Sets: []string{"Spanish"}, Size: 70}, {Sets: []string{"French"}, Size: 45},
				{Sets: []string{"English", "Spanish"}, Size: 30}, {Sets: []string{"English", "French"}, Size: 20},
				{Sets: []string{"Spanish", "French"}, Size: 8}, {Sets: []string{"English", "Spanish", "French"}, Size: 5},
			},
		}},
	}
}

// The map, text and set demos' data, in the Rust demos' order.
var (
	life = []charta.Region{
		{ID: "USA", Value: 78.5},
		{ID: "CAN", Value: 82.2},
		{ID: "MEX", Value: 76},
		{ID: "BRA", Value: 75.9},
		{ID: "ARG", Value: 76.6},
		{ID: "CHL", Value: 80.7},
		{ID: "COL", Value: 79.3},
		{ID: "PER", Value: 79.9},
		{ID: "VEN", Value: 73.9},
		{ID: "BOL", Value: 73.1},
		{ID: "GBR", Value: 81.4},
		{ID: "FRA", Value: 82.5},
		{ID: "DEU", Value: 81.7},
		{ID: "ESP", Value: 83.2},
		{ID: "ITA", Value: 83},
		{ID: "PRT", Value: 81.6},
		{ID: "NOR", Value: 82.6},
		{ID: "SWE", Value: 82.4},
		{ID: "FIN", Value: 81.6},
		{ID: "POL", Value: 78.3},
		{ID: "UKR", Value: 73},
		{ID: "RUS", Value: 73.2},
		{ID: "TUR", Value: 78.6},
		{ID: "EGY", Value: 71.8},
		{ID: "NGA", Value: 62.6},
		{ID: "ETH", Value: 68.7},
		{ID: "KEN", Value: 66.1},
		{ID: "ZAF", Value: 65.3},
		{ID: "COD", Value: 62.4},
		{ID: "AGO", Value: 63.1},
		{ID: "TZA", Value: 66.2},
		{ID: "DZA", Value: 77.1},
		{ID: "MAR", Value: 73},
		{ID: "SAU", Value: 75.1},
		{ID: "IRN", Value: 77.3},
		{ID: "PAK", Value: 65.6},
		{ID: "IND", Value: 70.8},
		{ID: "CHN", Value: 77.4},
		{ID: "JPN", Value: 84.3},
		{ID: "KOR", Value: 83.3},
		{ID: "IDN", Value: 71.3},
		{ID: "PHL", Value: 70.4},
		{ID: "VNM", Value: 73.7},
		{ID: "THA", Value: 77.7},
		{ID: "AUS", Value: 83},
		{ID: "NZL", Value: 82},
		{ID: "MNG", Value: 68.1},
		{ID: "KAZ", Value: 73.2},
		{ID: "AFG", Value: 63.2},
		{ID: "BGD", Value: 74.3},
	}
	cities = []charta.Place{
		{Name: "Tokyo", Lon: 139.69, Lat: 35.69, Value: 37.4},
		{Name: "Delhi", Lon: 77.21, Lat: 28.61, Value: 31},
		{Name: "Shanghai", Lon: 121.47, Lat: 31.23, Value: 27.1},
		{Name: "São Paulo", Lon: -46.63, Lat: -23.55, Value: 22},
		{Name: "Mexico City", Lon: -99.13, Lat: 19.43, Value: 21.8},
		{Name: "Cairo", Lon: 31.24, Lat: 30.04, Value: 21.3},
		{Name: "Dhaka", Lon: 90.41, Lat: 23.81, Value: 21},
		{Name: "Mumbai", Lon: 72.88, Lat: 19.08, Value: 20.7},
		{Name: "Beijing", Lon: 116.41, Lat: 39.9, Value: 20.5},
		{Name: "Osaka", Lon: 135.5, Lat: 34.69, Value: 19.2},
		{Name: "New York", Lon: -74.01, Lat: 40.71, Value: 18.8},
		{Name: "Karachi", Lon: 67.01, Lat: 24.86, Value: 16.1},
		{Name: "Buenos Aires", Lon: -58.38, Lat: -34.6, Value: 15.2},
		{Name: "Istanbul", Lon: 28.98, Lat: 41.01, Value: 15.2},
		{Name: "Lagos", Lon: 3.38, Lat: 6.52, Value: 14.4},
		{Name: "Kinshasa", Lon: 15.27, Lat: -4.44, Value: 14.3},
		{Name: "Manila", Lon: 120.98, Lat: 14.6, Value: 13.9},
		{Name: "Rio de Janeiro", Lon: -43.17, Lat: -22.91, Value: 13.5},
		{Name: "Moscow", Lon: 37.62, Lat: 55.76, Value: 12.5},
		{Name: "Los Angeles", Lon: -118.24, Lat: 34.05, Value: 12.5},
		{Name: "Paris", Lon: 2.35, Lat: 48.86, Value: 11},
		{Name: "Jakarta", Lon: 106.85, Lat: -6.21, Value: 10.8},
		{Name: "Lima", Lon: -77.04, Lat: -12.05, Value: 10.7},
		{Name: "London", Lon: -0.13, Lat: 51.51, Value: 9.5},
	}
	quakes = []charta.Dot{
		{Lon: 142.8, Lat: 36.4, Group: "Shallow"}, {Lon: 143.5, Lat: 41.9, Group: "Shallow"}, {Lon: 145.8, Lat: 41.8, Group: "Shallow"}, {Lon: 140.7, Lat: 36.7, Group: "Deep"},
		{Lon: 145.2, Lat: 40.3, Group: "Shallow"}, {Lon: 138.1, Lat: 37.5, Group: "Deep"}, {Lon: 142.7, Lat: 35.8, Group: "Shallow"}, {Lon: 142.5, Lat: 35.2, Group: "Deep"},
		{Lon: 120.4, Lat: -5.4, Group: "Deep"}, {Lon: 120.5, Lat: -2.5, Group: "Shallow"}, {Lon: 119.4, Lat: -5.8, Group: "Deep"}, {Lon: 117.1, Lat: -2.3, Group: "Deep"},
		{Lon: 121.1, Lat: -8.2, Group: "Shallow"}, {Lon: 123.3, Lat: -4.1, Group: "Shallow"}, {Lon: 116.2, Lat: -2, Group: "Shallow"}, {Lon: 119.8, Lat: -5.7, Group: "Shallow"},
		{Lon: -73.7, Lat: -33.2, Group: "Shallow"}, {Lon: -68.5, Lat: -32.1, Group: "Shallow"}, {Lon: -70.2, Lat: -27, Group: "Shallow"}, {Lon: -72.4, Lat: -33.9, Group: "Shallow"},
		{Lon: -72.8, Lat: -26.2, Group: "Shallow"}, {Lon: -69.8, Lat: -29.2, Group: "Shallow"}, {Lon: -70.7, Lat: -30.8, Group: "Deep"}, {Lon: -74, Lat: -31.9, Group: "Deep"},
		{Lon: -149.8, Lat: 56, Group: "Shallow"}, {Lon: -151.9, Lat: 58.1, Group: "Deep"}, {Lon: -148.7, Lat: 62.8, Group: "Shallow"}, {Lon: -154.2, Lat: 56.6, Group: "Shallow"},
		{Lon: -148.1, Lat: 63.5, Group: "Deep"}, {Lon: -153.5, Lat: 59, Group: "Deep"}, {Lon: -149.9, Lat: 56.5, Group: "Shallow"}, {Lon: -154.5, Lat: 56.3, Group: "Deep"},
		{Lon: -96, Lat: 16.1, Group: "Shallow"}, {Lon: -98.6, Lat: 14, Group: "Shallow"}, {Lon: -96.6, Lat: 15.2, Group: "Shallow"}, {Lon: -98.6, Lat: 18.4, Group: "Shallow"},
		{Lon: -102.7, Lat: 13.7, Group: "Shallow"}, {Lon: -101.5, Lat: 14, Group: "Deep"}, {Lon: -102.8, Lat: 18.9, Group: "Deep"}, {Lon: -102.2, Lat: 20.3, Group: "Deep"},
		{Lon: 122.4, Lat: 12, Group: "Shallow"}, {Lon: 124.9, Lat: 8.5, Group: "Shallow"}, {Lon: 124.7, Lat: 11.5, Group: "Shallow"}, {Lon: 124.6, Lat: 6.4, Group: "Deep"},
		{Lon: 124.8, Lat: 11.2, Group: "Deep"}, {Lon: 122, Lat: 13.1, Group: "Shallow"}, {Lon: 122.9, Lat: 8.4, Group: "Shallow"}, {Lon: 128.5, Lat: 8.6, Group: "Deep"},
		{Lon: 173.2, Lat: -40.6, Group: "Deep"}, {Lon: 175.8, Lat: -37.1, Group: "Shallow"}, {Lon: 173.1, Lat: -42.8, Group: "Deep"}, {Lon: 175.2, Lat: -38.5, Group: "Shallow"},
		{Lon: 175.1, Lat: -39.5, Group: "Deep"}, {Lon: 177.8, Lat: -39.2, Group: "Shallow"}, {Lon: 174.9, Lat: -43.2, Group: "Shallow"}, {Lon: 174.4, Lat: -38.4, Group: "Shallow"},
		{Lon: 54.3, Lat: 33.8, Group: "Shallow"}, {Lon: 54.4, Lat: 27.1, Group: "Deep"}, {Lon: 52.9, Lat: 26.2, Group: "Shallow"}, {Lon: 55.5, Lat: 27.3, Group: "Shallow"},
		{Lon: 51, Lat: 32.8, Group: "Shallow"}, {Lon: 52.2, Lat: 30, Group: "Shallow"}, {Lon: 56.9, Lat: 27.2, Group: "Shallow"}, {Lon: 55.6, Lat: 30.4, Group: "Shallow"},
		{Lon: 36.2, Lat: 38.8, Group: "Deep"}, {Lon: 33.6, Lat: 41.1, Group: "Shallow"}, {Lon: 38.5, Lat: 39.2, Group: "Deep"}, {Lon: 31.8, Lat: 38.3, Group: "Shallow"},
		{Lon: 33, Lat: 41, Group: "Shallow"}, {Lon: 36.4, Lat: 41.3, Group: "Shallow"}, {Lon: 38.1, Lat: 35.6, Group: "Deep"}, {Lon: 32.9, Lat: 34.4, Group: "Deep"},
		{Lon: -75, Lat: -13.3, Group: "Shallow"}, {Lon: -79.8, Lat: -10, Group: "Deep"}, {Lon: -72.1, Lat: -8.6, Group: "Deep"}, {Lon: -78.7, Lat: -8.1, Group: "Shallow"},
		{Lon: -75.9, Lat: -11.9, Group: "Shallow"}, {Lon: -79.3, Lat: -12.6, Group: "Shallow"}, {Lon: -76.5, Lat: -9.6, Group: "Shallow"}, {Lon: -74.3, Lat: -9, Group: "Shallow"},
	}
	airports = []charta.Location{
		{Name: "Singapore", Lon: 103.82, Lat: 1.35},
		{Name: "London", Lon: -0.13, Lat: 51.51},
		{Name: "Sydney", Lon: 151.21, Lat: -33.87},
		{Name: "Tokyo", Lon: 139.69, Lat: 35.69},
		{Name: "Dubai", Lon: 55.27, Lat: 25.2},
		{Name: "Mumbai", Lon: 72.88, Lat: 19.08},
		{Name: "Beijing", Lon: 116.41, Lat: 39.9},
		{Name: "San Francisco", Lon: -122.42, Lat: 37.77},
		{Name: "Johannesburg", Lon: 28.05, Lat: -26.2},
		{Name: "Frankfurt", Lon: 8.68, Lat: 50.11},
		{Name: "Auckland", Lon: 174.76, Lat: -36.85},
	}
	flights = []charta.Link{
		link("Singapore", "London", 28), link("Singapore", "Sydney", 35), link("Singapore", "Tokyo", 42), link("Singapore", "Dubai", 21),
		link("Singapore", "Mumbai", 18), link("Singapore", "Beijing", 24), link("Singapore", "San Francisco", 7), link("Singapore", "Johannesburg", 5),
		link("Singapore", "Frankfurt", 14), link("Singapore", "Auckland", 9),
	}
	coffeePlaces = []charta.Location{
		{Name: "Brazil", Lon: -51.9, Lat: -14.2},
		{Name: "Vietnam", Lon: 108.3, Lat: 14.1},
		{Name: "Colombia", Lon: -74.3, Lat: 4.6},
		{Name: "Ethiopia", Lon: 40.5, Lat: 9.1},
		{Name: "USA", Lon: -98.6, Lat: 39.8},
		{Name: "Germany", Lon: 10.5, Lat: 51.2},
		{Name: "Japan", Lon: 138.3, Lat: 36.2},
	}
	coffee = []charta.Link{
		link("Brazil", "USA", 400), link("Brazil", "Germany", 300), link("Vietnam", "Germany", 200), link("Vietnam", "USA", 150),
		link("Colombia", "USA", 250), link("Ethiopia", "Germany", 80), link("Brazil", "Japan", 120), link("Vietnam", "Japan", 100),
	}
	words = []charta.Word{
		word("data", 90), word("chart", 72), word("color", 60), word("scale", 55), word("axis", 50),
		word("legend", 44), word("tooltip", 40), word("label", 38), word("grid", 33), word("series", 31),
		word("bar", 29), word("line", 27), word("area", 25), word("hover", 23), word("dark mode", 22),
		word("contrast", 20), word("palette", 19), word("marker", 17), word("baseline", 16), word("tick", 15),
		word("zoom", 14), word("brush", 13), word("small multiples", 12), word("annotation", 11), word("density", 10),
		word("outlier", 9), word("trend", 9), word("ramp", 8), word("hue", 8), word("pixel", 7),
		word("layout", 7), word("glyph", 6), word("texture", 6), word("focus", 5), word("story", 5),
	}
)

// The relationship demos' links, in the Rust demos' order: nodes appear in
// first-mention order, so order is layout.
var (
	network = []charta.Link{
		link("Ana", "Ben", 3), link("Ana", "Cleo", 2), link("Ben", "Cleo", 2), link("Ana", "Dev", 1), link("Cleo", "Dev", 1),
		link("Eli", "Fay", 3), link("Eli", "Gus", 2), link("Fay", "Gus", 1), link("Fay", "Hal", 2), link("Gus", "Hal", 1),
		link("Ivy", "Jon", 2), link("Ivy", "Kai", 2), link("Jon", "Kai", 1), link("Kai", "Lea", 3), link("Jon", "Lea", 1),
		link("Lea", "Max", 1), link("Max", "Ivy", 1), link("Dev", "Eli", 1), link("Hal", "Ivy", 1), link("Cleo", "Kai", 1),
	}
	papers = []charta.Link{
		link("Kim", "Lee", 4), link("Kim", "Park", 2), link("Lee", "Park", 3), link("Park", "Choi", 1), link("Choi", "Jung", 2),
		link("Jung", "Kang", 3), link("Kang", "Yoon", 1), link("Kim", "Yoon", 2), link("Lee", "Kang", 1), link("Choi", "Yoon", 1),
		link("Jung", "Han", 2),
	}
	migration = []charta.Link{
		link("North", "South", 30), link("South", "North", 12), link("North", "East", 18), link("East", "North", 9),
		link("East", "West", 15), link("West", "East", 22), link("West", "South", 25), link("South", "West", 8),
		link("North", "West", 10), link("South", "East", 14), link("Central", "North", 16), link("North", "Central", 6),
		link("Central", "South", 11), link("East", "Central", 7),
	}
	deps = []charta.Link{
		link("core", "utils", 2), link("core", "io", 2), link("core", "log", 1), link("io", "net", 1), link("io", "fs", 2),
		link("net", "http", 2), link("http", "json", 1), link("http", "auth", 1), link("auth", "crypto", 2), link("fs", "path", 1),
		link("log", "format", 1), link("json", "format", 1), link("cli", "core", 2), link("cli", "format", 1),
		link("test", "core", 1), link("test", "utils", 1),
	}
	energy = []charta.Link{
		link("Coal", "Electricity", 120), link("Gas", "Electricity", 90), link("Gas", "Homes", 60), link("Gas", "Industry", 50),
		link("Nuclear", "Electricity", 80), link("Renewables", "Electricity", 70), link("Electricity", "Homes", 110),
		link("Electricity", "Industry", 130), link("Electricity", "Losses", 120),
	}
)

func word(text string, weight float64) charta.Word {
	return charta.Word{Text: text, Weight: weight}
}

func link(source, target string, v float64) charta.Link {
	return charta.Link{Source: source, Target: target, Value: v}
}

func titanic() charta.ParallelSetsChart {
	c := charta.ParallelSetsChart{Title: "Titanic passengers", Dimensions: []string{"Class", "Sex", "Survived"}}
	for _, r := range []struct {
		class, sex, survived string
		n                    float64
	}{
		{"First", "Male", "No", 118}, {"First", "Male", "Yes", 62}, {"First", "Female", "No", 4}, {"First", "Female", "Yes", 141},
		{"Second", "Male", "No", 154}, {"Second", "Male", "Yes", 25}, {"Second", "Female", "No", 13}, {"Second", "Female", "Yes", 93},
		{"Third", "Male", "No", 422}, {"Third", "Male", "Yes", 88}, {"Third", "Female", "No", 106}, {"Third", "Female", "Yes", 90},
		{"Crew", "Male", "No", 670}, {"Crew", "Male", "Yes", 192}, {"Crew", "Female", "No", 3}, {"Crew", "Female", "Yes", 20},
	} {
		c.Combinations = append(c.Combinations, charta.Combination{Categories: []string{r.class, r.sex, r.survived}, Count: r.n})
	}
	return c
}

func orderFlow() charta.FlowChart {
	return charta.FlowChart{
		Title: "Order handling",
		Steps: []charta.Step{
			{ID: "order", Label: "Order placed", Kind: charta.Terminal},
			{ID: "stock", Label: "In stock?", Kind: charta.Decision},
			{ID: "pack", Label: "Pack", Kind: charta.Process},
			{ID: "reorder", Label: "Reorder", Kind: charta.Process},
			{ID: "ship", Label: "Ship", Kind: charta.Process},
			{ID: "wait", Label: "Wait for stock", Kind: charta.Process},
			{ID: "done", Label: "Delivered", Kind: charta.Terminal},
		},
		Arrows: []charta.Arrow{
			{From: "order", To: "stock"}, {From: "stock", To: "pack", Label: "Yes"}, {From: "stock", To: "reorder", Label: "No"},
			{From: "pack", To: "ship"}, {From: "reorder", To: "wait"}, {From: "wait", To: "stock"}, {From: "ship", To: "done"},
		},
	}
}

func leaf(name string, v float64) charta.Node { return charta.Node{Name: name, Value: &v} }

func branch(name string, kids ...charta.Node) charta.Node {
	return charta.Node{Name: name, Children: kids}
}

func revenue() charta.Node {
	return branch("Revenue",
		branch("Software", leaf("Cloud", 42), leaf("Licenses", 18), leaf("Support", 12), leaf("Training", 4)),
		branch("Hardware", leaf("Servers", 28), leaf("Storage", 16), leaf("Networking", 9)),
		branch("Services", leaf("Consulting", 22), leaf("Managed", 14)),
	)
}

func teams() charta.Node {
	return branch("Company",
		branch("Engineering", leaf("Frontend", 12), leaf("Backend", 18), leaf("Mobile", 8), leaf("Data", 10), leaf("Platform", 6)),
		branch("Product", leaf("PM", 6), leaf("Design", 9), leaf("Research", 3)),
		branch("Go-to-market", leaf("Sales", 15), leaf("Marketing", 8), leaf("Support", 11)),
	)
}

func disk() charta.Node {
	return branch("Disk",
		branch("Documents", branch("Work", leaf("Reports", 12), leaf("Slides", 8)), leaf("Personal", 6)),
		branch("Media", leaf("Photos", 48), leaf("Videos", 72), leaf("Music", 20)),
		branch("Apps", leaf("System", 35), leaf("Games", 40), leaf("Tools", 10)),
	)
}

// topic is a node whose children, named by kids, are bare leaves.
func topic(name string, kids ...string) charta.Node {
	n := charta.Node{Name: name}
	for _, k := range kids {
		n.Children = append(n.Children, charta.Node{Name: k})
	}
	return n
}

func org() charta.Node {
	return branch("CEO",
		branch("CTO", charta.Node{Name: "Platform"}, topic("Product eng", "Web", "Mobile"), charta.Node{Name: "Data"}),
		topic("CFO", "Finance", "Legal"),
		topic("COO", "Sales", "Support", "Operations"),
	)
}

func launchPlan() charta.Node {
	return branch("Launch plan",
		topic("Marketing", "Social", "Email", "Ads"),
		topic("Product", "Features", "Pricing", "Docs"),
		topic("Sales", "Training", "Partners"),
		topic("Support", "FAQ", "Onboarding"),
	)
}

var monthNames = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

func radialColumn() charta.RadialColumnChart {
	rain := []float64{78, 62, 55, 48, 61, 84, 102, 96, 73, 88, 95, 86}
	data := make([]charta.Bar, len(rain))
	for i, v := range rain {
		data[i] = charta.Bar{Label: monthNames[i], Value: v}
	}
	return charta.RadialColumnChart{Title: "Rainfall by month (mm)", Data: data}
}

// lcg is a fixed pseudo-random sequence (the classic C rand LCG): demo
// series look organic yet stay identical run to run and match the Rust
// demos exactly, because it is all integer arithmetic.
type lcg uint64

func (r *lcg) below(n uint64) int64 {
	*r = (*r*1_103_515_245 + 12_345) % (1 << 31)
	return int64(uint64(*r) % n)
}

// candles is six weeks of trading days from Monday 2 March 2026, prices in
// cents.
func candles() []charta.Candle {
	r := lcg(7)
	price := int64(10_000)
	start := base.ToMS(2026, 3, 2, 0, 0, 0)
	var out []charta.Candle
	for d := int64(0); d < 42; d++ {
		if d%7 >= 5 {
			continue
		}
		open := price
		cl := open + r.below(401) - 200
		high := max(open, cl) + r.below(150)
		low := min(open, cl) - r.below(150)
		price = cl
		c := func(v int64) float64 { return float64(v) / 100 }
		out = append(out, charta.Candle{Time: start + d*base.Day, Open: c(open), High: c(high), Low: c(low), Close: c(cl)})
	}
	return out
}

// closes is 150 daily closes from 1 January 2026, trending up and down by
// month.
func closes() []charta.TimeValue {
	r := lcg(11)
	price := int64(5_000)
	start := base.ToMS(2026, 1, 1, 0, 0, 0)
	out := make([]charta.TimeValue, 0, 150)
	for d := int64(0); d < 150; d++ {
		drift := int64(-8)
		if (d/30)%2 == 0 {
			drift = 8
		}
		price += r.below(301) - 150 + drift
		out = append(out, charta.TimeValue{Time: start + d*base.Day, Value: float64(price) / 100})
	}
	return out
}

func gantt() charta.GanttChart {
	day := func(m, d int) int64 { return base.ToMS(2026, m, d, 0, 0, 0) }
	return charta.GanttChart{
		Title: "Website relaunch",
		Tasks: []charta.Task{
			{Name: "Research", Start: day(1, 5), End: day(1, 17), Group: "Design"},
			{Name: "Wireframes", Start: day(1, 12), End: day(1, 31), Group: "Design"},
			{Name: "Visual design", Start: day(1, 26), End: day(2, 21), Group: "Design"},
			{Name: "Backend", Start: day(2, 2), End: day(3, 14), Group: "Build"},
			{Name: "Frontend", Start: day(2, 16), End: day(3, 21), Group: "Build"},
			{Name: "QA", Start: day(3, 16), End: day(4, 1), Group: "Launch"},
			{Name: "Go live", Start: day(4, 1), End: day(4, 1), Group: "Launch"},
		},
	}
}

func timeline() charta.TimelineChart {
	at := func(y int64, m, d int) int64 { return base.ToMS(y, m, d, 0, 0, 0) }
	return charta.TimelineChart{
		Title: "Company milestones",
		Events: []charta.Event{
			{Time: at(2019, 3, 14), Label: "Founded"},
			{Time: at(2019, 11, 2), Label: "First prototype"},
			{Time: at(2020, 6, 20), Label: "Seed round"},
			{Time: at(2021, 2, 10), Label: "Beta launch"},
			{Time: at(2021, 9, 1), Label: "1.0 release"},
			{Time: at(2022, 5, 17), Label: "10k users"},
			{Time: at(2023, 8, 8), Label: "Series A"},
			{Time: at(2024, 4, 22), Label: "Mobile app"},
			{Time: at(2025, 10, 30), Label: "1M users"},
		},
	}
}

func stream() charta.StreamChart {
	times := make([]int64, 12)
	for m := range times {
		times[m] = base.ToMS(2025, m+1, 1, 0, 0, 0)
	}
	return charta.StreamChart{
		Title: "Listening hours by genre (k)",
		Times: times,
		Series: []charta.Series{
			{Name: "Pop", Values: []float64{22, 24, 23, 27, 30, 36, 41, 39, 32, 28, 26, 31}},
			{Name: "Rock", Values: []float64{18, 17, 19, 18, 16, 15, 17, 18, 19, 21, 20, 19}},
			{Name: "Hip-hop", Values: []float64{9, 11, 13, 16, 18, 21, 24, 25, 22, 20, 19, 21}},
			{Name: "Jazz", Values: []float64{6, 6, 5, 5, 4, 4, 4, 5, 6, 7, 8, 10}},
		},
	}
}

// calendar is commits per day through 2025, quieter at weekends.
func calendar() charta.CalendarChart {
	r := lcg(3)
	start := base.ToMS(2025, 1, 1, 0, 0, 0)
	days := make([]charta.TimeValue, 365)
	for d := range days {
		// 1 January 2025 was a Wednesday (2 days after Monday).
		v := r.below(10)
		if (d+2)%7 >= 5 {
			v /= 3
		}
		days[d] = charta.TimeValue{Time: start + int64(d)*base.Day, Value: float64(v)}
	}
	return charta.CalendarChart{Title: "Commits per day, 2025", Days: days}
}

func timetable() charta.TimetableChart {
	slot := func(day string, h0, m0, h1, m1 int, label, group string) charta.Slot {
		return charta.Slot{Day: day, Start: h0*60 + m0, End: h1*60 + m1, Label: label, Group: group}
	}
	return charta.TimetableChart{
		Title: "Class timetable",
		Days:  []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
		Slots: []charta.Slot{
			slot("Mon", 9, 0, 10, 30, "Maths", "Lecture"),
			slot("Mon", 11, 0, 12, 0, "Physics", "Lecture"),
			slot("Mon", 14, 0, 16, 0, "Chemistry", "Lab"),
			slot("Tue", 9, 0, 10, 0, "English", "Seminar"),
			slot("Tue", 10, 30, 12, 0, "Maths", "Lecture"),
			slot("Wed", 13, 0, 15, 0, "Physics", "Lab"),
			slot("Wed", 14, 0, 15, 0, "Tutorial", "Seminar"),
			slot("Thu", 9, 0, 10, 30, "Maths", "Lecture"),
			slot("Thu", 11, 0, 12, 30, "History", "Seminar"),
			slot("Fri", 10, 0, 12, 0, "Biology", "Lab"),
			slot("Fri", 13, 0, 14, 0, "Physics", "Lecture"),
		},
	}
}

// spiral is the monthly mean temperature, 2023–2025: a seasonal cycle in
// tenths of a degree, warming a little each year.
func spiral() charta.SpiralChart {
	baseTemp := [12]int64{40, 50, 85, 120, 160, 195, 220, 215, 180, 130, 80, 50}
	names := monthNames
	r := lcg(5)
	data := make([]charta.Bar, 36)
	for i := range data {
		y, m := i/12, i%12
		t := baseTemp[m] + int64(y)*4 + r.below(21) - 10
		data[i] = charta.Bar{Label: fmt.Sprintf("%s %d", names[m], 2023+y), Value: float64(t) / 10}
	}
	return charta.SpiralChart{Title: "Monthly mean temperature (°C)", Period: 12, AngleLabels: names, Data: data}
}

func laptops() charta.ParallelChart {
	return charta.ParallelChart{
		Title:      "Laptops",
		Dimensions: []string{"Price ($)", "Weight (kg)", "Battery (h)", "Rating"},
		Rows: []charta.ParallelRow{
			{Label: "Air 13", Group: "Ultrabook", Values: []float64{1099, 1.24, 18, 4.7}},
			{Label: "Zen 14", Group: "Ultrabook", Values: []float64{899, 1.39, 14, 4.3}},
			{Label: "XPS 13", Group: "Ultrabook", Values: []float64{1199, 1.17, 12, 4.5}},
			{Label: "Legion 5", Group: "Gaming", Values: []float64{1299, 2.4, 5, 4.4}},
			{Label: "ROG 16", Group: "Gaming", Values: []float64{1799, 2.5, 6, 4.6}},
			{Label: "Nitro 5", Group: "Gaming", Values: []float64{849, 2.6, 4, 4.0}},
			{Label: "Chromebook", Group: "Budget", Values: []float64{299, 1.5, 10, 3.8}},
			{Label: "IdeaPad 3", Group: "Budget", Values: []float64{449, 1.65, 7, 3.9}},
		},
	}
}

// Line is the multi-series line demo, exposed on its own because the tests
// drive its crosshair and legend.
func Line() charta.LineChart {
	return charta.LineChart{
		Title:      "Weekly signups",
		Categories: months,
		Series: []charta.Series{
			{Name: "Web", Values: []float64{120, 135, 128, 160, 172, 190}},
			{Name: "iOS", Values: []float64{80, 95, 110, 108, 125, 140}},
			{Name: "Android", Values: []float64{60, 70, 66, 85, 90, 104}},
		},
	}
}
