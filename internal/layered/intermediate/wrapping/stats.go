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

// GraphStats is the source GraphStats cache used by the graph-wrapping
// heuristics. ELK's estimates intentionally include one spacing quantum for
// every node and layer.
type GraphStats struct {
	arena   *layered.LGraphArena
	graphID layered.LGraphID

	spacing        float64
	inLayerSpacing float64

	DesiredAspectRatio float64
	LongestPath        int

	maxWidth  *float64
	maxHeight *float64
	sumWidth  *float64
	widths    []float64
	heights   []float64
	cuts      []bool
}

func NewGraphStats(arena *layered.LGraphArena, graphID layered.LGraphID) *GraphStats {
	graph := arena.Graph(graphID)
	aspectRatio := basegraph.Get(&graph.Properties, layered.AspectRatio)
	correction := basegraph.Get(&graph.Properties, layered.WrappingCorrectionFactor)
	desired := aspectRatio * correction
	direction := basegraph.Get(&graph.Properties, layered.DirectionOption)
	if direction == layered.DirectionUp || direction == layered.DirectionDown {
		desired = 1 / desired
	}
	return &GraphStats{
		arena:              arena,
		graphID:            graphID,
		spacing:            basegraph.Get(&graph.Properties, layered.SpacingNodeNodeBetweenLayers),
		inLayerSpacing:     basegraph.Get(&graph.Properties, layered.SpacingNodeNode),
		DesiredAspectRatio: desired,
		LongestPath:        len(graph.Layers),
	}
}

func (stats *GraphStats) ApproximateLayerWidth(layerID layered.LayerID) float64 {
	return stats.determineLayerWidth(layerID)
}

func (stats *GraphStats) ApproximateLayeringWidth() float64 { return stats.SumWidth() }

func (stats *GraphStats) ApproximateChunkBasedLayeringWidth(cuts []int) float64 {
	if len(cuts) == 0 {
		return stats.ApproximateLayeringWidth()
	}
	width := 0.0
	low := 0
	for _, high := range cuts {
		width = math.Max(width, stats.determineChunkWidth(low, high))
		low = high
	}
	return math.Max(width, stats.determineChunkWidth(low, len(stats.arena.Graph(stats.graphID).Layers)))
}

func (stats *GraphStats) ApproximateLayerHeight(layerID layered.LayerID) float64 {
	return stats.determineLayerHeight(layerID)
}

func (stats *GraphStats) ApproximateLayeringHeight() float64 { return stats.MaxHeight() }

func (stats *GraphStats) ApproximateChunkBasedLayeringHeight(cuts []int) float64 {
	if len(cuts) == 0 {
		return 0
	}
	height := 0.0
	low := 0
	for _, high := range cuts {
		height += stats.determineChunkHeight(low, high)
		low = high
	}
	return height + stats.determineChunkHeight(low, len(stats.arena.Graph(stats.graphID).Layers))
}

func (stats *GraphStats) MaxWidth() float64 {
	if stats.maxWidth == nil {
		value := math.Inf(-1)
		for _, layerID := range stats.arena.Graph(stats.graphID).Layers {
			value = math.Max(value, stats.determineLayerWidth(layerID))
		}
		stats.maxWidth = &value
	}
	return *stats.maxWidth
}

func (stats *GraphStats) SumWidth() float64 {
	if stats.sumWidth == nil {
		value := 0.0
		for _, layerID := range stats.arena.Graph(stats.graphID).Layers {
			value += stats.determineLayerWidth(layerID)
		}
		stats.sumWidth = &value
	}
	return *stats.sumWidth
}

func (stats *GraphStats) Widths() []float64 {
	stats.initWidthsAndHeights()
	return stats.widths
}

func (stats *GraphStats) Heights() []float64 {
	stats.initWidthsAndHeights()
	return stats.heights
}

func (stats *GraphStats) initWidthsAndHeights() {
	if stats.widths != nil {
		return
	}
	stats.widths = make([]float64, stats.LongestPath)
	stats.heights = make([]float64, stats.LongestPath)
	for index, layerID := range stats.arena.Graph(stats.graphID).Layers {
		stats.widths[index] = stats.determineLayerWidth(layerID)
		stats.heights[index] = stats.determineLayerHeight(layerID)
	}
}

func (stats *GraphStats) determineLayerWidth(layerID layered.LayerID) float64 {
	maximum := 0.0
	for _, nodeID := range stats.arena.Layer(layerID).Nodes {
		node := stats.arena.Node(nodeID)
		width := node.Size.X + node.Margin.Right + node.Margin.Left + stats.spacing
		maximum = math.Max(maximum, width)
	}
	return maximum
}

func (stats *GraphStats) determineChunkWidth(low, high int) float64 {
	width := 0.0
	for _, layerID := range stats.arena.Graph(stats.graphID).Layers[low:high] {
		width += stats.determineLayerWidth(layerID)
	}
	return width
}

func (stats *GraphStats) MaxHeight() float64 {
	if stats.maxHeight == nil {
		value := math.Inf(-1)
		for _, layerID := range stats.arena.Graph(stats.graphID).Layers {
			value = math.Max(value, stats.determineLayerHeight(layerID))
		}
		stats.maxHeight = &value
	}
	return *stats.maxHeight
}

func (stats *GraphStats) determineLayerHeight(layerID layered.LayerID) float64 {
	height := 0.0
	for _, nodeID := range stats.arena.Layer(layerID).Nodes {
		node := stats.arena.Node(nodeID)
		height += node.Size.Y + node.Margin.Bottom + node.Margin.Top + stats.inLayerSpacing
		for _, edgeID := range stats.arena.NodeIncomingEdges(nodeID) {
			source := stats.arena.EdgeSourceNode(edgeID)
			if stats.arena.Node(source).Type != layered.NorthSouthPortNode {
				continue
			}
			origin, found := basegraph.TryGet(&stats.arena.Node(source).Properties, layered.OriginalElement)
			if !found || origin.Kind != layered.OriginLNode {
				continue
			}
			original := stats.arena.Node(origin.LNode)
			height += original.Size.Y + original.Margin.Bottom + original.Margin.Top
		}
	}
	return height
}

func (stats *GraphStats) determineChunkHeight(low, high int) float64 {
	maximum := 0.0
	for _, layerID := range stats.arena.Graph(stats.graphID).Layers[low:high] {
		maximum = math.Max(maximum, stats.determineLayerHeight(layerID))
	}
	return maximum
}

func (stats *GraphStats) IsCutAllowed(layerIndex int) bool {
	stats.initCutsAllowed()
	return stats.cuts[layerIndex]
}

func (stats *GraphStats) CutsAllowed() []bool {
	stats.initCutsAllowed()
	return stats.cuts
}

func (stats *GraphStats) initCutsAllowed() {
	if stats.cuts != nil {
		return
	}
	stats.cuts = make([]bool, len(stats.arena.Graph(stats.graphID).Layers))
	if len(stats.cuts) == 0 {
		return
	}
	stats.cuts[0] = false
	graph := stats.arena.Graph(stats.graphID)
	if forbidden, found := basegraph.TryGet(&graph.Properties, layered.WrappingValidifyForbiddenIndices); found {
		// This preserves ELK 0.8.1 literally: the freshly allocated array is
		// never filled true in the explicit-forbidden-indices branch.
		for _, index := range forbidden {
			if index > 0 && index < len(stats.cuts) {
				stats.cuts[index] = false
			}
		}
		return
	}
	for index := 1; index < len(graph.Layers); index++ {
		stats.cuts[index] = stats.cutAllowedBefore(graph.Layers[index])
	}
}

func (stats *GraphStats) cutAllowedBefore(layerID layered.LayerID) bool {
	var firstTarget, firstSource *layered.LNodeID
	for _, targetID := range stats.arena.Layer(layerID).Nodes {
		for _, edgeID := range stats.arena.NodeIncomingEdges(targetID) {
			if firstTarget != nil && *firstTarget != targetID {
				return false
			}
			target := targetID
			firstTarget = &target
			sourceID := stats.arena.EdgeSourceNode(edgeID)
			if firstSource != nil && *firstSource != sourceID {
				return false
			}
			source := sourceID
			firstSource = &source
		}
	}
	return true
}
