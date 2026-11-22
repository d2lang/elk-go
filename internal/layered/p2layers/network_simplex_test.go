// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package p2layers

import (
	"testing"

	"github.com/d2lang/elk-go/internal/layered"
)

func TestNetworkSimplexLayersChain(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	nodes := []layered.LNodeID{arena.AddNode(graphID), arena.AddNode(graphID), arena.AddNode(graphID)}
	arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, nodes...)
	ports := make([]layered.LPortID, len(nodes))
	for index, nodeID := range nodes {
		ports[index] = arena.AddPort()
		arena.SetPortNode(ports[index], &nodeID)
	}
	for index := 0; index+1 < len(nodes); index++ {
		edge := arena.AddEdge()
		source, target := ports[index], ports[index+1]
		arena.SetEdgeSource(edge, &source)
		arena.SetEdgeTarget(edge, &target)
	}
	if err := (&NetworkSimplexLayerer{}).Process(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if len(arena.Graph(graphID).LayerlessNodes) != 0 {
		t.Fatal("layerless list was not cleared")
	}
	if len(arena.Graph(graphID).Layers) != 3 {
		t.Fatalf("layer count = %d, want 3", len(arena.Graph(graphID).Layers))
	}
	for index, nodeID := range nodes {
		if got := arena.NodeIndexInLayer(nodeID); got != 0 {
			t.Fatalf("node %d position in layer = %d", index, got)
		}
		if got := arena.LayerIndex(*arena.Node(nodeID).Layer); got != index {
			t.Fatalf("node %d layer = %d", index, got)
		}
	}
}

func TestConnectedComponentsPutsLargestFirst(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	nodes := []layered.LNodeID{arena.AddNode(graphID), arena.AddNode(graphID), arena.AddNode(graphID)}
	ports := make([]layered.LPortID, len(nodes))
	for index, nodeID := range nodes {
		ports[index] = arena.AddPort()
		arena.SetPortNode(ports[index], &nodeID)
	}
	edge := arena.AddEdge()
	arena.SetEdgeSource(edge, &ports[1])
	arena.SetEdgeTarget(edge, &ports[2])
	components := connectedComponents(arena, nodes)
	if len(components) != 2 || len(components[0]) != 2 {
		t.Fatalf("components = %v", components)
	}
}

func connectedComponents(arena *layered.LGraphArena, nodes []layered.LNodeID) [][]layered.LNodeID {
	return connectedComponentsInto(arena, nodes, &layeringScratch{})
}
