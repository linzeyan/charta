// Command gio-gallery shows every chart charta can draw in a Gio window:
// pick a chart on the left; hover for tooltips, click legend entries to hide
// or show a series.
//
// Usage: go run ./cmd/gio-gallery [-dark]
package main

import (
	"flag"
	"image/color"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/gio"
	"github.com/linzeyan/charta/internal/demo"
)

func main() {
	dark := flag.Bool("dark", false, "use the dark palette")
	flag.Parse()
	go func() {
		w := new(app.Window)
		w.Option(app.Title("charta gio gallery"), app.Size(unit.Dp(920), unit.Dp(540)))
		if err := run(w, *dark); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func nrgba(c base.Color, dark bool) color.NRGBA {
	r := base.Resolve(c, dark)
	return color.NRGBA{R: r.R, G: r.G, B: r.B, A: r.A}
}

func run(w *app.Window, dark bool) error {
	th := material.NewTheme()
	th.Bg, th.Fg = nrgba(base.Surface, dark), nrgba(base.TextPrimary, dark)
	demos := demo.All()
	// One view per demo, so each keeps its own hidden series across picks.
	views := make([]*gio.ChartView, len(demos))
	buttons := make([]widget.Clickable, len(demos))
	// The catalogue outgrows any window height, so the picker scrolls.
	list := widget.List{List: layout.List{Axis: layout.Vertical}}
	for i, d := range demos {
		views[i] = &gio.ChartView{Chart: d.Chart, Shaper: th.Shaper, Dark: dark}
	}
	pick := 0
	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			for i := range buttons {
				if buttons[i].Clicked(gtx) {
					pick = i
				}
			}
			paint.FillShape(gtx.Ops, th.Bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
			layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return material.List(th, &list).Layout(gtx, len(demos), func(gtx layout.Context, i int) layout.Dimensions {
							return buttons[i].Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								l := material.Body1(th, demos[i].Name)
								l.Color = nrgba(base.TextSecondary, dark)
								if i == pick {
									l.Color, l.Font.Weight = th.Fg, font.SemiBold
								}
								return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Right: unit.Dp(12)}.Layout(gtx, l.Layout)
							})
						})
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return views[pick].Layout(gtx)
				}),
			)
			e.Frame(gtx.Ops)
		}
	}
}
