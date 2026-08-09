// Copyright (c) 2011, 2018 Kiel University and others.
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

func TestTransformGraphDownReadingDirectionRoundTrip(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	graph := arena.Graph(graphID)
	graph.Size = geometry.Point{X: 100, Y: 200}
	graph.Offset = geometry.Point{X: 3, Y: 7}
	graph.Padding = geometry.Spacing{Top: 1, Right: 2, Bottom: 3, Left: 4}
	basegraph.Set(&graph.Properties, layered.DirectionOption, layered.DirectionDown)
	basegraph.Set(&graph.Properties, layered.DirectionCongruencyOption, layered.DirectionCongruencyReading)

	nodeID := arena.AddNode(graphID)
	graph.LayerlessNodes = append(graph.LayerlessNodes, nodeID)
	node := arena.Node(nodeID)
	node.Position = geometry.Point{X: 10, Y: 20}
	node.Size = geometry.Point{X: 30, Y: 40}
	node.Padding = geometry.Spacing{Top: 5, Right: 6, Bottom: 7, Left: 8}
	portID := arena.AddPort()
	arena.SetPortNode(portID, &nodeID)
	arena.Port(portID).Size = geometry.Point{X: 2, Y: 4}
	arena.Port(portID).Position = geometry.Point{X: 30, Y: 12}
	arena.Port(portID).Anchor = geometry.Point{X: 1.5, Y: 3}
	arena.Port(portID).ExplicitAnchor = true
	arena.SetPortSide(portID, layered.EastSide)

	TransformGraph(arena, graphID, ToInternalLTR)
	if got, want := node.Position, (geometry.Point{X: 20, Y: 10}); got != want {
		t.Fatalf("internal node position = %v, want %v", got, want)
	}
	if got, want := node.Size, (geometry.Point{X: 40, Y: 30}); got != want {
		t.Fatalf("internal node size = %v, want %v", got, want)
	}
	if got := arena.Port(portID).Side; got != layered.SouthSide {
		t.Fatalf("internal port side = %v, want SOUTH", got)
	}
	if got, want := graph.Padding, (geometry.Spacing{Top: 4, Right: 3, Bottom: 2, Left: 1}); got != want {
		t.Fatalf("internal graph padding = %v, want %v", got, want)
	}

	TransformGraph(arena, graphID, ToInputDirection)
	if got, want := node.Position, (geometry.Point{X: 10, Y: 20}); got != want {
		t.Fatalf("round-trip node position = %v, want %v", got, want)
	}
	if got, want := node.Size, (geometry.Point{X: 30, Y: 40}); got != want {
		t.Fatalf("round-trip node size = %v, want %v", got, want)
	}
	if got := arena.Port(portID).Side; got != layered.EastSide {
		t.Fatalf("round-trip port side = %v, want EAST", got)
	}
	if got, want := graph.Padding, (geometry.Spacing{Top: 1, Right: 2, Bottom: 3, Left: 4}); got != want {
		t.Fatalf("round-trip graph padding = %v, want %v", got, want)
	}
}

func TestTransformGraphLeftMirrorsAgainstGraphWidth(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	graph := arena.Graph(graphID)
	graph.Size.X = 100
	basegraph.Set(&graph.Properties, layered.DirectionOption, layered.DirectionLeft)
	basegraph.Set(&graph.Properties, layered.DirectionCongruencyOption, layered.DirectionCongruencyReading)
	nodeID := arena.AddNode(graphID)
	graph.LayerlessNodes = append(graph.LayerlessNodes, nodeID)
	arena.Node(nodeID).Position.X = 15
	arena.Node(nodeID).Size.X = 20

	TransformGraph(arena, graphID, ToInternalLTR)
	if got := arena.Node(nodeID).Position.X; got != 65 {
		t.Fatalf("mirrored x = %v, want 65", got)
	}
	TransformGraph(arena, graphID, ToInputDirection)
	if got := arena.Node(nodeID).Position.X; got != 15 {
		t.Fatalf("round-trip x = %v, want 15", got)
	}
}
