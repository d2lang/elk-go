// Copyright (c) 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"sort"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

type Neighbor struct {
	Node layered.LNodeID
	Edge layered.LEdgeID
}

// NeighborhoodInformation caches the immutable neighbor relation used by all
// four Brandes-Koepf directional layouts. Arrays are indexed by the scratch
// IDs assigned in BuildNeighborhoodInformation.
type NeighborhoodInformation struct {
	NodeCount      int
	LayerIndex     []int
	NodeIndex      []int
	LeftNeighbors  [][]Neighbor
	RightNeighbors [][]Neighbor
}

func BuildNeighborhoodInformation(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) *NeighborhoodInformation {
	layers := append([]layered.LayerID(nil), arena.Graph(graphID).Layers...)
	nodeCount := 0
	for _, layerID := range layers {
		nodeCount += len(arena.Layer(layerID).Nodes)
	}
	info := &NeighborhoodInformation{
		NodeCount:      nodeCount,
		LayerIndex:     make([]int, len(layers)),
		NodeIndex:      make([]int, nodeCount),
		LeftNeighbors:  make([][]Neighbor, nodeCount),
		RightNeighbors: make([][]Neighbor, nodeCount),
	}
	nodeScratchID := 0
	for layerIndex, layerID := range layers {
		arena.Layer(layerID).ID = layerIndex
		info.LayerIndex[layerIndex] = layerIndex
		for nodeIndex, nodeID := range arena.Layer(layerID).Nodes {
			arena.Node(nodeID).ID = nodeScratchID
			info.NodeIndex[nodeScratchID] = nodeIndex
			nodeScratchID++
		}
	}
	for _, layerID := range layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			nodeIndex := arena.Node(nodeID).ID
			info.LeftNeighbors[nodeIndex] = info.neighbors(arena, nodeID, true)
			info.RightNeighbors[nodeIndex] = info.neighbors(arena, nodeID, false)
		}
	}
	return info
}

func (info *NeighborhoodInformation) Cleanup() {
	info.LayerIndex = nil
	info.NodeIndex = nil
	info.LeftNeighbors = nil
	info.RightNeighbors = nil
}

func (info *NeighborhoodInformation) neighbors(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	left bool,
) []Neighbor {
	result := make([]Neighbor, 0)
	maxPriority := 0
	edges := arena.NodeOutgoingEdges(nodeID)
	if left {
		edges = arena.NodeIncomingEdges(nodeID)
	}
	for _, edgeID := range edges {
		if arena.EdgeIsSelfLoop(edgeID) || arena.EdgeIsInLayer(edgeID) {
			continue
		}
		priority := basegraph.Get(&arena.Edge(edgeID).Properties, layered.PriorityStraightness)
		if priority > maxPriority {
			maxPriority = priority
			result = result[:0]
		}
		if priority == maxPriority {
			neighbor := arena.EdgeTargetNode(edgeID)
			if left {
				neighbor = arena.EdgeSourceNode(edgeID)
			}
			result = append(result, Neighbor{Node: neighbor, Edge: edgeID})
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return info.NodeIndex[arena.Node(result[i].Node).ID] < info.NodeIndex[arena.Node(result[j].Node).ID]
	})
	return result
}
