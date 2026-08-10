// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package p1cycles

import (
	"reflect"
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestSCCNodeTypePreferredGroupsSelectSourceOrTargetEdges(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	nodes := make([]layered.LNodeID, 3)
	for index, group := range []int{1, 2, 5} {
		nodes[index] = arena.AddNode(graphID)
		arena.Graph(graphID).LayerlessNodes = append(
			arena.Graph(graphID).LayerlessNodes, nodes[index])
		basegraph.Set(&arena.Node(nodes[index]).Properties, layered.ModelOrder, index)
		basegraph.Set(&arena.Node(nodes[index]).Properties, layered.CycleBreakingGroupID, group)
	}
	addEdge := func(sourceIndex, targetIndex int) layered.LEdgeID {
		sourcePort, targetPort := arena.AddPort(), arena.AddPort()
		arena.SetPortNode(sourcePort, &nodes[sourceIndex])
		arena.SetPortNode(targetPort, &nodes[targetIndex])
		edgeID := arena.AddEdge()
		arena.SetEdgeSource(edgeID, &sourcePort)
		arena.SetEdgeTarget(edgeID, &targetPort)
		return edgeID
	}
	_ = addEdge(0, 1)
	_ = addEdge(1, 2)
	cycleEdge := addEdge(2, 0)
	secondTargetEdge := addEdge(2, 1)
	component := []layered.LNodeID{nodes[0], nodes[1], nodes[2]}

	basegraph.Set(
		&arena.Graph(graphID).Properties,
		layered.CycleBreakingPreferredSourceID,
		1,
	)
	if got, want := findSCCNodeTypeReversals(arena, graphID, component, 3, 9),
		[]layered.LEdgeID{cycleEdge}; !reflect.DeepEqual(got, want) {
		t.Fatalf("preferred-source reversals = %v, want %v", got, want)
	}

	basegraph.Unset(
		&arena.Graph(graphID).Properties,
		layered.CycleBreakingPreferredSourceID,
	)
	basegraph.Set(
		&arena.Graph(graphID).Properties,
		layered.CycleBreakingPreferredTargetID,
		5,
	)
	if got, want := findSCCNodeTypeReversals(arena, graphID, component, 3, 9),
		[]layered.LEdgeID{cycleEdge, secondTargetEdge}; !reflect.DeepEqual(got, want) {
		t.Fatalf("preferred-target reversals = %v, want %v", got, want)
	}
}

func TestSCCNodeTypeCycleBreakerProducesAcyclicGraph(t *testing.T) {
	arena, graphID := alternativeCycleGraph()
	basegraph.Set(
		&arena.Graph(graphID).Properties,
		layered.CycleBreakingPreferredSourceID,
		0,
	)
	if err := (&SCCNodeTypeCycleBreaker{}).Process(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if hasCycle(arena, graphID) {
		t.Fatal("cycle remained after SCC node-type cycle breaking")
	}
	if !basegraph.Get(&arena.Graph(graphID).Properties, layered.Cyclic) {
		t.Fatal("cycle flag was not set")
	}
}
