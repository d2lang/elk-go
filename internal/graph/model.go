// Copyright (c) 2016 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package graph implements ELK's mutable graph as deterministic typed arenas.
package graph

import "github.com/d2lang/elk-go/internal/geometry"

type NodeID uint32
type PortID uint32
type EdgeID uint32
type LabelID uint32
type SectionID uint32

// ShapeID identifies anything an edge may connect to.
type ShapeID struct {
	Node NodeID
	Port PortID
	Kind ShapeKind
}

type ShapeKind uint8

const (
	NodeShape ShapeKind = iota
	PortShape
)

func NodeRef(id NodeID) ShapeID { return ShapeID{Node: id, Kind: NodeShape} }
func PortRef(id PortID) ShapeID { return ShapeID{Port: id, Kind: PortShape} }

// ElementID identifies anything that may own labels.
type ElementID struct {
	Node  NodeID
	Port  PortID
	Edge  EdgeID
	Label LabelID
	Kind  ElementKind
}

type ElementKind uint8

const (
	NodeElement ElementKind = iota
	PortElement
	EdgeElement
	LabelElement
)

func NodeElementRef(id NodeID) ElementID   { return ElementID{Node: id, Kind: NodeElement} }
func PortElementRef(id PortID) ElementID   { return ElementID{Port: id, Kind: PortElement} }
func EdgeElementRef(id EdgeID) ElementID   { return ElementID{Edge: id, Kind: EdgeElement} }
func LabelElementRef(id LabelID) ElementID { return ElementID{Label: id, Kind: LabelElement} }

// Shape stores parent-relative geometry. Presence records which values were
// explicitly present in imported JSON before layout.
type Shape struct {
	X, Y, Width, Height float64
	Presence            ShapePresence
}

type ShapePresence uint8

const (
	HasX ShapePresence = 1 << iota
	HasY
	HasWidth
	HasHeight
)

func (shape Shape) Position() geometry.Point { return geometry.Point{X: shape.X, Y: shape.Y} }
func (shape Shape) Rectangle() geometry.Rectangle {
	return geometry.Rectangle{X: shape.X, Y: shape.Y, Width: shape.Width, Height: shape.Height}
}
func (shape *Shape) SetLocation(x, y float64) { shape.X, shape.Y = x, y; shape.Presence |= HasX | HasY }
func (shape *Shape) SetDimensions(width, height float64) {
	shape.Width, shape.Height = width, height
	shape.Presence |= HasWidth | HasHeight
}

type Node struct {
	Identifier        string
	IdentifierPresent bool
	Shape             Shape
	Properties        PropertyMap
	Labels            []LabelID
	Parent            *NodeID
	Children          []NodeID
	Ports             []PortID
	ContainedEdges    []EdgeID
	OutgoingEdges     []EdgeID
	IncomingEdges     []EdgeID
}

type Port struct {
	Identifier        string
	IdentifierPresent bool
	Shape             Shape
	Properties        PropertyMap
	Labels            []LabelID
	Parent            NodeID
	OutgoingEdges     []EdgeID
	IncomingEdges     []EdgeID
}

type Label struct {
	Identifier        string
	IdentifierPresent bool
	Text              string
	TextPresent       bool
	Shape             Shape
	Properties        PropertyMap
	Labels            []LabelID
	Parent            ElementID
}

type Edge struct {
	Identifier        string
	IdentifierPresent bool
	Properties        PropertyMap
	Labels            []LabelID
	ContainingNode    *NodeID
	Sources           []ShapeID
	Targets           []ShapeID
	Sections          []SectionID
}

type EdgeSection struct {
	Identifier               string
	IdentifierPresent        bool
	Properties               PropertyMap
	Parent                   EdgeID
	Start, End               geometry.Point
	StartPresent, EndPresent bool
	BendPoints               geometry.Chain
	BendPointsPresent        bool
	OutgoingShape            *ShapeID
	IncomingShape            *ShapeID
	OutgoingSections         []SectionID
	IncomingSections         []SectionID
}

// Model owns every element. Arena order is creation order and is never
// derived from map iteration.
type Model struct {
	Nodes    []Node
	Ports    []Port
	Edges    []Edge
	Labels   []Label
	Sections []EdgeSection
	Root     NodeID
}

func NewModel() *Model {
	return &Model{Nodes: []Node{{}}, Root: 0}
}

func (model *Model) Node(id NodeID) *Node              { return &model.Nodes[int(id)] }
func (model *Model) Port(id PortID) *Port              { return &model.Ports[int(id)] }
func (model *Model) Edge(id EdgeID) *Edge              { return &model.Edges[int(id)] }
func (model *Model) Label(id LabelID) *Label           { return &model.Labels[int(id)] }
func (model *Model) Section(id SectionID) *EdgeSection { return &model.Sections[int(id)] }

func (model *Model) AddNode(parent *NodeID) NodeID {
	id := NodeID(len(model.Nodes))
	model.Nodes = append(model.Nodes, Node{Parent: copyNodeID(parent)})
	if parent != nil {
		model.Node(*parent).Children = append(model.Node(*parent).Children, id)
	}
	return id
}

func (model *Model) AddPort(parent NodeID) PortID {
	id := PortID(len(model.Ports))
	model.Ports = append(model.Ports, Port{Parent: parent})
	model.Node(parent).Ports = append(model.Node(parent).Ports, id)
	return id
}

func (model *Model) AddLabel(parent ElementID, text string) LabelID {
	id := LabelID(len(model.Labels))
	model.Labels = append(model.Labels, Label{Parent: parent, Text: text})
	switch parent.Kind {
	case NodeElement:
		model.Node(parent.Node).Labels = append(model.Node(parent.Node).Labels, id)
	case PortElement:
		model.Port(parent.Port).Labels = append(model.Port(parent.Port).Labels, id)
	case EdgeElement:
		model.Edge(parent.Edge).Labels = append(model.Edge(parent.Edge).Labels, id)
	case LabelElement:
		model.Label(parent.Label).Labels = append(model.Label(parent.Label).Labels, id)
	}
	return id
}

func (model *Model) AddEdge(containingNode *NodeID) EdgeID {
	id := EdgeID(len(model.Edges))
	model.Edges = append(model.Edges, Edge{ContainingNode: copyNodeID(containingNode)})
	if containingNode != nil {
		model.Node(*containingNode).ContainedEdges = append(model.Node(*containingNode).ContainedEdges, id)
	}
	return id
}

func (model *Model) AddSource(edge EdgeID, source ShapeID) {
	model.Edge(edge).Sources = append(model.Edge(edge).Sources, source)
	if source.Kind == NodeShape {
		model.Node(source.Node).OutgoingEdges = append(model.Node(source.Node).OutgoingEdges, edge)
	} else {
		model.Port(source.Port).OutgoingEdges = append(model.Port(source.Port).OutgoingEdges, edge)
	}
}

func (model *Model) AddTarget(edge EdgeID, target ShapeID) {
	model.Edge(edge).Targets = append(model.Edge(edge).Targets, target)
	if target.Kind == NodeShape {
		model.Node(target.Node).IncomingEdges = append(model.Node(target.Node).IncomingEdges, edge)
	} else {
		model.Port(target.Port).IncomingEdges = append(model.Port(target.Port).IncomingEdges, edge)
	}
}

func (model *Model) AddSection(edge EdgeID) SectionID {
	id := SectionID(len(model.Sections))
	model.Sections = append(model.Sections, EdgeSection{Parent: edge})
	model.Edge(edge).Sections = append(model.Edge(edge).Sections, id)
	return id
}

func (model *Model) ShapeNode(shape ShapeID) NodeID {
	if shape.Kind == NodeShape {
		return shape.Node
	}
	return model.Port(shape.Port).Parent
}

func (model *Model) Shape(shape ShapeID) *Shape {
	if shape.Kind == NodeShape {
		return &model.Node(shape.Node).Shape
	}
	return &model.Port(shape.Port).Shape
}

func (model *Model) Properties(element ElementID) *PropertyMap {
	switch element.Kind {
	case NodeElement:
		return &model.Node(element.Node).Properties
	case PortElement:
		return &model.Port(element.Port).Properties
	case EdgeElement:
		return &model.Edge(element.Edge).Properties
	case LabelElement:
		return &model.Label(element.Label).Properties
	default:
		panic("unknown graph element kind")
	}
}

func (model *Model) Descendants(node NodeID) []NodeID {
	result := make([]NodeID, 0)
	stack := append([]NodeID(nil), model.Node(node).Children...)
	for len(stack) > 0 {
		current := stack[0]
		stack = stack[1:]
		result = append(result, current)
		children := model.Node(current).Children
		if len(children) > 0 {
			next := make([]NodeID, 0, len(children)+len(stack))
			next = append(next, children...)
			next = append(next, stack...)
			stack = next
		}
	}
	return result
}

func (model *Model) IsDescendant(node, ancestor NodeID) bool {
	for parent := model.Node(node).Parent; parent != nil; parent = model.Node(*parent).Parent {
		if *parent == ancestor {
			return true
		}
	}
	return false
}

func (model *Model) Depth(node NodeID) int {
	depth := 0
	for parent := model.Node(node).Parent; parent != nil; parent = model.Node(*parent).Parent {
		depth++
	}
	return depth
}

func (model *Model) LowestCommonAncestor(a, b NodeID) (NodeID, bool) {
	ancestors := make(map[NodeID]struct{})
	for current := &a; current != nil; current = model.Node(*current).Parent {
		ancestors[*current] = struct{}{}
	}
	for current := &b; current != nil; current = model.Node(*current).Parent {
		if _, found := ancestors[*current]; found {
			return *current, true
		}
	}
	return 0, false
}

func (model *Model) BestEdgeContainment(edge EdgeID) (NodeID, bool) {
	currentEdge := model.Edge(edge)
	incident := make([]NodeID, 0, len(currentEdge.Sources)+len(currentEdge.Targets))
	for _, shape := range currentEdge.Sources {
		incident = append(incident, model.ShapeNode(shape))
	}
	for _, shape := range currentEdge.Targets {
		incident = append(incident, model.ShapeNode(shape))
	}
	if len(incident) == 0 {
		panic("edge must have at least one endpoint")
	}
	if len(incident) == 1 {
		parent := model.Node(incident[0]).Parent
		if parent == nil {
			return 0, false
		}
		return *parent, true
	}
	allSame := true
	for _, node := range incident[1:] {
		if node != incident[0] {
			allSame = false
			break
		}
	}
	if allSame {
		parent := model.Node(incident[0]).Parent
		if parent == nil {
			return 0, false
		}
		return *parent, true
	}
	common := incident[0]
	for _, node := range incident[1:] {
		if node == common || model.IsDescendant(node, common) {
			continue
		}
		var found bool
		common, found = model.LowestCommonAncestor(common, node)
		if !found {
			return 0, false
		}
	}
	return common, true
}

func (model *Model) UpdateContainment(edge EdgeID) {
	best, found := model.BestEdgeContainment(edge)
	current := model.Edge(edge).ContainingNode
	if current != nil && (!found || *current != best) {
		model.Node(*current).ContainedEdges = removeEdge(model.Node(*current).ContainedEdges, edge)
	}
	if !found {
		model.Edge(edge).ContainingNode = nil
		return
	}
	if current == nil || *current != best {
		model.Edge(edge).ContainingNode = copyNodeID(&best)
		model.Node(best).ContainedEdges = append(model.Node(best).ContainedEdges, edge)
	}
}

// FirstSection returns an edge's first route section. Resetting preserves that
// section's identity while replacing the complete pre-existing route.
func (model *Model) FirstSection(edge EdgeID, reset bool) SectionID {
	if len(model.Edge(edge).Sections) == 0 {
		return model.AddSection(edge)
	}
	id := model.Edge(edge).Sections[0]
	if reset {
		model.Edge(edge).Sections = model.Edge(edge).Sections[:1]
		section := model.Section(id)
		identifier, present, parent := section.Identifier, section.IdentifierPresent, section.Parent
		*section = EdgeSection{Identifier: identifier, IdentifierPresent: present, Parent: parent}
	}
	return id
}

func (model *Model) SectionChain(section SectionID) geometry.Chain {
	value := model.Section(section)
	result := make(geometry.Chain, 0, len(value.BendPoints)+2)
	result = append(result, value.Start)
	result = append(result, value.BendPoints...)
	result = append(result, value.End)
	return result
}

func copyNodeID(value *NodeID) *NodeID {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}
func removeEdge(edges []EdgeID, target EdgeID) []EdgeID {
	for index, edge := range edges {
		if edge == target {
			return append(edges[:index], edges[index+1:]...)
		}
	}
	return edges
}
