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

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// Raw keys of the properties read without a type check: their stored value
// may be an ID or a pointer to one.
var (
	inLayerLayoutUnitKey = layered.InLayerLayoutUnit.Key()
	portDummyKey         = layered.PortDummy.Key()
	originKey            = layered.OriginalElement.Key()
)

// CrossingsCounter is the exact ELK 0.8.1 reduction of between-layer,
// in-layer, and north/south crossings to inversion counting. Port IDs are
// scratch indices into PortPositions; AllCrossingsCounter assigns them in
// Java traversal order.
type CrossingsCounter struct {
	PortPositions []int
	indexTree     *binaryIndexedTree

	// scratch holds the counter's reused buffers and its tree's storage:
	// its own, or the workspace's shared ones (every count empties or
	// resets what it uses, and counts never nest).
	scratch *countScratch

	table *portTable // ports and edges, if the minimizer built a table
}

// portIndex is arena.Port(portID).ID.
func (counter *CrossingsCounter) portIndex(arena *layered.LGraphArena, portID layered.LPortID) int {
	if table := counter.table; table != nil {
		return int(table.ports[portID].id)
	}
	return arena.Port(portID).ID
}

// countScratch is a crossings counter's working storage. ports is the
// reused result buffer of the position initializers (every caller consumes
// it before the next initialization); ends is empty between ports.
type countScratch struct {
	ports         []layered.LPortID
	ends          []int
	cardinalities []int
	tree          binaryIndexedTree
}

// resetIndexTree gives the counter an empty tree for values in [0,maxNum),
// the state newBinaryIndexedTree(maxNum) returns, in its scratch storage.
func (counter *CrossingsCounter) resetIndexTree(maxNum int) {
	if counter.indexTree == nil {
		counter.indexTree = &counter.scratch.tree
	}
	counter.indexTree.reset(maxNum)
}

func NewCrossingsCounter(portPositions []int) *CrossingsCounter {
	return &CrossingsCounter{PortPositions: portPositions, scratch: &countScratch{}}
}

func (counter *CrossingsCounter) CountCrossingsBetweenLayers(
	arena *layered.LGraphArena,
	leftLayerNodes, rightLayerNodes []layered.LNodeID,
) int {
	if table := counter.table; table != nil {
		ports := counter.scratch.ports[:0]
		ports = counter.initPositionsFromTable(arena, table, leftLayerNodes, ports, layered.EastSide, true, false)
		ports = counter.initPositionsFromTable(arena, table, rightLayerNodes, ports, layered.WestSide, false, false)
		counter.scratch.ports = ports
		counter.resetIndexTree(len(ports))
		return counter.countCrossingsOnPortsFromTable(table, ports)
	}
	ports := counter.initPortPositionsCounterClockwise(arena, leftLayerNodes, rightLayerNodes)
	counter.resetIndexTree(len(ports))
	return counter.countCrossingsOnPorts(arena, ports)
}

func (counter *CrossingsCounter) CountInLayerCrossingsOnSide(
	arena *layered.LGraphArena,
	nodes []layered.LNodeID,
	side layered.PortSide,
) int {
	if table := counter.table; table != nil {
		ports := counter.initPositionsFromTable(arena, table, nodes, counter.scratch.ports[:0], side, true, true)
		counter.scratch.ports = ports
		counter.resetIndexTree(len(ports))
		return counter.countInLayerCrossingsOnPortsFromTable(table, ports)
	}
	ports := counter.InitPortPositionsForInLayerCrossings(arena, nodes, side)
	return counter.countInLayerCrossingsOnPorts(arena, ports)
}

// initPositionsFromTable is initPositions with the ports' scratch IDs read
// from the table and the position list kept in a local slice (appended to
// ports and returned).
func (counter *CrossingsCounter) initPositionsFromTable(
	arena *layered.LGraphArena,
	table *portTable,
	nodes []layered.LNodeID,
	ports []layered.LPortID,
	side layered.PortSide,
	topDown, getCardinalities bool,
) []layered.LPortID {
	numPorts := len(ports)
	positions := counter.PortPositions
	entries := table.ports
	var cardinalities []int
	if getCardinalities {
		cardinalities = zeroedInts(counter.scratch.cardinalities, len(nodes))
		counter.scratch.cardinalities = cardinalities
	}
	start, end, step := 0, len(nodes), 1
	if !topDown {
		start, end, step = len(nodes)-1, -1, -1
	}
	// portsForDirection's order: the side view, reversed for EAST bottom-up
	// and for the other sides top-down.
	reverse := side == layered.EastSide && !topDown || side != layered.EastSide && topDown
	for index := start; index != end; index += step {
		nodeID := nodes[index]
		view := arena.NodePortSideView(nodeID, side)
		if getCardinalities {
			cardinalities[arena.Node(nodeID).ID] = len(view)
		}
		if reverse {
			for portIndex := len(view) - 1; portIndex >= 0; portIndex-- {
				portID := view[portIndex]
				positions[entries[portID].id] = numPorts
				numPorts++
				ports = append(ports, portID)
			}
		} else {
			for _, portID := range view {
				positions[entries[portID].id] = numPorts
				numPorts++
				ports = append(ports, portID)
			}
		}
	}
	return ports
}

// countCrossingsOnPortsFromTable is countCrossingsOnPorts' table branch with
// the tree and the pending ends in local variables: the same tree
// operations in the same order.
func (counter *CrossingsCounter) countCrossingsOnPortsFromTable(table *portTable, ports []layered.LPortID) int {
	crossings := 0
	positions := counter.PortPositions
	tree := counter.indexTree
	pending := counter.scratch.ends
	for _, portID := range ports {
		entry := &table.ports[portID]
		position := positions[entry.id]
		tree.removeAll(position)
		ends := table.connected(entry)
		for index := range ends {
			endPosition := positions[ends[index].otherID]
			if endPosition > position {
				crossings += tree.rank(endPosition)
				pending = append(pending, endPosition)
			}
		}
		for len(pending) > 0 {
			last := len(pending) - 1
			tree.add(pending[last])
			pending = pending[:last]
		}
	}
	counter.scratch.ends = pending
	return crossings
}

// countInLayerCrossingsOnPortsFromTable is countInLayerCrossingsOnPorts'
// table branch with local variables (see countCrossingsOnPortsFromTable).
func (counter *CrossingsCounter) countInLayerCrossingsOnPortsFromTable(table *portTable, ports []layered.LPortID) int {
	crossings := 0
	positions := counter.PortPositions
	tree := counter.indexTree
	pending := counter.scratch.ends
	for _, portID := range ports {
		entry := &table.ports[portID]
		position := positions[entry.id]
		tree.removeAll(position)
		betweenLayerEdges := 0
		ends := table.connected(entry)
		for index := range ends {
			if ends[index].inLayer {
				endPosition := positions[ends[index].otherID]
				if endPosition > position {
					crossings += tree.rank(endPosition)
					pending = append(pending, endPosition)
				}
			} else {
				betweenLayerEdges++
			}
		}
		crossings += tree.size * betweenLayerEdges
		for len(pending) > 0 {
			last := len(pending) - 1
			tree.add(pending[last])
			pending = pending[:last]
		}
	}
	counter.scratch.ends = pending
	return crossings
}

func (counter *CrossingsCounter) CountNorthSouthPortCrossingsInLayer(
	arena *layered.LGraphArena,
	nodes []layered.LNodeID,
) int {
	ports := counter.initPositionsForNorthSouthCounting(arena, nodes)
	counter.resetIndexTree(len(ports))
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
	counter.resetIndexTree(len(ports))
}

func (counter *CrossingsCounter) InitPortPositionsForInLayerCrossings(
	arena *layered.LGraphArena,
	nodes []layered.LNodeID,
	side layered.PortSide,
) []layered.LPortID {
	ports := counter.scratch.ports[:0]
	counter.initPositions(arena, nodes, &ports, side, true, true)
	counter.scratch.ports = ports
	counter.resetIndexTree(len(ports))
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
		counter.PortPositions[portIndex] += counter.scratch.cardinalities[arena.Node(wasLowerNode).ID]
	}
	for _, portID := range portsInNorthSouthEastWestOrder(arena, wasLowerNode, side) {
		portIndex := arena.Port(portID).ID
		counter.PortPositions[portIndex] -= counter.scratch.cardinalities[arena.Node(wasUpperNode).ID]
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
	if table := counter.table; table != nil {
		// The edges' other ends as EdgeOtherPort returns them, incoming
		// edges first (PortConnectedEdges' order).
		positions := counter.PortPositions
		for _, portID := range ports {
			entry := &table.ports[portID]
			position := positions[entry.id]
			counter.indexTree.removeAll(position)
			for _, end := range table.connected(entry) {
				endPosition := positions[end.otherID]
				if endPosition > position {
					crossings += counter.indexTree.rank(endPosition)
					counter.scratch.ends = append(counter.scratch.ends, endPosition)
				}
			}
			counter.addEnds()
		}
		return crossings
	}
	for _, portID := range ports {
		position := counter.positionOf(arena, portID)
		counter.indexTree.removeAll(position)
		// PortConnectedEdges order: incoming, then outgoing.
		port := arena.Port(portID)
		for _, edges := range [2][]layered.LEdgeID{port.IncomingEdges, port.OutgoingEdges} {
			for _, edgeID := range edges {
				other := arena.EdgeOtherPort(edgeID, portID)
				endPosition := counter.positionOf(arena, other)
				if endPosition > position {
					crossings += counter.indexTree.rank(endPosition)
					counter.scratch.ends = append(counter.scratch.ends, endPosition)
				}
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
	if table := counter.table; table != nil {
		positions := counter.PortPositions
		for _, portID := range ports {
			entry := &table.ports[portID]
			position := positions[entry.id]
			counter.indexTree.removeAll(position)
			betweenLayerEdges := 0
			for _, end := range table.connected(entry) {
				if end.inLayer {
					endPosition := positions[end.otherID]
					if endPosition > position {
						crossings += counter.indexTree.rank(endPosition)
						counter.scratch.ends = append(counter.scratch.ends, endPosition)
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
	for _, portID := range ports {
		position := counter.positionOf(arena, portID)
		counter.indexTree.removeAll(position)
		betweenLayerEdges := 0
		port := arena.Port(portID)
		for _, edges := range [2][]layered.LEdgeID{port.IncomingEdges, port.OutgoingEdges} {
			for _, edgeID := range edges {
				if arena.EdgeIsInLayer(edgeID) {
					endPosition := counter.positionOf(arena, arena.EdgeOtherPort(edgeID, portID))
					if endPosition > position {
						crossings += counter.indexTree.rank(endPosition)
						counter.scratch.ends = append(counter.scratch.ends, endPosition)
					}
				} else {
					betweenLayerEdges++
				}
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
			if dummyID, ok := rawNodeID(&arena.Port(portID).Properties, portDummyKey); ok {
				for _, targetID := range arena.Node(dummyID).Ports {
					targets = append(targets, targetAndDegree{targetID, portDegree(arena, targetID)})
				}
			}
		case layered.LongEdgeNode:
			for _, targetID := range arena.Node(nodeID).Ports {
				if targetID != portID {
					targets = append(targets, targetAndDegree{targetID, portDegree(arena, targetID)})
					break
				}
			}
		case layered.NorthSouthPortNode:
			if originID, ok := rawPortID(&arena.Port(portID).Properties, originKey); ok {
				targets = append(targets, targetAndDegree{originID, portDegree(arena, portID)})
			}
		}

		for _, target := range targets {
			endPosition := counter.positionOf(arena, target.port)
			if endPosition > position {
				crossings += counter.indexTree.rank(endPosition) * target.degree
				counter.scratch.ends = append(counter.scratch.ends, endPosition)
			}
		}
		counter.addEnds()
	}
	return crossings
}

func (counter *CrossingsCounter) addEnds() {
	for len(counter.scratch.ends) > 0 {
		last := len(counter.scratch.ends) - 1
		counter.indexTree.add(counter.scratch.ends[last])
		counter.scratch.ends = counter.scratch.ends[:last]
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
		counter.scratch.cardinalities = zeroedInts(counter.scratch.cardinalities, len(nodes))
	}
	start, end, step := 0, len(nodes), 1
	if !topDown {
		start, end, step = len(nodes)-1, -1, -1
	}
	// portsForDirection's order: the side view, reversed for EAST bottom-up
	// and for the other sides top-down.
	reverse := side == layered.EastSide && !topDown || side != layered.EastSide && topDown
	for index := start; index != end; index += step {
		nodeID := nodes[index]
		view := arena.NodePortSideView(nodeID, side)
		if getCardinalities {
			counter.scratch.cardinalities[arena.Node(nodeID).ID] = len(view)
		}
		if reverse {
			for portIndex := len(view) - 1; portIndex >= 0; portIndex-- {
				portID := view[portIndex]
				counter.PortPositions[counter.portIndex(arena, portID)] = numPorts
				numPorts++
				*ports = append(*ports, portID)
			}
		} else {
			for _, portID := range view {
				counter.PortPositions[counter.portIndex(arena, portID)] = numPorts
				numPorts++
				*ports = append(*ports, portID)
			}
		}
	}
}

func (counter *CrossingsCounter) initPortPositionsCounterClockwise(
	arena *layered.LGraphArena,
	leftLayerNodes, rightLayerNodes []layered.LNodeID,
) []layered.LPortID {
	ports := counter.scratch.ports[:0]
	counter.initPositions(arena, leftLayerNodes, &ports, layered.EastSide, true, false)
	counter.initPositions(arena, rightLayerNodes, &ports, layered.WestSide, false, false)
	counter.scratch.ports = ports
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
		if unit, ok := rawNodeID(&arena.Node(current).Properties, inLayerLayoutUnitKey); ok {
			unitCopy := unit
			lastLayoutUnit = &unitCopy
		}

		switch arena.Node(current).Type {
		case layered.NormalNode:
			for _, portID := range portsWithDummy(arena, current, layered.NorthSide) {
				counter.PortPositions[counter.portIndex(arena, portID)] = index
				index++
				ports = append(ports, portID)
			}
			index = counter.emptyStack(arena, &stack, &ports, layered.EastSide, index)
			for _, portID := range portsWithDummy(arena, current, layered.SouthSide) {
				counter.PortPositions[counter.portIndex(arena, portID)] = index
				index++
				ports = append(ports, portID)
			}
		case layered.NorthSouthPortNode:
			if view := arena.NodePortSideView(current, layered.WestSide); len(view) > 0 {
				portID := view[0]
				counter.PortPositions[counter.portIndex(arena, portID)] = index
				index++
				ports = append(ports, portID)
			}
			if len(arena.NodePortSideView(current, layered.EastSide)) > 0 {
				stack = append(stack, current)
			}
		case layered.LongEdgeNode:
			for _, portID := range arena.NodePortSideView(current, layered.WestSide) {
				counter.PortPositions[counter.portIndex(arena, portID)] = index
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
		counter.PortPositions[counter.portIndex(arena, portID)] = index
		index++
		*ports = append(*ports, portID)
	}
	return index
}

func (counter *CrossingsCounter) positionOf(arena *layered.LGraphArena, portID layered.LPortID) int {
	return counter.PortPositions[counter.portIndex(arena, portID)]
}

func portsWithDummy(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	side layered.PortSide,
) []layered.LPortID {
	result := make([]layered.LPortID, 0)
	for _, portID := range arena.NodePortSideView(nodeID, side) {
		if arena.Port(portID).Properties.HasKey(portDummyKey) {
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
	if lastUnit == nil || *lastUnit == nodeID || !arena.Node(nodeID).Properties.HasKey(inLayerLayoutUnitKey) {
		return false
	}
	unit, ok := rawNodeID(&arena.Node(nodeID).Properties, inLayerLayoutUnitKey)
	return ok && unit != *lastUnit
}

// portDegree is len(arena.PortConnectedEdges(portID)) without the copy.
func portDegree(arena *layered.LGraphArena, portID layered.LPortID) int {
	port := arena.Port(portID)
	return len(port.IncomingEdges) + len(port.OutgoingEdges)
}

// zeroedInts returns a zeroed slice of length n, reusing buffer's storage.
func zeroedInts(buffer []int, n int) []int {
	if cap(buffer) < n {
		return make([]int, n)
	}
	buffer = buffer[:n]
	for index := range buffer {
		buffer[index] = 0
	}
	return buffer
}

func rawNodeID(properties *basegraph.PropertyMap, key basegraph.PropertyKey) (layered.LNodeID, bool) {
	value, ok := properties.RawKey(key)
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

func rawPortID(properties *basegraph.PropertyMap, key basegraph.PropertyKey) (layered.LPortID, bool) {
	value, ok := properties.RawKey(key)
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
