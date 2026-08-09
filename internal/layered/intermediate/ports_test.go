// Copyright (c) 2011, 2018 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestSortPortListsFixedPositionClockwise(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	nodeID := arena.AddNode(graphID)
	layerID := arena.AddLayerToGraph(graphID)
	arena.SetNodeLayer(nodeID, &layerID)
	basegraph.Set(&arena.Node(nodeID).Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedPosition)

	add := func(side layered.PortSide, x, y float64) layered.LPortID {
		portID := arena.AddPort()
		arena.SetPortNode(portID, &nodeID)
		arena.SetPortSide(portID, side)
		arena.Port(portID).Position.X = x
		arena.Port(portID).Position.Y = y
		return portID
	}
	southLeft := add(layered.SouthSide, 2, 10)
	northRight := add(layered.NorthSide, 8, 0)
	westTop := add(layered.WestSide, 0, 2)
	eastTop := add(layered.EastSide, 10, 2)
	northLeft := add(layered.NorthSide, 2, 0)
	southRight := add(layered.SouthSide, 8, 10)
	westBottom := add(layered.WestSide, 0, 8)
	eastBottom := add(layered.EastSide, 10, 8)

	if err := SortPortLists(arena, graphID); err != nil {
		t.Fatal(err)
	}
	want := []layered.LPortID{northLeft, northRight, eastTop, eastBottom, southRight, southLeft, westBottom, westTop}
	for index, portID := range want {
		if got := arena.Node(nodeID).Ports[index]; got != portID {
			t.Fatalf("port %d = %d, want %d; order=%v", index, got, portID, arena.Node(nodeID).Ports)
		}
	}
}

func TestSortPortListsFixedSideMatchesELK081ShortRangeBehavior(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	nodeID := arena.AddNode(graphID)
	layerID := arena.AddLayerToGraph(graphID)
	arena.SetNodeLayer(nodeID, &layerID)
	basegraph.Set(&arena.Node(nodeID).Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedSide)

	add := func(side layered.PortSide) layered.LPortID {
		portID := arena.AddPort()
		arena.SetPortNode(portID, &nodeID)
		arena.SetPortSide(portID, side)
		return portID
	}
	westFirst, southFirst := add(layered.WestSide), add(layered.SouthSide)
	east := add(layered.EastSide)
	westSecond, southSecond := add(layered.WestSide), add(layered.SouthSide)

	if err := SortPortLists(arena, graphID); err != nil {
		t.Fatal(err)
	}
	// ELK 0.8.1 leaves two-port side ranges in model order. Its south and
	// west range reversals cancel for this exact interleaving as well.
	want := []layered.LPortID{east, southFirst, southSecond, westFirst, westSecond}
	for index, portID := range want {
		if got := arena.Node(nodeID).Ports[index]; got != portID {
			t.Fatalf("port %d = %d, want %d; order=%v", index, got, portID, arena.Node(nodeID).Ports)
		}
	}
}

func TestReversePortSideMatchesELK081ExclusiveUpperBound(t *testing.T) {
	arena := layered.NewLGraphArena()
	ports := make([]layered.LPortID, 5)
	for index := range ports {
		ports[index] = arena.AddPort()
		arena.SetPortSide(ports[index], layered.WestSide)
	}
	modelOrder := append([]layered.LPortID(nil), ports...)

	reversePortSide(arena, ports, layered.WestSide)
	want := []layered.LPortID{modelOrder[3], modelOrder[2], modelOrder[1], modelOrder[0], modelOrder[4]}
	for index, portID := range want {
		if got := ports[index]; got != portID {
			t.Fatalf("port %d = %d, want %d; order=%v", index, got, portID, ports)
		}
	}
}

func TestSortPortListsPortDegree(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayerToGraph(graphID)
	basegraph.Set(&arena.Graph(graphID).Properties, layered.PortSortingStrategyOption, layered.PortSortingPortDegree)

	addNode := func() layered.LNodeID {
		nodeID := arena.AddNode(graphID)
		arena.SetNodeLayer(nodeID, &layerID)
		basegraph.Set(&arena.Node(nodeID).Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedSide)
		return nodeID
	}
	addPort := func(nodeID layered.LNodeID, side layered.PortSide) layered.LPortID {
		portID := arena.AddPort()
		arena.SetPortNode(portID, &nodeID)
		arena.SetPortSide(portID, side)
		return portID
	}
	addEdge := func(source, target layered.LPortID, reversed bool) {
		edgeID := arena.AddEdge()
		arena.SetEdgeSource(edgeID, &source)
		arena.SetEdgeTarget(edgeID, &target)
		basegraph.Set(&arena.Edge(edgeID).Properties, layered.Reversed, reversed)
	}

	source := addNode()
	eastLow := addPort(source, layered.EastSide)
	eastHigh := addPort(source, layered.EastSide)
	target := addNode()
	westHigh := addPort(target, layered.WestSide)
	westLow := addPort(target, layered.WestSide)

	addEdge(eastHigh, westHigh, false)
	addEdge(eastHigh, westHigh, false)
	addEdge(eastLow, westLow, false)
	// Reversed edges are excluded from the original-model degree.
	addEdge(eastLow, westLow, true)

	if err := SortPortLists(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if got := arena.Node(source).Ports; len(got) != 2 || got[0] != eastHigh || got[1] != eastLow {
		t.Fatalf("east port order = %v, want high-degree then low-degree", got)
	}
	if got := arena.Node(target).Ports; len(got) != 2 || got[0] != westLow || got[1] != westHigh {
		t.Fatalf("west port order = %v, want low-degree then high-degree", got)
	}
}
