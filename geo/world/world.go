// Package world is Natural Earth's 1:110m countries (public domain), the
// Rust crate's geo-world feature. Importing it registers the basemap
// "world" for the map charts' Basemap field:
//
//	import _ "github.com/linzeyan/charta/geo/world"
//
// It lives in its own package so programs that draw no world map do not
// carry its ~130 KB.
package world

import (
	_ "embed"
	"slices"
	"sync"

	"github.com/linzeyan/charta/geo"
)

//go:embed world-110m.txt
var src string

var parsed = sync.OnceValue(func() []geo.Feature {
	f, err := geo.ParseBasemap(src)
	if err != nil {
		panic("geo/world: " + err.Error())
	}
	return f
})

// Features returns the countries, id = ISO 3166-1 alpha-3 (Natural Earth's
// own code where there is none, e.g. "KOS"). The slice is the caller's; the
// geometry inside is shared, so do not modify it.
func Features() []geo.Feature { return slices.Clone(parsed()) }

func init() { geo.RegisterBasemap("world", Features) }
