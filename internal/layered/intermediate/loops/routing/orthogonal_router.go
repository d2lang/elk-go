// Copyright (c) 2019 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package routing

import (
	"math"

	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/layered/intermediate/loops"
)

type edgeRoutingDirection uint8

const (
	clockwise edgeRoutingDirection = iota
	counterClockwise
)

// RouteOrthogonal ports OrthogonalSelfLoopRouter from ELK 0.8.1.
func RouteOrthogonal(arena *layered.LGraphArena, holder *loops.Holder) {
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
			bends := computeOrthogonalBendPoints(arena, edge, direction, slotPositions)
			arena.Edge(edge.LEdge).BendPoints = bends
			for _, bend := range bends {
				updateMargins(node.Size, &newMargins, bend)
			}
		}
		if loop.Labels != nil {
			placeOrthogonalRoutedLabels(arena, loop, slotPositions, edgeLabelDistance)
			updateMargins(node.Size, &newMargins, loop.Labels.Position)
			updateMargins(node.Size, &newMargins, loop.Labels.Position.Add(loop.Labels.Size))
		}
	}
	node.Margin = newMargins
}

func computeEdgeRoutingDirection(arena *layered.LGraphArena, edge *loops.Edge) edgeRoutingDirection {
	source := arena.Port(edge.Source.LPort)
	target := arena.Port(edge.Target.LPort)
	if source.Side == target.Side {
		if source.ID < target.ID {
			return clockwise
		}
		return counterClockwise
	}
	if sideRight(source.Side) == target.Side {
		return clockwise
	}
	if sideLeft(source.Side) == target.Side {
		return counterClockwise
	}
	if edge.HyperLoop.OccupiedSides[sideRight(source.Side)] {
		return clockwise
	}
	return counterClockwise
}

func placeOrthogonalRoutedLabels(
	arena *layered.LGraphArena,
	loop *loops.HyperLoop,
	positions [5][]float64,
	edgeLabelDistance float64,
) {
	if hasInlineLabels(arena, loop) {
		edgeLabelDistance = 0
	}
	placeRoutedLabels(loop, positions, edgeLabelDistance)
}

func placeRoutedLabels(loop *loops.HyperLoop, positions [5][]float64, edgeLabelDistance float64) {
	labels := loop.Labels
	position := positions[labels.Side][loop.RoutingSlot[labels.Side]]
	switch labels.Side {
	case layered.NorthSide:
		labels.Position.Y = position - edgeLabelDistance - labels.Size.Y
	case layered.SouthSide:
		labels.Position.Y = position + edgeLabelDistance
	case layered.WestSide:
		labels.Position.X = position - edgeLabelDistance - labels.Size.X
	case layered.EastSide:
		labels.Position.X = position + edgeLabelDistance
	}
}

func updateMargins(size geometry.Point, margin *geometry.Spacing, point geometry.Point) {
	margin.Left = math.Max(margin.Left, -point.X)
	margin.Right = math.Max(margin.Right, point.X-size.X)
	margin.Top = math.Max(margin.Top, -point.Y)
	margin.Bottom = math.Max(margin.Bottom, point.Y-size.Y)
}

func computeRoutingSlotPositions(
	arena *layered.LGraphArena,
	holder *loops.Holder,
	edgeEdgeDistance, edgeLabelDistance, nodeLoopDistance float64,
) [5][]float64 {
	var result [5][]float64
	for side := layered.UndefinedSide; side <= layered.WestSide; side++ {
		result[side] = make([]float64, holder.RoutingSlotCount[side])
	}
	initializeWithMaxLabelHeight(result[:], holder, layered.NorthSide)
	initializeWithMaxLabelHeight(result[:], holder, layered.SouthSide)
	for side := layered.NorthSide; side <= layered.WestSide; side++ {
		computeSlotPositions(arena, result[:], holder, side, edgeEdgeDistance, edgeLabelDistance, nodeLoopDistance)
	}
	return result
}

func initializeWithMaxLabelHeight(positions [][]float64, holder *loops.Holder, side layered.PortSide) {
	for _, loop := range holder.HyperLoops {
		if loop.Labels != nil && loop.Labels.Side == side {
			slot := loop.RoutingSlot[side]
			positions[side][slot] = math.Max(positions[side][slot], loop.Labels.Size.Y)
		}
	}
}

func computeSlotPositions(
	arena *layered.LGraphArena,
	positions [][]float64,
	holder *loops.Holder,
	side layered.PortSide,
	edgeEdgeDistance, edgeLabelDistance, nodeLoopDistance float64,
) {
	current := baselinePosition(arena, holder, side, nodeLoopDistance)
	factor := 1.0
	if side == layered.NorthSide || side == layered.WestSide {
		factor = -1
	}
	for slot, largestLabelSize := range positions[side] {
		if largestLabelSize > 0 {
			largestLabelSize += edgeLabelDistance
		}
		positions[side][slot] = current
		current += factor * (largestLabelSize + edgeEdgeDistance)
	}
}

func baselinePosition(
	arena *layered.LGraphArena,
	holder *loops.Holder,
	side layered.PortSide,
	distance float64,
) float64 {
	node := arena.Node(holder.LNode)
	switch side {
	case layered.NorthSide:
		return -node.Margin.Top - distance
	case layered.EastSide:
		return node.Size.X + node.Margin.Right + distance
	case layered.SouthSide:
		return node.Size.Y + node.Margin.Bottom + distance
	case layered.WestSide:
		return -node.Margin.Left - distance
	default:
		return -1
	}
}

func computeOrthogonalBendPoints(
	arena *layered.LGraphArena,
	edge *loops.Edge,
	direction edgeRoutingDirection,
	positions [5][]float64,
) geometry.Chain {
	result := make(geometry.Chain, 0, 6)
	result = append(result, outerBendPoint(arena, edge, edge.Source, positions))
	result = addCornerBendPoints(arena, edge, direction, positions, result)
	result = append(result, outerBendPoint(arena, edge, edge.Target, positions))
	return result
}

func outerBendPoint(
	arena *layered.LGraphArena,
	edge *loops.Edge,
	port *loops.Port,
	positions [5][]float64,
) geometry.Point {
	lport := arena.Port(port.LPort)
	result := baseVector(lport.Side, edge.HyperLoop.RoutingSlot[lport.Side], positions)
	anchor := lport.Position.Add(lport.Anchor)
	if lport.Side == layered.NorthSide || lport.Side == layered.SouthSide {
		result.X += anchor.X
	} else {
		result.Y += anchor.Y
	}
	return result
}

func addCornerBendPoints(
	arena *layered.LGraphArena,
	edge *loops.Edge,
	direction edgeRoutingDirection,
	positions [5][]float64,
	result geometry.Chain,
) geometry.Chain {
	current := arena.Port(edge.Source.LPort).Side
	target := arena.Port(edge.Target.LPort).Side
	inline := edge.IsInline(arena) && edge.HyperLoop.Labels != nil
	labelSide := edge.LabelSide()
	labelSize := geometry.Point{}
	if inline {
		labelSize = edge.HyperLoop.Labels.Size
	}
	for current != target {
		next := sideLeft(current)
		if direction == clockwise {
			next = sideRight(current)
		}
		currentPart := baseVector(current, edge.HyperLoop.RoutingSlot[current], positions)
		nextPart := baseVector(next, edge.HyperLoop.RoutingSlot[next], positions)
		if inline {
			if current == labelSide {
				adjustVectorForLabelSide(&currentPart, labelSide, labelSize)
			} else if next == labelSide {
				adjustVectorForLabelSide(&nextPart, labelSide, labelSize)
			}
		}
		result = append(result, currentPart.Add(nextPart))
		current = next
	}
	return result
}

func adjustVectorForLabelSide(
	point *geometry.Point,
	side layered.PortSide,
	size geometry.Point,
) {
	switch side {
	case layered.NorthSide:
		point.Y -= size.Y / 2
	case layered.SouthSide:
		point.Y += size.Y / 2
	case layered.WestSide:
		point.X -= size.X / 2
	case layered.EastSide:
		point.X += size.X / 2
	}
}

func baseVector(side layered.PortSide, slot int, positions [5][]float64) geometry.Point {
	position := positions[side][slot]
	if side == layered.NorthSide || side == layered.SouthSide {
		return geometry.Point{Y: position}
	}
	return geometry.Point{X: position}
}
