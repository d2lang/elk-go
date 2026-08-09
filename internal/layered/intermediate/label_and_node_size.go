// Copyright (c) 2010, 2020 Kiel University and others.
// Copyright (c) 2017, 2019 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"math"
	"strings"

	"github.com/d2lang/elk-go/internal/common/nodespacing"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// ProcessLabelAndNodeSizes performs the post-ordering part of ELK 0.8.1's
// LabelAndNodeSizeProcessor for normal layered nodes. Original node and label
// sizing has already run on the imported ELK graph; the important second pass
// fixes the positions of ports whose order was chosen during phase 3.
//
// D2 either supplies FIXED_POS ports or zero-sized implicit ports. The latter
// use ELK's default DISTRIBUTED alignment. The implementation below also keeps
// the source algorithm's size, margin, spacing, border-offset, and alignment
// arithmetic for non-zero ports.
func ProcessLabelAndNodeSizes(arena *layered.LGraphArena, graphID layered.LGraphID) {
	for _, nodeID := range arena.GraphAllNodes(graphID) {
		node := arena.Node(nodeID)
		if node.Type != layered.NormalNode {
			continue
		}
		constraints := basegraph.Get(&node.Properties, layered.PortConstraintsOption)
		if constraints.IsRatioFixed() || constraints.IsPositionFixed() {
			// FIXED_RATIO/FIXED_POS preserve the coordinate along the selected
			// side, but NodeDimensionCalculation still snaps the orthogonal
			// coordinate to the (possibly resized) node border.
			for _, portID := range node.Ports {
				port := arena.Port(portID)
				placePortOnBorder(port, node, port.Side)
			}
		} else {
			for _, side := range []layered.PortSide{
				layered.NorthSide,
				layered.EastSide,
				layered.SouthSide,
				layered.WestSide,
			} {
				placePortsOnSide(arena, graphID, nodeID, side)
			}
			basegraph.Set(&node.Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedPosition)
		}
		placePortLabels(arena, graphID, nodeID)
		placeNodeLabels(arena, graphID, nodeID)
	}
}

type nodeLabelLocation struct {
	inside          bool
	horizontalAlign horizontalLabelAlignment
	verticalAlign   verticalLabelAlignment
	outsideSide     layered.PortSide
}

func placeNodeLabels(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	nodeID layered.LNodeID,
) {
	node := arena.Node(nodeID)
	defaultPlacement := basegraph.Get(&node.Properties, nodespacing.NodeLabelsPlacement)
	groups := make(map[nodeLabelLocation][]layered.LLabelID)
	locations := make([]nodeLabelLocation, 0)
	for _, labelID := range node.Labels {
		placement := defaultPlacement
		if individual, present := basegraph.TryGet(
			&arena.Label(labelID).Properties, nodespacing.NodeLabelsPlacement); present {
			placement = individual
		}
		location, valid := parseNodeLabelLocation(placement)
		if !valid {
			continue
		}
		if _, seen := groups[location]; !seen {
			locations = append(locations, location)
		}
		groups[location] = append(groups[location], labelID)
	}

	verticalLayout := directionIsVertical(
		basegraph.Get(&arena.Graph(graphID).Properties, layered.DirectionOption))
	gap := layered.IndividualOrDefault(arena, nodeID, &layered.SpacingLabelLabel)
	nodeLabelSpacing := layered.IndividualOrDefault(arena, nodeID, &layered.SpacingLabelNode)
	padding := individualNodeLabelPadding(arena, graphID, nodeID)
	for _, location := range locations {
		cell := newEndLabelCell(arena, groups[location], gap, !verticalLayout)
		cell.horizontalAlign = location.horizontalAlign
		cell.verticalAlign = location.verticalAlign
		cell.rect = nodeLabelCellRectangle(node, cell.minimum, padding, nodeLabelSpacing, location)
		applyEndLabelCell(arena, cell)
	}
}

func parseNodeLabelLocation(values []string) (nodeLabelLocation, bool) {
	location := nodeLabelLocation{outsideSide: layered.UndefinedSide}
	horizontalCount, verticalCount, insideOutsideCount := 0, 0, 0
	hPriority := false
	for _, value := range values {
		switch strings.ToUpper(strings.TrimSpace(value)) {
		case "H_LEFT":
			location.horizontalAlign, horizontalCount = horizontalLeft, horizontalCount+1
		case "H_CENTER":
			location.horizontalAlign, horizontalCount = horizontalCenter, horizontalCount+1
		case "H_RIGHT":
			location.horizontalAlign, horizontalCount = horizontalRight, horizontalCount+1
		case "V_TOP":
			location.verticalAlign, verticalCount = verticalTop, verticalCount+1
		case "V_CENTER":
			location.verticalAlign, verticalCount = verticalCenter, verticalCount+1
		case "V_BOTTOM":
			location.verticalAlign, verticalCount = verticalBottom, verticalCount+1
		case "INSIDE":
			location.inside, insideOutsideCount = true, insideOutsideCount+1
		case "OUTSIDE":
			location.inside, insideOutsideCount = false, insideOutsideCount+1
		case "H_PRIORITY":
			hPriority = true
		}
	}
	if horizontalCount != 1 || verticalCount != 1 || insideOutsideCount != 1 {
		return nodeLabelLocation{}, false
	}
	if location.inside {
		return location, true
	}
	if location.verticalAlign != verticalCenter &&
		(!hPriority || location.horizontalAlign == horizontalCenter) {
		if location.verticalAlign == verticalTop {
			location.outsideSide = layered.NorthSide
		} else {
			location.outsideSide = layered.SouthSide
		}
	} else if location.horizontalAlign != horizontalCenter {
		if location.horizontalAlign == horizontalLeft {
			location.outsideSide = layered.WestSide
		} else {
			location.outsideSide = layered.EastSide
		}
	}
	return location, location.outsideSide != layered.UndefinedSide
}

func individualNodeLabelPadding(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	nodeID layered.LNodeID,
) geometry.Spacing {
	node := arena.Node(nodeID)
	if individual, found := basegraph.TryGet(&node.Properties, layered.SpacingIndividual); found {
		if value, explicit := basegraph.TryGet(&individual.Properties, layered.NodeLabelsPadding); explicit {
			return value
		}
	}
	if value, explicit := basegraph.TryGet(&node.Properties, layered.NodeLabelsPadding); explicit {
		return value
	}
	return basegraph.Get(&arena.Graph(graphID).Properties, layered.NodeLabelsPadding)
}

func nodeLabelCellRectangle(
	node *layered.LNode,
	minimum geometry.Point,
	padding geometry.Spacing,
	labelNodeSpacing float64,
	location nodeLabelLocation,
) geometry.Rectangle {
	rectangle := geometry.Rectangle{Width: minimum.X, Height: minimum.Y}
	if location.inside {
		switch location.horizontalAlign {
		case horizontalLeft:
			rectangle.X = padding.Left
		case horizontalCenter:
			rectangle.X = (node.Size.X - minimum.X) / 2
		case horizontalRight:
			rectangle.X = node.Size.X - padding.Right - minimum.X
		}
		switch location.verticalAlign {
		case verticalTop:
			rectangle.Y = padding.Top
		case verticalCenter:
			rectangle.Y = (node.Size.Y - minimum.Y) / 2
		case verticalBottom:
			rectangle.Y = node.Size.Y - padding.Bottom - minimum.Y
		}
		return rectangle
	}
	switch location.outsideSide {
	case layered.NorthSide, layered.SouthSide:
		switch location.horizontalAlign {
		case horizontalLeft:
			rectangle.X = 0
		case horizontalCenter:
			rectangle.X = (node.Size.X - minimum.X) / 2
		case horizontalRight:
			rectangle.X = node.Size.X - minimum.X
		}
		if location.outsideSide == layered.NorthSide {
			rectangle.Y = -labelNodeSpacing - minimum.Y
		} else {
			rectangle.Y = node.Size.Y + labelNodeSpacing
		}
	case layered.WestSide, layered.EastSide:
		switch location.verticalAlign {
		case verticalTop:
			rectangle.Y = 0
		case verticalCenter:
			rectangle.Y = (node.Size.Y - minimum.Y) / 2
		case verticalBottom:
			rectangle.Y = node.Size.Y - minimum.Y
		}
		if location.outsideSide == layered.WestSide {
			rectangle.X = -labelNodeSpacing - minimum.X
		} else {
			rectangle.X = node.Size.X + labelNodeSpacing
		}
	}
	return rectangle
}

func placePortLabels(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	nodeID layered.LNodeID,
) {
	node := arena.Node(nodeID)
	placement := append([]string(nil),
		basegraph.Get(&node.Properties, nodespacing.PortLabelsPlacement)...)
	if basegraph.Get(&node.Properties, core.PortLabelsNextToPortIfPossible) &&
		!hasPlacement(placement, "NEXT_TO_PORT_IF_POSSIBLE") {
		placement = append(placement, "NEXT_TO_PORT_IF_POSSIBLE")
	}
	inside, outside := hasPlacement(placement, "INSIDE"), hasPlacement(placement, "OUTSIDE")
	if inside == outside {
		return
	}
	verticalLayout := directionIsVertical(
		basegraph.Get(&arena.Graph(graphID).Properties, layered.DirectionOption))
	gap := layered.IndividualOrDefault(arena, nodeID, &layered.SpacingLabelLabel)
	horizontalSpacing := layered.IndividualOrDefault(
		arena, nodeID, &layered.SpacingLabelPortHorizontal)
	verticalSpacing := layered.IndividualOrDefault(
		arena, nodeID, &layered.SpacingLabelPortVertical)
	borderPadding := nodeContainerBorderPadding(arena, nodeID)
	nextIfPossible := hasPlacement(placement, "NEXT_TO_PORT_IF_POSSIBLE")
	treatAsGroup := basegraph.Get(&node.Properties, core.PortLabelsTreatAsGroup)
	alwaysSame := hasPlacement(placement, "ALWAYS_SAME_SIDE")
	alwaysOther := hasPlacement(placement, "ALWAYS_OTHER_SAME_SIDE")
	spaceEfficient := hasPlacement(placement, "SPACE_EFFICIENT")
	compound := node.NestedGraph != nil ||
		basegraph.Get(&node.Properties, core.InsideSelfLoopsActivate)

	for _, side := range []layered.PortSide{
		layered.NorthSide, layered.EastSide, layered.SouthSide, layered.WestSide,
	} {
		ports := portsInPlacementOrder(arena, nodeID, side)
		if len(ports) == 0 {
			continue
		}
		firstNext := portLabelsNextToPort(
			arena, ports[0], inside, compound, nextIfPossible)
		placeFirstDifferently := len(ports) >= 2 && !firstNext && !alwaysSame &&
			(len(ports) == 2 || spaceEfficient)
		for _, portID := range ports {
			port := arena.Port(portID)
			if len(port.Labels) == 0 {
				continue
			}
			cell := newEndLabelCell(arena, port.Labels, gap, !verticalLayout)
			firstLabelHeight := arena.Label(port.Labels[0]).Size.Y
			next := portLabelsNextToPort(arena, portID, inside, compound, nextIfPossible)
			if inside {
				placeInsidePortLabelCell(
					cell, port, borderPadding, horizontalSpacing, verticalSpacing,
					next, treatAsGroup, firstLabelHeight)
			} else {
				placeOutsidePortLabelCell(
					cell, port, horizontalSpacing, verticalSpacing,
					next, treatAsGroup, firstLabelHeight,
					placeFirstDifferently || alwaysOther)
			}
			applyEndLabelCell(arena, cell)
			placeFirstDifferently = false
		}
	}
}

func portsInPlacementOrder(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	side layered.PortSide,
) []layered.LPortID {
	ports := arena.NodePortsOnSide(nodeID, side)
	if side == layered.SouthSide || side == layered.WestSide {
		for left, right := 0, len(ports)-1; left < right; left, right = left+1, right-1 {
			ports[left], ports[right] = ports[right], ports[left]
		}
	}
	return ports
}

func hasPlacement(values []string, wanted string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), wanted) {
			return true
		}
	}
	return false
}

func portLabelsNextToPort(
	arena *layered.LGraphArena,
	portID layered.LPortID,
	inside, compound, requested bool,
) bool {
	port := arena.Port(portID)
	if inside {
		if !compound {
			return true
		}
		return requested && !basegraph.Get(&port.Properties, layered.InsideConnections)
	}
	return requested && len(port.IncomingEdges) == 0 && len(port.OutgoingEdges) == 0
}

func nodeContainerBorderPadding(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
) geometry.Spacing {
	var padding geometry.Spacing
	for _, portID := range arena.Node(nodeID).Ports {
		port := arena.Port(portID)
		offset := basegraph.Get(&port.Properties, layered.PortBorderOffset)
		if offset >= 0 {
			continue
		}
		inside := -offset
		switch port.Side {
		case layered.NorthSide:
			padding.Top = math.Max(padding.Top, inside)
		case layered.EastSide:
			padding.Right = math.Max(padding.Right, inside)
		case layered.SouthSide:
			padding.Bottom = math.Max(padding.Bottom, inside)
		case layered.WestSide:
			padding.Left = math.Max(padding.Left, inside)
		}
	}
	return padding
}

func placeInsidePortLabelCell(
	cell *endLabelCell,
	port *layered.LPort,
	borderPadding geometry.Spacing,
	horizontalSpacing, verticalSpacing float64,
	next, treatAsGroup bool,
	firstLabelHeight float64,
) {
	offset := basegraph.Get(&port.Properties, layered.PortBorderOffset)
	switch port.Side {
	case layered.NorthSide:
		cell.horizontalAlign, cell.verticalAlign = horizontalCenter, verticalTop
		if next {
			cell.rect.X = (port.Size.X - cell.rect.Width) / 2
		} else {
			cell.rect.X = port.Size.X + horizontalSpacing
		}
		cell.rect.Y = port.Size.Y + offset + borderPadding.Top + verticalSpacing
	case layered.SouthSide:
		cell.horizontalAlign, cell.verticalAlign = horizontalCenter, verticalBottom
		if next {
			cell.rect.X = (port.Size.X - cell.rect.Width) / 2
		} else {
			cell.rect.X = port.Size.X + horizontalSpacing
		}
		cell.rect.Y = -offset - borderPadding.Bottom - verticalSpacing - cell.rect.Height
	case layered.EastSide:
		cell.horizontalAlign, cell.verticalAlign = horizontalRight, verticalCenter
		cell.rect.X = -offset - borderPadding.Right - horizontalSpacing - cell.rect.Width
		cell.rect.Y = verticalPortLabelY(
			cell, port, verticalSpacing, next, treatAsGroup, firstLabelHeight)
	case layered.WestSide:
		cell.horizontalAlign, cell.verticalAlign = horizontalLeft, verticalCenter
		cell.rect.X = port.Size.X + offset + borderPadding.Left + horizontalSpacing
		cell.rect.Y = verticalPortLabelY(
			cell, port, verticalSpacing, next, treatAsGroup, firstLabelHeight)
	}
}

func placeOutsidePortLabelCell(
	cell *endLabelCell,
	port *layered.LPort,
	horizontalSpacing, verticalSpacing float64,
	next, treatAsGroup bool,
	firstLabelHeight float64,
	otherSide bool,
) {
	switch port.Side {
	case layered.NorthSide:
		cell.verticalAlign = verticalBottom
		if next {
			cell.horizontalAlign = horizontalCenter
			cell.rect.X = (port.Size.X - cell.rect.Width) / 2
		} else if otherSide {
			cell.horizontalAlign = horizontalRight
			cell.rect.X = -cell.rect.Width - horizontalSpacing
		} else {
			cell.horizontalAlign = horizontalLeft
			cell.rect.X = port.Size.X + horizontalSpacing
		}
		cell.rect.Y = -cell.rect.Height - verticalSpacing
	case layered.SouthSide:
		cell.verticalAlign = verticalTop
		if next {
			cell.horizontalAlign = horizontalCenter
			cell.rect.X = (port.Size.X - cell.rect.Width) / 2
		} else if otherSide {
			cell.horizontalAlign = horizontalRight
			cell.rect.X = -cell.rect.Width - horizontalSpacing
		} else {
			cell.horizontalAlign = horizontalLeft
			cell.rect.X = port.Size.X + horizontalSpacing
		}
		cell.rect.Y = port.Size.Y + verticalSpacing
	case layered.EastSide:
		cell.horizontalAlign = horizontalLeft
		cell.rect.X = port.Size.X + horizontalSpacing
		if next {
			cell.verticalAlign = verticalCenter
			cell.rect.Y = verticalPortLabelY(
				cell, port, verticalSpacing, true, treatAsGroup, firstLabelHeight)
		} else if otherSide {
			cell.verticalAlign = verticalBottom
			cell.rect.Y = -cell.rect.Height - verticalSpacing
		} else {
			cell.verticalAlign = verticalTop
			cell.rect.Y = port.Size.Y + verticalSpacing
		}
	case layered.WestSide:
		cell.horizontalAlign = horizontalRight
		cell.rect.X = -cell.rect.Width - horizontalSpacing
		if next {
			cell.verticalAlign = verticalCenter
			cell.rect.Y = verticalPortLabelY(
				cell, port, verticalSpacing, true, treatAsGroup, firstLabelHeight)
		} else if otherSide {
			cell.verticalAlign = verticalBottom
			cell.rect.Y = -cell.rect.Height - verticalSpacing
		} else {
			cell.verticalAlign = verticalTop
			cell.rect.Y = port.Size.Y + verticalSpacing
		}
	}
}

func verticalPortLabelY(
	cell *endLabelCell,
	port *layered.LPort,
	verticalSpacing float64,
	next, treatAsGroup bool,
	firstLabelHeight float64,
) float64 {
	if !next {
		return port.Size.Y + verticalSpacing
	}
	labelHeight := cell.rect.Height
	if !treatAsGroup {
		labelHeight = firstLabelHeight
	}
	return (port.Size.Y - labelHeight) / 2
}

func placePortsOnSide(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	nodeID layered.LNodeID,
	side layered.PortSide,
) {
	node := arena.Node(nodeID)
	ports := make([]layered.LPortID, 0, len(node.Ports))
	for _, portID := range node.Ports {
		if arena.Port(portID).Side == side {
			ports = append(ports, portID)
		}
	}
	if len(ports) == 0 {
		return
	}
	// NodeContext's TreeMultimap presents ports left-to-right / top-to-bottom,
	// whereas LNode's canonical clockwise order runs backwards on S and W.
	if side == layered.SouthSide || side == layered.WestSide {
		for left, right := 0, len(ports)-1; left < right; left, right = left+1, right-1 {
			ports[left], ports[right] = ports[right], ports[left]
		}
	}

	horizontal := side == layered.NorthSide || side == layered.SouthSide
	length := node.Size.Y
	startMargin := basegraph.Get(&arena.Graph(graphID).Properties, layered.SpacingPortsSurrounding).Top
	endMargin := basegraph.Get(&arena.Graph(graphID).Properties, layered.SpacingPortsSurrounding).Bottom
	if horizontal {
		length = node.Size.X
		startMargin = basegraph.Get(&arena.Graph(graphID).Properties, layered.SpacingPortsSurrounding).Left
		endMargin = basegraph.Get(&arena.Graph(graphID).Properties, layered.SpacingPortsSurrounding).Right
	}

	occupied := 0.0
	for _, portID := range ports {
		port := arena.Port(portID)
		if horizontal {
			occupied += port.Margin.Left + port.Size.X + port.Margin.Right
		} else {
			occupied += port.Margin.Top + port.Size.Y + port.Margin.Bottom
		}
	}
	available := length - startMargin - endMargin
	gap := individualPortSpacing(arena, graphID, nodeID)
	alignment := portAlignment(node, side)
	placementSize := occupied + gap*float64(len(ports)-1)
	position := startMargin

	if len(ports) == 1 && (alignment == "DISTRIBUTED" || alignment == "JUSTIFIED") {
		alignment = "CENTER"
	}
	switch alignment {
	case "BEGIN":
	case "END":
		position += available - placementSize
	case "JUSTIFIED":
		if len(ports) > 1 {
			gap += (available - placementSize) / float64(len(ports)-1)
		}
	case "CENTER":
		position += (available - placementSize) / 2
	default: // DISTRIBUTED is ELK Layered's effective default for this path.
		gap += (available - (occupied + gap*float64(len(ports)+1))) / float64(len(ports)+1)
		position += gap
	}

	for _, portID := range ports {
		port := arena.Port(portID)
		if horizontal {
			position += port.Margin.Left
			port.Position.X = position
			position += port.Size.X + port.Margin.Right + gap
		} else {
			position += port.Margin.Top
			port.Position.Y = position
			position += port.Size.Y + port.Margin.Bottom + gap
		}
		placePortOnBorder(port, node, side)
	}
}

func placePortOnBorder(port *layered.LPort, node *layered.LNode, side layered.PortSide) {
	offset := basegraph.Get(&port.Properties, layered.PortBorderOffset)
	switch side {
	case layered.NorthSide:
		port.Position.Y = -port.Size.Y - offset
	case layered.EastSide:
		port.Position.X = node.Size.X + offset
	case layered.SouthSide:
		port.Position.Y = node.Size.Y + offset
	case layered.WestSide:
		port.Position.X = -port.Size.X - offset
	}
}

func individualPortSpacing(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	nodeID layered.LNodeID,
) float64 {
	if individual, ok := basegraph.TryGet(&arena.Node(nodeID).Properties, layered.SpacingIndividual); ok {
		if value, explicit := basegraph.TryGet(&individual.Properties, layered.SpacingPortPort); explicit {
			return value
		}
	}
	return basegraph.Get(&arena.Graph(graphID).Properties, layered.SpacingPortPort)
}

func portAlignment(node *layered.LNode, side layered.PortSide) string {
	suffix := map[layered.PortSide]string{
		layered.NorthSide: "north",
		layered.EastSide:  "east",
		layered.SouthSide: "south",
		layered.WestSide:  "west",
	}[side]
	if value, ok := node.Properties.Raw("org.eclipse.elk.portAlignment." + suffix); ok {
		return strings.ToUpper(strings.TrimSpace(toString(value)))
	}
	if value, ok := node.Properties.Raw("org.eclipse.elk.portAlignment.default"); ok {
		return strings.ToUpper(strings.TrimSpace(toString(value)))
	}
	return "DISTRIBUTED"
}

func toString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
