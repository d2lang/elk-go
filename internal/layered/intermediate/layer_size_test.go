// Copyright (c) 2010, 2020 Kiel University and others.
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

func TestCalculateLayerSizesAndGraphHeight(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayerToGraph(graphID)
	first, last := arena.AddNode(graphID), arena.AddNode(graphID)
	arena.SetNodeLayer(first, &layerID)
	arena.SetNodeLayer(last, &layerID)
	arena.Node(first).Position = geometry.Point{X: 10, Y: 20}
	arena.Node(first).Size = geometry.Point{X: 30, Y: 40}
	arena.Node(first).Margin = geometry.Spacing{Top: 3, Right: 7, Bottom: 5, Left: 11}
	arena.Node(last).Position = geometry.Point{X: 5, Y: 90}
	arena.Node(last).Size = geometry.Point{X: 60, Y: 20}
	arena.Node(last).Margin = geometry.Spacing{Top: 2, Right: 4, Bottom: 13, Left: 6}

	CalculateLayerSizesAndGraphHeight(arena, graphID)

	if got := arena.Layer(layerID).Size; got.X != 70 || got.Y != 106 {
		t.Fatalf("layer size = %v, want (70,106)", got)
	}
	if got := arena.Graph(graphID).Size.Y; got != 106 {
		t.Fatalf("graph height = %v, want 106", got)
	}
	if got := arena.Graph(graphID).Offset.Y; got != -17 {
		t.Fatalf("graph offset y = %v, want -17", got)
	}
}

func TestCalculateLayerSizesIncludesExternalPortSurroundingSpacing(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayerToGraph(graphID)
	node := arena.AddNode(graphID)
	arena.SetNodeLayer(node, &layerID)
	arena.Node(node).Type = layered.ExternalPortNode
	arena.Node(node).Position.Y = 20
	arena.Node(node).Size.Y = 10
	basegraph.Set(&arena.Graph(graphID).Properties, layered.SpacingPortsSurrounding,
		geometry.Spacing{Top: 4, Bottom: 6})

	CalculateLayerSizesAndGraphHeight(arena, graphID)

	if got := arena.Layer(layerID).Size.Y; got != 20 {
		t.Fatalf("layer height = %v, want 20", got)
	}
	if got := arena.Graph(graphID).Offset.Y; got != -16 {
		t.Fatalf("graph offset y = %v, want -16", got)
	}
}

func TestCalculateLayerSizesEmptyGraph(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	arena.AddLayerToGraph(graphID)
	arena.Graph(graphID).Size.Y = 99
	arena.Graph(graphID).Offset.Y = 7

	CalculateLayerSizesAndGraphHeight(arena, graphID)

	if got := arena.Graph(graphID).Size.Y; got != 0 {
		t.Fatalf("empty graph height = %v, want 0", got)
	}
	if got := arena.Graph(graphID).Offset.Y; got != 7 {
		t.Fatalf("empty graph offset = %v, want 7", got)
	}
}
