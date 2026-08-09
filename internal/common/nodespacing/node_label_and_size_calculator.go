// Copyright (c) 2014, 2015 Kiel University and others.
// Copyright (c) 2017, 2019 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package nodespacing

import (
	"math"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

const (
	defaultMinimumWidth  = 20
	defaultMinimumHeight = 20
)

// CalculateLabelAndNodeSizes processes every direct child of parent. This is
// the NodeDimensionCalculation entry point used by layered import.
func CalculateLabelAndNodeSizes(model *graph.Model, parent graph.NodeID, filter func(graph.NodeID) bool) {
	for _, node := range model.Node(parent).Children {
		if filter == nil || filter(node) {
			ProcessNodeSize(model, parent, node, true, false)
		}
	}
}

// ProcessNodeSize applies ELK's fixed/minimum-size path and fixed-position
// port placement. Those are the paths exercised by D2's ELK graphs. The
// function deliberately exposes the same apply/ignore switches as the Java
// entry point so the remaining cell-system constraints can be added without
// changing callers.
func ProcessNodeSize(
	model *graph.Model,
	parent, nodeID graph.NodeID,
	apply bool,
	_ bool,
) geometry.Point {
	node := model.Node(nodeID)
	size := geometry.Point{X: node.Shape.Width, Y: node.Shape.Height}
	constraints := graph.Get(&node.Properties, core.NodeSizeConstraints)
	if len(constraints) != 0 && !(len(constraints) == 1 && constraints[0] == "PORT_LABELS") {
		if has(constraints, "MINIMUM_SIZE") {
			minimum := graph.Get(&node.Properties, core.NodeSizeMinimum)
			if has(graph.Get(&node.Properties, NodeSizeOptions), "DEFAULT_MINIMUM_SIZE") {
				if minimum.X <= 0 {
					minimum.X = defaultMinimumWidth
				}
				if minimum.Y <= 0 {
					minimum.Y = defaultMinimumHeight
				}
			}
			size.X = math.Max(0, minimum.X)
			size.Y = math.Max(0, minimum.Y)
		}
	}
	if !apply {
		return size
	}

	node.Shape.SetDimensions(size.X, size.Y)
	placeFixedPorts(model, nodeID, size)
	placeNodeLabels(model, nodeID, size)
	return size
}

func placeFixedPorts(model *graph.Model, nodeID graph.NodeID, size geometry.Point) {
	node := model.Node(nodeID)
	if graph.Get(&node.Properties, core.PortConstraints) != "FIXED_POS" {
		return
	}
	for _, portID := range node.Ports {
		port := model.Port(portID)
		offset, hasOffset := graph.TryGet(&port.Properties, PortBorderOffset)
		switch graph.Get(&port.Properties, core.PortSide) {
		case "NORTH":
			port.Shape.Y = -port.Shape.Height
			if hasOffset {
				port.Shape.Y -= offset
			}
		case "SOUTH":
			port.Shape.Y = size.Y
			if hasOffset {
				port.Shape.Y += offset
			}
		case "EAST":
			port.Shape.X = size.X
			if hasOffset {
				port.Shape.X += offset
			}
		case "WEST":
			port.Shape.X = -port.Shape.Width
			if hasOffset {
				port.Shape.X -= offset
			}
		}
		port.Shape.Presence |= graph.HasX | graph.HasY
	}
}

func placeNodeLabels(model *graph.Model, nodeID graph.NodeID, size geometry.Point) {
	node := model.Node(nodeID)
	nodePlacement := graph.Get(&node.Properties, NodeLabelsPlacement)
	for _, labelID := range node.Labels {
		label := model.Label(labelID)
		placement := nodePlacement
		if individual, found := graph.TryGet(&label.Properties, NodeLabelsPlacement); found {
			placement = individual
		}
		if len(placement) == 0 {
			continue
		}
		if has(placement, "H_LEFT") {
			label.Shape.X = 0
		} else if has(placement, "H_RIGHT") {
			label.Shape.X = size.X - label.Shape.Width
		} else {
			label.Shape.X = (size.X - label.Shape.Width) / 2
		}
		if has(placement, "V_TOP") {
			label.Shape.Y = 0
		} else if has(placement, "V_BOTTOM") {
			label.Shape.Y = size.Y - label.Shape.Height
		} else {
			label.Shape.Y = (size.Y - label.Shape.Height) / 2
		}
		label.Shape.Presence |= graph.HasX | graph.HasY
	}
}
