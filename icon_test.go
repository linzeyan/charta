package charta

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/linzeyan/charta/scene"
)

func mustPath(t *testing.T, d string) []scene.PathOp {
	t.Helper()
	ops, err := parsePath(d)
	if err != nil {
		t.Fatalf("%q: %v", d, err)
	}
	return ops
}

func TestRelativeAndAbsoluteCommandsAgree(t *testing.T) {
	abs := mustPath(t, "M2 2 L10 2 H12 V8 C12 9 11 10 10 10 Q6 12 2 10 Z")
	rel := mustPath(t, "m2 2 l8 0 h2 v6 c0 1-1 2-2 2 q-4 2-8 0z")
	if !reflect.DeepEqual(abs, rel) {
		t.Fatalf("absolute %v\nrelative %v", abs, rel)
	}
	// A quadratic becomes the cubic two thirds of the way to its control.
	q := abs[5]
	if q.Verb != scene.CubicTo || !near(q.X1, 10+2.0/3.0*(6-10)) || !near(q.Y2, 10+2.0/3.0*(12-10)) || q.X != 2 {
		t.Fatalf("quadratic as %+v", q)
	}
	// Numbers run together the way minified icon sets write them.
	got := mustPath(t, "M1.5.5L-2e1,3")
	if !reflect.DeepEqual(got, []scene.PathOp{scene.Move(1.5, 0.5), scene.Line(-20, 3)}) {
		t.Fatalf("%v", got)
	}
}

func TestImplicitLinesFollowAMoveAndZReturnsToTheStart(t *testing.T) {
	got := mustPath(t, "m1 1 2 0 0 2z l1 1")
	want := []scene.PathOp{scene.Move(1, 1), scene.Line(3, 1), scene.Line(3, 3), scene.Close(), scene.Line(2, 2)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%v", got)
	}
	if got := mustPath(t, "M0 0 1 1 L2 2 3 3"); len(got) != 4 || got[3] != scene.Line(3, 3) {
		t.Fatalf("absolute repeats: %v", got)
	}
}

func TestUnsupportedCommandsSayWhatToDo(t *testing.T) {
	_, err := parsePath("M0 0 A5 5 0 0 1 10 10")
	if err == nil || !strings.Contains(err.Error(), "'A'") || !strings.Contains(err.Error(), "arcs") {
		t.Fatalf("%v", err)
	}
	for _, bad := range []string{"L1 1", "M1", "", "  ", "5 5", "M0 0Z 1 1", "M0 0 L1"} {
		if _, err := parsePath(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

func TestAClipKeepsTheLeftPartOfAConcaveShape(t *testing.T) {
	// A U shape cut through both arms keeps both arms' left parts as one
	// polygon whose area is exactly the left part's.
	u := []scene.PathOp{
		scene.Move(0, 0), scene.Line(2, 0), scene.Line(2, 4), scene.Line(4, 4), scene.Line(4, 0),
		scene.Line(6, 0), scene.Line(6, 6), scene.Line(0, 6), scene.Close(),
	}
	// Left of x = 5: the left arm (2×6), the bridge (2×2) and a 1×6 strip of
	// the right arm.
	if a := area(clipLeft(u, 5)); math.Abs(a-(12+4+6)) > 1e-9 {
		t.Errorf("left of 5: %v", a)
	}
	// A triangle is the smallest polygon a clip keeps.
	tri := []scene.PathOp{scene.Move(0, 0), scene.Line(4, 2), scene.Line(0, 4), scene.Close()}
	if a := area(clipLeft(tri, 5)); math.Abs(a-8) > 1e-9 {
		t.Errorf("a triangle left of the cut: %v", a)
	}
	if c := clipLeft(u, -1); len(c) != 0 {
		t.Errorf("left of the shape: %v", c)
	}
	if a := area(clipLeft(u, 7)); math.Abs(a-area(u)) > 1e-9 {
		t.Errorf("right of the shape: %v, want %v", a, area(u))
	}
}

func TestIconsPlaceIntoTheirBox(t *testing.T) {
	// A built-in icon spans its 24 box minus the margin; a path icon its own
	// size. Both scale to the side asked for.
	sq, err := Icon{Kind: IconSquare}.place(10, 20, 48)
	if err != nil {
		t.Fatal(err)
	}
	if sq[0] != scene.Move(14, 24) || sq[2] != scene.Line(54, 64) {
		t.Fatalf("%v", sq)
	}
	p, err := Icon{Kind: IconPath, D: "M0 0H10V10H0Z", Size: 10}.place(0, 0, 20)
	if err != nil || p[2] != scene.Line(20, 20) {
		t.Fatalf("%v %v", p, err)
	}
	c, _ := Icon{Kind: IconCircle}.place(0, 0, 24)
	if c[0] != scene.Move(12, 2) || c[1].X != 22 || c[1].Y != 12 {
		t.Fatalf("circle %v", c)
	}
	if _, err := (Icon{Kind: IconPath, D: "M0 0H1Z"}).place(0, 0, 10); err == nil {
		t.Fatal("a path icon needs a size")
	}
	if _, err := (Icon{Kind: 7}).place(0, 0, 10); err == nil {
		t.Fatal("an unknown kind must fail")
	}
}

func TestIconsAndShapesReadTheSharedJSON(t *testing.T) {
	for in, want := range map[string]Icon{
		`"person"`: {}, `"circle"`: {Kind: IconCircle}, `"square"`: {Kind: IconSquare},
		`{"path":{"d":"M0 0Z","size":24}}`: {Kind: IconPath, D: "M0 0Z", Size: 24},
	} {
		var ic Icon
		if err := json.Unmarshal([]byte(in), &ic); err != nil || ic != want {
			t.Errorf("%s: %+v %v", in, ic, err)
		}
		out, err := json.Marshal(want)
		var back Icon
		if err != nil || json.Unmarshal(out, &back) != nil || back != want {
			t.Errorf("%+v round-trips as %s", want, out)
		}
	}
	for _, bad := range []string{`"path"`, `"Person"`, `{"path":{"d":"M0 0"}}`, `{"square":null}`, `null`} {
		var ic Icon
		if err := json.Unmarshal([]byte(bad), &ic); err == nil {
			t.Errorf("%s decoded as %+v", bad, ic)
		}
	}
	var s AreaShape
	if err := json.Unmarshal([]byte(`"square"`), &s); err != nil || s != AreaSquare {
		t.Fatalf("%v %v", s, err)
	}
	if err := json.Unmarshal([]byte(`"triangle"`), &s); err == nil {
		t.Fatal("an unknown shape must fail")
	}
	if out, err := json.Marshal(AreaCircle); err != nil || string(out) != `"circle"` {
		t.Fatalf("%s %v", out, err)
	}
}
