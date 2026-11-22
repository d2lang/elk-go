// Copyright (c) 2010, 2015 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"sort"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/layered/p5edges/orthogonal"
)

// RouteHierarchicalPorts restores north/south external-port dummies, routes
// their temporary connections, and fixes every external dummy to the graph
// border. It is ELK's HierarchicalPortOrthogonalEdgeRouter.
func RouteHierarchicalPorts(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	northSouth := restoreNorthSouthHierarchyDummies(arena, graphID)
	setNorthSouthHierarchyCoordinates(arena, graphID, northSouth)
	routeNorthSouthHierarchyEdges(arena, graphID, northSouth)
	removeTemporaryNorthSouthHierarchyDummies(arena, graphID)
	fixHierarchyDummyCoordinates(arena, graphID)
	correctHierarchySlantedSegments(arena, graphID)
	return nil
}

func restoreNorthSouthHierarchyDummies(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) []layered.LNodeID {
	dummies, found := basegraph.TryGet(&arena.Graph(graphID).Properties,
		layered.ExternalPortReplacedDummies)
	if !found || len(dummies) == 0 {
		return nil
	}
	restored := append([]layered.LNodeID(nil), dummies...)
	for _, dummy := range restored {
		port := arena.Node(dummy).Ports[0]
		switch basegraph.Get(&arena.Node(dummy).Properties, layered.ExternalPortSide) {
		case layered.NorthSide:
			arena.SetPortSide(port, layered.SouthSide)
		case layered.SouthSide:
			arena.SetPortSide(port, layered.NorthSide)
		}
	}
	for _, layerID := range append([]layered.LayerID(nil), arena.Graph(graphID).Layers...) {
		for _, nodeID := range append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...) {
			if arena.Node(nodeID).Type != layered.ExternalPortNode {
				continue
			}
			replaced, found := basegraph.TryGet(&arena.Node(nodeID).Properties,
				layered.ExternalPortReplacedDummy)
			if found {
				connectHierarchyRepresentative(arena, nodeID, replaced)
			}
		}
	}
	if layers := arena.Graph(graphID).Layers; len(layers) > 0 {
		last := layers[len(layers)-1]
		for _, dummy := range restored {
			arena.SetNodeLayer(dummy, &last)
		}
	}
	return restored
}

func connectHierarchyRepresentative(
	arena *layered.LGraphArena,
	representative, original layered.LNodeID,
) {
	port := arena.AddPort()
	arena.SetPortNode(port, &representative)
	arena.SetPortSide(port, basegraph.Get(&arena.Node(representative).Properties,
		layered.ExternalPortSide))
	target := arena.Node(original).Ports[0]
	edge := arena.AddEdge()
	arena.SetEdgeSource(edge, &port)
	arena.SetEdgeTarget(edge, &target)
}

func setNorthSouthHierarchyCoordinates(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	dummies []layered.LNodeID,
) {
	constraints := basegraph.Get(&arena.Graph(graphID).Properties, layered.PortConstraintsOption)
	graph := arena.Graph(graphID)
	originalGraphSize := graph.Size
	graphWidth := graph.Size.X + graph.Padding.Left + graph.Padding.Right
	northY := -graph.Padding.Top - graph.Offset.Y
	southY := graph.Size.Y + graph.Padding.Top + graph.Padding.Bottom - graph.Offset.Y
	northern, southern := make([]layered.LNodeID, 0), make([]layered.LNodeID, 0)
	for _, dummy := range dummies {
		switch constraints {
		case layered.PortConstraintsFree, layered.PortConstraintsFixedSide,
			layered.PortConstraintsFixedOrder, layered.PortConstraintsUndefined:
			calculateNorthSouthHierarchyPosition(arena, dummy)
		case layered.PortConstraintsFixedRatio:
			anchor := basegraph.Get(&arena.Node(dummy).Properties, layered.PortAnchor)
			ratio := basegraph.Get(&arena.Node(dummy).Properties, layered.PortRatioOrPosition)
			arena.Node(dummy).Position.X = graphWidth*ratio - anchor.X
			arena.NodeBorderToContentCoordinates(dummy, true, false)
		case layered.PortConstraintsFixedPosition:
			anchor := basegraph.Get(&arena.Node(dummy).Properties, layered.PortAnchor)
			position := basegraph.Get(&arena.Node(dummy).Properties, layered.PortRatioOrPosition)
			arena.Node(dummy).Position.X = position - anchor.X
			arena.NodeBorderToContentCoordinates(dummy, true, false)
			newX := arena.Node(dummy).Position.X + arena.Node(dummy).Size.X/2
			if newX > originalGraphSize.X {
				arena.Graph(graphID).Size.X = newX
			}
		}
		switch basegraph.Get(&arena.Node(dummy).Properties, layered.ExternalPortSide) {
		case layered.NorthSide:
			arena.Node(dummy).Position.Y = northY
			northern = append(northern, dummy)
		case layered.SouthSide:
			arena.Node(dummy).Position.Y = southY
			southern = append(southern, dummy)
		}
	}
	switch constraints {
	case layered.PortConstraintsFree, layered.PortConstraintsFixedSide,
		layered.PortConstraintsUndefined:
		ensureUniqueHierarchyPositions(arena, graphID, northern, false)
		ensureUniqueHierarchyPositions(arena, graphID, southern, false)
	case layered.PortConstraintsFixedOrder:
		ensureUniqueHierarchyPositions(arena, graphID, northern, true)
		ensureUniqueHierarchyPositions(arena, graphID, southern, true)
	}
}

func calculateNorthSouthHierarchyPosition(arena *layered.LGraphArena, dummy layered.LNodeID) {
	port := arena.Node(dummy).Ports[0]
	edges := arena.PortConnectedEdges(port)
	if len(edges) == 0 {
		arena.Node(dummy).Position.X = 0
		return
	}
	sum := 0.0
	for _, edgeID := range edges {
		opposite := arena.EdgeOtherPort(edgeID, port)
		oppositePort := arena.Port(opposite)
		sum += arena.Node(*oppositePort.Node).Position.X + oppositePort.Position.X + oppositePort.Anchor.X
	}
	anchor := basegraph.Get(&arena.Node(dummy).Properties, layered.PortAnchor)
	arena.Node(dummy).Position.X = sum/float64(len(edges)) - anchor.X
}

func ensureUniqueHierarchyPositions(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	dummies []layered.LNodeID,
	byConfiguredOrder bool,
) {
	if len(dummies) == 0 {
		return
	}
	dummies = append([]layered.LNodeID(nil), dummies...)
	sort.SliceStable(dummies, func(first, second int) bool {
		if byConfiguredOrder {
			return basegraph.Get(&arena.Node(dummies[first]).Properties, layered.PortRatioOrPosition) <
				basegraph.Get(&arena.Node(dummies[second]).Properties, layered.PortRatioOrPosition)
		}
		return arena.Node(dummies[first]).Position.X < arena.Node(dummies[second]).Position.X
	})
	spacing := basegraph.Get(&arena.Graph(graphID).Properties, layered.SpacingPortPort)
	first := arena.Node(dummies[0])
	nextValid := first.Position.X + first.Size.X + first.Margin.Right + spacing
	for _, dummy := range dummies[1:] {
		node := arena.Node(dummy)
		delta := node.Position.X - node.Margin.Left - nextValid
		if delta < 0 {
			node.Position.X -= delta
		}
		if required := node.Position.X + node.Size.X; required > arena.Graph(graphID).Size.X {
			arena.Graph(graphID).Size.X = required
		}
		nextValid = node.Position.X + node.Size.X + node.Margin.Right + spacing
	}
}

func routeNorthSouthHierarchyEdges(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	dummies []layered.LNodeID,
) {
	northernSources, northernTargets := make([]layered.LNodeID, 0), make([]layered.LNodeID, 0)
	southernSources, southernTargets := make([]layered.LNodeID, 0), make([]layered.LNodeID, 0)
	for _, dummy := range dummies {
		switch basegraph.Get(&arena.Node(dummy).Properties, layered.ExternalPortSide) {
		case layered.NorthSide:
			appendUniqueNode(&northernTargets, dummy)
			for _, edgeID := range arena.NodeIncomingEdges(dummy) {
				appendUniqueNode(&northernSources, arena.EdgeSourceNode(edgeID))
			}
		case layered.SouthSide:
			appendUniqueNode(&southernTargets, dummy)
			for _, edgeID := range arena.NodeIncomingEdges(dummy) {
				appendUniqueNode(&southernSources, arena.EdgeSourceNode(edgeID))
			}
		}
	}
	nodeSpacing := basegraph.Get(&arena.Graph(graphID).Properties, layered.SpacingNodeNode)
	edgeSpacing := basegraph.Get(&arena.Graph(graphID).Properties, layered.SpacingEdgeEdge)
	if len(northernSources) > 0 {
		generator := orthogonal.NewGenerator(orthogonal.SouthToNorth, edgeSpacing)
		start := -nodeSpacing - arena.Graph(graphID).Offset.Y
		slots := generator.RouteEdges(arena, graphID, northernSources, 0, northernTargets, start)
		generator.Release()
		if slots > 0 {
			height := nodeSpacing + float64(slots-1)*edgeSpacing
			arena.Graph(graphID).Offset.Y += height
			arena.Graph(graphID).Size.Y += height
		}
	}
	if len(southernSources) > 0 {
		generator := orthogonal.NewGenerator(orthogonal.NorthToSouth, edgeSpacing)
		start := arena.Graph(graphID).Size.Y + nodeSpacing - arena.Graph(graphID).Offset.Y
		slots := generator.RouteEdges(arena, graphID, southernSources, 0, southernTargets, start)
		generator.Release()
		if slots > 0 {
			arena.Graph(graphID).Size.Y += nodeSpacing + float64(slots-1)*edgeSpacing
		}
	}
}

func appendUniqueNode(nodes *[]layered.LNodeID, node layered.LNodeID) {
	for _, candidate := range *nodes {
		if candidate == node {
			return
		}
	}
	*nodes = append(*nodes, node)
}

func removeTemporaryNorthSouthHierarchyDummies(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) {
	remove := make([]layered.LNodeID, 0)
	for _, layerID := range append([]layered.LayerID(nil), arena.Graph(graphID).Layers...) {
		for _, nodeID := range append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...) {
			replaced, found := basegraph.TryGet(&arena.Node(nodeID).Properties,
				layered.ExternalPortReplacedDummy)
			if arena.Node(nodeID).Type != layered.ExternalPortNode || !found {
				continue
			}
			var input, output, originPort *layered.LPortID
			for _, portID := range arena.Node(nodeID).Ports {
				copy := portID
				switch arena.Port(portID).Side {
				case layered.WestSide:
					input = &copy
				case layered.EastSide:
					output = &copy
				default:
					originPort = &copy
				}
			}
			if input == nil || output == nil || originPort == nil ||
				len(arena.Port(*originPort).OutgoingEdges) == 0 {
				continue
			}
			connector := arena.Port(*originPort).OutgoingEdges[0]
			incomingBends := arena.Edge(connector).BendPoints.Clone()
			first := arena.Node(nodeID).Position.Add(arena.Port(*originPort).Position)
			incomingBends = append(geometry.Chain{first}, incomingBends...)
			outgoingBends := arena.Edge(connector).BendPoints.Reverse()
			outgoingBends = append(outgoingBends,
				arena.Node(nodeID).Position.Add(arena.Port(*originPort).Position))
			restoredPort := arena.Node(replaced).Ports[0]
			for _, edgeID := range append([]layered.LEdgeID(nil), arena.Port(*input).IncomingEdges...) {
				arena.SetEdgeTarget(edgeID, &restoredPort)
				arena.Edge(edgeID).BendPoints = append(arena.Edge(edgeID).BendPoints, incomingBends...)
			}
			for _, edgeID := range append([]layered.LEdgeID(nil), arena.Port(*output).OutgoingEdges...) {
				arena.SetEdgeSource(edgeID, &restoredPort)
				joined := append(geometry.Chain(nil), outgoingBends...)
				arena.Edge(edgeID).BendPoints = append(joined, arena.Edge(edgeID).BendPoints...)
			}
			arena.SetEdgeSource(connector, nil)
			arena.SetEdgeTarget(connector, nil)
			remove = append(remove, nodeID)
		}
	}
	for _, nodeID := range remove {
		arena.SetNodeLayer(nodeID, nil)
	}
}

func fixHierarchyDummyCoordinates(arena *layered.LGraphArena, graphID layered.LGraphID) {
	layers := arena.Graph(graphID).Layers
	if len(layers) == 0 {
		return
	}
	constraints := basegraph.Get(&arena.Graph(graphID).Properties, layered.PortConstraintsOption)
	fixHierarchyDummyLayerCoordinates(arena, graphID, layers[0], constraints)
	fixHierarchyDummyLayerCoordinates(arena, graphID, layers[len(layers)-1], constraints)
}

func fixHierarchyDummyLayerCoordinates(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	layerID layered.LayerID,
	constraints layered.PortConstraints,
) {
	graph := arena.Graph(graphID)
	actualHeight := arena.GraphActualSize(graphID).Y
	newHeight := actualHeight
	nodes := append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...)
	for _, nodeID := range nodes {
		if arena.Node(nodeID).Type != layered.ExternalPortNode {
			continue
		}
		side := basegraph.Get(&arena.Node(nodeID).Properties, layered.ExternalPortSide)
		extSize := basegraph.Get(&arena.Node(nodeID).Properties, layered.ExternalPortSize)
		switch side {
		case layered.EastSide:
			arena.Node(nodeID).Position.X = graph.Size.X + graph.Padding.Right - graph.Offset.X
		case layered.WestSide:
			arena.Node(nodeID).Position.X = -graph.Offset.X - graph.Padding.Left
		}
		requiredHeight := 0.0
		if side == layered.EastSide || side == layered.WestSide {
			anchor := basegraph.Get(&arena.Node(nodeID).Properties, layered.PortAnchor)
			switch constraints {
			case layered.PortConstraintsFixedRatio:
				ratio := basegraph.Get(&arena.Node(nodeID).Properties, layered.PortRatioOrPosition)
				arena.Node(nodeID).Position.Y = actualHeight*ratio - anchor.Y
				requiredHeight = arena.Node(nodeID).Position.Y + extSize.Y
				arena.NodeBorderToContentCoordinates(nodeID, false, true)
			case layered.PortConstraintsFixedPosition:
				position := basegraph.Get(&arena.Node(nodeID).Properties, layered.PortRatioOrPosition)
				arena.Node(nodeID).Position.Y = position - anchor.Y
				requiredHeight = arena.Node(nodeID).Position.Y + extSize.Y
				arena.NodeBorderToContentCoordinates(nodeID, false, true)
			}
		}
		if requiredHeight > newHeight {
			newHeight = requiredHeight
		}
	}
	arena.Graph(graphID).Size.Y += newHeight - actualHeight
	graph = arena.Graph(graphID)
	for _, nodeID := range nodes {
		if arena.Node(nodeID).Type != layered.ExternalPortNode {
			continue
		}
		switch basegraph.Get(&arena.Node(nodeID).Properties, layered.ExternalPortSide) {
		case layered.NorthSide:
			arena.Node(nodeID).Position.Y = -graph.Offset.Y - graph.Padding.Top
		case layered.SouthSide:
			arena.Node(nodeID).Position.Y = graph.Size.Y + graph.Padding.Bottom - graph.Offset.Y
		}
	}
}

func correctHierarchySlantedSegments(arena *layered.LGraphArena, graphID layered.LGraphID) {
	layers := arena.Graph(graphID).Layers
	if len(layers) == 0 {
		return
	}
	correctHierarchySlantedLayer(arena, layers[0])
	correctHierarchySlantedLayer(arena, layers[len(layers)-1])
}

func correctHierarchySlantedLayer(arena *layered.LGraphArena, layerID layered.LayerID) {
	for _, nodeID := range append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...) {
		if arena.Node(nodeID).Type != layered.ExternalPortNode {
			continue
		}
		side := basegraph.Get(&arena.Node(nodeID).Properties, layered.ExternalPortSide)
		if side != layered.EastSide && side != layered.WestSide {
			continue
		}
		for _, edgeID := range arena.NodeConnectedEdges(nodeID) {
			edge := arena.Edge(edgeID)
			if len(edge.BendPoints) == 0 {
				continue
			}
			if edge.Source != nil && arena.Port(*edge.Source).Node != nil &&
				*arena.Port(*edge.Source).Node == nodeID {
				edge.BendPoints[0].Y = hierarchyPortAbsoluteAnchor(arena, *edge.Source).Y
			}
			if edge.Target != nil && arena.Port(*edge.Target).Node != nil &&
				*arena.Port(*edge.Target).Node == nodeID {
				edge.BendPoints[len(edge.BendPoints)-1].Y =
					hierarchyPortAbsoluteAnchor(arena, *edge.Target).Y
			}
		}
	}
}

func hierarchyPortAbsoluteAnchor(arena *layered.LGraphArena, portID layered.LPortID) geometry.Point {
	port := arena.Port(portID)
	return arena.Node(*port.Node).Position.Add(port.Position).Add(port.Anchor)
}
