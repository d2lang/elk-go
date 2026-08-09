// Copyright (c) 2011, 2015 Kiel University and others.
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

// InteractiveCycleBreaker first follows the direction suggested by existing
// x coordinates, then uses ELK's DFS pass to remove any remaining cycles.
type InteractiveCycleBreaker struct{}

func (*InteractiveCycleBreaker) ProcessorConfiguration(
	_ *layered.LGraphArena,
	_ layered.LGraphID,
) layered.ProcessorConfiguration {
	var configuration layered.ProcessorConfiguration
	configuration.
		AddBefore(layered.PhaseCycleBreaking, layered.InteractiveExternalPortPositioner).
		AddAfter(layered.PhaseEdgeRouting, layered.ReversedEdgeRestorer)
	return configuration
}

func (*InteractiveCycleBreaker) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	nodes := arena.Graph(graphID).LayerlessNodes
	reverse := make([]layered.LEdgeID, 0)
	for _, source := range nodes {
		arena.Node(source).ID = 1
		sourceX := interactiveReferencePoint(arena, source).X
		for _, edgeID := range arena.NodeOutgoingEdges(source) {
			target := arena.EdgeTargetNode(edgeID)
			if target != source && interactiveReferencePoint(arena, target).X < sourceX {
				reverse = append(reverse, edgeID)
			}
		}
	}
	for _, edgeID := range reverse {
		arena.ReverseEdge(graphID, edgeID, true)
	}

	reverse = reverse[:0]
	var findCycles func(layered.LNodeID)
	findCycles = func(nodeID layered.LNodeID) {
		arena.Node(nodeID).ID = -1
		for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
			target := arena.EdgeTargetNode(edgeID)
			if target == nodeID {
				continue
			}
			switch {
			case arena.Node(target).ID < 0:
				reverse = append(reverse, edgeID)
			case arena.Node(target).ID > 0:
				findCycles(target)
			}
		}
		arena.Node(nodeID).ID = 0
	}
	for _, nodeID := range nodes {
		if arena.Node(nodeID).ID > 0 {
			findCycles(nodeID)
		}
	}
	for _, edgeID := range reverse {
		arena.ReverseEdge(graphID, edgeID, true)
	}
	return nil
}

func interactiveReferencePoint(arena *layered.LGraphArena, nodeID layered.LNodeID) (point struct{ X, Y float64 }) {
	node := arena.Node(nodeID)
	point.X, point.Y = node.Position.X, node.Position.Y
	if basegraph.Get(&arena.Graph(arena.NodeGraph(nodeID)).Properties, layered.InteractiveReferencePointOption) == layered.InteractiveReferenceCenter {
		point.X += node.Size.X / 2
		point.Y += node.Size.Y / 2
	}
	return point
}
