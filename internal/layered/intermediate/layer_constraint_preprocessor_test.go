// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestLayerConstraintPreprocessorHidesSeparateNodes(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	first := addLayerConstraintTestNode(arena, graphID)
	middle := addLayerConstraintTestNode(arena, graphID)
	last := addLayerConstraintTestNode(arena, graphID)
	basegraph.Set(&arena.Node(first).Properties, layered.LayerConstraintOption,
		layered.LayerConstraintFirstSeparate)
	basegraph.Set(&arena.Node(last).Properties, layered.LayerConstraintOption,
		layered.LayerConstraintLastSeparate)
	firstEdge := addLayerConstraintTestEdge(arena, first, middle)
	lastEdge := addLayerConstraintTestEdge(arena, middle, last)

	if err := ProcessLayerConstraintsBeforeLayering(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if got := arena.Graph(graphID).LayerlessNodes; len(got) != 1 || got[0] != middle {
		t.Fatalf("visible nodes = %v, want [%v]", got, middle)
	}
	hidden := basegraph.Get(&arena.Graph(graphID).Properties, layered.HiddenNodes)
	if len(hidden) != 2 || hidden[0] != first || hidden[1] != last {
		t.Fatalf("hidden nodes = %v, want [%v %v]", hidden, first, last)
	}
	if arena.Edge(firstEdge).Target != nil || arena.Edge(lastEdge).Source != nil {
		t.Fatalf("hidden endpoints = first target %v, last source %v",
			arena.Edge(firstEdge).Target, arena.Edge(lastEdge).Source)
	}
	if basegraph.Has(&arena.Node(middle).Properties, layered.LayerConstraintOption) {
		t.Fatalf("middle node received a constraint despite connections to both separate sides")
	}
}

func TestLayerConstraintPreprocessorConstrainsOrphanedNeighbor(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	normal := addLayerConstraintTestNode(arena, graphID)
	hidden := addLayerConstraintTestNode(arena, graphID)
	basegraph.Set(&arena.Node(hidden).Properties, layered.LayerConstraintOption,
		layered.LayerConstraintLastSeparate)
	addLayerConstraintTestEdge(arena, normal, hidden)

	if err := (&LayerConstraintPreprocessor{}).Process(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if got := basegraph.Get(&arena.Node(normal).Properties, layered.LayerConstraintOption); got != layered.LayerConstraintLast {
		t.Fatalf("orphaned neighbor constraint = %v, want LAST", got)
	}
}

func TestLayerConstraintPreprocessorRejectsWrongDirection(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	hidden := addLayerConstraintTestNode(arena, graphID)
	normal := addLayerConstraintTestNode(arena, graphID)
	basegraph.Set(&arena.Node(hidden).Properties, layered.LayerConstraintOption,
		layered.LayerConstraintFirstSeparate)
	addLayerConstraintTestEdge(arena, normal, hidden)

	if err := ProcessLayerConstraintsBeforeLayering(arena, graphID); err == nil {
		t.Fatal("FIRST_SEPARATE node with incoming edge was accepted")
	}
}

func addLayerConstraintTestNode(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) layered.LNodeID {
	nodeID := arena.AddNode(graphID)
	arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, nodeID)
	return nodeID
}

func addLayerConstraintTestEdge(
	arena *layered.LGraphArena,
	source, target layered.LNodeID,
) layered.LEdgeID {
	sourcePort := arena.AddPort()
	targetPort := arena.AddPort()
	arena.SetPortNode(sourcePort, &source)
	arena.SetPortNode(targetPort, &target)
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &sourcePort)
	arena.SetEdgeTarget(edgeID, &targetPort)
	return edgeID
}
