// Copyright (c) 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
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

	neighborBacking []Neighbor
}

func BuildNeighborhoodInformation(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) *NeighborhoodInformation {
	info := &NeighborhoodInformation{}
	buildNeighborhoodInformation(arena, graphID, info)
	return info
}

// buildNeighborhoodInformation fills info, reusing its slices' storage. Every
// entry of every slice is assigned, so the result equals a fresh one.
func buildNeighborhoodInformation(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	info *NeighborhoodInformation,
) {
	layers := arena.Graph(graphID).Layers
	nodeCount := 0
	for _, layerID := range layers {
		nodeCount += len(arena.Layer(layerID).Nodes)
	}
	info.NodeCount = nodeCount
	info.LayerIndex = resizeInts(info.LayerIndex, len(layers))
	info.NodeIndex = resizeInts(info.NodeIndex, nodeCount)
	info.LeftNeighbors = resizeNeighborLists(info.LeftNeighbors, nodeCount)
	info.RightNeighbors = resizeNeighborLists(info.RightNeighbors, nodeCount)
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
	// All neighbor lists are cut from one growing array; each list is final
	// once cut and has its capacity capped at its length.
	backing := info.neighborBacking[:0]
	for _, layerID := range layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			nodeIndex := arena.Node(nodeID).ID
			var list []Neighbor
			list, backing = info.neighbors(arena, nodeID, true, backing)
			info.LeftNeighbors[nodeIndex] = list
			list, backing = info.neighbors(arena, nodeID, false, backing)
			info.RightNeighbors[nodeIndex] = list
		}
	}
	info.neighborBacking = backing
}

func resizeInts(values []int, n int) []int {
	if cap(values) < n {
		return make([]int, n)
	}
	return values[:n]
}

func resizeNeighborLists(values [][]Neighbor, n int) [][]Neighbor {
	if cap(values) < n {
		return make([][]Neighbor, n)
	}
	return values[:n]
}

func (info *NeighborhoodInformation) Cleanup() {
	info.LayerIndex = nil
	info.NodeIndex = nil
	info.LeftNeighbors = nil
	info.RightNeighbors = nil
	info.neighborBacking = nil
}

// neighbors appends nodeID's neighbor list to backing and returns it (as a
// capacity-capped slice) together with the grown backing array.
func (info *NeighborhoodInformation) neighbors(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	left bool,
	backing []Neighbor,
) ([]Neighbor, []Neighbor) {
	start := len(backing)
	maxPriority := 0
	// NodeIncomingEdges / NodeOutgoingEdges order, without the copy.
	for _, portID := range arena.Node(nodeID).Ports {
		edges := arena.Port(portID).OutgoingEdges
		if left {
			edges = arena.Port(portID).IncomingEdges
		}
		for _, edgeID := range edges {
			if arena.EdgeIsSelfLoop(edgeID) || arena.EdgeIsInLayer(edgeID) {
				continue
			}
			priority := basegraph.Get(&arena.Edge(edgeID).Properties, layered.PriorityStraightness)
			if priority > maxPriority {
				maxPriority = priority
				backing = backing[:start]
			}
			if priority == maxPriority {
				neighbor := arena.EdgeTargetNode(edgeID)
				if left {
					neighbor = arena.EdgeSourceNode(edgeID)
				}
				backing = append(backing, Neighbor{Node: neighbor, Edge: edgeID})
			}
		}
	}
	result := backing[start:len(backing):len(backing)]
	javacompat.GoStableSort(result, func(first, second Neighbor) bool {
		return info.NodeIndex[arena.Node(first.Node).ID] < info.NodeIndex[arena.Node(second.Node).ID]
	})
	return result, backing
}
