// Package assets embeds the files shared verbatim with the Rust crate, which
// reads the same files with include_str!. go:embed cannot reach parent
// directories, so this package lives next to them.
package assets

import _ "embed"

// Palette is the light/dark palette: "token light dark" per line.
//
//go:embed palette.txt
var Palette string

// InterWidths holds Inter advance widths for text measurement.
//
//go:embed inter-widths.txt
var InterWidths string

// RuntimeJS powers tooltips and hover in HTML output.
//
//go:embed runtime.js
var RuntimeJS string

// RuntimeCSS styles the HTML runtime's tooltip, hover and focus states.
//
//go:embed runtime.css
var RuntimeCSS string
