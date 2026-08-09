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

type minWidthResult struct {
	maximumWidth float64
	layers       [][]layered.LNodeID
}

// MinWidthLayerer is ELK 0.8.1's size-aware minimum-width heuristic.
type MinWidthLayerer struct {
	dummySize      float64
	normalizedSize []float64
	averageSize    float64
	inDegree       []int
	outDegree      []int
}

func (*MinWidthLayerer) ProcessorConfiguration(
	_ *layered.LGraphArena,
	_ layered.LGraphID,
) layered.ProcessorConfiguration {
	return layered.NetworkSimplexLayeringConfiguration()
}

func (layerer *MinWidthLayerer) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	graph := arena.Graph(graphID)
	nodes := graph.LayerlessNodes
	if len(nodes) == 0 {
		return nil
	}
	upperBound := basegraph.Get(&graph.Properties, layered.MinWidthUpperBound)
	compensator := basegraph.Get(&graph.Properties, layered.MinWidthUpperLayerScaling)
	layerer.dummySize = basegraph.Get(&graph.Properties, layered.SpacingEdgeEdge)
	minimumNodeSize := math.Inf(1)
	for _, nodeID := range nodes {
		if node := arena.Node(nodeID); node.Type == layered.NormalNode {
			minimumNodeSize = math.Min(minimumNodeSize, node.Size.Y)
		}
	}
	minimumNodeSize = math.Max(1, minimumNodeSize)
	layerer.inDegree = make([]int, len(nodes))
	layerer.outDegree = make([]int, len(nodes))
	layerer.normalizedSize = make([]float64, len(nodes))
	layerer.averageSize = 0
	for index, nodeID := range nodes {
		node := arena.Node(nodeID)
		node.ID = index
		layerer.inDegree[index] = minWidthDegree(arena, arena.NodeIncomingEdges(nodeID))
		layerer.outDegree[index] = minWidthDegree(arena, arena.NodeOutgoingEdges(nodeID))
		layerer.normalizedSize[index] = node.Size.Y / minimumNodeSize
		layerer.averageSize += layerer.normalizedSize[index]
	}
	layerer.dummySize /= minimumNodeSize
	layerer.averageSize /= float64(len(nodes))

	successors := make([]map[layered.LNodeID]struct{}, len(nodes))
	for _, nodeID := range nodes {
		set := make(map[layered.LNodeID]struct{})
		for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
			if !arena.EdgeIsSelfLoop(edgeID) {
				set[arena.EdgeTargetNode(edgeID)] = struct{}{}
			}
		}
		successors[arena.Node(nodeID).ID] = set
	}
	sort.SliceStable(nodes, func(first, second int) bool {
		return layerer.outDegree[arena.Node(nodes[first]).ID] > layerer.outDegree[arena.Node(nodes[second]).ID]
	})

	upperStart, upperEnd := upperBound, upperBound
	compensatorStart, compensatorEnd := compensator, compensator
	if upperBound < 0 {
		upperStart, upperEnd = 1, 4
	}
	if compensator < 0 {
		compensatorStart, compensatorEnd = 1, 2
	}
	minimumWidth := math.Inf(1)
	minimumLayers := int(^uint32(0) >> 1)
	var candidate [][]layered.LNodeID
	for bound := upperStart; bound <= upperEnd; bound++ {
		for scale := compensatorStart; scale <= compensatorEnd; scale++ {
			result := layerer.compute(arena, bound, scale, nodes, successors)
			if result.maximumWidth < minimumWidth ||
				result.maximumWidth == minimumWidth && len(result.layers) < minimumLayers {
				minimumWidth = result.maximumWidth
				minimumLayers = len(result.layers)
				candidate = result.layers
			}
		}
	}
	for _, nodeLayer := range candidate {
		layerID := arena.AddLayer(graphID)
		for _, nodeID := range nodeLayer {
			arena.SetNodeLayer(nodeID, &layerID)
		}
		graph.Layers = append(graph.Layers, layerID)
	}
	for left, right := 0, len(graph.Layers)-1; left < right; left, right = left+1, right-1 {
		graph.Layers[left], graph.Layers[right] = graph.Layers[right], graph.Layers[left]
	}
	graph.LayerlessNodes = graph.LayerlessNodes[:0]
	return nil
}

func (layerer *MinWidthLayerer) compute(
	arena *layered.LGraphArena,
	upperBound, compensator int,
	nodes []layered.LNodeID,
	successors []map[layered.LNodeID]struct{},
) minWidthResult {
	unplaced := append([]layered.LNodeID(nil), nodes...)
	alreadyCurrent := make(map[layered.LNodeID]struct{})
	alreadyOther := make(map[layered.LNodeID]struct{})
	currentLayer := make([]layered.LNodeID, 0)
	layers := make([][]layered.LNodeID, 0)
	boundWithSize := float64(upperBound) * layerer.averageSize
	widthCurrent, widthUp := 0.0, 0.0
	maximumWidth, realWidth := 0.0, 0.0
	currentSpanningEdges, goingOut := 0.0, 0.0

	for len(unplaced) > 0 {
		selectedIndex := -1
		for index, nodeID := range unplaced {
			allPlaced := true
			for successor := range successors[arena.Node(nodeID).ID] {
				if _, found := alreadyOther[successor]; !found {
					allPlaced = false
					break
				}
			}
			if allPlaced {
				selectedIndex = index
				break
			}
		}
		var selected layered.LNodeID
		selectedFound := selectedIndex >= 0
		inDegree, outDegree := 0, 0
		if selectedFound {
			selected = unplaced[selectedIndex]
			unplaced = append(unplaced[:selectedIndex], unplaced[selectedIndex+1:]...)
			currentLayer = append(currentLayer, selected)
			alreadyCurrent[selected] = struct{}{}
			index := arena.Node(selected).ID
			outDegree = layerer.outDegree[index]
			widthCurrent += layerer.normalizedSize[index] - float64(outDegree)*layerer.dummySize
			inDegree = layerer.inDegree[index]
			widthUp += float64(inDegree) * layerer.dummySize
			goingOut += float64(outDegree) * layerer.dummySize
			realWidth += layerer.normalizedSize[index]
		}
		goUp := !selectedFound || len(unplaced) == 0
		if selectedFound {
			index := arena.Node(selected).ID
			goUp = goUp || widthCurrent >= boundWithSize &&
				layerer.normalizedSize[index] > float64(outDegree)*layerer.dummySize ||
				widthUp >= float64(compensator)*boundWithSize
		}
		if goUp {
			layers = append(layers, currentLayer)
			currentLayer = make([]layered.LNodeID, 0)
			for nodeID := range alreadyCurrent {
				alreadyOther[nodeID] = struct{}{}
			}
			clear(alreadyCurrent)
			currentSpanningEdges -= goingOut
			maximumWidth = math.Max(maximumWidth, currentSpanningEdges*layerer.dummySize+realWidth)
			currentSpanningEdges += widthUp
			widthCurrent, widthUp = widthUp, 0
			goingOut, realWidth = 0, 0
		}
	}
	return minWidthResult{maximumWidth: maximumWidth, layers: layers}
}

func minWidthDegree(arena *layered.LGraphArena, edges []layered.LEdgeID) int {
	degree := 0
	for _, edgeID := range edges {
		if !arena.EdgeIsSelfLoop(edgeID) {
			degree++
		}
	}
	return degree
}
