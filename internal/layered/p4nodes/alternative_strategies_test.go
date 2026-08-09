// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"testing"

	"github.com/d2lang/elk-go/internal/layered"
)

func TestSimpleNodePlacerStacksSourceShapedLayer(t *testing.T) {
	arena, graphID, nodes := alternativePlacementLayer(0, 0)
	if err := (&SimpleNodePlacer{}).Process(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if got := arena.Node(nodes[0]).Position.Y; got != 0 {
		t.Fatalf("first y = %v, want 0", got)
	}
	if got := arena.Node(nodes[1]).Position.Y; got != 30 {
		t.Fatalf("second y = %v, want 30", got)
	}
}

func TestInteractiveNodePlacerOnlyMovesOverlappingNode(t *testing.T) {
	arena, graphID, nodes := alternativePlacementLayer(5, 8)
	if err := (&InteractiveNodePlacer{}).Process(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if got := arena.Node(nodes[0]).Position.Y; got != 5 {
		t.Fatalf("first y = %v, want 5", got)
	}
	if got := arena.Node(nodes[1]).Position.Y; got != 35 {
		t.Fatalf("second y = %v, want 35", got)
	}
}

func TestLinearSegmentsNodePlacerKeepsSourceShapedLayerSeparated(t *testing.T) {
	arena, graphID, nodes := alternativePlacementLayer(0, 0)
	if err := (&LinearSegmentsNodePlacer{}).Process(arena, graphID); err != nil {
		t.Fatal(err)
	}
	first, second := arena.Node(nodes[0]), arena.Node(nodes[1])
	if first.Position.Y+first.Size.Y > second.Position.Y {
		t.Fatalf("linear-segment placement overlaps: first=%v..%v second=%v", first.Position.Y, first.Position.Y+first.Size.Y, second.Position.Y)
	}
}

func alternativePlacementLayer(firstY, secondY float64) (
	*layered.LGraphArena,
	layered.LGraphID,
	[]layered.LNodeID,
) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = append(arena.Graph(graphID).Layers, layerID)
	nodes := []layered.LNodeID{arena.AddNode(graphID), arena.AddNode(graphID)}
	for index, nodeID := range nodes {
		arena.SetNodeLayer(nodeID, &layerID)
		arena.Node(nodeID).Size.Y = float64(10 + index*10)
	}
	arena.Node(nodes[0]).Position.Y = firstY
	arena.Node(nodes[1]).Position.Y = secondY
	return arena, graphID, nodes
}
