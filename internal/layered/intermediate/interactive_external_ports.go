// Copyright (c) 2015 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"fmt"
	"math"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

const interactiveExternalPortSpacing = 10

// PositionInteractiveExternalPorts assigns previous-layout coordinates to
// external-port dummies before the interactive Layered phases run.
func PositionInteractiveExternalPorts(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	graph := arena.Graph(graphID)
	if basegraph.Get(&graph.Properties, layered.GraphProperties)&layered.GraphPropertyExternalPorts == 0 {
		return nil
	}
	minimumX, maximumX := math.Inf(1), math.Inf(-1)
	minimumY, maximumY := math.Inf(1), math.Inf(-1)
	for _, nodeID := range graph.LayerlessNodes {
		node := arena.Node(nodeID)
		if node.Type != layered.NormalNode {
			continue
		}
		minimumX = math.Min(minimumX, node.Position.X-node.Margin.Left)
		maximumX = math.Max(maximumX, node.Position.X+node.Size.X+node.Margin.Right)
		minimumY = math.Min(minimumY, node.Position.Y-node.Margin.Top)
		maximumY = math.Max(maximumY, node.Position.Y+node.Size.Y+node.Margin.Bottom)
	}

	for _, nodeID := range graph.LayerlessNodes {
		node := arena.Node(nodeID)
		if node.Type == layered.NormalNode {
			continue
		}
		if node.Type != layered.ExternalPortNode {
			return fmt.Errorf("layered: node type %s is not supported by interactive external-port positioning", node.Type)
		}
		switch basegraph.Get(&node.Properties, layered.LayerConstraintOption) {
		case layered.LayerConstraintFirstSeparate:
			node.Position.X = minimumX - interactiveExternalPortSpacing
			if y, found := interactiveConnectedY(arena, nodeID, true); found {
				node.Position.Y = y
			}
			continue
		case layered.LayerConstraintLastSeparate:
			node.Position.X = maximumX + interactiveExternalPortSpacing
			if y, found := interactiveConnectedY(arena, nodeID, false); found {
				node.Position.Y = y
			}
			continue
		}
		switch basegraph.Get(&node.Properties, layered.InternalInLayerConstraint) {
		case layered.InLayerConstraintTop:
			if x, found, err := interactiveNorthSouthX(arena, nodeID); err != nil {
				return err
			} else if found {
				node.Position.X = x + interactiveExternalPortSpacing
			}
			node.Position.Y = minimumY - interactiveExternalPortSpacing
		case layered.InLayerConstraintBottom:
			if x, found, err := interactiveNorthSouthX(arena, nodeID); err != nil {
				return err
			} else if found {
				node.Position.X = x + interactiveExternalPortSpacing
			}
			node.Position.Y = maximumY + interactiveExternalPortSpacing
		}
	}
	return nil
}

func interactiveConnectedY(
	arena *layered.LGraphArena,
	dummy layered.LNodeID,
	useTarget bool,
) (float64, bool) {
	for _, edgeID := range arena.NodeConnectedEdges(dummy) {
		other := arena.EdgeSourceNode(edgeID)
		if useTarget {
			other = arena.EdgeTargetNode(edgeID)
		}
		node := arena.Node(other)
		return node.Position.Y + node.Size.Y/2, true
	}
	return 0, false
}

func interactiveNorthSouthX(
	arena *layered.LGraphArena,
	dummy layered.LNodeID,
) (float64, bool, error) {
	ports := arena.Node(dummy).Ports
	if len(ports) != 1 {
		return 0, false, fmt.Errorf("layered: interactive external-port dummy must have exactly one port")
	}
	port := arena.Port(ports[0])
	if len(port.OutgoingEdges) > 0 && len(port.IncomingEdges) > 0 {
		return 0, false, fmt.Errorf("layered: interactive layout does not support north/south ports with incoming and outgoing edges")
	}
	if len(port.OutgoingEdges) > 0 {
		minimum := math.Inf(1)
		for _, edgeID := range port.OutgoingEdges {
			node := arena.Node(arena.EdgeTargetNode(edgeID))
			minimum = math.Min(minimum, node.Position.X-node.Margin.Left)
		}
		return minimum, true, nil
	}
	if len(port.IncomingEdges) > 0 {
		maximum := math.Inf(-1)
		for _, edgeID := range port.IncomingEdges {
			node := arena.Node(arena.EdgeSourceNode(edgeID))
			maximum = math.Max(maximum, node.Position.X+node.Size.X+node.Margin.Right)
		}
		return maximum, true, nil
	}
	return 0, false, nil
}
