// Copyright (c) 2019 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package loops

import (
	"testing"

	"github.com/d2lang/elk-go/internal/layered"
)

func TestHolderBasicSelfLoopsMatchesUpstreamShape(t *testing.T) {
	arena, graphID, nodeID := basicSelfLoopGraph(t)
	if !NeedsProcessing(arena, nodeID) {
		t.Fatal("normal node with self loops was not detected")
	}
	holder, err := Install(arena, nodeID)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(holder.Ports), len(arena.Node(nodeID).Ports); got != want {
		t.Fatalf("represented ports = %d, want %d", got, want)
	}
	if got, want := len(holder.HyperLoops), len(arena.Node(nodeID).Ports)/2; got != want {
		t.Fatalf("hyper loops = %d, want %d", got, want)
	}
	if arena.Graph(graphID).LayerlessNodes[1] != nodeID {
		t.Fatal("test graph creation order changed")
	}
}

func TestHolderSeparatesDisconnectedHyperLoops(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	nodeID := arena.AddNode(graphID)
	arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, nodeID)
	ports := addPorts(arena, nodeID, 4, layered.NorthSide)
	first := addEdge(arena, ports[0], ports[1])
	second := addEdge(arena, ports[2], ports[3])
	holder, err := Install(arena, nodeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(holder.HyperLoops) != 2 {
		t.Fatalf("hyper loops = %d, want 2", len(holder.HyperLoops))
	}
	if holder.HyperLoops[0].Edges[0].LEdge != first || holder.HyperLoops[1].Edges[0].LEdge != second {
		t.Fatalf("hyper-loop creation order does not follow port/edge insertion order")
	}
}

func TestTypeFromPortSides(t *testing.T) {
	tests := []struct {
		sides []layered.PortSide
		want  Type
	}{
		{[]layered.PortSide{layered.NorthSide}, OneSide},
		{[]layered.PortSide{layered.NorthSide, layered.EastSide}, TwoSidesCorner},
		{[]layered.PortSide{layered.NorthSide, layered.SouthSide}, TwoSidesOpposing},
		{[]layered.PortSide{layered.NorthSide, layered.EastSide, layered.SouthSide}, ThreeSides},
		{[]layered.PortSide{layered.NorthSide, layered.EastSide, layered.SouthSide, layered.WestSide}, FourSides},
	}
	for _, test := range tests {
		var sides [5]bool
		for _, side := range test.sides {
			sides[side] = true
		}
		got, err := TypeFromPortSides(sides)
		if err != nil || got != test.want {
			t.Fatalf("TypeFromPortSides(%v) = %v, %v; want %v", test.sides, got, err, test.want)
		}
	}
}

func basicSelfLoopGraph(t *testing.T) (*layered.LGraphArena, layered.LGraphID, layered.LNodeID) {
	t.Helper()
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	firstNode := arena.AddNode(graphID)
	arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, firstNode)
	firstPort := addPorts(arena, firstNode, 1, layered.EastSide)[0]
	secondNode := arena.AddNode(graphID)
	arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, secondNode)
	secondPort := addPorts(arena, secondNode, 1, layered.WestSide)[0]
	addEdge(arena, firstPort, secondPort)
	addPorts(arena, secondNode, 2, layered.NorthSide)
	addPorts(arena, secondNode, 2, layered.EastSide)
	addPorts(arena, secondNode, 2, layered.SouthSide)
	addPorts(arena, secondNode, 1, layered.WestSide)
	ports := arena.Node(secondNode).Ports
	for index := 1; index < len(ports); index += 2 {
		addEdge(arena, ports[index-1], ports[index])
	}
	return arena, graphID, secondNode
}

func addPorts(arena *layered.LGraphArena, nodeID layered.LNodeID, count int, side layered.PortSide) []layered.LPortID {
	result := make([]layered.LPortID, 0, count)
	for range count {
		portID := arena.AddPort()
		arena.SetPortNode(portID, &nodeID)
		arena.SetPortSide(portID, side)
		result = append(result, portID)
	}
	return result
}

func addEdge(arena *layered.LGraphArena, source, target layered.LPortID) layered.LEdgeID {
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &source)
	arena.SetEdgeTarget(edgeID, &target)
	return edgeID
}
