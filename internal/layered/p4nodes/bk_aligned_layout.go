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
// Brandes-Koepf layouts. Pointer entries preserve Java's null-filled object
// arrays and make missing algorithm initialization fail loudly.
type BKAlignedLayout struct {
	Root       []*layered.LNodeID
	BlockSize  []*float64
	Align      []*layered.LNodeID
	InnerShift []*float64
	Sink       []*layered.LNodeID
	Shift      []*float64
	Y          []*float64
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
	odd := make([]bool, nodeCount)
	for index := range odd {
		odd[index] = true
	}
	return &BKAlignedLayout{
		Root: make([]*layered.LNodeID, nodeCount), BlockSize: make([]*float64, nodeCount),
		Align: make([]*layered.LNodeID, nodeCount), InnerShift: make([]*float64, nodeCount),
		Sink: make([]*layered.LNodeID, nodeCount), Shift: make([]*float64, nodeCount),
		Y: make([]*float64, nodeCount), SU: make([]bool, nodeCount), OD: odd,
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
		layout.Y[index] = floatPointer(value)
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

func nodePointer(value layered.LNodeID) *layered.LNodeID { return &value }
func floatPointer(value float64) *float64                { return &value }

func requireNode(value *layered.LNodeID, field string) layered.LNodeID {
	if value == nil {
		panic(fmt.Sprintf("p4nodes: uninitialized %s", field))
	}
	return *value
}

func requireFloat(value *float64, field string) float64 {
	if value == nil {
		panic(fmt.Sprintf("p4nodes: uninitialized %s", field))
	}
	return *value
}
