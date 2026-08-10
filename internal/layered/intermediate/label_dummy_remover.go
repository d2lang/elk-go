// Copyright (c) 2012, 2020 Kiel University and others.
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

// RemoveLabelDummies places represented center labels, rejoins their edges,
// and removes label dummies from their layers.
func RemoveLabelDummies(arena *layered.LGraphArena, graphID layered.LGraphID) {
	graph := arena.Graph(graphID)
	edgeLabelSpacing := basegraph.Get(&graph.Properties, layered.SpacingEdgeLabel)
	labelLabelSpacing := basegraph.Get(&graph.Properties, layered.SpacingLabelLabel)
	direction := basegraph.Get(&graph.Properties, layered.DirectionOption)

	for _, layerID := range graph.Layers {
		nodes := arena.Layer(layerID).Nodes
		retained := nodes[:0]
		for _, nodeID := range nodes {
			node := arena.Node(nodeID)
			if node.Type != layered.LabelNode {
				retained = append(retained, nodeID)
				continue
			}
			origin := basegraph.Get(&node.Properties, layered.OriginalElement)
			if origin.Kind != layered.OriginLEdge {
				panic("layered: label dummy origin is not an LEdge")
			}
			originEdge := arena.Edge(origin.LEdge)
			thickness := basegraph.Get(&originEdge.Properties, layered.EdgeThickness)
			labelsBelow := basegraph.Get(&node.Properties, layered.InternalLabelSide) == layered.LabelSideBelow
			labelPosition := node.Position
			if labelsBelow {
				labelPosition.Y += thickness + edgeLabelSpacing
			}
			represented := basegraph.Get(&node.Properties, layered.RepresentedLabels)
			inline := representedLabelsAreInline(arena, represented)
			spaceX := node.Size.X
			spaceY := node.Size.Y
			if !inline {
				spaceY -= thickness + edgeLabelSpacing
			}
			if directionIsVertical(direction) {
				placeVerticalCenterLabels(arena, represented, labelPosition.X, labelPosition.Y,
					labelLabelSpacing, spaceY, labelsBelow, direction)
			} else {
				placeHorizontalCenterLabels(arena, represented, labelPosition.X, labelPosition.Y,
					labelLabelSpacing, spaceX)
			}
			originEdge.Labels = append(originEdge.Labels, represented...)
			JoinLongEdgeAt(arena, nodeID,
				basegraph.Get(&graph.Properties, layered.EdgeRoutingOption) == layered.EdgeRoutingPolyline)
		}
		arena.Layer(layerID).Nodes = retained
	}
}

func representedLabelsAreInline(
	arena *layered.LGraphArena,
	labels []layered.LLabelID,
) bool {
	for _, labelID := range labels {
		if !basegraph.Get(&arena.Label(labelID).Properties, layered.EdgeLabelsInline) {
			return false
		}
	}
	return true
}

func placeHorizontalCenterLabels(
	arena *layered.LGraphArena,
	labels []layered.LLabelID,
	x, y, spacing, spaceX float64,
) {
	for _, labelID := range labels {
		label := arena.Label(labelID)
		label.Position.X = x + (spaceX-label.Size.X)/2
		label.Position.Y = y
		y += label.Size.Y + spacing
	}
}

func placeVerticalCenterLabels(
	arena *layered.LGraphArena,
	labels []layered.LLabelID,
	x, y, spacing, spaceY float64,
	leftAligned bool,
	direction layered.Direction,
) {
	inline := representedLabelsAreInline(arena, labels)
	indices := make([]int, len(labels))
	for index := range indices {
		indices[index] = index
		if direction == layered.DirectionUp {
			indices[index] = len(labels) - 1 - index
		}
	}
	for _, index := range indices {
		label := arena.Label(labels[index])
		label.Position.X = x
		switch {
		case inline:
			label.Position.Y = y + (spaceY-label.Size.Y)/2
		case leftAligned:
			label.Position.Y = y
		default:
			label.Position.Y = y + spaceY - label.Size.Y
		}
		x += label.Size.X + spacing
	}
}
