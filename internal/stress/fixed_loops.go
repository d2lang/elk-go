// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package stress

import (
	"fmt"

	"github.com/d2lang/elk-go/internal/force"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/selfloops"
)

// Pinned nodes cannot yield space to another pinned node. If the preferred
// loop arrangement enters one, try the remaining sides before solving.
func fitPinnedLoops(model *graph.Model, component *force.FGraph) error {
	var fixed []*force.FNode
	for _, node := range component.Nodes {
		if graph.Get(&model.Node(node.Origin).Properties, Fixed) {
			fixed = append(fixed, node)
		}
	}
	for _, owner := range fixed {
		plan := owner.LoopPlan
		if plan == nil {
			continue
		}
		topLeft := owner.Position.Sub(owner.Size.Scale(.5))
		fits := func() bool {
			for _, other := range fixed {
				if owner == other {
					continue
				}
				p := other.Position.Sub(other.Size.Scale(.5)).Sub(topLeft)
				box := geometry.Rectangle{X: p.X, Y: p.Y, Width: other.Size.X, Height: other.Size.Y}
				if plan.IntersectsBox(model, box) {
					return false
				}
			}
			return true
		}
		if fits() {
			continue
		}
		clear := false
		for _, side := range []selfloops.Side{selfloops.West, selfloops.East, selfloops.North, selfloops.South} {
			plan.Arrange([]selfloops.Side{side})
			if fits() {
				clear = true
				break
			}
		}
		if !clear {
			return fmt.Errorf("cannot route self-loops on fixed node %q clear of other fixed nodes", model.Node(owner.Origin).Identifier)
		}
	}
	return nil
}
