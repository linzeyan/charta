package charta_test

import (
	"math"
	"testing"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/demo"
	"github.com/linzeyan/charta/scene"
)

// The hierarchy layouts are iterative (front chains, contours, row
// greedy choices), so one changed comparison or operation order moves marks
// without breaking any invariant. These are the Rust crate's positions for
// the gallery demos at 640×400, as its conformance fixtures record them
// (two decimals): the Go port must place every mark in the same spot.
var hierarchyReference = map[string]map[string][]float64{
	"Circle packing": {
		"Engineering":              {252.46, 224.72, 86.69},
		"Engineering / Frontend":   {302.57, 225.57, 29.37},
		"Engineering / Backend":    {234.49, 225.57, 35.97},
		"Engineering / Mobile":     {274.29, 177.14, 23.98},
		"Engineering / Data":       {274.56, 277.41, 26.81},
		"Engineering / Platform":   {228.02, 166.45, 20.77},
		"Product":                  {347.32, 327.3, 50.31},
		"Product / PM":             {374.13, 327.3, 20.77},
		"Product / Design":         {325.19, 327.3, 25.44},
		"Product / Research":       {353.52, 359.46, 14.69},
		"Go-to-market":             {408.06, 224.72, 66.18},
		"Go-to-market / Sales":     {378.65, 216.23, 32.84},
		"Go-to-market / Marketing": {414.74, 263.61, 23.98},
		"Go-to-market / Support":   {442.35, 216.23, 28.12},
	},
	"Treemap": {
		"Software / Cloud":      {19, 87, 191.23, 231.68},
		"Software / Licenses":   {212.23, 87, 80.81, 231.68},
		"Software / Support":    {19, 320.68, 205.04, 60.32},
		"Software / Training":   {226.04, 320.68, 67.01, 60.32},
		"Hardware / Servers":    {299.05, 87, 169.14, 166.99},
		"Hardware / Storage":    {470.19, 87, 95.8, 166.99},
		"Hardware / Networking": {567.99, 87, 53.01, 166.99},
		"Services / Consulting": {299.05, 273.99, 195.97, 107.01},
		"Services / Managed":    {497.02, 273.99, 123.98, 107.01},
	},
	"Tree diagram": {
		"CEO":                        {47.06, 215, 4},
		"CTO":                        {217.69, 107.45, 4},
		"CTO / Platform":             {388.33, 76.73, 4},
		"CTO / Product eng":          {388.33, 107.45, 4},
		"CTO / Product eng / Web":    {558.96, 92.09, 4},
		"CTO / Product eng / Mobile": {558.96, 122.82, 4},
		"CTO / Data":                 {388.33, 138.18, 4},
		"CFO":                        {217.69, 215, 4},
		"CFO / Finance":              {388.33, 199.64, 4},
		"CFO / Legal":                {388.33, 230.36, 4},
		"COO":                        {217.69, 322.55, 4},
		"COO / Sales":                {388.33, 291.82, 4},
		"COO / Support":              {388.33, 322.55, 4},
		"COO / Operations":           {388.33, 353.27, 4},
	},
	"Brainstorm": {
		"Marketing":            {340.4, 174.73, 63.97, 21},
		"Marketing / Social":   {378.14, 106.27, 4},
		"Marketing / Email":    {424.77, 143.45, 4},
		"Marketing / Ads":      {450.64, 197.18, 4},
		"Product":              {323.05, 276.86, 52.05, 21},
		"Product / Features":   {424.77, 310.55, 4},
		"Product / Pricing":    {378.14, 347.73, 4},
		"Product / Docs":       {320, 361, 4},
		"Sales":                {240.19, 245.57, 38.9, 21},
		"Sales / Training":     {215.23, 310.55, 4},
		"Sales / Partners":     {189.36, 256.82, 4},
		"Support":              {251.94, 164.12, 52.57, 21},
		"Support / FAQ":        {215.23, 143.45, 4},
		"Support / Onboarding": {261.86, 106.27, 4},
		"Launch plan":          {270.82, 212, 98.35, 30},
	},
	// Each sector's first point: where its outer arc starts.
	"Sunburst": {
		"Documents":                  {320, 148.5},
		"Documents / Work":           {320, 109.25},
		"Documents / Work / Reports": {320, 70},
		"Documents / Work / Slides":  {366.46, 77.03},
		"Documents / Personal":       {376.52, 123.7},
		"Media":                      {367.56, 164.55},
		"Media / Photos":             {391.34, 133.32},
		"Media / Videos":             {433.11, 259.72},
		"Media / Music":              {262.19, 329.58},
		"Apps":                       {253.36, 268.5},
		"Apps / System":              {220.05, 289.25},
		"Apps / Games":               {208.2, 190.05},
		"Apps / Tools":               {290.83, 112.92},
	},
	// Keyed by category and series: the hit label is the category.
	"Marimekko": {
		"North America 0": {54.15, 208.09, 170.04, 155.91},
		"North America 1": {54.15, 129.14, 170.04, 76.95},
		"North America 2": {54.15, 76.5, 170.04, 50.64},
		"Europe 0":        {226.19, 250.2, 154.4, 113.8},
		"Europe 1":        {226.19, 119.92, 154.4, 128.28},
		"Europe 2":        {226.19, 76.5, 154.4, 41.43},
		"Asia 0":          {382.59, 299.19, 201.31, 64.81},
		"Asia 1":          {382.59, 176.71, 201.31, 120.48},
		"Asia 2":          {382.59, 76.5, 201.31, 98.21},
		"Other 0":         {585.9, 279.15, 37.1, 84.85},
		"Other 1":         {585.9, 221.25, 37.1, 55.9},
		"Other 2":         {585.9, 76.5, 37.1, 142.75},
	},
}

func TestHierarchyLayoutsMatchTheRustReference(t *testing.T) {
	round := func(v float64) float64 { return math.Round(v*100) / 100 }
	seen := 0
	for _, d := range demo.All() {
		want, ok := hierarchyReference[d.Name]
		if !ok {
			continue
		}
		seen++
		s, err := d.Chart.Scene(640, 400, base.InterMetrics{})
		if err != nil {
			t.Fatal(err)
		}
		got := map[string][]float64{}
		series := map[string]int{}
		for _, m := range s.Marks {
			if m.Hit == nil || m.Hit.Kind != scene.HitMark {
				continue
			}
			key := m.Hit.Label
			var g []float64
			switch sh := m.Shape.(type) {
			case scene.CircleShape:
				g = []float64{sh.CX, sh.CY, sh.R}
			case scene.RectShape:
				g = []float64{sh.Rect.X, sh.Rect.Y, sh.Rect.W, sh.Rect.H}
				if d.Name == "Marimekko" {
					key = key + " " + string(rune('0'+series[key]))
					series[m.Hit.Label]++
				}
			case scene.PathShape:
				g = []float64{sh.Ops[0].X, sh.Ops[0].Y}
			}
			for i := range g {
				g[i] = round(g[i])
			}
			got[key] = g
		}
		if len(got) != len(want) {
			t.Errorf("%s: %d marks, want %d", d.Name, len(got), len(want))
		}
		for k, w := range want {
			g := got[k]
			if len(g) != len(w) {
				t.Errorf("%s %q: got %v, want %v", d.Name, k, g, w)
				continue
			}
			for i := range w {
				if g[i] != w[i] {
					t.Errorf("%s %q: got %v, want %v", d.Name, k, g, w)
					break
				}
			}
		}
	}
	if seen != len(hierarchyReference) {
		t.Fatalf("found %d of %d demos", seen, len(hierarchyReference))
	}
}
