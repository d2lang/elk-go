// Copyright (c) 2008, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package elkutil ports the core ElkUtil operations shared by native layout
// providers. The implementation intentionally preserves Java arithmetic and
// mutation order where those are visible in JSON output.
package elkutil

import (
	"fmt"
	"math"

	"github.com/d2lang/elk-go/internal/basicoptions"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

const (
	defaultMinWidth  = 20.0
	defaultMinHeight = 20.0
)

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

// EffectiveMinSizeConstraintFor mirrors ElkUtil.effectiveMinSizeConstraintFor.
func EffectiveMinSizeConstraintFor(model *graph.Model, nodeID graph.NodeID) geometry.Point {
	node := model.Node(nodeID)
	if !contains(graph.Get(&node.Properties, core.NodeSizeConstraints), "MINIMUM_SIZE") {
		return geometry.Point{}
	}
	minimum := graph.Get(&node.Properties, core.NodeSizeMinimum)
	if contains(graph.Get(&node.Properties, basicoptions.NodeSizeOptions), "DEFAULT_MINIMUM_SIZE") {
		if minimum.X <= 0 {
			minimum.X = defaultMinWidth
		}
		if minimum.Y <= 0 {
			minimum.Y = defaultMinHeight
		}
	}
	return minimum
}

// ResizeNodeForConstraints is ElkUtil.resizeNode(ElkNode). It reports false
// when the node's size is fixed and no mutation is performed.
func ResizeNodeForConstraints(model *graph.Model, nodeID graph.NodeID) bool {
	node := model.Node(nodeID)
	constraints := graph.Get(&node.Properties, core.NodeSizeConstraints)
	if len(constraints) == 0 {
		return false
	}
	newWidth, newHeight := 0.0, 0.0
	if contains(constraints, "PORTS") {
		minNorth, minEast, minSouth, minWest := 2.0, 2.0, 2.0, 2.0
		fixedPorts := graph.Get(&node.Properties, core.PortConstraints) == "FIXED_POS"
		direction := applicableDirection(model, nodeID)
		for _, portID := range node.Ports {
			port := model.Port(portID)
			side := graph.Get(&port.Properties, core.PortSide)
			if side == "UNDEFINED" {
				side = CalcPortSide(model, portID, direction)
				graph.Set(&port.Properties, core.PortSide, side)
			}
			if fixedPorts {
				switch side {
				case "NORTH":
					minNorth = math.Max(minNorth, port.Shape.X+port.Shape.Width)
				case "EAST":
					minEast = math.Max(minEast, port.Shape.Y+port.Shape.Height)
				case "SOUTH":
					minSouth = math.Max(minSouth, port.Shape.X+port.Shape.Width)
				case "WEST":
					minWest = math.Max(minWest, port.Shape.Y+port.Shape.Height)
				}
			} else {
				switch side {
				case "NORTH":
					minNorth += port.Shape.Width + 2
				case "EAST":
					minEast += port.Shape.Height + 2
				case "SOUTH":
					minSouth += port.Shape.Width + 2
				case "WEST":
					minWest += port.Shape.Height + 2
				}
			}
		}
		newWidth = math.Max(minNorth, minSouth)
		newHeight = math.Max(minEast, minWest)
	}
	ResizeNode(model, nodeID, newWidth, newHeight, true, true)
	return true
}

// ResizeNode changes a node's dimensions and moves fixed peripheral content
// exactly as ElkUtil.resizeNode does.
func ResizeNode(model *graph.Model, nodeID graph.NodeID, width, height float64, movePorts, moveLabels bool) geometry.Point {
	node := model.Node(nodeID)
	oldSize := geometry.Point{X: node.Shape.Width, Y: node.Shape.Height}
	newSize := EffectiveMinSizeConstraintFor(model, nodeID)
	newSize.X = math.Max(newSize.X, width)
	newSize.Y = math.Max(newSize.Y, height)
	widthRatio := newSize.X / oldSize.X
	heightRatio := newSize.Y / oldSize.Y
	widthDiff := newSize.X - oldSize.X
	heightDiff := newSize.Y - oldSize.Y

	if movePorts {
		direction := applicableDirection(model, nodeID)
		fixedPorts := graph.Get(&node.Properties, core.PortConstraints) == "FIXED_POS"
		for _, portID := range node.Ports {
			port := model.Port(portID)
			side := graph.Get(&port.Properties, core.PortSide)
			if side == "UNDEFINED" {
				side = CalcPortSide(model, portID, direction)
				graph.Set(&port.Properties, core.PortSide, side)
			}
			switch side {
			case "NORTH":
				if !fixedPorts {
					port.Shape.X *= widthRatio
				}
			case "EAST":
				port.Shape.X += widthDiff
				if !fixedPorts {
					port.Shape.Y *= heightRatio
				}
			case "SOUTH":
				if !fixedPorts {
					port.Shape.X *= widthRatio
				}
				port.Shape.Y += heightDiff
			case "WEST":
				if !fixedPorts {
					port.Shape.Y *= heightRatio
				}
			}
			port.Shape.Presence |= graph.HasX | graph.HasY
		}
	}

	node.Shape.SetDimensions(newSize.X, newSize.Y)
	if moveLabels {
		for _, labelID := range node.Labels {
			label := model.Label(labelID)
			midx := label.Shape.X + label.Shape.Width/2
			midy := label.Shape.Y + label.Shape.Height/2
			widthPercent := midx / oldSize.X
			heightPercent := midy / oldSize.Y
			if widthPercent+heightPercent >= 1 {
				if widthPercent-heightPercent > 0 && midy >= 0 {
					label.Shape.X += widthDiff
					label.Shape.Y += heightDiff * heightPercent
				} else if widthPercent-heightPercent < 0 && midx >= 0 {
					label.Shape.X += widthDiff * widthPercent
					label.Shape.Y += heightDiff
				}
				label.Shape.Presence |= graph.HasX | graph.HasY
			}
		}
	}
	graph.Set(&node.Properties, core.NodeSizeConstraints, []string{})
	return geometry.Point{X: widthRatio, Y: heightRatio}
}

func applicableDirection(model *graph.Model, nodeID graph.NodeID) string {
	node := model.Node(nodeID)
	if node.Parent == nil {
		return graph.Get(&node.Properties, core.Direction)
	}
	return graph.Get(&model.Node(*node.Parent).Properties, core.Direction)
}

// CalcPortSide mirrors ElkUtil.calcPortSide.
func CalcPortSide(model *graph.Model, portID graph.PortID, direction string) string {
	port := model.Port(portID)
	node := model.Node(port.Parent)
	if node.Shape.Width <= 0 && node.Shape.Height <= 0 {
		return "UNDEFINED"
	}
	switch direction {
	case "LEFT", "RIGHT":
		if port.Shape.X < 0 {
			return "WEST"
		}
		if port.Shape.X+port.Shape.Width > node.Shape.Width {
			return "EAST"
		}
	case "UP", "DOWN":
		if port.Shape.Y < 0 {
			return "NORTH"
		}
		if port.Shape.Y+port.Shape.Height > node.Shape.Height {
			return "SOUTH"
		}
	}
	widthPercent := (port.Shape.X + port.Shape.Width/2) / node.Shape.Width
	heightPercent := (port.Shape.Y + port.Shape.Height/2) / node.Shape.Height
	switch {
	case widthPercent+heightPercent <= 1 && widthPercent-heightPercent <= 0:
		return "WEST"
	case widthPercent+heightPercent >= 1 && widthPercent-heightPercent >= 0:
		return "EAST"
	case heightPercent < 0.5:
		return "NORTH"
	default:
		return "SOUTH"
	}
}

// Translate moves the contents and contained edges of parent.
func Translate(model *graph.Model, parent graph.NodeID, x, y float64) {
	node := model.Node(parent)
	for _, childID := range node.Children {
		child := model.Node(childID)
		child.Shape.X += x
		child.Shape.Y += y
		child.Shape.Presence |= graph.HasX | graph.HasY
	}
	for _, edgeID := range node.ContainedEdges {
		TranslateEdge(model, edgeID, x, y)
	}
}

func TranslateEdge(model *graph.Model, edgeID graph.EdgeID, x, y float64) {
	edge := model.Edge(edgeID)
	for _, sectionID := range edge.Sections {
		section := model.Section(sectionID)
		section.Start.X += x
		section.Start.Y += y
		section.End.X += x
		section.End.Y += y
		section.BendPoints.Offset(geometry.Point{X: x, Y: y})
		section.StartPresent, section.EndPresent = true, true
	}
	for _, labelID := range edge.Labels {
		label := model.Label(labelID)
		label.Shape.X += x
		label.Shape.Y += y
		label.Shape.Presence |= graph.HasX | graph.HasY
	}
	if points, found := graph.TryGet(&edge.Properties, core.JunctionPoints); found {
		points.Offset(geometry.Point{X: x, Y: y})
		graph.Set(&edge.Properties, core.JunctionPoints, points)
	}
}

// TranslateAligned applies CoreOptions.CONTENT_ALIGNMENT after growth.
func TranslateAligned(model *graph.Model, parent graph.NodeID, newSize, oldSize geometry.Point) {
	alignments := graph.Get(&model.Node(parent).Properties, core.ContentAlignment)
	x, y := 0.0, 0.0
	if newSize.X > oldSize.X {
		if contains(alignments, "H_CENTER") {
			x = (newSize.X - oldSize.X) / 2
		} else if contains(alignments, "H_RIGHT") {
			x = newSize.X - oldSize.X
		}
	}
	if newSize.Y > oldSize.Y {
		if contains(alignments, "V_CENTER") {
			y = (newSize.Y - oldSize.Y) / 2
		} else if contains(alignments, "V_BOTTOM") {
			y = newSize.Y - oldSize.Y
		}
	}
	Translate(model, parent, x, y)
}

func AllOutgoingEdges(model *graph.Model, nodeID graph.NodeID) []graph.EdgeID {
	node := model.Node(nodeID)
	result := append([]graph.EdgeID(nil), node.OutgoingEdges...)
	for _, portID := range node.Ports {
		result = append(result, model.Port(portID).OutgoingEdges...)
	}
	return result
}

func AllIncomingEdges(model *graph.Model, nodeID graph.NodeID) []graph.EdgeID {
	node := model.Node(nodeID)
	result := append([]graph.EdgeID(nil), node.IncomingEdges...)
	for _, portID := range node.Ports {
		result = append(result, model.Port(portID).IncomingEdges...)
	}
	return result
}

func IsHierarchical(model *graph.Model, edgeID graph.EdgeID) bool {
	edge := model.Edge(edgeID)
	var common *graph.NodeID
	initialized := false
	for _, shape := range append(append([]graph.ShapeID(nil), edge.Sources...), edge.Targets...) {
		node := model.ShapeNode(shape)
		parent := model.Node(node).Parent
		if !initialized {
			initialized = true
			if parent != nil {
				copyOfParent := *parent
				common = &copyOfParent
			}
			continue
		}
		if (common == nil) != (parent == nil) || common != nil && *common != *parent {
			return true
		}
	}
	return false
}

// ApplyVectorChain writes a fixed route to one edge section.
func ApplyVectorChain(model *graph.Model, sectionID graph.SectionID, chain geometry.Chain) {
	section := model.Section(sectionID)
	section.Start = chain[0]
	section.End = chain[len(chain)-1]
	section.BendPoints = append(section.BendPoints[:0], chain[1:len(chain)-1]...)
	section.StartPresent, section.EndPresent = true, true
	section.BendPointsPresent = len(section.BendPoints) > 0
}

func sectionPoints(model *graph.Model, sectionID graph.SectionID) geometry.Chain {
	section := model.Section(sectionID)
	result := make(geometry.Chain, 0, len(section.BendPoints)+2)
	result = append(result, section.Start)
	result = append(result, section.BendPoints...)
	result = append(result, section.End)
	for index := 1; index < len(result)-1; {
		previous, current, next := result[index-1], result[index], result[index+1]
		if previous.X == current.X && current.X == next.X || previous.Y == current.Y && current.Y == next.Y {
			copy(result[index:], result[index+1:])
			result = result[:len(result)-1]
		} else {
			index++
		}
	}
	return result
}

// DetermineJunctionPoints implements ElkUtil.determineJunctionPoints.
func DetermineJunctionPoints(model *graph.Model, edgeID graph.EdgeID) (geometry.Chain, error) {
	edge := model.Edge(edgeID)
	if len(edge.Sections) != 1 {
		return nil, fmt.Errorf("edge needs exactly one edge section, found %d", len(edge.Sections))
	}
	var result geometry.Chain
	if edge.Sources[0].Kind == graph.PortShape {
		points, err := junctionPointsAtPort(model, edgeID, edge.Sources[0].Port, false)
		if err != nil {
			return nil, err
		}
		result = append(result, points...)
	}
	if edge.Targets[0].Kind == graph.PortShape {
		points, err := junctionPointsAtPort(model, edgeID, edge.Targets[0].Port, true)
		if err != nil {
			return nil, err
		}
		result = append(result, points...)
	}
	return result, nil
}

type connectedSection struct {
	points geometry.Chain
	offset geometry.Point
}

func junctionPointsAtPort(model *graph.Model, edgeID graph.EdgeID, portID graph.PortID, reverse bool) (geometry.Chain, error) {
	mainPoints := sectionPoints(model, model.Edge(edgeID).Sections[0])
	port := model.Port(portID)
	incident := append(append([]graph.EdgeID(nil), port.IncomingEdges...), port.OutgoingEdges...)
	connected := make([]connectedSection, 0, len(incident))
	for _, otherEdgeID := range incident {
		if otherEdgeID == edgeID {
			continue
		}
		otherEdge := model.Edge(otherEdgeID)
		if len(otherEdge.Sections) != 1 {
			return nil, fmt.Errorf("connected edge needs exactly one edge section, found %d", len(otherEdge.Sections))
		}
		otherPoints := sectionPoints(model, otherEdge.Sections[0])
		var offset geometry.Point
		if reverse {
			offset = mainPoints[len(mainPoints)-1].Sub(otherPoints[len(otherPoints)-1])
		} else {
			offset = mainPoints[0].Sub(otherPoints[0])
		}
		connected = append(connected, connectedSection{points: otherPoints, offset: offset})
	}
	if len(connected) == 0 {
		return nil, nil
	}
	index := func(i, length int) int {
		if reverse {
			return length - 1 - i
		}
		return i
	}
	p1 := mainPoints[index(0, len(mainPoints))]
	var result geometry.Chain
	for i := 1; i < len(mainPoints); i++ {
		p2 := mainPoints[index(i, len(mainPoints))]
		remaining := connected[:0]
		for _, other := range connected {
			if len(other.points) <= i {
				continue
			}
			p3 := other.points[index(i, len(other.points))].Add(other.offset)
			if p2 != p3 {
				dx2, dy2 := p2.X-p1.X, p2.Y-p1.Y
				dx3, dy3 := p3.X-p1.X, p3.Y-p1.Y
				if dx3*dy2 == dy3*dx2 && javaSignum(dx2) == javaSignum(dx3) && javaSignum(dy2) == javaSignum(dy3) {
					if math.Abs(dx2) < math.Abs(dx3) || math.Abs(dy2) < math.Abs(dy3) {
						result = append(result, p2)
					}
				} else if i > 1 {
					result = append(result, p1)
				}
				continue
			}
			remaining = append(remaining, other)
		}
		connected = remaining
		p1 = p2
	}
	return result, nil
}

func javaSignum(value float64) float64 {
	if value == 0 || math.IsNaN(value) {
		return value
	}
	if value > 0 {
		return 1
	}
	return -1
}
