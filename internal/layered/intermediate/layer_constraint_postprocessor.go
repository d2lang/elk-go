// Copyright (c) 2010, 2020 Kiel University and others.
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

// LayerConstraintPostprocessor moves FIRST and LAST nodes to their boundary
// layers and restores the separate-layer nodes hidden before phase 2.
type LayerConstraintPostprocessor struct{}

func (*LayerConstraintPostprocessor) Process(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) error {
	graph := arena.Graph(graphID)
	if len(graph.Layers) > 0 {
		firstLayer := graph.Layers[0]
		lastLayer := graph.Layers[len(graph.Layers)-1]
		firstLabelLayer := arena.AddLayer(graphID)
		lastLabelLayer := arena.AddLayer(graphID)
		if err := moveFirstAndLastLayerConstraintNodes(
			arena, graphID, firstLayer, lastLayer, firstLabelLayer, lastLabelLayer,
		); err != nil {
			return err
		}
		if len(arena.Layer(firstLabelLayer).Nodes) > 0 {
			graph.Layers = append([]layered.LayerID{firstLabelLayer}, graph.Layers...)
		}
		if len(arena.Layer(lastLabelLayer).Nodes) > 0 {
			graph.Layers = append(graph.Layers, lastLabelLayer)
		}
	}

	hidden, found := basegraph.TryGet(&graph.Properties, layered.HiddenNodes)
	if !found {
		return nil
	}
	firstSeparateLayer := arena.AddLayer(graphID)
	lastSeparateLayer := arena.AddLayer(graphID)
	if err := restoreHiddenLayerConstraintNodes(
		arena, hidden, firstSeparateLayer, lastSeparateLayer,
	); err != nil {
		return err
	}
	if len(arena.Layer(firstSeparateLayer).Nodes) > 0 {
		graph.Layers = append([]layered.LayerID{firstSeparateLayer}, graph.Layers...)
	}
	if len(arena.Layer(lastSeparateLayer).Nodes) > 0 {
		graph.Layers = append(graph.Layers, lastSeparateLayer)
	}
	return nil
}

// ProcessLayerConstraintsAfterLayering is the function-shaped API used by the
// layered pipeline's intermediate-processor dispatcher.
func ProcessLayerConstraintsAfterLayering(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) error {
	return (&LayerConstraintPostprocessor{}).Process(arena, graphID)
}

func moveFirstAndLastLayerConstraintNodes(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	firstLayer, lastLayer, firstLabelLayer, lastLabelLayer layered.LayerID,
) error {
	for _, layerID := range append([]layered.LayerID(nil), arena.Graph(graphID).Layers...) {
		for _, nodeID := range append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...) {
			switch basegraph.Get(&arena.Node(nodeID).Properties, layered.LayerConstraintOption) {
			case layered.LayerConstraintFirst:
				if err := ensureNoIncomingLayerConstraintEdges(arena, nodeID); err != nil {
					return err
				}
				arena.SetNodeLayer(nodeID, &firstLayer)
				moveAdjacentLabelDummies(arena, nodeID, true, firstLabelLayer)
			case layered.LayerConstraintLast:
				if err := ensureNoOutgoingLayerConstraintEdges(arena, nodeID); err != nil {
					return err
				}
				arena.SetNodeLayer(nodeID, &lastLayer)
				moveAdjacentLabelDummies(arena, nodeID, false, lastLabelLayer)
			}
		}
	}
	nonEmpty := make([]layered.LayerID, 0, len(arena.Graph(graphID).Layers))
	for _, layerID := range arena.Graph(graphID).Layers {
		if len(arena.Layer(layerID).Nodes) > 0 {
			nonEmpty = append(nonEmpty, layerID)
		}
	}
	arena.Graph(graphID).Layers = nonEmpty
	return nil
}

func moveAdjacentLabelDummies(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	incoming bool,
	labelLayer layered.LayerID,
) {
	edges := arena.NodeOutgoingEdges(nodeID)
	if incoming {
		edges = arena.NodeIncomingEdges(nodeID)
	}
	for _, edgeID := range append([]layered.LEdgeID(nil), edges...) {
		edge := arena.Edge(edgeID)
		if edge.Source == nil || edge.Target == nil {
			continue
		}
		possibleLabel := arena.EdgeTargetNode(edgeID)
		if incoming {
			possibleLabel = arena.EdgeSourceNode(edgeID)
		}
		if arena.Node(possibleLabel).Type == layered.LabelNode {
			arena.SetNodeLayer(possibleLabel, &labelLayer)
		}
	}
}

func restoreHiddenLayerConstraintNodes(
	arena *layered.LGraphArena,
	hidden []layered.LNodeID,
	firstSeparateLayer, lastSeparateLayer layered.LayerID,
) error {
	for _, nodeID := range hidden {
		switch basegraph.Get(&arena.Node(nodeID).Properties, layered.LayerConstraintOption) {
		case layered.LayerConstraintFirstSeparate:
			arena.SetNodeLayer(nodeID, &firstSeparateLayer)
		case layered.LayerConstraintLastSeparate:
			arena.SetNodeLayer(nodeID, &lastSeparateLayer)
		default:
			return fmt.Errorf("layered: hidden node %d has no separate layer constraint", nodeID)
		}
		for _, edgeID := range append([]layered.LEdgeID(nil), arena.NodeConnectedEdges(nodeID)...) {
			edge := arena.Edge(edgeID)
			if edge.Source != nil && edge.Target != nil {
				continue
			}
			opposite, found := basegraph.TryGet(&edge.Properties, layered.OriginalOppositePort)
			if !found {
				return fmt.Errorf("layered: hidden edge %d has no original opposite port", edgeID)
			}
			if edge.Target == nil {
				arena.SetEdgeTarget(edgeID, &opposite)
			} else {
				arena.SetEdgeSource(edgeID, &opposite)
			}
		}
	}
	return nil
}

func ensureNoIncomingLayerConstraintEdges(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
) error {
	for _, edgeID := range arena.NodeIncomingEdges(nodeID) {
		edge := arena.Edge(edgeID)
		if edge.Source != nil && arena.Node(arena.EdgeSourceNode(edgeID)).Type == layered.LabelNode {
			continue
		}
		return fmt.Errorf("layered: FIRST node %d has a non-label incoming edge", nodeID)
	}
	return nil
}

func ensureNoOutgoingLayerConstraintEdges(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
) error {
	for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
		edge := arena.Edge(edgeID)
		if edge.Target != nil && arena.Node(arena.EdgeTargetNode(edgeID)).Type == layered.LabelNode {
			continue
		}
		return fmt.Errorf("layered: LAST node %d has a non-label outgoing edge", nodeID)
	}
	return nil
}
