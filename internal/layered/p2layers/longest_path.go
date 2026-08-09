// Copyright (c) 2010, 2020 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p2layers

import "github.com/d2lang/elk-go/internal/layered"

// LongestPathLayerer assigns each node according to its longest path to a
// sink. Layers are prepended exactly as in ELK 0.8.1.
type LongestPathLayerer struct{}

func (*LongestPathLayerer) ProcessorConfiguration(
	_ *layered.LGraphArena,
	_ layered.LGraphID,
) layered.ProcessorConfiguration {
	return layered.NetworkSimplexLayeringConfiguration()
}

func (*LongestPathLayerer) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	graph := arena.Graph(graphID)
	nodes := graph.LayerlessNodes
	heights := make([]int, len(nodes))
	for index, nodeID := range nodes {
		arena.Node(nodeID).ID = index
		heights[index] = -1
	}

	var visit func(layered.LNodeID) int
	visit = func(nodeID layered.LNodeID) int {
		index := arena.Node(nodeID).ID
		if heights[index] >= 0 {
			return heights[index]
		}
		maximum := 1
		for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
			target := arena.EdgeTargetNode(edgeID)
			if target == nodeID {
				continue
			}
			height := visit(target) + 1
			if height > maximum {
				maximum = height
			}
		}
		for len(graph.Layers) < maximum {
			newLayer := arena.AddLayer(graphID)
			graph.Layers = append(graph.Layers, 0)
			copy(graph.Layers[1:], graph.Layers[:len(graph.Layers)-1])
			graph.Layers[0] = newLayer
		}
		layerID := graph.Layers[len(graph.Layers)-maximum]
		arena.SetNodeLayer(nodeID, &layerID)
		heights[index] = maximum
		return maximum
	}

	for _, nodeID := range nodes {
		visit(nodeID)
	}
	graph.LayerlessNodes = graph.LayerlessNodes[:0]
	return nil
}
