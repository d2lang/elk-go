// Copyright (c) 2010, 2015 Kiel University and others.
// Copyright (c) 2016 Kiel University and others.
// Copyright (c) 2017 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"sort"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// ProcessHierarchicalPortConstraints imposes fixed-order constraints on
// east/west external-port dummies and replaces fixed-side north/south dummies
// with the per-layer representatives expected by the later router.
func ProcessHierarchicalPortConstraints(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) error {
	processEastWestHierarchicalPortConstraints(arena, graphID)
	processNorthSouthHierarchicalPortConstraints(arena, graphID)
	return nil
}

func processEastWestHierarchicalPortConstraints(arena *layered.LGraphArena, graphID layered.LGraphID) {
	if !basegraph.Get(&arena.Graph(graphID).Properties, layered.PortConstraintsOption).IsOrderFixed() {
		return
	}
	layers := arena.Graph(graphID).Layers
	if len(layers) == 0 {
		return
	}
	processEastWestHierarchyLayer(arena, layers[0])
	processEastWestHierarchyLayer(arena, layers[len(layers)-1])
}

func processEastWestHierarchyLayer(arena *layered.LGraphArena, layerID layered.LayerID) {
	nodes := append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...)
	sort.SliceStable(nodes, func(first, second int) bool {
		firstExternal := arena.Node(nodes[first]).Type == layered.ExternalPortNode
		secondExternal := arena.Node(nodes[second]).Type == layered.ExternalPortNode
		if firstExternal != secondExternal {
			return firstExternal
		}
		if !firstExternal {
			return false
		}
		return basegraph.Get(&arena.Node(nodes[first]).Properties, layered.PortRatioOrPosition) <
			basegraph.Get(&arena.Node(nodes[second]).Properties, layered.PortRatioOrPosition)
	})
	var previous *layered.LNodeID
	for _, nodeID := range nodes {
		if arena.Node(nodeID).Type != layered.ExternalPortNode {
			break
		}
		side := basegraph.Get(&arena.Node(nodeID).Properties, layered.ExternalPortSide)
		if side != layered.WestSide && side != layered.EastSide {
			continue
		}
		if previous != nil {
			successors := basegraph.Get(&arena.Node(*previous).Properties,
				layered.InLayerSuccessorConstraints)
			successors = append(successors, nodeID)
			basegraph.Set(&arena.Node(*previous).Properties,
				layered.InLayerSuccessorConstraints, successors)
		}
		copy := nodeID
		previous = &copy
	}
}

func processNorthSouthHierarchicalPortConstraints(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) {
	if !basegraph.Get(&arena.Graph(graphID).Properties, layered.PortConstraintsOption).IsSideFixed() {
		return
	}
	originalLayers := append([]layered.LayerID(nil), arena.Graph(graphID).Layers...)
	layerCount := len(originalLayers)
	representatives := make([]map[layered.Origin]layered.LNodeID, layerCount+2)
	newNodes := make([][]layered.LNodeID, layerCount+2)
	for index := range representatives {
		representatives[index] = make(map[layered.Origin]layered.LNodeID)
	}
	originalDummies := make([]layered.LNodeID, 0)
	for layerIndex, layerID := range originalLayers {
		for _, nodeID := range append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...) {
			if isNorthSouthHierarchyDummy(arena, nodeID) {
				originalDummies = append(originalDummies, nodeID)
				continue
			}
			for _, edgeID := range append([]layered.LEdgeID(nil), arena.NodeIncomingEdges(nodeID)...) {
				source := arena.EdgeSourceNode(edgeID)
				if !isNorthSouthHierarchyDummy(arena, source) {
					continue
				}
				key := hierarchyDummyOrigin(arena, source)
				dummy, found := representatives[layerIndex][key]
				if !found {
					dummy = createNorthSouthHierarchyRepresentative(arena, graphID, source)
					representatives[layerIndex][key] = dummy
					newNodes[layerIndex] = append(newNodes[layerIndex], dummy)
				}
				output := arena.Node(dummy).Ports[1]
				arena.SetEdgeSource(edgeID, &output)
			}
			for _, edgeID := range append([]layered.LEdgeID(nil), arena.NodeOutgoingEdges(nodeID)...) {
				target := arena.EdgeTargetNode(edgeID)
				if !isNorthSouthHierarchyDummy(arena, target) {
					continue
				}
				key := hierarchyDummyOrigin(arena, target)
				mapIndex := layerIndex + 2
				dummy, found := representatives[mapIndex][key]
				if !found {
					dummy = createNorthSouthHierarchyRepresentative(arena, graphID, target)
					representatives[mapIndex][key] = dummy
					newNodes[mapIndex] = append(newNodes[mapIndex], dummy)
				}
				input := arena.Node(dummy).Ports[0]
				arena.SetEdgeTarget(edgeID, &input)
			}
		}
	}
	for index, nodes := range newNodes {
		if len(nodes) == 0 {
			continue
		}
		var layerID layered.LayerID
		switch {
		case index == 0:
			layerID = arena.AddLayer(graphID)
			arena.Graph(graphID).Layers = append([]layered.LayerID{layerID}, arena.Graph(graphID).Layers...)
		case index == len(newNodes)-1:
			layerID = arena.AddLayerToGraph(graphID)
		default:
			layerID = arena.Graph(graphID).Layers[index-1]
		}
		for _, nodeID := range nodes {
			arena.SetNodeLayer(nodeID, &layerID)
		}
	}
	for _, nodeID := range originalDummies {
		arena.SetNodeLayer(nodeID, nil)
	}
	basegraph.Set(&arena.Graph(graphID).Properties,
		layered.ExternalPortReplacedDummies, originalDummies)
}

func isNorthSouthHierarchyDummy(arena *layered.LGraphArena, nodeID layered.LNodeID) bool {
	if arena.Node(nodeID).Type != layered.ExternalPortNode {
		return false
	}
	side := basegraph.Get(&arena.Node(nodeID).Properties, layered.ExternalPortSide)
	return side == layered.NorthSide || side == layered.SouthSide
}

func hierarchyDummyOrigin(arena *layered.LGraphArena, nodeID layered.LNodeID) layered.Origin {
	if origin, found := basegraph.TryGet(&arena.Node(nodeID).Properties, layered.OriginalElement); found {
		return origin
	}
	return layered.LNodeOrigin(nodeID)
}

func createNorthSouthHierarchyRepresentative(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	original layered.LNodeID,
) layered.LNodeID {
	dummy := arena.AddNode(graphID)
	arena.Node(dummy).Properties.CopyFrom(&arena.Node(original).Properties)
	basegraph.Set(&arena.Node(dummy).Properties, layered.ExternalPortReplacedDummy, original)
	basegraph.Set(&arena.Node(dummy).Properties, layered.PortConstraintsOption,
		layered.PortConstraintsFixedPosition)
	basegraph.Set(&arena.Node(dummy).Properties, layered.AlignmentOption, layered.AlignmentCenter)
	arena.Node(dummy).Type = layered.ExternalPortNode
	input := arena.AddPort()
	arena.SetPortNode(input, &dummy)
	arena.SetPortSide(input, layered.WestSide)
	output := arena.AddPort()
	arena.SetPortNode(output, &dummy)
	arena.SetPortSide(output, layered.EastSide)
	return dummy
}

// SizeHierarchicalPortDummies gives north/south representatives the staggered
// widths required to separate their vertical edge segments.
func SizeHierarchicalPortDummies(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	delta := 2 * basegraph.Get(&arena.Graph(graphID).Properties,
		layered.SpacingEdgeEdgeBetweenLayers)
	for _, layerID := range append([]layered.LayerID(nil), arena.Graph(graphID).Layers...) {
		north, south := make([]layered.LNodeID, 0), make([]layered.LNodeID, 0)
		for _, nodeID := range arena.Layer(layerID).Nodes {
			if arena.Node(nodeID).Type != layered.ExternalPortNode {
				continue
			}
			switch basegraph.Get(&arena.Node(nodeID).Properties, layered.ExternalPortSide) {
			case layered.NorthSide:
				north = append(north, nodeID)
			case layered.SouthSide:
				south = append(south, nodeID)
			}
		}
		setHierarchyDummyWidths(arena, north, true, delta)
		setHierarchyDummyWidths(arena, south, false, delta)
	}
	return nil
}

func setHierarchyDummyWidths(
	arena *layered.LGraphArena,
	nodes []layered.LNodeID,
	topDown bool,
	delta float64,
) {
	width, step := 0.0, delta
	if !topDown {
		width = delta * float64(len(nodes)-1)
		step = -step
	}
	for _, nodeID := range nodes {
		basegraph.Set(&arena.Node(nodeID).Properties, layered.AlignmentOption, layered.AlignmentCenter)
		arena.Node(nodeID).Size.X = width
		for _, portID := range arena.NodePortsOnSide(nodeID, layered.EastSide) {
			arena.Port(portID).Position.X = width
		}
		width += step
	}
}

// PositionHierarchicalPorts fixes east/west dummy y coordinates for fixed
// ratio and fixed-position external ports.
func PositionHierarchicalPorts(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	layers := arena.Graph(graphID).Layers
	if len(layers) > 0 {
		positionHierarchyLayerPorts(arena, graphID, layers[0])
	}
	if len(layers) > 1 {
		positionHierarchyLayerPorts(arena, graphID, layers[len(layers)-1])
	}
	return nil
}

func positionHierarchyLayerPorts(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	layerID layered.LayerID,
) {
	constraints := basegraph.Get(&arena.Graph(graphID).Properties, layered.PortConstraintsOption)
	if !constraints.IsRatioFixed() && !constraints.IsPositionFixed() {
		return
	}
	graphHeight := arena.GraphActualSize(graphID).Y
	for _, nodeID := range append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...) {
		if arena.Node(nodeID).Type != layered.ExternalPortNode {
			continue
		}
		side := basegraph.Get(&arena.Node(nodeID).Properties, layered.ExternalPortSide)
		if side != layered.EastSide && side != layered.WestSide {
			continue
		}
		finalY := basegraph.Get(&arena.Node(nodeID).Properties, layered.PortRatioOrPosition)
		if constraints == layered.PortConstraintsFixedRatio {
			finalY *= graphHeight
		}
		anchor := basegraph.Get(&arena.Node(nodeID).Properties, layered.PortAnchor)
		arena.Node(nodeID).Position.Y = finalY - anchor.Y
		arena.NodeBorderToContentCoordinates(nodeID, false, true)
	}
}

// ResizeHierarchicalNode finishes a child graph and transfers its actual size
// and external-port positions to the compound node that owns it.
func ResizeHierarchicalNode(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	graph := arena.Graph(graphID)
	// Neither the layer list nor a layer's node list changes in the loop (a
	// layer's list is dropped from the layer and then only read here), so
	// they are walked without copies.
	for _, layerID := range graph.Layers {
		nodes := arena.Layer(layerID).Nodes
		graph.LayerlessNodes = append(graph.LayerlessNodes, nodes...)
		arena.Layer(layerID).Nodes = nil
		for _, nodeID := range nodes {
			arena.Node(nodeID).Layer = nil
		}
	}
	graph.Layers = nil
	resizeHierarchyGraph(arena, graphID)
	if graph.ParentNode != nil {
		transferHierarchyGraphToNode(arena, graphID, *graph.ParentNode)
	}
	return nil
}

func resizeHierarchyGraph(arena *layered.LGraphArena, graphID layered.LGraphID) {
	graph := arena.Graph(graphID)
	oldSize := arena.GraphActualSize(graphID)
	newSize := oldSize
	if containsString(basegraph.Get(&graph.Properties, layered.NodeSizeConstraints), "MINIMUM_SIZE") {
		minimum := basegraph.Get(&graph.Properties, layered.NodeSizeMinimum)
		if minimum.X <= 0 {
			minimum.X = 20
		}
		if minimum.Y <= 0 {
			minimum.Y = 20
		}
		if minimum.X > newSize.X {
			newSize.X = minimum.X
		}
		if minimum.Y > newSize.Y {
			newSize.Y = minimum.Y
		}
	}
	alignment := basegraph.Get(&graph.Properties, layered.ContentAlignment)
	if newSize.X > oldSize.X {
		if containsString(alignment, "H_CENTER") {
			graph.Offset.X += (newSize.X - oldSize.X) / 2
		} else if containsString(alignment, "H_RIGHT") {
			graph.Offset.X += newSize.X - oldSize.X
		}
	}
	if newSize.Y > oldSize.Y {
		if containsString(alignment, "V_CENTER") {
			graph.Offset.Y += (newSize.Y - oldSize.Y) / 2
		} else if containsString(alignment, "V_BOTTOM") {
			graph.Offset.Y += newSize.Y - oldSize.Y
		}
	}
	properties := basegraph.Get(&graph.Properties, layered.GraphProperties)
	if properties&layered.GraphPropertyExternalPorts != 0 &&
		(newSize.X > oldSize.X || newSize.Y > oldSize.Y) {
		for _, nodeID := range graph.LayerlessNodes {
			if arena.Node(nodeID).Type != layered.ExternalPortNode {
				continue
			}
			switch basegraph.Get(&arena.Node(nodeID).Properties, layered.ExternalPortSide) {
			case layered.EastSide:
				arena.Node(nodeID).Position.X += newSize.X - oldSize.X
			case layered.SouthSide:
				arena.Node(nodeID).Position.Y += newSize.Y - oldSize.Y
			}
		}
	}
	graph.Size = geometry.Point{
		X: newSize.X - graph.Padding.Left - graph.Padding.Right,
		Y: newSize.Y - graph.Padding.Top - graph.Padding.Bottom,
	}
}

func transferHierarchyGraphToNode(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	parent layered.LNodeID,
) {
	graph := arena.Graph(graphID)
	for _, child := range graph.LayerlessNodes {
		origin, found := basegraph.TryGet(&arena.Node(child).Properties, layered.OriginalElement)
		if !found || origin.Kind != layered.OriginLPort {
			continue
		}
		port := arena.Port(origin.LPort)
		port.Position = arena.ExternalPortPosition(graphID, child, port.Size.X, port.Size.Y)
		side := basegraph.Get(&arena.Node(child).Properties, layered.ExternalPortSide)
		arena.SetPortSide(origin.LPort, side)
	}
	actualSize := arena.GraphActualSize(graphID)
	properties := basegraph.Get(&graph.Properties, layered.GraphProperties)
	if properties&layered.GraphPropertyExternalPorts != 0 {
		basegraph.Set(&arena.Node(parent).Properties, layered.PortConstraintsOption,
			layered.PortConstraintsFixedPosition)
		parentGraph := arena.NodeGraph(parent)
		graphProperties := basegraph.Get(&arena.Graph(parentGraph).Properties, layered.GraphProperties)
		basegraph.Set(&arena.Graph(parentGraph).Properties, layered.GraphProperties,
			graphProperties|layered.GraphPropertyNonFreePorts)
		arena.ResizeNode(parent, actualSize, false, true)
	} else {
		arena.ResizeNode(parent, actualSize, true, true)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
