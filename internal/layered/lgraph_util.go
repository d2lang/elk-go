// Copyright (c) 2010, 2019 Kiel University and others.
// Copyright (c) 2014, 2020 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
)

// ReverseEdge mirrors LEdge.reverse. Reversal updates adjacency in the same
// sequence as Java, toggles REVERSED, reverses bend points, and swaps HEAD and
// TAIL label placement. Collector-port adaptation is used by cycle breaking.
func (arena *LGraphArena) ReverseEdge(graphID LGraphID, edgeID LEdgeID, adaptPorts bool) {
	edge := arena.Edge(edgeID)
	oldSource, oldTarget := *edge.Source, *edge.Target
	arena.SetEdgeSource(edgeID, nil)
	arena.SetEdgeTarget(edgeID, nil)

	newSource := oldTarget
	if adaptPorts && basegraph.Get(&arena.Port(oldTarget).Properties, InputCollect) {
		newSource = arena.ProvideCollectorPort(graphID, *arena.Port(oldTarget).Node, PortTypeOutput, EastSide)
	}
	newTarget := oldSource
	if adaptPorts && basegraph.Get(&arena.Port(oldSource).Properties, OutputCollect) {
		newTarget = arena.ProvideCollectorPort(graphID, *arena.Port(oldSource).Node, PortTypeInput, WestSide)
	}
	arena.SetEdgeSource(edgeID, &newSource)
	arena.SetEdgeTarget(edgeID, &newTarget)

	for _, labelID := range arena.Edge(edgeID).Labels {
		placement := basegraph.Get(&arena.Label(labelID).Properties, EdgeLabelsPlacement)
		switch placement {
		case EdgeLabelPlacementHead:
			basegraph.Set(&arena.Label(labelID).Properties, EdgeLabelsPlacement, EdgeLabelPlacementTail)
		case EdgeLabelPlacementTail:
			basegraph.Set(&arena.Label(labelID).Properties, EdgeLabelsPlacement, EdgeLabelPlacementHead)
		}
	}
	basegraph.Set(&arena.Edge(edgeID).Properties, Reversed,
		!basegraph.Get(&arena.Edge(edgeID).Properties, Reversed))
	arena.Edge(edgeID).BendPoints = arena.Edge(edgeID).BendPoints.Reverse()
}

func (arena *LGraphArena) ProvideCollectorPort(
	graphID LGraphID,
	nodeID LNodeID,
	portType PortType,
	side PortSide,
) LPortID {
	property := InputCollect
	if portType == PortTypeOutput {
		property = OutputCollect
	}
	for _, portID := range arena.Node(nodeID).Ports {
		if basegraph.Get(&arena.Port(portID).Properties, property) {
			return portID
		}
	}
	portID := arena.AddPort()
	arena.SetPortNode(portID, &nodeID)
	arena.SetPortSide(portID, side)
	basegraph.Set(&arena.Port(portID).Properties, property, true)
	return portID
}

func (arena *LGraphArena) AddLayerToGraph(graphID LGraphID) LayerID {
	layerID := arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = append(arena.Graph(graphID).Layers, layerID)
	return layerID
}

func (arena *LGraphArena) RemoveNodeFromGraph(nodeID LNodeID) {
	node := arena.Node(nodeID)
	if node.Layer != nil {
		arena.SetNodeLayer(nodeID, nil)
	}
	graph := arena.Graph(arena.NodeGraph(nodeID))
	graph.LayerlessNodes = removeNode(graph.LayerlessNodes, nodeID)
}

// CreateExternalPortDummy is the arena-backed counterpart of
// LGraphUtil.createExternalPortDummy. The caller deliberately decides when to
// add the returned node to Graph.LayerlessNodes, matching the Java helper.
func (arena *LGraphArena) CreateExternalPortDummy(
	propertyHolder *basegraph.PropertyMap,
	portConstraints PortConstraints,
	portSide PortSide,
	netFlow int,
	portNodeSize *geometry.Point,
	portPosition *geometry.Point,
	portSize geometry.Point,
	layoutDirection Direction,
	graphID LGraphID,
) LNodeID {
	finalSide := portSide
	dummyID := arena.AddNode(graphID)
	dummy := arena.Node(dummyID)
	dummy.Type = ExternalPortNode
	basegraph.Set(&dummy.Properties, ExternalPortSize, portSize)
	basegraph.Set(&dummy.Properties, PortConstraintsOption, PortConstraintsFixedPosition)
	borderOffset := basegraph.Get(propertyHolder, PortBorderOffset)
	basegraph.Set(&dummy.Properties, PortBorderOffset, borderOffset)

	dummyPortID := arena.AddPort()
	arena.SetPortNode(dummyPortID, &dummyID)

	if !portConstraints.IsSideFixed() {
		if layoutDirection == DirectionUndefined {
			layoutDirection = DirectionRight
		}
		finalSide = DirectionPortSide(layoutDirection)
		if netFlow < 0 {
			finalSide = finalSide.Opposed()
		}
		basegraph.Set(propertyHolder, PortSideOption, finalSide)
	}

	anchor, explicitAnchor := basegraph.TryGet(propertyHolder, PortAnchor)
	if !explicitAnchor {
		anchor = geometry.Point{X: portSize.X / 2, Y: portSize.Y / 2}
	}
	dummyPort := arena.Port(dummyPortID)
	switch finalSide {
	case WestSide:
		basegraph.Set(&dummy.Properties, LayerConstraintOption, LayerConstraintFirstSeparate)
		basegraph.Set(&dummy.Properties, InternalEdgeConstraint, EdgeConstraintOutgoingOnly)
		dummy.Size.Y = portSize.Y
		if borderOffset < 0 {
			dummy.Size.X = -borderOffset
		}
		arena.SetPortSide(dummyPortID, EastSide)
		if !explicitAnchor {
			anchor.X = portSize.X
		}
		anchor.X -= portSize.X
	case EastSide:
		basegraph.Set(&dummy.Properties, LayerConstraintOption, LayerConstraintLastSeparate)
		basegraph.Set(&dummy.Properties, InternalEdgeConstraint, EdgeConstraintIncomingOnly)
		dummy.Size.Y = portSize.Y
		if borderOffset < 0 {
			dummy.Size.X = -borderOffset
		}
		arena.SetPortSide(dummyPortID, WestSide)
		if !explicitAnchor {
			anchor.X = 0
		}
	case NorthSide:
		basegraph.Set(&dummy.Properties, InternalInLayerConstraint, InLayerConstraintTop)
		dummy.Size.X = portSize.X
		if borderOffset < 0 {
			dummy.Size.Y = -borderOffset
		}
		arena.SetPortSide(dummyPortID, SouthSide)
		if !explicitAnchor {
			anchor.Y = portSize.Y
		}
		anchor.Y -= portSize.Y
	case SouthSide:
		basegraph.Set(&dummy.Properties, InternalInLayerConstraint, InLayerConstraintBottom)
		dummy.Size.X = portSize.X
		if borderOffset < 0 {
			dummy.Size.Y = -borderOffset
		}
		arena.SetPortSide(dummyPortID, NorthSide)
		if !explicitAnchor {
			anchor.Y = 0
		}
	default:
		// ELK asserts here. RIGHT is its effective fallback whenever an
		// undefined direction reaches this helper.
		finalSide = EastSide
		basegraph.Set(&dummy.Properties, LayerConstraintOption, LayerConstraintLastSeparate)
		basegraph.Set(&dummy.Properties, InternalEdgeConstraint, EdgeConstraintIncomingOnly)
		arena.SetPortSide(dummyPortID, WestSide)
	}
	dummyPort.Position = anchor
	basegraph.Set(&dummy.Properties, PortAnchor, anchor)

	if portConstraints.IsOrderFixed() {
		information := 0.0
		if portConstraints == PortConstraintsFixedOrder {
			if index, found := basegraph.TryGet(propertyHolder, PortIndex); found {
				information = float64(index)
				if finalSide == SouthSide || finalSide == WestSide {
					information = -information
				}
				basegraph.Set(&dummy.Properties, PortRatioOrPosition, information)
				basegraph.Set(&dummy.Properties, ExternalPortSide, finalSide)
				return dummyID
			}
		}
		if portPosition != nil {
			if finalSide == WestSide || finalSide == EastSide {
				information = portPosition.Y
				if portConstraints.IsRatioFixed() && portNodeSize != nil && portNodeSize.Y != 0 {
					information /= portNodeSize.Y
				}
			} else {
				information = portPosition.X
				if portConstraints.IsRatioFixed() && portNodeSize != nil && portNodeSize.X != 0 {
					information /= portNodeSize.X
				}
			}
		}
		basegraph.Set(&dummy.Properties, PortRatioOrPosition, information)
	}
	basegraph.Set(&dummy.Properties, ExternalPortSide, finalSide)
	return dummyID
}

// ExternalPortPosition calculates an external port's top-left position from
// its laid-out dummy and updates the dummy's border coordinate, exactly as
// LGraphUtil.getExternalPortPosition does.
func (arena *LGraphArena) ExternalPortPosition(
	graphID LGraphID,
	dummyID LNodeID,
	portWidth, portHeight float64,
) geometry.Point {
	graph, dummy := arena.Graph(graphID), arena.Node(dummyID)
	position := dummy.Position.Add(dummy.Size.Scale(.5))
	offset := basegraph.Get(&dummy.Properties, PortBorderOffset)
	switch basegraph.Get(&dummy.Properties, ExternalPortSide) {
	case NorthSide:
		position.X += graph.Padding.Left + graph.Offset.X - portWidth/2
		position.Y = -portHeight - offset
		dummy.Position.Y = -(graph.Padding.Top + offset + graph.Offset.Y)
	case EastSide:
		position.X = graph.Size.X + graph.Padding.Left + graph.Padding.Right + offset
		position.Y += graph.Padding.Top + graph.Offset.Y - portHeight/2
		dummy.Position.X = graph.Size.X + graph.Padding.Right + offset - graph.Offset.X
	case SouthSide:
		position.X += graph.Padding.Left + graph.Offset.X - portWidth/2
		position.Y = graph.Size.Y + graph.Padding.Top + graph.Padding.Bottom + offset
		dummy.Position.Y = graph.Size.Y + graph.Padding.Bottom + offset - graph.Offset.Y
	case WestSide:
		position.X = -portWidth - offset
		position.Y += graph.Padding.Top + graph.Offset.Y - portHeight/2
		dummy.Position.X = -(graph.Padding.Left + offset + graph.Offset.X)
	}
	return position
}

func (arena *LGraphArena) IsDescendant(childID, parentID LNodeID) bool {
	current := childID
	for {
		graph := arena.Graph(arena.NodeGraph(current))
		if graph.ParentNode == nil {
			return false
		}
		current = *graph.ParentNode
		if current == parentID {
			return true
		}
	}
}

// ResizeNode mirrors LGraphUtil.resizeNode. ELK intentionally performs its
// ratio and delta calculations as float32 even though coordinates are doubles.
func (arena *LGraphArena) ResizeNode(
	nodeID LNodeID,
	newSize geometry.Point,
	movePorts, moveLabels bool,
) {
	node := arena.Node(nodeID)
	oldSize := node.Size
	widthRatio := float32(newSize.X / oldSize.X)
	heightRatio := float32(newSize.Y / oldSize.Y)
	widthDiff := float32(newSize.X - oldSize.X)
	heightDiff := float32(newSize.Y - oldSize.Y)
	if movePorts {
		fixed := basegraph.Get(&node.Properties, PortConstraintsOption) == PortConstraintsFixedPosition
		for _, portID := range append([]LPortID(nil), node.Ports...) {
			port := arena.Port(portID)
			switch port.Side {
			case NorthSide:
				if !fixed {
					port.Position.X *= float64(widthRatio)
				}
			case EastSide:
				port.Position.X += float64(widthDiff)
				if !fixed {
					port.Position.Y *= float64(heightRatio)
				}
			case SouthSide:
				if !fixed {
					port.Position.X *= float64(widthRatio)
				}
				port.Position.Y += float64(heightDiff)
			case WestSide:
				if !fixed {
					port.Position.Y *= float64(heightRatio)
				}
			}
		}
	}
	if moveLabels {
		for _, labelID := range append([]LLabelID(nil), node.Labels...) {
			label := arena.Label(labelID)
			middleX := label.Position.X + label.Size.X/2
			middleY := label.Position.Y + label.Size.Y/2
			widthPercent := middleX / oldSize.X
			heightPercent := middleY / oldSize.Y
			if widthPercent+heightPercent >= 1 {
				if widthPercent-heightPercent > 0 && middleY >= 0 {
					label.Position.X += float64(widthDiff)
					label.Position.Y += float64(heightDiff) * heightPercent
				} else if widthPercent-heightPercent < 0 && middleX >= 0 {
					label.Position.X += float64(widthDiff) * widthPercent
					label.Position.Y += float64(heightDiff)
				}
			}
		}
	}
	node.Size = newSize
	basegraph.Set(&node.Properties, NodeSizeConstraints, []string{})
}
