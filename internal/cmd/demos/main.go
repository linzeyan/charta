// Command demos writes every gallery demo's scene and HTML to
// <out>/<NN>-<name>.{scene.json,html} for `make test-conformance` to diff
// against the Rust crate's `cargo run -p charta-demos --example dump`, and
// for a chart with continuous axes also the middle half of it, zoomed, to
// <NN>-<name>.zoomed.scene.json. The demos live in the repo (unlike the
// fixtures in docs/), so CI can hold the two ports to identical output.
//
// Usage: go run ./internal/cmd/demos <out-dir>
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/linzeyan/charta"
	"github.com/linzeyan/charta/base"
	"github.com/linzeyan/charta/internal/demo"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: demos <out-dir>")
		os.Exit(2)
	}
	out := os.Args[1]
	if err := os.MkdirAll(out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	failed := false
	for i, d := range demo.All() {
		if err := write(filepath.Join(out, fmt.Sprintf("%02d-%s", i, slug(d.Name))), d.Chart); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", d.Name, err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func write(stem string, c charta.Chart) error {
	opts := charta.Options{}
	s, err := c.Scene(640, 400, base.InterMetrics{})
	if err != nil {
		return err
	}
	page, err := charta.RenderHTML(c, opts)
	if err != nil {
		return err
	}
	if err := os.WriteFile(stem+".scene.json", []byte(s.Dump()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(stem+".html", []byte(page), 0o644); err != nil {
		return err
	}
	v, ok := c.(charta.Viewable)
	if !ok {
		return nil
	}
	_, plot, err := v.PlotScene(640, 400, base.InterMetrics{})
	if err != nil {
		return err
	}
	mid := func(r [2]float64) [2]float64 { return [2]float64{r[0] + (r[1]-r[0])/4, r[1] - (r[1]-r[0])/4} }
	var y *[2]float64
	if plot.Y != nil {
		m := mid(*plot.Y)
		y = &m
	}
	zoomed, err := v.View(mid(plot.X), y).Scene(640, 400, base.InterMetrics{})
	if err != nil {
		return err
	}
	return os.WriteFile(stem+".zoomed.scene.json", []byte(zoomed.Dump()), 0o644)
}

// slug lowercases ASCII letters and digits and turns anything else into a
// dash: the same file names as the Rust runner's.
func slug(name string) string {
	b := []byte(name)
	for i, c := range b {
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case c >= 'A' && c <= 'Z':
			b[i] = c + 'a' - 'A'
		default:
			b[i] = '-'
		}
	}
	return string(b)
}
