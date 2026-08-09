// Copyright (c) 2010, 2017 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// ReverseConstraintEdges enforces ELK's outer-layer constraints and the
// fixed-side feedback-node rule before cycle breaking.
func ReverseConstraintEdges(arena *layered.LGraphArena, graphID layered.LGraphID) {
	remaining := make([]layered.LNodeID, 0, len(arena.Graph(graphID).LayerlessNodes))
	for _, nodeID := range arena.Graph(graphID).LayerlessNodes {
		constraint := basegraph.Get(&arena.Node(nodeID).Properties, layered.LayerConstraintOption)
		targetType, constrained := constraintPortType(constraint)
		if !constrained {
			remaining = append(remaining, nodeID)
			continue
		}
		// ELK 0.8.1 writes OUTGOING_ONLY for both branches here.
		basegraph.Set(&arena.Node(nodeID).Properties, layered.InternalEdgeConstraint, layered.EdgeConstraintOutgoingOnly)
		reverseNodeEdges(arena, graphID, nodeID, constraint, targetType)
	}

	for _, nodeID := range remaining {
		node := arena.Node(nodeID)
		constraints := basegraph.Get(&node.Properties, layered.PortConstraintsOption)
		if !constraints.IsSideFixed() || len(node.Ports) == 0 {
			continue
		}
		allReversed := true
		for _, portID := range node.Ports {
			port := arena.Port(portID)
			netFlow := len(port.IncomingEdges) - len(port.OutgoingEdges)
			if !(port.Side == layered.EastSide && netFlow > 0 ||
				port.Side == layered.WestSide && netFlow < 0) {
				allReversed = false
				break
			}
			for _, edgeID := range port.OutgoingEdges {
				constraint := basegraph.Get(
					&arena.Node(arena.EdgeTargetNode(edgeID)).Properties, layered.LayerConstraintOption,
				)
				if constraint == layered.LayerConstraintLast || constraint == layered.LayerConstraintLastSeparate {
					allReversed = false
					break
				}
			}
			for _, edgeID := range port.IncomingEdges {
				constraint := basegraph.Get(
					&arena.Node(arena.EdgeSourceNode(edgeID)).Properties, layered.LayerConstraintOption,
				)
				if constraint == layered.LayerConstraintFirst || constraint == layered.LayerConstraintFirstSeparate {
					allReversed = false
					break
				}
			}
		}
		if allReversed {
			reverseNodeEdges(arena, graphID, nodeID, layered.LayerConstraintNone, layered.PortTypeUndefined)
		}
	}
}

func constraintPortType(constraint layered.LayerConstraint) (layered.PortType, bool) {
	switch constraint {
	case layered.LayerConstraintFirst, layered.LayerConstraintFirstSeparate:
		return layered.PortTypeOutput, true
	case layered.LayerConstraintLast, layered.LayerConstraintLastSeparate:
		return layered.PortTypeInput, true
	default:
		return layered.PortTypeUndefined, false
	}
}

func reverseNodeEdges(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	nodeID layered.LNodeID,
	constraint layered.LayerConstraint,
	targetType layered.PortType,
) {
	for _, portID := range append([]layered.LPortID(nil), arena.Node(nodeID).Ports...) {
		if targetType == layered.PortTypeInput || targetType == layered.PortTypeUndefined {
			for _, edgeID := range append([]layered.LEdgeID(nil), arena.Port(portID).OutgoingEdges...) {
				if canReverseConstraintOutgoing(arena, edgeID, constraint) {
					arena.ReverseEdge(graphID, edgeID, true)
				}
			}
		}
		if targetType == layered.PortTypeOutput || targetType == layered.PortTypeUndefined {
			for _, edgeID := range append([]layered.LEdgeID(nil), arena.Port(portID).IncomingEdges...) {
				if canReverseConstraintIncoming(arena, edgeID, constraint) {
					arena.ReverseEdge(graphID, edgeID, true)
				}
			}
		}
	}
}

func canReverseConstraintOutgoing(
	arena *layered.LGraphArena,
	edgeID layered.LEdgeID,
	sourceConstraint layered.LayerConstraint,
) bool {
	edge := arena.Edge(edgeID)
	if basegraph.Get(&edge.Properties, layered.Reversed) {
		return false
	}
	target := arena.Node(arena.EdgeTargetNode(edgeID))
	if sourceConstraint == layered.LayerConstraintLast && target.Type == layered.LabelNode {
		return false
	}
	return basegraph.Get(&target.Properties, layered.LayerConstraintOption) != layered.LayerConstraintLastSeparate
}

func canReverseConstraintIncoming(
	arena *layered.LGraphArena,
	edgeID layered.LEdgeID,
	targetConstraint layered.LayerConstraint,
) bool {
	edge := arena.Edge(edgeID)
	if basegraph.Get(&edge.Properties, layered.Reversed) {
		return false
	}
	source := arena.Node(arena.EdgeSourceNode(edgeID))
	if targetConstraint == layered.LayerConstraintFirst && source.Type == layered.LabelNode {
		return false
	}
	return basegraph.Get(&source.Properties, layered.LayerConstraintOption) != layered.LayerConstraintFirstSeparate
}
