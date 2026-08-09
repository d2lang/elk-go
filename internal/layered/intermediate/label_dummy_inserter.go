// Copyright (c) 2012, 2017 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"math"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// InsertLabelDummies replaces center labels on non-self-loop edges with one
// label dummy per edge. New dummies are appended only after the original
// layerless-node traversal, as in ELK's implementation.
func InsertLabelDummies(arena *layered.LGraphArena, graphID layered.LGraphID) {
	graph := arena.Graph(graphID)
	edgeLabelSpacing := basegraph.Get(&graph.Properties, layered.SpacingEdgeLabel)
	labelLabelSpacing := basegraph.Get(&graph.Properties, layered.SpacingLabelLabel)
	vertical := directionIsVertical(basegraph.Get(&graph.Properties, layered.DirectionOption))
	newDummies := make([]layered.LNodeID, 0)

	for _, nodeID := range append([]layered.LNodeID(nil), graph.LayerlessNodes...) {
		for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
			if arena.EdgeIsSelfLoop(edgeID) || !edgeHasCenterLabel(arena, edgeID) {
				continue
			}
			thickness := basegraph.Get(&arena.Edge(edgeID).Properties, layered.EdgeThickness)
			if thickness < 0 {
				thickness = 0
				basegraph.Set(&arena.Edge(edgeID).Properties, layered.EdgeThickness, thickness)
			}
			dummyID := createLabelDummy(arena, graphID, edgeID)
			newDummies = append(newDummies, dummyID)
			dummy := arena.Node(dummyID)
			represented := make([]layered.LLabelID, 0, len(arena.Edge(edgeID).Labels))
			retained := arena.Edge(edgeID).Labels[:0]
			for _, labelID := range arena.Edge(edgeID).Labels {
				label := arena.Label(labelID)
				if basegraph.Get(&label.Properties, layered.EdgeLabelsPlacement) != layered.EdgeLabelPlacementCenter {
					retained = append(retained, labelID)
					continue
				}
				if vertical {
					dummy.Size.X += label.Size.X + labelLabelSpacing
					dummy.Size.Y = math.Max(dummy.Size.Y, label.Size.Y)
				} else {
					dummy.Size.X = math.Max(dummy.Size.X, label.Size.X)
					dummy.Size.Y += label.Size.Y + labelLabelSpacing
				}
				represented = append(represented, labelID)
			}
			arena.Edge(edgeID).Labels = retained
			basegraph.Set(&dummy.Properties, layered.RepresentedLabels, represented)
			if vertical {
				dummy.Size.X -= labelLabelSpacing
				dummy.Size.Y += edgeLabelSpacing + thickness
			} else {
				dummy.Size.Y += edgeLabelSpacing - labelLabelSpacing + thickness
			}
		}
	}
	graph.LayerlessNodes = append(graph.LayerlessNodes, newDummies...)
}

func edgeHasCenterLabel(arena *layered.LGraphArena, edgeID layered.LEdgeID) bool {
	for _, labelID := range arena.Edge(edgeID).Labels {
		if basegraph.Get(&arena.Label(labelID).Properties, layered.EdgeLabelsPlacement) == layered.EdgeLabelPlacementCenter {
			return true
		}
	}
	return false
}

func createLabelDummy(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	edgeID layered.LEdgeID,
) layered.LNodeID {
	dummyID := arena.AddNode(graphID)
	dummy := arena.Node(dummyID)
	dummy.Type = layered.LabelNode
	basegraph.Set(&dummy.Properties, layered.OriginalElement, layered.LEdgeOrigin(edgeID))
	basegraph.Set(&dummy.Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedPosition)
	basegraph.Set(&dummy.Properties, layered.LongEdgeSource, *arena.Edge(edgeID).Source)
	basegraph.Set(&dummy.Properties, layered.LongEdgeTarget, *arena.Edge(edgeID).Target)
	SplitEdgeAtDummy(arena, edgeID, dummyID)
	portPosition := math.Floor(basegraph.Get(&arena.Edge(edgeID).Properties, layered.EdgeThickness) / 2)
	for _, portID := range dummy.Ports {
		arena.Port(portID).Position.Y = portPosition
	}
	return dummyID
}

func directionIsVertical(direction layered.Direction) bool {
	return direction == layered.DirectionDown || direction == layered.DirectionUp
}
