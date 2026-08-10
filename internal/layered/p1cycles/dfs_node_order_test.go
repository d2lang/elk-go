// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package p1cycles

import (
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestDFSNodeOrderCycleBreakerUsesEnforcedGroupOrder(t *testing.T) {
	tests := []struct {
		name       string
		strategy   layered.GroupOrderStrategy
		reversedID int
	}{
		{"within-groups", layered.GroupOrderOnlyWithinGroup, 3},
		{"enforced-groups", layered.GroupOrderEnforced, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			arena, graphID := dfsNodeOrderTestGraph()
			basegraph.Set(&arena.Graph(graphID).Properties, layered.CycleBreakingGroupOrder, test.strategy)
			if err := (&DFSNodeOrderCycleBreaker{}).Process(arena, graphID); err != nil {
				t.Fatal(err)
			}
			for edgeIndex := 0; edgeIndex < 4; edgeIndex++ {
				got := basegraph.Get(&arena.Edge(layered.LEdgeID(edgeIndex)).Properties, layered.Reversed)
				if got != (edgeIndex == test.reversedID) {
					t.Fatalf("edge %d reversed = %v, want %v", edgeIndex, got, edgeIndex == test.reversedID)
				}
			}
		})
	}
}

func dfsNodeOrderTestGraph() (*layered.LGraphArena, layered.LGraphID) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	nodes := make([]layered.LNodeID, 4)
	groups := []int{4, 3, 2, 1}
	for index := range nodes {
		nodes[index] = arena.AddNode(graphID)
		arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, nodes[index])
		basegraph.Set(&arena.Node(nodes[index]).Properties, layered.ModelOrder, index)
		basegraph.Set(&arena.Node(nodes[index]).Properties, layered.CycleBreakingGroupID, groups[index])
	}
	basegraph.Set(&arena.Graph(graphID).Properties, layered.MaximumModelOrderNodes, len(nodes))
	for _, endpoints := range [][2]int{{0, 2}, {1, 0}, {1, 2}, {2, 0}} {
		source, target := arena.AddPort(), arena.AddPort()
		arena.SetPortNode(source, &nodes[endpoints[0]])
		arena.SetPortNode(target, &nodes[endpoints[1]])
		edgeID := arena.AddEdge()
		arena.SetEdgeSource(edgeID, &source)
		arena.SetEdgeTarget(edgeID, &target)
	}
	return arena, graphID
}
