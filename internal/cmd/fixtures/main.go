// Command fixtures renders every fixture <name>.json in a directory to
// <out>/<name>.scene.json (for the Rust/Go conformance diff) and
// <out>/<name>.html (for the gallery), like the Rust examples/fixtures.rs.
//
// Usage: go run ./internal/cmd/fixtures <fixtures-dir> <out-dir>
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/linzeyan/charta"
	"github.com/linzeyan/charta/base"
	// The world fixtures name the "world" basemap.
	_ "github.com/linzeyan/charta/geo/world"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: fixtures <fixtures-dir> <out-dir>")
		os.Exit(2)
	}
	src, out := os.Args[1], os.Args[2]
	if err := os.MkdirAll(out, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	paths, err := filepath.Glob(filepath.Join(src, "*.json"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	sort.Strings(paths)
	failed := false
	for _, p := range paths {
		if err := render(p, out); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", p, err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func render(path, out string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var fx struct {
		Width  *float64        `json:"width"`
		Height *float64        `json:"height"`
		Chart  json.RawMessage `json:"chart"`
	}
	if err := json.Unmarshal(data, &fx); err != nil {
		return err
	}
	if fx.Width == nil || fx.Height == nil {
		return fmt.Errorf("missing width or height")
	}
	chart, err := charta.DecodeSpec(fx.Chart)
	if err != nil {
		return err
	}
	s, err := chart.Scene(*fx.Width, *fx.Height, base.InterMetrics{})
	if err != nil {
		return err
	}
	page, err := charta.RenderHTML(chart, charta.Options{Width: *fx.Width, Height: *fx.Height})
	if err != nil {
		return err
	}
	name := strings.TrimSuffix(filepath.Base(path), ".json")
	if err := os.WriteFile(filepath.Join(out, name+".scene.json"), []byte(s.Dump()), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, name+".html"), []byte(page), 0o644)
}
