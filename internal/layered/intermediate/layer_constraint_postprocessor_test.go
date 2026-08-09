// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestLayerConstraintPostprocessorRestoresSeparateLayers(t *testing.T) {
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
	firstTarget := *arena.Edge(firstEdge).Target
	lastSource := *arena.Edge(lastEdge).Source

	if err := ProcessLayerConstraintsBeforeLayering(arena, graphID); err != nil {
		t.Fatal(err)
	}
	layerID := arena.AddLayerToGraph(graphID)
	arena.Graph(graphID).LayerlessNodes = nil
	arena.SetNodeLayer(middle, &layerID)
	if err := ProcessLayerConstraintsAfterLayering(arena, graphID); err != nil {
		t.Fatal(err)
	}

	layers := arena.Graph(graphID).Layers
	if len(layers) != 3 {
		t.Fatalf("layers = %v, want three", layers)
	}
	if got := arena.Layer(layers[0]).Nodes; len(got) != 1 || got[0] != first {
		t.Fatalf("first separate layer = %v", got)
	}
	if got := arena.Layer(layers[1]).Nodes; len(got) != 1 || got[0] != middle {
		t.Fatalf("middle layer = %v", got)
	}
	if got := arena.Layer(layers[2]).Nodes; len(got) != 1 || got[0] != last {
		t.Fatalf("last separate layer = %v", got)
	}
	if arena.Edge(firstEdge).Target == nil || *arena.Edge(firstEdge).Target != firstTarget ||
		arena.Edge(lastEdge).Source == nil || *arena.Edge(lastEdge).Source != lastSource {
		t.Fatalf("restored endpoints = first target %v, last source %v",
			arena.Edge(firstEdge).Target, arena.Edge(lastEdge).Source)
	}
}

func TestLayerConstraintPostprocessorMovesFirstAndLabelDummy(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	firstLayer := arena.AddLayerToGraph(graphID)
	secondLayer := arena.AddLayerToGraph(graphID)
	label := arena.AddNode(graphID)
	arena.Node(label).Type = layered.LabelNode
	first := arena.AddNode(graphID)
	basegraph.Set(&arena.Node(first).Properties, layered.LayerConstraintOption,
		layered.LayerConstraintFirst)
	other := arena.AddNode(graphID)
	arena.SetNodeLayer(label, &firstLayer)
	arena.SetNodeLayer(other, &firstLayer)
	arena.SetNodeLayer(first, &secondLayer)
	addLayerConstraintTestEdge(arena, label, first)

	if err := (&LayerConstraintPostprocessor{}).Process(arena, graphID); err != nil {
		t.Fatal(err)
	}
	layers := arena.Graph(graphID).Layers
	if len(layers) != 2 {
		t.Fatalf("layers = %v, want label and first layers", layers)
	}
	if got := arena.Layer(layers[0]).Nodes; len(got) != 1 || got[0] != label {
		t.Fatalf("label layer = %v", got)
	}
	if got := arena.Layer(layers[1]).Nodes; len(got) != 2 || got[0] != other || got[1] != first {
		t.Fatalf("first layer = %v, want [%v %v]", got, other, first)
	}
}
