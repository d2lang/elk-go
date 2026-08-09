// Copyright (c) 2019 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package routing

import (
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/layered/intermediate/loops"
	"github.com/d2lang/elk-go/internal/layered/p5edges"
)

// RouteSpline uses ELK's orthogonal self-loop skeleton and inserts the
// offset intermediate controls consumed by its cubic NUB conversion.
func RouteSpline(arena *layered.LGraphArena, holder *loops.Holder) {
	node := arena.Node(holder.LNode)
	edgeEdgeDistance := loops.IndividualOrInherited(arena, holder.LNode, layered.SpacingEdgeEdge)
	edgeLabelDistance := loops.IndividualOrInherited(arena, holder.LNode, layered.SpacingEdgeLabel)
	nodeLoopDistance := loops.IndividualOrInherited(arena, holder.LNode, layered.SpacingNodeSelfLoop)
	newMargins := node.Margin
	slotPositions := computeRoutingSlotPositions(
		arena, holder, edgeEdgeDistance, edgeLabelDistance, nodeLoopDistance)

	for _, loop := range holder.HyperLoops {
		for _, edge := range loop.Edges {
			direction := computeEdgeRoutingDirection(arena, edge)
			orthogonal := computeOrthogonalBendPoints(arena, edge, direction, slotPositions)
			controls := splineSelfLoopControls(arena, edge, direction, orthogonal, edgeLabelDistance)
			bends := p5edges.CubicNUBToBezier(controls)
			arena.Edge(edge.LEdge).BendPoints = bends
			for _, bend := range bends {
				updateMargins(node.Size, &newMargins, bend)
			}
		}
		if loop.Labels != nil {
			placeRoutedLabels(loop, slotPositions, edgeLabelDistance)
			updateMargins(node.Size, &newMargins, loop.Labels.Position)
			updateMargins(node.Size, &newMargins, loop.Labels.Position.Add(loop.Labels.Size))
		}
	}
	node.Margin = newMargins
}

func splineSelfLoopControls(
	arena *layered.LGraphArena,
	edge *loops.Edge,
	direction edgeRoutingDirection,
	orthogonal geometry.Chain,
	edgeLabelDistance float64,
) geometry.Chain {
	sourcePort := arena.Port(edge.Source.LPort)
	targetPort := arena.Port(edge.Target.LPort)
	controls := geometry.Chain{sourcePort.Position.Add(sourcePort.Anchor)}
	currentSide := sourcePort.Side
	first := orthogonal[0]
	for _, second := range orthogonal[1:] {
		controls = append(controls, first)
		middle := geometry.Point{X: (first.X + second.X) / 2, Y: (first.Y + second.Y) / 2}
		directionVector := splinePortDirection(currentSide)
		middle.X += directionVector.X * edgeLabelDistance
		middle.Y += directionVector.Y * edgeLabelDistance
		controls = append(controls, middle)
		first = second
		if direction == clockwise {
			currentSide = sideRight(currentSide)
		} else {
			currentSide = sideLeft(currentSide)
		}
	}
	controls = append(controls, orthogonal[len(orthogonal)-1])
	controls = append(controls, targetPort.Position.Add(targetPort.Anchor))
	return controls
}

func splinePortDirection(side layered.PortSide) geometry.Point {
	switch side {
	case layered.NorthSide:
		return geometry.Point{Y: -1}
	case layered.EastSide:
		return geometry.Point{X: 1}
	case layered.SouthSide:
		return geometry.Point{Y: 1}
	case layered.WestSide:
		return geometry.Point{X: -1}
	default:
		return geometry.Point{}
	}
}
