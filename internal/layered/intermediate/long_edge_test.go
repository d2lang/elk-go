// Copyright (c) 2010, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestSplitAndJoinLongEdge(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layers := []layered.LayerID{
		arena.AddLayerToGraph(graphID), arena.AddLayerToGraph(graphID), arena.AddLayerToGraph(graphID),
	}
	source, target := arena.AddNode(graphID), arena.AddNode(graphID)
	arena.SetNodeLayer(source, &layers[0])
	arena.SetNodeLayer(target, &layers[2])
	sourcePort, targetPort := arena.AddPort(), arena.AddPort()
	arena.SetPortNode(sourcePort, &source)
	arena.SetPortNode(targetPort, &target)
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &sourcePort)
	arena.SetEdgeTarget(edgeID, &targetPort)
	labelID := arena.AddLabel("head")
	basegraph.Set(&arena.Label(labelID).Properties, layered.EdgeLabelsPlacement, layered.EdgeLabelPlacementHead)
	arena.Edge(edgeID).Labels = append(arena.Edge(edgeID).Labels, labelID)

	SplitLongEdges(arena, graphID)

	if got := len(arena.Layer(layers[1]).Nodes); got != 1 {
		t.Fatalf("middle layer nodes = %d, want 1", got)
	}
	dummyID := arena.Layer(layers[1]).Nodes[0]
	if got := arena.Node(dummyID).Type; got != layered.LongEdgeNode {
		t.Fatalf("dummy type = %v, want LONG_EDGE", got)
	}
	if got := arena.Node(dummyID).Size.Y; got != 1 {
		t.Fatalf("dummy thickness = %v, want 1", got)
	}
	if len(arena.Edge(edgeID).Labels) != 0 {
		t.Fatal("head label was not moved to final split edge")
	}
	eastPorts := arena.NodePortsOnSide(dummyID, layered.EastSide)
	if len(eastPorts) != 1 || len(arena.Port(eastPorts[0]).OutgoingEdges) != 1 {
		t.Fatalf("dummy east edges = %v", eastPorts)
	}
	droppedID := arena.Port(eastPorts[0]).OutgoingEdges[0]
	arena.Edge(edgeID).BendPoints = geometry.Chain{{X: 10, Y: 20}}
	arena.Edge(droppedID).BendPoints = geometry.Chain{{X: 30, Y: 40}}

	JoinLongEdges(arena, graphID)

	if got := len(arena.Layer(layers[1]).Nodes); got != 0 {
		t.Fatalf("middle layer nodes after join = %d, want 0", got)
	}
	if arena.Node(dummyID).Layer == nil || *arena.Node(dummyID).Layer != layers[1] {
		t.Fatal("joined dummy unexpectedly lost its layer pointer")
	}
	edge := arena.Edge(edgeID)
	if edge.Target == nil || *edge.Target != targetPort {
		t.Fatalf("joined target = %v, want %v", edge.Target, targetPort)
	}
	if len(edge.BendPoints) != 2 || edge.BendPoints[0].X != 10 || edge.BendPoints[1].X != 30 {
		t.Fatalf("joined bends = %v", edge.BendPoints)
	}
	if len(edge.Labels) != 1 || edge.Labels[0] != labelID {
		t.Fatalf("joined labels = %v, want [%v]", edge.Labels, labelID)
	}
}

func TestSplitEdgeClampsNegativeThickness(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	dummy, source, target := arena.AddNode(graphID), arena.AddNode(graphID), arena.AddNode(graphID)
	sourcePort, targetPort := arena.AddPort(), arena.AddPort()
	arena.SetPortNode(sourcePort, &source)
	arena.SetPortNode(targetPort, &target)
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &sourcePort)
	arena.SetEdgeTarget(edgeID, &targetPort)
	basegraph.Set(&arena.Edge(edgeID).Properties, layered.EdgeThickness, -4.0)

	SplitEdgeAtDummy(arena, edgeID, dummy)

	if got := basegraph.Get(&arena.Edge(edgeID).Properties, layered.EdgeThickness); got != 0 {
		t.Fatalf("edge thickness = %v, want 0", got)
	}
	if got := arena.Node(dummy).Size.Y; got != 0 {
		t.Fatalf("dummy height = %v, want 0", got)
	}
}
