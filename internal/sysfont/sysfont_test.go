package sysfont

import "testing"

func TestStyleNamesReadAsCSSWeights(t *testing.T) {
	for style, want := range map[string]int{
		"regular": 400, "book": 400, "semibold": 600, "demi bold": 600,
		"bold": 700, "extrabold": 800, "medium": 500, "light": 300,
		"extralight": 200, "black": 900, "w3": 300, "w6": 600, "w0": 100,
	} {
		if got := weight(style); got != want {
			t.Errorf("%q: %d, want %d", style, got, want)
		}
	}
}

func TestPickFollowsCSSFontMatching(t *testing.T) {
	// A family with no 600: semibold takes the next heavier face, as
	// fontdb does for the Rust crate, not the nearer Medium.
	ix := &index{faces: []face{
		{family: "Helvetica Neue", weight: 400, path: "r"},
		{family: "Helvetica Neue", weight: 500, path: "m"},
		{family: "Helvetica Neue", weight: 700, path: "b"},
		{family: "Helvetica Neue", weight: 300, path: "l"},
	}}
	for want, path := range map[int]string{600: "b", 400: "r", 500: "m", 800: "b", 200: "l"} {
		if got, ok := ix.pick("helvetica neue", want); !ok || got.path != path {
			t.Errorf("weight %d picked %+v, want %s", want, got, path)
		}
	}
	if _, ok := ix.pick("Inter", 400); ok {
		t.Error("picked a family that is not installed")
	}
}

func TestTheSystemHasASansSerif(t *testing.T) {
	// Fails loud on a machine with no fonts at all: PNG output then needs
	// faces passed in, and CI should install some rather than skip this.
	if Sans(false) == nil || Sans(true) == nil {
		t.Fatal("no sans-serif face found in the system font directories")
	}
}
