package geo_test

import (
	"strings"
	"testing"

	"github.com/linzeyan/charta"
	"github.com/linzeyan/charta/base"
)

func TestTheWorldBasemapWithoutItsPackageSaysWhatToImport(t *testing.T) {
	// This test binary does not import geo/world, like a program that never
	// asked for the basemap: the error must name the import, not just say
	// "unknown basemap".
	_, err := charta.ChoroplethChart{Basemap: "world"}.Scene(640, 400, base.InterMetrics{})
	if err == nil || !strings.Contains(err.Error(), "github.com/linzeyan/charta/geo/world") {
		t.Fatalf("%v", err)
	}
}
