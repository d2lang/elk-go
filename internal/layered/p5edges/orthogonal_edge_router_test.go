// Copyright (c) 2010, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p5edges

import (
	"reflect"
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestOrthogonalEdgeRouterPlacesLayersAndRoutesEdge(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	leftLayer, rightLayer := arena.AddLayerToGraph(graphID), arena.AddLayerToGraph(graphID)
	arena.Layer(leftLayer).Size.X = 30
	arena.Layer(rightLayer).Size.X = 40
	leftNode, rightNode := arena.AddNode(graphID), arena.AddNode(graphID)
	arena.Node(leftNode).Size = geometry.Point{X: 30, Y: 20}
	arena.Node(rightNode).Size = geometry.Point{X: 40, Y: 20}
	arena.Node(rightNode).Position.Y = 30
	arena.SetNodeLayer(leftNode, &leftLayer)
	arena.SetNodeLayer(rightNode, &rightLayer)
	leftPort, rightPort := arena.AddPort(), arena.AddPort()
	arena.SetPortNode(leftPort, &leftNode)
	arena.SetPortNode(rightPort, &rightNode)
	arena.SetPortSide(leftPort, layered.EastSide)
	arena.SetPortSide(rightPort, layered.WestSide)
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &leftPort)
	arena.SetEdgeTarget(edgeID, &rightPort)

	if err := (&OrthogonalEdgeRouter{}).Process(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if got, want := arena.Graph(graphID).Size.X, 90.0; got != want {
		t.Fatalf("graph width = %v, want %v", got, want)
	}
	if got, want := arena.Node(rightNode).Position.X, 50.0; got != want {
		t.Fatalf("right node x = %v, want %v", got, want)
	}
	if got, want := arena.Edge(edgeID).BendPoints, (geometry.Chain{{X: 40}, {X: 40, Y: 30}}); !reflect.DeepEqual(got, want) {
		t.Fatalf("bend points = %v, want %v", got, want)
	}
}

func TestPlaceNodesHorizontallyHonorsAlignmentAndMargins(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayerToGraph(graphID)
	arena.Layer(layerID).Size.X = 100
	left, right := arena.AddNode(graphID), arena.AddNode(graphID)
	for _, nodeID := range []layered.LNodeID{left, right} {
		arena.Node(nodeID).Size.X = 20
		arena.SetNodeLayer(nodeID, &layerID)
	}
	arena.Node(left).Margin.Left = 7
	arena.Node(right).Margin.Right = 11
	basegraph.Set(&arena.Node(left).Properties, layered.AlignmentOption, layered.AlignmentLeft)
	basegraph.Set(&arena.Node(right).Properties, layered.AlignmentOption, layered.AlignmentRight)
	placeNodesHorizontally(arena, layerID, 5)
	if arena.Node(left).Position.X != 12 || arena.Node(right).Position.X != 74 {
		t.Fatalf("node positions = %v, %v; want 12, 74", arena.Node(left).Position.X, arena.Node(right).Position.X)
	}
}

func TestExternalPortPredicateMatchesWestEastDummyRule(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	nodeID := arena.AddNode(graphID)
	arena.Node(nodeID).Type = layered.ExternalPortNode
	basegraph.Set(&arena.Node(nodeID).Properties, layered.ExternalPortSide, layered.WestSide)
	if !allExternalWestOrEastPorts(arena, []layered.LNodeID{nodeID}) {
		t.Fatal("west external-port dummy not recognized")
	}
	basegraph.Set(&arena.Node(nodeID).Properties, layered.ExternalPortSide, layered.NorthSide)
	if allExternalWestOrEastPorts(arena, []layered.LNodeID{nodeID}) {
		t.Fatal("north external-port dummy incorrectly recognized")
	}
	if !allExternalWestOrEastPorts(arena, nil) {
		t.Fatal("Iterables.all(null-boundary replacement) must be true")
	}
}

func TestOrthogonalConfigurationAddsFeatureProcessors(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	basegraph.Set(&arena.Graph(graphID).Properties, layered.GraphProperties,
		layered.GraphPropertyHyperedges|layered.GraphPropertyNonFreePorts|
			layered.GraphPropertyNorthSouthPorts|layered.GraphPropertyExternalPorts|
			layered.GraphPropertySelfLoops|layered.GraphPropertyHypernodes|
			layered.GraphPropertyCenterLabels|layered.GraphPropertyEndLabels)
	configuration := (&OrthogonalEdgeRouter{}).ProcessorConfiguration(arena, graphID)
	tests := []struct {
		slot      int
		processor layered.IntermediateProcessorStrategy
	}{
		{0, layered.SelfLoopPreprocessor},
		{1, layered.LabelDummyInserter},
		{2, layered.HierarchicalPortConstraintProcessor},
		{2, layered.InvertedPortProcessor},
		{2, layered.NorthSouthPortPreprocessor},
		{3, layered.SelfLoopRouter},
		{3, layered.HyperedgeDummyMerger},
		{3, layered.HierarchicalPortDummySizeProcessor},
		{5, layered.HypernodeProcessor},
		{5, layered.HierarchicalPortOrthogonalEdgeRouter},
		{5, layered.SelfLoopPostprocessor},
		{5, layered.NorthSouthPortPostprocessor},
		{5, layered.LabelDummyRemover},
		{5, layered.EndLabelPostprocessor},
	}
	for _, test := range tests {
		if !containsProcessor(configuration.Slot(test.slot), test.processor) {
			t.Errorf("slot %d lacks processor %v", test.slot, test.processor)
		}
	}
}

func containsProcessor(values []layered.IntermediateProcessorStrategy, target layered.IntermediateProcessorStrategy) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
