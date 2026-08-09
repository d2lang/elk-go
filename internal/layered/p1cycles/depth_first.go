// Copyright (c) 2017, 2020 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p1cycles

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// DepthFirstCycleBreaker is the direct ELK 0.8.1 depth-first cycle breaker.
// Edges encountered while their target is active on the DFS path are reversed
// after traversal, preserving Java's edge-iteration and reversal order.
type DepthFirstCycleBreaker struct{}

func (*DepthFirstCycleBreaker) ProcessorConfiguration(
	_ *layered.LGraphArena,
	_ layered.LGraphID,
) layered.ProcessorConfiguration {
	return layered.GreedyCycleConfiguration()
}

func (*DepthFirstCycleBreaker) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	nodes := arena.Graph(graphID).LayerlessNodes
	visited := make([]bool, len(nodes))
	active := make([]bool, len(nodes))
	sources := make([]layered.LNodeID, 0)
	reverse := make([]layered.LEdgeID, 0)

	for index, nodeID := range nodes {
		arena.Node(nodeID).ID = index
		hasIncoming := false
		for _, edgeID := range arena.NodeIncomingEdges(nodeID) {
			if !arena.EdgeIsSelfLoop(edgeID) {
				hasIncoming = true
				break
			}
		}
		if !hasIncoming {
			sources = append(sources, nodeID)
		}
	}

	var visit func(layered.LNodeID)
	visit = func(nodeID layered.LNodeID) {
		index := arena.Node(nodeID).ID
		if visited[index] {
			return
		}
		visited[index] = true
		active[index] = true
		for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
			if arena.EdgeIsSelfLoop(edgeID) {
				continue
			}
			target := arena.EdgeTargetNode(edgeID)
			if active[arena.Node(target).ID] {
				reverse = append(reverse, edgeID)
			} else {
				visit(target)
			}
		}
		active[index] = false
	}

	for _, source := range sources {
		visit(source)
	}
	for index, nodeID := range nodes {
		if !visited[index] {
			visit(nodeID)
		}
	}
	for _, edgeID := range reverse {
		arena.ReverseEdge(graphID, edgeID, true)
		basegraph.Set(&arena.Graph(graphID).Properties, layered.Cyclic, true)
	}
	return nil
}
