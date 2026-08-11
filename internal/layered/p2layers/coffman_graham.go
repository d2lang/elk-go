// Copyright (c) 2016, 2020 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p2layers

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
)

// CoffmanGrahamLayerer bounds the number of nodes per layer after computing
// the exact ELK 0.12.0 transitive reduction and priority topological order.
type CoffmanGrahamLayerer struct {
	nodeMark           []bool
	edgeMark           []bool
	inDegree           []int
	outDegree          []int
	topologicalOrder   []int
	inTopologicalOrder map[layered.LNodeID][]int
}

func (*CoffmanGrahamLayerer) ProcessorConfiguration(
	_ *layered.LGraphArena,
	_ layered.LGraphID,
) layered.ProcessorConfiguration {
	return layered.NetworkSimplexLayeringConfiguration()
}

func (layerer *CoffmanGrahamLayerer) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	graph := arena.Graph(graphID)
	if len(graph.LayerlessNodes) == 0 {
		return nil
	}
	layerBound := basegraph.Get(&graph.Properties, layered.CoffmanGrahamLayerBound)
	edgeCount := 0
	for index, nodeID := range graph.LayerlessNodes {
		arena.Node(nodeID).ID = index
		for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
			arena.Edge(edgeID).ID = edgeCount
			edgeCount++
		}
	}
	layerer.nodeMark = make([]bool, len(graph.LayerlessNodes))
	layerer.edgeMark = make([]bool, edgeCount)
	layerer.inDegree = make([]int, len(graph.LayerlessNodes))
	layerer.outDegree = make([]int, len(graph.LayerlessNodes))
	layerer.topologicalOrder = make([]int, len(graph.LayerlessNodes))
	layerer.inTopologicalOrder = make(map[layered.LNodeID][]int)
	layerer.transitiveReduction(arena, graphID)

	sources := javacompat.NewGWTPriorityQueue(func(first, second layered.LNodeID) int {
		return layerer.compareNodesInTopologicalOrder(arena, first, second)
	})
	for _, nodeID := range graph.LayerlessNodes {
		for _, edgeID := range arena.NodeIncomingEdges(nodeID) {
			if !layerer.edgeMark[arena.Edge(edgeID).ID] {
				layerer.inDegree[arena.Node(nodeID).ID]++
			}
		}
		if layerer.inDegree[arena.Node(nodeID).ID] == 0 {
			sources.Add(nodeID)
		}
	}
	order := 0
	for !sources.IsEmpty() {
		nodeID, _ := sources.Poll()
		layerer.topologicalOrder[arena.Node(nodeID).ID] = order
		order++
		for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
			if layerer.edgeMark[arena.Edge(edgeID).ID] {
				continue
			}
			target := arena.EdgeTargetNode(edgeID)
			targetIndex := arena.Node(target).ID
			layerer.inDegree[targetIndex]--
			layerer.inTopologicalOrder[target] = append(
				layerer.inTopologicalOrder[target], layerer.topologicalOrder[arena.Node(nodeID).ID],
			)
			if layerer.inDegree[targetIndex] == 0 {
				sources.Add(target)
			}
		}
	}

	sinks := javacompat.NewGWTPriorityQueue(func(first, second layered.LNodeID) int {
		firstOrder := layerer.topologicalOrder[arena.Node(first).ID]
		secondOrder := layerer.topologicalOrder[arena.Node(second).ID]
		if firstOrder > secondOrder {
			return -1
		}
		if firstOrder < secondOrder {
			return 1
		}
		return 0
	})
	for _, nodeID := range graph.LayerlessNodes {
		for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
			if !layerer.edgeMark[arena.Edge(edgeID).ID] {
				layerer.outDegree[arena.Node(nodeID).ID]++
			}
		}
		if layerer.outDegree[arena.Node(nodeID).ID] == 0 {
			sinks.Add(nodeID)
		}
	}

	layers := []layered.LayerID{arena.AddLayer(graphID)}
	currentLayer := layers[0]
	for !sinks.IsEmpty() {
		nodeID, _ := sinks.Poll()
		if len(arena.Layer(currentLayer).Nodes) >= layerBound || !layerer.canAdd(arena, nodeID, currentLayer) {
			currentLayer = arena.AddLayer(graphID)
			layers = append(layers, currentLayer)
		}
		arena.SetNodeLayer(nodeID, &currentLayer)
		for _, edgeID := range arena.NodeIncomingEdges(nodeID) {
			if layerer.edgeMark[arena.Edge(edgeID).ID] {
				continue
			}
			source := arena.EdgeSourceNode(edgeID)
			sourceIndex := arena.Node(source).ID
			layerer.outDegree[sourceIndex]--
			if layerer.outDegree[sourceIndex] == 0 {
				sinks.Add(source)
			}
		}
	}
	for index := len(layers) - 1; index >= 0; index-- {
		graph.Layers = append(graph.Layers, layers[index])
	}
	graph.LayerlessNodes = graph.LayerlessNodes[:0]
	return nil
}

func (layerer *CoffmanGrahamLayerer) compareNodesInTopologicalOrder(
	arena *layered.LGraphArena,
	first, second layered.LNodeID,
) int {
	firstValues := layerer.inTopologicalOrder[first]
	secondValues := layerer.inTopologicalOrder[second]
	firstIndex, secondIndex := len(firstValues), len(secondValues)
	for firstIndex > 0 && secondIndex > 0 {
		firstIndex--
		secondIndex--
		if firstValues[firstIndex] < secondValues[secondIndex] {
			return -1
		}
		if firstValues[firstIndex] > secondValues[secondIndex] {
			return 1
		}
	}
	// ELK tests hasNext() after traversing each ListIterator backwards.
	// Thus a non-empty iterator reports true after at least one previous() call,
	// even when it is exhausted in the backwards direction. Preserve that
	// seemingly unusual source (and elkjs) behavior exactly. ELK 0.12.0 adds
	// node ID as the deterministic final criterion when both lists are empty.
	firstHasNext := firstIndex < len(firstValues)
	secondHasNext := secondIndex < len(secondValues)
	if !firstHasNext && !secondHasNext {
		firstID, secondID := arena.Node(first).ID, arena.Node(second).ID
		if firstID < secondID {
			return -1
		}
		if firstID > secondID {
			return 1
		}
		return 0
	}
	if firstHasNext {
		return 1
	}
	return -1
}

func (layerer *CoffmanGrahamLayerer) transitiveReduction(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) {
	for _, start := range arena.Graph(graphID).LayerlessNodes {
		clear(layerer.nodeMark)
		for _, edgeID := range arena.NodeOutgoingEdges(start) {
			layerer.transitiveDFS(arena, start, arena.EdgeTargetNode(edgeID))
		}
	}
}

func (layerer *CoffmanGrahamLayerer) transitiveDFS(
	arena *layered.LGraphArena,
	start, nodeID layered.LNodeID,
) {
	index := arena.Node(nodeID).ID
	if layerer.nodeMark[index] {
		return
	}
	for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
		target := arena.EdgeTargetNode(edgeID)
		for _, candidate := range arena.NodeIncomingEdges(target) {
			if arena.EdgeSourceNode(candidate) == start {
				layerer.edgeMark[arena.Edge(candidate).ID] = true
			}
		}
		layerer.transitiveDFS(arena, start, target)
	}
	layerer.nodeMark[index] = true
}

func (layerer *CoffmanGrahamLayerer) canAdd(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	layerID layered.LayerID,
) bool {
	for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
		targetLayer := arena.Node(arena.EdgeTargetNode(edgeID)).Layer
		if targetLayer != nil && *targetLayer == layerID {
			return false
		}
	}
	return true
}
