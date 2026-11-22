// Copyright (c) 2014, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import "github.com/d2lang/elk-go/internal/layered"

// AllCrossingsCounter coordinates the specialized counters and implements
// ELK's IInitializable traversal callbacks explicitly.
type AllCrossingsCounter struct {
	crossingCounter          *CrossingsCounter
	hasHyperEdgesEastOfIndex []bool
	hasNorthSouthPorts       []bool
	inLayerEdgeCounts        []int
	nPorts                   int
}

func NewAllCrossingsCounter(numberOfLayers int) *AllCrossingsCounter {
	return newAllCrossingsCounter(nil, numberOfLayers)
}

// newAllCrossingsCounter is NewAllCrossingsCounter with its storage taken
// from ws (nil: allocated).
func newAllCrossingsCounter(ws *crossMinWorkspace, numberOfLayers int) *AllCrossingsCounter {
	counter := ws.allCounterSlab().one()
	counter.hasHyperEdgesEastOfIndex = ws.boolSlab().take(numberOfLayers)
	counter.hasNorthSouthPorts = ws.boolSlab().take(numberOfLayers)
	counter.inLayerEdgeCounts = ws.intSlab().take(numberOfLayers)
	return counter
}

func (counter *AllCrossingsCounter) CountAllCrossings(
	arena *layered.LGraphArena,
	currentOrder [][]layered.LNodeID,
) int {
	if len(currentOrder) == 0 {
		return 0
	}
	if counter.crossingCounter == nil {
		panic("p3order: AllCrossingsCounter used before InitAfterTraversal")
	}
	crossings := counter.crossingCounter.CountInLayerCrossingsOnSide(arena, currentOrder[0], layered.WestSide)
	crossings += counter.crossingCounter.CountInLayerCrossingsOnSide(arena, currentOrder[len(currentOrder)-1], layered.EastSide)
	for layerIndex := range currentOrder {
		crossings += counter.countCrossingsAt(arena, layerIndex, currentOrder)
	}
	return crossings
}

func (counter *AllCrossingsCounter) countCrossingsAt(
	arena *layered.LGraphArena,
	layerIndex int,
	currentOrder [][]layered.LNodeID,
) int {
	total := 0
	leftLayer := currentOrder[layerIndex]
	if layerIndex < len(currentOrder)-1 {
		rightLayer := currentOrder[layerIndex+1]
		if counter.hasHyperEdgesEastOfIndex[layerIndex] {
			total = countHyperedgeCrossings(arena, counter.crossingCounter.PortPositions, leftLayer, rightLayer)
			total += counter.crossingCounter.CountInLayerCrossingsOnSide(arena, leftLayer, layered.EastSide)
			total += counter.crossingCounter.CountInLayerCrossingsOnSide(arena, rightLayer, layered.WestSide)
		} else {
			total = counter.crossingCounter.CountCrossingsBetweenLayers(arena, leftLayer, rightLayer)
		}
	}
	if counter.hasNorthSouthPorts[layerIndex] {
		total += counter.crossingCounter.CountNorthSouthPortCrossingsInLayer(arena, leftLayer)
	}
	return total
}

func (counter *AllCrossingsCounter) InitAtNodeLevel(
	arena *layered.LGraphArena,
	layerIndex, nodeIndex int,
	nodeOrder [][]layered.LNodeID,
) {
	nodeID := nodeOrder[layerIndex][nodeIndex]
	counter.hasNorthSouthPorts[layerIndex] = counter.hasNorthSouthPorts[layerIndex] ||
		arena.Node(nodeID).Type == layered.NorthSouthPortNode
}

func (counter *AllCrossingsCounter) InitAtPortLevel(
	arena *layered.LGraphArena,
	layerIndex, nodeIndex, portIndex int,
	nodeOrder [][]layered.LNodeID,
) {
	portID := arena.Node(nodeOrder[layerIndex][nodeIndex]).Ports[portIndex]
	arena.Port(portID).ID = counter.nPorts
	counter.nPorts++
	port := arena.Port(portID)
	if len(port.OutgoingEdges)+len(port.IncomingEdges) > 1 {
		if port.Side == layered.EastSide {
			counter.hasHyperEdgesEastOfIndex[layerIndex] = true
		} else if port.Side == layered.WestSide && layerIndex > 0 {
			counter.hasHyperEdgesEastOfIndex[layerIndex-1] = true
		}
	}
}

func (counter *AllCrossingsCounter) InitAtEdgeLevel(
	arena *layered.LGraphArena,
	layerIndex, nodeIndex, portIndex int,
	edgeID layered.LEdgeID,
	nodeOrder [][]layered.LNodeID,
) {
	portID := arena.Node(nodeOrder[layerIndex][nodeIndex]).Ports[portIndex]
	edge := arena.Edge(edgeID)
	if edge.Source != nil && *edge.Source == portID && arena.EdgeIsInLayer(edgeID) {
		counter.inLayerEdgeCounts[layerIndex]++
	}
}

func (counter *AllCrossingsCounter) InitAfterTraversal() {
	counter.initAfterTraversal(nil)
}

// initAfterTraversal is InitAfterTraversal with storage from ws (nil:
// allocated); with a workspace the counter's buffers and tree are the
// workspace's shared ones.
func (counter *AllCrossingsCounter) initAfterTraversal(ws *crossMinWorkspace) {
	if ws == nil {
		counter.crossingCounter = NewCrossingsCounter(make([]int, counter.nPorts))
		return
	}
	crossings := ws.counterSlab().one()
	crossings.PortPositions = ws.intSlab().take(counter.nPorts)
	crossings.scratch = &ws.countScratch
	counter.crossingCounter = crossings
}

// InitializeAllCrossingsCounter performs IInitializable.init's exact
// layer/node/port/edge nesting and connected-edge order.
func InitializeAllCrossingsCounter(
	counter *AllCrossingsCounter,
	arena *layered.LGraphArena,
	nodeOrder [][]layered.LNodeID,
) {
	for layerIndex, nodes := range nodeOrder {
		for nodeIndex, nodeID := range nodes {
			counter.InitAtNodeLevel(arena, layerIndex, nodeIndex, nodeOrder)
			for portIndex, portID := range arena.Node(nodeID).Ports {
				counter.InitAtPortLevel(arena, layerIndex, nodeIndex, portIndex, nodeOrder)
				for _, edgeID := range arena.PortConnectedEdges(portID) {
					counter.InitAtEdgeLevel(arena, layerIndex, nodeIndex, portIndex, edgeID, nodeOrder)
				}
			}
		}
	}
	counter.InitAfterTraversal()
}
