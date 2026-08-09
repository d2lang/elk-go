// Copyright (c) 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// NorthSouthEdgeNeighbouringNodeCrossingsCounter accounts for north/south
// port dummies and their crossings with long-edge dummies.
type NorthSouthEdgeNeighbouringNodeCrossingsCounter struct {
	UpperLowerCrossings int
	LowerUpperCrossings int
	portPositions       map[layered.LPortID]int
}

func NewNorthSouthEdgeNeighbouringNodeCrossingsCounter(
	arena *layered.LGraphArena,
	layer []layered.LNodeID,
) *NorthSouthEdgeNeighbouringNodeCrossingsCounter {
	counter := &NorthSouthEdgeNeighbouringNodeCrossingsCounter{
		portPositions: make(map[layered.LPortID]int),
	}
	for _, nodeID := range layer {
		counter.setPortIDsOn(arena, nodeID, layered.SouthSide)
		counter.setPortIDsOn(arena, nodeID, layered.NorthSide)
	}
	return counter
}

func (counter *NorthSouthEdgeNeighbouringNodeCrossingsCounter) setPortIDsOn(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	side layered.PortSide,
) {
	portID := 0
	for _, port := range portsInNorthSouthEastWestOrder(arena, nodeID, side) {
		counter.portPositions[port] = portID
		portID++
	}
}

func (counter *NorthSouthEdgeNeighbouringNodeCrossingsCounter) CountCrossings(
	arena *layered.LGraphArena,
	upper, lower layered.LNodeID,
) {
	counter.UpperLowerCrossings = 0
	counter.LowerUpperCrossings = 0
	counter.processTwoNorthSouthNodes(arena, upper, lower)
	counter.processNorthSouthLongEdgeDummyCrossing(arena, upper, lower)
	counter.processNormalNodeWithNorthSouthPortsAndLongEdgeDummy(arena, upper, lower)
}

func (counter *NorthSouthEdgeNeighbouringNodeCrossingsCounter) processTwoNorthSouthNodes(
	arena *layered.LGraphArena,
	upper, lower layered.LNodeID,
) {
	if arena.Node(upper).Type != layered.NorthSouthPortNode ||
		arena.Node(lower).Type != layered.NorthSouthPortNode ||
		haveDifferentDummyOrigins(arena, upper, lower) {
		return
	}
	if isNorthOfNormalNode(arena, upper) {
		counter.countTwoNorthSouthDummies(arena, upper, lower)
	} else {
		counter.countTwoNorthSouthDummies(arena, lower, upper)
	}
}

func (counter *NorthSouthEdgeNeighbouringNodeCrossingsCounter) countTwoNorthSouthDummies(
	arena *layered.LGraphArena,
	further, closer layered.LNodeID,
) {
	if counter.originPortPosition(arena, further) > counter.originPortPosition(arena, closer) {
		counter.UpperLowerCrossings = degreeOfFirstPort(arena, closer, layered.EastSide)
		counter.LowerUpperCrossings = degreeOfFirstPort(arena, further, layered.WestSide)
	} else {
		counter.UpperLowerCrossings = degreeOfFirstPort(arena, closer, layered.WestSide)
		counter.LowerUpperCrossings = degreeOfFirstPort(arena, further, layered.EastSide)
	}
}

func (counter *NorthSouthEdgeNeighbouringNodeCrossingsCounter) processNorthSouthLongEdgeDummyCrossing(
	arena *layered.LGraphArena,
	upper, lower layered.LNodeID,
) {
	if arena.Node(upper).Type == layered.NorthSouthPortNode && arena.Node(lower).Type == layered.LongEdgeNode {
		if isNorthOfNormalNode(arena, upper) {
			counter.UpperLowerCrossings = 1
		} else {
			counter.LowerUpperCrossings = 1
		}
	} else if arena.Node(lower).Type == layered.NorthSouthPortNode && arena.Node(upper).Type == layered.LongEdgeNode {
		if isNorthOfNormalNode(arena, lower) {
			counter.LowerUpperCrossings = 1
		} else {
			counter.UpperLowerCrossings = 1
		}
	}
}

func (counter *NorthSouthEdgeNeighbouringNodeCrossingsCounter) processNormalNodeWithNorthSouthPortsAndLongEdgeDummy(
	arena *layered.LGraphArena,
	upper, lower layered.LNodeID,
) {
	if arena.Node(upper).Type == layered.NormalNode && arena.Node(lower).Type == layered.LongEdgeNode {
		counter.UpperLowerCrossings = numberOfNorthSouthEdges(arena, upper, layered.SouthSide)
		counter.LowerUpperCrossings = numberOfNorthSouthEdges(arena, upper, layered.NorthSide)
	}
	if arena.Node(lower).Type == layered.NormalNode && arena.Node(upper).Type == layered.LongEdgeNode {
		counter.UpperLowerCrossings = numberOfNorthSouthEdges(arena, lower, layered.NorthSide)
		counter.LowerUpperCrossings = numberOfNorthSouthEdges(arena, lower, layered.SouthSide)
	}
}

func (counter *NorthSouthEdgeNeighbouringNodeCrossingsCounter) originPortPosition(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
) int {
	origin := northSouthOriginPort(arena, nodeID)
	position, found := counter.portPositions[origin]
	if !found {
		panic("p3order: north/south origin port has no saved position")
	}
	return position
}

func degreeOfFirstPort(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	side layered.PortSide,
) int {
	ports := arena.NodePortsOnSide(nodeID, side)
	if len(ports) == 0 {
		return 0
	}
	return len(arena.PortConnectedEdges(ports[0]))
}

func numberOfNorthSouthEdges(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	side layered.PortSide,
) int {
	number := 0
	for _, portID := range arena.NodePortsOnSide(nodeID, side) {
		if basegraph.Has(&arena.Port(portID).Properties, layered.PortDummy) {
			number++
		}
	}
	return number
}

func haveDifferentDummyOrigins(
	arena *layered.LGraphArena,
	upper, lower layered.LNodeID,
) bool {
	upperOrigin, upperFound := basegraph.TryGet(&arena.Node(upper).Properties, layered.OriginalElement)
	lowerOrigin, lowerFound := basegraph.TryGet(&arena.Node(lower).Properties, layered.OriginalElement)
	return upperFound != lowerFound || upperOrigin != lowerOrigin
}

func northSouthOriginPort(arena *layered.LGraphArena, nodeID layered.LNodeID) layered.LPortID {
	portID := arena.Node(nodeID).Ports[0]
	origin, found := basegraph.TryGet(&arena.Port(portID).Properties, layered.OriginalElement)
	if !found || origin.Kind != layered.OriginLPort {
		panic("p3order: north/south dummy port has no LPort origin")
	}
	return origin.LPort
}

func isNorthOfNormalNode(arena *layered.LGraphArena, nodeID layered.LNodeID) bool {
	return arena.Port(northSouthOriginPort(arena, nodeID)).Side == layered.NorthSide
}
