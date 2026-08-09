// Copyright (c) 2019 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"reflect"
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/layered/intermediate/loops"
)

func TestSelfLoopPreprocessorHidesOnlyPureLoopPorts(t *testing.T) {
	for _, constraints := range []layered.PortConstraints{
		layered.PortConstraintsFree,
		layered.PortConstraintsFixedSide,
	} {
		arena, graphID, nodeID, regular, unconnected, _ := preprocessorGraph(t, constraints)
		if err := PreprocessSelfLoops(arena, graphID); err != nil {
			t.Fatal(err)
		}
		if got, want := arena.Node(nodeID).Ports, append([]layered.LPortID{regular}, unconnected...); !reflect.DeepEqual(got, want) {
			t.Fatalf("remaining ports for constraints %v = %v, want %v", constraints, got, want)
		}
		assertNoAttachedSelfLoops(t, arena, nodeID)
	}
}

func TestSelfLoopPreprocessorKeepsFixedOrderPorts(t *testing.T) {
	for _, constraints := range []layered.PortConstraints{
		layered.PortConstraintsFixedOrder,
		layered.PortConstraintsFixedRatio,
		layered.PortConstraintsFixedPosition,
	} {
		arena, graphID, nodeID, _, _, all := preprocessorGraph(t, constraints)
		if err := PreprocessSelfLoops(arena, graphID); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(arena.Node(nodeID).Ports, all) {
			t.Fatalf("fixed-order ports changed for constraints %v", constraints)
		}
		assertNoAttachedSelfLoops(t, arena, nodeID)
	}
}

func TestSelfLoopEqualDistributionUsesUpstreamTargetSequence(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	nodeID := arena.AddNode(graphID)
	arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, nodeID)
	basegraph.Set(&arena.Node(nodeID).Properties, layered.PortConstraintsOption, layered.PortConstraintsFree)
	basegraph.Set(&arena.Node(nodeID).Properties, layered.OriginalPortConstraints, layered.PortConstraintsFree)
	basegraph.Set(&arena.Node(nodeID).Properties, layered.SelfLoopDistribution, layered.SelfLoopDistributionEqually)

	for range 8 {
		source := addSelfLoopTestPort(arena, nodeID, layered.UndefinedSide, geometry.Point{})
		target := addSelfLoopTestPort(arena, nodeID, layered.UndefinedSide, geometry.Point{})
		edgeID := arena.AddEdge()
		arena.SetEdgeSource(edgeID, &source)
		arena.SetEdgeTarget(edgeID, &target)
	}
	if err := PreprocessSelfLoops(arena, graphID); err != nil {
		t.Fatal(err)
	}
	layerID := arena.AddLayerToGraph(graphID)
	arena.Graph(graphID).LayerlessNodes = nil
	arena.SetNodeLayer(nodeID, &layerID)
	if err := RestoreSelfLoopPorts(arena, graphID); err != nil {
		t.Fatal(err)
	}

	holder := basegraph.Get(&arena.Node(nodeID).Properties, loops.HolderProperty)
	want := [][2]layered.PortSide{
		{layered.NorthSide, layered.NorthSide},
		{layered.SouthSide, layered.SouthSide},
		{layered.EastSide, layered.EastSide},
		{layered.WestSide, layered.WestSide},
		{layered.WestSide, layered.NorthSide},
		{layered.NorthSide, layered.EastSide},
		{layered.SouthSide, layered.WestSide},
		{layered.EastSide, layered.SouthSide},
	}
	for index, loop := range holder.HyperLoops {
		got := [2]layered.PortSide{
			arena.Port(loop.Ports[0].LPort).Side,
			arena.Port(loop.Ports[1].LPort).Side,
		}
		if got != want[index] {
			t.Fatalf("loop %d target sides = %v, want %v", index, got, want[index])
		}
	}
}

func TestOrthogonalSelfLoopChainRoutesAndRestoresCoordinates(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	graph := arena.Graph(graphID)
	basegraph.Set(&graph.Properties, layered.DirectionOption, layered.DirectionRight)
	basegraph.Set(&graph.Properties, layered.EdgeRoutingOption, layered.EdgeRoutingOrthogonal)
	basegraph.Set(&graph.Properties, layered.Random, javacompat.NewRandom(1))
	nodeID := arena.AddNode(graphID)
	graph.LayerlessNodes = append(graph.LayerlessNodes, nodeID)
	node := arena.Node(nodeID)
	node.Size = geometry.Point{X: 50, Y: 50}
	basegraph.Set(&node.Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedOrder)
	basegraph.Set(&node.Properties, layered.OriginalPortConstraints, layered.PortConstraintsFixedOrder)
	source := addSelfLoopTestPort(arena, nodeID, layered.NorthSide, geometry.Point{X: 10})
	target := addSelfLoopTestPort(arena, nodeID, layered.NorthSide, geometry.Point{X: 40})
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &source)
	arena.SetEdgeTarget(edgeID, &target)
	labelID := arena.AddLabel("loop")
	arena.Label(labelID).Size = geometry.Point{X: 20, Y: 5}
	arena.Edge(edgeID).Labels = append(arena.Edge(edgeID).Labels, labelID)

	if err := PreprocessSelfLoops(arena, graphID); err != nil {
		t.Fatal(err)
	}
	layerID := arena.AddLayerToGraph(graphID)
	graph.LayerlessNodes = nil
	arena.SetNodeLayer(nodeID, &layerID)
	node.Position = geometry.Point{X: 100, Y: 200}
	if err := RestoreSelfLoopPorts(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if err := RouteSelfLoops(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if got, want := arena.Edge(edgeID).BendPoints, (geometry.Chain{{X: 10, Y: -10}, {X: 40, Y: -10}}); !reflect.DeepEqual(got, want) {
		t.Fatalf("node-local bends = %v, want %v", got, want)
	}
	if node.Margin.Top != 17 {
		t.Fatalf("top margin = %v, want 17", node.Margin.Top)
	}
	PostprocessSelfLoops(arena, graphID)
	if arena.Edge(edgeID).Source == nil || *arena.Edge(edgeID).Source != source ||
		arena.Edge(edgeID).Target == nil || *arena.Edge(edgeID).Target != target {
		t.Fatal("self-loop endpoints were not restored")
	}
	if got, want := arena.Edge(edgeID).BendPoints, (geometry.Chain{{X: 110, Y: 190}, {X: 140, Y: 190}}); !reflect.DeepEqual(got, want) {
		t.Fatalf("absolute bends = %v, want %v", got, want)
	}
	if got, want := arena.Label(labelID).Position, (geometry.Point{X: 115, Y: 183}); got != want {
		t.Fatalf("label position = %v, want %v", got, want)
	}
}

func preprocessorGraph(
	t *testing.T,
	constraints layered.PortConstraints,
) (*layered.LGraphArena, layered.LGraphID, layered.LNodeID, layered.LPortID, []layered.LPortID, []layered.LPortID) {
	t.Helper()
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	first := arena.AddNode(graphID)
	second := arena.AddNode(graphID)
	arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, first, second)
	firstPort := addSelfLoopTestPort(arena, first, layered.EastSide, geometry.Point{})
	regular := addSelfLoopTestPort(arena, second, layered.WestSide, geometry.Point{})
	regularEdge := arena.AddEdge()
	arena.SetEdgeSource(regularEdge, &firstPort)
	arena.SetEdgeTarget(regularEdge, &regular)
	for _, sideCount := range []struct {
		side  layered.PortSide
		count int
	}{{layered.NorthSide, 2}, {layered.EastSide, 2}, {layered.SouthSide, 2}, {layered.WestSide, 1}} {
		for range sideCount.count {
			addSelfLoopTestPort(arena, second, sideCount.side, geometry.Point{})
		}
	}
	all := append([]layered.LPortID(nil), arena.Node(second).Ports...)
	for _, endpoints := range [][2]int{{0, 1}, {1, 2}, {3, 4}, {4, 5}} {
		edgeID := arena.AddEdge()
		source, target := all[endpoints[0]], all[endpoints[1]]
		arena.SetEdgeSource(edgeID, &source)
		arena.SetEdgeTarget(edgeID, &target)
	}
	basegraph.Set(&arena.Node(second).Properties, layered.PortConstraintsOption, constraints)
	return arena, graphID, second, regular, []layered.LPortID{all[6], all[7]}, all
}

func addSelfLoopTestPort(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	side layered.PortSide,
	position geometry.Point,
) layered.LPortID {
	portID := arena.AddPort()
	arena.SetPortNode(portID, &nodeID)
	arena.SetPortSide(portID, side)
	arena.Port(portID).Position = position
	return portID
}

func assertNoAttachedSelfLoops(t *testing.T, arena *layered.LGraphArena, nodeID layered.LNodeID) {
	t.Helper()
	for _, edgeID := range arena.NodeConnectedEdges(nodeID) {
		if arena.EdgeIsSelfLoop(edgeID) {
			t.Fatalf("self loop %d remains attached", edgeID)
		}
	}
	holder, found := basegraph.TryGet(&arena.Node(nodeID).Properties, loops.HolderProperty)
	if !found || len(holder.HyperLoops) == 0 {
		t.Fatal("self-loop holder missing")
	}
}
