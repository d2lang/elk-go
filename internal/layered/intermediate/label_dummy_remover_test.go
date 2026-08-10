// Copyright (c) 2012, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestRemoveLabelDummiesPlacesAndReattachesLabels(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	basegraph.Set(&arena.Graph(graphID).Properties, layered.DirectionOption, layered.DirectionRight)
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
	first, second := arena.AddLabel("first"), arena.AddLabel("second")
	arena.Label(first).Size = geometry.Point{X: 10, Y: 4}
	arena.Label(second).Size = geometry.Point{X: 20, Y: 5}
	basegraph.Set(&arena.Label(first).Properties, layered.EdgeLabelsPlacement, layered.EdgeLabelPlacementCenter)
	basegraph.Set(&arena.Label(second).Properties, layered.EdgeLabelsPlacement, layered.EdgeLabelPlacementCenter)
	arena.Edge(edgeID).Labels = []layered.LLabelID{first, second}
	InsertLabelDummies(arena, graphID)
	dummyID := arena.Graph(graphID).LayerlessNodes[2]
	layerID := arena.AddLayerToGraph(graphID)
	arena.SetNodeLayer(dummyID, &layerID)
	arena.Node(dummyID).Position = geometry.Point{X: 100, Y: 30}

	RemoveLabelDummies(arena, graphID)

	if len(arena.Layer(layerID).Nodes) != 0 {
		t.Fatal("label dummy remains in layer")
	}
	if got := arena.Edge(edgeID).Labels; len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("reattached labels = %v", got)
	}
	if got := arena.Label(first).Position; got.X != 105 || got.Y != 30 {
		t.Fatalf("first label position = %v, want (105,30)", got)
	}
	if got := arena.Label(second).Position; got.X != 100 || got.Y != 37 {
		t.Fatalf("second label position = %v, want (100,37)", got)
	}
	if arena.Edge(edgeID).Target == nil || *arena.Edge(edgeID).Target != tp {
		t.Fatalf("joined target = %v, want %v", arena.Edge(edgeID).Target, tp)
	}
}

func TestVerticalCenterLabelPlacementReversesUpOrder(t *testing.T) {
	arena := layered.NewLGraphArena()
	first, second := arena.AddLabel("first"), arena.AddLabel("second")
	arena.Label(first).Size = geometry.Point{X: 10, Y: 4}
	arena.Label(second).Size = geometry.Point{X: 20, Y: 6}

	placeVerticalCenterLabels(arena, []layered.LLabelID{first, second}, 10, 20, 3, 12, false, layered.DirectionUp)

	if got := arena.Label(second).Position; got.X != 10 || got.Y != 26 {
		t.Fatalf("second (first placed) = %v, want (10,26)", got)
	}
	if got := arena.Label(first).Position; got.X != 33 || got.Y != 28 {
		t.Fatalf("first (second placed) = %v, want (33,28)", got)
	}
}

func TestRemoveInlineLabelUsesNormalizedDummyHeight(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	graph := arena.Graph(graphID)
	basegraph.Set(&graph.Properties, layered.DirectionOption, layered.DirectionDown)
	basegraph.Set(&graph.Properties, layered.SpacingEdgeLabel, 2.0)
	source, target := arena.AddNode(graphID), arena.AddNode(graphID)
	graph.LayerlessNodes = []layered.LNodeID{source, target}
	sourcePort, targetPort := arena.AddPort(), arena.AddPort()
	arena.SetPortNode(sourcePort, &source)
	arena.SetPortNode(targetPort, &target)
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &sourcePort)
	arena.SetEdgeTarget(edgeID, &targetPort)
	labelID := arena.AddLabel("inline")
	arena.Label(labelID).Size = geometry.Point{X: 10, Y: 4}
	basegraph.Set(&arena.Label(labelID).Properties,
		layered.EdgeLabelsPlacement, layered.EdgeLabelPlacementCenter)
	basegraph.Set(&arena.Label(labelID).Properties, layered.EdgeLabelsInline, true)
	arena.Edge(edgeID).Labels = []layered.LLabelID{labelID}

	InsertLabelDummies(arena, graphID)
	dummyID := graph.LayerlessNodes[2]
	layerID := arena.AddLayerToGraph(graphID)
	arena.SetNodeLayer(dummyID, &layerID)
	arena.Node(dummyID).Position = geometry.Point{X: 100, Y: 30}
	applyLabelDummySide(arena, graphID, dummyID, layered.LabelSideAbove)
	if got := arena.Node(dummyID).Size.Y; got != 4 {
		t.Fatalf("normalized dummy height = %v, want 4", got)
	}

	RemoveLabelDummies(arena, graphID)
	if got, want := arena.Label(labelID).Position, (geometry.Point{X: 100, Y: 30}); got != want {
		t.Fatalf("inline label position = %v, want %v", got, want)
	}
}
