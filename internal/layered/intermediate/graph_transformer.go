// Copyright (c) 2011, 2018 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"math"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// GraphTransformMode selects one of GraphTransformer's two passes.
type GraphTransformMode uint8

const (
	ToInternalLTR GraphTransformMode = iota
	ToInputDirection
)

// TransformGraph applies ELK Layered's direction normalization. The internal
// layered phases always operate left-to-right; the second pass restores the
// input direction after routing.
func TransformGraph(arena *layered.LGraphArena, graphID layered.LGraphID, mode GraphTransformMode) {
	graph := arena.Graph(graphID)
	nodes := arena.GraphAllNodes(graphID)
	direction := basegraph.Get(&graph.Properties, layered.DirectionOption)
	congruency := basegraph.Get(&graph.Properties, layered.DirectionCongruencyOption)

	if congruency == layered.DirectionCongruencyReading {
		switch direction {
		case layered.DirectionLeft:
			mirrorAllX(arena, graphID, nodes)
		case layered.DirectionDown:
			transposeAll(arena, graphID, nodes)
		case layered.DirectionUp:
			if mode == ToInternalLTR {
				transposeAll(arena, graphID, nodes)
				mirrorAllY(arena, graphID, nodes)
			} else {
				mirrorAllY(arena, graphID, nodes)
				transposeAll(arena, graphID, nodes)
			}
		}
		return
	}

	if mode == ToInternalLTR {
		switch direction {
		case layered.DirectionLeft:
			mirrorAllX(arena, graphID, nodes)
			mirrorAllY(arena, graphID, nodes)
		case layered.DirectionDown:
			transposeAll(arena, graphID, nodes)
			mirrorAllX(arena, graphID, nodes)
		case layered.DirectionUp:
			mirrorAllX(arena, graphID, nodes)
			transposeAll(arena, graphID, nodes)
		}
	} else {
		switch direction {
		case layered.DirectionLeft:
			mirrorAllX(arena, graphID, nodes)
			mirrorAllY(arena, graphID, nodes)
		case layered.DirectionDown:
			mirrorAllX(arena, graphID, nodes)
			transposeAll(arena, graphID, nodes)
		case layered.DirectionUp:
			transposeAll(arena, graphID, nodes)
			mirrorAllX(arena, graphID, nodes)
		}
	}
}

func mirrorAllX(arena *layered.LGraphArena, graphID layered.LGraphID, nodes []layered.LNodeID) {
	graph := arena.Graph(graphID)
	offset := 0.0
	if graph.Size.X == 0 {
		for _, nodeID := range nodes {
			node := arena.Node(nodeID)
			offset = math.Max(offset, node.Position.X+node.Size.X+node.Margin.Right)
		}
	} else {
		offset = graph.Size.X - graph.Offset.X
	}
	offset -= graph.Offset.X

	for _, nodeID := range nodes {
		node := arena.Node(nodeID)
		node.Position.X = offset - node.Size.X - node.Position.X
		node.Padding.Left, node.Padding.Right = node.Padding.Right, node.Padding.Left
		if position, found := basegraph.TryGet(&node.Properties, layered.PositionOption); found {
			position.X = offset - node.Size.X - position.X
			basegraph.Set(&node.Properties, layered.PositionOption, position)
		}
		switch basegraph.Get(&node.Properties, layered.AlignmentOption) {
		case layered.AlignmentLeft:
			basegraph.Set(&node.Properties, layered.AlignmentOption, layered.AlignmentRight)
		case layered.AlignmentRight:
			basegraph.Set(&node.Properties, layered.AlignmentOption, layered.AlignmentLeft)
		}
		for _, portID := range node.Ports {
			port := arena.Port(portID)
			port.Position.X = node.Size.X - port.Size.X - port.Position.X
			port.Anchor.X = port.Size.X - port.Anchor.X
			switch port.Side {
			case layered.EastSide:
				arena.SetPortSide(portID, layered.WestSide)
			case layered.WestSide:
				arena.SetPortSide(portID, layered.EastSide)
			}
			for _, edgeID := range port.OutgoingEdges {
				edge := arena.Edge(edgeID)
				for index := range edge.BendPoints {
					edge.BendPoints[index].X = offset - edge.BendPoints[index].X
				}
				if junctions, found := basegraph.TryGet(&edge.Properties, layered.JunctionPoints); found {
					for index := range junctions {
						junctions[index].X = offset - junctions[index].X
					}
					basegraph.Set(&edge.Properties, layered.JunctionPoints, junctions)
				}
				for _, labelID := range edge.Labels {
					label := arena.Label(labelID)
					label.Position.X = offset - label.Size.X - label.Position.X
				}
			}
			for _, labelID := range port.Labels {
				label := arena.Label(labelID)
				label.Position.X = port.Size.X - label.Size.X - label.Position.X
			}
		}
		if node.Type == layered.ExternalPortNode {
			side := basegraph.Get(&node.Properties, layered.ExternalPortSide)
			if side == layered.EastSide {
				side = layered.WestSide
			} else if side == layered.WestSide {
				side = layered.EastSide
			}
			basegraph.Set(&node.Properties, layered.ExternalPortSide, side)
			switch basegraph.Get(&node.Properties, layered.LayerConstraintOption) {
			case layered.LayerConstraintFirst:
				basegraph.Set(&node.Properties, layered.LayerConstraintOption, layered.LayerConstraintLast)
			case layered.LayerConstraintFirstSeparate:
				basegraph.Set(&node.Properties, layered.LayerConstraintOption, layered.LayerConstraintLastSeparate)
			case layered.LayerConstraintLast:
				basegraph.Set(&node.Properties, layered.LayerConstraintOption, layered.LayerConstraintFirst)
			case layered.LayerConstraintLastSeparate:
				basegraph.Set(&node.Properties, layered.LayerConstraintOption, layered.LayerConstraintFirstSeparate)
			}
		}
		for _, labelID := range node.Labels {
			label := arena.Label(labelID)
			label.Position.X = node.Size.X - label.Size.X - label.Position.X
		}
	}
	graph.Padding.Left, graph.Padding.Right = graph.Padding.Right, graph.Padding.Left
	padding := basegraph.Get(&graph.Properties, layered.NodeLabelsPadding)
	padding.Left, padding.Right = padding.Right, padding.Left
	basegraph.Set(&graph.Properties, layered.NodeLabelsPadding, padding)
}

func mirrorAllY(arena *layered.LGraphArena, graphID layered.LGraphID, nodes []layered.LNodeID) {
	graph := arena.Graph(graphID)
	offset := 0.0
	if graph.Size.Y == 0 {
		for _, nodeID := range nodes {
			node := arena.Node(nodeID)
			offset = math.Max(offset, node.Position.Y+node.Size.Y+node.Margin.Bottom)
		}
	} else {
		offset = graph.Size.Y - graph.Offset.Y
	}
	offset -= graph.Offset.Y

	for _, nodeID := range nodes {
		node := arena.Node(nodeID)
		node.Position.Y = offset - node.Size.Y - node.Position.Y
		node.Padding.Top, node.Padding.Bottom = node.Padding.Bottom, node.Padding.Top
		if position, found := basegraph.TryGet(&node.Properties, layered.PositionOption); found {
			position.Y = offset - node.Size.Y - position.Y
			basegraph.Set(&node.Properties, layered.PositionOption, position)
		}
		switch basegraph.Get(&node.Properties, layered.AlignmentOption) {
		case layered.AlignmentTop:
			basegraph.Set(&node.Properties, layered.AlignmentOption, layered.AlignmentBottom)
		case layered.AlignmentBottom:
			basegraph.Set(&node.Properties, layered.AlignmentOption, layered.AlignmentTop)
		}
		for _, portID := range node.Ports {
			port := arena.Port(portID)
			port.Position.Y = node.Size.Y - port.Size.Y - port.Position.Y
			port.Anchor.Y = port.Size.Y - port.Anchor.Y
			switch port.Side {
			case layered.NorthSide:
				arena.SetPortSide(portID, layered.SouthSide)
			case layered.SouthSide:
				arena.SetPortSide(portID, layered.NorthSide)
			}
			for _, edgeID := range port.OutgoingEdges {
				edge := arena.Edge(edgeID)
				for index := range edge.BendPoints {
					edge.BendPoints[index].Y = offset - edge.BendPoints[index].Y
				}
				if junctions, found := basegraph.TryGet(&edge.Properties, layered.JunctionPoints); found {
					for index := range junctions {
						junctions[index].Y = offset - junctions[index].Y
					}
					basegraph.Set(&edge.Properties, layered.JunctionPoints, junctions)
				}
				for _, labelID := range edge.Labels {
					label := arena.Label(labelID)
					label.Position.Y = offset - label.Size.Y - label.Position.Y
				}
			}
			for _, labelID := range port.Labels {
				label := arena.Label(labelID)
				label.Position.Y = port.Size.Y - label.Size.Y - label.Position.Y
			}
		}
		if node.Type == layered.ExternalPortNode {
			side := basegraph.Get(&node.Properties, layered.ExternalPortSide)
			if side == layered.NorthSide {
				side = layered.SouthSide
			} else if side == layered.SouthSide {
				side = layered.NorthSide
			}
			basegraph.Set(&node.Properties, layered.ExternalPortSide, side)
			switch basegraph.Get(&node.Properties, layered.InternalInLayerConstraint) {
			case layered.InLayerConstraintTop:
				basegraph.Set(&node.Properties, layered.InternalInLayerConstraint, layered.InLayerConstraintBottom)
			case layered.InLayerConstraintBottom:
				basegraph.Set(&node.Properties, layered.InternalInLayerConstraint, layered.InLayerConstraintTop)
			}
		}
		for _, labelID := range node.Labels {
			label := arena.Label(labelID)
			label.Position.Y = node.Size.Y - label.Size.Y - label.Position.Y
		}
	}
	graph.Padding.Top, graph.Padding.Bottom = graph.Padding.Bottom, graph.Padding.Top
	padding := basegraph.Get(&graph.Properties, layered.NodeLabelsPadding)
	padding.Top, padding.Bottom = padding.Bottom, padding.Top
	basegraph.Set(&graph.Properties, layered.NodeLabelsPadding, padding)
}

func transposeAll(arena *layered.LGraphArena, graphID layered.LGraphID, nodes []layered.LNodeID) {
	graph := arena.Graph(graphID)
	for _, nodeID := range nodes {
		node := arena.Node(nodeID)
		transposePoint(&node.Position)
		transposePoint(&node.Size)
		transposeSpacing(&node.Padding)
		if minimum, found := basegraph.TryGet(&node.Properties, layered.NodeSizeMinimum); found {
			transposePoint(&minimum)
			basegraph.Set(&node.Properties, layered.NodeSizeMinimum, minimum)
		}
		switch basegraph.Get(&node.Properties, layered.AlignmentOption) {
		case layered.AlignmentLeft:
			basegraph.Set(&node.Properties, layered.AlignmentOption, layered.AlignmentTop)
		case layered.AlignmentRight:
			basegraph.Set(&node.Properties, layered.AlignmentOption, layered.AlignmentBottom)
		case layered.AlignmentTop:
			basegraph.Set(&node.Properties, layered.AlignmentOption, layered.AlignmentLeft)
		case layered.AlignmentBottom:
			basegraph.Set(&node.Properties, layered.AlignmentOption, layered.AlignmentRight)
		}
		if position, found := basegraph.TryGet(&node.Properties, layered.PositionOption); found {
			transposePoint(&position)
			basegraph.Set(&node.Properties, layered.PositionOption, position)
		}
		for _, portID := range node.Ports {
			port := arena.Port(portID)
			transposePoint(&port.Position)
			transposePoint(&port.Anchor)
			transposePoint(&port.Size)
			arena.SetPortSide(portID, transposePortSide(port.Side))
			for _, edgeID := range port.OutgoingEdges {
				edge := arena.Edge(edgeID)
				for index := range edge.BendPoints {
					transposePoint(&edge.BendPoints[index])
				}
				if junctions, found := basegraph.TryGet(&edge.Properties, layered.JunctionPoints); found {
					for index := range junctions {
						transposePoint(&junctions[index])
					}
					basegraph.Set(&edge.Properties, layered.JunctionPoints, junctions)
				}
				for _, labelID := range edge.Labels {
					label := arena.Label(labelID)
					transposePoint(&label.Position)
					transposePoint(&label.Size)
				}
			}
			for _, labelID := range port.Labels {
				label := arena.Label(labelID)
				transposePoint(&label.Position)
				transposePoint(&label.Size)
			}
		}
		if node.Type == layered.ExternalPortNode {
			basegraph.Set(&node.Properties, layered.ExternalPortSide,
				transposePortSide(basegraph.Get(&node.Properties, layered.ExternalPortSide)))
			transposeExternalConstraints(node)
		}
		for _, labelID := range node.Labels {
			label := arena.Label(labelID)
			transposePoint(&label.Position)
			transposePoint(&label.Size)
		}
	}
	transposePoint(&graph.Offset)
	transposePoint(&graph.Size)
	transposeSpacing(&graph.Padding)
	padding := basegraph.Get(&graph.Properties, layered.NodeLabelsPadding)
	transposeSpacing(&padding)
	basegraph.Set(&graph.Properties, layered.NodeLabelsPadding, padding)
	selection := basegraph.Get(&graph.Properties, layered.EdgeLabelSideSelectionOption)
	switch selection {
	case layered.EdgeLabelSideAlwaysUp:
		selection = layered.EdgeLabelSideAlwaysDown
	case layered.EdgeLabelSideAlwaysDown:
		selection = layered.EdgeLabelSideAlwaysUp
	case layered.EdgeLabelSideDirectionUp:
		selection = layered.EdgeLabelSideDirectionDown
	case layered.EdgeLabelSideDirectionDown:
		selection = layered.EdgeLabelSideDirectionUp
	case layered.EdgeLabelSideSmartUp:
		selection = layered.EdgeLabelSideSmartDown
	case layered.EdgeLabelSideSmartDown:
		selection = layered.EdgeLabelSideSmartUp
	}
	basegraph.Set(&graph.Properties, layered.EdgeLabelSideSelectionOption, selection)
}

func transposeExternalConstraints(node *layered.LNode) {
	layerConstraint := basegraph.Get(&node.Properties, layered.LayerConstraintOption)
	inLayerConstraint := basegraph.Get(&node.Properties, layered.InternalInLayerConstraint)
	switch {
	case layerConstraint == layered.LayerConstraintFirstSeparate:
		basegraph.Set(&node.Properties, layered.LayerConstraintOption, layered.LayerConstraintNone)
		basegraph.Set(&node.Properties, layered.InternalInLayerConstraint, layered.InLayerConstraintTop)
	case layerConstraint == layered.LayerConstraintLastSeparate:
		basegraph.Set(&node.Properties, layered.LayerConstraintOption, layered.LayerConstraintNone)
		basegraph.Set(&node.Properties, layered.InternalInLayerConstraint, layered.InLayerConstraintBottom)
	case inLayerConstraint == layered.InLayerConstraintTop:
		basegraph.Set(&node.Properties, layered.LayerConstraintOption, layered.LayerConstraintFirstSeparate)
		basegraph.Set(&node.Properties, layered.InternalInLayerConstraint, layered.InLayerConstraintNone)
	case inLayerConstraint == layered.InLayerConstraintBottom:
		basegraph.Set(&node.Properties, layered.LayerConstraintOption, layered.LayerConstraintLastSeparate)
		basegraph.Set(&node.Properties, layered.InternalInLayerConstraint, layered.InLayerConstraintNone)
	}
}

func transposePoint(point *geometry.Point) { point.X, point.Y = point.Y, point.X }

func transposeSpacing(spacing *geometry.Spacing) {
	spacing.Top, spacing.Right, spacing.Bottom, spacing.Left =
		spacing.Left, spacing.Bottom, spacing.Right, spacing.Top
}

func transposePortSide(side layered.PortSide) layered.PortSide {
	switch side {
	case layered.NorthSide:
		return layered.WestSide
	case layered.WestSide:
		return layered.NorthSide
	case layered.SouthSide:
		return layered.EastSide
	case layered.EastSide:
		return layered.SouthSide
	default:
		return layered.UndefinedSide
	}
}
