// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestHyperedgeDummyMergerJoinsAdjacentParallelDummies(t *testing.T) {
	arena, graphID, source, first, second, firstTarget, secondTarget := buildHyperedgeMergerGraph(t, false)

	if err := MergeHyperedgeDummies(arena, graphID); err != nil {
		t.Fatal(err)
	}
	middleLayer := arena.Graph(graphID).Layers[1]
	if got := arena.Layer(middleLayer).Nodes; len(got) != 1 || got[0] != first {
		t.Fatalf("merged middle layer = %v, want [%v]", got, first)
	}
	inputs := arena.NodePortsOnSide(first, layered.WestSide)
	outputs := arena.NodePortsOnSide(first, layered.EastSide)
	if len(inputs) != 1 || len(arena.Port(inputs[0]).IncomingEdges) != 2 ||
		len(outputs) != 1 || len(arena.Port(outputs[0]).OutgoingEdges) != 2 {
		t.Fatalf("merged port degrees = input %v output %v", inputs, outputs)
	}
	for _, portID := range arena.Node(second).Ports {
		if len(arena.PortConnectedEdges(portID)) != 0 {
			t.Fatalf("removed dummy port %v still has edges", portID)
		}
	}
	if got, found := basegraph.TryGet(&arena.Node(first).Properties, layered.LongEdgeSource); !found || got != source {
		t.Fatalf("long-edge source = %v, %v; want %v, true", got, found, source)
	}
	if _, found := basegraph.TryGet(&arena.Node(first).Properties, layered.LongEdgeTarget); found {
		t.Fatal("different long-edge targets were retained")
	}
	if firstTarget == secondTarget {
		t.Fatal("test graph unexpectedly shares target ports")
	}
}

func TestHyperedgeDummyMergerRequiresAdjacency(t *testing.T) {
	arena, graphID, _, first, second, _, _ := buildHyperedgeMergerGraph(t, true)
	if err := (&HyperedgeDummyMerger{}).Process(arena, graphID); err != nil {
		t.Fatal(err)
	}
	middle := arena.Layer(arena.Graph(graphID).Layers[1]).Nodes
	if len(middle) != 3 || middle[0] != first || middle[2] != second {
		t.Fatalf("non-adjacent dummies were merged: %v", middle)
	}
}

func TestHyperedgeDummyMergerHonorsLabelSide(t *testing.T) {
	arena, graphID, _, first, second, _, _ := buildHyperedgeMergerGraph(t, false)
	for _, nodeID := range []layered.LNodeID{first, second} {
		basegraph.Set(&arena.Node(nodeID).Properties, layered.LongEdgeHasLabelDummies, true)
		basegraph.Set(&arena.Node(nodeID).Properties, layered.LongEdgeBeforeLabelDummy, true)
	}
	if err := MergeHyperedgeDummies(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if got := len(arena.Layer(arena.Graph(graphID).Layers[1]).Nodes); got != 1 {
		t.Fatalf("same-source dummies before labels were not merged: %d nodes", got)
	}
}

func buildHyperedgeMergerGraph(
	t *testing.T,
	separate bool,
) (*layered.LGraphArena, layered.LGraphID, layered.LPortID, layered.LNodeID,
	layered.LNodeID, layered.LPortID, layered.LPortID) {
	t.Helper()
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layers := []layered.LayerID{
		arena.AddLayerToGraph(graphID),
		arena.AddLayerToGraph(graphID),
		arena.AddLayerToGraph(graphID),
	}
	sourceNode := arena.AddNode(graphID)
	first := arena.AddNode(graphID)
	second := arena.AddNode(graphID)
	firstTargetNode := arena.AddNode(graphID)
	secondTargetNode := arena.AddNode(graphID)
	arena.Node(first).Type = layered.LongEdgeNode
	arena.Node(second).Type = layered.LongEdgeNode
	arena.SetNodeLayer(sourceNode, &layers[0])
	arena.SetNodeLayer(first, &layers[1])
	if separate {
		normal := arena.AddNode(graphID)
		arena.SetNodeLayer(normal, &layers[1])
	}
	arena.SetNodeLayer(second, &layers[1])
	arena.SetNodeLayer(firstTargetNode, &layers[2])
	arena.SetNodeLayer(secondTargetNode, &layers[2])

	source := hyperedgeMergerPort(arena, sourceNode, layered.EastSide)
	firstInput := hyperedgeMergerPort(arena, first, layered.WestSide)
	firstOutput := hyperedgeMergerPort(arena, first, layered.EastSide)
	secondInput := hyperedgeMergerPort(arena, second, layered.WestSide)
	secondOutput := hyperedgeMergerPort(arena, second, layered.EastSide)
	firstTarget := hyperedgeMergerPort(arena, firstTargetNode, layered.WestSide)
	secondTarget := hyperedgeMergerPort(arena, secondTargetNode, layered.WestSide)
	hyperedgeMergerEdge(arena, source, firstInput)
	hyperedgeMergerEdge(arena, source, secondInput)
	hyperedgeMergerEdge(arena, firstOutput, firstTarget)
	hyperedgeMergerEdge(arena, secondOutput, secondTarget)
	for _, nodeID := range []layered.LNodeID{first, second} {
		basegraph.Set(&arena.Node(nodeID).Properties, layered.LongEdgeSource, source)
	}
	basegraph.Set(&arena.Node(first).Properties, layered.LongEdgeTarget, firstTarget)
	basegraph.Set(&arena.Node(second).Properties, layered.LongEdgeTarget, secondTarget)
	return arena, graphID, source, first, second, firstTarget, secondTarget
}

func hyperedgeMergerPort(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	side layered.PortSide,
) layered.LPortID {
	portID := arena.AddPort()
	arena.SetPortNode(portID, &nodeID)
	arena.SetPortSide(portID, side)
	return portID
}

func hyperedgeMergerEdge(
	arena *layered.LGraphArena,
	source, target layered.LPortID,
) layered.LEdgeID {
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &source)
	arena.SetEdgeTarget(edgeID, &target)
	return edgeID
}
