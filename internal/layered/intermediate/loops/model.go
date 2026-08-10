// Copyright (c) 2019 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package loops contains ELK Layered 0.8.1's temporary self-loop model.
package loops

import (
	"fmt"
	"math"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// HolderProperty is InternalProperties.SELF_LOOP_HOLDER. Keeping the
// property here avoids making the shared layered package depend on an
// intermediate processor package.
var HolderProperty = basegraph.NewProperty[*Holder]("selfLoopHolder")

type Type uint8

const (
	OneSide Type = iota
	TwoSidesCorner
	TwoSidesOpposing
	ThreeSides
	FourSides
)

func TypeFromPortSides(sides [5]bool) (Type, error) {
	if sides[layered.UndefinedSide] {
		return 0, fmt.Errorf("self-loop port sides must not contain UNDEFINED")
	}
	count := 0
	for side := layered.NorthSide; side <= layered.WestSide; side++ {
		if sides[side] {
			count++
		}
	}
	switch count {
	case 1:
		return OneSide, nil
	case 2:
		if sides[layered.EastSide] && sides[layered.WestSide] ||
			sides[layered.NorthSide] && sides[layered.SouthSide] {
			return TwoSidesOpposing, nil
		}
		return TwoSidesCorner, nil
	case 3:
		return ThreeSides, nil
	case 4:
		return FourSides, nil
	default:
		return 0, fmt.Errorf("self-loop must occupy between one and four port sides")
	}
}

type Port struct {
	LPort            layered.LPortID
	HadOnlySelfLoops bool
	Incoming         []*Edge
	Outgoing         []*Edge
	Hidden           bool
}

func (port *Port) NetFlow() int { return len(port.Incoming) - len(port.Outgoing) }

type Edge struct {
	LEdge     layered.LEdgeID
	HyperLoop *HyperLoop
	Source    *Port
	Target    *Port
}

// IsInline reports whether this self-loop edge has at least one inline label.
// ELK 0.11 uses the per-edge flag while routing opposing-side self loops.
func (edge *Edge) IsInline(arena *layered.LGraphArena) bool {
	for _, labelID := range arena.Edge(edge.LEdge).Labels {
		if basegraph.Get(&arena.Label(labelID).Properties, layered.EdgeLabelsInline) {
			return true
		}
	}
	return false
}

// LabelSide returns the aggregate side selected for this edge's hyper-loop.
func (edge *Edge) LabelSide() layered.PortSide {
	if edge.HyperLoop == nil || edge.HyperLoop.Labels == nil {
		return layered.UndefinedSide
	}
	return edge.HyperLoop.Labels.Side
}

// LabelAlignment is SelfHyperLoopLabels.Alignment in declaration order.
type LabelAlignment uint8

const (
	LabelCenter LabelAlignment = iota
	LabelLeft
	LabelRight
	LabelTop
)

type Labels struct {
	ID                 int
	LLabels            []layered.LLabelID
	Size               geometry.Point
	Position           geometry.Point
	LayoutDirection    layered.Direction
	LabelLabelSpacing  float64
	Side               layered.PortSide
	Alignment          LabelAlignment
	AlignmentReference *Port
}

func newLabels(arena *layered.LGraphArena, loop *HyperLoop) *Labels {
	nodeID := loop.Holder.LNode
	graph := arena.Graph(arena.NodeGraph(nodeID))
	return &Labels{
		LayoutDirection:   basegraph.Get(&graph.Properties, layered.DirectionOption),
		LabelLabelSpacing: IndividualOrInherited(arena, nodeID, layered.SpacingLabelLabel),
		Side:              layered.UndefinedSide,
	}
}

func (labels *Labels) Add(arena *layered.LGraphArena, ids []layered.LLabelID) {
	for _, id := range ids {
		labels.LLabels = append(labels.LLabels, id)
		size := arena.Label(id).Size
		if directionHorizontal(labels.LayoutDirection) {
			labels.Size.X = math.Max(labels.Size.X, size.X)
			labels.Size.Y += size.Y
			if len(labels.LLabels) > 1 {
				labels.Size.Y += labels.LabelLabelSpacing
			}
		} else {
			labels.Size.X += size.X
			labels.Size.Y = math.Max(labels.Size.Y, size.Y)
			if len(labels.LLabels) > 1 {
				labels.Size.X += labels.LabelLabelSpacing
			}
		}
	}
}

// ApplyPlacement transfers the aggregate label box back to individual labels.
func (labels *Labels) ApplyPlacement(arena *layered.LGraphArena, offset geometry.Point) {
	x, y := labels.Position.X, labels.Position.Y
	for _, labelID := range labels.LLabels {
		label := arena.Label(labelID)
		if directionHorizontal(labels.LayoutDirection) {
			switch {
			case labels.Alignment == LabelLeft || labels.Side == layered.EastSide:
				label.Position.X = x
			case labels.Alignment == LabelRight || labels.Side == layered.WestSide:
				label.Position.X = x + labels.Size.X - label.Size.X
			default:
				label.Position.X = x + (labels.Size.X-label.Size.X)/2
			}
			label.Position.Y = y
			y += label.Size.Y + labels.LabelLabelSpacing
		} else {
			label.Position.X = x
			if labels.Side == layered.NorthSide {
				label.Position.Y = y + labels.Size.Y - label.Size.Y
			} else {
				label.Position.Y = y
			}
			x += label.Size.X + labels.LabelLabelSpacing
		}
		label.Position = label.Position.Add(offset)
	}
}

type HyperLoop struct {
	Holder *Holder
	Ports  []*Port
	Edges  []*Edge
	Labels *Labels

	Type            Type
	PortsBySide     [5][]*Port
	LeftmostPort    *Port
	RightmostPort   *Port
	OccupiedSides   [5]bool
	RoutingSlot     [5]int
	portsPerSideSet bool
}

func (loop *HyperLoop) AddEdge(arena *layered.LGraphArena, edge *Edge) {
	for _, existing := range loop.Edges {
		if existing == edge {
			return
		}
	}
	loop.Edges = append(loop.Edges, edge)
	edge.HyperLoop = loop
	loop.addPort(edge.Source)
	loop.addPort(edge.Target)
	if len(arena.Edge(edge.LEdge).Labels) != 0 {
		if loop.Labels == nil {
			loop.Labels = newLabels(arena, loop)
		}
		loop.Labels.Add(arena, arena.Edge(edge.LEdge).Labels)
	}
}

func (loop *HyperLoop) addPort(port *Port) {
	for _, existing := range loop.Ports {
		if existing == port {
			return
		}
	}
	loop.Ports = append(loop.Ports, port)
}

func (loop *HyperLoop) ComputePortsPerSide(arena *layered.LGraphArena) error {
	if loop.portsPerSideSet {
		return fmt.Errorf("self-loop ports per side already computed")
	}
	var sides [5]bool
	for _, port := range loop.Ports {
		side := arena.Port(port.LPort).Side
		if side == layered.UndefinedSide {
			return fmt.Errorf("self-loop port %d has undefined side", port.LPort)
		}
		loop.PortsBySide[side] = append(loop.PortsBySide[side], port)
		sides[side] = true
	}
	typeValue, err := TypeFromPortSides(sides)
	if err != nil {
		return err
	}
	loop.Type = typeValue
	loop.portsPerSideSet = true
	return nil
}

func (loop *HyperLoop) HasPortsOnSide(side layered.PortSide) bool {
	return len(loop.PortsBySide[side]) != 0
}

func (loop *HyperLoop) PortSides() []layered.PortSide {
	result := make([]layered.PortSide, 0, 4)
	for side := layered.NorthSide; side <= layered.WestSide; side++ {
		if loop.HasPortsOnSide(side) {
			result = append(result, side)
		}
	}
	return result
}

func (loop *HyperLoop) SetRoutingSlot(side layered.PortSide, slot int) {
	loop.RoutingSlot[side] = slot
	if slot+1 > loop.Holder.RoutingSlotCount[side] {
		loop.Holder.RoutingSlotCount[side] = slot + 1
	}
}

type Holder struct {
	LNode            layered.LNodeID
	HyperLoops       []*HyperLoop
	Ports            map[layered.LPortID]*Port
	PortOrder        []layered.LPortID
	PortsHidden      bool
	RoutingSlotCount [5]int
}

func NeedsProcessing(arena *layered.LGraphArena, nodeID layered.LNodeID) bool {
	if arena.Node(nodeID).Type != layered.NormalNode {
		return false
	}
	for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
		if arena.EdgeIsSelfLoop(edgeID) {
			return true
		}
	}
	return false
}

func Install(arena *layered.LGraphArena, nodeID layered.LNodeID) (*Holder, error) {
	if !NeedsProcessing(arena, nodeID) {
		return nil, fmt.Errorf("self-loop holder requires a normal node with self loops")
	}
	holder := &Holder{LNode: nodeID, Ports: make(map[layered.LPortID]*Port)}
	basegraph.Set(&arena.Node(nodeID).Properties, HolderProperty, holder)
	if err := holder.initialize(arena); err != nil {
		return nil, err
	}
	return holder, nil
}

func (holder *Holder) initialize(arena *layered.LGraphArena) error {
	for _, edgeID := range arena.NodeOutgoingEdges(holder.LNode) {
		if !arena.EdgeIsSelfLoop(edgeID) {
			continue
		}
		edge := arena.Edge(edgeID)
		source := holder.portFor(arena, *edge.Source)
		target := holder.portFor(arena, *edge.Target)
		slEdge := &Edge{LEdge: edgeID, Source: source, Target: target}
		source.Outgoing = append(source.Outgoing, slEdge)
		target.Incoming = append(target.Incoming, slEdge)
	}

	// ELK temporarily reuses LPort.id for BFS state.
	for _, portID := range holder.PortOrder {
		arena.Port(portID).ID = 0
	}
	for _, portID := range holder.PortOrder {
		if arena.Port(portID).ID == 0 {
			holder.HyperLoops = append(holder.HyperLoops, holder.initializeHyperLoop(arena, holder.Ports[portID]))
		}
	}
	return nil
}

func (holder *Holder) portFor(arena *layered.LGraphArena, portID layered.LPortID) *Port {
	if port, found := holder.Ports[portID]; found {
		return port
	}
	connected := arena.PortConnectedEdges(portID)
	hadOnly := len(connected) != 0
	for _, edgeID := range connected {
		if !arena.EdgeIsSelfLoop(edgeID) {
			hadOnly = false
			break
		}
	}
	port := &Port{LPort: portID, HadOnlySelfLoops: hadOnly}
	holder.Ports[portID] = port
	holder.PortOrder = append(holder.PortOrder, portID)
	return port
}

func (holder *Holder) initializeHyperLoop(arena *layered.LGraphArena, start *Port) *HyperLoop {
	loop := &HyperLoop{Holder: holder}
	queue := []*Port{start}
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		arena.Port(current.LPort).ID = 1
		for _, edge := range current.Outgoing {
			loop.AddEdge(arena, edge)
			if arena.Port(edge.Target.LPort).ID == 0 {
				queue = append(queue, edge.Target)
			}
		}
		for _, edge := range current.Incoming {
			loop.AddEdge(arena, edge)
			if arena.Port(edge.Source.LPort).ID == 0 {
				queue = append(queue, edge.Source)
			}
		}
	}
	return loop
}

func IndividualOrInherited(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	property basegraph.Property[float64],
) float64 {
	node := arena.Node(nodeID)
	if individual, found := basegraph.TryGet(&node.Properties, layered.SpacingIndividual); found {
		if value, explicit := basegraph.TryGet(&individual.Properties, property); explicit {
			return value
		}
	}
	return basegraph.Get(&arena.Graph(arena.NodeGraph(nodeID)).Properties, property)
}

func directionHorizontal(direction layered.Direction) bool {
	return direction == layered.DirectionRight || direction == layered.DirectionLeft
}
