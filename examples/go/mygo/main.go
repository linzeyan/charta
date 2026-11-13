// Command mygo shows one interactive chart in a MyGo native window, drawn by
// MyGo on the GPU: hover for tooltips, click legend entries to hide a series,
// and zoom with the wheel, drag to pan, shift-drag to brush, double click to
// reset. No cgo, so it cross-compiles from any machine.
//
// Usage: go run ./examples/go/mygo
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/linzeyan/charta"
	chartamygo "github.com/linzeyan/charta/mygo"
)

func main() {
	months := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	// Keep the view across frames: it remembers the pointer, the hidden
	// series and the zoom. It follows the system's light or dark look.
	view := &chartamygo.ChartView{Chart: charta.LineChart{
		Title:      "Visits by month (k)",
		Categories: months,
		Series: []charta.Series{
			{Name: "Organic", Values: []float64{31, 35, 40, 38, 45, 52, 58, 55, 49, 44, 40, 47}},
			{Name: "Paid", Values: []float64{12, 15, 14, 20, 22, 25, 24, 28, 26, 21, 19, 30}},
		},
	}}
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:   "charta in MyGo",
			Width:   720,
			Height:  440,
			Content: ui.View(func(c *ui.Context) { view.Build(c).Fill() }),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
