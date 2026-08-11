// Copyright (c) 2016, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"sort"

	"github.com/d2lang/elk-go/internal/core"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

const partitionConstraintEdgePriority = 1000

// PreprocessPartitions reverses edges from higher-indexed to lower-indexed
// partitions before cycle breaking. The elevated direction priority mirrors
// ELK's safeguard against the cycle breaker reversing the edge again.
func PreprocessPartitions(arena *layered.LGraphArena, graphID layered.LGraphID) {
	partitionedNodes := make([]layered.LNodeID, 0)
	for _, nodeID := range arena.Graph(graphID).LayerlessNodes {
		if basegraph.Has(&arena.Node(nodeID).Properties, core.PartitioningPartition) {
			partitionedNodes = append(partitionedNodes, nodeID)
		}
	}
	var reverse []layered.LEdgeID
	for _, sourceNodeID := range arena.Graph(graphID).LayerlessNodes {
		sourceNode := arena.Node(sourceNodeID)
		sourcePartition, sourcePartitioned := basegraph.TryGet(
			&sourceNode.Properties, core.PartitioningPartition)
		if !sourcePartitioned {
			continue
		}
		for _, edgeID := range arena.NodeOutgoingEdges(sourceNodeID) {
			edge := arena.Edge(edgeID)
			if edge.Target == nil || arena.Port(*edge.Target).Node == nil {
				continue
			}
			targetNode := arena.Node(*arena.Port(*edge.Target).Node)
			targetPartition, targetPartitioned := basegraph.TryGet(
				&targetNode.Properties, core.PartitioningPartition)
			if targetPartitioned && sourcePartition > targetPartition ||
				!targetPartitioned && reachesLowerPartition(
					arena, sourceNodeID, sourcePartition, partitionedNodes) {
				reverse = append(reverse, edgeID)
			}
		}
	}
	for _, edgeID := range reverse {
		arena.ReverseEdge(graphID, edgeID, true)
		priority := partitionConstraintEdgePriority
		if existing, present := basegraph.TryGet(
			&arena.Edge(edgeID).Properties, layered.PriorityDirection); present {
			priority += existing
		}
		basegraph.Set(&arena.Edge(edgeID).Properties, layered.PriorityDirection, priority)
	}
}

// reachesLowerPartition ports ELK 0.12's breadth-first search through nodes
// without a partition. The source edge is reversed when any reachable
// partitioned node has a lower partition ID; the remainder of that path is
// deliberately left to the ordinary cycle-breaking/layering pipeline.
func reachesLowerPartition(
	arena *layered.LGraphArena,
	source layered.LNodeID,
	sourcePartition int,
	partitionedNodes []layered.LNodeID,
) bool {
	lowerPartitions := make(map[layered.LNodeID]struct{})
	for _, nodeID := range partitionedNodes {
		partition := basegraph.Get(&arena.Node(nodeID).Properties, core.PartitioningPartition)
		if partition < sourcePartition {
			lowerPartitions[nodeID] = struct{}{}
		}
	}
	queue := []layered.LNodeID{source}
	visited := map[layered.LNodeID]struct{}{source: {}}
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		if _, found := lowerPartitions[current]; found {
			return true
		}
		for _, edgeID := range arena.NodeOutgoingEdges(current) {
			target := arena.EdgeTargetNode(edgeID)
			if _, found := visited[target]; found {
				continue
			}
			visited[target] = struct{}{}
			queue = append(queue, target)
		}
	}
	return false
}

// ProcessPartitionsBeforeLayering connects every node in a partition to every
// node in the next partition through disposable constraint ports and edges.
func ProcessPartitionsBeforeLayering(arena *layered.LGraphArena, graphID layered.LGraphID) {
	partitionNodes := make(map[int][]layered.LNodeID)
	partitionIDs := make([]int, 0)
	for _, nodeID := range arena.Graph(graphID).LayerlessNodes {
		partition, present := basegraph.TryGet(
			&arena.Node(nodeID).Properties, core.PartitioningPartition)
		if !present {
			continue
		}
		if _, seen := partitionNodes[partition]; !seen {
			partitionIDs = append(partitionIDs, partition)
		}
		partitionNodes[partition] = append(partitionNodes[partition], nodeID)
	}
	if len(partitionIDs) < 2 {
		return
	}
	sort.Ints(partitionIDs)
	for index := 0; index+1 < len(partitionIDs); index++ {
		connectPartitions(arena,
			partitionNodes[partitionIDs[index]],
			partitionNodes[partitionIDs[index+1]])
	}
}

func connectPartitions(
	arena *layered.LGraphArena,
	first, second []layered.LNodeID,
) {
	for _, sourceNodeID := range first {
		sourcePortID := arena.AddPort()
		arena.SetPortNode(sourcePortID, &sourceNodeID)
		arena.SetPortSide(sourcePortID, layered.EastSide)
		basegraph.Set(&arena.Port(sourcePortID).Properties, layered.PartitionDummy, true)

		for _, targetNodeID := range second {
			targetPortID := arena.AddPort()
			arena.SetPortNode(targetPortID, &targetNodeID)
			arena.SetPortSide(targetPortID, layered.WestSide)
			basegraph.Set(&arena.Port(targetPortID).Properties, layered.PartitionDummy, true)

			edgeID := arena.AddEdge()
			basegraph.Set(&arena.Edge(edgeID).Properties, layered.PartitionDummy, true)
			arena.SetEdgeSource(edgeID, &sourcePortID)
			arena.SetEdgeTarget(edgeID, &targetPortID)
		}
	}
}

// PostprocessPartitions detaches the disposable partition ports after
// layering. Their incident constraint edges consequently disappear from the
// graph topology, exactly as when ELK removes the ports from each node.
func PostprocessPartitions(arena *layered.LGraphArena, graphID layered.LGraphID) {
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			ports := append([]layered.LPortID(nil), arena.Node(nodeID).Ports...)
			for _, portID := range ports {
				if basegraph.Get(&arena.Port(portID).Properties, layered.PartitionDummy) {
					arena.SetPortNode(portID, nil)
				}
			}
		}
	}
}
