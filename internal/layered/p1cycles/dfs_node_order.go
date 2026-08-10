// Copyright (c) 2025 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p1cycles

import (
	"math"
	"sort"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// DFSNodeOrderCycleBreaker ports ELK 0.12.0's depth-first cycle breaker that
// visits outgoing targets in node model order (or enforced group-model order)
// instead of raw edge order.
type DFSNodeOrderCycleBreaker struct{}

func (*DFSNodeOrderCycleBreaker) ProcessorConfiguration(
	_ *layered.LGraphArena,
	_ layered.LGraphID,
) layered.ProcessorConfiguration {
	return layered.GreedyCycleConfiguration()
}

func (*DFSNodeOrderCycleBreaker) Process(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) error {
	graph := arena.Graph(graphID)
	nodes := graph.LayerlessNodes
	visited := make([]bool, len(nodes))
	active := make([]bool, len(nodes))
	sources := make([]layered.LNodeID, 0)
	reverse := make([]layered.LEdgeID, 0)
	for index, nodeID := range nodes {
		arena.Node(nodeID).ID = index
		if len(arena.NodeIncomingEdges(nodeID)) == 0 {
			sources = append(sources, nodeID)
		}
	}

	groupModelOrder := basegraph.Get(
		&graph.Properties, layered.CycleBreakingGroupOrder,
	) == layered.GroupOrderEnforced
	maximumModelOrder := basegraph.Get(&graph.Properties, layered.MaximumModelOrderNodes)

	var visit func(layered.LNodeID)
	visit = func(nodeID layered.LNodeID) {
		index := arena.Node(nodeID).ID
		if visited[index] {
			return
		}
		visited[index] = true
		active[index] = true

		edgesByOrder := make(map[int][]layered.LEdgeID)
		for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
			target := arena.EdgeTargetNode(edgeID)
			modelOrder, found := basegraph.TryGet(
				&arena.Node(target).Properties, layered.ModelOrder)
			if !found {
				// Match the Java map-size-derived unique high keys: the first
				// target without model order is visited last.
				key := math.MaxInt - len(edgesByOrder)
				edgesByOrder[key] = []layered.LEdgeID{edgeID}
				continue
			}
			if groupModelOrder {
				group := basegraph.Get(
					&arena.Node(target).Properties, layered.CycleBreakingGroupID)
				modelOrder = maximumModelOrder*group + modelOrder
			}
			edgesByOrder[modelOrder] = append(edgesByOrder[modelOrder], edgeID)
		}
		orders := make([]int, 0, len(edgesByOrder))
		for order := range edgesByOrder {
			orders = append(orders, order)
		}
		sort.Ints(orders)
		for _, order := range orders {
			edges := edgesByOrder[order]
			edgeID := edges[0]
			if arena.EdgeIsSelfLoop(edgeID) {
				continue
			}
			target := arena.EdgeTargetNode(edgeID)
			if active[arena.Node(target).ID] {
				reverse = append(reverse, edges...)
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
