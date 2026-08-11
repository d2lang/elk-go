// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"testing"

	"github.com/d2lang/elk-go/internal/core"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestPartitionPreprocessorReversesContradictingEdges(t *testing.T) {
	arena, graphID := partitionTestGraph(2, 1)
	source, target := arena.Graph(graphID).LayerlessNodes[0], arena.Graph(graphID).LayerlessNodes[1]
	edgeID := addPartitionTestEdge(arena, source, target)
	basegraph.Set(&arena.Edge(edgeID).Properties, layered.PriorityDirection, 7)

	PreprocessPartitions(arena, graphID)

	if got := arena.EdgeSourceNode(edgeID); got != target {
		t.Fatalf("edge source = %v, want lower-partition node %v", got, target)
	}
	if got := arena.EdgeTargetNode(edgeID); got != source {
		t.Fatalf("edge target = %v, want higher-partition node %v", got, source)
	}
	if !basegraph.Get(&arena.Edge(edgeID).Properties, layered.Reversed) {
		t.Fatal("reversed marker was not set")
	}
	if got := basegraph.Get(&arena.Edge(edgeID).Properties, layered.PriorityDirection); got != 1007 {
		t.Fatalf("direction priority = %d, want 1007", got)
	}
}

func TestPartitionPreprocessorReversesEdgeLeadingThroughUnpartitionedNodes(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	high := arena.AddNode(graphID)
	bridgeOne := arena.AddNode(graphID)
	bridgeTwo := arena.AddNode(graphID)
	low := arena.AddNode(graphID)
	arena.Graph(graphID).LayerlessNodes = []layered.LNodeID{high, bridgeOne, bridgeTwo, low}
	basegraph.Set(&arena.Node(high).Properties, core.PartitioningPartition, 2)
	basegraph.Set(&arena.Node(low).Properties, core.PartitioningPartition, 0)

	first := addPartitionTestEdge(arena, high, bridgeOne)
	addPartitionTestEdge(arena, bridgeOne, bridgeTwo)
	addPartitionTestEdge(arena, bridgeTwo, low)

	PreprocessPartitions(arena, graphID)
	if source, target := arena.EdgeSourceNode(first), arena.EdgeTargetNode(first); source != bridgeOne || target != high {
		t.Fatalf("first path edge = %d -> %d, want %d -> %d", source, target, bridgeOne, high)
	}
	if got := basegraph.Get(&arena.Edge(first).Properties, layered.PriorityDirection); got != partitionConstraintEdgePriority {
		t.Fatalf("reversed path-edge priority = %d, want %d", got, partitionConstraintEdgePriority)
	}
}

func TestPartitionMidprocessorConnectsConsecutivePartitions(t *testing.T) {
	arena, graphID := partitionTestGraph(0, 0, 4, 4, 9)

	ProcessPartitionsBeforeLayering(arena, graphID)

	if got := len(arena.Edges); got != 6 {
		t.Fatalf("constraint edge count = %d, want 6", got)
	}
	if got := len(arena.Ports); got != 10 {
		t.Fatalf("constraint port count = %d, want 10", got)
	}
	for index := range arena.Edges {
		edgeID := layered.LEdgeID(index)
		if !basegraph.Get(&arena.Edge(edgeID).Properties, layered.PartitionDummy) {
			t.Fatalf("edge %d is not marked as a partition dummy", index)
		}
		sourcePartition := basegraph.Get(
			&arena.Node(arena.EdgeSourceNode(edgeID)).Properties, core.PartitioningPartition)
		targetPartition := basegraph.Get(
			&arena.Node(arena.EdgeTargetNode(edgeID)).Properties, core.PartitioningPartition)
		if !((sourcePartition == 0 && targetPartition == 4) ||
			(sourcePartition == 4 && targetPartition == 9)) {
			t.Fatalf("edge %d connects partitions %d -> %d", index, sourcePartition, targetPartition)
		}
	}
	for index := range arena.Ports {
		portID := layered.LPortID(index)
		if !basegraph.Get(&arena.Port(portID).Properties, layered.PartitionDummy) {
			t.Fatalf("port %d is not marked as a partition dummy", index)
		}
	}
}

func TestPartitionPostprocessorDetachesDummyPorts(t *testing.T) {
	arena, graphID := partitionTestGraph(0, 1)
	nodes := append([]layered.LNodeID(nil), arena.Graph(graphID).LayerlessNodes...)
	originalPort := arena.AddPort()
	arena.SetPortNode(originalPort, &nodes[0])
	ProcessPartitionsBeforeLayering(arena, graphID)

	layerID := arena.AddLayerToGraph(graphID)
	for _, nodeID := range nodes {
		arena.SetNodeLayer(nodeID, &layerID)
	}
	arena.Graph(graphID).LayerlessNodes = nil
	PostprocessPartitions(arena, graphID)

	if got := arena.Node(nodes[0]).Ports; len(got) != 1 || got[0] != originalPort {
		t.Fatalf("first node ports = %v, want only original port %v", got, originalPort)
	}
	if got := arena.Node(nodes[1]).Ports; len(got) != 0 {
		t.Fatalf("second node still has ports %v", got)
	}
	for index := 1; index < len(arena.Ports); index++ {
		if arena.Port(layered.LPortID(index)).Node != nil {
			t.Fatalf("dummy port %d is still attached", index)
		}
	}
}

func partitionTestGraph(partitions ...int) (*layered.LGraphArena, layered.LGraphID) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	for _, partition := range partitions {
		nodeID := arena.AddNode(graphID)
		basegraph.Set(&arena.Node(nodeID).Properties, core.PartitioningPartition, partition)
		arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, nodeID)
	}
	return arena, graphID
}

func addPartitionTestEdge(
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
