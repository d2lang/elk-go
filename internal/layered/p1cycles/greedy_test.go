// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package p1cycles

import (
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestGreedyModelOrderBreaksCycleDeterministically(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	nodes := []layered.LNodeID{arena.AddNode(graphID), arena.AddNode(graphID), arena.AddNode(graphID)}
	arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, nodes...)
	ports := make([]layered.LPortID, len(nodes))
	for index, nodeID := range nodes {
		basegraph.Set(&arena.Node(nodeID).Properties, layered.ModelOrder, index)
		ports[index] = arena.AddPort()
		arena.SetPortNode(ports[index], &nodeID)
	}
	for index := range nodes {
		edge := arena.AddEdge()
		source, target := ports[index], ports[(index+1)%len(nodes)]
		arena.SetEdgeSource(edge, &source)
		arena.SetEdgeTarget(edge, &target)
	}

	if err := NewGreedyModelOrderCycleBreaker().Process(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if !basegraph.Get(&arena.Graph(graphID).Properties, layered.Cyclic) {
		t.Fatal("cycle flag was not set")
	}
	if hasCycle(arena, graphID) {
		t.Fatal("cycle remained after greedy cycle breaking")
	}
}

func hasCycle(arena *layered.LGraphArena, graphID layered.LGraphID) bool {
	state := make(map[layered.LNodeID]uint8)
	var visit func(layered.LNodeID) bool
	visit = func(nodeID layered.LNodeID) bool {
		if state[nodeID] == 1 {
			return true
		}
		if state[nodeID] == 2 {
			return false
		}
		state[nodeID] = 1
		for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
			if arena.EdgeIsSelfLoop(edgeID) {
				continue
			}
			if visit(arena.EdgeTargetNode(edgeID)) {
				return true
			}
		}
		state[nodeID] = 2
		return false
	}
	for _, nodeID := range arena.Graph(graphID).LayerlessNodes {
		if visit(nodeID) {
			return true
		}
	}
	return false
}
