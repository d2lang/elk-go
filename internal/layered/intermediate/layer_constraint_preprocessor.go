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

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

type hiddenNodeConnections uint8

const (
	hiddenNodeConnectionsNone hiddenNodeConnections = iota
	hiddenNodeConnectionsFirstSeparate
	hiddenNodeConnectionsLastSeparate
	hiddenNodeConnectionsBoth
)

var hiddenNodeConnectionsProperty = basegraph.PropertyWithDefault(
	"separateLayerConnections",
	func() hiddenNodeConnections { return hiddenNodeConnectionsNone },
)

// LayerConstraintPreprocessor hides FIRST_SEPARATE and LAST_SEPARATE nodes
// before layering. It is the arena-backed counterpart of ELK 0.8.1's
// intermediate processor of the same name.
type LayerConstraintPreprocessor struct{}

func (*LayerConstraintPreprocessor) Process(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) error {
	graph := arena.Graph(graphID)
	visible := make([]layered.LNodeID, 0, len(graph.LayerlessNodes))
	hidden := make([]layered.LNodeID, 0)
	for _, nodeID := range graph.LayerlessNodes {
		if !hasSeparateLayerConstraint(arena, nodeID) {
			visible = append(visible, nodeID)
			continue
		}
		if err := hideLayerConstraintNode(arena, nodeID); err != nil {
			return err
		}
		hidden = append(hidden, nodeID)
	}
	graph.LayerlessNodes = visible
	if len(hidden) > 0 {
		basegraph.Set(&graph.Properties, layered.HiddenNodes, hidden)
	}
	return nil
}

// ProcessLayerConstraintsBeforeLayering is the function-shaped API used by
// the layered pipeline's intermediate-processor dispatcher.
func ProcessLayerConstraintsBeforeLayering(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) error {
	return (&LayerConstraintPreprocessor{}).Process(arena, graphID)
}

func hasSeparateLayerConstraint(arena *layered.LGraphArena, nodeID layered.LNodeID) bool {
	switch basegraph.Get(&arena.Node(nodeID).Properties, layered.LayerConstraintOption) {
	case layered.LayerConstraintFirstSeparate, layered.LayerConstraintLastSeparate:
		return true
	default:
		return false
	}
}

func hideLayerConstraintNode(arena *layered.LGraphArena, nodeID layered.LNodeID) error {
	if err := ensureAcceptableSeparateLayerEdges(arena, nodeID); err != nil {
		return err
	}
	for _, edgeID := range append([]layered.LEdgeID(nil), arena.NodeConnectedEdges(nodeID)...) {
		edge := arena.Edge(edgeID)
		isOutgoing := edge.Source != nil && arena.Port(*edge.Source).Node != nil &&
			*arena.Port(*edge.Source).Node == nodeID
		var opposite layered.LPortID
		if isOutgoing {
			if edge.Target == nil {
				continue
			}
			opposite = *edge.Target
			arena.SetEdgeTarget(edgeID, nil)
		} else {
			if edge.Source == nil {
				continue
			}
			opposite = *edge.Source
			arena.SetEdgeSource(edgeID, nil)
		}
		basegraph.Set(&arena.Edge(edgeID).Properties, layered.OriginalOppositePort, opposite)
		if oppositeNode := arena.Port(opposite).Node; oppositeNode != nil {
			updateOppositeNodeLayerConstraint(arena, nodeID, *oppositeNode)
		}
	}
	return nil
}

func updateOppositeNodeLayerConstraint(
	arena *layered.LGraphArena,
	hiddenNode, oppositeNode layered.LNodeID,
) {
	properties := &arena.Node(oppositeNode).Properties
	if basegraph.Has(properties, layered.LayerConstraintOption) {
		return
	}
	connections := combineHiddenNodeConnections(
		basegraph.Get(properties, hiddenNodeConnectionsProperty),
		basegraph.Get(&arena.Node(hiddenNode).Properties, layered.LayerConstraintOption),
	)
	basegraph.Set(properties, hiddenNodeConnectionsProperty, connections)
	if len(arena.NodeConnectedEdges(oppositeNode)) > 0 {
		return
	}
	switch connections {
	case hiddenNodeConnectionsFirstSeparate:
		basegraph.Set(properties, layered.LayerConstraintOption, layered.LayerConstraintFirst)
	case hiddenNodeConnectionsLastSeparate:
		basegraph.Set(properties, layered.LayerConstraintOption, layered.LayerConstraintLast)
	}
}

func combineHiddenNodeConnections(
	connections hiddenNodeConnections,
	constraint layered.LayerConstraint,
) hiddenNodeConnections {
	first := constraint == layered.LayerConstraintFirstSeparate
	switch connections {
	case hiddenNodeConnectionsNone:
		if first {
			return hiddenNodeConnectionsFirstSeparate
		}
		return hiddenNodeConnectionsLastSeparate
	case hiddenNodeConnectionsFirstSeparate:
		if first {
			return hiddenNodeConnectionsFirstSeparate
		}
		return hiddenNodeConnectionsBoth
	case hiddenNodeConnectionsLastSeparate:
		if first {
			return hiddenNodeConnectionsBoth
		}
		return hiddenNodeConnectionsLastSeparate
	default:
		return hiddenNodeConnectionsBoth
	}
}

func ensureAcceptableSeparateLayerEdges(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
) error {
	constraint := basegraph.Get(&arena.Node(nodeID).Properties, layered.LayerConstraintOption)
	var edges []layered.LEdgeID
	var description string
	switch constraint {
	case layered.LayerConstraintFirstSeparate:
		edges = arena.NodeIncomingEdges(nodeID)
		description = "FIRST_SEPARATE node has an incoming edge"
	case layered.LayerConstraintLastSeparate:
		edges = arena.NodeOutgoingEdges(nodeID)
		description = "LAST_SEPARATE node has an outgoing edge"
	default:
		return nil
	}
	for _, edgeID := range edges {
		edge := arena.Edge(edgeID)
		if edge.Source != nil && edge.Target != nil &&
			arena.Node(arena.EdgeSourceNode(edgeID)).Type == layered.ExternalPortNode &&
			arena.Node(arena.EdgeTargetNode(edgeID)).Type == layered.ExternalPortNode {
			continue
		}
		return fmt.Errorf("layered: node %d %s", nodeID, description)
	}
	return nil
}
