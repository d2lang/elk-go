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

func TestPolylineEdgeRouterRoutesAcrossLayersAndReservesSlopeSpace(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	leftLayer, rightLayer := arena.AddLayerToGraph(graphID), arena.AddLayerToGraph(graphID)
	arena.Layer(leftLayer).Size.X, arena.Layer(rightLayer).Size.X = 10, 10
	leftNode, rightNode := arena.AddNode(graphID), arena.AddNode(graphID)
	arena.Node(leftNode).Size.X, arena.Node(rightNode).Size.X = 10, 10
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

	router := &PolylineEdgeRouter{}
	if err := router.Process(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if got, want := arena.Graph(graphID).Size.X, 46.0; got != want {
		t.Fatalf("graph width = %v, want %v", got, want)
	}
	if got, want := arena.Node(rightNode).Position.X, 36.0; got != want {
		t.Fatalf("right node x = %v, want %v", got, want)
	}
	if got, want := arena.Edge(edgeID).BendPoints, (geometry.Chain{{X: 10}}); !reflect.DeepEqual(got, want) {
		t.Fatalf("bend points = %v, want %v", got, want)
	}
}

func TestPolylineConfigurationAlwaysAddsInvertedPortProcessor(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	basegraph.Set(&arena.Graph(graphID).Properties, layered.GraphProperties,
		layered.GraphPropertyNorthSouthPorts|layered.GraphPropertySelfLoops)
	configuration := (&PolylineEdgeRouter{}).ProcessorConfiguration(arena, graphID)
	if !containsProcessor(configuration.Slot(2), layered.InvertedPortProcessor) ||
		!containsProcessor(configuration.Slot(2), layered.NorthSouthPortPreprocessor) ||
		!containsProcessor(configuration.Slot(5), layered.NorthSouthPortPostprocessor) {
		t.Fatalf("polyline configuration missing required processors: before-p3=%v after-p5=%v",
			configuration.Slot(2), configuration.Slot(5))
	}
}

func TestPolylineEdgeRouterAddsThreePointsForInLayerEdge(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayerToGraph(graphID)
	arena.Layer(layerID).Size.X = 20
	sourceNode, targetNode := arena.AddNode(graphID), arena.AddNode(graphID)
	arena.Node(sourceNode).Size.X, arena.Node(targetNode).Size.X = 20, 20
	arena.Node(targetNode).Position.Y = 30
	arena.SetNodeLayer(sourceNode, &layerID)
	arena.SetNodeLayer(targetNode, &layerID)
	sourcePort, targetPort := arena.AddPort(), arena.AddPort()
	arena.SetPortNode(sourcePort, &sourceNode)
	arena.SetPortNode(targetPort, &targetNode)
	arena.SetPortSide(sourcePort, layered.EastSide)
	arena.SetPortSide(targetPort, layered.EastSide)
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &sourcePort)
	arena.SetEdgeTarget(edgeID, &targetPort)

	router := &PolylineEdgeRouter{}
	if err := router.Process(arena, graphID); err != nil {
		t.Fatal(err)
	}
	want := geometry.Chain{{X: 20}, {X: 26, Y: 15}, {X: 20, Y: 30}}
	if got := arena.Edge(edgeID).BendPoints; !reflect.DeepEqual(got, want) {
		t.Fatalf("bend points = %v, want %v", got, want)
	}
}

func TestPolylineJunctionPointIsCreatedOnlyOncePerPosition(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayerToGraph(graphID)
	nodeID := arena.AddNode(graphID)
	arena.SetNodeLayer(nodeID, &layerID)
	portID := arena.AddPort()
	arena.SetPortNode(portID, &nodeID)
	arena.SetPortSide(portID, layered.EastSide)
	router := &PolylineEdgeRouter{createdJunctionPoints: make(map[geometry.Point]struct{})}
	bend := geometry.Point{X: 10, Y: 5}

	for index := 0; index < 2; index++ {
		targetNode := arena.AddNode(graphID)
		arena.SetNodeLayer(targetNode, &layerID)
		targetPort := arena.AddPort()
		arena.SetPortNode(targetPort, &targetNode)
		edgeID := arena.AddEdge()
		arena.SetEdgeSource(edgeID, &portID)
		arena.SetEdgeTarget(edgeID, &targetPort)
		router.addBendPoint(arena, edgeID, bend, true, portID)
		points, present := arena.Edge(edgeID).Properties.Raw(layered.JunctionPoints.ID)
		if index == 0 && (!present || len(points.(geometry.Chain)) != 1) {
			t.Fatal("first edge did not receive junction point")
		}
		if index == 1 && present {
			t.Fatal("duplicate position received a second junction point")
		}
	}
}
