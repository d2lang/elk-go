// Copyright (c) 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"fmt"
	"math"

	"github.com/d2lang/elk-go/internal/layered"
)

type VerticalDirection uint8

const (
	VerticalBalanced VerticalDirection = iota
	VerticalDown
	VerticalUp
)

type HorizontalDirection uint8

const (
	HorizontalBalanced HorizontalDirection = iota
	HorizontalRight
	HorizontalLeft
)

// BKAlignedLayout contains all per-node data for one of the four directional
// Brandes-Koepf layouts. Optional entries preserve Java's null-filled object
// arrays and make missing algorithm initialization fail loudly. (Java never
// shares or mutates a boxed entry through an alias; every store is a fresh
// value, so a value plus a set flag represents an entry exactly.)
type BKAlignedLayout struct {
	Root       []optNode
	BlockSize  []optFloat
	Align      []optNode
	InnerShift []optFloat
	Sink       []optNode
	Shift      []optFloat
	Y          []optFloat
	VDirection VerticalDirection
	HDirection HorizontalDirection
	SU         []bool
	OD         []bool
	Graph      layered.LGraphID
}

func NewBKAlignedLayout(
	graphID layered.LGraphID,
	nodeCount int,
	vertical VerticalDirection,
	horizontal HorizontalDirection,
) *BKAlignedLayout {
	layout := &BKAlignedLayout{}
	layout.init(graphID, nodeCount, vertical, horizontal,
		make([]optNode, 3*nodeCount), make([]optFloat, 4*nodeCount), make([]bool, 2*nodeCount))
	return layout
}

// init makes layout a new layout over the given zeroed storage (3, 4 and 2
// times nodeCount entries), exactly as NewBKAlignedLayout's.
func (layout *BKAlignedLayout) init(
	graphID layered.LGraphID,
	nodeCount int,
	vertical VerticalDirection,
	horizontal HorizontalDirection,
	nodes []optNode,
	floats []optFloat,
	flags []bool,
) {
	// Three backing arrays, cut into capacity-capped per-field slices.
	odd := flags[nodeCount : 2*nodeCount : 2*nodeCount]
	for index := range odd {
		odd[index] = true
	}
	cut := func(start int) (int, int, int) {
		return start * nodeCount, (start + 1) * nodeCount, (start + 1) * nodeCount
	}
	r0, r1, r2 := cut(0)
	a0, a1, a2 := cut(1)
	s0, s1, s2 := cut(2)
	b0, b1, b2 := cut(0)
	i0, i1, i2 := cut(1)
	h0, h1, h2 := cut(2)
	y0, y1, y2 := cut(3)
	*layout = BKAlignedLayout{
		Root: nodes[r0:r1:r2], BlockSize: floats[b0:b1:b2],
		Align: nodes[a0:a1:a2], InnerShift: floats[i0:i1:i2],
		Sink: nodes[s0:s1:s2], Shift: floats[h0:h1:h2],
		Y: floats[y0:y1:y2], SU: flags[0:nodeCount:nodeCount], OD: odd,
		VDirection: vertical, HDirection: horizontal, Graph: graphID,
	}
}

func (layout *BKAlignedLayout) Cleanup() {
	layout.Root = nil
	layout.BlockSize = nil
	layout.Align = nil
	layout.InnerShift = nil
	layout.Sink = nil
	layout.Shift = nil
	layout.Y = nil
}

func (layout *BKAlignedLayout) LayoutSize(arena *layered.LGraphArena) float64 {
	minimum, maximum := math.Inf(1), math.Inf(-1)
	for _, layerID := range arena.Graph(layout.Graph).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			index := arena.Node(nodeID).ID
			yMin := requireFloat(layout.Y[index], "y")
			rootID := requireNode(layout.Root[index], "root")
			yMax := yMin + requireFloat(layout.BlockSize[arena.Node(rootID).ID], "block size")
			minimum = math.Min(minimum, yMin)
			maximum = math.Max(maximum, yMax)
		}
	}
	return maximum - minimum
}

func (layout *BKAlignedLayout) CalculateDelta(
	arena *layered.LGraphArena,
	sourceID, targetID layered.LPortID,
) float64 {
	sourceNode, targetNode := *arena.Port(sourceID).Node, *arena.Port(targetID).Node
	source := arena.Port(sourceID)
	target := arena.Port(targetID)
	sourcePosition := requireFloat(layout.Y[arena.Node(sourceNode).ID], "y") +
		requireFloat(layout.InnerShift[arena.Node(sourceNode).ID], "inner shift") +
		source.Position.Y + source.Anchor.Y
	targetPosition := requireFloat(layout.Y[arena.Node(targetNode).ID], "y") +
		requireFloat(layout.InnerShift[arena.Node(targetNode).ID], "inner shift") +
		target.Position.Y + target.Anchor.Y
	return targetPosition - sourcePosition
}

func (layout *BKAlignedLayout) ShiftBlock(
	arena *layered.LGraphArena,
	rootID layered.LNodeID,
	delta float64,
) {
	current := rootID
	for {
		index := arena.Node(current).ID
		value := requireFloat(layout.Y[index], "y") + delta
		layout.Y[index] = someFloat(value)
		current = requireNode(layout.Align[index], "align")
		if current == rootID {
			return
		}
	}
}

func (layout *BKAlignedLayout) CheckSpaceAbove(
	arena *layered.LGraphArena,
	blockRoot layered.LNodeID,
	delta float64,
	info *NeighborhoodInformation,
) float64 {
	available := delta
	current := blockRoot
	for {
		current = requireNode(layout.Align[arena.Node(current).ID], "align")
		minimum := layout.MinY(arena, current)
		if neighbor, found := layout.upperNeighbor(arena, current, info); found {
			available = math.Min(available, minimum-(layout.MaxY(arena, neighbor)+layered.VerticalSpacing(arena, current, neighbor)))
		}
		if current == blockRoot {
			return available
		}
	}
}

func (layout *BKAlignedLayout) CheckSpaceBelow(
	arena *layered.LGraphArena,
	blockRoot layered.LNodeID,
	delta float64,
	info *NeighborhoodInformation,
) float64 {
	available := delta
	current := blockRoot
	for {
		current = requireNode(layout.Align[arena.Node(current).ID], "align")
		maximum := layout.MaxY(arena, current)
		if neighbor, found := layout.lowerNeighbor(arena, current, info); found {
			available = math.Min(available, layout.MinY(arena, neighbor)-(maximum+layered.VerticalSpacing(arena, current, neighbor)))
		}
		if current == blockRoot {
			return available
		}
	}
}

func (layout *BKAlignedLayout) MinY(arena *layered.LGraphArena, nodeID layered.LNodeID) float64 {
	index := arena.Node(nodeID).ID
	rootID := requireNode(layout.Root[index], "root")
	return requireFloat(layout.Y[arena.Node(rootID).ID], "y") +
		requireFloat(layout.InnerShift[index], "inner shift") - arena.Node(nodeID).Margin.Top
}

func (layout *BKAlignedLayout) MaxY(arena *layered.LGraphArena, nodeID layered.LNodeID) float64 {
	index := arena.Node(nodeID).ID
	rootID := requireNode(layout.Root[index], "root")
	return requireFloat(layout.Y[arena.Node(rootID).ID], "y") +
		requireFloat(layout.InnerShift[index], "inner shift") + arena.Node(nodeID).Size.Y + arena.Node(nodeID).Margin.Bottom
}

func (layout *BKAlignedLayout) lowerNeighbor(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	info *NeighborhoodInformation,
) (layered.LNodeID, bool) {
	layerID := *arena.Node(nodeID).Layer
	index := info.NodeIndex[arena.Node(nodeID).ID]
	if index < len(arena.Layer(layerID).Nodes)-1 {
		return arena.Layer(layerID).Nodes[index+1], true
	}
	return 0, false
}

func (layout *BKAlignedLayout) upperNeighbor(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	info *NeighborhoodInformation,
) (layered.LNodeID, bool) {
	layerID := *arena.Node(nodeID).Layer
	index := info.NodeIndex[arena.Node(nodeID).ID]
	if index > 0 {
		return arena.Layer(layerID).Nodes[index-1], true
	}
	return 0, false
}

func (layout *BKAlignedLayout) String() string {
	result := ""
	switch layout.HDirection {
	case HorizontalRight:
		result = "RIGHT"
	case HorizontalLeft:
		result = "LEFT"
	}
	switch layout.VDirection {
	case VerticalDown:
		return result + "DOWN"
	case VerticalUp:
		return result + "UP"
	default:
		return result + "BALANCED"
	}
}

// optNode and optFloat are entries of Java's null-filled arrays; the zero
// value is null.
type optNode struct {
	value layered.LNodeID
	set   bool
}

type optFloat struct {
	value float64
	set   bool
}

func someNode(value layered.LNodeID) optNode { return optNode{value: value, set: true} }
func someFloat(value float64) optFloat       { return optFloat{value: value, set: true} }

func requireNode(value optNode, field string) layered.LNodeID {
	if !value.set {
		panic(fmt.Sprintf("p4nodes: uninitialized %s", field))
	}
	return value.value
}

func requireFloat(value optFloat, field string) float64 {
	if !value.set {
		panic(fmt.Sprintf("p4nodes: uninitialized %s", field))
	}
	return value.value
}
