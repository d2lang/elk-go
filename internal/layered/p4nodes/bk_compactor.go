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
	classShift *float64
	node       layered.LNodeID
	outgoing   []classEdge
	indegree   int
}

type classEdge struct {
	separation float64
	target     int
}

type BKCompactor struct {
	threshold  *ThresholdStrategy
	sinkNodes  map[layered.LNodeID]int
	sinkOrder  []layered.LNodeID
	classNodes []classNode
}

func NewBKCompactor(arena *layered.LGraphArena, graphID layered.LGraphID) *BKCompactor {
	straightening := basegraph.Get(&arena.Graph(graphID).Properties, layered.BKEdgeStraightening)
	return &BKCompactor{
		threshold: NewThresholdStrategy(straightening == layered.EdgeStraighteningImprove),
		sinkNodes: make(map[layered.LNodeID]int),
	}
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
			layout.Sink[index] = nodePointer(nodeID)
			shift := math.Inf(1)
			if layout.VDirection == VerticalUp {
				shift = math.Inf(-1)
			}
			layout.Shift[index] = floatPointer(shift)
		}
	}
	clear(compactor.sinkNodes)
	compactor.sinkOrder = compactor.sinkOrder[:0]
	compactor.classNodes = compactor.classNodes[:0]
	layers := append([]layered.LayerID(nil), arena.Graph(graphID).Layers...)
	if layout.HDirection == HorizontalLeft {
		reverseLayers(layers)
	}
	compactor.threshold.Init()
	for index := range layout.Y {
		layout.Y[index] = nil
	}
	for _, layerID := range layers {
		nodes := append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...)
		if layout.VDirection == VerticalUp {
			reverseNodes(nodes)
		}
		for _, nodeID := range nodes {
			if requireNode(layout.Root[arena.Node(nodeID).ID], "root") == nodeID {
				compactor.placeBlock(arena, graphID, info, layout, nodeID)
			}
		}
	}
	compactor.placeClasses(arena, layout)
	for _, layerID := range layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			index := arena.Node(nodeID).ID
			rootID := requireNode(layout.Root[index], "root")
			layout.Y[index] = floatPointer(requireFloat(layout.Y[arena.Node(rootID).ID], "y"))
			if rootID == nodeID {
				sinkID := requireNode(layout.Sink[index], "sink")
				sinkShift := requireFloat(layout.Shift[arena.Node(sinkID).ID], "shift")
				if layout.VDirection == VerticalUp && !math.IsInf(sinkShift, -1) ||
					layout.VDirection == VerticalDown && !math.IsInf(sinkShift, 1) {
					layout.Y[index] = floatPointer(requireFloat(layout.Y[index], "y") + sinkShift)
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
	if layout.Y[rootIndex] != nil {
		return
	}
	initialAssignment := true
	layout.Y[rootIndex] = floatPointer(0)
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
				layout.Sink[rootIndex] = nodePointer(requireNode(layout.Sink[arena.Node(neighborRoot).ID], "sink"))
			}
			rootSink := requireNode(layout.Sink[rootIndex], "sink")
			neighborSink := requireNode(layout.Sink[arena.Node(neighborRoot).ID], "sink")
			if rootSink == neighborSink {
				spacing := layered.VerticalSpacing(arena, current, neighbor)
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
					layout.Y[rootIndex] = floatPointer(value)
				} else {
					newPosition = requireFloat(layout.Y[arena.Node(neighborRoot).ID], "y") +
						requireFloat(layout.InnerShift[arena.Node(neighbor).ID], "inner shift") + arena.Node(neighbor).Size.Y +
						arena.Node(neighbor).Margin.Bottom + spacing + arena.Node(current).Margin.Top -
						requireFloat(layout.InnerShift[arena.Node(current).ID], "inner shift")
					value := math.Max(newPosition, threshold)
					if !initialAssignment {
						value = math.Max(currentPositionValue, value)
					}
					layout.Y[rootIndex] = floatPointer(value)
				}
				initialAssignment = false
			} else {
				spacing := basegraph.Get(&arena.Graph(graphID).Properties, layered.SpacingNodeNode)
				sinkNode := compactor.classNode(rootSink)
				neighborClass := compactor.classNode(neighborSink)
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
	compactor.threshold.FinishBlock(rootID)
}

func (compactor *BKCompactor) placeClasses(arena *layered.LGraphArena, layout *BKAlignedLayout) {
	queue := make([]int, 0)
	for _, sinkID := range compactor.sinkOrder {
		index := compactor.sinkNodes[sinkID]
		if compactor.classNodes[index].indegree == 0 {
			queue = append(queue, index)
		}
	}
	for len(queue) > 0 {
		index := queue[0]
		queue = queue[1:]
		if compactor.classNodes[index].classShift == nil {
			compactor.classNodes[index].classShift = floatPointer(0)
		}
		shift := *compactor.classNodes[index].classShift
		for _, edge := range compactor.classNodes[index].outgoing {
			target := &compactor.classNodes[edge.target]
			candidate := shift + edge.separation
			if target.classShift == nil {
				target.classShift = floatPointer(candidate)
			} else if layout.VDirection == VerticalDown {
				target.classShift = floatPointer(math.Min(*target.classShift, candidate))
			} else {
				target.classShift = floatPointer(math.Max(*target.classShift, candidate))
			}
			target.indegree--
			if target.indegree == 0 {
				queue = append(queue, edge.target)
			}
		}
	}
	for _, sinkID := range compactor.sinkOrder {
		class := compactor.classNodes[compactor.sinkNodes[sinkID]]
		layout.Shift[arena.Node(class.node).ID] = class.classShift
	}
}

func (compactor *BKCompactor) classNode(sinkID layered.LNodeID) int {
	if index, found := compactor.sinkNodes[sinkID]; found {
		return index
	}
	index := len(compactor.classNodes)
	compactor.classNodes = append(compactor.classNodes, classNode{node: sinkID})
	compactor.sinkNodes[sinkID] = index
	compactor.sinkOrder = append(compactor.sinkOrder, sinkID)
	return index
}

func (compactor *BKCompactor) addClassEdge(source, target int, separation float64) {
	compactor.classNodes[target].indegree++
	compactor.classNodes[source].outgoing = append(compactor.classNodes[source].outgoing, classEdge{separation, target})
}
