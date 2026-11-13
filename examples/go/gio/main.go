// Command gio shows one interactive chart in a Gio window: hover for
// tooltips, click legend entries to hide a series. Gio needs cgo on macOS
// and Linux.
//
// Usage: go run ./examples/go/gio
package main

import (
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/linzeyan/charta"
	"github.com/linzeyan/charta/gio"
)

func main() {
	chart := charta.LineChart{
		Title:      "Weekly active users (k)",
		Categories: []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
		Series: []charta.Series{
			{Name: "Web", Values: []float64{120, 132, 101, 134, 90, 230, 210}},
			{Name: "iOS", Values: []float64{220, 182, 191, 234, 290, 330, 310}},
		},
	}
	go func() {
		w := new(app.Window)
		w.Option(app.Title("charta in Gio"), app.Size(unit.Dp(720), unit.Dp(440)))
		// Keep the view across frames: it remembers the pointer, the hidden
		// series and the zoom.
		view := &gio.ChartView{Chart: chart, Shaper: text.NewShaper(text.WithCollection(gofont.Collection()))}
		var ops op.Ops
		for {
			switch e := w.Event().(type) {
			case app.DestroyEvent:
				if e.Err != nil {
					log.Fatal(e.Err)
				}
				os.Exit(0)
			case app.FrameEvent:
				gtx := app.NewContext(&ops, e)
				view.Layout(gtx)
				e.Frame(gtx.Ops)
			}
		}
	}()
	app.Main()
}
