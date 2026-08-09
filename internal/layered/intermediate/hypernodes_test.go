// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"testing"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestProcessHypernodesReplacesFirstJoinBend(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	arena.Graph(graphID).Size = geometry.Point{X: 200, Y: 100}
	layerID := arena.AddLayerToGraph(graphID)
	hypernode := arena.AddNode(graphID)
	target := arena.AddNode(graphID)
	arena.SetNodeLayer(hypernode, &layerID)
	arena.SetNodeLayer(target, &layerID)
	arena.Node(hypernode).Position = geometry.Point{X: 50, Y: 0}
	arena.Node(hypernode).Size = geometry.Point{X: 20, Y: 10}
	arena.Node(target).Position = geometry.Point{X: 150, Y: 40}
	basegraph.Set(&arena.Node(hypernode).Properties, core.Hypernode, true)

	output := arena.AddPort()
	arena.SetPortNode(output, &hypernode)
	arena.SetPortSide(output, layered.EastSide)
	input := arena.AddPort()
	arena.SetPortNode(input, &target)
	arena.SetPortSide(input, layered.WestSide)
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &output)
	arena.SetEdgeTarget(edgeID, &input)
	arena.Edge(edgeID).BendPoints = geometry.Chain{{X: 100, Y: 5}, {X: 100, Y: 50}}
	basegraph.Set(&arena.Edge(edgeID).Properties, layered.JunctionPoints,
		geometry.Chain{{X: 100, Y: 5}, {X: 100, Y: 50}})

	ProcessHypernodes(arena, graphID)
	if got := arena.Node(hypernode).Position.X; got != 90 {
		t.Fatalf("hypernode x = %v, want 90", got)
	}
	if len(arena.Edge(edgeID).BendPoints) != 1 ||
		arena.Edge(edgeID).BendPoints[0] != (geometry.Point{X: 100, Y: 50}) {
		t.Fatalf("bend points = %+v", arena.Edge(edgeID).BendPoints)
	}
	if arena.Edge(edgeID).Source == nil {
		t.Fatal("edge source is missing")
	}
	newSource := arena.Port(*arena.Edge(edgeID).Source)
	if newSource.Side != layered.SouthSide || newSource.Node == nil || *newSource.Node != hypernode {
		t.Fatalf("new source port = %+v", newSource)
	}
	if junctions := basegraph.Get(&arena.Edge(edgeID).Properties, layered.JunctionPoints); len(junctions) != 1 || junctions[0] != (geometry.Point{X: 100, Y: 50}) {
		t.Fatalf("junctions = %+v", junctions)
	}
}

func TestProcessConstraintsSkipsDummyPositionIDs(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	firstLayer := arena.AddLayerToGraph(graphID)
	secondLayer := arena.AddLayerToGraph(graphID)
	first := arena.AddNode(graphID)
	dummy := arena.AddNode(graphID)
	last := arena.AddNode(graphID)
	arena.Node(dummy).Type = layered.LongEdgeNode
	arena.SetNodeLayer(first, &firstLayer)
	arena.SetNodeLayer(dummy, &firstLayer)
	arena.SetNodeLayer(last, &secondLayer)

	ProcessConstraints(arena, graphID)
	if got := basegraph.Get(&arena.Node(first).Properties, layered.LayerIDOption); got != 0 {
		t.Fatalf("first layer ID = %d", got)
	}
	if got := basegraph.Get(&arena.Node(first).Properties, layered.PositionIDOption); got != 0 {
		t.Fatalf("first position ID = %d", got)
	}
	if basegraph.Has(&arena.Node(dummy).Properties, layered.LayerIDOption) {
		t.Fatal("dummy received a layer ID")
	}
	if got := basegraph.Get(&arena.Node(last).Properties, layered.LayerIDOption); got != 1 {
		t.Fatalf("last layer ID = %d", got)
	}
}
