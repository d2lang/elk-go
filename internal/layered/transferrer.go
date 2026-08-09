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
	if err := transferrer.applyGraph(arena, topGraph, make(map[LGraphID]bool)); err != nil {
		return err
	}
	seenEdges := make(map[basegraph.EdgeID]bool)
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

func (transferrer *ElkGraphLayoutTransferrer) applyGraph(
	arena *LGraphArena,
	graphID LGraphID,
	seen map[LGraphID]bool,
) error {
	if seen[graphID] {
		return nil
	}
	seen[graphID] = true
	graph := arena.Graph(graphID)
	origin, found := basegraph.TryGet(&graph.Properties, OriginalElement)
	if !found || origin.Kind != OriginNode {
		return fmt.Errorf("layered: graph %d has no original parent node", graphID)
	}
	offset := graph.Offset.Add(geometry.Point{X: graph.Padding.Left, Y: graph.Padding.Top})
	processed := make(map[LNodeID]bool)
	for _, nodeID := range arena.GraphAllNodes(graphID) {
		if processed[nodeID] {
			continue
		}
		processed[nodeID] = true
		nodeOrigin, hasOrigin := basegraph.TryGet(&arena.Node(nodeID).Properties, OriginalElement)
		if hasOrigin && nodeOrigin.Kind == OriginNode {
			transferrer.applyNode(arena, nodeID, nodeOrigin.Node, offset)
		} else if hasOrigin && nodeOrigin.Kind == OriginPort && graph.ParentNode == nil {
			// A top-level external-port dummy represents a port on the node
			// whose contents are currently being laid out. Its position is
			// relative to that node rather than to a regular child node.
			port := transferrer.Model.Port(nodeOrigin.Port)
			position := arena.ExternalPortPosition(
				graphID, nodeID, port.Shape.Width, port.Shape.Height)
			port.Shape.SetLocation(position.X, position.Y)
		}
		if arena.Node(nodeID).NestedGraph != nil {
			if err := transferrer.applyGraph(arena, *arena.Node(nodeID).NestedGraph, seen); err != nil {
				return err
			}
		}
	}
	if graph.ParentNode == nil {
		transferrer.applyParentNodeLayout(arena, graphID, origin.Node)
	}
	return nil
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
	basegraph.Set(&original.Properties, LayerIDOption,
		func() int {
			if node.Layer == nil {
				return -1
			}
			return arena.LayerIndex(*node.Layer)
		}())
	basegraph.Set(&original.Properties, PositionIDOption, arena.NodeIndexInLayer(nodeID))

	for _, portID := range node.Ports {
		origin, found := basegraph.TryGet(&arena.Port(portID).Properties, OriginalElement)
		if !found || origin.Kind != OriginPort {
			continue
		}
		port := arena.Port(portID)
		originalPort := transferrer.Model.Port(origin.Port)
		originalPort.Shape.SetLocation(port.Position.X, port.Position.Y)
		originalPort.Shape.SetDimensions(port.Size.X, port.Size.Y)
		basegraph.Set(&originalPort.Properties, core.PortSide, port.Side.String())
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
	origin, found := basegraph.TryGet(&arena.Label(labelID).Properties, OriginalElement)
	if !found || origin.Kind != OriginLabel {
		return
	}
	label := arena.Label(labelID)
	original := transferrer.Model.Label(origin.Label)
	original.Shape.SetDimensions(label.Size.X, label.Size.Y)
	original.Shape.SetLocation(label.Position.X+offset.X, label.Position.Y+offset.Y)
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
	sourceGraphOrigin := transferrer.graphContentOrigin(arena, arena.NodeGraph(sourceNode))
	targetGraphOrigin := transferrer.graphContentOrigin(arena, arena.NodeGraph(targetNode))
	sourcePort := arena.Port(*edge.Source)
	targetPort := arena.Port(*edge.Target)
	sourcePoint := sourceGraphOrigin.
		Add(arena.Node(sourceNode).Position).
		Add(sourcePort.Position).
		Add(sourcePort.Anchor)
	targetPoint := targetGraphOrigin.
		Add(arena.Node(targetNode).Position).
		Add(targetPort.Position).
		Add(targetPort.Anchor)
	// ELK's layered graph reports an endpoint relative to its own graph and
	// TARGET_OFFSET translates it into the source graph's coordinate system.
	// graphContentOrigin has already performed that hierarchy translation for
	// both endpoints, so applying TARGET_OFFSET here would count it twice.

	containerOrigin := geometry.Point{}
	if original.ContainingNode != nil {
		containerOrigin = transferrer.absoluteNodePosition(*original.ContainingNode)
	}
	sectionID := transferrer.Model.FirstSection(originalID, true)
	section := transferrer.Model.Section(sectionID)
	section.Start = sourcePoint.Sub(containerOrigin)
	section.End = targetPoint.Sub(containerOrigin)
	section.StartPresent, section.EndPresent = true, true
	section.IncomingShape = copyShapeID(&original.Sources[0])
	section.OutgoingShape = copyShapeID(&original.Targets[0])
	section.BendPoints = make(geometry.Chain, len(edge.BendPoints))
	for index, point := range edge.BendPoints {
		section.BendPoints[index] = sourceGraphOrigin.Add(point).Sub(containerOrigin)
	}
	section.BendPointsPresent = len(section.BendPoints) > 0

	labelOffset := sourceGraphOrigin.Sub(containerOrigin)
	for _, labelID := range edge.Labels {
		transferrer.applyLabel(arena, labelID, labelOffset)
	}
	if junctions, found := basegraph.TryGet(&edge.Properties, JunctionPoints); found && len(junctions) > 0 {
		translated := make(geometry.Chain, len(junctions))
		for index, point := range junctions {
			translated[index] = sourceGraphOrigin.Add(point).Sub(containerOrigin)
		}
		basegraph.Set(&original.Properties, core.JunctionPoints, translated)
	} else {
		basegraph.Unset(&original.Properties, core.JunctionPoints)
	}
}

func (transferrer *ElkGraphLayoutTransferrer) graphContentOrigin(
	arena *LGraphArena,
	graphID LGraphID,
) geometry.Point {
	graph := arena.Graph(graphID)
	local := graph.Offset.Add(geometry.Point{X: graph.Padding.Left, Y: graph.Padding.Top})
	if graph.ParentNode == nil {
		return local
	}
	parentNode := *graph.ParentNode
	outerGraph := arena.NodeGraph(parentNode)
	return transferrer.graphContentOrigin(arena, outerGraph).
		Add(arena.Node(parentNode).Position).
		Add(local)
}

func (transferrer *ElkGraphLayoutTransferrer) absoluteNodePosition(nodeID basegraph.NodeID) geometry.Point {
	position := transferrer.Model.Node(nodeID).Shape.Position()
	for parent := transferrer.Model.Node(nodeID).Parent; parent != nil; parent = transferrer.Model.Node(*parent).Parent {
		position = position.Add(transferrer.Model.Node(*parent).Shape.Position())
	}
	return position
}

func copyShapeID(value *basegraph.ShapeID) *basegraph.ShapeID {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
