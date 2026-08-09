// Copyright (c) 2016, 2018 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"sort"

	"github.com/d2lang/elk-go/internal/layered"
)

const (
	inLayerLayoutUnitPropertyID = "inLayerLayoutUnit"
	portDummyPropertyID         = "portDummy"
	originPropertyID            = "origin"
)

// CrossingsCounter is the exact ELK 0.8.1 reduction of between-layer,
// in-layer, and north/south crossings to inversion counting. Port IDs are
// scratch indices into PortPositions; AllCrossingsCounter assigns them in
// Java traversal order.
type CrossingsCounter struct {
	PortPositions     []int
	indexTree         *binaryIndexedTree
	ends              []int
	nodeCardinalities []int
}

func NewCrossingsCounter(portPositions []int) *CrossingsCounter {
	return &CrossingsCounter{PortPositions: portPositions}
}

func (counter *CrossingsCounter) CountCrossingsBetweenLayers(
	arena *layered.LGraphArena,
	leftLayerNodes, rightLayerNodes []layered.LNodeID,
) int {
	ports := counter.initPortPositionsCounterClockwise(arena, leftLayerNodes, rightLayerNodes)
	counter.indexTree = newBinaryIndexedTree(len(ports))
	return counter.countCrossingsOnPorts(arena, ports)
}

func (counter *CrossingsCounter) CountInLayerCrossingsOnSide(
	arena *layered.LGraphArena,
	nodes []layered.LNodeID,
	side layered.PortSide,
) int {
	ports := counter.InitPortPositionsForInLayerCrossings(arena, nodes, side)
	return counter.countInLayerCrossingsOnPorts(arena, ports)
}

func (counter *CrossingsCounter) CountNorthSouthPortCrossingsInLayer(
	arena *layered.LGraphArena,
	nodes []layered.LNodeID,
) int {
	ports := counter.initPositionsForNorthSouthCounting(arena, nodes)
	counter.indexTree = newBinaryIndexedTree(len(ports))
	return counter.countNorthSouthCrossingsOnPorts(arena, ports)
}

func (counter *CrossingsCounter) CountCrossingsBetweenPortsInBothOrders(
	arena *layered.LGraphArena,
	upperPort, lowerPort layered.LPortID,
) (int, int) {
	ports := counter.connectedPortsSortedByPosition(arena, upperPort, lowerPort)
	upperLower := counter.countCrossingsOnPorts(arena, ports)
	counter.indexTree.clear()
	counter.SwitchPorts(arena, upperPort, lowerPort)
	sort.SliceStable(ports, func(i, j int) bool {
		return counter.positionOf(arena, ports[i]) < counter.positionOf(arena, ports[j])
	})
	lowerUpper := counter.countCrossingsOnPorts(arena, ports)
	counter.indexTree.clear()
	counter.SwitchPorts(arena, lowerPort, upperPort)
	return upperLower, lowerUpper
}

func (counter *CrossingsCounter) CountInLayerCrossingsBetweenNodesInBothOrders(
	arena *layered.LGraphArena,
	upperNode, lowerNode layered.LNodeID,
	side layered.PortSide,
) (int, int) {
	ports := counter.connectedInLayerPortsSortedByPosition(arena, upperNode, lowerNode, side)
	upperLower := counter.countInLayerCrossingsOnPorts(arena, ports)
	counter.SwitchNodes(arena, upperNode, lowerNode, side)
	counter.indexTree.clear()
	sort.SliceStable(ports, func(i, j int) bool {
		return counter.positionOf(arena, ports[i]) < counter.positionOf(arena, ports[j])
	})
	lowerUpper := counter.countInLayerCrossingsOnPorts(arena, ports)
	counter.SwitchNodes(arena, lowerNode, upperNode, side)
	counter.indexTree.clear()
	return upperLower, lowerUpper
}

func (counter *CrossingsCounter) InitForCountingBetween(
	arena *layered.LGraphArena,
	leftLayerNodes, rightLayerNodes []layered.LNodeID,
) {
	ports := counter.initPortPositionsCounterClockwise(arena, leftLayerNodes, rightLayerNodes)
	counter.indexTree = newBinaryIndexedTree(len(ports))
}

func (counter *CrossingsCounter) InitPortPositionsForInLayerCrossings(
	arena *layered.LGraphArena,
	nodes []layered.LNodeID,
	side layered.PortSide,
) []layered.LPortID {
	ports := make([]layered.LPortID, 0)
	counter.initPositions(arena, nodes, &ports, side, true, true)
	counter.indexTree = newBinaryIndexedTree(len(ports))
	return ports
}

func (counter *CrossingsCounter) SwitchPorts(
	arena *layered.LGraphArena,
	topPort, bottomPort layered.LPortID,
) {
	topID, bottomID := arena.Port(topPort).ID, arena.Port(bottomPort).ID
	counter.PortPositions[topID], counter.PortPositions[bottomID] =
		counter.PortPositions[bottomID], counter.PortPositions[topID]
}

func (counter *CrossingsCounter) SwitchNodes(
	arena *layered.LGraphArena,
	wasUpperNode, wasLowerNode layered.LNodeID,
	side layered.PortSide,
) {
	for _, portID := range portsInNorthSouthEastWestOrder(arena, wasUpperNode, side) {
		portIndex := arena.Port(portID).ID
		counter.PortPositions[portIndex] += counter.nodeCardinalities[arena.Node(wasLowerNode).ID]
	}
	for _, portID := range portsInNorthSouthEastWestOrder(arena, wasLowerNode, side) {
		portIndex := arena.Port(portID).ID
		counter.PortPositions[portIndex] -= counter.nodeCardinalities[arena.Node(wasUpperNode).ID]
	}
}

func (counter *CrossingsCounter) connectedInLayerPortsSortedByPosition(
	arena *layered.LGraphArena,
	upperNode, lowerNode layered.LNodeID,
	side layered.PortSide,
) []layered.LPortID {
	byPosition := make(map[int]layered.LPortID)
	for _, nodeID := range []layered.LNodeID{upperNode, lowerNode} {
		for _, portID := range portsInNorthSouthEastWestOrder(arena, nodeID, side) {
			for _, edgeID := range arena.PortConnectedEdges(portID) {
				if arena.EdgeIsSelfLoop(edgeID) {
					continue
				}
				byPosition[counter.positionOf(arena, portID)] = portID
				if arena.EdgeIsInLayer(edgeID) {
					other := arena.EdgeOtherPort(edgeID, portID)
					byPosition[counter.positionOf(arena, other)] = other
				}
			}
		}
	}
	return portsSortedByPosition(byPosition)
}

func (counter *CrossingsCounter) connectedPortsSortedByPosition(
	arena *layered.LGraphArena,
	upperPort, lowerPort layered.LPortID,
) []layered.LPortID {
	byPosition := make(map[int]layered.LPortID)
	for _, portID := range []layered.LPortID{upperPort, lowerPort} {
		byPosition[counter.positionOf(arena, portID)] = portID
		for _, edgeID := range arena.PortConnectedEdges(portID) {
			edge := arena.Edge(edgeID)
			if edge.Source != nil && edge.Target != nil && *edge.Source == *edge.Target {
				continue
			}
			other := arena.EdgeOtherPort(edgeID, portID)
			byPosition[counter.positionOf(arena, other)] = other
		}
	}
	return portsSortedByPosition(byPosition)
}

func portsSortedByPosition(byPosition map[int]layered.LPortID) []layered.LPortID {
	positions := make([]int, 0, len(byPosition))
	for position := range byPosition {
		positions = append(positions, position)
	}
	sort.Ints(positions)
	result := make([]layered.LPortID, 0, len(positions))
	for _, position := range positions {
		result = append(result, byPosition[position])
	}
	return result
}

func (counter *CrossingsCounter) countCrossingsOnPorts(
	arena *layered.LGraphArena,
	ports []layered.LPortID,
) int {
	crossings := 0
	for _, portID := range ports {
		position := counter.positionOf(arena, portID)
		counter.indexTree.removeAll(position)
		for _, edgeID := range arena.PortConnectedEdges(portID) {
			other := arena.EdgeOtherPort(edgeID, portID)
			endPosition := counter.positionOf(arena, other)
			if endPosition > position {
				crossings += counter.indexTree.rank(endPosition)
				counter.ends = append(counter.ends, endPosition)
			}
		}
		counter.addEnds()
	}
	return crossings
}

func (counter *CrossingsCounter) countInLayerCrossingsOnPorts(
	arena *layered.LGraphArena,
	ports []layered.LPortID,
) int {
	crossings := 0
	for _, portID := range ports {
		position := counter.positionOf(arena, portID)
		counter.indexTree.removeAll(position)
		betweenLayerEdges := 0
		for _, edgeID := range arena.PortConnectedEdges(portID) {
			if arena.EdgeIsInLayer(edgeID) {
				endPosition := counter.positionOf(arena, arena.EdgeOtherPort(edgeID, portID))
				if endPosition > position {
					crossings += counter.indexTree.rank(endPosition)
					counter.ends = append(counter.ends, endPosition)
				}
			} else {
				betweenLayerEdges++
			}
		}
		crossings += counter.indexTree.size * betweenLayerEdges
		counter.addEnds()
	}
	return crossings
}

func (counter *CrossingsCounter) countNorthSouthCrossingsOnPorts(
	arena *layered.LGraphArena,
	ports []layered.LPortID,
) int {
	type targetAndDegree struct {
		port   layered.LPortID
		degree int
	}
	crossings := 0
	for _, portID := range ports {
		position := counter.positionOf(arena, portID)
		counter.indexTree.removeAll(position)
		targets := make([]targetAndDegree, 0, 2)
		nodeID := *arena.Port(portID).Node
		switch arena.Node(nodeID).Type {
		case layered.NormalNode:
			if dummyID, ok := rawNodeID(&arena.Port(portID).Properties, portDummyPropertyID); ok {
				for _, targetID := range arena.Node(dummyID).Ports {
					targets = append(targets, targetAndDegree{targetID, len(arena.PortConnectedEdges(targetID))})
				}
			}
		case layered.LongEdgeNode:
			for _, targetID := range arena.Node(nodeID).Ports {
				if targetID != portID {
					targets = append(targets, targetAndDegree{targetID, len(arena.PortConnectedEdges(targetID))})
					break
				}
			}
		case layered.NorthSouthPortNode:
			if originID, ok := rawPortID(&arena.Port(portID).Properties, originPropertyID); ok {
				targets = append(targets, targetAndDegree{originID, len(arena.PortConnectedEdges(portID))})
			}
		}

		for _, target := range targets {
			endPosition := counter.positionOf(arena, target.port)
			if endPosition > position {
				crossings += counter.indexTree.rank(endPosition) * target.degree
				counter.ends = append(counter.ends, endPosition)
			}
		}
		counter.addEnds()
	}
	return crossings
}

func (counter *CrossingsCounter) addEnds() {
	for len(counter.ends) > 0 {
		last := len(counter.ends) - 1
		counter.indexTree.add(counter.ends[last])
		counter.ends = counter.ends[:last]
	}
}

func (counter *CrossingsCounter) initPositions(
	arena *layered.LGraphArena,
	nodes []layered.LNodeID,
	ports *[]layered.LPortID,
	side layered.PortSide,
	topDown, getCardinalities bool,
) {
	numPorts := len(*ports)
	if getCardinalities {
		counter.nodeCardinalities = make([]int, len(nodes))
	}
	start, end, step := 0, len(nodes), 1
	if !topDown {
		start, end, step = len(nodes)-1, -1, -1
	}
	for index := start; index != end; index += step {
		nodeID := nodes[index]
		nodePorts := counter.portsForDirection(arena, nodeID, side, topDown)
		if getCardinalities {
			counter.nodeCardinalities[arena.Node(nodeID).ID] = len(nodePorts)
		}
		for _, portID := range nodePorts {
			counter.PortPositions[arena.Port(portID).ID] = numPorts
			numPorts++
		}
		*ports = append(*ports, nodePorts...)
	}
}

func (counter *CrossingsCounter) initPortPositionsCounterClockwise(
	arena *layered.LGraphArena,
	leftLayerNodes, rightLayerNodes []layered.LNodeID,
) []layered.LPortID {
	ports := make([]layered.LPortID, 0)
	counter.initPositions(arena, leftLayerNodes, &ports, layered.EastSide, true, false)
	counter.initPositions(arena, rightLayerNodes, &ports, layered.WestSide, false, false)
	return ports
}

func (counter *CrossingsCounter) initPositionsForNorthSouthCounting(
	arena *layered.LGraphArena,
	nodes []layered.LNodeID,
) []layered.LPortID {
	ports := make([]layered.LPortID, 0)
	stack := make([]layered.LNodeID, 0)
	var lastLayoutUnit *layered.LNodeID
	index := 0
	for _, current := range nodes {
		if layoutUnitChanged(arena, lastLayoutUnit, current) {
			index = counter.emptyStack(arena, &stack, &ports, layered.EastSide, index)
		}
		if unit, ok := rawNodeID(&arena.Node(current).Properties, inLayerLayoutUnitPropertyID); ok {
			unitCopy := unit
			lastLayoutUnit = &unitCopy
		}

		switch arena.Node(current).Type {
		case layered.NormalNode:
			for _, portID := range portsWithDummy(arena, current, layered.NorthSide) {
				counter.PortPositions[arena.Port(portID).ID] = index
				index++
				ports = append(ports, portID)
			}
			index = counter.emptyStack(arena, &stack, &ports, layered.EastSide, index)
			for _, portID := range portsWithDummy(arena, current, layered.SouthSide) {
				counter.PortPositions[arena.Port(portID).ID] = index
				index++
				ports = append(ports, portID)
			}
		case layered.NorthSouthPortNode:
			if view := arena.NodePortSideView(current, layered.WestSide); len(view) > 0 {
				portID := view[0]
				counter.PortPositions[arena.Port(portID).ID] = index
				index++
				ports = append(ports, portID)
			}
			if len(arena.NodePortSideView(current, layered.EastSide)) > 0 {
				stack = append(stack, current)
			}
		case layered.LongEdgeNode:
			for _, portID := range arena.NodePortSideView(current, layered.WestSide) {
				counter.PortPositions[arena.Port(portID).ID] = index
				index++
				ports = append(ports, portID)
			}
			for range arena.NodePortSideView(current, layered.EastSide) {
				stack = append(stack, current)
			}
		}
	}
	counter.emptyStack(arena, &stack, &ports, layered.EastSide, index)
	return ports
}

func (counter *CrossingsCounter) emptyStack(
	arena *layered.LGraphArena,
	stack *[]layered.LNodeID,
	ports *[]layered.LPortID,
	side layered.PortSide,
	index int,
) int {
	for len(*stack) > 0 {
		last := len(*stack) - 1
		dummyID := (*stack)[last]
		*stack = (*stack)[:last]
		portID := arena.NodePortSideView(dummyID, side)[0]
		counter.PortPositions[arena.Port(portID).ID] = index
		index++
		*ports = append(*ports, portID)
	}
	return index
}

func (counter *CrossingsCounter) portsForDirection(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	side layered.PortSide,
	topDown bool,
) []layered.LPortID {
	ports := arena.NodePortSideView(nodeID, side)
	reverse := side == layered.EastSide && !topDown || side != layered.EastSide && topDown
	if !reverse {
		return ports
	}
	result := append([]layered.LPortID(nil), ports...)
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result
}

func (counter *CrossingsCounter) positionOf(arena *layered.LGraphArena, portID layered.LPortID) int {
	return counter.PortPositions[arena.Port(portID).ID]
}

func portsWithDummy(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	side layered.PortSide,
) []layered.LPortID {
	result := make([]layered.LPortID, 0)
	for _, portID := range arena.NodePortSideView(nodeID, side) {
		if arena.Port(portID).Properties.Has(portDummyPropertyID) {
			result = append(result, portID)
		}
	}
	return result
}

func layoutUnitChanged(
	arena *layered.LGraphArena,
	lastUnit *layered.LNodeID,
	nodeID layered.LNodeID,
) bool {
	if lastUnit == nil || *lastUnit == nodeID || !arena.Node(nodeID).Properties.Has(inLayerLayoutUnitPropertyID) {
		return false
	}
	unit, ok := rawNodeID(&arena.Node(nodeID).Properties, inLayerLayoutUnitPropertyID)
	return ok && unit != *lastUnit
}

type rawPropertyMap interface {
	Raw(string) (any, bool)
}

func rawNodeID(properties rawPropertyMap, id string) (layered.LNodeID, bool) {
	value, ok := properties.Raw(id)
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case layered.LNodeID:
		return typed, true
	case *layered.LNodeID:
		if typed != nil {
			return *typed, true
		}
		return 0, false
	default:
		return 0, false
	}
}

func rawPortID(properties rawPropertyMap, id string) (layered.LPortID, bool) {
	value, ok := properties.Raw(id)
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case layered.LPortID:
		return typed, true
	case *layered.LPortID:
		if typed != nil {
			return *typed, true
		}
		return 0, false
	case layered.Origin:
		if typed.Kind == layered.OriginLPort {
			return typed.LPort, true
		}
		return 0, false
	default:
		return 0, false
	}
}
