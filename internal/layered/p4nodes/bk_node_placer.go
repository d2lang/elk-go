// Copyright (c) 2012, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"math"
	"sort"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// BrandesKoepfPlacer adapts the default node placer to the layered pipeline.
type BrandesKoepfPlacer struct{}

func (BrandesKoepfPlacer) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	PlaceBrandesKoepf(arena, graphID)
	return nil
}

func (BrandesKoepfPlacer) ProcessorConfiguration(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) layered.ProcessorConfiguration {
	var configuration layered.ProcessorConfiguration
	properties := basegraph.Get(&arena.Graph(graphID).Properties, layered.GraphProperties)
	if properties&layered.GraphPropertyExternalPorts != 0 {
		configuration.AddBefore(layered.PhaseEdgeRouting, layered.HierarchicalPortPositionProcessor)
	}
	return configuration
}

// PlaceBrandesKoepf runs ELK 0.8.1's extended Brandes-Koepf node placement.
func PlaceBrandesKoepf(arena *layered.LGraphArena, graphID layered.LGraphID) {
	info := BuildNeighborhoodInformation(arena, graphID)
	alignment := basegraph.Get(&arena.Graph(graphID).Properties, layered.BKFixedAlignment)
	favorStraightEdges := basegraph.Get(&arena.Graph(graphID).Properties, layered.FavorStraightEdges)
	produceBalanced := alignment == layered.FixedAlignmentNone && !favorStraightEdges ||
		alignment == layered.FixedAlignmentBalanced
	markedEdges := make(map[layered.LEdgeID]bool)
	markConflicts(arena, graphID, info, markedEdges)
	layouts := createDirectionalLayouts(graphID, info.NodeCount, alignment)
	aligner := NewBKAligner(arena, graphID, info)
	for _, layout := range layouts {
		aligner.VerticalAlignment(layout, markedEdges)
		aligner.InsideBlockShift(layout)
	}
	compactor := NewBKCompactor(arena, graphID)
	for _, layout := range layouts {
		compactor.HorizontalCompaction(arena, graphID, info, layout)
	}
	var chosen *BKAlignedLayout
	if produceBalanced {
		balanced := createBalancedLayout(arena, graphID, layouts, info.NodeCount)
		if checkOrderConstraint(arena, graphID, balanced) {
			chosen = balanced
		}
	}
	if chosen == nil {
		for _, layout := range layouts {
			if !checkOrderConstraint(arena, graphID, layout) {
				continue
			}
			if chosen == nil || chosen.LayoutSize(arena) > layout.LayoutSize(arena) {
				chosen = layout
			}
		}
	}
	if chosen == nil {
		chosen = layouts[0]
	}
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			index := arena.Node(nodeID).ID
			arena.Node(nodeID).Position.Y = requireFloat(chosen.Y[index], "y") +
				requireFloat(chosen.InnerShift[index], "inner shift")
		}
	}
	info.Cleanup()
}

func createDirectionalLayouts(
	graphID layered.LGraphID,
	nodeCount int,
	alignment layered.FixedAlignment,
) []*BKAlignedLayout {
	makeLayout := func(vertical VerticalDirection, horizontal HorizontalDirection) *BKAlignedLayout {
		return NewBKAlignedLayout(graphID, nodeCount, vertical, horizontal)
	}
	switch alignment {
	case layered.FixedAlignmentLeftDown:
		return []*BKAlignedLayout{makeLayout(VerticalDown, HorizontalLeft)}
	case layered.FixedAlignmentLeftUp:
		return []*BKAlignedLayout{makeLayout(VerticalUp, HorizontalLeft)}
	case layered.FixedAlignmentRightDown:
		return []*BKAlignedLayout{makeLayout(VerticalDown, HorizontalRight)}
	case layered.FixedAlignmentRightUp:
		return []*BKAlignedLayout{makeLayout(VerticalUp, HorizontalRight)}
	default:
		return []*BKAlignedLayout{
			makeLayout(VerticalDown, HorizontalRight),
			makeLayout(VerticalUp, HorizontalRight),
			makeLayout(VerticalDown, HorizontalLeft),
			makeLayout(VerticalUp, HorizontalLeft),
		}
	}
}

const minimumLayersForConflicts = 3

func markConflicts(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	info *NeighborhoodInformation,
	markedEdges map[layered.LEdgeID]bool,
) {
	layers := arena.Graph(graphID).Layers
	if len(layers) < minimumLayersForConflicts {
		return
	}
	layerSizes := make([]int, len(layers))
	for index, layerID := range layers {
		layerSizes[index] = len(arena.Layer(layerID).Nodes)
	}
	for layerIndex := 1; layerIndex < len(layers)-1; layerIndex++ {
		currentNodes := arena.Layer(layers[layerIndex+1]).Nodes
		k0, lowerStart := 0, 0
		for lowerEnd := 0; lowerEnd < layerSizes[layerIndex+1]; lowerEnd++ {
			nodeID := currentNodes[lowerEnd]
			inner := incidentToInnerSegment(arena, info, nodeID, layerIndex+1, layerIndex)
			if lowerEnd != layerSizes[layerIndex+1]-1 && !inner {
				continue
			}
			k1 := layerSizes[layerIndex] - 1
			if inner {
				neighbors := info.LeftNeighbors[arena.Node(nodeID).ID]
				k1 = info.NodeIndex[arena.Node(neighbors[0].Node).ID]
			}
			for lowerStart <= lowerEnd {
				current := currentNodes[lowerStart]
				if !incidentToInnerSegment(arena, info, current, layerIndex+1, layerIndex) {
					for _, neighbor := range info.LeftNeighbors[arena.Node(current).ID] {
						position := info.NodeIndex[arena.Node(neighbor.Node).ID]
						if position < k0 || position > k1 {
							markedEdges[neighbor.Edge] = true
						}
					}
				}
				lowerStart++
			}
			k0 = k1
		}
	}
}

func incidentToInnerSegment(
	arena *layered.LGraphArena,
	info *NeighborhoodInformation,
	nodeID layered.LNodeID,
	layer1, layer2 int,
) bool {
	if arena.Node(nodeID).Type != layered.LongEdgeNode {
		return false
	}
	for _, edgeID := range arena.NodeIncomingEdges(nodeID) {
		source := arena.EdgeSourceNode(edgeID)
		if arena.Node(source).Type == layered.LongEdgeNode &&
			info.LayerIndex[arena.Layer(*arena.Node(source).Layer).ID] == layer2 &&
			info.LayerIndex[arena.Layer(*arena.Node(nodeID).Layer).ID] == layer1 {
			return true
		}
	}
	return false
}

func createBalancedLayout(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	layouts []*BKAlignedLayout,
	nodeCount int,
) *BKAlignedLayout {
	balanced := NewBKAlignedLayout(graphID, nodeCount, VerticalBalanced, HorizontalBalanced)
	width := make([]float64, len(layouts))
	minimum := make([]float64, len(layouts))
	maximum := make([]float64, len(layouts))
	for index := range minimum {
		minimum[index] = float64(math.MaxInt32) // Java initializes min to Integer.MAX_VALUE.
		maximum[index] = float64(math.MinInt32)
	}
	minimumWidthLayout := 0
	for layoutIndex, layout := range layouts {
		width[layoutIndex] = layout.LayoutSize(arena)
		if width[minimumWidthLayout] > width[layoutIndex] {
			minimumWidthLayout = layoutIndex
		}
		for _, layerID := range arena.Graph(graphID).Layers {
			for _, nodeID := range arena.Layer(layerID).Nodes {
				index := arena.Node(nodeID).ID
				position := requireFloat(layout.Y[index], "y") + requireFloat(layout.InnerShift[index], "inner shift")
				minimum[layoutIndex] = math.Min(minimum[layoutIndex], position)
				maximum[layoutIndex] = math.Max(maximum[layoutIndex], position+arena.Node(nodeID).Size.Y)
			}
		}
	}
	shift := make([]float64, len(layouts))
	for index, layout := range layouts {
		if layout.VDirection == VerticalDown {
			shift[index] = minimum[minimumWidthLayout] - minimum[index]
		} else {
			shift[index] = maximum[minimumWidthLayout] - maximum[index]
		}
	}
	calculated := make([]float64, len(layouts))
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			index := arena.Node(nodeID).ID
			for layoutIndex, layout := range layouts {
				calculated[layoutIndex] = requireFloat(layout.Y[index], "y") +
					requireFloat(layout.InnerShift[index], "inner shift") + shift[layoutIndex]
			}
			sort.Float64s(calculated)
			balanced.Y[index] = floatPointer((calculated[1] + calculated[2]) / 2)
			balanced.InnerShift[index] = floatPointer(0)
		}
	}
	return balanced
}

func checkOrderConstraint(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	layout *BKAlignedLayout,
) bool {
	for _, layerID := range arena.Graph(graphID).Layers {
		position := math.Inf(-1)
		for _, nodeID := range arena.Layer(layerID).Nodes {
			index := arena.Node(nodeID).ID
			top := requireFloat(layout.Y[index], "y") + requireFloat(layout.InnerShift[index], "inner shift") - arena.Node(nodeID).Margin.Top
			bottom := requireFloat(layout.Y[index], "y") + requireFloat(layout.InnerShift[index], "inner shift") +
				arena.Node(nodeID).Size.Y + arena.Node(nodeID).Margin.Bottom
			if top <= position || bottom <= position {
				return false
			}
			position = bottom
		}
	}
	return true
}
