// Command fyne shows one interactive chart as a Fyne widget: hover for
// tooltips, click legend entries to hide a series. Fyne needs cgo.
//
// Usage: go run ./examples/go/fyne
package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/theme"

	"github.com/linzeyan/charta"
	chartafyne "github.com/linzeyan/charta/fyne"
)

func main() {
	a := app.New()
	w := a.NewWindow("charta in Fyne")
	view := chartafyne.NewChartView(charta.PieChart{
		Title: "Browser share (%)",
		Data:  []charta.Bar{{Label: "Chrome", Value: 65}, {Label: "Safari", Value: 19}, {Label: "Edge", Value: 5}, {Label: "Other", Value: 11}},
	})
	// Fyne has no chart palette of its own: follow its theme variant.
	view.Dark = a.Settings().ThemeVariant() == theme.VariantDark
	w.SetContent(view)
	w.Resize(fyne.NewSize(720, 440))
	w.ShowAndRun()
}
