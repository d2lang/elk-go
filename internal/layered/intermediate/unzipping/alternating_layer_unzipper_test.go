// Copyright (c) 2024 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package unzipping

import (
	"reflect"
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestAlternateLayersUsesLowestExplicitSplit(t *testing.T) {
	arena, graphID, layerID, nodes := unzipTestLayer(4)
	basegraph.Set(&arena.Node(nodes[0]).Properties, layered.LayerUnzippingLayerSplit, 3)
	basegraph.Set(&arena.Node(nodes[1]).Properties, layered.LayerUnzippingLayerSplit, 2)

	AlternateLayers(arena, graphID)
	if got := len(arena.Graph(graphID).Layers); got != 2 {
		t.Fatalf("layer count = %d, want 2", got)
	}
	if got, want := arena.Layer(layerID).Nodes, []layered.LNodeID{nodes[0], nodes[2]}; !reflect.DeepEqual(got, want) {
		t.Fatalf("first sublayer = %v, want %v", got, want)
	}
	secondLayer := arena.Graph(graphID).Layers[1]
	if got, want := arena.Layer(secondLayer).Nodes, []layered.LNodeID{nodes[1], nodes[3]}; !reflect.DeepEqual(got, want) {
		t.Fatalf("second sublayer = %v, want %v", got, want)
	}
}

func TestAlternateLayersResetOnLongEdges(t *testing.T) {
	tests := []struct {
		name       string
		reset      bool
		firstLayer []int
		second     []int
	}{
		{"reset", true, []int{0, 1, 3}, []int{2}},
		{"continue", false, []int{0, 2}, []int{1, 3}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			arena, graphID, layerID, nodes := unzipTestLayer(4)
			arena.Node(nodes[0]).Type = layered.LongEdgeNode
			basegraph.Set(&arena.Node(nodes[0]).Properties, layered.LayerUnzippingLayerSplit, 2)
			basegraph.Set(&arena.Node(nodes[0]).Properties, layered.LayerUnzippingResetOnLongEdges, test.reset)

			AlternateLayers(arena, graphID)
			wantFirst := unzipNodeIDs(nodes, test.firstLayer)
			if got := arena.Layer(layerID).Nodes; !reflect.DeepEqual(got, wantFirst) {
				t.Fatalf("first sublayer = %v, want %v", got, wantFirst)
			}
			wantSecond := unzipNodeIDs(nodes, test.second)
			if got := arena.Layer(arena.Graph(graphID).Layers[1]).Nodes; !reflect.DeepEqual(got, wantSecond) {
				t.Fatalf("second sublayer = %v, want %v", got, wantSecond)
			}
		})
	}
}

func TestAlternateLayersMinimizeEdgeLengthCanKeepWideLayer(t *testing.T) {
	arena, graphID, layerID, nodes := unzipTestLayer(3)
	for _, nodeID := range nodes {
		arena.Node(nodeID).Size.X = 1000
		arena.Node(nodeID).Size.Y = 10
	}
	basegraph.Set(&arena.Node(nodes[0]).Properties, layered.LayerUnzippingMinimizeEdgeLength, true)
	AlternateLayers(arena, graphID)
	if got := arena.Graph(graphID).Layers; len(got) != 1 || got[0] != layerID {
		t.Fatalf("layers = %v, want original layer only", got)
	}
}

func unzipTestLayer(count int) (*layered.LGraphArena, layered.LGraphID, layered.LayerID, []layered.LNodeID) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayerToGraph(graphID)
	nodes := make([]layered.LNodeID, count)
	for index := range nodes {
		nodes[index] = arena.AddNode(graphID)
		arena.SetNodeLayer(nodes[index], &layerID)
	}
	return arena, graphID, layerID, nodes
}

func unzipNodeIDs(nodes []layered.LNodeID, indices []int) []layered.LNodeID {
	result := make([]layered.LNodeID, len(indices))
	for index, nodeIndex := range indices {
		result[index] = nodes[nodeIndex]
	}
	return result
}
