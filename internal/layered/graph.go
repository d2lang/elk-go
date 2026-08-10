// Copyright (c) 2009, 2015 Kiel University and others.
// Copyright (c) 2010, 2019 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package layered implements ELK Layered's internal graph and five-phase
// layout pipeline. The types in this file are the arena-backed counterparts
// of ELK 0.8.1's LGraph, LNode, LPort, LEdge, LLabel, and Layer classes.
package layered

import (
	"fmt"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
)

type LGraphID uint32
type LNodeID uint32
type LPortID uint32
type LEdgeID uint32
type LLabelID uint32
type LayerID uint32

func (id LGraphID) Index() int { return int(id) }
func (id LNodeID) Index() int  { return int(id) }
func (id LPortID) Index() int  { return int(id) }
func (id LEdgeID) Index() int  { return int(id) }
func (id LLabelID) Index() int { return int(id) }
func (id LayerID) Index() int  { return int(id) }

// NodeType is the exact ELK 0.8.1 dummy-node classification. Its declaration
// order is significant: spacing tables index directly by this value.
type NodeType uint8

const (
	NormalNode NodeType = iota
	LongEdgeNode
	ExternalPortNode
	NorthSouthPortNode
	LabelNode
	BreakingPointNode
	PlaceholderNode
	NonShiftingPlaceholderNode
)

func (kind NodeType) String() string {
	switch kind {
	case NormalNode:
		return "NORMAL"
	case LongEdgeNode:
		return "LONG_EDGE"
	case ExternalPortNode:
		return "EXTERNAL_PORT"
	case NorthSouthPortNode:
		return "NORTH_SOUTH_PORT"
	case LabelNode:
		return "LABEL"
	case BreakingPointNode:
		return "BREAKING_POINT"
	case PlaceholderNode:
		return "PLACEHOLDER"
	case NonShiftingPlaceholderNode:
		return "NONSHIFTING_PLACEHOLDER"
	default:
		return fmt.Sprintf("NodeType(%d)", kind)
	}
}

// PortSide matches org.eclipse.elk.core.options.PortSide's ordinal order.
type PortSide uint8

const (
	UndefinedSide PortSide = iota
	NorthSide
	EastSide
	SouthSide
	WestSide
)

func (side PortSide) String() string {
	switch side {
	case UndefinedSide:
		return "UNDEFINED"
	case NorthSide:
		return "NORTH"
	case EastSide:
		return "EAST"
	case SouthSide:
		return "SOUTH"
	case WestSide:
		return "WEST"
	default:
		return fmt.Sprintf("PortSide(%d)", side)
	}
}

func (side PortSide) Opposed() PortSide {
	switch side {
	case NorthSide:
		return SouthSide
	case EastSide:
		return WestSide
	case SouthSide:
		return NorthSide
	case WestSide:
		return EastSide
	default:
		return UndefinedSide
	}
}

type LGraph struct {
	ID             int
	Size           geometry.Point
	Padding        geometry.Spacing
	Offset         geometry.Point
	LayerlessNodes []LNodeID
	Layers         []LayerID
	ParentNode     *LNodeID
	Properties     basegraph.PropertyMap
}

type Layer struct {
	ID         int
	Graph      LGraphID
	Nodes      []LNodeID
	Size       geometry.Point
	Properties basegraph.PropertyMap
}

type LNode struct {
	ID          int
	Graph       LGraphID
	Layer       *LayerID
	Type        NodeType
	Ports       []LPortID
	Labels      []LLabelID
	NestedGraph *LGraphID
	Margin      geometry.Spacing
	Padding     geometry.Spacing
	Position    geometry.Point
	Size        geometry.Point
	Properties  basegraph.PropertyMap

	portSideRanges  [5]indexRange
	portSidesCached bool
}

type LPort struct {
	ID                       int
	Node                     *LNodeID
	Side                     PortSide
	Anchor                   geometry.Point
	ExplicitAnchor           bool
	Margin                   geometry.Spacing
	Labels                   []LLabelID
	IncomingEdges            []LEdgeID
	OutgoingEdges            []LEdgeID
	ConnectedToExternalNodes bool
	Position                 geometry.Point
	Size                     geometry.Point
	Properties               basegraph.PropertyMap
}

type LEdge struct {
	ID         int
	BendPoints geometry.Chain
	Source     *LPortID
	Target     *LPortID
	Labels     []LLabelID
	Properties basegraph.PropertyMap
}

type LLabel struct {
	ID         int
	Text       string
	Position   geometry.Point
	Size       geometry.Point
	Properties basegraph.PropertyMap
}

type indexRange struct{ Start, End int }

// LGraphArena gives Go stable identities without pointers into growing slices.
// Arena order is Java object creation order and must never be reconstructed
// from a map.
type LGraphArena struct {
	Graphs []LGraph
	Nodes  []LNode
	Ports  []LPort
	Edges  []LEdge
	Labels []LLabel
	Layers []Layer
}

func NewLGraphArena() *LGraphArena { return &LGraphArena{} }

func (arena *LGraphArena) Graph(id LGraphID) *LGraph { return &arena.Graphs[id.Index()] }
func (arena *LGraphArena) Node(id LNodeID) *LNode    { return &arena.Nodes[id.Index()] }
func (arena *LGraphArena) Port(id LPortID) *LPort    { return &arena.Ports[id.Index()] }
func (arena *LGraphArena) Edge(id LEdgeID) *LEdge    { return &arena.Edges[id.Index()] }
func (arena *LGraphArena) Label(id LLabelID) *LLabel { return &arena.Labels[id.Index()] }
func (arena *LGraphArena) Layer(id LayerID) *Layer   { return &arena.Layers[id.Index()] }

func (arena *LGraphArena) AddGraph() LGraphID {
	id := LGraphID(len(arena.Graphs))
	arena.Graphs = append(arena.Graphs, LGraph{})
	return id
}

// AddNode is new LNode(graph); callers add the node to graph.LayerlessNodes
// explicitly, just as the Java importer does.
func (arena *LGraphArena) AddNode(graphID LGraphID) LNodeID {
	id := LNodeID(len(arena.Nodes))
	arena.Nodes = append(arena.Nodes, LNode{Graph: graphID, Type: NormalNode})
	return id
}

func (arena *LGraphArena) AddPort() LPortID {
	id := LPortID(len(arena.Ports))
	arena.Ports = append(arena.Ports, LPort{ConnectedToExternalNodes: true})
	return id
}

func (arena *LGraphArena) AddEdge() LEdgeID {
	id := LEdgeID(len(arena.Edges))
	arena.Edges = append(arena.Edges, LEdge{})
	return id
}

func (arena *LGraphArena) AddLabel(text string) LLabelID {
	id := LLabelID(len(arena.Labels))
	arena.Labels = append(arena.Labels, LLabel{Text: text})
	return id
}

// AddLayer is new Layer(graph); it deliberately does not append the layer to
// graph.Layers. This mirrors the subtle Java constructor behavior.
func (arena *LGraphArena) AddLayer(graphID LGraphID) LayerID {
	id := LayerID(len(arena.Layers))
	arena.Layers = append(arena.Layers, Layer{Graph: graphID})
	return id
}

func (arena *LGraphArena) SetPortNode(portID LPortID, nodeID *LNodeID) {
	port := arena.Port(portID)
	if port.Node != nil {
		old := *port.Node
		arena.Node(old).Ports = removePort(arena.Node(old).Ports, portID)
		arena.InvalidatePortSideCache(old)
	}
	port.Node = copyLNodeID(nodeID)
	if nodeID != nil {
		arena.Node(*nodeID).Ports = append(arena.Node(*nodeID).Ports, portID)
		arena.InvalidatePortSideCache(*nodeID)
	}
}

func (arena *LGraphArena) SetPortSide(portID LPortID, side PortSide) {
	port := arena.Port(portID)
	port.Side = side
	if !port.ExplicitAnchor {
		switch side {
		case NorthSide:
			port.Anchor = geometry.Point{X: port.Size.X / 2}
		case EastSide:
			port.Anchor = geometry.Point{X: port.Size.X, Y: port.Size.Y / 2}
		case SouthSide:
			port.Anchor = geometry.Point{X: port.Size.X / 2, Y: port.Size.Y}
		case WestSide:
			port.Anchor = geometry.Point{Y: port.Size.Y / 2}
		}
	}
	if port.Node != nil {
		arena.InvalidatePortSideCache(*port.Node)
	}
}

func (arena *LGraphArena) SetEdgeSource(edgeID LEdgeID, portID *LPortID) {
	edge := arena.Edge(edgeID)
	if edge.Source != nil {
		arena.Port(*edge.Source).OutgoingEdges = removeEdge(arena.Port(*edge.Source).OutgoingEdges, edgeID)
	}
	edge.Source = copyLPortID(portID)
	if portID != nil {
		arena.Port(*portID).OutgoingEdges = append(arena.Port(*portID).OutgoingEdges, edgeID)
	}
}

func (arena *LGraphArena) SetEdgeTarget(edgeID LEdgeID, portID *LPortID) {
	arena.SetEdgeTargetAt(edgeID, portID, -1)
}

func (arena *LGraphArena) SetEdgeTargetAt(edgeID LEdgeID, portID *LPortID, index int) {
	edge := arena.Edge(edgeID)
	if edge.Target != nil {
		arena.Port(*edge.Target).IncomingEdges = removeEdge(arena.Port(*edge.Target).IncomingEdges, edgeID)
	}
	edge.Target = copyLPortID(portID)
	if portID == nil {
		return
	}
	incoming := arena.Port(*portID).IncomingEdges
	if index < 0 {
		arena.Port(*portID).IncomingEdges = append(incoming, edgeID)
		return
	}
	if index > len(incoming) {
		panic("layered: edge target insertion index out of bounds")
	}
	incoming = append(incoming, 0)
	copy(incoming[index+1:], incoming[index:])
	incoming[index] = edgeID
	arena.Port(*portID).IncomingEdges = incoming
}

func (arena *LGraphArena) SetNodeLayer(nodeID LNodeID, layerID *LayerID) {
	arena.SetNodeLayerAt(nodeID, layerID, -1)
}

func (arena *LGraphArena) SetNodeLayerAt(nodeID LNodeID, layerID *LayerID, index int) {
	node := arena.Node(nodeID)
	if node.Layer != nil {
		arena.Layer(*node.Layer).Nodes = removeNode(arena.Layer(*node.Layer).Nodes, nodeID)
	}
	node.Layer = copyLayerID(layerID)
	if layerID == nil {
		return
	}
	nodes := arena.Layer(*layerID).Nodes
	if index < 0 {
		arena.Layer(*layerID).Nodes = append(nodes, nodeID)
		return
	}
	if index > len(nodes) {
		panic("layered: node layer insertion index out of bounds")
	}
	nodes = append(nodes, 0)
	copy(nodes[index+1:], nodes[index:])
	nodes[index] = nodeID
	arena.Layer(*layerID).Nodes = nodes
}

func (arena *LGraphArena) NodeGraph(nodeID LNodeID) LGraphID {
	node := arena.Node(nodeID)
	if node.Layer != nil {
		return arena.Layer(*node.Layer).Graph
	}
	return node.Graph
}

func (arena *LGraphArena) NodeIncomingEdges(nodeID LNodeID) []LEdgeID {
	var edges []LEdgeID
	for _, portID := range arena.Node(nodeID).Ports {
		edges = append(edges, arena.Port(portID).IncomingEdges...)
	}
	return edges
}

func (arena *LGraphArena) NodeOutgoingEdges(nodeID LNodeID) []LEdgeID {
	var edges []LEdgeID
	for _, portID := range arena.Node(nodeID).Ports {
		edges = append(edges, arena.Port(portID).OutgoingEdges...)
	}
	return edges
}

func (arena *LGraphArena) NodeConnectedEdges(nodeID LNodeID) []LEdgeID {
	var edges []LEdgeID
	for _, portID := range arena.Node(nodeID).Ports {
		port := arena.Port(portID)
		edges = append(edges, port.IncomingEdges...)
		edges = append(edges, port.OutgoingEdges...)
	}
	return edges
}

func (arena *LGraphArena) PortConnectedEdges(portID LPortID) []LEdgeID {
	port := arena.Port(portID)
	edges := append([]LEdgeID(nil), port.IncomingEdges...)
	return append(edges, port.OutgoingEdges...)
}

func (arena *LGraphArena) NodePortsOnSide(nodeID LNodeID, side PortSide) []LPortID {
	ports := make([]LPortID, 0)
	for _, portID := range arena.Node(nodeID).Ports {
		if arena.Port(portID).Side == side {
			ports = append(ports, portID)
		}
	}
	return ports
}

func (arena *LGraphArena) NodePortSideView(nodeID LNodeID, side PortSide) []LPortID {
	node := arena.Node(nodeID)
	if !node.portSidesCached {
		arena.findPortSideRanges(nodeID)
		node = arena.Node(nodeID)
	}
	rangeForSide := node.portSideRanges[int(side)]
	return node.Ports[rangeForSide.Start:rangeForSide.End]
}

func (arena *LGraphArena) CachePortSides(nodeID LNodeID) {
	arena.findPortSideRanges(nodeID)
	arena.Node(nodeID).portSidesCached = true
}

func (arena *LGraphArena) findPortSideRanges(nodeID LNodeID) {
	node := arena.Node(nodeID)
	var ranges [5]indexRange
	for side := UndefinedSide; side <= WestSide; side++ {
		start, end := -1, -1
		for index, portID := range node.Ports {
			if arena.Port(portID).Side == side {
				if start < 0 {
					start = index
				}
				end = index + 1
			} else if start >= 0 {
				break
			}
		}
		if start < 0 {
			start, end = 0, 0
		}
		ranges[int(side)] = indexRange{Start: start, End: end}
	}
	node.portSideRanges = ranges
}

func (arena *LGraphArena) InvalidatePortSideCache(nodeID LNodeID) {
	node := arena.Node(nodeID)
	// LNode.getPortSideView recomputes its indices on every call until
	// cachePortSides is called. Once explicitly cached, the Java implementation
	// deliberately keeps the index ranges across structural port-list changes;
	// several Layered processors rely on that observable sublist behavior.
	if !node.portSidesCached {
		node.portSideRanges = [5]indexRange{}
	}
}

func (arena *LGraphArena) EdgeIsSelfLoop(edgeID LEdgeID) bool {
	edge := arena.Edge(edgeID)
	if edge.Source == nil || edge.Target == nil {
		return false
	}
	sourceNode, targetNode := arena.Port(*edge.Source).Node, arena.Port(*edge.Target).Node
	return sourceNode != nil && targetNode != nil && *sourceNode == *targetNode
}

func (arena *LGraphArena) EdgeIsInLayer(edgeID LEdgeID) bool {
	if arena.EdgeIsSelfLoop(edgeID) {
		return false
	}
	source, target := arena.EdgeSourceNode(edgeID), arena.EdgeTargetNode(edgeID)
	return equalLayer(arena.Node(source).Layer, arena.Node(target).Layer)
}

func (arena *LGraphArena) EdgeSourceNode(edgeID LEdgeID) LNodeID {
	return *arena.Port(*arena.Edge(edgeID).Source).Node
}

func (arena *LGraphArena) EdgeTargetNode(edgeID LEdgeID) LNodeID {
	return *arena.Port(*arena.Edge(edgeID).Target).Node
}

func (arena *LGraphArena) EdgeOtherPort(edgeID LEdgeID, portID LPortID) LPortID {
	edge := arena.Edge(edgeID)
	if edge.Source != nil && *edge.Source == portID {
		return *edge.Target
	}
	if edge.Target != nil && *edge.Target == portID {
		return *edge.Source
	}
	panic("layered: port is not incident to edge")
}

func (arena *LGraphArena) EdgeOtherNode(edgeID LEdgeID, nodeID LNodeID) LNodeID {
	source, target := arena.EdgeSourceNode(edgeID), arena.EdgeTargetNode(edgeID)
	if source == nodeID {
		return target
	}
	if target == nodeID {
		return source
	}
	panic("layered: node is not incident to edge")
}

func (arena *LGraphArena) LayerIndex(layerID LayerID) int {
	graphID := arena.Layer(layerID).Graph
	for index, candidate := range arena.Graph(graphID).Layers {
		if candidate == layerID {
			return index
		}
	}
	return -1
}

func (arena *LGraphArena) NodeIndexInLayer(nodeID LNodeID) int {
	layerID := arena.Node(nodeID).Layer
	if layerID == nil {
		return -1
	}
	for index, candidate := range arena.Layer(*layerID).Nodes {
		if candidate == nodeID {
			return index
		}
	}
	return -1
}

func (arena *LGraphArena) GraphActualSize(graphID LGraphID) geometry.Point {
	graph := arena.Graph(graphID)
	return geometry.Point{
		X: graph.Size.X + graph.Padding.Left + graph.Padding.Right,
		Y: graph.Size.Y + graph.Padding.Top + graph.Padding.Bottom,
	}
}

func (arena *LGraphArena) GraphAllNodes(graphID LGraphID) []LNodeID {
	graph := arena.Graph(graphID)
	nodes := append([]LNodeID(nil), graph.LayerlessNodes...)
	for _, layerID := range graph.Layers {
		nodes = append(nodes, arena.Layer(layerID).Nodes...)
	}
	return nodes
}

func (arena *LGraphArena) NodeBorderToContentCoordinates(nodeID LNodeID, horizontal, vertical bool) {
	graph := arena.Graph(arena.NodeGraph(nodeID))
	node := arena.Node(nodeID)
	if horizontal {
		node.Position.X -= graph.Padding.Left + graph.Offset.X
	}
	if vertical {
		node.Position.Y -= graph.Padding.Top + graph.Offset.Y
	}
}

func copyLNodeID(value *LNodeID) *LNodeID {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func copyLPortID(value *LPortID) *LPortID {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func copyLayerID(value *LayerID) *LayerID {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func removeNode(values []LNodeID, target LNodeID) []LNodeID {
	for index, value := range values {
		if value == target {
			return append(values[:index], values[index+1:]...)
		}
	}
	return values
}

func removePort(values []LPortID, target LPortID) []LPortID {
	for index, value := range values {
		if value == target {
			return append(values[:index], values[index+1:]...)
		}
	}
	return values
}

func removeEdge(values []LEdgeID, target LEdgeID) []LEdgeID {
	for index, value := range values {
		if value == target {
			return append(values[:index], values[index+1:]...)
		}
	}
	return values
}

func equalLayer(first, second *LayerID) bool {
	if first == nil || second == nil {
		return first == nil && second == nil
	}
	return *first == *second
}
