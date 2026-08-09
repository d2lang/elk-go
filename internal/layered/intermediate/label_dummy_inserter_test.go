// Copyright (c) 2012, 2017 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestInsertLabelDummiesMovesOnlyCenterLabels(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	basegraph.Set(&arena.Graph(graphID).Properties, layered.DirectionOption, layered.DirectionDown)
	basegraph.Set(&arena.Graph(graphID).Properties, layered.SpacingEdgeLabel, 2.0)
	basegraph.Set(&arena.Graph(graphID).Properties, layered.SpacingLabelLabel, 3.0)
	source, target := arena.AddNode(graphID), arena.AddNode(graphID)
	arena.Graph(graphID).LayerlessNodes = []layered.LNodeID{source, target}
	sp, tp := arena.AddPort(), arena.AddPort()
	arena.SetPortNode(sp, &source)
	arena.SetPortNode(tp, &target)
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &sp)
	arena.SetEdgeTarget(edgeID, &tp)
	centerA, tail, centerB := arena.AddLabel("a"), arena.AddLabel("tail"), arena.AddLabel("b")
	arena.Label(centerA).Size.X, arena.Label(centerA).Size.Y = 20, 7
	arena.Label(centerB).Size.X, arena.Label(centerB).Size.Y = 30, 9
	basegraph.Set(&arena.Label(centerA).Properties, layered.EdgeLabelsPlacement, layered.EdgeLabelPlacementCenter)
	basegraph.Set(&arena.Label(tail).Properties, layered.EdgeLabelsPlacement, layered.EdgeLabelPlacementTail)
	basegraph.Set(&arena.Label(centerB).Properties, layered.EdgeLabelsPlacement, layered.EdgeLabelPlacementCenter)
	arena.Edge(edgeID).Labels = []layered.LLabelID{centerA, tail, centerB}

	InsertLabelDummies(arena, graphID)

	if got := len(arena.Graph(graphID).LayerlessNodes); got != 3 {
		t.Fatalf("layerless nodes = %d, want 3", got)
	}
	dummyID := arena.Graph(graphID).LayerlessNodes[2]
	dummy := arena.Node(dummyID)
	if dummy.Type != layered.LabelNode {
		t.Fatalf("dummy type = %v, want LABEL", dummy.Type)
	}
	if got := basegraph.Get(&dummy.Properties, layered.RepresentedLabels); len(got) != 2 || got[0] != centerA || got[1] != centerB {
		t.Fatalf("represented labels = %v", got)
	}
	if got := arena.Edge(edgeID).Labels; len(got) != 1 || got[0] != tail {
		t.Fatalf("retained edge labels = %v", got)
	}
	// Vertical layout stacks widths: 20 + spacing + 30.
	if dummy.Size.X != 53 || dummy.Size.Y != 12 {
		t.Fatalf("dummy size = %v, want (53,12)", dummy.Size)
	}
}

func TestInsertLabelDummiesSkipsSelfLoops(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	node := arena.AddNode(graphID)
	arena.Graph(graphID).LayerlessNodes = []layered.LNodeID{node}
	port := arena.AddPort()
	arena.SetPortNode(port, &node)
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &port)
	arena.SetEdgeTarget(edgeID, &port)
	labelID := arena.AddLabel("loop")
	basegraph.Set(&arena.Label(labelID).Properties, layered.EdgeLabelsPlacement, layered.EdgeLabelPlacementCenter)
	arena.Edge(edgeID).Labels = []layered.LLabelID{labelID}

	InsertLabelDummies(arena, graphID)

	if len(arena.Graph(graphID).LayerlessNodes) != 1 || len(arena.Edge(edgeID).Labels) != 1 {
		t.Fatal("self-loop center label was moved")
	}
}
