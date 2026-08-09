// Copyright (c) 2010, 2020 Kiel University and others.
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

// NodeMarginCalculator is a direct port of ELK v0.8.1's calculator. Its
// switches intentionally mutate the receiver, mirroring the Java fluent API.
type NodeMarginCalculator struct {
	includeLabels             bool
	includePorts              bool
	includePortLabels         bool
	includeEdgeHeadTailLabels bool
}

func NewNodeMarginCalculator() *NodeMarginCalculator {
	return &NodeMarginCalculator{
		includeLabels:             true,
		includePorts:              true,
		includePortLabels:         true,
		includeEdgeHeadTailLabels: true,
	}
}

func (calculator *NodeMarginCalculator) ExcludeLabels() *NodeMarginCalculator {
	calculator.includeLabels = false
	return calculator
}

func (calculator *NodeMarginCalculator) ExcludePorts() *NodeMarginCalculator {
	calculator.includePorts = false
	return calculator
}

func (calculator *NodeMarginCalculator) ExcludePortLabels() *NodeMarginCalculator {
	calculator.includePortLabels = false
	return calculator
}

func (calculator *NodeMarginCalculator) ExcludeEdgeHeadTailLabels() *NodeMarginCalculator {
	calculator.includeEdgeHeadTailLabels = false
	return calculator
}

// Process calculates margins for every direct child of parent.
func (calculator *NodeMarginCalculator) Process(model *graph.Model, parent graph.NodeID) {
	labelSpacing := graph.Get(&model.Node(parent).Properties, SpacingLabelNode)
	for _, node := range model.Node(parent).Children {
		calculator.processNode(model, node, labelSpacing)
	}
}

func (calculator *NodeMarginCalculator) ProcessNode(model *graph.Model, parent, node graph.NodeID) {
	calculator.processNode(model, node, graph.Get(&model.Node(parent).Properties, SpacingLabelNode))
}

func (calculator *NodeMarginCalculator) processNode(model *graph.Model, nodeID graph.NodeID, labelSpacing float64) {
	node := model.Node(nodeID)
	position := node.Shape.Position()
	size := geometry.Point{X: node.Shape.Width, Y: node.Shape.Height}
	boundingBox := node.Shape.Rectangle()

	if calculator.includeLabels {
		for _, labelID := range node.Labels {
			label := model.Label(labelID)
			box := label.Shape.Rectangle().Move(position)
			boundingBox = boundingBox.Union(box)
		}
	}

	for _, portID := range node.Ports {
		port := model.Port(portID)
		portPosition := position.Add(port.Shape.Position())
		if calculator.includePorts {
			boundingBox = boundingBox.Union(port.Shape.Rectangle().Move(position))
		}
		if calculator.includePortLabels {
			for _, labelID := range port.Labels {
				boundingBox = boundingBox.Union(model.Label(labelID).Shape.Rectangle().Move(portPosition))
			}
		}
		if calculator.includeEdgeHeadTailLabels {
			required := geometry.Point{X: -labelSpacing, Y: -labelSpacing}
			if has(graph.Get(&node.Properties, PortLabelsPlacement), "OUTSIDE") {
				for _, labelID := range port.Labels {
					label := model.Label(labelID)
					required.X += label.Shape.Width + labelSpacing
					required.Y += label.Shape.Height + labelSpacing
				}
			}
			required.X = math.Max(required.X, 0)
			required.Y = math.Max(required.Y, 0)
			calculator.processEdgeLabels(model, &boundingBox, port.OutgoingEdges, false, nodeID, &portID, required, labelSpacing)
			calculator.processEdgeLabels(model, &boundingBox, port.IncomingEdges, true, nodeID, &portID, required, labelSpacing)
		}
	}

	if calculator.includeEdgeHeadTailLabels {
		calculator.processEdgeLabels(model, &boundingBox, node.OutgoingEdges, false, nodeID, nil, geometry.Point{}, labelSpacing)
		calculator.processEdgeLabels(model, &boundingBox, node.IncomingEdges, true, nodeID, nil, geometry.Point{}, labelSpacing)
	}

	graph.Set(&node.Properties, Margins, geometry.Spacing{
		Top:    math.Max(0, position.Y-boundingBox.Y),
		Right:  math.Max(0, boundingBox.MaxX()-(position.X+size.X)),
		Bottom: math.Max(0, boundingBox.MaxY()-(position.Y+size.Y)),
		Left:   math.Max(0, position.X-boundingBox.X),
	})
}

func (calculator *NodeMarginCalculator) processEdgeLabels(
	model *graph.Model,
	boundingBox *geometry.Rectangle,
	edges []graph.EdgeID,
	incoming bool,
	nodeID graph.NodeID,
	portID *graph.PortID,
	portLabelSpace geometry.Point,
	labelSpacing float64,
) {
	wantedPlacement := "TAIL"
	if incoming {
		wantedPlacement = "HEAD"
	}
	for _, edgeID := range edges {
		for _, labelID := range model.Edge(edgeID).Labels {
			label := model.Label(labelID)
			if graph.Get(&label.Properties, EdgeLabelsPlacement) != wantedPlacement {
				continue
			}
			box := calculator.edgeLabelBox(model, labelID, incoming, nodeID, portID, portLabelSpace, labelSpacing)
			*boundingBox = boundingBox.Union(box)
		}
	}
}

func (calculator *NodeMarginCalculator) edgeLabelBox(
	model *graph.Model,
	labelID graph.LabelID,
	incoming bool,
	nodeID graph.NodeID,
	portID *graph.PortID,
	portLabelSpace geometry.Point,
	labelSpacing float64,
) geometry.Rectangle {
	node := model.Node(nodeID)
	label := model.Label(labelID)
	box := geometry.Rectangle{X: node.Shape.X, Y: node.Shape.Y, Width: label.Shape.Width, Height: label.Shape.Height}
	if portID == nil {
		if incoming {
			box.X -= labelSpacing + label.Shape.Width
		} else {
			box.X += node.Shape.Width + labelSpacing
		}
		return box
	}

	port := model.Port(*portID)
	box.X += port.Shape.X
	box.Y += port.Shape.Y
	switch graph.Get(&port.Properties, core.PortSide) {
	case "WEST":
		box.X -= labelSpacing + portLabelSpace.X + labelSpacing + label.Shape.Width
	case "NORTH":
		box.X += port.Shape.Width + labelSpacing
		box.Y -= labelSpacing + portLabelSpace.Y + labelSpacing + label.Shape.Height
	case "SOUTH":
		box.X += port.Shape.Width + labelSpacing
		box.Y += port.Shape.Height + labelSpacing + portLabelSpace.Y + labelSpacing
	default: // UNDEFINED and EAST
		box.X += port.Shape.Width + labelSpacing + portLabelSpace.X + labelSpacing
	}
	return box
}

func CalculateNodeMargins(model *graph.Model, parent graph.NodeID, excludeEdgeHeadTailLabels bool) {
	calculator := NewNodeMarginCalculator()
	if excludeEdgeHeadTailLabels {
		calculator.ExcludeEdgeHeadTailLabels()
	}
	calculator.Process(model, parent)
}
