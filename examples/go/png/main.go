// Command png renders one chart to a PNG file with the system fonts, for
// reports, e-mail and anywhere a picture goes. It needs no cgo and no GUI.
//
// Usage: go run ./examples/go/png [-o chart.png] [-dark]
package main

import (
	"flag"
	"log"
	"os"

	"github.com/linzeyan/charta"
	"github.com/linzeyan/charta/png"
)

func main() {
	out := flag.String("o", "chart.png", "output file")
	dark := flag.Bool("dark", false, "use the dark palette")
	flag.Parse()

	chart := charta.BarChart{
		Title: "Fruit sold this week",
		Data:  []charta.Bar{{Label: "Apples", Value: 42}, {Label: "Pears", Value: 18}, {Label: "Figs", Value: 33}},
	}
	// Scale 2 paints 1280×800 device pixels, sharp on a Retina display.
	img, err := png.Render(chart, png.Options{Scale: 2, Dark: *dark})
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, img, 0o644); err != nil {
		log.Fatal(err)
	}
}
