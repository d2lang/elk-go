// Copyright (c) 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"math"

	"github.com/d2lang/elk-go/internal/layered"
)

const straighteningThreshold = math.MaxFloat64

type postprocessable struct {
	free     layered.LNodeID
	isRoot   bool
	hasEdges bool
	edge     *layered.LEdgeID
}

type ThresholdStrategy struct {
	simple           bool
	blockFinished    map[layered.LNodeID]bool
	postprocessQueue []postprocessable
	postprocessStack []postprocessable
}

func NewThresholdStrategy(simple bool) *ThresholdStrategy {
	return &ThresholdStrategy{simple: simple, blockFinished: make(map[layered.LNodeID]bool)}
}

func (strategy *ThresholdStrategy) Init() {
	clear(strategy.blockFinished)
	strategy.postprocessQueue = strategy.postprocessQueue[:0]
	strategy.postprocessStack = strategy.postprocessStack[:0]
}

func (strategy *ThresholdStrategy) FinishBlock(nodeID layered.LNodeID) {
	strategy.blockFinished[nodeID] = true
}

func (strategy *ThresholdStrategy) CalculateThreshold(
	arena *layered.LGraphArena,
	layout *BKAlignedLayout,
	oldThreshold float64,
	blockRoot, currentNode layered.LNodeID,
) float64 {
	if !strategy.simple {
		if layout.VDirection == VerticalUp {
			return math.Inf(1)
		}
		return math.Inf(-1)
	}
	isRoot := blockRoot == currentNode
	isLast := requireNode(layout.Align[arena.Node(currentNode).ID], "align") == blockRoot
	if !isRoot && !isLast {
		return oldThreshold
	}
	threshold := oldThreshold
	if isRoot {
		threshold = strategy.bound(arena, layout, blockRoot, true)
	}
	if math.IsInf(threshold, 0) && isLast {
		threshold = strategy.bound(arena, layout, currentNode, false)
	}
	return threshold
}

func (strategy *ThresholdStrategy) pickEdge(
	arena *layered.LGraphArena,
	layout *BKAlignedLayout,
	processable *postprocessable,
) {
	var edges []layered.LEdgeID
	if processable.isRoot {
		if layout.HDirection == HorizontalRight {
			edges = arena.NodeIncomingEdges(processable.free)
		} else {
			edges = arena.NodeOutgoingEdges(processable.free)
		}
	} else if layout.HDirection == HorizontalLeft {
		edges = arena.NodeIncomingEdges(processable.free)
	} else {
		edges = arena.NodeOutgoingEdges(processable.free)
	}
	hasEdges := false
	for _, edgeID := range edges {
		rootID := requireNode(layout.Root[arena.Node(processable.free).ID], "root")
		if !layout.OD[arena.Node(rootID).ID] && arena.EdgeIsInLayer(edgeID) {
			continue
		}
		if layout.SU[arena.Node(rootID).ID] {
			continue
		}
		hasEdges = true
		other := arena.EdgeOtherNode(edgeID, processable.free)
		otherRoot := requireNode(layout.Root[arena.Node(other).ID], "root")
		if strategy.blockFinished[otherRoot] {
			processable.hasEdges = true
			processable.edge = edgePointer(edgeID)
			return
		}
	}
	processable.hasEdges = hasEdges
	processable.edge = nil
}

func (strategy *ThresholdStrategy) bound(
	arena *layered.LGraphArena,
	layout *BKAlignedLayout,
	blockNode layered.LNodeID,
	isRoot bool,
) float64 {
	invalid := math.Inf(-1)
	if layout.VDirection == VerticalUp {
		invalid = math.Inf(1)
	}
	processable := postprocessable{free: blockNode, isRoot: isRoot}
	strategy.pickEdge(arena, layout, &processable)
	if processable.edge == nil {
		if processable.hasEdges {
			strategy.postprocessQueue = append(strategy.postprocessQueue, processable)
		}
		return invalid
	}
	edge := arena.Edge(*processable.edge)
	left, right := *edge.Source, *edge.Target
	rootPort, otherPort := left, right
	if isRoot {
		if layout.HDirection == HorizontalRight {
			rootPort, otherPort = right, left
		}
	} else if layout.HDirection == HorizontalLeft {
		rootPort, otherPort = right, left
	}
	otherNode := *arena.Port(otherPort).Node
	rootPortNode := *arena.Port(rootPort).Node
	otherRoot := requireNode(layout.Root[arena.Node(otherNode).ID], "root")
	threshold := requireFloat(layout.Y[arena.Node(otherRoot).ID], "y") +
		requireFloat(layout.InnerShift[arena.Node(otherNode).ID], "inner shift") +
		arena.Port(otherPort).Position.Y + arena.Port(otherPort).Anchor.Y -
		requireFloat(layout.InnerShift[arena.Node(rootPortNode).ID], "inner shift") -
		arena.Port(rootPort).Position.Y - arena.Port(rootPort).Anchor.Y
	leftRoot := requireNode(layout.Root[arena.Node(*arena.Port(left).Node).ID], "root")
	rightRoot := requireNode(layout.Root[arena.Node(*arena.Port(right).Node).ID], "root")
	layout.SU[arena.Node(leftRoot).ID] = true
	layout.SU[arena.Node(rightRoot).ID] = true
	return threshold
}

func (strategy *ThresholdStrategy) PostProcess(
	arena *layered.LGraphArena,
	info *NeighborhoodInformation,
	layout *BKAlignedLayout,
) {
	if !strategy.simple {
		return
	}
	for len(strategy.postprocessQueue) > 0 {
		processable := strategy.postprocessQueue[0]
		strategy.postprocessQueue = strategy.postprocessQueue[1:]
		strategy.pickEdge(arena, layout, &processable)
		if processable.edge == nil {
			continue
		}
		rootID := requireNode(layout.Root[arena.Node(processable.free).ID], "root")
		if !layout.OD[arena.Node(rootID).ID] && arena.EdgeIsInLayer(*processable.edge) {
			continue
		}
		if !processPostprocessable(arena, info, layout, processable) {
			strategy.postprocessStack = append(strategy.postprocessStack, processable)
		}
	}
	for len(strategy.postprocessStack) > 0 {
		last := len(strategy.postprocessStack) - 1
		processable := strategy.postprocessStack[last]
		strategy.postprocessStack = strategy.postprocessStack[:last]
		processPostprocessable(arena, info, layout, processable)
	}
}

func processPostprocessable(
	arena *layered.LGraphArena,
	info *NeighborhoodInformation,
	layout *BKAlignedLayout,
	processable postprocessable,
) bool {
	edgeID := *processable.edge
	edge := arena.Edge(edgeID)
	fix, block := *edge.Target, *edge.Source
	if arena.EdgeSourceNode(edgeID) != processable.free {
		fix, block = *edge.Source, *edge.Target
	}
	delta := layout.CalculateDelta(arena, fix, block)
	blockNode := *arena.Port(block).Node
	if delta > 0 && delta < straighteningThreshold {
		available := layout.CheckSpaceAbove(arena, blockNode, delta, info)
		layout.ShiftBlock(arena, blockNode, -available)
		return available > 0
	}
	if delta < 0 && -delta < straighteningThreshold {
		available := layout.CheckSpaceBelow(arena, blockNode, -delta, info)
		layout.ShiftBlock(arena, blockNode, available)
		return available > 0
	}
	return false
}

func edgePointer(value layered.LEdgeID) *layered.LEdgeID { return &value }
