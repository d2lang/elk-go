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

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// WrapSingleEdgeGraph is the direct SingleEdgeGraphWrapper port. It applies
// only after layering and crossing minimization, immediately before node
// placement.
func WrapSingleEdgeGraph(arena *layered.LGraphArena, graphID layered.LGraphID) {
	graph := arena.Graph(graphID)
	if len(graph.Layers) == 0 {
		return
	}
	stats := NewGraphStats(arena, graphID)
	sumWidth := stats.MaxWidth() * float64(stats.LongestPath)
	currentAspectRatio := sumWidth / stats.MaxWidth()
	if stats.DesiredAspectRatio > currentAspectRatio {
		return
	}

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
	if !calculator.guaranteesValid() {
		switch basegraph.Get(&graph.Properties, layered.WrappingValidifyStrategy) {
		case layered.ValidifyLookBack:
			cuts = ValidifyIndexesLookingBack(stats, cuts)
		case layered.ValidifyGreedy:
			cuts = ValidifyIndexesGreedily(stats, cuts)
		}
	}
	performCuts(arena, graphID, stats, cuts)
}

func ValidifyIndexesGreedily(stats *GraphStats, desired []int) []int {
	valid := make([]int, 0, len(desired))
	offset := 0
	for _, requested := range desired {
		cut := requested + offset
		for cut < stats.LongestPath && !stats.IsCutAllowed(cut) {
			cut++
			offset++
		}
		if cut >= stats.LongestPath {
			break
		}
		valid = append(valid, cut)
	}
	return valid
}

func ValidifyIndexesLookingBack(stats *GraphStats, desired []int) []int {
	if len(desired) == 0 {
		return nil
	}
	valid := []int{math.MinInt32}
	for index := 1; index < stats.LongestPath; index++ {
		if stats.IsCutAllowed(index) {
			valid = append(valid, index)
		}
	}
	if len(valid) == 1 {
		return nil
	}
	valid = append(valid, math.MaxInt32)
	return validifyIndexesLookingBack(desired, valid)
}

func validifyIndexesLookingBack(desired, valid []int) []int {
	finalCuts := make([]int, 0, len(desired))
	validIndex, cutIndex, offset := 0, 0, 0
	for validIndex < len(valid)-1 && cutIndex < len(desired) {
		current := desired[cutIndex] + offset
		for valid[validIndex+1] < current {
			validIndex++
		}
		selection := 0
		distanceLower := current - valid[validIndex]
		distanceHigher := valid[validIndex+1] - current
		if distanceLower > distanceHigher {
			selection++
		}
		selected := valid[validIndex+selection]
		finalCuts = append(finalCuts, selected)
		offset += selected - current
		cutIndex++
		for cutIndex < len(desired) && desired[cutIndex]+offset <= selected {
			cutIndex++
		}
		validIndex += 1 + selection
	}
	return finalCuts
}

func performCuts(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	stats *GraphStats,
	cuts []int,
) {
	if len(cuts) == 0 {
		return
	}
	graph := arena.Graph(graphID)
	index, newIndex, cutIndex := 0, 0, 0
	nextCut := cuts[0]
	for index < stats.LongestPath {
		if index == nextCut {
			newIndex = 0
			cutIndex++
			if cutIndex < len(cuts) {
				nextCut = cuts[cutIndex]
			} else {
				nextCut = stats.LongestPath + 1
			}
		}
		if index != newIndex {
			oldLayer := graph.Layers[index]
			newLayer := graph.Layers[newIndex]
			nodesToMove := append([]layered.LNodeID(nil), arena.Layer(oldLayer).Nodes...)
			for _, nodeID := range nodesToMove {
				arena.SetNodeLayerAt(nodeID, &newLayer, len(arena.Layer(newLayer).Nodes))
				if newIndex == 0 {
					incoming := append([]layered.LEdgeID(nil), arena.NodeIncomingEdges(nodeID)...)
					for _, edgeID := range incoming {
						arena.ReverseEdge(graphID, edgeID, true)
						basegraph.Set(&graph.Properties, layered.Cyclic, true)
						InsertDummies(arena, graphID, edgeID, 1)
					}
				}
			}
		}
		newIndex++
		index++
	}
	retained := graph.Layers[:0]
	for _, layerID := range graph.Layers {
		if len(arena.Layer(layerID).Nodes) > 0 {
			retained = append(retained, layerID)
		}
	}
	graph.Layers = retained
}
