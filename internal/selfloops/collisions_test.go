// SPDX-License-Identifier: EPL-2.0

package selfloops

import (
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
)

func TestLoopObstacleChecksOnlyOccupiedGeometry(t *testing.T) {
	plan := &Plan{loops: []loop{{
		path:   geometry.Chain{{X: 0, Y: 8}, {X: -30, Y: 8}, {X: -30, Y: 20}, {X: 0, Y: 20}},
		center: geometry.Point{X: -30, Y: 14}, labelWidth: 20, labelHeight: 4,
	}}}
	for _, tc := range []struct {
		name string
		box  geometry.Rectangle
		want bool
	}{
		{"space between loop segments", geometry.Rectangle{X: -10, Y: 11, Width: 6, Height: 5}, false},
		{"external label", geometry.Rectangle{X: -39, Y: 13, Width: 2, Height: 2}, true},
		{"segment", geometry.Rectangle{X: -10, Y: 5, Width: 5, Height: 5}, true},
		{"touches segment", geometry.Rectangle{X: -10, Y: 8, Width: 5, Height: 2}, false},
	} {
		if got := plan.IntersectsBox(nil, tc.box); got != tc.want {
			t.Errorf("%s: intersection = %t, want %t", tc.name, got, tc.want)
		}
	}
}
