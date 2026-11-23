// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package force

import (
	"cmp"
	"math"
	"slices"

	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/selfloops"
)

// OccupiedBounds includes the node, its external self-loops, and their labels.
// Size remains the visible body used to clip ordinary edge endpoints.
func (node *FNode) OccupiedBounds() (topLeft, bottomRight geometry.Point) {
	half := node.Size.Scale(.5)
	topLeft = node.Position.Sub(half)
	bottomRight = node.Position.Add(half)
	if plan := node.LoopPlan; plan != nil {
		topLeft.X -= plan.Left
		topLeft.Y -= plan.Top
		bottomRight.X += plan.Right
		bottomRight.Y += plan.Bottom
	}
	return topLeft, bottomRight
}

// Keep upstream's radius for the body, extending it by the occupied margin
// facing the other particle. A loop on the left need not push a node on the right.
func (p *Particle) loopExtent(dx, dy, distance float64) float64 {
	if p.node == nil || p.node.LoopPlan == nil {
		return 0
	}
	plan := p.node.LoopPlan
	horizontal := math.Max(dx*plan.Right, -dx*plan.Left)
	vertical := math.Max(dy*plan.Bottom, -dy*plan.Top)
	return (horizontal + vertical) / distance
}

// ArrangeSelfLoops chooses open sides using current node positions. Each
// neighbor contributes a larger penalty to sides it faces and to closer nodes.
// Use all clear sides, or the least crowded side when the node is surrounded.
func (g *FGraph) ArrangeSelfLoops() bool {
	hasLoops := false
	for _, node := range g.Nodes {
		if node.LoopPlan == nil {
			continue
		}
		hasLoops = true
		type candidate struct {
			side    selfloops.Side
			penalty float64
		}
		sides := [...]candidate{{side: selfloops.West}, {side: selfloops.East}, {side: selfloops.North}, {side: selfloops.South}}
		for _, other := range g.Nodes {
			delta := other.Position.Sub(node.Position)
			distance := math.Hypot(delta.X, delta.Y)
			if distance == 0 {
				continue
			}
			// Weight the squared directional cosine by inverse distance.
			components := [...]float64{-delta.X, delta.X, -delta.Y, delta.Y}
			for i, projection := range components {
				if projection > 0 {
					cosine := projection / distance
					sides[i].penalty += cosine * cosine / distance
				}
			}
		}
		slices.SortStableFunc(sides[:], func(a, b candidate) int { return cmp.Compare(a.penalty, b.penalty) })
		preferred := []selfloops.Side{sides[0].side}
		for _, s := range sides[1:] {
			if s.penalty == 0 {
				preferred = append(preferred, s.side)
			}
		}
		node.LoopPlan.Arrange(preferred)
	}
	return hasLoops
}
