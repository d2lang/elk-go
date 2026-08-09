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

type cutIndexCalculator interface {
	cutIndexes(*layered.LGraphArena, layered.LGraphID, *GraphStats) []int
	guaranteesValid() bool
}

type manualCutIndexCalculator struct{}

func (manualCutIndexCalculator) cutIndexes(arena *layered.LGraphArena, graphID layered.LGraphID, _ *GraphStats) []int {
	cuts, found := basegraph.TryGet(&arena.Graph(graphID).Properties, layered.WrappingCuttingCuts)
	if !found {
		return nil
	}
	return append([]int(nil), cuts...)
}

func (manualCutIndexCalculator) guaranteesValid() bool { return false }

type ardCutIndexHeuristic struct{}

func (ardCutIndexHeuristic) cutIndexes(_ *layered.LGraphArena, _ layered.LGraphID, stats *GraphStats) []int {
	rows := chunkCount(stats)
	capacity := rows - 1
	if capacity < 0 {
		capacity = 0
	}
	cuts := make([]int, 0, capacity)
	step := float64(stats.LongestPath) / float64(rows)
	for index := 1; index < rows; index++ {
		cuts = append(cuts, int(math.Round(float64(index)*step)))
	}
	return cuts
}

func (ardCutIndexHeuristic) guaranteesValid() bool { return false }

func chunkCount(stats *GraphStats) int {
	rows := int(math.Round(math.Sqrt(stats.SumWidth() / (stats.DesiredAspectRatio * stats.MaxHeight()))))
	if rows > stats.LongestPath {
		rows = stats.LongestPath
	}
	return rows
}

type msdCutIndexHeuristic struct{}

func (msdCutIndexHeuristic) guaranteesValid() bool { return false }

func (msdCutIndexHeuristic) cutIndexes(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	stats *GraphStats,
) []int {
	widths := stats.Widths()
	heights := stats.Heights()
	widthAtIndex := make([]float64, len(widths))
	widthAtIndex[0] = widths[0]
	total := widths[0]
	for index := 1; index < len(widths); index++ {
		widthAtIndex[index] = widthAtIndex[index-1] + widths[index]
		total += widths[index]
	}

	cutCount := chunkCount(stats) - 1
	freedom := basegraph.Get(&arena.Graph(graphID).Properties, layered.WrappingCuttingMSDFreedom)
	bestMaxScale := math.Inf(-1)
	var bestCuts []int
	minimumCuts := cutCount - freedom
	if minimumCuts < 0 {
		minimumCuts = 0
	}
	maximumCuts := cutCount + freedom
	if maximumCuts > stats.LongestPath-1 {
		maximumCuts = stats.LongestPath - 1
	}
	for count := minimumCuts; count <= maximumCuts; count++ {
		rowSum := total / float64(count+1)
		sumSoFar := 0.0
		index := 1
		cuts := make([]int, 0, count)
		width := math.Inf(-1)
		lastCutWidth := 0.0
		height := 0.0
		rowHeightMaximum := heights[0]

		if count == 0 {
			width = total
			height = stats.MaxHeight()
		} else {
			for index < stats.LongestPath {
				if widthAtIndex[index-1]-sumSoFar >= rowSum {
					cuts = append(cuts, index)
					width = math.Max(width, widthAtIndex[index-1]-lastCutWidth)
					height += rowHeightMaximum
					sumSoFar += widthAtIndex[index-1] - sumSoFar
					lastCutWidth = widthAtIndex[index-1]
					rowHeightMaximum = heights[index]
				}
				rowHeightMaximum = math.Max(rowHeightMaximum, heights[index])
				index++
			}
			height += rowHeightMaximum
		}

		maxScale := math.Min(1/width, (1/stats.DesiredAspectRatio)/height)
		if maxScale > bestMaxScale {
			bestMaxScale = maxScale
			bestCuts = cuts
		}
	}
	return bestCuts
}
