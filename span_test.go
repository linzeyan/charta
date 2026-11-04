package charta

import "testing"

func TestSpanRunsFromLowToHighWithBothEndsRounded(t *testing.T) {
	s := build(t, SpanChart{Data: []Span{{"Mon", 12, 21}, {"Tue", 9, 18}}})
	m := dataRects(s)[0]
	r := radiusOf(m)
	if !(r.TL > 0 && r.BR > 0) || rectOf(m).H > barMax || m.Hit.Value != "12 – 21" {
		t.Fatalf("%+v %+v %q", r, rectOf(m), m.Hit.Value)
	}
	rejects(t, SpanChart{Data: []Span{{"x", 5, 1}}})
}
