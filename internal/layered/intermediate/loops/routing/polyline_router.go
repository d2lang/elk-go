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
	"math"

	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/layered/intermediate/loops"
)

const (
	polylineCornerDistance  = 10.0
	polylineCornerTolerance = 0.01
)

// RoutePolyline ports ELK 0.8.1's PolylineSelfLoopRouter. It starts with the
// orthogonal self-loop skeleton and replaces every corner with two points.
func RoutePolyline(arena *layered.LGraphArena, holder *loops.Holder) {
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
			bends := polylineSelfLoopBendPoints(arena, edge, orthogonal)
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

func polylineSelfLoopBendPoints(
	arena *layered.LGraphArena,
	edge *loops.Edge,
	orthogonal geometry.Chain,
) geometry.Chain {
	source := arena.Port(edge.Source.LPort)
	target := arena.Port(edge.Target.LPort)
	withEndpoints := make(geometry.Chain, 0, len(orthogonal)+2)
	withEndpoints = append(withEndpoints, source.Position.Add(source.Anchor))
	withEndpoints = append(withEndpoints, orthogonal...)
	withEndpoints = append(withEndpoints, target.Position.Add(target.Anchor))
	return CutPolylineCorners(withEndpoints, polylineCornerDistance)
}

// CutPolylineCorners ports PolylineSelfLoopRouter.cutCorners. The first and
// last points are endpoints and are intentionally excluded from the result.
func CutPolylineCorners(points geometry.Chain, distance float64) geometry.Chain {
	if len(points) <= 2 {
		return nil
	}
	result := make(geometry.Chain, 0, 2*(len(points)-2))
	for index := 1; index+1 < len(points); index++ {
		previous, corner, next := points[index-1], points[index], points[index+1]
		offset1 := nearZeroToZero(previous.Sub(corner))
		offset2 := nearZeroToZero(next.Sub(corner))

		effectiveDistance := distance
		effectiveDistance = math.Min(effectiveDistance, math.Abs(offset1.X+offset1.Y)/2)
		effectiveDistance = math.Min(effectiveDistance, math.Abs(offset2.X+offset2.Y)/2)
		offset1 = geometry.Point{
			X: javaSignum(offset1.X) * effectiveDistance,
			Y: javaSignum(offset1.Y) * effectiveDistance,
		}
		offset2 = geometry.Point{
			X: javaSignum(offset2.X) * effectiveDistance,
			Y: javaSignum(offset2.Y) * effectiveDistance,
		}
		result = append(result, corner.Add(offset1), corner.Add(offset2))
	}
	return result
}

func nearZeroToZero(point geometry.Point) geometry.Point {
	if point.X >= -polylineCornerTolerance && point.X <= polylineCornerTolerance {
		point.X = 0
	}
	if point.Y >= -polylineCornerTolerance && point.Y <= polylineCornerTolerance {
		point.Y = 0
	}
	return point
}

func javaSignum(value float64) float64 {
	if value > 0 {
		return 1
	}
	if value < 0 {
		return -1
	}
	return value
}
