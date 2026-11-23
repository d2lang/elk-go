// SPDX-License-Identifier: EPL-2.0

package force

import (
	"math"
	"sort"

	"github.com/d2lang/elk-go/internal/geometry"
)

// SeparateSelfLoopFootprints reserves the space occupied by loops after the
// iterative solver. Its equilibrium distances do not guarantee that node,
// loop and label rectangles are disjoint. Keep fixed nodes as obstacles and
// move each remaining node only when its footprint conflicts with a loop.
// Movement is limited to the axes the selected layout permits.
func (g *FGraph) SeparateSelfLoopFootprints(canMove func(*FNode) bool, moveX, moveY bool) {
	hasLoops := false
	for _, node := range g.Nodes {
		if node.LoopPlan != nil {
			hasLoops = true
			break
		}
	}
	if !hasLoops || len(g.Nodes) < 2 || !moveX && !moveY {
		return
	}

	placed := make([]*FNode, 0, len(g.Nodes))
	if canMove != nil {
		for _, node := range g.Nodes {
			if !canMove(node) {
				placed = append(placed, node)
			}
		}
	}
	var x, y []displacementInterval
	var moved map[*FNode]geometry.Point
	gap := math.Max(0, g.SpacingNodeNode)
	for _, node := range g.Nodes {
		if canMove != nil && !canMove(node) {
			continue
		}
		x, y = x[:0], y[:0]
		conflict := false
		minimum, maximum := node.OccupiedBounds()
		for _, other := range placed {
			otherMin, otherMax := other.OccupiedBounds()
			dx := displacementInterval{otherMin.X - maximum.X - gap, otherMax.X - minimum.X + gap}
			dy := displacementInterval{otherMin.Y - maximum.Y - gap, otherMax.Y - minimum.Y + gap}
			overlapX := dx.min < 0 && dx.max > 0
			overlapY := dy.min < 0 && dy.max > 0
			_, displaced := moved[other]
			conflict = conflict || overlapX && overlapY && (node.LoopPlan != nil || other.LoopPlan != nil || displaced)
			if moveX && overlapY {
				x = append(x, dx)
			}
			if moveY && overlapX {
				y = append(y, dy)
			}
		}
		if conflict {
			dx, dy := math.Inf(1), math.Inf(1)
			if moveX {
				dx = nearestClearDisplacement(x)
			}
			if moveY {
				dy = nearestClearDisplacement(y)
			}
			displacement := geometry.Point{Y: dy}
			if math.Abs(dx) <= math.Abs(dy) {
				displacement = geometry.Point{X: dx}
			}
			if moved == nil {
				moved = make(map[*FNode]geometry.Point)
			}
			moved[node] = displacement
			node.Position = node.Position.Add(displacement)
		}
		placed = append(placed, node)
	}
	if len(moved) == 0 {
		return
	}
	// Split can include the same edge twice in a cyclic component.
	adjusted := make(map[*FEdge]bool)
	for _, edge := range g.Edges {
		source, target := moved[edge.Source], moved[edge.Target]
		if adjusted[edge] || source == (geometry.Point{}) && target == (geometry.Point{}) {
			continue
		}
		adjusted[edge] = true
		for i, bend := range edge.Bendpoints {
			t := float64(i+1) / float64(len(edge.Bendpoints)+1)
			bend.Position = bend.Position.Add(source.Scale(1 - t)).Add(target.Scale(t))
		}
		for _, label := range edge.Labels {
			label.RefreshPosition()
		}
	}
}

// An open interval of axial translations that would intersect an obstacle.
type displacementInterval struct{ min, max float64 }

// Zero is inside at least one interval. The nearest free translation lies at
// either boundary of the merged interval containing zero.
func nearestClearDisplacement(intervals []displacementInterval) float64 {
	sort.Slice(intervals, func(i, j int) bool { return intervals[i].min < intervals[j].min })
	run := intervals[0]
	for _, next := range intervals[1:] {
		if next.min >= run.max {
			if run.max > 0 {
				break
			}
			run = next
			continue
		}
		run.max = math.Max(run.max, next.max)
	}
	if -run.min < run.max {
		return run.min
	}
	return run.max
}
