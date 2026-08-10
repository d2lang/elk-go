// Copyright (c) 2016 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package wrapping

import (
	"math"
	"sort"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

var breakingPointInfo = basegraph.NewProperty[*BPInfo]("breakingPoint.info")

type BPInfo struct {
	Start, End                         layered.LNodeID
	NodeStartEdge, StartEndEdge        layered.LEdgeID
	OriginalEdge                       layered.LEdgeID
	StartInLayerDummy, EndInLayerDummy *layered.LNodeID
	StartInLayerEdge, EndInLayerEdge   *layered.LEdgeID
	Prev, Next                         *BPInfo
}

func IsBreakingPointStart(arena *layered.LGraphArena, nodeID layered.LNodeID) bool {
	info, found := basegraph.TryGet(&arena.Node(nodeID).Properties, breakingPointInfo)
	return found && info != nil && info.Start == nodeID
}

func IsBreakingPointEnd(arena *layered.LGraphArena, nodeID layered.LNodeID) bool {
	info, found := basegraph.TryGet(&arena.Node(nodeID).Properties, breakingPointInfo)
	return found && info != nil && info.End == nodeID
}

// InsertBreakingPoints computes multi-edge cuts after layering and introduces
// the paired marker layers used by the later wrapping processors.
func InsertBreakingPoints(arena *layered.LGraphArena, graphID layered.LGraphID) {
	graph := arena.Graph(graphID)
	stats := NewGraphStats(arena, graphID)
	var calculator cutIndexCalculator
	switch basegraph.Get(&graph.Properties, layered.WrappingCuttingStrategy) {
	case layered.CuttingManual:
		calculator = manualCutIndexCalculator{}
	case layered.CuttingARD:
		calculator = ardCutIndexHeuristic{}
	default:
		calculator = msdCutIndexHeuristic{}
	}
	cuts := calculator.cutIndexes(arena, graphID, stats)
	if basegraph.Get(&graph.Properties, layered.WrappingMultiEdgeImproveCuts) {
		cuts = improveCuts(arena, graphID, cuts)
	}
	if !calculator.guaranteesValid() && basegraph.Has(&graph.Properties, layered.WrappingValidifyStrategy) {
		switch basegraph.Get(&graph.Properties, layered.WrappingValidifyStrategy) {
		case layered.ValidifyLookBack:
			cuts = ValidifyIndexesLookingBack(stats, cuts)
		case layered.ValidifyGreedy:
			cuts = ValidifyIndexesGreedily(stats, cuts)
		}
	}
	if len(cuts) == 0 {
		return
	}
	applyCuts(arena, graphID, cuts)
}

func applyCuts(arena *layered.LGraphArena, graphID layered.LGraphID, cuts []int) int {
	graph := arena.Graph(graphID)
	position, originalIndex, cutIndex := 0, 0, 0
	cut := cuts[0]
	splitCount := 0
	alreadySplit := make(map[layered.LEdgeID]bool)
	open := make([]layered.LEdgeID, 0)
	openSet := make(map[layered.LEdgeID]bool)

	for position < len(graph.Layers) {
		layerID := graph.Layers[position]
		position++
		for _, nodeID := range arena.Layer(layerID).Nodes {
			for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
				if !openSet[edgeID] {
					openSet[edgeID] = true
					open = append(open, edgeID)
				}
			}
			for _, edgeID := range arena.NodeIncomingEdges(nodeID) {
				if openSet[edgeID] {
					openSet[edgeID] = false
				}
			}
		}

		if originalIndex+1 == cut {
			firstLayer := arena.AddLayer(graphID)
			graph.Layers = insertLayer(graph.Layers, position, firstLayer)
			position++
			secondLayer := arena.AddLayer(graphID)
			graph.Layers = insertLayer(graph.Layers, position, secondLayer)
			position++

			for _, originalEdge := range open {
				if !openSet[originalEdge] {
					continue
				}
				if !alreadySplit[originalEdge] {
					splitCount++
					alreadySplit[originalEdge] = true
				}

				start := arena.AddNode(graphID)
				arena.Node(start).Type = layered.BreakingPointNode
				basegraph.Set(&arena.Node(start).Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedSide)
				arena.SetNodeLayer(start, &firstLayer)
				startIn := arena.AddPort()
				arena.SetPortNode(startIn, &start)
				arena.SetPortSide(startIn, layered.WestSide)
				startOut := arena.AddPort()
				arena.SetPortNode(startOut, &start)
				arena.SetPortSide(startOut, layered.EastSide)

				end := arena.AddNode(graphID)
				arena.Node(end).Type = layered.BreakingPointNode
				basegraph.Set(&arena.Node(end).Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedSide)
				arena.SetNodeLayer(end, &secondLayer)
				endIn := arena.AddPort()
				arena.SetPortNode(endIn, &end)
				arena.SetPortSide(endIn, layered.WestSide)
				endOut := arena.AddPort()
				arena.SetPortNode(endOut, &end)
				arena.SetPortSide(endOut, layered.EastSide)

				nodeStartEdge := arena.AddEdge()
				oldSource := *arena.Edge(originalEdge).Source
				arena.SetEdgeSource(nodeStartEdge, &oldSource)
				arena.SetEdgeTarget(nodeStartEdge, &startIn)
				startEndEdge := arena.AddEdge()
				arena.SetEdgeSource(startEndEdge, &startOut)
				arena.SetEdgeTarget(startEndEdge, &endIn)
				if modelOrder, found := basegraph.TryGet(
					&arena.Edge(originalEdge).Properties,
					layered.ModelOrder,
				); found {
					basegraph.Set(&arena.Edge(nodeStartEdge).Properties, layered.ModelOrder, modelOrder)
					basegraph.Set(&arena.Edge(startEndEdge).Properties, layered.ModelOrder, modelOrder)
				}
				arena.SetEdgeSource(originalEdge, &endOut)

				info := &BPInfo{
					Start: start, End: end,
					NodeStartEdge: nodeStartEdge, StartEndEdge: startEndEdge,
					OriginalEdge: originalEdge,
				}
				basegraph.Set(&arena.Node(start).Properties, breakingPointInfo, info)
				basegraph.Set(&arena.Node(end).Properties, breakingPointInfo, info)
				previousNode := arena.EdgeSourceNode(nodeStartEdge)
				if arena.Node(previousNode).Type == layered.BreakingPointNode {
					previousInfo := basegraph.Get(&arena.Node(previousNode).Properties, breakingPointInfo)
					previousInfo.Next = info
					info.Prev = previousInfo
				}
			}

			cutIndex++
			if cutIndex < len(cuts) {
				cut = cuts[cutIndex]
			} else {
				break
			}
		}
		originalIndex++
	}
	return splitCount
}

func insertLayer(layers []layered.LayerID, index int, layerID layered.LayerID) []layered.LayerID {
	layers = append(layers, 0)
	copy(layers[index+1:], layers[index:])
	layers[index] = layerID
	return layers
}

type cut struct {
	index, newIndex int
	prev, successor *cut
	assigned        bool
}

func (candidate *cut) selfOrNext() *cut {
	if !candidate.assigned {
		return candidate
	}
	if candidate.successor != nil {
		return candidate.successor.selfOrNext()
	}
	return nil
}

func (candidate *cut) next() *cut {
	if candidate.successor != nil {
		return candidate.successor.selfOrNext()
	}
	return nil
}

func (candidate *cut) offset() {
	offset := candidate.newIndex - candidate.index
	candidate.offsetSelf(offset)
	candidate.offsetPrevious(offset)
	candidate.offsetSuccessor(offset)
}

func (candidate *cut) offsetSelf(offset int) { candidate.index += offset }

func (candidate *cut) offsetPrevious(offset int) {
	if candidate.prev != nil && !candidate.prev.assigned {
		candidate.prev.offsetSelf(offset)
		candidate.prev.offsetPrevious(offset)
	}
}

func (candidate *cut) offsetSuccessor(offset int) {
	if candidate.successor != nil && !candidate.successor.assigned {
		candidate.successor.offsetSelf(offset)
		candidate.successor.offsetSuccessor(offset)
	}
}

func improveCuts(arena *layered.LGraphArena, graphID layered.LGraphID, cuts []int) []int {
	improved := make([]int, 0, len(cuts))
	candidates := make([]*cut, 0, len(cuts))
	var last *cut
	for _, index := range cuts {
		candidate := &cut{index: index}
		candidates = append(candidates, candidate)
		if last != nil {
			candidate.prev = last
			last.successor = candidate
		}
		last = candidate
	}
	spans := computeEdgeSpans(arena, graphID)
	for range candidates {
		var left *cut
		right := candidates[0].selfOrNext()
		var best *cut
		bestScore := math.Inf(1)
		for index := 1; index < len(arena.Graph(graphID).Layers); index++ {
			var hit *cut
			var rightDistance int
			if right != nil {
				rightDistance = absInt(right.index - index)
			} else {
				rightDistance = absInt(index-left.index) + 1
			}
			leftDistance := rightDistance + 1
			if left != nil {
				leftDistance = absInt(index - left.index)
			}
			distance := rightDistance
			hit = right
			if leftDistance < rightDistance {
				distance = leftDistance
				hit = left
			}
			score := computeCutScore(arena, graphID, spans[index], distance)
			if score < bestScore {
				bestScore = score
				best = hit
				best.newIndex = index
			}
			if right != nil && index == right.index {
				left = right
				right = right.next()
			}
		}
		if best != nil {
			improved = append(improved, best.newIndex)
			best.assigned = true
			best.offset()
		}
	}
	sort.Ints(improved)
	return improved
}

func computeCutScore(arena *layered.LGraphArena, graphID layered.LGraphID, spans, distance int) float64 {
	penalty := basegraph.Get(&arena.Graph(graphID).Properties, layered.WrappingMultiEdgeDistancePenalty)
	return float64(spans) + math.Pow(float64(distance), penalty)
}

func computeEdgeSpans(arena *layered.LGraphArena, graphID layered.LGraphID) []int {
	spans := make([]int, len(arena.Graph(graphID).Layers)+1)
	open := make(map[layered.LEdgeID]bool)
	for index, layerID := range arena.Graph(graphID).Layers {
		spans[index] = len(open)
		for _, nodeID := range arena.Layer(layerID).Nodes {
			for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
				open[edgeID] = true
			}
		}
		for _, nodeID := range arena.Layer(layerID).Nodes {
			for _, edgeID := range arena.NodeIncomingEdges(nodeID) {
				delete(open, edgeID)
			}
		}
	}
	return spans
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
