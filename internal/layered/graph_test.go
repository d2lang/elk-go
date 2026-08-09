// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import "testing"

func TestGraphMaintainsAdjacencyAndLayerOrder(t *testing.T) {
	arena := NewLGraphArena()
	graphID := arena.AddGraph()
	first := arena.AddNode(graphID)
	second := arena.AddNode(graphID)
	source := arena.AddPort()
	target := arena.AddPort()
	arena.SetPortNode(source, &first)
	arena.SetPortNode(target, &second)
	edge := arena.AddEdge()
	arena.SetEdgeSource(edge, &source)
	arena.SetEdgeTarget(edge, &target)

	if got := arena.NodeOutgoingEdges(first); len(got) != 1 || got[0] != edge {
		t.Fatalf("outgoing edges = %v", got)
	}
	if got := arena.NodeIncomingEdges(second); len(got) != 1 || got[0] != edge {
		t.Fatalf("incoming edges = %v", got)
	}

	layer := arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = append(arena.Graph(graphID).Layers, layer)
	arena.SetNodeLayer(first, &layer)
	arena.SetNodeLayerAt(second, &layer, 0)
	if got := arena.Layer(layer).Nodes; len(got) != 2 || got[0] != second || got[1] != first {
		t.Fatalf("layer nodes = %v", got)
	}
}

func TestPortSideAnchorAndViews(t *testing.T) {
	arena := NewLGraphArena()
	graphID := arena.AddGraph()
	node := arena.AddNode(graphID)
	for _, side := range []PortSide{NorthSide, EastSide, EastSide, SouthSide, WestSide} {
		port := arena.AddPort()
		arena.Port(port).Size.X = 10
		arena.Port(port).Size.Y = 6
		arena.SetPortNode(port, &node)
		arena.SetPortSide(port, side)
	}

	east := arena.NodePortSideView(node, EastSide)
	if len(east) != 2 {
		t.Fatalf("east ports = %v", east)
	}
	if anchor := arena.Port(east[0]).Anchor; anchor.X != 10 || anchor.Y != 3 {
		t.Fatalf("east anchor = %+v", anchor)
	}
}

func TestExplicitPortSideCacheSurvivesStructuralChanges(t *testing.T) {
	arena := NewLGraphArena()
	graphID := arena.AddGraph()
	nodeID := arena.AddNode(graphID)
	addPort := func(side PortSide) LPortID {
		portID := arena.AddPort()
		arena.SetPortNode(portID, &nodeID)
		arena.SetPortSide(portID, side)
		return portID
	}

	east := addPort(EastSide)
	west := addPort(WestSide)
	arena.CachePortSides(nodeID)
	if got := arena.NodePortSideView(nodeID, WestSide); len(got) != 1 || got[0] != west {
		t.Fatalf("initial west view = %v", got)
	}

	firstNorth := addPort(NorthSide)
	secondNorth := addPort(NorthSide)
	// ELK's cachePortSides stores indices, not port identities. Port restoration
	// can structurally replace the list without recaching, so the old west
	// range intentionally observes the port now occupying its old index.
	arena.Node(nodeID).Ports = []LPortID{firstNorth, secondNorth, east, west}
	if got := arena.NodePortSideView(nodeID, WestSide); len(got) != 1 || got[0] != secondNorth {
		t.Fatalf("cached west view after structural replacement = %v", got)
	}
}

func TestEdgeRetargetAndSelfLoop(t *testing.T) {
	arena := NewLGraphArena()
	graphID := arena.AddGraph()
	first := arena.AddNode(graphID)
	second := arena.AddNode(graphID)
	firstPort := arena.AddPort()
	secondPort := arena.AddPort()
	arena.SetPortNode(firstPort, &first)
	arena.SetPortNode(secondPort, &second)
	edge := arena.AddEdge()
	arena.SetEdgeSource(edge, &firstPort)
	arena.SetEdgeTarget(edge, &secondPort)
	if arena.EdgeIsSelfLoop(edge) {
		t.Fatal("edge unexpectedly classified as self loop")
	}
	arena.SetEdgeTarget(edge, &firstPort)
	if !arena.EdgeIsSelfLoop(edge) {
		t.Fatal("retargeted edge was not classified as self loop")
	}
	if len(arena.Port(secondPort).IncomingEdges) != 0 {
		t.Fatalf("old target still has incoming edge: %v", arena.Port(secondPort).IncomingEdges)
	}
}
