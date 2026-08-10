// Copyright (c) 2011, 2015 Kiel University and others.
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

func TestComponentsProcessorSplitUsesStableDFSOrder(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	graph := arena.Graph(graphID)
	graph.Padding = geometry.Spacing{Top: 1, Right: 2, Bottom: 3, Left: 4}
	graph.Properties.SetRaw("test.property", "copied")
	basegraph.Set(&graph.Properties, layered.NodeSizeMinimum,
		geometry.Point{X: 500, Y: 400})

	a := addComponentNode(arena, graphID)
	b := addComponentNode(arena, graphID)
	c := addComponentNode(arena, graphID)
	connectComponentNodes(arena, a, b)
	// The visitation marker is an implementation field in Java. Split resets
	// it before walking the graph.
	arena.Node(a).ID, arena.Node(b).ID, arena.Node(c).ID = 8, 9, 10

	processor := NewComponentsProcessor()
	got, err := processor.Split(arena, graphID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("components = %v, want two", got)
	}
	if want := []layered.LNodeID{a, b}; !reflect.DeepEqual(
		arena.Graph(got[0]).LayerlessNodes, want,
	) {
		t.Fatalf("first component = %v, want %v",
			arena.Graph(got[0]).LayerlessNodes, want)
	}
	if want := []layered.LNodeID{c}; !reflect.DeepEqual(
		arena.Graph(got[1]).LayerlessNodes, want,
	) {
		t.Fatalf("second component = %v, want %v",
			arena.Graph(got[1]).LayerlessNodes, want)
	}
	for index, componentID := range got {
		component := arena.Graph(componentID)
		if component.Padding != graph.Padding {
			t.Fatalf("component %d padding = %v, want %v",
				index, component.Padding, graph.Padding)
		}
		if value, found := component.Properties.Raw("test.property"); !found || value != "copied" {
			t.Fatalf("component %d did not copy graph properties", index)
		}
		if component.Properties.Has(layered.NodeSizeMinimum.ID) {
			t.Fatalf("component %d retained the original minimum size", index)
		}
	}
	if arena.Node(a).Graph != got[0] || arena.Node(b).Graph != got[0] ||
		arena.Node(c).Graph != got[1] {
		t.Fatal("split did not transfer node graph ownership")
	}
}

func TestMakeLayerlessPreservesLayerAndNodeOrder(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	firstLayer := arena.AddLayerToGraph(graphID)
	secondLayer := arena.AddLayerToGraph(graphID)
	a := arena.AddNode(graphID)
	b := arena.AddNode(graphID)
	c := arena.AddNode(graphID)
	arena.SetNodeLayer(a, &firstLayer)
	arena.SetNodeLayer(b, &firstLayer)
	arena.SetNodeLayer(c, &secondLayer)

	MakeLayerless(arena, graphID)
	if want := []layered.LNodeID{a, b, c}; !reflect.DeepEqual(
		arena.Graph(graphID).LayerlessNodes, want,
	) {
		t.Fatalf("layerless nodes = %v, want %v",
			arena.Graph(graphID).LayerlessNodes, want)
	}
	if len(arena.Graph(graphID).Layers) != 0 ||
		len(arena.Layer(firstLayer).Nodes) != 0 || len(arena.Layer(secondLayer).Nodes) != 0 {
		t.Fatal("layer cleanup left graph or layer membership")
	}
	for _, nodeID := range []layered.LNodeID{a, b, c} {
		if arena.Node(nodeID).Layer != nil {
			t.Fatalf("node %d retained its layer", nodeID)
		}
	}
}

func TestComponentsProcessorSelectsExternalPortGraphPlacers(t *testing.T) {
	tests := []struct {
		name       string
		ordering   layered.ComponentOrderingStrategy
		groupModel bool
		rowModel   bool
		sector     bool
	}{
		{
			name:     "sector placer",
			ordering: layered.ComponentOrderingNone,
			sector:   true,
		},
		{
			name:     "inside-side model order",
			ordering: layered.ComponentOrderingInsidePortSideGroups,
			sector:   true,
		},
		{
			name:       "group model order",
			ordering:   layered.ComponentOrderingGroupModelOrder,
			groupModel: true,
		},
		{
			name:     "row model order",
			ordering: layered.ComponentOrderingModelOrder,
			rowModel: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			arena := layered.NewLGraphArena()
			graphID := arena.AddGraph()
			graph := arena.Graph(graphID)
			basegraph.Set(&graph.Properties, layered.GraphProperties,
				layered.GraphPropertyExternalPorts)
			basegraph.Set(&graph.Properties, layered.ComponentOrdering, test.ordering)

			external := addComponentNode(arena, graphID)
			arena.Node(external).Type = layered.ExternalPortNode
			basegraph.Set(&arena.Node(external).Properties,
				layered.ExternalPortSide, layered.NorthSide)
			addComponentNode(arena, graphID)

			processor := NewComponentsProcessor()
			components, err := processor.Split(arena, graphID)
			if err != nil {
				t.Fatal(err)
			}
			if len(components) != 2 {
				t.Fatalf("components = %v, want two", components)
			}
			_, isGroupModel := processor.graphPlacer.(*componentGroupModelOrderGraphPlacer)
			_, isSector := processor.graphPlacer.(*componentGroupGraphPlacer)
			row, isRow := processor.graphPlacer.(*simpleRowGraphPlacer)
			isRowModel := isRow && row.modelOrder
			if isGroupModel != test.groupModel || isSector != test.sector ||
				isRowModel != test.rowModel {
				t.Fatalf(
					"placer = %T, group model=%v sector=%v row model=%v",
					processor.graphPlacer, test.groupModel, test.sector, test.rowModel,
				)
			}
		})
	}
}

func addComponentNode(arena *layered.LGraphArena, graphID layered.LGraphID) layered.LNodeID {
	nodeID := arena.AddNode(graphID)
	arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, nodeID)
	return nodeID
}

func connectComponentNodes(
	arena *layered.LGraphArena,
	source, target layered.LNodeID,
) layered.LEdgeID {
	sourcePort, targetPort := arena.AddPort(), arena.AddPort()
	arena.SetPortNode(sourcePort, &source)
	arena.SetPortNode(targetPort, &target)
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &sourcePort)
	arena.SetEdgeTarget(edgeID, &targetPort)
	return edgeID
}
