// Copyright (c) 2012, 2015 Kiel University and others.
// Copyright (c) 2022 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package components

import (
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestComponentGroupGraphPlacerUsesExternalPortSectors(t *testing.T) {
	arena := layered.NewLGraphArena()
	target := arena.AddGraph()
	northGraph, northNode := addSizedComponent(
		arena, geometry.Point{X: 20, Y: 10}, sidesNorth)
	centerGraph, centerNode := addSizedComponent(
		arena, geometry.Point{X: 30, Y: 20}, sidesNone)
	eastGraph, eastNode := addSizedComponent(
		arena, geometry.Point{X: 15, Y: 25}, sidesEast)
	components := []layered.LGraphID{northGraph, centerGraph, eastGraph}

	placer := &componentGroupGraphPlacer{}
	if err := placer.combine(arena, components, target); err != nil {
		t.Fatal(err)
	}
	if got, want := arena.Node(northNode).Position, (geometry.Point{}); got != want {
		t.Fatalf("north position = %+v, want %+v", got, want)
	}
	if got, want := arena.Node(centerNode).Position,
		(geometry.Point{Y: 20}); got != want {
		t.Fatalf("center position = %+v, want %+v", got, want)
	}
	if got, want := arena.Node(eastNode).Position,
		(geometry.Point{X: 40, Y: 20}); got != want {
		t.Fatalf("east position = %+v, want %+v", got, want)
	}
	if got, want := arena.Graph(target).Size,
		(geometry.Point{X: 55, Y: 45}); got != want {
		t.Fatalf("target size = %+v, want %+v", got, want)
	}
	if got := len(arena.Graph(target).LayerlessNodes); got != 3 {
		t.Fatalf("target nodes = %d, want 3", got)
	}
}

func TestComponentGroupModelOrderGraphPlacerStartsNewDirectionalGroup(t *testing.T) {
	arena := layered.NewLGraphArena()
	target := arena.AddGraph()
	centerGraph, centerNode := addSizedComponent(
		arena, geometry.Point{X: 10, Y: 10}, sidesNone)
	northGraph, northNode := addSizedComponent(
		arena, geometry.Point{X: 10, Y: 10}, sidesNorth)
	for _, graphID := range []layered.LGraphID{centerGraph, northGraph} {
		basegraph.Set(&arena.Graph(graphID).Properties,
			layered.SpacingComponentComponent, 5.0)
		basegraph.Set(&arena.Graph(graphID).Properties,
			layered.DirectionOption, layered.DirectionRight)
	}

	placer := &componentGroupModelOrderGraphPlacer{}
	if err := placer.combine(arena,
		[]layered.LGraphID{centerGraph, northGraph}, target); err != nil {
		t.Fatal(err)
	}
	if got, want := arena.Node(centerNode).Position, (geometry.Point{}); got != want {
		t.Fatalf("center position = %+v, want %+v", got, want)
	}
	if got, want := arena.Node(northNode).Position,
		(geometry.Point{X: 15, Y: 15}); got != want {
		t.Fatalf("north position = %+v, want %+v", got, want)
	}
	if got, want := arena.Graph(target).Size,
		(geometry.Point{X: 25, Y: 25}); got != want {
		t.Fatalf("target size = %+v, want %+v", got, want)
	}
}

func addSizedComponent(
	arena *layered.LGraphArena,
	size geometry.Point,
	sides uint8,
) (layered.LGraphID, layered.LNodeID) {
	graphID := arena.AddGraph()
	graph := arena.Graph(graphID)
	graph.Size = size
	basegraph.Set(&graph.Properties, layered.ExternalPortConnections, sides)
	basegraph.Set(&graph.Properties, layered.SpacingComponentComponent, 10.0)
	basegraph.Set(&graph.Properties, layered.AspectRatio, 1.6)
	nodeID := arena.AddNode(graphID)
	arena.Node(nodeID).Size = size
	graph.LayerlessNodes = append(graph.LayerlessNodes, nodeID)
	return graphID, nodeID
}
