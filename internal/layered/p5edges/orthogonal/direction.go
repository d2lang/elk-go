// Copyright (c) 2010, 2020 Kiel University and others.
// Copyright (c) 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package orthogonal

import (
	"math"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// RoutingDirection is distinct from a graph direction because routing around
// northern and southern external ports changes direction within one graph.
type RoutingDirection uint8

const (
	WestToEast RoutingDirection = iota
	NorthToSouth
	SouthToNorth
)

type routingDirectionStrategy struct {
	direction             RoutingDirection
	createdJunctionPoints map[geometry.Point]struct{}
}

func newRoutingDirectionStrategy(direction RoutingDirection) *routingDirectionStrategy {
	return &routingDirectionStrategy{
		direction:             direction,
		createdJunctionPoints: make(map[geometry.Point]struct{}),
	}
}

func (strategy *routingDirectionStrategy) sourcePortSide() layered.PortSide {
	switch strategy.direction {
	case WestToEast:
		return layered.EastSide
	case NorthToSouth:
		return layered.SouthSide
	case SouthToNorth:
		return layered.NorthSide
	default:
		panic("orthogonal: invalid routing direction")
	}
}

func (strategy *routingDirectionStrategy) targetPortSide() layered.PortSide {
	switch strategy.direction {
	case WestToEast:
		return layered.WestSide
	case NorthToSouth:
		return layered.NorthSide
	case SouthToNorth:
		return layered.SouthSide
	default:
		panic("orthogonal: invalid routing direction")
	}
}

func (strategy *routingDirectionStrategy) portPosition(arena *layered.LGraphArena, portID layered.LPortID) float64 {
	port := arena.Port(portID)
	node := arena.Node(*port.Node)
	if strategy.direction == WestToEast {
		return node.Position.Y + port.Position.Y + port.Anchor.Y
	}
	return node.Position.X + port.Position.X + port.Anchor.X
}

func absoluteAnchor(arena *layered.LGraphArena, portID layered.LPortID) geometry.Point {
	port := arena.Port(portID)
	return arena.Node(*port.Node).Position.Add(port.Position).Add(port.Anchor)
}

func (strategy *routingDirectionStrategy) calculateBendPoints(
	arena *layered.LGraphArena,
	segment *HyperEdgeSegment,
	startPosition, edgeSpacing float64,
) {
	if segment.IsDummy() {
		return
	}
	switch strategy.direction {
	case WestToEast:
		strategy.calculateHorizontalBends(arena, segment, startPosition, edgeSpacing)
	case NorthToSouth:
		strategy.calculateVerticalBends(arena, segment, startPosition, edgeSpacing, 1)
	case SouthToNorth:
		strategy.calculateVerticalBends(arena, segment, startPosition, edgeSpacing, -1)
	default:
		panic("orthogonal: invalid routing direction")
	}
}

func (strategy *routingDirectionStrategy) calculateHorizontalBends(
	arena *layered.LGraphArena,
	segment *HyperEdgeSegment,
	startPosition, edgeSpacing float64,
) {
	segmentX := startPosition + float64(segment.RoutingSlot)*edgeSpacing
	for _, portID := range segment.Ports {
		sourceY := absoluteAnchor(arena, portID).Y
		for _, edgeID := range arena.Port(portID).OutgoingEdges {
			if arena.EdgeIsSelfLoop(edgeID) {
				continue
			}
			targetY := absoluteAnchor(arena, *arena.Edge(edgeID).Target).Y
			if math.Abs(sourceY-targetY) <= Tolerance {
				continue
			}
			currentX, currentSegment := segmentX, segment
			bend := geometry.Point{X: currentX, Y: sourceY}
			arena.Edge(edgeID).BendPoints = append(arena.Edge(edgeID).BendPoints, bend)
			strategy.addJunctionPointIfNecessary(arena, edgeID, currentSegment, bend, true)
			if partner := segment.SplitPartner; partner != nil {
				splitY := partner.IncomingCoordinates[0]
				bend = geometry.Point{X: currentX, Y: splitY}
				arena.Edge(edgeID).BendPoints = append(arena.Edge(edgeID).BendPoints, bend)
				strategy.addJunctionPointIfNecessary(arena, edgeID, currentSegment, bend, true)
				currentX = startPosition + float64(partner.RoutingSlot)*edgeSpacing
				currentSegment = partner
				bend = geometry.Point{X: currentX, Y: splitY}
				arena.Edge(edgeID).BendPoints = append(arena.Edge(edgeID).BendPoints, bend)
				strategy.addJunctionPointIfNecessary(arena, edgeID, currentSegment, bend, true)
			}
			bend = geometry.Point{X: currentX, Y: targetY}
			arena.Edge(edgeID).BendPoints = append(arena.Edge(edgeID).BendPoints, bend)
			strategy.addJunctionPointIfNecessary(arena, edgeID, currentSegment, bend, true)
		}
	}
}

func (strategy *routingDirectionStrategy) calculateVerticalBends(
	arena *layered.LGraphArena,
	segment *HyperEdgeSegment,
	startPosition, edgeSpacing, sign float64,
) {
	segmentY := startPosition + sign*float64(segment.RoutingSlot)*edgeSpacing
	for _, portID := range segment.Ports {
		sourceX := absoluteAnchor(arena, portID).X
		for _, edgeID := range arena.Port(portID).OutgoingEdges {
			if arena.EdgeIsSelfLoop(edgeID) {
				continue
			}
			targetX := absoluteAnchor(arena, *arena.Edge(edgeID).Target).X
			if math.Abs(sourceX-targetX) <= Tolerance {
				continue
			}
			currentY, currentSegment := segmentY, segment
			bend := geometry.Point{X: sourceX, Y: currentY}
			arena.Edge(edgeID).BendPoints = append(arena.Edge(edgeID).BendPoints, bend)
			strategy.addJunctionPointIfNecessary(arena, edgeID, currentSegment, bend, false)
			if partner := segment.SplitPartner; partner != nil {
				splitX := partner.IncomingCoordinates[0]
				bend = geometry.Point{X: splitX, Y: currentY}
				arena.Edge(edgeID).BendPoints = append(arena.Edge(edgeID).BendPoints, bend)
				strategy.addJunctionPointIfNecessary(arena, edgeID, currentSegment, bend, false)
				currentY = startPosition + sign*float64(partner.RoutingSlot)*edgeSpacing
				currentSegment = partner
				bend = geometry.Point{X: splitX, Y: currentY}
				arena.Edge(edgeID).BendPoints = append(arena.Edge(edgeID).BendPoints, bend)
				strategy.addJunctionPointIfNecessary(arena, edgeID, currentSegment, bend, false)
			}
			bend = geometry.Point{X: targetX, Y: currentY}
			arena.Edge(edgeID).BendPoints = append(arena.Edge(edgeID).BendPoints, bend)
			strategy.addJunctionPointIfNecessary(arena, edgeID, currentSegment, bend, false)
		}
	}
}

func (strategy *routingDirectionStrategy) addJunctionPointIfNecessary(
	arena *layered.LGraphArena,
	edgeID layered.LEdgeID,
	segment *HyperEdgeSegment,
	position geometry.Point,
	vertical bool,
) {
	coordinate := position.X
	if vertical {
		coordinate = position.Y
	}
	if _, found := strategy.createdJunctionPoints[position]; found {
		return
	}
	inside := coordinate > segment.Start && coordinate < segment.End
	atBoundary := false
	if len(segment.IncomingCoordinates) > 0 && len(segment.OutgoingCoordinates) > 0 {
		atBoundary = math.Abs(coordinate-segment.IncomingCoordinates[0]) < Tolerance &&
			math.Abs(coordinate-segment.OutgoingCoordinates[0]) < Tolerance
		lastIncoming := segment.IncomingCoordinates[len(segment.IncomingCoordinates)-1]
		lastOutgoing := segment.OutgoingCoordinates[len(segment.OutgoingCoordinates)-1]
		atBoundary = atBoundary || math.Abs(coordinate-lastIncoming) < Tolerance &&
			math.Abs(coordinate-lastOutgoing) < Tolerance
	}
	if inside || atBoundary {
		points := basegraph.Get(&arena.Edge(edgeID).Properties, layered.JunctionPoints)
		points = append(points, position)
		basegraph.Set(&arena.Edge(edgeID).Properties, layered.JunctionPoints, points)
		strategy.createdJunctionPoints[position] = struct{}{}
	}
}

func (strategy *routingDirectionStrategy) clearCreatedJunctionPoints() {
	clear(strategy.createdJunctionPoints)
}
