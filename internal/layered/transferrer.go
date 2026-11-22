// Copyright (c) 2008, 2020 Kiel University and others.
// Copyright (c) 2015, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"fmt"
	"strings"

	"github.com/d2lang/elk-go/internal/basicoptions"
	"github.com/d2lang/elk-go/internal/common/elkutil"
	"github.com/d2lang/elk-go/internal/common/nodespacing"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
)

type ElkGraphLayoutTransferrer struct {
	Model *basegraph.Model
	Maps  ImportMaps

	// Per-transfer scratch: the graphs already applied; per node, the stamp of
	// the graph visit that applied it (each visit is a fresh set: a visit
	// restores the stamps it overwrote when it ends, through undo); and the
	// section endpoint shapes, cut from one block.
	graphSeen []bool
	nodeStamp []uint32
	stamp     uint32
	undo      []stampUndo
	shapes    []basegraph.ShapeID
}

type stampUndo struct {
	node     LNodeID
	previous uint32
}

func NewElkGraphLayoutTransferrer(model *basegraph.Model, maps ImportMaps) *ElkGraphLayoutTransferrer {
	return &ElkGraphLayoutTransferrer{Model: model, Maps: maps}
}

func (transferrer *ElkGraphLayoutTransferrer) ApplyLayout(
	arena *LGraphArena,
	topGraph LGraphID,
) error {
	if transferrer.Model == nil {
		return fmt.Errorf("layered: cannot transfer into a nil graph")
	}
	transferrer.graphSeen = make([]bool, len(arena.Graphs))
	transferrer.nodeStamp = make([]uint32, len(arena.Nodes))
	if err := transferrer.applyGraph(arena, topGraph); err != nil {
		return err
	}
	// Every model edge transferred below gets a first section; reserve the
	// ones that do not exist yet in one step. (No section pointer is held
	// across a section addition, so the arena's capacity is not observable.)
	newSections, transferred := 0, 0
	counted := make([]bool, len(transferrer.Model.Edges))
	for index := range arena.Edges {
		origin, found := basegraph.TryGet(&arena.Edges[index].Properties, OriginalElement)
		if found && origin.Kind == OriginEdge && !counted[origin.Edge] {
			counted[origin.Edge] = true
			transferred++
			if len(transferrer.Model.Edge(origin.Edge).Sections) == 0 {
				newSections++
			}
		}
	}
	transferrer.Model.ReserveSections(newSections)
	// The second walk visits the same edges in the same order; the first
	// walk's flags are cleared and reused as its "seen" set.
	clear(counted)
	seenEdges := counted
	transferrer.shapes = make([]basegraph.ShapeID, 0, 2*transferred)
	for index := range arena.Edges {
		edgeID := LEdgeID(index)
		origin, found := basegraph.TryGet(&arena.Edge(edgeID).Properties, OriginalElement)
		if !found || origin.Kind != OriginEdge || seenEdges[origin.Edge] {
			continue
		}
		seenEdges[origin.Edge] = true
		transferrer.applyEdge(arena, edgeID, origin.Edge)
	}
	return nil
}

// applyGraph applies graphID's nodes, and the nested graphs of those, to the
// model. The recursion into nested graphs is applyGraph alone; the work per
// graph and per node is done by helpers outside it, so that a level of
// nesting costs one small frame.
func (transferrer *ElkGraphLayoutTransferrer) applyGraph(
	arena *LGraphArena,
	graphID LGraphID,
) error {
	if transferrer.graphSeen[graphID] {
		return nil
	}
	transferrer.graphSeen[graphID] = true
	originNode, offset, err := graphOriginAndOffset(arena, graphID)
	if err != nil {
		return err
	}
	// GraphAllNodes' order (layerless nodes, then layer by layer), walked in
	// place: applying a node changes no node or layer list. A node met twice
	// in one visit is applied once, as with the old per-visit set.
	transferrer.stamp++
	stamp := transferrer.stamp
	undoStart := len(transferrer.undo)
	graph := arena.Graph(graphID)
	for list := -1; list < len(graph.Layers); list++ {
		nodes := graph.LayerlessNodes
		if list >= 0 {
			nodes = arena.Layer(graph.Layers[list]).Nodes
		}
		for _, nodeID := range nodes {
			if nested := transferrer.applyNodeInVisit(arena, graphID, nodeID, offset, stamp); nested != nil {
				if err := transferrer.applyGraph(arena, *nested); err != nil {
					return err
				}
			}
		}
	}
	transferrer.endGraphVisit(arena, graphID, undoStart, originNode)
	return nil
}

// graphOriginAndOffset returns the model node graphID was imported from and
// the offset of its nodes' positions.
func graphOriginAndOffset(arena *LGraphArena, graphID LGraphID) (basegraph.NodeID, geometry.Point, error) {
	graph := arena.Graph(graphID)
	origin, found := basegraph.TryGet(&graph.Properties, OriginalElement)
	if !found || origin.Kind != OriginNode {
		return 0, geometry.Point{}, fmt.Errorf("layered: graph %d has no original parent node", graphID)
	}
	return origin.Node, graph.Offset.Add(geometry.Point{X: graph.Padding.Left, Y: graph.Padding.Top}), nil
}

// applyNodeInVisit applies nodeID in the graph visit with stamp, unless the
// visit applied it already, and returns the nested graph the visit applies
// next (nil: none, or the node was applied already).
func (transferrer *ElkGraphLayoutTransferrer) applyNodeInVisit(
	arena *LGraphArena,
	graphID LGraphID,
	nodeID LNodeID,
	offset geometry.Point,
	stamp uint32,
) *LGraphID {
	if transferrer.nodeStamp[nodeID] == stamp {
		return nil
	}
	transferrer.undo = append(transferrer.undo, stampUndo{node: nodeID, previous: transferrer.nodeStamp[nodeID]})
	transferrer.nodeStamp[nodeID] = stamp
	nodeOrigin, hasOrigin := basegraph.TryGet(&arena.Node(nodeID).Properties, OriginalElement)
	if hasOrigin && nodeOrigin.Kind == OriginNode {
		transferrer.applyNode(arena, nodeID, nodeOrigin.Node, offset)
	} else if hasOrigin && nodeOrigin.Kind == OriginPort && arena.Graph(graphID).ParentNode == nil {
		// A top-level external-port dummy represents a port on the node
		// whose contents are currently being laid out. Its position is
		// relative to that node rather than to a regular child node.
		port := transferrer.Model.Port(nodeOrigin.Port)
		position := arena.ExternalPortPosition(
			graphID, nodeID, port.Shape.Width, port.Shape.Height)
		port.Shape.SetLocation(position.X, position.Y)
	}
	return arena.Node(nodeID).NestedGraph
}

// endGraphVisit ends the visit of graphID: the stamps it overwrote are
// restored, and a top graph's parent node gets its layout.
func (transferrer *ElkGraphLayoutTransferrer) endGraphVisit(
	arena *LGraphArena,
	graphID LGraphID,
	undoStart int,
	originNode basegraph.NodeID,
) {
	for index := len(transferrer.undo) - 1; index >= undoStart; index-- {
		transferrer.nodeStamp[transferrer.undo[index].node] = transferrer.undo[index].previous
	}
	transferrer.undo = transferrer.undo[:undoStart]
	if arena.Graph(graphID).ParentNode == nil {
		transferrer.applyParentNodeLayout(arena, graphID, originNode)
	}
}

// applyParentNodeLayout mirrors ElkGraphLayoutTransferrer.applyParentNodeLayout.
// In particular, a graph without imported external-port dummies still has its
// ports moved by ElkUtil.resizeNode. This matters for isolated fixed-side ports
// on compound nodes whose dimensions change during their own layout run.
func (transferrer *ElkGraphLayoutTransferrer) applyParentNodeLayout(
	arena *LGraphArena,
	graphID LGraphID,
	originalID basegraph.NodeID,
) {
	graph := arena.Graph(graphID)
	original := transferrer.Model.Node(originalID)
	sizeConstraints := basegraph.Get(&original.Properties, core.NodeSizeConstraints)
	sizeConstraintsIncludedPortLabels := containsString(sizeConstraints, "PORT_LABELS")

	size := arena.GraphActualSize(graphID)
	graphProperties := basegraph.Get(&graph.Properties, GraphProperties)
	if graphProperties&GraphPropertyExternalPorts != 0 {
		// External-port dummies have already assigned final port positions.
		basegraph.Set(&original.Properties, core.PortConstraints, "FIXED_POS")
		elkutil.ResizeNode(transferrer.Model, originalID, size.X, size.Y, false, true)
	} else if !basegraph.Get(&original.Properties, basicoptions.NodeSizeFixedGraphSize) {
		// No external-port dummy was imported, so resizing must adjust the
		// original node's peripheral ports exactly like ELK does.
		elkutil.ResizeNode(transferrer.Model, originalID, size.X, size.Y, true, true)
	}

	if sizeConstraintsIncludedPortLabels {
		basegraph.Set(&original.Properties, core.NodeSizeConstraints, []string{"PORT_LABELS"})
	} else {
		basegraph.Set(&original.Properties, core.NodeSizeConstraints, []string{})
	}
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func (transferrer *ElkGraphLayoutTransferrer) applyNode(
	arena *LGraphArena,
	nodeID LNodeID,
	originalID basegraph.NodeID,
	offset geometry.Point,
) {
	node := arena.Node(nodeID)
	original := transferrer.Model.Node(originalID)
	original.Shape.SetLocation(node.Position.X+offset.X, node.Position.Y+offset.Y)
	original.Shape.SetDimensions(node.Size.X, node.Size.Y)
	basegraph.SetInt(&original.Properties, LayerIDOption,
		func() int {
			if node.Layer == nil {
				return -1
			}
			return arena.LayerIndex(*node.Layer)
		}())
	basegraph.SetInt(&original.Properties, PositionIDOption, arena.NodeIndexInLayer(nodeID))

	for _, portID := range node.Ports {
		origin, found := basegraph.TryGet(&arena.Port(portID).Properties, OriginalElement)
		if !found || origin.Kind != OriginPort {
			continue
		}
		port := arena.Port(portID)
		originalPort := transferrer.Model.Port(origin.Port)
		originalPort.Shape.SetLocation(port.Position.X, port.Position.Y)
		originalPort.Shape.SetDimensions(port.Size.X, port.Size.Y)
		setPortSideName(&originalPort.Properties, port.Side)
		if !fixedPortLabelPlacement(&node.Properties) {
			for _, labelID := range port.Labels {
				transferrer.applyLabel(arena, labelID, geometry.Point{})
			}
		}
	}
	nodeHasLabelPlacement := nonemptyPlacement(&node.Properties, nodespacing.NodeLabelsPlacement.ID)
	for _, labelID := range node.Labels {
		if nodeHasLabelPlacement || nonemptyPlacement(
			&arena.Label(labelID).Properties,
			nodespacing.NodeLabelsPlacement.ID,
		) {
			transferrer.applyLabel(arena, labelID, geometry.Point{})
		}
	}
}

func nonemptyPlacement(properties *basegraph.PropertyMap, id string) bool {
	value, found := properties.Raw(id)
	if !found {
		return false
	}
	switch typed := value.(type) {
	case []string:
		return len(typed) > 0
	case string:
		return strings.TrimSpace(typed) != ""
	default:
		return false
	}
}

func fixedPortLabelPlacement(properties *basegraph.PropertyMap) bool {
	value, found := properties.Raw(nodespacing.PortLabelsPlacement.ID)
	if !found {
		return false
	}
	switch typed := value.(type) {
	case []string:
		for _, item := range typed {
			if strings.EqualFold(item, "FIXED") {
				return true
			}
		}
	case string:
		for _, field := range strings.FieldsFunc(typed, func(r rune) bool {
			return r == ' ' || r == ',' || r == '[' || r == ']'
		}) {
			if strings.EqualFold(field, "FIXED") {
				return true
			}
		}
	}
	return false
}

func (transferrer *ElkGraphLayoutTransferrer) applyLabel(
	arena *LGraphArena,
	labelID LLabelID,
	offset geometry.Point,
) {
	transferrer.applyLabelInCoordinateSystem(arena, labelID, offset, geometry.Point{})
}

func (transferrer *ElkGraphLayoutTransferrer) applyLabelInCoordinateSystem(
	arena *LGraphArena,
	labelID LLabelID,
	offset geometry.Point,
	containerOrigin geometry.Point,
) {
	origin, found := basegraph.TryGet(&arena.Label(labelID).Properties, OriginalElement)
	if !found || origin.Kind != OriginLabel {
		return
	}
	label := arena.Label(labelID)
	original := transferrer.Model.Label(origin.Label)
	original.Shape.SetDimensions(label.Size.X, label.Size.Y)
	position := label.Position.Add(offset).Sub(containerOrigin)
	original.Shape.SetLocation(position.X, position.Y)
}

func (transferrer *ElkGraphLayoutTransferrer) applyEdge(
	arena *LGraphArena,
	edgeID LEdgeID,
	originalID basegraph.EdgeID,
) {
	edge := arena.Edge(edgeID)
	if edge.Source == nil || edge.Target == nil {
		return
	}
	original := transferrer.Model.Edge(originalID)
	if len(original.Sources) == 0 || len(original.Targets) == 0 {
		return
	}
	sourceNode := arena.EdgeSourceNode(edgeID)
	targetNode := arena.EdgeTargetNode(edgeID)
	descendantTarget := arena.IsDescendant(targetNode, sourceNode) &&
		arena.Node(sourceNode).NestedGraph != nil
	layoutGraph := arena.NodeGraph(sourceNode)
	if descendantTarget {
		layoutGraph = *arena.Node(sourceNode).NestedGraph
	}
	baseOffset, edgeOffset := transferrer.edgeLayoutOffsets(arena, edgeID, layoutGraph)
	sourcePort := arena.Port(*edge.Source)
	targetPort := arena.Port(*edge.Target)
	// LPort.getAbsoluteAnchor sums node, port, and anchor first. The layout
	// transferrer adds the graph offset afterwards. That evaluation order is
	// observable at the final ULP for fractional component and port positions.
	var sourcePoint geometry.Point
	if descendantTarget {
		// Descendant edges are laid out inside their source node. The source
		// anchor is already relative to that node; Java removes the base graph
		// offset before applying the complete edge offset below.
		sourcePoint = sourcePort.Position.Add(sourcePort.Anchor).Sub(baseOffset)
	} else {
		sourcePoint = arena.Node(sourceNode).Position.
			Add(sourcePort.Position).
			Add(sourcePort.Anchor)
	}
	sourcePoint = sourcePoint.Add(edgeOffset)
	targetPoint := arena.Node(targetNode).Position.
		Add(targetPort.Position).
		Add(targetPort.Anchor)
	if targetOffset, found := basegraph.TryGet(&edge.Properties, TargetOffset); found {
		targetPoint = targetPoint.Add(targetOffset)
	}
	targetPoint = targetPoint.Add(edgeOffset)
	// TARGET_OFFSET moves the target anchor into the edge's reference graph;
	// edgeOffset then converts that graph into the original edge container's
	// coordinate system.
	sectionID := transferrer.Model.FirstSection(originalID, true)
	section := transferrer.Model.Section(sectionID)
	section.Start = sourcePoint
	section.End = targetPoint
	section.StartPresent, section.EndPresent = true, true
	section.IncomingShape = transferrer.copyShapeID(original.Sources[0])
	section.OutgoingShape = transferrer.copyShapeID(original.Targets[0])
	section.BendPoints = make(geometry.Chain, len(edge.BendPoints))
	for index, point := range edge.BendPoints {
		section.BendPoints[index] = point.Add(edgeOffset)
	}
	section.BendPointsPresent = len(section.BendPoints) > 0

	for _, labelID := range edge.Labels {
		transferrer.applyLabel(arena, labelID, edgeOffset)
	}
	if junctions, found := basegraph.TryGet(&edge.Properties, JunctionPoints); found && len(junctions) > 0 {
		translated := make(geometry.Chain, len(junctions))
		for index, point := range junctions {
			translated[index] = point.Add(edgeOffset)
		}
		basegraph.Set(&original.Properties, core.JunctionPoints, translated)
	} else {
		basegraph.Unset(&original.Properties, core.JunctionPoints)
	}
}

func (transferrer *ElkGraphLayoutTransferrer) edgeLayoutOffsets(
	arena *LGraphArena,
	edgeID LEdgeID,
	layoutGraph LGraphID,
) (geometry.Point, geometry.Point) {
	graph := arena.Graph(layoutGraph)
	baseOffset := graph.Offset.Add(
		geometry.Point{X: graph.Padding.Left, Y: graph.Padding.Top})
	hierarchicalOffset := geometry.Point{}
	targetGraph, found := basegraph.TryGet(
		&arena.Edge(edgeID).Properties,
		CoordinateSystemOrigin,
	)
	if found {
		currentGraph := arena.NodeGraph(arena.EdgeSourceNode(edgeID))
		for currentGraph != targetGraph {
			current := arena.Graph(currentGraph)
			if current.ParentNode == nil {
				break
			}
			representingNode := *current.ParentNode
			hierarchicalOffset = hierarchicalOffset.Add(
				arena.Node(representingNode).Position)
			currentGraph = arena.NodeGraph(representingNode)
			parentGraph := arena.Graph(currentGraph)
			hierarchicalOffset = hierarchicalOffset.
				Add(parentGraph.Offset).
				Add(geometry.Point{
					X: parentGraph.Padding.Left,
					Y: parentGraph.Padding.Top,
				})
		}
	}
	return baseOffset, baseOffset.Add(hierarchicalOffset)
}

// copyShapeID returns a pointer to a new copy of value, taken from the
// transfer's block (every copy has its own element).
func (transferrer *ElkGraphLayoutTransferrer) copyShapeID(value basegraph.ShapeID) *basegraph.ShapeID {
	if len(transferrer.shapes) == cap(transferrer.shapes) {
		copy := value
		return &copy
	}
	transferrer.shapes = append(transferrer.shapes, value)
	return &transferrer.shapes[len(transferrer.shapes)-1]
}

// portSideNames holds side.String() of every defined side, boxed once.
var portSideNames = func() (names [WestSide + 1]any) {
	for side := range names {
		names[side] = PortSide(side).String()
	}
	return names
}()

// setPortSideName is Set(properties, core.PortSide, side.String()). A boxed
// string cannot be changed, so storing one shared box per side is the same as
// boxing the name anew.
func setPortSideName(properties *basegraph.PropertyMap, side PortSide) {
	if int(side) < len(portSideNames) {
		properties.SetRawKey(core.PortSide.Key(), portSideNames[side])
		return
	}
	basegraph.Set(properties, core.PortSide, side.String())
}
