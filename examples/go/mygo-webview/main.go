// Command mygo-webview shows a chart's HTML in a MyGo window's system
// webview (WKWebView, WebView2, WebKitGTK): the route for an app whose
// interface is a web page anyway. The HTML runtime does the tooltips and
// legend toggles; for a window MyGo draws itself, see examples/go/mygo.
//
// Usage: go run ./examples/go/mygo-webview
package main

import (
	"log"

	"github.com/egoist/mygo"

	"github.com/linzeyan/charta"
)

func main() {
	div, err := charta.RenderHTML(charta.BarChart{
		Title: "Fruit sold this week",
		Data:  []charta.Bar{{Label: "Apples", Value: 42}, {Label: "Pears", Value: 18}, {Label: "Figs", Value: 33}},
	}, charta.Options{Width: 680, Height: 400})
	if err != nil {
		log.Fatal(err)
	}
	page := "<!doctype html><meta charset=\"utf-8\"><body style=\"margin:20px\">" + div
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{Title: "charta in a MyGo webview", Width: 720, Height: 440})
		win.Page().LoadHTML(page, "")
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
