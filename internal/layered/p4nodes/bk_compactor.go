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

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

type classNode struct {
	classShift optFloat
	node       layered.LNodeID
	outgoing   []classEdge
	indegree   int
}

type classEdge struct {
	separation float64
	target     int
}

type BKCompactor struct {
	threshold *ThresholdStrategy
	// sinkClass holds, per node scratch ID, the index of the class whose sink
	// is that node plus one (zero: no class yet); sinkOrder lists the sinks in
	// creation order.
	sinkClass   []int32
	sinkScratch []int32
	sinkOrder   []layered.LNodeID
	classNodes  []classNode
	spacings    layered.SpacingCache
	queue       []int
}

func NewBKCompactor(arena *layered.LGraphArena, graphID layered.LGraphID) *BKCompactor {
	straightening := basegraph.Get(&arena.Graph(graphID).Properties, layered.BKEdgeStraightening)
	return &BKCompactor{
		threshold: NewThresholdStrategy(straightening == layered.EdgeStraighteningImprove),
	}
}

// reset puts a compactor created by acquireBKWorkspace in NewBKCompactor's
// state for graphID, keeping its storage. (HorizontalCompaction clears the
// per-layout state itself.)
func (compactor *BKCompactor) reset(arena *layered.LGraphArena, graphID layered.LGraphID) {
	straightening := basegraph.Get(&arena.Graph(graphID).Properties, layered.BKEdgeStraightening)
	compactor.threshold.reset(straightening == layered.EdgeStraighteningImprove)
	compactor.clearClasses()
	compactor.spacings.Reset(arena, graphID)
	for index := range compactor.classNodes {
		compactor.classNodes[index] = classNode{}
	}
	compactor.classNodes = compactor.classNodes[:0]
}

func (compactor *BKCompactor) HorizontalCompaction(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	info *NeighborhoodInformation,
	layout *BKAlignedLayout,
) {
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			index := arena.Node(nodeID).ID
			layout.Sink[index] = someNode(nodeID)
			shift := math.Inf(1)
			if layout.VDirection == VerticalUp {
				shift = math.Inf(-1)
			}
			layout.Shift[index] = someFloat(shift)
		}
	}
	compactor.clearClasses()
	compactor.classNodes = compactor.classNodes[:0]
	if !compactor.spacings.BoundTo(arena, graphID) {
		compactor.spacings.Reset(arena, graphID)
	}
	// ELK iterates reversed copies of the layer list and of each layer;
	// placement changes neither, so they are walked backwards in place.
	layers := arena.Graph(graphID).Layers
	leftward := layout.HDirection == HorizontalLeft
	compactor.threshold.initFor(len(layout.Y))
	for index := range layout.Y {
		layout.Y[index] = optFloat{}
	}
	for layerStep := range layers {
		layerID := layers[layerStep]
		if leftward {
			layerID = layers[len(layers)-1-layerStep]
		}
		nodes := arena.Layer(layerID).Nodes
		for nodeStep := range nodes {
			nodeID := nodes[nodeStep]
			if layout.VDirection == VerticalUp {
				nodeID = nodes[len(nodes)-1-nodeStep]
			}
			if requireNode(layout.Root[arena.Node(nodeID).ID], "root") == nodeID {
				compactor.placeBlock(arena, graphID, info, layout, nodeID)
			}
		}
	}
	compactor.placeClasses(arena, layout)
	for layerStep := range layers {
		layerID := layers[layerStep]
		if leftward {
			layerID = layers[len(layers)-1-layerStep]
		}
		for _, nodeID := range arena.Layer(layerID).Nodes {
			index := arena.Node(nodeID).ID
			rootID := requireNode(layout.Root[index], "root")
			layout.Y[index] = someFloat(requireFloat(layout.Y[arena.Node(rootID).ID], "y"))
			if rootID == nodeID {
				sinkID := requireNode(layout.Sink[index], "sink")
				sinkShift := requireFloat(layout.Shift[arena.Node(sinkID).ID], "shift")
				if layout.VDirection == VerticalUp && !math.IsInf(sinkShift, -1) ||
					layout.VDirection == VerticalDown && !math.IsInf(sinkShift, 1) {
					layout.Y[index] = someFloat(requireFloat(layout.Y[index], "y") + sinkShift)
				}
			}
		}
	}
	compactor.threshold.PostProcess(arena, info, layout)
}

func (compactor *BKCompactor) placeBlock(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	info *NeighborhoodInformation,
	layout *BKAlignedLayout,
	rootID layered.LNodeID,
) {
	rootIndex := arena.Node(rootID).ID
	if layout.Y[rootIndex].set {
		return
	}
	initialAssignment := true
	layout.Y[rootIndex] = someFloat(0)
	current := rootID
	threshold := math.Inf(1)
	if layout.VDirection == VerticalDown {
		threshold = math.Inf(-1)
	}
	for {
		currentPosition := info.NodeIndex[arena.Node(current).ID]
		layerID := *arena.Node(current).Layer
		layerSize := len(arena.Layer(layerID).Nodes)
		hasNeighbor := layout.VDirection == VerticalDown && currentPosition > 0 ||
			layout.VDirection == VerticalUp && currentPosition < layerSize-1
		if hasNeighbor {
			neighborPosition := currentPosition - 1
			if layout.VDirection == VerticalUp {
				neighborPosition = currentPosition + 1
			}
			neighbor := arena.Layer(layerID).Nodes[neighborPosition]
			neighborRoot := requireNode(layout.Root[arena.Node(neighbor).ID], "root")
			compactor.placeBlock(arena, graphID, info, layout, neighborRoot)
			threshold = compactor.threshold.CalculateThreshold(arena, layout, threshold, rootID, current)
			if requireNode(layout.Sink[rootIndex], "sink") == rootID {
				layout.Sink[rootIndex] = someNode(requireNode(layout.Sink[arena.Node(neighborRoot).ID], "sink"))
			}
			rootSink := requireNode(layout.Sink[rootIndex], "sink")
			neighborSink := requireNode(layout.Sink[arena.Node(neighborRoot).ID], "sink")
			if rootSink == neighborSink {
				spacing := compactor.spacings.Vertical(current, neighbor)
				currentPositionValue := requireFloat(layout.Y[rootIndex], "y")
				var newPosition float64
				if layout.VDirection == VerticalUp {
					newPosition = requireFloat(layout.Y[arena.Node(neighborRoot).ID], "y") +
						requireFloat(layout.InnerShift[arena.Node(neighbor).ID], "inner shift") -
						arena.Node(neighbor).Margin.Top - spacing - arena.Node(current).Margin.Bottom -
						arena.Node(current).Size.Y - requireFloat(layout.InnerShift[arena.Node(current).ID], "inner shift")
					value := math.Min(newPosition, threshold)
					if !initialAssignment {
						value = math.Min(currentPositionValue, value)
					}
					layout.Y[rootIndex] = someFloat(value)
				} else {
					newPosition = requireFloat(layout.Y[arena.Node(neighborRoot).ID], "y") +
						requireFloat(layout.InnerShift[arena.Node(neighbor).ID], "inner shift") + arena.Node(neighbor).Size.Y +
						arena.Node(neighbor).Margin.Bottom + spacing + arena.Node(current).Margin.Top -
						requireFloat(layout.InnerShift[arena.Node(current).ID], "inner shift")
					value := math.Max(newPosition, threshold)
					if !initialAssignment {
						value = math.Max(currentPositionValue, value)
					}
					layout.Y[rootIndex] = someFloat(value)
				}
				initialAssignment = false
			} else {
				spacing := basegraph.Get(&arena.Graph(graphID).Properties, layered.SpacingNodeNode)
				sinkNode := compactor.classNode(arena, rootSink)
				neighborClass := compactor.classNode(arena, neighborSink)
				var requiredSpace float64
				if layout.VDirection == VerticalUp {
					requiredSpace = requireFloat(layout.Y[rootIndex], "y") + requireFloat(layout.InnerShift[arena.Node(current).ID], "inner shift") +
						arena.Node(current).Size.Y + arena.Node(current).Margin.Bottom + spacing -
						(requireFloat(layout.Y[arena.Node(neighborRoot).ID], "y") + requireFloat(layout.InnerShift[arena.Node(neighbor).ID], "inner shift") - arena.Node(neighbor).Margin.Top)
				} else {
					requiredSpace = requireFloat(layout.Y[rootIndex], "y") + requireFloat(layout.InnerShift[arena.Node(current).ID], "inner shift") -
						arena.Node(current).Margin.Top - requireFloat(layout.Y[arena.Node(neighborRoot).ID], "y") -
						requireFloat(layout.InnerShift[arena.Node(neighbor).ID], "inner shift") - arena.Node(neighbor).Size.Y -
						arena.Node(neighbor).Margin.Bottom - spacing
				}
				compactor.addClassEdge(sinkNode, neighborClass, requiredSpace)
			}
		} else {
			threshold = compactor.threshold.CalculateThreshold(arena, layout, threshold, rootID, current)
		}
		current = requireNode(layout.Align[arena.Node(current).ID], "align")
		if current == rootID {
			break
		}
	}
	compactor.threshold.finishBlock(arena, rootID)
}

func (compactor *BKCompactor) placeClasses(arena *layered.LGraphArena, layout *BKAlignedLayout) {
	// A FIFO walked by head index (the same order as popping the front), in
	// a buffer kept for the next call.
	queue := compactor.queue[:0]
	for _, sinkID := range compactor.sinkOrder {
		index := compactor.sinkIndex(arena, sinkID)
		if compactor.classNodes[index].indegree == 0 {
			queue = append(queue, index)
		}
	}
	for head := 0; head < len(queue); head++ {
		index := queue[head]
		if !compactor.classNodes[index].classShift.set {
			compactor.classNodes[index].classShift = someFloat(0)
		}
		shift := compactor.classNodes[index].classShift.value
		for _, edge := range compactor.classNodes[index].outgoing {
			target := &compactor.classNodes[edge.target]
			candidate := shift + edge.separation
			if !target.classShift.set {
				target.classShift = someFloat(candidate)
			} else if layout.VDirection == VerticalDown {
				target.classShift = someFloat(math.Min(target.classShift.value, candidate))
			} else {
				target.classShift = someFloat(math.Max(target.classShift.value, candidate))
			}
			target.indegree--
			if target.indegree == 0 {
				queue = append(queue, edge.target)
			}
		}
	}
	compactor.queue = queue[:0]
	for _, sinkID := range compactor.sinkOrder {
		class := compactor.classNodes[compactor.sinkIndex(arena, sinkID)]
		layout.Shift[arena.Node(class.node).ID] = class.classShift
	}
}

// sinkIndex is the class index of a sink that has a class.
func (compactor *BKCompactor) sinkIndex(arena *layered.LGraphArena, sinkID layered.LNodeID) int {
	return int(compactor.sinkClass[arena.Node(sinkID).ID]) - 1
}

// clearClasses forgets every sink's class.
func (compactor *BKCompactor) clearClasses() {
	for _, scratch := range compactor.sinkScratch {
		compactor.sinkClass[scratch] = 0
	}
	compactor.sinkScratch = compactor.sinkScratch[:0]
	compactor.sinkOrder = compactor.sinkOrder[:0]
}

func (compactor *BKCompactor) classNode(arena *layered.LGraphArena, sinkID layered.LNodeID) int {
	scratch := arena.Node(sinkID).ID
	for scratch >= len(compactor.sinkClass) {
		compactor.sinkClass = append(compactor.sinkClass, 0)
	}
	if index := compactor.sinkClass[scratch]; index != 0 {
		return int(index) - 1
	}
	index := len(compactor.classNodes)
	compactor.classNodes = append(compactor.classNodes, classNode{node: sinkID})
	compactor.sinkClass[scratch] = int32(index + 1)
	compactor.sinkScratch = append(compactor.sinkScratch, int32(scratch))
	compactor.sinkOrder = append(compactor.sinkOrder, sinkID)
	return index
}

func (compactor *BKCompactor) addClassEdge(source, target int, separation float64) {
	compactor.classNodes[target].indegree++
	compactor.classNodes[source].outgoing = append(compactor.classNodes[source].outgoing, classEdge{separation, target})
}
