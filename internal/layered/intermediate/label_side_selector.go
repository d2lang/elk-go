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

// SelectLabelSides annotates edge labels and label dummies with the side on
// which they must be placed.
func SelectLabelSides(arena *layered.LGraphArena, graphID layered.LGraphID) {
	mode := basegraph.Get(&arena.Graph(graphID).Properties, layered.EdgeLabelSideSelectionOption)
	switch mode {
	case layered.EdgeLabelSideAlwaysUp:
		setAllLabelSides(arena, graphID, layered.LabelSideAbove)
	case layered.EdgeLabelSideAlwaysDown:
		setAllLabelSides(arena, graphID, layered.LabelSideBelow)
	case layered.EdgeLabelSideDirectionUp:
		setDirectionLabelSides(arena, graphID, layered.LabelSideAbove)
	case layered.EdgeLabelSideDirectionDown:
		setDirectionLabelSides(arena, graphID, layered.LabelSideBelow)
	case layered.EdgeLabelSideSmartUp:
		selectSmartLabelSides(arena, graphID, layered.LabelSideAbove)
	case layered.EdgeLabelSideSmartDown:
		selectSmartLabelSides(arena, graphID, layered.LabelSideBelow)
	}
}

func setAllLabelSides(arena *layered.LGraphArena, graphID layered.LGraphID, side layered.LabelSide) {
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			if arena.Node(nodeID).Type == layered.LabelNode {
				applyLabelDummySide(arena, graphID, nodeID, side)
			}
			for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
				applyEdgeLabelSide(arena, edgeID, side)
			}
		}
	}
}

func setDirectionLabelSides(arena *layered.LGraphArena, graphID layered.LGraphID, rightSide layered.LabelSide) {
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			if arena.Node(nodeID).Type == layered.LabelNode {
				side := rightSide
				if !labelDummyPointsRight(arena, nodeID) {
					side = oppositeLabelSide(side)
				}
				applyLabelDummySide(arena, graphID, nodeID, side)
			}
			for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
				side := rightSide
				if basegraph.Get(&arena.Edge(edgeID).Properties, layered.Reversed) {
					side = oppositeLabelSide(side)
				}
				applyEdgeLabelSide(arena, edgeID, side)
			}
		}
	}
}

func oppositeLabelSide(side layered.LabelSide) layered.LabelSide {
	if side == layered.LabelSideAbove {
		return layered.LabelSideBelow
	}
	if side == layered.LabelSideBelow {
		return layered.LabelSideAbove
	}
	return side
}

func selectSmartLabelSides(arena *layered.LGraphArena, graphID layered.LGraphID, defaultSide layered.LabelSide) {
	dummyQueue := make([]layered.LNodeID, 0)
	for _, layerID := range arena.Graph(graphID).Layers {
		topGroup := true
		labelCount := 0
		for _, nodeID := range arena.Layer(layerID).Nodes {
			switch arena.Node(nodeID).Type {
			case layered.LabelNode:
				labelCount++
				dummyQueue = append(dummyQueue, nodeID)
			case layered.LongEdgeNode:
				dummyQueue = append(dummyQueue, nodeID)
			case layered.NormalNode:
				smartRegularNodeLabels(arena, nodeID, defaultSide)
				if len(dummyQueue) != 0 {
					smartDummyRun(arena, graphID, dummyQueue, labelCount, topGroup, false, defaultSide)
					dummyQueue = dummyQueue[:0]
				}
				topGroup, labelCount = false, 0
			default:
				if len(dummyQueue) != 0 {
					smartDummyRun(arena, graphID, dummyQueue, labelCount, topGroup, false, defaultSide)
					dummyQueue = dummyQueue[:0]
				}
				topGroup, labelCount = false, 0
			}
		}
		if len(dummyQueue) != 0 {
			smartDummyRun(arena, graphID, dummyQueue, labelCount, topGroup, true, defaultSide)
			dummyQueue = dummyQueue[:0]
		}
	}
}

func smartDummyRun(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	dummies []layered.LNodeID,
	labelCount int,
	top, bottom bool,
	defaultSide layered.LabelSide,
) {
	switch {
	case top && (!bottom || len(dummies) > 1) && labelCount == 1 && arena.Node(dummies[0]).Type == layered.LabelNode:
		applyLabelDummySide(arena, graphID, dummies[0], layered.LabelSideAbove)
	case bottom && (!top || len(dummies) > 1) && labelCount == 1 && arena.Node(dummies[len(dummies)-1]).Type == layered.LabelNode:
		applyLabelDummySide(arena, graphID, dummies[len(dummies)-1], layered.LabelSideBelow)
	case len(dummies) == 2:
		applyLabelDummySide(arena, graphID, dummies[0], layered.LabelSideAbove)
		applyLabelDummySide(arena, graphID, dummies[1], layered.LabelSideBelow)
	default:
		applyDummyRunWithSimpleLoops(arena, graphID, dummies, defaultSide)
	}
}

func applyDummyRunWithSimpleLoops(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	dummies []layered.LNodeID,
	defaultSide layered.LabelSide,
) {
	currentRun := make([]layered.LNodeID, 0, len(dummies))
	var previousSource, previousTarget layered.LNodeID
	previousSourceFound, previousTargetFound := false, false
	for _, dummyID := range dummies {
		source, sourceFound := longEdgeEndNode(arena, dummyID, true)
		target, targetFound := longEdgeEndNode(arena, dummyID, false)
		if sourceFound != previousSourceFound || targetFound != previousTargetFound ||
			(sourceFound && source != previousSource) || (targetFound && target != previousTarget) {
			applyLabelDummyRun(arena, graphID, currentRun, defaultSide)
			currentRun = currentRun[:0]
			previousSource, previousSourceFound = source, sourceFound
			previousTarget, previousTargetFound = target, targetFound
		}
		currentRun = append(currentRun, dummyID)
	}
	applyLabelDummyRun(arena, graphID, currentRun, defaultSide)
}

func longEdgeEndNode(arena *layered.LGraphArena, dummyID layered.LNodeID, source bool) (layered.LNodeID, bool) {
	properties := &arena.Node(dummyID).Properties
	var portID layered.LPortID
	var found bool
	if source {
		portID, found = basegraph.TryGet(properties, layered.LongEdgeSource)
	} else {
		portID, found = basegraph.TryGet(properties, layered.LongEdgeTarget)
	}
	if !found || arena.Port(portID).Node == nil {
		return 0, false
	}
	return *arena.Port(portID).Node, true
}

func applyLabelDummyRun(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	run []layered.LNodeID,
	defaultSide layered.LabelSide,
) {
	if len(run) == 0 {
		return
	}
	if len(run) == 2 {
		applyLabelDummySide(arena, graphID, run[0], layered.LabelSideAbove)
		applyLabelDummySide(arena, graphID, run[1], layered.LabelSideBelow)
		return
	}
	for _, dummyID := range run {
		applyLabelDummySide(arena, graphID, dummyID, defaultSide)
	}
}

func smartRegularNodeLabels(arena *layered.LGraphArena, nodeID layered.LNodeID, defaultSide layered.LabelSide) {
	queue := make([][]layered.LLabelID, 0, len(arena.Node(nodeID).Ports))
	currentSide := layered.UndefinedSide
	for _, portID := range arena.Node(nodeID).Ports {
		port := arena.Port(portID)
		if port.Side != currentSide {
			if len(queue) != 0 {
				applyPortEndLabelQueue(arena, queue, currentSide, defaultSide)
			}
			queue = queue[:0]
			currentSide = port.Side
		}
		labels, incident := gatherPortEndLabels(arena, portID)
		if incident {
			queue = append(queue, labels)
		}
	}
	if len(queue) != 0 {
		applyPortEndLabelQueue(arena, queue, currentSide, defaultSide)
	}
}

func applyPortEndLabelQueue(
	arena *layered.LGraphArena,
	queue [][]layered.LLabelID,
	side layered.PortSide,
	defaultSide layered.LabelSide,
) {
	if len(queue) == 2 {
		if side == layered.NorthSide || side == layered.EastSide {
			applyLabelListSide(arena, queue[0], layered.LabelSideAbove)
			applyLabelListSide(arena, queue[1], layered.LabelSideBelow)
		} else {
			applyLabelListSide(arena, queue[0], layered.LabelSideBelow)
			applyLabelListSide(arena, queue[1], layered.LabelSideAbove)
		}
		return
	}
	for _, labels := range queue {
		applyLabelListSide(arena, labels, defaultSide)
	}
}

func gatherPortEndLabels(arena *layered.LGraphArena, portID layered.LPortID) ([]layered.LLabelID, bool) {
	labels := make([]layered.LLabelID, 0)
	maximum := gatherPortLabelsDirect(arena, portID, &labels)
	if dummyID, found := basegraph.TryGet(&arena.Port(portID).Properties, layered.PortDummy); found {
		for _, dummyPortID := range arena.Node(dummyID).Ports {
			origin, hasOrigin := basegraph.TryGet(&arena.Port(dummyPortID).Properties, layered.OriginalElement)
			if hasOrigin && ((origin.Kind == layered.OriginLPort && origin.LPort == portID) ||
				(origin.Kind == layered.OriginPort && uint32(origin.Port) == uint32(portID))) {
				maximum = math.Max(maximum, gatherPortLabelsDirect(arena, dummyPortID, &labels))
			}
		}
	}
	if len(labels) != 0 {
		basegraph.Set(&arena.Port(portID).Properties, layered.MaximumEdgeThickness, maximum)
	}
	return labels, maximum != -1
}

func gatherPortLabelsDirect(arena *layered.LGraphArena, portID layered.LPortID, labels *[]layered.LLabelID) float64 {
	maximum := -1.0
	// Only labels change below, so the edge lists are walked in place.
	for _, edges := range arena.PortEdgeLists(portID) {
		for _, edgeID := range edges {
			edge := arena.Edge(edgeID)
			maximum = math.Max(maximum, basegraph.Get(&edge.Properties, layered.EdgeThickness))
			wanted := layered.EdgeLabelPlacementHead
			if edge.Source != nil && *edge.Source == portID {
				wanted = layered.EdgeLabelPlacementTail
			}
			for _, labelID := range edge.Labels {
				if basegraph.Get(&arena.Label(labelID).Properties, layered.EdgeLabelsPlacement) != wanted {
					continue
				}
				*labels = append(*labels, labelID)
				if !basegraph.Has(&arena.Label(labelID).Properties, layered.EndLabelEdge) {
					basegraph.Set(&arena.Label(labelID).Properties, layered.EndLabelEdge, edgeID)
				}
			}
		}
	}
	return maximum
}

func applyLabelDummySide(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	dummyID layered.LNodeID,
	side layered.LabelSide,
) {
	dummy := arena.Node(dummyID)
	if dummy.Type != layered.LabelNode {
		return
	}
	effective := side
	inline := true
	for _, labelID := range basegraph.Get(&dummy.Properties, layered.RepresentedLabels) {
		if !basegraph.Get(&arena.Label(labelID).Properties, layered.EdgeLabelsInline) {
			inline = false
			break
		}
	}
	if inline {
		effective = layered.LabelSideInline
	}
	basegraph.Set(&dummy.Properties, layered.InternalLabelSide, effective)
	if effective == layered.LabelSideBelow {
		return
	}
	origin := basegraph.Get(&dummy.Properties, layered.OriginalElement)
	if origin.Kind != layered.OriginLEdge {
		panic("layered: label dummy origin is not an LEdge")
	}
	thickness := basegraph.Get(&arena.Edge(origin.LEdge).Properties, layered.EdgeThickness)
	portY := 0.0
	if effective == layered.LabelSideAbove {
		portY = dummy.Size.Y - math.Ceil(thickness/2)
	} else {
		edgeLabelSpacing := basegraph.Get(
			&arena.Graph(graphID).Properties, layered.SpacingEdgeLabel)
		portY = math.Ceil(dummy.Size.Y-edgeLabelSpacing-thickness) / 2
		dummy.Size.Y -= edgeLabelSpacing
		dummy.Size.Y -= thickness
	}
	for _, portID := range dummy.Ports {
		arena.Port(portID).Position.Y = portY
	}
}

func applyEdgeLabelSide(arena *layered.LGraphArena, edgeID layered.LEdgeID, side layered.LabelSide) {
	applyLabelListSide(arena, arena.Edge(edgeID).Labels, side)
}

func applyLabelListSide(arena *layered.LGraphArena, labels []layered.LLabelID, side layered.LabelSide) {
	for _, labelID := range labels {
		basegraph.Set(&arena.Label(labelID).Properties, layered.InternalLabelSide, side)
	}
}

func labelDummyPointsRight(arena *layered.LGraphArena, dummyID layered.LNodeID) bool {
	incoming, outgoing := arena.NodeIncomingEdges(dummyID), arena.NodeOutgoingEdges(dummyID)
	if len(incoming) == 0 || len(outgoing) == 0 {
		panic("layered: label dummy has incomplete edge chain")
	}
	return !basegraph.Get(&arena.Edge(incoming[0]).Properties, layered.Reversed) ||
		!basegraph.Get(&arena.Edge(outgoing[0]).Properties, layered.Reversed)
}
