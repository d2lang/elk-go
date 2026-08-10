// Copyright (c) 2012, 2020 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package components

import (
	"reflect"
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestSimpleRowGraphPlacerSortsByAreaAndStartsNewRow(t *testing.T) {
	arena := layered.NewLGraphArena()
	targetID := arena.AddGraph()
	largeID := arena.AddGraph()
	smallID := arena.AddGraph()
	large, small := arena.Graph(largeID), arena.Graph(smallID)
	large.Size = geometry.Point{X: 100, Y: 40}
	small.Size = geometry.Point{X: 20, Y: 20}
	large.Offset = geometry.Point{X: 5, Y: 7}
	small.Offset = geometry.Point{X: 1, Y: 2}
	largeNode := addComponentNode(arena, largeID)
	smallNode := addComponentNode(arena, smallID)

	components := []layered.LGraphID{largeID, smallID}
	if err := (&simpleRowGraphPlacer{}).combine(arena, components, targetID); err != nil {
		t.Fatal(err)
	}
	if want := []layered.LGraphID{smallID, largeID}; !reflect.DeepEqual(components, want) {
		t.Fatalf("component order = %v, want %v", components, want)
	}
	if got, want := arena.Node(smallNode).Position, (geometry.Point{X: 1, Y: 2}); got != want {
		t.Fatalf("small node position = %v, want %v", got, want)
	}
	if got, want := arena.Node(largeNode).Position, (geometry.Point{X: 5, Y: 47}); got != want {
		t.Fatalf("large node position = %v, want %v", got, want)
	}
	if got, want := arena.Graph(targetID).Size, (geometry.Point{X: 100, Y: 80}); got != want {
		t.Fatalf("target size = %v, want %v", got, want)
	}
	if want := []layered.LNodeID{smallNode, largeNode}; !reflect.DeepEqual(
		arena.Graph(targetID).LayerlessNodes, want,
	) {
		t.Fatalf("target nodes = %v, want %v",
			arena.Graph(targetID).LayerlessNodes, want)
	}
	if arena.Node(smallNode).Graph != targetID || arena.Node(largeNode).Graph != targetID {
		t.Fatal("combine did not transfer node graph ownership")
	}
	if large.Offset != (geometry.Point{}) || small.Offset != (geometry.Point{}) {
		t.Fatalf("component offsets were not reset: large=%v small=%v",
			large.Offset, small.Offset)
	}
}

func TestModelOrderRowGraphPlacerHonorsExternalPortSides(t *testing.T) {
	arena := layered.NewLGraphArena()
	targetID := arena.AddGraph()
	type componentSpec struct {
		size  geometry.Point
		sides []layered.PortSide
	}
	specs := []componentSpec{
		{size: geometry.Point{X: 40, Y: 20}, sides: []layered.PortSide{layered.SouthSide}},
		{size: geometry.Point{X: 70, Y: 30}, sides: []layered.PortSide{layered.NorthSide}},
		{size: geometry.Point{X: 30, Y: 10}, sides: []layered.PortSide{layered.EastSide}},
		{size: geometry.Point{X: 20, Y: 15}},
		{size: geometry.Point{X: 20, Y: 25}, sides: []layered.PortSide{layered.WestSide}},
	}
	components := make([]layered.LGraphID, 0, len(specs))
	nodes := make([]layered.LNodeID, 0, len(specs))
	for _, spec := range specs {
		componentID := arena.AddGraph()
		component := arena.Graph(componentID)
		component.Size = spec.size
		var sides uint8
		for _, side := range spec.sides {
			sides |= uint8(1) << uint8(side)
		}
		basegraph.Set(&component.Properties, layered.ExternalPortConnections, sides)
		components = append(components, componentID)
		nodes = append(nodes, addComponentNode(arena, componentID))
	}

	placeComponentsInModelOrder(
		arena, components, arena.Graph(targetID), 100, 10,
	)
	wantPositions := []geometry.Point{
		{X: 0, Y: 0},
		{X: 50, Y: 0},
		{X: 50, Y: 40},
		{X: 50, Y: 60},
		{X: 50, Y: 85},
	}
	for index, nodeID := range nodes {
		if got := arena.Node(nodeID).Position; got != wantPositions[index] {
			t.Fatalf("component %d position = %v, want %v",
				index, got, wantPositions[index])
		}
	}
	if got, want := arena.Graph(targetID).Size, (geometry.Point{X: 120, Y: 110}); got != want {
		t.Fatalf("target size = %v, want %v", got, want)
	}
}

func TestAbstractGraphPlacerOffsetsEdgeGeometryOnce(t *testing.T) {
	arena := layered.NewLGraphArena()
	targetID, sourceID := arena.AddGraph(), arena.AddGraph()
	source := arena.Graph(sourceID)
	source.Offset = geometry.Point{X: 3, Y: 4}
	a := addComponentNode(arena, sourceID)
	b := addComponentNode(arena, sourceID)
	edgeID := connectComponentNodes(arena, a, b)
	edge := arena.Edge(edgeID)
	edge.BendPoints = geometry.Chain{{X: 1, Y: 2}}
	basegraph.Set(&edge.Properties, layered.JunctionPoints,
		geometry.Chain{{X: 5, Y: 6}})
	labelID := arena.AddLabel("edge")
	arena.Label(labelID).Position = geometry.Point{X: 7, Y: 8}
	edge.Labels = append(edge.Labels, labelID)

	moveGraph(arena, targetID, sourceID, geometry.Point{X: 10, Y: 20})
	offset := geometry.Point{X: 13, Y: 24}
	if got := arena.Edge(edgeID).BendPoints[0]; got != (geometry.Point{X: 14, Y: 26}) {
		t.Fatalf("bend point = %v", got)
	}
	junctions := basegraph.Get(&arena.Edge(edgeID).Properties, layered.JunctionPoints)
	if got := junctions[0]; got != (geometry.Point{X: 18, Y: 30}) {
		t.Fatalf("junction point = %v", got)
	}
	if got := arena.Label(labelID).Position; got != (geometry.Point{X: 20, Y: 32}) {
		t.Fatalf("edge label = %v", got)
	}
	if arena.Node(a).Position != offset || arena.Node(b).Position != offset {
		t.Fatalf("nodes were not offset exactly once: a=%v b=%v",
			arena.Node(a).Position, arena.Node(b).Position)
	}
}
