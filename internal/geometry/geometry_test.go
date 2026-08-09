// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package geometry

import (
	"math"
	"testing"
)

func TestKVectorOperations(t *testing.T) {
	p := Point{X: 3, Y: 4}
	if p.Length() != 5 {
		t.Fatalf("length = %v", p.Length())
	}
	if got := p.Normalize().Length(); math.Abs(got-1) > 1e-12 {
		t.Fatalf("normalized length = %v", got)
	}
	if got := Between(Point{X: 1, Y: 2}, Point{X: 4, Y: 6}); got != p {
		t.Fatalf("between = %#v", got)
	}
}

func TestSpacingParseAndString(t *testing.T) {
	spacing, err := ParseSpacing("[top=1,left=2,bottom=3,right=4]")
	if err != nil {
		t.Fatal(err)
	}
	if spacing != (Spacing{Top: 1, Left: 2, Bottom: 3, Right: 4}) {
		t.Fatalf("spacing = %#v", spacing)
	}
	if spacing.String() != "[top=1,left=2,bottom=3,right=4]" {
		t.Fatalf("string = %s", spacing.String())
	}
}

func TestKVectorChainPointAt(t *testing.T) {
	chain := Chain{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}}
	if got := chain.PointAt(15); got != (Point{X: 10, Y: 5}) {
		t.Fatalf("point = %#v", got)
	}
	if got := chain.PointAt(-5); got != (Point{X: 10, Y: 5}) {
		t.Fatalf("reverse point = %#v", got)
	}
}
