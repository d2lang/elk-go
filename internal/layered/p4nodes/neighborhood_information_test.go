// Copyright (c) 2015 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"reflect"
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestBuildNeighborhoodInformationFiltersPriorityAndSorts(t *testing.T) {
	arena, graphID, layers := newPlacementGraph(2)
	a := addPlacementNode(arena, graphID, layers[0], 10)
	b := addPlacementNode(arena, graphID, layers[0], 10)
	c := addPlacementNode(arena, graphID, layers[1], 10)
	low := connectPlacementNodes(arena, a, c)
	high := connectPlacementNodes(arena, b, c)
	basegraph.Set(&arena.Edge(low).Properties, layered.PriorityStraightness, 1)
	basegraph.Set(&arena.Edge(high).Properties, layered.PriorityStraightness, 2)

	info := BuildNeighborhoodInformation(arena, graphID)
	if info.NodeCount != 3 {
		t.Fatalf("NodeCount = %d", info.NodeCount)
	}
	want := []Neighbor{{Node: b, Edge: high}}
	if got := info.LeftNeighbors[arena.Node(c).ID]; !reflect.DeepEqual(got, want) {
		t.Fatalf("left neighbors = %#v, want %#v", got, want)
	}
	if info.NodeIndex[arena.Node(a).ID] != 0 || info.NodeIndex[arena.Node(b).ID] != 1 {
		t.Fatalf("node indices = %v", info.NodeIndex)
	}
}

func newPlacementGraph(layerCount int) (*layered.LGraphArena, layered.LGraphID, []layered.LayerID) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layers := make([]layered.LayerID, layerCount)
	for index := range layers {
		layers[index] = arena.AddLayer(graphID)
	}
	arena.Graph(graphID).Layers = append([]layered.LayerID(nil), layers...)
	return arena, graphID, layers
}

func addPlacementNode(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	layerID layered.LayerID,
	height float64,
) layered.LNodeID {
	nodeID := arena.AddNode(graphID)
	arena.Node(nodeID).Size.Y = height
	arena.SetNodeLayer(nodeID, &layerID)
	return nodeID
}

func connectPlacementNodes(
	arena *layered.LGraphArena,
	sourceNode, targetNode layered.LNodeID,
) layered.LEdgeID {
	sourcePort := arena.AddPort()
	targetPort := arena.AddPort()
	arena.SetPortNode(sourcePort, &sourceNode)
	arena.SetPortNode(targetPort, &targetNode)
	arena.SetPortSide(sourcePort, layered.EastSide)
	arena.SetPortSide(targetPort, layered.WestSide)
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &sourcePort)
	arena.SetEdgeTarget(edgeID, &targetPort)
	return edgeID
}
