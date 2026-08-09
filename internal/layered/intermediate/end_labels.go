// Copyright (c) 2012, 2020 Kiel University and others.
// Copyright (c) 2015, 2017 Kiel University and others.
// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2017, 2019 Kiel University and others.
// Copyright (c) 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"math"
	"sort"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
)

// The types below are the small, end-label-specific subset of ELK 0.8.1's
// nodespacing LabelCell. Keeping the cell alive between phases four and five
// is important: EndLabelSorter reorders its labels without recomputing its
// dimensions, then EndLabelPostprocessor applies the final node offset.
type horizontalLabelAlignment uint8

const (
	horizontalLeft horizontalLabelAlignment = iota
	horizontalCenter
	horizontalRight
)

type verticalLabelAlignment uint8

const (
	verticalTop verticalLabelAlignment = iota
	verticalCenter
	verticalBottom
)

type endLabelCell struct {
	labels           []layered.LLabelID
	rect             geometry.Rectangle
	minimum          geometry.Point
	gap              float64
	horizontalLayout bool
	horizontalAlign  horizontalLabelAlignment
	verticalAlign    verticalLabelAlignment
}

var endLabelCells = basegraph.NewProperty[map[layered.LPortID]*endLabelCell]("endLabels")

func newEndLabelCell(
	arena *layered.LGraphArena,
	labels []layered.LLabelID,
	gap float64,
	horizontalLayout bool,
) *endLabelCell {
	if len(labels) == 0 {
		return nil
	}
	cell := &endLabelCell{
		labels:           append([]layered.LLabelID(nil), labels...),
		gap:              gap,
		horizontalLayout: horizontalLayout,
		horizontalAlign:  horizontalCenter,
		verticalAlign:    verticalCenter,
	}
	for index, labelID := range cell.labels {
		size := arena.Label(labelID).Size
		if horizontalLayout {
			cell.minimum.X = math.Max(cell.minimum.X, size.X)
			cell.minimum.Y += size.Y
			if index > 0 {
				cell.minimum.Y += gap
			}
		} else {
			cell.minimum.X += size.X
			cell.minimum.Y = math.Max(cell.minimum.Y, size.Y)
			if index > 0 {
				cell.minimum.X += gap
			}
		}
	}
	cell.rect.Width = cell.minimum.X
	cell.rect.Height = cell.minimum.Y
	return cell
}

// PreprocessEndLabels is the exact ELK 0.8.1 EndLabelPreprocessor pipeline for
// the arena graph. It gathers HEAD and TAIL labels into per-port cells, places
// those cells relative to their nodes, removes strip overlaps, and extends the
// node margins to cover the result.
func PreprocessEndLabels(arena *layered.LGraphArena, graphID layered.LGraphID) {
	graph := arena.Graph(graphID)
	edgeLabelSpacing := basegraph.Get(&graph.Properties, layered.SpacingEdgeLabel)
	labelLabelSpacing := basegraph.Get(&graph.Properties, layered.SpacingLabelLabel)
	verticalLayout := directionIsVertical(basegraph.Get(&graph.Properties, layered.DirectionOption))

	for _, layerID := range graph.Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			preprocessNodeEndLabels(arena, nodeID, edgeLabelSpacing, labelLabelSpacing, verticalLayout)
		}
	}
}

func preprocessNodeEndLabels(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	edgeLabelSpacing, labelLabelSpacing float64,
	verticalLayout bool,
) {
	node := arena.Node(nodeID)
	portCells := make([]*endLabelCell, len(node.Ports))
	for index, portID := range node.Ports {
		arena.Port(portID).ID = index
		labels, _ := gatherPortEndLabels(arena, portID)
		portCells[index] = newEndLabelCell(arena, labels, labelLabelSpacing, !verticalLayout)
	}

	placeEndLabels(arena, nodeID, portCells, labelLabelSpacing, edgeLabelSpacing, verticalLayout)

	cellsByPort := make(map[layered.LPortID]*endLabelCell)
	for index, cell := range portCells {
		if cell != nil {
			cellsByPort[node.Ports[index]] = cell
		}
	}
	if len(cellsByPort) == 0 {
		return
	}
	basegraph.Set(&node.Properties, endLabelCells, cellsByPort)
	updateEndLabelNodeMargins(node, portCells)
}

func placeEndLabels(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	portCells []*endLabelCell,
	labelLabelSpacing, edgeLabelSpacing float64,
	verticalLayout bool,
) {
	for _, portID := range arena.Node(nodeID).Ports {
		port := arena.Port(portID)
		if port.ID >= 0 && port.ID < len(portCells) && portCells[port.ID] != nil {
			placePortEndLabels(arena, portID, portCells[port.ID], edgeLabelSpacing)
		}
	}

	// In horizontal layouts north/south labels can overlap. In vertical
	// layouts the same is true of east/west labels.
	if verticalLayout {
		removeEndLabelOverlaps(arena, nodeID, portCells, layered.EastSide,
			2*labelLabelSpacing, edgeLabelSpacing)
		removeEndLabelOverlaps(arena, nodeID, portCells, layered.WestSide,
			2*labelLabelSpacing, edgeLabelSpacing)
	} else {
		removeEndLabelOverlaps(arena, nodeID, portCells, layered.NorthSide,
			2*labelLabelSpacing, edgeLabelSpacing)
		removeEndLabelOverlaps(arena, nodeID, portCells, layered.SouthSide,
			2*labelLabelSpacing, edgeLabelSpacing)
	}
}

func placePortEndLabels(
	arena *layered.LGraphArena,
	portID layered.LPortID,
	cell *endLabelCell,
	edgeLabelSpacing float64,
) {
	port := arena.Port(portID)
	if port.Node == nil {
		return
	}
	node := arena.Node(*port.Node)
	portAnchor := port.Position.Add(port.Anchor)
	maximumThickness := basegraph.Get(&port.Properties, layered.MaximumEdgeThickness)
	labelSide := basegraph.Get(&arena.Label(cell.labels[0]).Properties, layered.InternalLabelSide)

	switch port.Side {
	case layered.NorthSide:
		cell.verticalAlign = verticalBottom
		cell.rect.Y = -node.Margin.Top - edgeLabelSpacing - cell.rect.Height
		if labelSide == layered.LabelSideAbove {
			cell.horizontalAlign = horizontalRight
			cell.rect.X = portAnchor.X - maximumThickness - edgeLabelSpacing - cell.rect.Width
		} else {
			cell.horizontalAlign = horizontalLeft
			cell.rect.X = portAnchor.X + maximumThickness + edgeLabelSpacing
		}
	case layered.EastSide:
		cell.horizontalAlign = horizontalLeft
		cell.rect.X = node.Size.X + node.Margin.Right + edgeLabelSpacing
		if labelSide == layered.LabelSideAbove {
			cell.verticalAlign = verticalBottom
			cell.rect.Y = portAnchor.Y - maximumThickness - edgeLabelSpacing - cell.rect.Height
		} else {
			cell.verticalAlign = verticalTop
			cell.rect.Y = portAnchor.Y + maximumThickness + edgeLabelSpacing
		}
	case layered.SouthSide:
		cell.verticalAlign = verticalTop
		cell.rect.Y = node.Size.Y + node.Margin.Bottom + edgeLabelSpacing
		if labelSide == layered.LabelSideAbove {
			cell.horizontalAlign = horizontalRight
			cell.rect.X = portAnchor.X - maximumThickness - edgeLabelSpacing - cell.rect.Width
		} else {
			cell.horizontalAlign = horizontalLeft
			cell.rect.X = portAnchor.X + maximumThickness + edgeLabelSpacing
		}
	case layered.WestSide:
		cell.horizontalAlign = horizontalRight
		cell.rect.X = -node.Margin.Left - edgeLabelSpacing - cell.rect.Width
		if labelSide == layered.LabelSideAbove {
			cell.verticalAlign = verticalBottom
			cell.rect.Y = portAnchor.Y - maximumThickness - edgeLabelSpacing - cell.rect.Height
		} else {
			cell.verticalAlign = verticalTop
			cell.rect.Y = portAnchor.Y + maximumThickness + edgeLabelSpacing
		}
	}
}

type overlapRemovalDirection uint8

const (
	overlapUp overlapRemovalDirection = iota
	overlapDown
	overlapLeft
	overlapRight
)

type endLabelOverlapNode struct {
	cell     *endLabelCell
	x        float64
	width    float64
	height   float64
	y        float64
	overlaps []*endLabelOverlapNode
}

func removeEndLabelOverlaps(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	portCells []*endLabelCell,
	portSide layered.PortSide,
	gap, edgeLabelSpacing float64,
) {
	direction := overlapDirectionForPortSide(portSide)
	start := overlapStartCoordinate(arena.Node(nodeID), portSide, edgeLabelSpacing)
	nodes := make([]*endLabelOverlapNode, 0)
	for _, portID := range arena.NodePortsOnSide(nodeID, portSide) {
		port := arena.Port(portID)
		if port.ID < 0 || port.ID >= len(portCells) || portCells[port.ID] == nil {
			continue
		}
		cell := portCells[port.ID]
		node := &endLabelOverlapNode{cell: cell}
		if direction == overlapUp || direction == overlapDown {
			node.x, node.width, node.height = cell.rect.X, cell.rect.Width, cell.rect.Height
		} else {
			node.x, node.width, node.height = cell.rect.Y, cell.rect.Height, cell.rect.Width
		}
		nodes = append(nodes, node)
	}
	if len(nodes) == 0 {
		return
	}

	// RectangleStripOverlapRemover first stably orders left borders and uses a
	// right-border-ordered scanline set to construct the overlap graph.
	sort.SliceStable(nodes, func(i, j int) bool {
		return javacompat.DoubleCompare(nodes[i].x, nodes[j].x) < 0
	})
	active := make([]*endLabelOverlapNode, 0, len(nodes))
	for _, current := range nodes {
		kept := active[:0]
		for _, candidate := range active {
			if candidate.x+candidate.width >= current.x {
				kept = append(kept, candidate)
			}
		}
		active = kept
		sort.SliceStable(active, func(i, j int) bool {
			left := active[i].x + active[i].width
			right := active[j].x + active[j].width
			return javacompat.DoubleCompare(left, right) < 0
		})
		for _, candidate := range active {
			candidate.overlaps = append(candidate.overlaps, current)
			current.overlaps = append(current.overlaps, candidate)
		}
		// The source uses a TreeSet whose comparator only considers the right
		// border. A rectangle with an equal right border is therefore not added
		// to the active scanline set, even though it remains in the main list.
		currentRight := current.x + current.width
		duplicateRightBorder := false
		for _, candidate := range active {
			if javacompat.DoubleCompare(candidate.x+candidate.width, currentRight) == 0 {
				duplicateRightBorder = true
				break
			}
		}
		if !duplicateRightBorder {
			active = append(active, current)
		}
	}

	placed := make(map[*endLabelOverlapNode]bool, len(nodes))
	for _, current := range nodes {
		current.y = 0
		sort.SliceStable(current.overlaps, func(i, j int) bool {
			return javacompat.DoubleCompare(current.overlaps[i].y, current.overlaps[j].y) < 0
		})
		for _, overlap := range current.overlaps {
			if !placed[overlap] {
				continue
			}
			if current.y < overlap.y+overlap.height+gap &&
				current.y+current.height+gap > overlap.y {
				current.y = overlap.y + overlap.height + gap
			}
		}
		placed[current] = true
	}

	for _, node := range nodes {
		switch direction {
		case overlapUp:
			node.cell.rect.Y = start - node.height - node.y
		case overlapDown:
			node.cell.rect.Y = start + node.y
		case overlapLeft:
			node.cell.rect.X = start - node.height - node.y
		case overlapRight:
			node.cell.rect.X = start + node.y
		}
	}
}

func overlapDirectionForPortSide(side layered.PortSide) overlapRemovalDirection {
	switch side {
	case layered.NorthSide:
		return overlapUp
	case layered.SouthSide:
		return overlapDown
	case layered.EastSide:
		return overlapRight
	case layered.WestSide:
		return overlapLeft
	default:
		panic("layered: end-label overlap removal requested for undefined port side")
	}
}

func overlapStartCoordinate(node *layered.LNode, side layered.PortSide, edgeLabelSpacing float64) float64 {
	switch side {
	case layered.NorthSide:
		return -node.Margin.Top - edgeLabelSpacing
	case layered.SouthSide:
		return node.Size.Y + node.Margin.Bottom + edgeLabelSpacing
	case layered.EastSide:
		return node.Size.X + node.Margin.Right + edgeLabelSpacing
	case layered.WestSide:
		return -node.Margin.Left - edgeLabelSpacing
	default:
		panic("layered: end-label overlap start requested for undefined port side")
	}
}

func updateEndLabelNodeMargins(node *layered.LNode, cells []*endLabelCell) {
	marginRect := geometry.Rectangle{
		X:      -node.Margin.Left,
		Y:      -node.Margin.Top,
		Width:  node.Margin.Left + node.Size.X + node.Margin.Right,
		Height: node.Margin.Top + node.Size.Y + node.Margin.Bottom,
	}
	for _, cell := range cells {
		if cell != nil {
			marginRect = marginRect.Union(cell.rect)
		}
	}
	node.Margin.Left = -marginRect.X
	node.Margin.Top = -marginRect.Y
	node.Margin.Right = marginRect.Width - node.Margin.Left - node.Size.X
	node.Margin.Bottom = marginRect.Height - node.Margin.Top - node.Size.Y
}

// SortEndLabels ports ELK 0.8.1's EndLabelSorter. Groups of labels belonging
// to one edge retain their input order; groups are ordered by the opposite
// endpoint's final layer/port order.
func SortEndLabels(arena *layered.LGraphArena, graphID layered.LGraphID) {
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			if arena.Node(nodeID).Type == layered.NormalNode {
				sortNodeEndLabels(arena, graphID, nodeID)
			}
		}
	}
}

func sortNodeEndLabels(arena *layered.LGraphArena, graphID layered.LGraphID, nodeID layered.LNodeID) {
	cellsByPort, found := basegraph.TryGet(&arena.Node(nodeID).Properties, endLabelCells)
	if !found {
		return
	}
	initialized := false
	for _, portID := range arena.Node(nodeID).Ports {
		if !endLabelsNeedSorting(arena, portID) {
			continue
		}
		if !initialized {
			initializeEndLabelOrderIDs(arena, graphID)
			initialized = true
		}
		if cell := cellsByPort[portID]; cell != nil {
			sortEndLabelCell(arena, cell)
		}
	}
}

func endLabelsNeedSorting(arena *layered.LGraphArena, portID layered.LPortID) bool {
	edgesWithEndLabels := 0
	port := arena.Port(portID)
	for _, edgeID := range port.IncomingEdges {
		if edgeHasLabelAt(arena, edgeID, layered.EdgeLabelPlacementHead) {
			edgesWithEndLabels++
		}
	}
	for _, edgeID := range port.OutgoingEdges {
		if edgeHasLabelAt(arena, edgeID, layered.EdgeLabelPlacementTail) {
			edgesWithEndLabels++
		}
	}
	return edgesWithEndLabels >= 2
}

func edgeHasLabelAt(arena *layered.LGraphArena, edgeID layered.LEdgeID, placement layered.EdgeLabelPlacement) bool {
	for _, labelID := range arena.Edge(edgeID).Labels {
		if basegraph.Get(&arena.Label(labelID).Properties, layered.EdgeLabelsPlacement) == placement {
			return true
		}
	}
	return false
}

func initializeEndLabelOrderIDs(arena *layered.LGraphArena, graphID layered.LGraphID) {
	nextID := 0
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			arena.Node(nodeID).ID = nextID
			nextID++
			for _, portID := range arena.Node(nodeID).Ports {
				arena.Port(portID).ID = nextID
				nextID++
			}
		}
	}
}

type endLabelGroup struct {
	edge    layered.LEdgeID
	hasEdge bool
	labels  []layered.LLabelID
}

func sortEndLabelCell(arena *layered.LGraphArena, cell *endLabelCell) {
	groups := make([]endLabelGroup, 0)
	indices := make(map[layered.LEdgeID]int)
	missingIndex := -1
	for _, labelID := range cell.labels {
		edgeID, found := basegraph.TryGet(&arena.Label(labelID).Properties, layered.EndLabelEdge)
		index := -1
		if found {
			if existing, ok := indices[edgeID]; ok {
				index = existing
			} else {
				index = len(groups)
				indices[edgeID] = index
				groups = append(groups, endLabelGroup{edge: edgeID, hasEdge: true})
			}
		} else if missingIndex >= 0 {
			index = missingIndex
		} else {
			index = len(groups)
			missingIndex = index
			groups = append(groups, endLabelGroup{})
		}
		groups[index].labels = append(groups[index].labels, labelID)
	}

	sort.SliceStable(groups, func(i, j int) bool {
		left, right := groups[i], groups[j]
		if !left.hasEdge || !right.hasEdge {
			return left.hasEdge && !right.hasEdge
		}
		leftEdge, rightEdge := arena.Edge(left.edge), arena.Edge(right.edge)
		leftSourceID := arena.Port(*leftEdge.Source).ID
		rightSourceID := arena.Port(*rightEdge.Source).ID
		if leftSourceID != rightSourceID {
			return leftSourceID < rightSourceID
		}
		leftTargetNodeID := arena.Node(*arena.Port(*leftEdge.Target).Node).ID
		rightTargetNodeID := arena.Node(*arena.Port(*rightEdge.Target).Node).ID
		if leftTargetNodeID != rightTargetNodeID {
			return leftTargetNodeID < rightTargetNodeID
		}
		return arena.Port(*leftEdge.Target).ID > arena.Port(*rightEdge.Target).ID
	})

	cell.labels = cell.labels[:0]
	for _, group := range groups {
		cell.labels = append(cell.labels, group.labels...)
	}
}

// PostprocessEndLabels offsets all cells by their final node positions and
// applies the LabelCell layout to the individual labels.
func PostprocessEndLabels(arena *layered.LGraphArena, graphID layered.LGraphID) {
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			node := arena.Node(nodeID)
			if node.Type != layered.NormalNode {
				continue
			}
			cellsByPort, found := basegraph.TryGet(&node.Properties, endLabelCells)
			if !found {
				continue
			}
			for _, portID := range node.Ports {
				if cell := cellsByPort[portID]; cell != nil {
					cell.rect = cell.rect.Move(node.Position)
					applyEndLabelCell(arena, cell)
				}
			}
			basegraph.Unset(&node.Properties, endLabelCells)
		}
	}
}

func applyEndLabelCell(arena *layered.LGraphArena, cell *endLabelCell) {
	if cell.horizontalLayout {
		y := cell.rect.Y
		switch cell.verticalAlign {
		case verticalCenter:
			y += (cell.rect.Height - cell.minimum.Y) / 2
		case verticalBottom:
			y += cell.rect.Height - cell.minimum.Y
		}
		for _, labelID := range cell.labels {
			label := arena.Label(labelID)
			label.Position.Y = y
			y += label.Size.Y + cell.gap
			switch cell.horizontalAlign {
			case horizontalLeft:
				label.Position.X = cell.rect.X
			case horizontalCenter:
				label.Position.X = cell.rect.X + (cell.rect.Width-label.Size.X)/2
			case horizontalRight:
				label.Position.X = cell.rect.X + cell.rect.Width - label.Size.X
			}
		}
		return
	}

	x := cell.rect.X
	switch cell.horizontalAlign {
	case horizontalCenter:
		x += (cell.rect.Width - cell.minimum.X) / 2
	case horizontalRight:
		x += cell.rect.Width - cell.minimum.X
	}
	for _, labelID := range cell.labels {
		label := arena.Label(labelID)
		label.Position.X = x
		x += label.Size.X + cell.gap
		switch cell.verticalAlign {
		case verticalTop:
			label.Position.Y = cell.rect.Y
		case verticalCenter:
			label.Position.Y = cell.rect.Y + (cell.rect.Height-label.Size.Y)/2
		case verticalBottom:
			label.Position.Y = cell.rect.Y + cell.rect.Height - label.Size.Y
		}
	}
}

// ManageEndNodePortLabels corresponds to LabelManagementProcessor(false).
// JSON ELK graphs cannot carry Java's ILabelManager callback, so the exact
// elkjs/D2-active behavior is an intentional no-op.
func ManageEndNodePortLabels(_ *layered.LGraphArena, _ layered.LGraphID) {}
