// Copyright (c) 2021 Kiel University and others.
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

// ModelOrderCycleBreaker reverses edges that run against the imported model
// order, including ELK's offsets for first/last layer constraints.
type ModelOrderCycleBreaker struct {
	firstSeparateModelOrder int
	lastSeparateModelOrder  int
}

func (*ModelOrderCycleBreaker) ProcessorConfiguration(
	_ *layered.LGraphArena,
	_ layered.LGraphID,
) layered.ProcessorConfiguration {
	return layered.GreedyCycleConfiguration()
}

func (breaker *ModelOrderCycleBreaker) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	breaker.firstSeparateModelOrder = 0
	breaker.lastSeparateModelOrder = 0
	nodes := arena.Graph(graphID).LayerlessNodes
	offset := len(nodes)
	for _, nodeID := range nodes {
		if order, found := basegraph.TryGet(&arena.Node(nodeID).Properties, layered.ModelOrder); found && order+1 > offset {
			offset = order + 1
		}
	}

	reverse := make([]layered.LEdgeID, 0)
	for _, source := range nodes {
		sourceOrder := breaker.constraintModelOrder(arena, source, offset)
		for _, edgeID := range arena.NodeOutgoingEdges(source) {
			target := arena.EdgeTargetNode(edgeID)
			if breaker.constraintModelOrder(arena, target, offset) < sourceOrder {
				reverse = append(reverse, edgeID)
			}
		}
	}
	for _, edgeID := range reverse {
		arena.ReverseEdge(graphID, edgeID, true)
		basegraph.Set(&arena.Graph(graphID).Properties, layered.Cyclic, true)
	}
	return nil
}

func (breaker *ModelOrderCycleBreaker) constraintModelOrder(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	offset int,
) int {
	node := arena.Node(nodeID)
	order := 0
	switch basegraph.Get(&node.Properties, layered.LayerConstraintOption) {
	case layered.LayerConstraintFirstSeparate:
		order = -2*offset + breaker.firstSeparateModelOrder
		breaker.firstSeparateModelOrder++
	case layered.LayerConstraintFirst:
		order = -offset
	case layered.LayerConstraintLast:
		order = offset
	case layered.LayerConstraintLastSeparate:
		order = 2*offset + breaker.lastSeparateModelOrder
		breaker.lastSeparateModelOrder++
	}
	if modelOrder, found := basegraph.TryGet(&node.Properties, layered.ModelOrder); found {
		order += modelOrder
	}
	return order
}
