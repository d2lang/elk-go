// Copyright (c) 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"fmt"
	"strings"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/layered/intermediate/preserveorder"
)

const longEdgeOrderingOptionID = "org.eclipse.elk.layered.considerModelOrder.longEdgeStrategy"

// SortByInputModel ports ELK 0.8.1's SortByInputModelProcessor. It must run
// after long-edge splitting and before north/south port dummy insertion.
func SortByInputModel(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	graph := arena.Graph(graphID)
	ordering := basegraph.Get(&graph.Properties, layered.ConsiderModelOrder)
	longEdgeOrdering := graphLongEdgeOrdering(graph)
	for layerIndex, layerID := range graph.Layers {
		arena.Layer(layerID).ID = layerIndex
		previousIndex := layerIndex
		if previousIndex != 0 {
			previousIndex--
		}
		previousLayerID := graph.Layers[previousIndex]
		nodeComparator := preserveorder.NewNodeComparator(
			arena, arena.Layer(previousLayerID).Nodes, ordering, longEdgeOrdering, true)
		insertionSortNodes(arena.Layer(layerID).Nodes, nodeComparator.Compare)
		for _, nodeID := range arena.Layer(layerID).Nodes {
			node := arena.Node(nodeID)
			constraints := basegraph.Get(&node.Properties, layered.PortConstraintsOption)
			if constraints == layered.PortConstraintsFixedOrder || constraints == layered.PortConstraintsFixedPosition {
				continue
			}
			targetOrders, err := LongEdgeTargetNodePreprocessing(arena, nodeID)
			if err != nil {
				return err
			}
			comparator := preserveorder.NewPortComparator(
				arena,
				arena.Layer(previousLayerID).Nodes,
				ordering,
				targetOrders,
				basegraph.Get(&graph.Properties, layered.PortModelOrder),
			)
			javacompat.GWTStableSort(node.Ports, comparator.Compare)
			arena.InvalidatePortSideCache(nodeID)
		}
		nodeComparator = preserveorder.NewNodeComparator(
			arena, arena.Layer(previousLayerID).Nodes, ordering, longEdgeOrdering, false)
		insertionSortNodes(arena.Layer(layerID).Nodes, nodeComparator.Compare)
	}
	return nil
}

func insertionSortNodes(
	nodes []layered.LNodeID,
	compare func(layered.LNodeID, layered.LNodeID) int,
) {
	for index := 1; index < len(nodes); index++ {
		value := nodes[index]
		position := index
		for position > 0 && compare(nodes[position-1], value) > 0 {
			nodes[position] = nodes[position-1]
			position--
		}
		nodes[position] = value
	}
}

// LongEdgeTargetNodePreprocessing calculates each outgoing port's ultimate
// target and caches the minimum non-reversed edge order per target node.
func LongEdgeTargetNodePreprocessing(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
) (map[layered.LNodeID]int, error) {
	node := arena.Node(nodeID)
	if cached, found := basegraph.TryGet(&node.Properties, layered.TargetNodeModelOrder); found {
		return cached, nil
	}
	targetOrders := make(map[layered.LNodeID]int)
	for _, portID := range node.Ports {
		port := arena.Port(portID)
		if len(port.OutgoingEdges) == 0 {
			continue
		}
		targetNode, found, err := LongEdgeTargetNode(arena, portID)
		if err != nil {
			return nil, err
		}
		if found {
			basegraph.Set(&port.Properties, layered.LongEdgeTargetNode, targetNode)
		} else {
			basegraph.Unset(&port.Properties, layered.LongEdgeTargetNode)
			continue
		}
		edge := arena.Edge(port.OutgoingEdges[0])
		if basegraph.Get(&edge.Properties, layered.Reversed) {
			continue
		}
		order := basegraph.Get(&edge.Properties, layered.ModelOrder)
		if previous, exists := targetOrders[targetNode]; !exists || order < previous {
			targetOrders[targetNode] = order
		}
	}
	basegraph.Set(&node.Properties, layered.TargetNodeModelOrder, targetOrders)
	return targetOrders, nil
}

// LongEdgeTargetNode follows a proper long-edge dummy chain to its normal
// target. A dummy's LONG_EDGE_TARGET shortcut takes precedence, exactly as in
// SortByInputModelProcessor.getTargetNode.
func LongEdgeTargetNode(
	arena *layered.LGraphArena,
	portID layered.LPortID,
) (layered.LNodeID, bool, error) {
	outgoing := arena.Port(portID).OutgoingEdges
	if len(outgoing) == 0 {
		return 0, false, fmt.Errorf("model-order target lookup requires an outgoing edge")
	}
	edgeID := outgoing[0]
	for {
		edge := arena.Edge(edgeID)
		if edge.Target == nil || arena.Port(*edge.Target).Node == nil {
			return 0, false, fmt.Errorf("model-order target lookup encountered a detached edge")
		}
		nodeID := *arena.Port(*edge.Target).Node
		node := arena.Node(nodeID)
		if targetPort, found := basegraph.TryGet(&node.Properties, layered.LongEdgeTarget); found {
			if arena.Port(targetPort).Node == nil {
				return 0, false, nil
			}
			return *arena.Port(targetPort).Node, true, nil
		}
		if node.Type == layered.NormalNode {
			return nodeID, true, nil
		}
		next := arena.NodeOutgoingEdges(nodeID)
		if len(next) == 0 {
			return 0, false, nil
		}
		edgeID = next[0]
	}
}

func graphLongEdgeOrdering(graph *layered.LGraph) preserveorder.LongEdgeOrderingStrategy {
	raw, found := graph.Properties.Raw(longEdgeOrderingOptionID)
	if !found {
		return preserveorder.DummyNodeOver
	}
	if typed, ok := raw.(preserveorder.LongEdgeOrderingStrategy); ok {
		return typed
	}
	switch strings.ToUpper(fmt.Sprint(raw)) {
	case "DUMMY_NODE_UNDER":
		return preserveorder.DummyNodeUnder
	case "EQUAL":
		return preserveorder.LongEdgeEqual
	default:
		return preserveorder.DummyNodeOver
	}
}
