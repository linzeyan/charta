// Command fyne-gallery shows every chart charta can draw, as Fyne widgets.
// The palette follows the app's light/dark theme variant.
//
// Usage: go run ./cmd/fyne-gallery
package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	chartafyne "github.com/linzeyan/charta/fyne"
	"github.com/linzeyan/charta/internal/demo"
)

func main() {
	a := app.New()
	w := a.NewWindow("charta fyne gallery")
	demos := demo.All()
	// One view per demo, so each remembers its own hidden series.
	views := make([]*chartafyne.ChartView, len(demos))
	for i, d := range demos {
		views[i] = chartafyne.NewChartView(d.Chart)
	}
	dark := func() bool { return a.Settings().ThemeVariant() == theme.VariantDark }
	stage := container.NewStack(views[0])
	show := func(i int) {
		views[i].Dark = dark()
		stage.Objects = []fyne.CanvasObject{views[i]}
		stage.Refresh()
	}
	a.Settings().AddListener(func(fyne.Settings) {
		for _, v := range views {
			v.Dark = dark()
			v.Refresh()
		}
	})

	list := widget.NewList(
		func() int { return len(demos) },
		func() fyne.CanvasObject { return widget.NewLabel("Population pyramid") },
		func(i widget.ListItemID, o fyne.CanvasObject) { o.(*widget.Label).SetText(demos[i].Name) },
	)
	list.OnSelected = func(i widget.ListItemID) { show(i) }
	list.Select(0)

	split := container.NewHSplit(list, stage)
	split.Offset = 0.2
	w.SetContent(split)
	w.Resize(fyne.NewSize(920, 540))
	w.ShowAndRun()
}
