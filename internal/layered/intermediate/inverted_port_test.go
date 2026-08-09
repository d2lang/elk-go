// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestProcessInvertedWestOutputPort(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	firstLayer := arena.AddLayerToGraph(graphID)
	secondLayer := arena.AddLayerToGraph(graphID)
	source := arena.AddNode(graphID)
	target := arena.AddNode(graphID)
	basegraph.Set(&arena.Node(source).Properties, layered.PortConstraintsOption,
		layered.PortConstraintsFixedSide)
	basegraph.Set(&arena.Node(target).Properties, layered.PortConstraintsOption,
		layered.PortConstraintsFixedSide)
	arena.SetNodeLayer(source, &firstLayer)
	arena.SetNodeLayer(target, &secondLayer)
	west := arena.AddPort()
	targetPort := arena.AddPort()
	arena.SetPortNode(west, &source)
	arena.SetPortSide(west, layered.WestSide)
	arena.SetPortNode(targetPort, &target)
	arena.SetPortSide(targetPort, layered.WestSide)
	edge := arena.AddEdge()
	arena.SetEdgeSource(edge, &west)
	arena.SetEdgeTarget(edge, &targetPort)
	label := arena.AddLabel("head")
	basegraph.Set(&arena.Label(label).Properties, layered.EdgeLabelsPlacement,
		layered.EdgeLabelPlacementHead)
	arena.Edge(edge).Labels = append(arena.Edge(edge).Labels, label)

	if err := ProcessInvertedPorts(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if len(arena.Layer(firstLayer).Nodes) != 2 {
		t.Fatalf("first layer nodes = %v", arena.Layer(firstLayer).Nodes)
	}
	dummy := arena.Layer(firstLayer).Nodes[1]
	if arena.Node(dummy).Type != layered.LongEdgeNode || len(arena.Node(dummy).Ports) != 2 {
		t.Fatalf("dummy = %#v", arena.Node(dummy))
	}
	if arena.EdgeTargetNode(edge) != dummy {
		t.Fatalf("original target node = %v", arena.EdgeTargetNode(edge))
	}
	if len(arena.Edges) != 2 || arena.EdgeTargetNode(1) != target {
		t.Fatalf("dummy edge target = %v", arena.EdgeTargetNode(1))
	}
	if len(arena.Edge(edge).Labels) != 0 || len(arena.Edge(1).Labels) != 1 ||
		arena.Edge(1).Labels[0] != label {
		t.Fatalf("head label split = original %v dummy %v", arena.Edge(edge).Labels, arena.Edge(1).Labels)
	}
	if got := basegraph.Get(&arena.Node(dummy).Properties, layered.LongEdgeSource); got != west {
		t.Fatalf("long edge source = %v", got)
	}
	if got := basegraph.Get(&arena.Node(dummy).Properties, layered.LongEdgeTarget); got != targetPort {
		t.Fatalf("long edge target = %v", got)
	}
}

func TestProcessInvertedEastInputPort(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	firstLayer := arena.AddLayerToGraph(graphID)
	secondLayer := arena.AddLayerToGraph(graphID)
	source := arena.AddNode(graphID)
	target := arena.AddNode(graphID)
	basegraph.Set(&arena.Node(target).Properties, layered.PortConstraintsOption,
		layered.PortConstraintsFixedSide)
	arena.SetNodeLayer(source, &firstLayer)
	arena.SetNodeLayer(target, &secondLayer)
	sourcePort := arena.AddPort()
	east := arena.AddPort()
	arena.SetPortNode(sourcePort, &source)
	arena.SetPortSide(sourcePort, layered.EastSide)
	arena.SetPortNode(east, &target)
	arena.SetPortSide(east, layered.EastSide)
	edge := arena.AddEdge()
	arena.SetEdgeSource(edge, &sourcePort)
	arena.SetEdgeTarget(edge, &east)

	if err := ProcessInvertedPorts(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if len(arena.Layer(secondLayer).Nodes) != 2 {
		t.Fatalf("second layer nodes = %v", arena.Layer(secondLayer).Nodes)
	}
	dummy := arena.Layer(secondLayer).Nodes[1]
	if arena.EdgeTargetNode(edge) != dummy || len(arena.Edges) != 2 || arena.EdgeTargetNode(1) != target {
		t.Fatalf("east input split did not preserve target")
	}
}
