// Copyright (c) 2010, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p5edges

import (
	"math"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

const (
	minimumVerticalDifference = 1.0
	layerSpaceFactor          = 0.4
)

// PolylineEdgeRouter routes every edge in two steps: at its source layer and
// later at its target layer. It is the exact non-spline phase-5 alternative
// from ELK Layered 0.8.1.
type PolylineEdgeRouter struct {
	createdJunctionPoints map[geometry.Point]struct{}
}

var _ layered.ConfiguredProcessor = (*PolylineEdgeRouter)(nil)

func (*PolylineEdgeRouter) ProcessorConfiguration(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) layered.ProcessorConfiguration {
	properties := basegraph.Get(&arena.Graph(graphID).Properties, layered.GraphProperties)
	var configuration layered.ProcessorConfiguration
	configuration.AddBefore(layered.PhaseNodeOrdering, layered.InvertedPortProcessor)
	if properties&layered.GraphPropertyNorthSouthPorts != 0 {
		configuration.
			AddBefore(layered.PhaseNodeOrdering, layered.NorthSouthPortPreprocessor).
			AddAfter(layered.PhaseEdgeRouting, layered.NorthSouthPortPostprocessor)
	}
	if properties&layered.GraphPropertySelfLoops != 0 {
		configuration.
			AddBefore(layered.PhaseCycleBreaking, layered.SelfLoopPreprocessor).
			AddBefore(layered.PhaseNodePlacement, layered.SelfLoopPortRestorer).
			AddBefore(layered.PhaseNodePlacement, layered.SelfLoopRouter).
			AddAfter(layered.PhaseEdgeRouting, layered.SelfLoopPostprocessor)
	}
	if properties&layered.GraphPropertyCenterLabels != 0 {
		configuration.
			AddBefore(layered.PhaseLayering, layered.LabelDummyInserter).
			AddBefore(layered.PhaseNodePlacement, layered.LabelDummySwitcher).
			AddBefore(layered.PhaseNodePlacement, layered.LabelSideSelector).
			AddAfter(layered.PhaseEdgeRouting, layered.LabelDummyRemover)
	}
	if properties&layered.GraphPropertyEndLabels != 0 {
		configuration.
			AddBefore(layered.PhaseNodePlacement, layered.LabelSideSelector).
			AddBefore(layered.PhaseNodePlacement, layered.EndLabelPreprocessor).
			AddAfter(layered.PhaseEdgeRouting, layered.EndLabelPostprocessor)
	}
	return configuration
}

func (router *PolylineEdgeRouter) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	graph := arena.Graph(graphID)
	slopedZoneWidth := basegraph.Get(&graph.Properties, layered.PolylineSlopedEdgeZoneWidth)
	nodeSpacing := basegraph.Get(&graph.Properties, layered.SpacingNodeNodeBetweenLayers)
	edgeSpacing := basegraph.Get(&graph.Properties, layered.SpacingEdgeEdgeBetweenLayers)
	edgeSpaceFactor := math.Min(1, edgeSpacing/nodeSpacing)
	if router.createdJunctionPoints == nil {
		router.createdJunctionPoints = make(map[geometry.Point]struct{})
	}
	xPosition := 0.0
	if len(graph.Layers) > 0 {
		yDifference := router.calculateWestInLayerEdgeYDifference(arena, graph.Layers[0])
		xPosition = layerSpaceFactor * edgeSpaceFactor * yDifference
	}
	for layerIndex, layerID := range graph.Layers {
		layer := arena.Layer(layerID)
		externalLayer := allExternalWestOrEastPorts(arena, layer.Nodes)
		if externalLayer && xPosition > 0 {
			xPosition -= nodeSpacing
		}
		placeNodesHorizontally(arena, layerID, xPosition)
		maximumVerticalDifference := 0.0
		for _, nodeID := range layer.Nodes {
			node := arena.Node(nodeID)
			maximumCurrentOutputDifference := 0.0
			for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
				edge := arena.Edge(edgeID)
				sourcePosition := absolutePortAnchor(arena, *edge.Source).Y
				targetPosition := absolutePortAnchor(arena, *edge.Target).Y
				targetNode := arena.EdgeTargetNode(edgeID)
				if equalLayerID(node.Layer, arena.Node(targetNode).Layer) && !arena.EdgeIsSelfLoop(edgeID) {
					router.processInLayerEdge(arena, edgeID, xPosition,
						layerSpaceFactor*edgeSpaceFactor*math.Abs(sourcePosition-targetPosition))
					if arena.Port(*edge.Source).Side == layered.WestSide {
						sourcePosition, targetPosition = 0, 0
					}
				}
				maximumCurrentOutputDifference = math.Max(
					maximumCurrentOutputDifference, math.Abs(targetPosition-sourcePosition))
			}
			switch node.Type {
			case layered.NormalNode, layered.LabelNode, layered.LongEdgeNode,
				layered.NorthSouthPortNode, layered.BreakingPointNode:
				router.processNode(arena, nodeID, xPosition, slopedZoneWidth)
			}
			maximumVerticalDifference = math.Max(maximumVerticalDifference, maximumCurrentOutputDifference)
		}
		if layerIndex+1 < len(graph.Layers) {
			nextDifference := router.calculateWestInLayerEdgeYDifference(arena, graph.Layers[layerIndex+1])
			maximumVerticalDifference = math.Max(maximumVerticalDifference, nextDifference)
		}
		layerSpacing := layerSpaceFactor * edgeSpaceFactor * maximumVerticalDifference
		if !externalLayer && layerIndex+1 < len(graph.Layers) {
			layerSpacing += nodeSpacing
		}
		xPosition += layer.Size.X + layerSpacing
	}
	clear(router.createdJunctionPoints)
	graph.Size.X = xPosition
	return nil
}

func (router *PolylineEdgeRouter) processNode(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	layerLeftPosition, maximumAcceptableDifference float64,
) {
	node := arena.Node(nodeID)
	layerRightPosition := layerLeftPosition + arena.Layer(*node.Layer).Size.X
	for _, portID := range node.Ports {
		port := arena.Port(portID)
		absoluteAnchor := absolutePortAnchor(arena, portID)
		if node.Type == layered.NorthSouthPortNode {
			origin := basegraph.Get(&port.Properties, layered.OriginalElement)
			if origin.Kind == layered.OriginLPort {
				absoluteAnchor.X = absolutePortAnchor(arena, origin.LPort).X
				node.Position.X = absoluteAnchor.X
			}
		}
		bendPoint := geometry.Point{Y: absoluteAnchor.Y}
		switch port.Side {
		case layered.EastSide:
			bendPoint.X = layerRightPosition
		case layered.WestSide:
			bendPoint.X = layerLeftPosition
		default:
			continue
		}
		if math.Abs(absoluteAnchor.X-bendPoint.X) <= maximumAcceptableDifference &&
			!router.isInLayerDummy(arena, nodeID) {
			continue
		}
		addJunctionPoint := len(port.OutgoingEdges)+len(port.IncomingEdges) > 1
		for _, edgeID := range arena.PortConnectedEdges(portID) {
			otherPort := arena.EdgeOtherPort(edgeID, portID)
			if math.Abs(absolutePortAnchor(arena, otherPort).Y-bendPoint.Y) > minimumVerticalDifference {
				router.addBendPoint(arena, edgeID, bendPoint, addJunctionPoint, portID)
			}
		}
	}
}

func (*PolylineEdgeRouter) processInLayerEdge(
	arena *layered.LGraphArena,
	edgeID layered.LEdgeID,
	layerPosition, edgeSpacing float64,
) {
	edge := arena.Edge(edgeID)
	sourcePort, targetPort := *edge.Source, *edge.Target
	sourceY := absolutePortAnchor(arena, sourcePort).Y
	middleY := (sourceY + absolutePortAnchor(arena, targetPort).Y) / 2
	bend := geometry.Point{Y: middleY}
	if arena.Port(sourcePort).Side == layered.EastSide {
		sourceNode := *arena.Port(sourcePort).Node
		bend.X = layerPosition + arena.Layer(*arena.Node(sourceNode).Layer).Size.X + edgeSpacing
	} else {
		bend.X = layerPosition - edgeSpacing
	}
	edge.BendPoints = append(edge.BendPoints, geometry.Point{})
	copy(edge.BendPoints[1:], edge.BendPoints[:len(edge.BendPoints)-1])
	edge.BendPoints[0] = bend
}

func (*PolylineEdgeRouter) calculateWestInLayerEdgeYDifference(
	arena *layered.LGraphArena,
	layerID layered.LayerID,
) float64 {
	maximum := 0.0
	for _, nodeID := range arena.Layer(layerID).Nodes {
		for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
			edge := arena.Edge(edgeID)
			if equalLayerID(arena.Node(arena.EdgeTargetNode(edgeID)).Layer, &layerID) &&
				arena.Port(*edge.Source).Side == layered.WestSide {
				maximum = math.Max(maximum, math.Abs(
					absolutePortAnchor(arena, *edge.Target).Y-absolutePortAnchor(arena, *edge.Source).Y))
			}
		}
	}
	return maximum
}

func (router *PolylineEdgeRouter) addBendPoint(
	arena *layered.LGraphArena,
	edgeID layered.LEdgeID,
	bendPoint geometry.Point,
	addJunctionPoint bool,
	currentPort layered.LPortID,
) {
	edge := arena.Edge(edgeID)
	if (!arena.EdgeIsInLayer(edgeID) && absolutePortAnchor(arena, currentPort) == bendPoint) ||
		arena.EdgeIsSelfLoop(edgeID) {
		return
	}
	if *edge.Source == currentPort {
		edge.BendPoints = append(edge.BendPoints, geometry.Point{})
		copy(edge.BendPoints[1:], edge.BendPoints[:len(edge.BendPoints)-1])
		edge.BendPoints[0] = bendPoint
	} else {
		edge.BendPoints = append(edge.BendPoints, bendPoint)
	}
	if addJunctionPoint {
		if _, found := router.createdJunctionPoints[bendPoint]; !found {
			points := basegraph.Get(&edge.Properties, layered.JunctionPoints)
			points = append(points, bendPoint)
			basegraph.Set(&edge.Properties, layered.JunctionPoints, points)
			router.createdJunctionPoints[bendPoint] = struct{}{}
		}
	}
}

func (*PolylineEdgeRouter) isInLayerDummy(arena *layered.LGraphArena, nodeID layered.LNodeID) bool {
	if arena.Node(nodeID).Type != layered.LongEdgeNode {
		return false
	}
	for _, edgeID := range arena.NodeConnectedEdges(nodeID) {
		if arena.EdgeIsInLayer(edgeID) {
			return true
		}
	}
	return false
}

func absolutePortAnchor(arena *layered.LGraphArena, portID layered.LPortID) geometry.Point {
	port := arena.Port(portID)
	return arena.Node(*port.Node).Position.Add(port.Position).Add(port.Anchor)
}

func equalLayerID(first, second *layered.LayerID) bool {
	if first == nil || second == nil {
		return first == nil && second == nil
	}
	return *first == *second
}
