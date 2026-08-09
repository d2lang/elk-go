// Copyright (c) 2010, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package p2layers implements ELK Layered phase 2.
package p2layers

import (
	"math"

	"github.com/d2lang/elk-go/internal/common/networksimplex"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

const iterationLimitFactor = 4

type NetworkSimplexLayerer struct{}

func (*NetworkSimplexLayerer) ProcessorConfiguration(
	_ *layered.LGraphArena,
	_ layered.LGraphID,
) layered.ProcessorConfiguration {
	return layered.NetworkSimplexLayeringConfiguration()
}

func (*NetworkSimplexLayerer) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	graph := arena.Graph(graphID)
	nodes := graph.LayerlessNodes
	if len(nodes) == 0 {
		return nil
	}
	components := connectedComponents(arena, nodes)
	thoroughness := basegraph.Get(&graph.Properties, layered.Thoroughness) * iterationLimitFactor
	var previousLayeringNodeCounts []int

	for _, component := range components {
		componentSet := make(map[layered.LNodeID]struct{}, len(component))
		for _, nodeID := range component {
			componentSet[nodeID] = struct{}{}
		}
		outgoing := func(nodeID layered.LNodeID) []layered.LEdgeID {
			edges := make([]layered.LEdgeID, 0)
			for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
				if arena.EdgeIsSelfLoop(edgeID) {
					continue
				}
				if _, inside := componentSet[arena.EdgeTargetNode(edgeID)]; inside {
					edges = append(edges, edgeID)
				}
			}
			return edges
		}
		iterationLimit := thoroughness * int(math.Sqrt(float64(len(component))))
		layers := networksimplex.Layer(networksimplex.Adapter[layered.LNodeID, layered.LEdgeID]{
			Nodes:    component,
			Outgoing: outgoing,
			Source:   arena.EdgeSourceNode,
			Target:   arena.EdgeTargetNode,
			Weight: func(edgeID layered.LEdgeID) float64 {
				priority := basegraph.Get(&arena.Edge(edgeID).Properties, layered.PriorityShortness)
				if priority < 1 {
					priority = 1
				}
				return float64(priority)
			},
			MinLength: func(layered.LEdgeID) int { return 1 },
		}, networksimplex.Options{
			Balance:                    true,
			PreviousLayeringNodeCounts: previousLayeringNodeCounts,
			IterationLimit:             iterationLimit,
		})

		for _, nodeID := range component {
			layerIndex := layers[nodeID]
			for len(graph.Layers) <= layerIndex {
				arena.AddLayerToGraph(graphID)
			}
			layerID := graph.Layers[layerIndex]
			arena.SetNodeLayer(nodeID, &layerID)
		}
		if len(components) > 1 {
			previousLayeringNodeCounts = make([]int, len(graph.Layers))
			for index, layerID := range graph.Layers {
				previousLayeringNodeCounts[index] = len(arena.Layer(layerID).Nodes)
			}
		}
	}
	graph.LayerlessNodes = graph.LayerlessNodes[:0]
	return nil
}

func connectedComponents(arena *layered.LGraphArena, nodes []layered.LNodeID) [][]layered.LNodeID {
	for index, nodeID := range nodes {
		arena.Node(nodeID).ID = index
	}
	visited := make([]bool, len(nodes))
	components := make([][]layered.LNodeID, 0)
	var visit func(layered.LNodeID, *[]layered.LNodeID)
	visit = func(nodeID layered.LNodeID, component *[]layered.LNodeID) {
		index := arena.Node(nodeID).ID
		if visited[index] {
			return
		}
		visited[index] = true
		*component = append(*component, nodeID)
		for _, portID := range arena.Node(nodeID).Ports {
			for _, edgeID := range arena.PortConnectedEdges(portID) {
				opposite := arena.EdgeOtherNode(edgeID, nodeID)
				if !visited[arena.Node(opposite).ID] {
					visit(opposite, component)
				}
			}
		}
	}
	for _, nodeID := range nodes {
		if visited[arena.Node(nodeID).ID] {
			continue
		}
		component := make([]layered.LNodeID, 0)
		visit(nodeID, &component)
		if len(components) == 0 || len(components[0]) < len(component) {
			components = append(components, nil)
			copy(components[1:], components[:len(components)-1])
			components[0] = component
		} else {
			components = append(components, component)
		}
	}
	return components
}
