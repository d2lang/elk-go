// Copyright (c) 2016 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestHighDegreeNodeProcessorMovesBoundedTreesBesideHub(t *testing.T) {
	arena, graphID, layers := advancedLayeringGraph(6)
	source := addAdvancedNode(arena, graphID, layers[0], "source")
	hub := addAdvancedNode(arena, graphID, layers[2], "hub")
	root := addAdvancedNode(arena, graphID, layers[4], "root")
	leaf := addAdvancedNode(arena, graphID, layers[4], "leaf")
	child := addAdvancedNode(arena, graphID, layers[5], "child")
	_ = addAdvancedNode(arena, graphID, layers[4], "anchor-4")
	_ = addAdvancedNode(arena, graphID, layers[5], "anchor-5")
	addAdvancedEdge(arena, source, hub)
	addAdvancedEdge(arena, hub, root)
	addAdvancedEdge(arena, hub, leaf)
	addAdvancedEdge(arena, root, child)
	basegraph.Set(&arena.Graph(graphID).Properties, layered.HighDegreeNodeThreshold, 3)
	basegraph.Set(&arena.Graph(graphID).Properties, layered.HighDegreeNodeTreeHeight, 2)

	ProcessHighDegreeNodes(arena, graphID)

	hubLayer := arena.LayerIndex(*arena.Node(hub).Layer)
	if got := arena.LayerIndex(*arena.Node(source).Layer); got != hubLayer-1 {
		t.Fatalf("incoming tree layer = %d, want %d", got, hubLayer-1)
	}
	for _, nodeID := range []layered.LNodeID{root, leaf} {
		if got := arena.LayerIndex(*arena.Node(nodeID).Layer); got != hubLayer+1 {
			t.Fatalf("outgoing root %d layer = %d, want %d", nodeID, got, hubLayer+1)
		}
	}
	if got := arena.LayerIndex(*arena.Node(child).Layer); got != hubLayer+2 {
		t.Fatalf("tree child layer = %d, want %d", got, hubLayer+2)
	}
}

func TestHighDegreeNodeProcessorHonorsTreeHeightBoundary(t *testing.T) {
	arena, graphID, layers := advancedLayeringGraph(4)
	hub := addAdvancedNode(arena, graphID, layers[0], "hub")
	first := addAdvancedNode(arena, graphID, layers[1], "first")
	second := addAdvancedNode(arena, graphID, layers[2], "second")
	third := addAdvancedNode(arena, graphID, layers[3], "third")
	leafA := addAdvancedNode(arena, graphID, layers[1], "leaf-a")
	leafB := addAdvancedNode(arena, graphID, layers[1], "leaf-b")
	addAdvancedEdge(arena, hub, first)
	addAdvancedEdge(arena, first, second)
	addAdvancedEdge(arena, second, third)
	addAdvancedEdge(arena, hub, leafA)
	addAdvancedEdge(arena, hub, leafB)
	basegraph.Set(&arena.Graph(graphID).Properties, layered.HighDegreeNodeThreshold, 3)
	basegraph.Set(&arena.Graph(graphID).Properties, layered.HighDegreeNodeTreeHeight, 2)
	originalFirstLayer := *arena.Node(first).Layer

	ProcessHighDegreeNodes(arena, graphID)

	if *arena.Node(first).Layer != originalFirstLayer {
		t.Fatalf("over-height tree root moved from layer %d to %d", originalFirstLayer, *arena.Node(first).Layer)
	}
}

func TestNodePromotionReducesLongEdgeDummyEstimate(t *testing.T) {
	arena, graphID, layers := advancedLayeringGraph(4)
	a := addAdvancedNode(arena, graphID, layers[0], "a")
	_ = addAdvancedNode(arena, graphID, layers[1], "filler")
	b := addAdvancedNode(arena, graphID, layers[2], "b")
	d := addAdvancedNode(arena, graphID, layers[3], "d")
	addAdvancedEdge(arena, a, d)
	addAdvancedEdge(arena, b, d)
	basegraph.Set(&arena.Graph(graphID).Properties, layered.NodePromotionStrategyOption, layered.NodePromotionNoBoundary)
	before := arena.LayerIndex(*arena.Node(a).Layer) - arena.LayerIndex(*arena.Node(d).Layer)
	if before < 0 {
		before = -before
	}

	PromoteNodes(arena, graphID)

	after := arena.LayerIndex(*arena.Node(a).Layer) - arena.LayerIndex(*arena.Node(d).Layer)
	if after < 0 {
		after = -after
	}
	if after >= before {
		t.Fatalf("long edge span = %d, want less than original %d", after, before)
	}
	for _, layerID := range arena.Graph(graphID).Layers {
		if len(arena.Layer(layerID).Nodes) == 0 {
			t.Fatalf("promotion retained empty layer %d", layerID)
		}
	}
}

func TestHorizontalPostCompactionPreservesSpacingAndRoutes(t *testing.T) {
	arena, graphID, layers := advancedLayeringGraph(2)
	left := addAdvancedNode(arena, graphID, layers[0], "left")
	right := addAdvancedNode(arena, graphID, layers[1], "right")
	arena.Node(left).Position = geometry.Point{X: 100, Y: 20}
	arena.Node(right).Position = geometry.Point{X: 240, Y: 60}
	arena.Node(left).Size = geometry.Point{X: 30, Y: 20}
	arena.Node(right).Size = geometry.Point{X: 30, Y: 20}
	edgeID := addAdvancedEdge(arena, left, right)
	source, target := *arena.Edge(edgeID).Source, *arena.Edge(edgeID).Target
	arena.SetPortSide(source, layered.EastSide)
	arena.SetPortSide(target, layered.WestSide)
	arena.Port(source).Position = geometry.Point{X: 30, Y: 10}
	arena.Port(target).Position = geometry.Point{Y: 10}
	arena.Edge(edgeID).BendPoints = geometry.Chain{{X: 170, Y: 30}, {X: 170, Y: 70}}
	basegraph.Set(&arena.Graph(graphID).Properties, layered.EdgeRoutingOption, layered.EdgeRoutingOrthogonal)
	basegraph.Set(&arena.Graph(graphID).Properties, layered.PostCompactionStrategy, layered.GraphCompactionLeft)
	basegraph.Set(&arena.Graph(graphID).Properties, layered.PostCompactionConstraints, layered.ConstraintCalculationScanline)
	originalBendX := arena.Edge(edgeID).BendPoints[0].X

	if err := CompactHorizontally(arena, graphID); err != nil {
		t.Fatal(err)
	}

	if arena.Node(right).Position.X >= 240 {
		t.Fatalf("right node x = %v, expected actual left compaction", arena.Node(right).Position.X)
	}
	if arena.Node(right).Position.X-arena.Node(left).Position.X < arena.Node(left).Size.X {
		t.Fatalf("nodes overlap after compaction: left=%v right=%v", arena.Node(left).Position, arena.Node(right).Position)
	}
	if arena.Edge(edgeID).BendPoints[0].X == originalBendX {
		t.Fatalf("vertical route x remained %v; route was not part of constraint graph", originalBendX)
	}
	if arena.Edge(edgeID).BendPoints[0].X != arena.Edge(edgeID).BendPoints[1].X {
		t.Fatalf("vertical route was skewed: %v", arena.Edge(edgeID).BendPoints)
	}
}

func advancedLayeringGraph(layerCount int) (*layered.LGraphArena, layered.LGraphID, []layered.LayerID) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layers := make([]layered.LayerID, layerCount)
	for index := range layers {
		layers[index] = arena.AddLayerToGraph(graphID)
	}
	return arena, graphID, layers
}

func addAdvancedNode(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	layerID layered.LayerID,
	name string,
) layered.LNodeID {
	nodeID := arena.AddNode(graphID)
	arena.Node(nodeID).ID = len(arena.Nodes)
	basegraph.Set(&arena.Node(nodeID).Properties, layered.OriginalElement, layered.Origin{})
	basegraph.Set(&arena.Node(nodeID).Properties, layered.ModelOrder, len(arena.Nodes))
	_ = name
	arena.SetNodeLayer(nodeID, &layerID)
	return nodeID
}

func addAdvancedEdge(
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
