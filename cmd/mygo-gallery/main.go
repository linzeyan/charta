// Command mygo-gallery shows every chart charta can draw in a MyGo native
// window: pick a chart on the left; hover for tooltips, click legend entries
// to hide or show a series, and zoom charts with an axis to zoom. The palette
// follows the system's light or dark appearance.
//
// Usage: go run ./cmd/mygo-gallery
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/linzeyan/charta/internal/demo"
	chartamygo "github.com/linzeyan/charta/mygo"
)

type gallery struct {
	demos []demo.Demo
	// One view per demo, so each keeps its own hidden series and zoom
	// across picks.
	views []*chartamygo.ChartView
	pick  int
}

func (g *gallery) view(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Fill().Children(func() {
		// The catalogue outgrows any window height, so the picker scrolls.
		ui.Scroll(c).Width(200).Children(func() {
			ui.Column(c).Padding(16).Gap(8).Children(func() {
				for i, d := range g.demos {
					item := ui.Text(c.Key(d.Name), d.Name).Cursor(ui.CursorPointer).TextColor(t.TextMuted)
					if i == g.pick {
						item.TextColor(t.Text).Bold()
					}
					if item.Clicked() {
						g.pick = i
					}
				}
			})
		})
		g.views[g.pick].Build(c.Key(g.demos[g.pick].Name)).Grow(1)
	})
}

func main() {
	g := &gallery{demos: demo.All()}
	for _, d := range g.demos {
		g.views = append(g.views, &chartamygo.ChartView{Chart: d.Chart})
	}
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:   "charta mygo gallery",
			Width:   920,
			Height:  540,
			Content: ui.View(g.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
