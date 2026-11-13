// Command html writes a page with one interactive chart to stdout: tooltips,
// hover highlight and legend toggles run in the page, without a server.
//
// Usage: go run ./examples/go/html > chart.html
package main

import (
	"fmt"
	"log"

	"github.com/linzeyan/charta"
)

func main() {
	chart := charta.BarChart{
		Title: "Fruit sold this week",
		Data:  []charta.Bar{{Label: "Apples", Value: 42}, {Label: "Pears", Value: 18}, {Label: "Figs", Value: 33}},
	}
	// The zero Options is 640×400 and follows the viewer's light or dark
	// scheme. The result is a <div> to embed: it carries its own styles and
	// script.
	div, err := charta.RenderHTML(chart, charta.Options{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("<!doctype html>\n<meta charset=\"utf-8\">\n<title>%s</title>\n%s\n", chart.Title, div)
}
