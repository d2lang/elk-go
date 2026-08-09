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
	"math"
	"sort"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// StretchWidthLayerer is ELK 0.8.1's minimum-width heuristic. It builds the
// layering bottom-up and increases its width bound only when a layer cannot
// accept any eligible node.
type StretchWidthLayerer struct{}

func (*StretchWidthLayerer) ProcessorConfiguration(
	_ *layered.LGraphArena,
	_ layered.LGraphID,
) layered.ProcessorConfiguration {
	return layered.NetworkSimplexLayeringConfiguration()
}

func (*StretchWidthLayerer) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	graph := arena.Graph(graphID)
	if len(graph.LayerlessNodes) == 0 {
		return nil
	}

	minimumNodeSize, maximumNodeSize := math.Inf(1), math.Inf(-1)
	dummySize := basegraph.Get(&graph.Properties, layered.SpacingEdgeEdge)
	sortedNodes := append([]layered.LNodeID(nil), graph.LayerlessNodes...)
	for _, nodeID := range graph.LayerlessNodes {
		arena.Node(nodeID).ID = stretchWidthRank(arena, nodeID)
	}
	sort.SliceStable(sortedNodes, func(first, second int) bool {
		return arena.Node(sortedNodes[first]).ID > arena.Node(sortedNodes[second]).ID
	})
	for index, nodeID := range sortedNodes {
		arena.Node(nodeID).ID = index
	}

	inDegree := make([]int, len(sortedNodes))
	outDegree := make([]int, len(sortedNodes))
	for _, nodeID := range sortedNodes {
		index := arena.Node(nodeID).ID
		inDegree[index] = len(arena.NodeIncomingEdges(nodeID))
		outDegree[index] = len(arena.NodeOutgoingEdges(nodeID))
		if arena.Node(nodeID).Type == layered.NormalNode {
			minimumNodeSize = math.Min(minimumNodeSize, arena.Node(nodeID).Size.Y)
			maximumNodeSize = math.Max(maximumNodeSize, arena.Node(nodeID).Size.Y)
		}
	}
	normalizedSize := make([]float64, len(sortedNodes))
	for _, nodeID := range sortedNodes {
		normalizedSize[arena.Node(nodeID).ID] = arena.Node(nodeID).Size.Y / minimumNodeSize
	}
	minimumNodeSize = math.Max(1, minimumNodeSize)
	maximumNodeSize = math.Max(1, maximumNodeSize)
	dummySize /= minimumNodeSize
	maximumWidth := maximumNodeSize / minimumNodeSize
	allOutgoing := 0.0
	for _, nodeID := range graph.LayerlessNodes {
		allOutgoing += float64(len(arena.NodeOutgoingEdges(nodeID)))
	}
	upperLayerInfluence := allOutgoing / float64(len(graph.LayerlessNodes))

	currentLayer := arena.AddLayer(graphID)
	graph.Layers = append(graph.Layers, currentLayer)
	temporaryNodes := append([]layered.LNodeID(nil), sortedNodes...)
	remainingOutgoing := append([]int(nil), outDegree...)
	alreadyPlaced := make(map[layered.LNodeID]struct{})
	alreadyPlacedOtherLayers := make(map[layered.LNodeID]struct{})
	widthCurrent, widthUp := 0.0, 0.0

	conditionGoUp := func(nodeID layered.LNodeID) bool {
		index := arena.Node(nodeID).ID
		currentTooWide := widthCurrent-float64(outDegree[index])*dummySize+normalizedSize[index] > maximumWidth
		upperTooWide := widthUp+float64(inDegree[index])*dummySize > maximumWidth*upperLayerInfluence*dummySize
		return currentTooWide || upperTooWide
	}
	for len(temporaryNodes) > 0 {
		selectedIndex := -1
		for index, nodeID := range temporaryNodes {
			if remainingOutgoing[arena.Node(nodeID).ID] <= 0 {
				selectedIndex = index
				break
			}
		}
		if selectedIndex < 0 || conditionGoUp(temporaryNodes[selectedIndex]) && len(alreadyPlaced) > 0 {
			for _, nodeID := range arena.Layer(currentLayer).Nodes {
				for _, edgeID := range arena.NodeIncomingEdges(nodeID) {
					source := arena.EdgeSourceNode(edgeID)
					remainingOutgoing[arena.Node(source).ID]--
				}
			}
			currentLayer = arena.AddLayer(graphID)
			graph.Layers = append(graph.Layers, currentLayer)
			for nodeID := range alreadyPlaced {
				alreadyPlacedOtherLayers[nodeID] = struct{}{}
			}
			clear(alreadyPlaced)
			widthCurrent, widthUp = widthUp, 0
			continue
		}

		selected := temporaryNodes[selectedIndex]
		if conditionGoUp(selected) {
			graph.Layers = graph.Layers[:0]
			currentLayer = arena.AddLayer(graphID)
			graph.Layers = append(graph.Layers, currentLayer)
			widthCurrent, widthUp = 0, 0
			clear(alreadyPlaced)
			clear(alreadyPlacedOtherLayers)
			maximumWidth++
			temporaryNodes = append(temporaryNodes[:0], sortedNodes...)
			copy(remainingOutgoing, outDegree)
			continue
		}

		arena.SetNodeLayer(selected, &currentLayer)
		temporaryNodes = append(temporaryNodes[:selectedIndex], temporaryNodes[selectedIndex+1:]...)
		alreadyPlaced[selected] = struct{}{}
		index := arena.Node(selected).ID
		widthCurrent = widthCurrent - float64(outDegree[index])*dummySize + normalizedSize[index]
		widthUp += float64(inDegree[index]) * dummySize
	}

	graph.LayerlessNodes = graph.LayerlessNodes[:0]
	for left, right := 0, len(graph.Layers)-1; left < right; left, right = left+1, right-1 {
		graph.Layers[left], graph.Layers[right] = graph.Layers[right], graph.Layers[left]
	}
	return nil
}

func stretchWidthRank(arena *layered.LGraphArena, nodeID layered.LNodeID) int {
	maximum := len(arena.NodeOutgoingEdges(nodeID))
	for _, edgeID := range arena.NodeIncomingEdges(nodeID) {
		predecessor := arena.EdgeSourceNode(edgeID)
		if degree := len(arena.NodeOutgoingEdges(predecessor)); degree > maximum {
			maximum = degree
		}
	}
	return maximum
}
