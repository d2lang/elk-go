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
	"math"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/reuse"
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

	// Immutable cells holding every ID value, for the optional-ID pointer
	// fields (LPort.Node, LEdge.Source/Target, LNode.Layer). Nothing writes
	// through those pointers or compares them (only their targets are read
	// and only nil is tested), so pointing at a shared cell holding the value
	// is the same as pointing at a fresh copy of it. The cells depend only on
	// their index, so they are kept when the arena is reused.
	nodeCells  idCellTable[LNodeID]
	portCells  idCellTable[LPortID]
	layerCells idCellTable[LayerID]

	// Boxed origins by kind and ID, made on first use and kept when the arena
	// is reused (see SetOrigin).
	origins [OriginLPort + 1][]any

	// allNodes backs GraphAllNodesScratch.
	allNodes []LNodeID

	// scratch holds processors' working storage (see Scratch).
	scratch [scratchSlots]any

	// The last boxed Point and Spacing values SetPoint/SetSpacing stored.
	lastPoint      geometry.Point
	lastPointBox   any
	lastSpacing    geometry.Spacing
	lastSpacingBox any
}

// SetPoint is basegraph.Set for a Point property. A value equal bit for bit
// to the previous one stored this way shares its box: a boxed struct cannot
// change (reading it copies it out), so a shared box holding an identical
// value is the same as a new one.
func (arena *LGraphArena) SetPoint(
	properties *basegraph.PropertyMap,
	property basegraph.Property[geometry.Point],
	value geometry.Point,
) {
	if arena.lastPointBox == nil ||
		math.Float64bits(arena.lastPoint.X) != math.Float64bits(value.X) ||
		math.Float64bits(arena.lastPoint.Y) != math.Float64bits(value.Y) {
		arena.lastPoint, arena.lastPointBox = value, value
	}
	properties.SetRawKey(property.Key(), arena.lastPointBox)
}

// SetSpacing is SetPoint for a Spacing property.
func (arena *LGraphArena) SetSpacing(
	properties *basegraph.PropertyMap,
	property basegraph.Property[geometry.Spacing],
	value geometry.Spacing,
) {
	last := arena.lastSpacing
	if arena.lastSpacingBox == nil ||
		math.Float64bits(last.Top) != math.Float64bits(value.Top) ||
		math.Float64bits(last.Right) != math.Float64bits(value.Right) ||
		math.Float64bits(last.Bottom) != math.Float64bits(value.Bottom) ||
		math.Float64bits(last.Left) != math.Float64bits(value.Left) {
		arena.lastSpacing, arena.lastSpacingBox = value, value
	}
	properties.SetRawKey(property.Key(), arena.lastSpacingBox)
}

// ScratchSlot names the working storage one processor keeps in an arena.
type ScratchSlot uint8

const (
	ScratchGreedyCycles ScratchSlot = iota
	ScratchConnectedComponents
	ScratchLayerConstraints
	ScratchCycleDetector
	ScratchInLayerConstraints
	ScratchNetworkSimplexLayering
	scratchSlots
)

// Scratch returns the value last stored in slot by SetScratch, or nil. The
// value stays with the arena when its storage is reused for later layouts:
// it must be working storage that its processor resets before reading, that
// refers to no element of the arena by pointer, and that is not in use while
// the processor is entered again. Processors store grown buffers back into
// it on their normal returns (see package reuse).
func (arena *LGraphArena) Scratch(slot ScratchSlot) any { return arena.scratch[slot] }

// SetScratch stores value in slot (see Scratch).
func (arena *LGraphArena) SetScratch(slot ScratchSlot, value any) { arena.scratch[slot] = value }

// maxBoxedOriginID bounds the IDs whose boxed origins are kept.
const maxBoxedOriginID = 1 << 16

// SetOrigin is basegraph.Set(properties, OriginalElement, origin). Origins
// made by the kind constructors (NodeOrigin, LEdgeOrigin, ...) are stored
// from a box kept per kind and ID: an interface holding a struct value cannot
// be changed, so a shared box holding an equal origin is the same as boxing
// the origin anew.
func (arena *LGraphArena) SetOrigin(properties *basegraph.PropertyMap, origin Origin) {
	id, canonical := origin.canonicalID()
	if !canonical || id >= maxBoxedOriginID {
		basegraph.Set(properties, OriginalElement, origin)
		return
	}
	boxes := arena.origins[origin.Kind]
	if int(id) >= len(boxes) {
		if int(id) < cap(boxes) {
			boxes = boxes[:int(id)+1]
		} else {
			grown := make([]any, int(id)+1, 2*int(id)+2)
			copy(grown, boxes)
			boxes = grown
		}
		arena.origins[origin.Kind] = boxes
	}
	if boxes[id] == nil {
		boxes[id] = origin
	}
	properties.SetRawKey(OriginalElement.Key(), boxes[id])
}

const idCellChunk = 256

// idCellTable hands out pointers to cells whose value is their index. Cells
// live in fixed chunks that are never moved or written after creation.
type idCellTable[T ~uint32] struct {
	chunks []*[idCellChunk]T
}

func (table *idCellTable[T]) cell(id T) *T {
	index := int(id) / idCellChunk
	for index >= len(table.chunks) {
		chunk := new([idCellChunk]T)
		base := len(table.chunks) * idCellChunk
		for offset := range chunk {
			chunk[offset] = T(base + offset)
		}
		table.chunks = append(table.chunks, chunk)
	}
	return &table.chunks[index][int(id)%idCellChunk]
}

func NewLGraphArena() *LGraphArena { return &LGraphArena{} }

// Released arenas keep their slices' storage for the next layout. Element
// pointers into an arena are never used after the arena may have grown
// (checked statically and by tests with an arena that moves on every Add), so
// the capacity an arena starts with is not observable.
var arenaPool reuse.Pool[LGraphArena]

// A released arena in the slot may stay alive indefinitely, so the storage it
// keeps is bounded: an arena of more than maxSlotArenaElements element slots
// goes to the sync.Pool only (gc), which garbage collection empties, or is
// dropped (TinyGo, see reuse.OversizedToPool), a kept slot keeps at most maxKeptPropertyEntries entries of
// property storage, and at most maxKeptOriginBoxes boxed origins of each kind
// are kept. A slot with its storage takes at most about 1.2 KB on a 64-bit
// platform (an LNode is 320 bytes, 32 property entries 768, its ID lists
// little), so a kept arena about 5 MB with its origin boxes (0.7 MB), ID
// cells and processors' scratch (sized by the same elements); the corpus's
// largest arena has 2449 slots and 15747 entries of property storage.
const (
	maxSlotArenaElements   = 1 << 12
	maxKeptPropertyEntries = 32
	maxKeptOriginBoxes     = 1 << 11
)

// AcquireLGraphArena returns an empty arena, reusing released storage when
// some is available.
func AcquireLGraphArena() *LGraphArena {
	if arena := arenaPool.Get(); arena != nil {
		return arena
	}
	return &LGraphArena{}
}

// ReleaseLGraphArena empties arena and offers its storage for reuse. Neither
// the arena nor anything obtained from it may be used afterwards.
//
// The released elements stay in their slots beyond the slices' lengths: each
// Add* overwrites its slot with a whole new element, taking only the slot's
// property storage. The elements give up here what they refer to that may be
// the caller's (property values, label texts): their property storage is
// emptied (and dropped when large), and the boxes of the last stored point
// and spacing are dropped. Nothing else reads beyond the lengths.
func ReleaseLGraphArena(arena *LGraphArena) {
	if arena == nil {
		return
	}
	elements := cap(arena.Graphs) + cap(arena.Nodes) + cap(arena.Ports) +
		cap(arena.Edges) + cap(arena.Labels) + cap(arena.Layers)
	oversized := elements > maxSlotArenaElements
	if oversized && !reuse.OversizedToPool {
		return
	}
	for index := range arena.Graphs {
		keepPropertyStorage(&arena.Graphs[index].Properties)
	}
	for index := range arena.Nodes {
		keepPropertyStorage(&arena.Nodes[index].Properties)
	}
	for index := range arena.Ports {
		keepPropertyStorage(&arena.Ports[index].Properties)
	}
	for index := range arena.Edges {
		keepPropertyStorage(&arena.Edges[index].Properties)
	}
	for index := range arena.Labels {
		arena.Labels[index].Text = ""
		keepPropertyStorage(&arena.Labels[index].Properties)
	}
	for index := range arena.Layers {
		keepPropertyStorage(&arena.Layers[index].Properties)
	}
	for kind := range arena.origins {
		if cap(arena.origins[kind]) > maxKeptOriginBoxes {
			arena.origins[kind] = nil
		}
	}
	arena.lastPointBox, arena.lastSpacingBox = nil, nil
	arena.Graphs, arena.Nodes, arena.Ports = arena.Graphs[:0], arena.Nodes[:0], arena.Ports[:0]
	arena.Edges, arena.Labels, arena.Layers = arena.Edges[:0], arena.Labels[:0], arena.Layers[:0]
	arenaPool.Put(arena, !oversized)
}

// keepPropertyStorage empties properties for the next element of its slot:
// the entries are cleared, so that nothing they refer to is kept alive, and
// their storage is kept unless it holds more than maxKeptPropertyEntries.
func keepPropertyStorage(properties *basegraph.PropertyMap) {
	if properties.EntryCapacity() > maxKeptPropertyEntries {
		*properties = basegraph.PropertyMap{}
		return
	}
	*properties = properties.Recycled()
}

func (arena *LGraphArena) Graph(id LGraphID) *LGraph { return &arena.Graphs[id.Index()] }
func (arena *LGraphArena) Node(id LNodeID) *LNode    { return &arena.Nodes[id.Index()] }
func (arena *LGraphArena) Port(id LPortID) *LPort    { return &arena.Ports[id.Index()] }
func (arena *LGraphArena) Edge(id LEdgeID) *LEdge    { return &arena.Edges[id.Index()] }
func (arena *LGraphArena) Label(id LLabelID) *LLabel { return &arena.Labels[id.Index()] }
func (arena *LGraphArena) Layer(id LayerID) *Layer   { return &arena.Layers[id.Index()] }

// Each Add* gives the new element the property storage of the element left
// in its slot by ReleaseLGraphArena (none in a fresh arena), emptied there
// (Recycled again here, which then finds nothing to clear); an element's
// property map starts empty either way.

func (arena *LGraphArena) AddGraph() LGraphID {
	id := LGraphID(len(arena.Graphs))
	var properties basegraph.PropertyMap
	if n := len(arena.Graphs); n < cap(arena.Graphs) {
		properties = arena.Graphs[:n+1][n].Properties.Recycled()
	}
	arena.Graphs = append(arena.Graphs, LGraph{Properties: properties})
	return id
}

// AddNode is new LNode(graph); callers add the node to graph.LayerlessNodes
// explicitly, just as the Java importer does.
func (arena *LGraphArena) AddNode(graphID LGraphID) LNodeID {
	id := LNodeID(len(arena.Nodes))
	var properties basegraph.PropertyMap
	if n := len(arena.Nodes); n < cap(arena.Nodes) {
		properties = arena.Nodes[:n+1][n].Properties.Recycled()
	}
	arena.Nodes = append(arena.Nodes, LNode{Graph: graphID, Type: NormalNode, Properties: properties})
	return id
}

func (arena *LGraphArena) AddPort() LPortID {
	id := LPortID(len(arena.Ports))
	var properties basegraph.PropertyMap
	if n := len(arena.Ports); n < cap(arena.Ports) {
		properties = arena.Ports[:n+1][n].Properties.Recycled()
	}
	arena.Ports = append(arena.Ports, LPort{ConnectedToExternalNodes: true, Properties: properties})
	return id
}

func (arena *LGraphArena) AddEdge() LEdgeID {
	id := LEdgeID(len(arena.Edges))
	var properties basegraph.PropertyMap
	if n := len(arena.Edges); n < cap(arena.Edges) {
		properties = arena.Edges[:n+1][n].Properties.Recycled()
	}
	arena.Edges = append(arena.Edges, LEdge{Properties: properties})
	return id
}

func (arena *LGraphArena) AddLabel(text string) LLabelID {
	id := LLabelID(len(arena.Labels))
	var properties basegraph.PropertyMap
	if n := len(arena.Labels); n < cap(arena.Labels) {
		properties = arena.Labels[:n+1][n].Properties.Recycled()
	}
	arena.Labels = append(arena.Labels, LLabel{Text: text, Properties: properties})
	return id
}

// AddLayer is new Layer(graph); it deliberately does not append the layer to
// graph.Layers. This mirrors the subtle Java constructor behavior.
func (arena *LGraphArena) AddLayer(graphID LGraphID) LayerID {
	id := LayerID(len(arena.Layers))
	var properties basegraph.PropertyMap
	if n := len(arena.Layers); n < cap(arena.Layers) {
		properties = arena.Layers[:n+1][n].Properties.Recycled()
	}
	arena.Layers = append(arena.Layers, Layer{Graph: graphID, Properties: properties})
	return id
}

func (arena *LGraphArena) SetPortNode(portID LPortID, nodeID *LNodeID) {
	port := arena.Port(portID)
	if port.Node != nil {
		old := *port.Node
		arena.Node(old).Ports = removePort(arena.Node(old).Ports, portID)
		arena.InvalidatePortSideCache(old)
	}
	port.Node = nil
	if nodeID != nil {
		port.Node = arena.nodeCells.cell(*nodeID)
	}
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
	edge.Source = nil
	if portID != nil {
		edge.Source = arena.portCells.cell(*portID)
	}
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
	edge.Target = nil
	if portID != nil {
		edge.Target = arena.portCells.cell(*portID)
	}
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
	node.Layer = nil
	if layerID != nil {
		node.Layer = arena.layerCells.cell(*layerID)
	}
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

// PortEdgeLists returns the port's incoming and outgoing edge lists;
// iterating them in order visits PortConnectedEdges' edges without copying
// them. Callers must not change the lists while iterating.
func (arena *LGraphArena) PortEdgeLists(portID LPortID) [2][]LEdgeID {
	port := arena.Port(portID)
	return [2][]LEdgeID{port.IncomingEdges, port.OutgoingEdges}
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

// GraphAllNodesScratch is GraphAllNodes filled into storage the arena keeps:
// the list is valid until the next GraphAllNodesScratch call, so only a
// caller that does not call it again (directly or through what it calls)
// while using the list may use it. It is nil when there are no nodes, like
// GraphAllNodes' result.
func (arena *LGraphArena) GraphAllNodesScratch(graphID LGraphID) []LNodeID {
	graph := arena.Graph(graphID)
	nodes := append(arena.allNodes[:0], graph.LayerlessNodes...)
	for _, layerID := range graph.Layers {
		nodes = append(nodes, arena.Layer(layerID).Nodes...)
	}
	arena.allNodes = nodes
	if len(nodes) == 0 {
		return nil
	}
	return nodes
}

func (arena *LGraphArena) GraphAllNodes(graphID LGraphID) []LNodeID {
	graph := arena.Graph(graphID)
	// One allocation of the final size; nil when there are no nodes, as the
	// appends to a nil slice gave.
	total := len(graph.LayerlessNodes)
	for _, layerID := range graph.Layers {
		total += len(arena.Layer(layerID).Nodes)
	}
	if total == 0 {
		return nil
	}
	nodes := make([]LNodeID, 0, total)
	nodes = append(nodes, graph.LayerlessNodes...)
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
