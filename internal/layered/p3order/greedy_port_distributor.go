// Copyright (c) 2016 Kiel University.
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

// GreedyPortDistributor is the distributor used by ELK's two-sided greedy
// switch. It repeatedly swaps adjacent ports whenever the swap strictly
// decreases crossings.
type GreedyPortDistributor struct {
	crossingsCounter             *CrossingsCounter
	nPorts                       int
	hierarchicalCrossingsCounter *betweenLayerEdgeTwoNodeCrossingsCounter
}

func NewGreedyPortDistributor() *GreedyPortDistributor { return &GreedyPortDistributor{} }

func (distributor *GreedyPortDistributor) InitAtNodeLevel(
	arena *layered.LGraphArena,
	layerIndex, nodeIndex int,
	order [][]layered.LNodeID,
) {
	distributor.nPorts += len(arena.Node(order[layerIndex][nodeIndex]).Ports)
}

func (distributor *GreedyPortDistributor) InitAfterTraversal() {
	distributor.crossingsCounter = NewCrossingsCounter(make([]int, distributor.nPorts))
}

func InitializeGreedyPortDistributor(
	distributor *GreedyPortDistributor,
	arena *layered.LGraphArena,
	order [][]layered.LNodeID,
) {
	nPorts := 0
	for layerIndex, nodes := range order {
		for nodeIndex, nodeID := range nodes {
			distributor.InitAtNodeLevel(arena, layerIndex, nodeIndex, order)
			for _, portID := range arena.Node(nodeID).Ports {
				arena.Port(portID).ID = nPorts
				nPorts++
			}
		}
	}
	distributor.InitAfterTraversal()
}

func (distributor *GreedyPortDistributor) DistributePortsWhileSweeping(
	arena *layered.LGraphArena,
	order [][]layered.LNodeID,
	currentIndex int,
	forward bool,
) bool {
	distributor.initialize(arena, order, currentIndex, forward)
	return distributor.distributePortsInLayer(arena, order, currentIndex, forward)
}

func (distributor *GreedyPortDistributor) initialize(
	arena *layered.LGraphArena,
	order [][]layered.LNodeID,
	currentIndex int,
	forward bool,
) {
	if distributor.crossingsCounter == nil {
		panic("p3order: GreedyPortDistributor used before InitAfterTraversal")
	}
	if forward && currentIndex > 0 {
		distributor.crossingsCounter.InitForCountingBetween(
			arena, order[currentIndex-1], order[currentIndex],
		)
	} else if !forward && currentIndex < len(order)-1 {
		distributor.crossingsCounter.InitForCountingBetween(
			arena, order[currentIndex], order[currentIndex+1],
		)
	} else {
		side := layered.EastSide
		if forward {
			side = layered.WestSide
		}
		distributor.crossingsCounter.InitPortPositionsForInLayerCrossings(
			arena, order[currentIndex], side,
		)
	}
}

func (distributor *GreedyPortDistributor) distributePortsInLayer(
	arena *layered.LGraphArena,
	order [][]layered.LNodeID,
	currentIndex int,
	forward bool,
) bool {
	side := layered.EastSide
	if forward {
		side = layered.WestSide
	}
	improved := false
	for _, nodeID := range order[currentIndex] {
		if basegraph.Get(&arena.Node(nodeID).Properties, layered.PortConstraintsOption).IsOrderFixed() {
			continue
		}
		nested := arena.Node(nodeID).NestedGraph
		useHierarchicalCounter := len(arena.NodePortsOnSide(nodeID, side)) > 0 && nested != nil
		if useHierarchicalCounter {
			innerOrder := nodeOrderOfGraph(arena, *nested)
			freeLayer := len(innerOrder) - 1
			if forward {
				freeLayer = 0
			}
			distributor.hierarchicalCrossingsCounter =
				newBetweenLayerEdgeTwoNodeCrossingsCounter(arena, innerOrder, freeLayer)
		}
		if distributor.distributePortsOnNode(arena, nodeID, side, useHierarchicalCounter) {
			improved = true
		}
	}
	return improved
}

func (distributor *GreedyPortDistributor) distributePortsOnNode(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	side layered.PortSide,
	useHierarchicalCounter bool,
) bool {
	ports := append([]layered.LPortID(nil), arena.NodePortsOnSide(nodeID, side)...)
	if side == layered.SouthSide || side == layered.WestSide {
		reversePorts(ports)
	}
	improved := false
	for {
		continueSwitching := false
		for index := 0; index+1 < len(ports); index++ {
			if !distributor.switchingDecreasesCrossings(
				arena, ports[index], ports[index+1], useHierarchicalCounter,
			) {
				continue
			}
			improved = true
			distributor.switchPorts(arena, nodeID, ports, index, index+1)
			continueSwitching = true
		}
		if !continueSwitching {
			break
		}
	}
	if improved {
		arena.InvalidatePortSideCache(nodeID)
	}
	return improved
}

func (distributor *GreedyPortDistributor) switchingDecreasesCrossings(
	arena *layered.LGraphArena,
	upperPort, lowerPort layered.LPortID,
	useHierarchicalCounter bool,
) bool {
	upperLower, lowerUpper := distributor.crossingsCounter.CountCrossingsBetweenPortsInBothOrders(
		arena, upperPort, lowerPort,
	)
	if useHierarchicalCounter {
		upperNode, upperFound := basegraph.TryGet(&arena.Port(upperPort).Properties, layered.PortDummy)
		lowerNode, lowerFound := basegraph.TryGet(&arena.Port(lowerPort).Properties, layered.PortDummy)
		if upperFound && lowerFound {
			distributor.hierarchicalCrossingsCounter.countBothSideCrossings(arena, upperNode, lowerNode)
			upperLower += distributor.hierarchicalCrossingsCounter.upperLowerCrossings
			lowerUpper += distributor.hierarchicalCrossingsCounter.lowerUpperCrossings
		}
	}
	return upperLower > lowerUpper
}

func (distributor *GreedyPortDistributor) switchPorts(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	ports []layered.LPortID,
	top, bottom int,
) {
	distributor.crossingsCounter.SwitchPorts(arena, ports[top], ports[bottom])
	nodePorts := arena.Node(nodeID).Ports
	topIndex, bottomIndex := -1, -1
	for index, portID := range nodePorts {
		if portID == ports[top] {
			topIndex = index
		}
		if portID == ports[bottom] {
			bottomIndex = index
		}
	}
	if topIndex < 0 || bottomIndex < 0 {
		panic("p3order: port side view does not belong to node")
	}
	nodePorts[topIndex], nodePorts[bottomIndex] = nodePorts[bottomIndex], nodePorts[topIndex]
	ports[top], ports[bottom] = ports[bottom], ports[top]
	arena.Node(nodeID).Ports = nodePorts
}

func nodeOrderOfGraph(arena *layered.LGraphArena, graphID layered.LGraphID) [][]layered.LNodeID {
	order := make([][]layered.LNodeID, len(arena.Graph(graphID).Layers))
	for index, layerID := range arena.Graph(graphID).Layers {
		order[index] = append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...)
	}
	return order
}

type betweenLayerEdgeTwoNodeCrossingsCounter struct {
	upperLowerCrossings int
	lowerUpperCrossings int
	freeLayer           []layered.LNodeID
	portPositions       map[layered.LPortID]int
	easternAdjacencies  map[layered.LNodeID]*portAdjacencyList
	westernAdjacencies  map[layered.LNodeID]*portAdjacencyList
}

func newBetweenLayerEdgeTwoNodeCrossingsCounter(
	arena *layered.LGraphArena,
	order [][]layered.LNodeID,
	freeLayer int,
) *betweenLayerEdgeTwoNodeCrossingsCounter {
	counter := &betweenLayerEdgeTwoNodeCrossingsCounter{
		freeLayer:          append([]layered.LNodeID(nil), order[freeLayer]...),
		portPositions:      make(map[layered.LPortID]int),
		easternAdjacencies: make(map[layered.LNodeID]*portAdjacencyList),
		westernAdjacencies: make(map[layered.LNodeID]*portAdjacencyList),
	}
	if freeLayer > 0 {
		counter.setPortPositionsForLayer(arena, order[freeLayer-1], layered.EastSide)
	}
	if freeLayer < len(order)-1 {
		counter.setPortPositionsForLayer(arena, order[freeLayer+1], layered.WestSide)
	}
	return counter
}

func (counter *betweenLayerEdgeTwoNodeCrossingsCounter) setPortPositionsForLayer(
	arena *layered.LGraphArena,
	layer []layered.LNodeID,
	side layered.PortSide,
) {
	position := 0
	for _, nodeID := range layer {
		for _, portID := range portsInNorthSouthEastWestOrder(arena, nodeID, side) {
			counter.portPositions[portID] = position
			position++
		}
	}
}

func (counter *betweenLayerEdgeTwoNodeCrossingsCounter) countBothSideCrossings(
	arena *layered.LGraphArena,
	upper, lower layered.LNodeID,
) {
	counter.upperLowerCrossings = 0
	counter.lowerUpperCrossings = 0
	if upper == lower {
		return
	}
	counter.addCrossings(arena, upper, lower, layered.WestSide)
	counter.addCrossings(arena, upper, lower, layered.EastSide)
}

func (counter *betweenLayerEdgeTwoNodeCrossingsCounter) countEasternEdgeCrossings(
	arena *layered.LGraphArena,
	upper, lower layered.LNodeID,
) {
	counter.upperLowerCrossings = 0
	counter.lowerUpperCrossings = 0
	if upper != lower {
		counter.addCrossings(arena, upper, lower, layered.EastSide)
	}
}

func (counter *betweenLayerEdgeTwoNodeCrossingsCounter) countWesternEdgeCrossings(
	arena *layered.LGraphArena,
	upper, lower layered.LNodeID,
) {
	counter.upperLowerCrossings = 0
	counter.lowerUpperCrossings = 0
	if upper != lower {
		counter.addCrossings(arena, upper, lower, layered.WestSide)
	}
}

func (counter *betweenLayerEdgeTwoNodeCrossingsCounter) addCrossings(
	arena *layered.LGraphArena,
	upper, lower layered.LNodeID,
	side layered.PortSide,
) {
	adjacencies := counter.westernAdjacencies
	if side == layered.EastSide {
		adjacencies = counter.easternAdjacencies
	}
	if len(adjacencies) == 0 {
		for _, nodeID := range counter.freeLayer {
			adjacencies[nodeID] = newPortAdjacencyList(
				arena, nodeID, side, counter.portPositions,
			)
		}
	}
	upperList, lowerList := adjacencies[upper], adjacencies[lower]
	upperList.reset()
	lowerList.reset()
	for !upperList.empty() && !lowerList.empty() {
		if upperList.first() > lowerList.first() {
			counter.upperLowerCrossings += upperList.currentSize
			lowerList.removeFirst()
		} else if lowerList.first() > upperList.first() {
			counter.lowerUpperCrossings += lowerList.currentSize
			upperList.removeFirst()
		} else {
			counter.upperLowerCrossings += upperList.adjacenciesBelowFirst()
			counter.lowerUpperCrossings += lowerList.adjacenciesBelowFirst()
			upperList.removeFirst()
			lowerList.removeFirst()
		}
	}
}

type portAdjacency struct {
	position           int
	cardinality        int
	currentCardinality int
}

type portAdjacencyList struct {
	entries      []portAdjacency
	size         int
	currentSize  int
	currentIndex int
}

func newPortAdjacencyList(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	side layered.PortSide,
	portPositions map[layered.LPortID]int,
) *portAdjacencyList {
	list := &portAdjacencyList{}
	for _, portID := range portsInNorthSouthEastWestOrder(arena, nodeID, side) {
		edges := arena.Port(portID).OutgoingEdges
		if side == layered.WestSide {
			edges = arena.Port(portID).IncomingEdges
		}
		for _, edgeID := range edges {
			if arena.EdgeIsSelfLoop(edgeID) || arena.EdgeIsInLayer(edgeID) {
				continue
			}
			adjacentPort := *arena.Edge(edgeID).Target
			if side == layered.WestSide {
				adjacentPort = *arena.Edge(edgeID).Source
			}
			position, found := portPositions[adjacentPort]
			if !found {
				panic("p3order: adjacent port is not in a neighboring layer")
			}
			last := len(list.entries) - 1
			if last >= 0 && list.entries[last].position == position {
				list.entries[last].cardinality++
				list.entries[last].currentCardinality++
			} else {
				list.entries = append(list.entries, portAdjacency{position, 1, 1})
			}
			list.size++
			list.currentSize++
		}
	}
	sort.SliceStable(list.entries, func(i, j int) bool {
		return list.entries[i].position < list.entries[j].position
	})
	return list
}

func (list *portAdjacencyList) reset() {
	list.currentIndex = 0
	list.currentSize = list.size
	if !list.empty() {
		list.entries[0].currentCardinality = list.entries[0].cardinality
	}
}

func (list *portAdjacencyList) empty() bool { return list.currentSize == 0 }

func (list *portAdjacencyList) first() int { return list.entries[list.currentIndex].position }

func (list *portAdjacencyList) adjacenciesBelowFirst() int {
	return list.currentSize - list.entries[list.currentIndex].currentCardinality
}

func (list *portAdjacencyList) removeFirst() {
	if list.empty() {
		return
	}
	entry := &list.entries[list.currentIndex]
	if entry.currentCardinality == 1 {
		list.currentIndex++
		if list.currentIndex < len(list.entries) {
			list.entries[list.currentIndex].currentCardinality =
				list.entries[list.currentIndex].cardinality
		}
	} else {
		entry.currentCardinality--
	}
	list.currentSize--
}

func reversePorts(ports []layered.LPortID) {
	for left, right := 0, len(ports)-1; left < right; left, right = left+1, right-1 {
		ports[left], ports[right] = ports[right], ports[left]
	}
}
