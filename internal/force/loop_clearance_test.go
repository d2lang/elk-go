// SPDX-License-Identifier: EPL-2.0

package force

import (
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/selfloops"
)

func TestLoopClearanceRespectsFixedNodesAndAxes(t *testing.T) {
	at := func(x float64) *FNode {
		return &FNode{Particle: Particle{Position: geometry.Point{X: x}, Size: geometry.Point{X: 10, Y: 10}}}
	}
	// A fixed loop occupies [-5, 45]. Shifting B just past the loop would
	// put it over another fixed node. Moving C must then account for B.
	owner, b, obstacle, c := at(0), at(40), at(54), at(66)
	owner.LoopPlan = &selfloops.Plan{Right: 40}
	component := &FGraph{Nodes: []*FNode{owner, b, obstacle, c}, SpacingNodeNode: 2}
	component.SeparateOccupiedFootprints(func(n *FNode) bool { return n != owner && n != obstacle }, true, false)
	if owner.Position.X != 0 || obstacle.Position.X != 54 || b.Position.X != 66 || c.Position.X != 78 {
		t.Fatalf("positions after clearance: %v %v %v %v", owner.Position, b.Position, obstacle.Position, c.Position)
	}
	for _, n := range component.Nodes {
		if n.Position.Y != 0 {
			t.Fatalf("unavailable Y axis used: %v", n.Position)
		}
	}
}

func TestNearestClearDisplacement(t *testing.T) {
	// Separate connected intervals, open at their ends. A gap far away
	// must not increase the nearest legal distance.
	for _, tc := range []struct {
		intervals []displacementInterval
		want      float64
	}{
		{[]displacementInterval{{-4, 5}, {5, 8}, {-9, -2}}, 5},
		{[]displacementInterval{{-2, 5}, {-4, -2}, {4, 10}}, -2},
		{[]displacementInterval{{-12, -3}, {-1, 15}, {12, 18}}, -1},
	} {
		if got := nearestClearDisplacement(tc.intervals); got != tc.want {
			t.Errorf("clear displacement for %v = %v, want %v", tc.intervals, got, tc.want)
		}
	}
}
