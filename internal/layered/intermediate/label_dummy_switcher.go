// Copyright (c) 2012, 2017 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"math"
	"sort"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

var includeLabel = basegraph.PropertyWithDefault("edgelabelcenterednessanalysis.includelabel", func() bool { return false })

type labelDummyInfo struct {
	labelDummy           layered.LNodeID
	placement            layered.CenterEdgeLabelPlacementStrategy
	leftLongEdgeDummies  []layered.LNodeID
	rightLongEdgeDummies []layered.LNodeID
	leftmostLayerIndex   int
	rightmostLayerIndex  int
}

func (info *labelDummyInfo) totalDummyCount() int {
	return info.rightmostLayerIndex - info.leftmostLayerIndex + 1
}

func (info *labelDummyInfo) dummyAt(index int) layered.LNodeID {
	if index < len(info.leftLongEdgeDummies) {
		return info.leftLongEdgeDummies[index]
	}
	if index == len(info.leftLongEdgeDummies) {
		return info.labelDummy
	}
	return info.rightLongEdgeDummies[index-len(info.leftLongEdgeDummies)-1]
}

// SwitchLabelDummies moves center-label dummies to the layer selected by the
// configured ELK strategy while preserving each layer's crossing-minimized
// node position.
func SwitchLabelDummies(arena *layered.LGraphArena, graphID layered.LGraphID) {
	graph := arena.Graph(graphID)
	for index, layerID := range graph.Layers {
		arena.Layer(layerID).ID = index
	}
	defaultStrategy := basegraph.Get(&graph.Properties, layered.CenterEdgeLabelPlacement)
	groups := make([][]*labelDummyInfo, int(layered.CenterLabelCenterLayer)+1)
	for _, layerID := range graph.Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			if arena.Node(nodeID).Type != layered.LabelNode {
				continue
			}
			info := gatherLabelDummyInfo(arena, nodeID, defaultStrategy)
			groups[int(info.placement)] = append(groups[int(info.placement)], info)
		}
	}

	layerWidths := make([]float64, len(graph.Layers))
	if hasWidthBasedLabelStrategy(groups) {
		for index, layerID := range graph.Layers {
			if directionIsVertical(basegraph.Get(&graph.Properties, layered.DirectionOption)) {
				continue
			}
			for _, nodeID := range arena.Layer(layerID).Nodes {
				if arena.Node(nodeID).Type == layered.NormalNode {
					layerWidths[index] = math.Max(layerWidths[index], arena.Node(nodeID).Size.X)
				}
			}
		}
	}

	// ELK iterates enum values twice: non-width strategies first, then width
	// strategies. Declaration order is observable.
	for strategy := layered.CenterLabelMedianLayer; strategy <= layered.CenterLabelCenterLayer; strategy++ {
		if !labelStrategyUsesWidth(strategy) {
			processLabelDummyStrategy(arena, graphID, groups[int(strategy)], strategy, layerWidths)
		}
	}
	for strategy := layered.CenterLabelMedianLayer; strategy <= layered.CenterLabelCenterLayer; strategy++ {
		if labelStrategyUsesWidth(strategy) {
			processLabelDummyStrategy(arena, graphID, groups[int(strategy)], strategy, layerWidths)
		}
	}
}

func hasWidthBasedLabelStrategy(groups [][]*labelDummyInfo) bool {
	for strategy := layered.CenterLabelMedianLayer; strategy <= layered.CenterLabelCenterLayer; strategy++ {
		if labelStrategyUsesWidth(strategy) && len(groups[int(strategy)]) != 0 {
			return true
		}
	}
	return false
}

func labelStrategyUsesWidth(strategy layered.CenterEdgeLabelPlacementStrategy) bool {
	return strategy == layered.CenterLabelWidestLayer ||
		strategy == layered.CenterLabelCenterLayer ||
		strategy == layered.CenterLabelSpaceEfficientLayer
}

func gatherLabelDummyInfo(
	arena *layered.LGraphArena,
	labelDummy layered.LNodeID,
	defaultStrategy layered.CenterEdgeLabelPlacementStrategy,
) *labelDummyInfo {
	info := &labelDummyInfo{labelDummy: labelDummy, placement: defaultStrategy}
	current := labelDummy
	for {
		incoming := arena.NodeIncomingEdges(current)
		if len(incoming) == 0 {
			panic("layered: label dummy chain has no incoming edge")
		}
		current = arena.EdgeSourceNode(incoming[0])
		if arena.Node(current).Type != layered.LongEdgeNode {
			break
		}
		info.leftLongEdgeDummies = append(info.leftLongEdgeDummies, current)
	}
	for left, right := 0, len(info.leftLongEdgeDummies)-1; left < right; left, right = left+1, right-1 {
		info.leftLongEdgeDummies[left], info.leftLongEdgeDummies[right] =
			info.leftLongEdgeDummies[right], info.leftLongEdgeDummies[left]
	}
	current = labelDummy
	for {
		outgoing := arena.NodeOutgoingEdges(current)
		if len(outgoing) == 0 {
			panic("layered: label dummy chain has no outgoing edge")
		}
		current = arena.EdgeTargetNode(outgoing[0])
		if arena.Node(current).Type != layered.LongEdgeNode {
			break
		}
		info.rightLongEdgeDummies = append(info.rightLongEdgeDummies, current)
	}
	if len(info.leftLongEdgeDummies) == 0 {
		info.leftmostLayerIndex = arena.LayerIndex(*arena.Node(labelDummy).Layer)
	} else {
		info.leftmostLayerIndex = arena.LayerIndex(*arena.Node(info.leftLongEdgeDummies[0]).Layer)
	}
	if len(info.rightLongEdgeDummies) == 0 {
		info.rightmostLayerIndex = arena.LayerIndex(*arena.Node(labelDummy).Layer)
	} else {
		last := info.rightLongEdgeDummies[len(info.rightLongEdgeDummies)-1]
		info.rightmostLayerIndex = arena.LayerIndex(*arena.Node(last).Layer)
	}
	for _, labelID := range basegraph.Get(&arena.Node(labelDummy).Properties, layered.RepresentedLabels) {
		if strategy, present := basegraph.TryGet(&arena.Label(labelID).Properties, layered.CenterEdgeLabelPlacement); present {
			info.placement = strategy
			break
		}
	}
	return info
}

func processLabelDummyStrategy(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	infos []*labelDummyInfo,
	strategy layered.CenterEdgeLabelPlacementStrategy,
	layerWidths []float64,
) {
	if len(infos) == 0 {
		return
	}
	if strategy == layered.CenterLabelSpaceEfficientLayer {
		assignSpaceEfficientLabels(arena, infos, layerWidths)
		return
	}
	for _, info := range infos {
		target := info.leftmostLayerIndex
		switch strategy {
		case layered.CenterLabelMedianLayer:
			target += (info.totalDummyCount() - 1) / 2
		case layered.CenterLabelTailLayer, layered.CenterLabelHeadLayer:
			reversed := labelDummyPartOfReversedEdge(arena, info)
			head := strategy == layered.CenterLabelHeadLayer
			if (head && !reversed) || (!head && reversed) {
				target = info.rightmostLayerIndex
			}
			if (head && !reversed) || (!head && reversed) {
				basegraph.Set(&arena.Node(info.labelDummy).Properties, layered.AlignmentOption, layered.AlignmentRight)
			} else {
				basegraph.Set(&arena.Node(info.labelDummy).Properties, layered.AlignmentOption, layered.AlignmentLeft)
			}
		case layered.CenterLabelWidestLayer:
			for index := info.leftmostLayerIndex + 1; index <= info.rightmostLayerIndex; index++ {
				if layerWidths[index] > layerWidths[target] {
					target = index
				}
			}
		case layered.CenterLabelCenterLayer:
			sums := computeLabelLayerWidthSums(arena, graphID, info, layerWidths)
			threshold := sums[len(sums)-1] / 2
			for index, sum := range sums {
				if sum >= threshold {
					target = info.leftmostLayerIndex + index
					break
				}
			}
		}
		assignLabelDummyLayer(arena, info, target, layerWidths)
		markDummiesBeforeLabel(arena, info.labelDummy)
	}
}

func computeLabelLayerWidthSums(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	info *labelDummyInfo,
	layerWidths []float64,
) []float64 {
	graph := arena.Graph(graphID)
	between := math.Max(
		basegraph.Get(&graph.Properties, layered.SpacingEdgeNodeBetweenLayers)*2,
		basegraph.Get(&graph.Properties, layered.SpacingNodeNodeBetweenLayers),
	)
	sums := make([]float64, info.totalDummyCount())
	current, index := -between, 0
	for _, dummyID := range info.leftLongEdgeDummies {
		current += layerWidths[arena.LayerIndex(*arena.Node(dummyID).Layer)] + between
		sums[index], index = current, index+1
	}
	current += layerWidths[arena.LayerIndex(*arena.Node(info.labelDummy).Layer)] + between
	sums[index], index = current, index+1
	for _, dummyID := range info.rightLongEdgeDummies {
		current += layerWidths[arena.LayerIndex(*arena.Node(dummyID).Layer)] + between
		sums[index], index = current, index+1
	}
	return sums
}

func labelDummyPartOfReversedEdge(arena *layered.LGraphArena, info *labelDummyInfo) bool {
	incoming, outgoing := arena.NodeIncomingEdges(info.labelDummy), arena.NodeOutgoingEdges(info.labelDummy)
	if len(incoming) == 0 || len(outgoing) == 0 {
		panic("layered: label dummy has incomplete edge chain")
	}
	return basegraph.Get(&arena.Edge(incoming[0]).Properties, layered.Reversed) ||
		basegraph.Get(&arena.Edge(outgoing[0]).Properties, layered.Reversed)
}

func assignSpaceEfficientLabels(arena *layered.LGraphArena, infos []*labelDummyInfo, layerWidths []float64) {
	remaining := make([]*labelDummyInfo, 0, len(infos))
	for _, info := range infos {
		if info.leftmostLayerIndex == info.rightmostLayerIndex {
			assignLabelDummyLayer(arena, info, info.leftmostLayerIndex, layerWidths)
		} else if !assignLabelToAlreadyWideLayer(arena, info, layerWidths) {
			remaining = append(remaining, info)
		}
	}
	if len(remaining) == 0 {
		return
	}
	sort.SliceStable(remaining, func(i, j int) bool {
		return arena.Node(remaining[i].labelDummy).Size.X > arena.Node(remaining[j].labelDummy).Size.X
	})
	for index, info := range remaining {
		target := potentiallyWidestLayer(arena, remaining, index, layerWidths)
		assignLabelDummyLayer(arena, info, target, layerWidths)
	}
}

func assignLabelToAlreadyWideLayer(arena *layered.LGraphArena, info *labelDummyInfo, layerWidths []float64) bool {
	width := arena.Node(info.labelDummy).Size.X
	graphID := arena.NodeGraph(info.labelDummy)
	for index := info.leftmostLayerIndex; index <= info.rightmostLayerIndex; index++ {
		// The Java source checks Layer.size here rather than the separately
		// calculated layerWidths array. Preserve that distinction.
		if arena.Layer(arena.Graph(graphID).Layers[index]).Size.X >= width {
			assignLabelDummyLayer(arena, info, index, layerWidths)
			return true
		}
	}
	return false
}

func potentiallyWidestLayer(
	arena *layered.LGraphArena,
	infos []*labelDummyInfo,
	labelIndex int,
	layerWidths []float64,
) int {
	info := infos[labelIndex]
	width := arena.Node(info.labelDummy).Size.X
	widestIndex, widestWidth := info.leftmostLayerIndex, 0.0
	for layerIndex := info.leftmostLayerIndex; layerIndex <= info.rightmostLayerIndex; layerIndex++ {
		if width <= layerWidths[layerIndex] {
			return layerIndex
		}
		potential := layerWidths[layerIndex]
		var largest *labelDummyInfo
		for index := labelIndex + 1; index < len(infos); index++ {
			candidate := infos[index]
			if candidate.leftmostLayerIndex <= layerIndex && candidate.rightmostLayerIndex >= layerIndex {
				// ELK overwrites this while scanning the descending list; the last
				// applicable label is therefore observed despite the variable name.
				largest = candidate
			}
		}
		if largest != nil {
			potential = math.Max(potential, arena.Node(largest.labelDummy).Size.X)
		}
		if potential > widestWidth {
			widestIndex, widestWidth = layerIndex, potential
		}
	}
	return widestIndex
}

func assignLabelDummyLayer(
	arena *layered.LGraphArena,
	info *labelDummyInfo,
	targetLayerIndex int,
	layerWidths []float64,
) {
	current := info.leftmostLayerIndex + len(info.leftLongEdgeDummies)
	if targetLayerIndex != current {
		swapLabelAndLongEdgeDummies(arena, info.labelDummy, info.dummyAt(targetLayerIndex-info.leftmostLayerIndex))
	}
	newLayer := arena.LayerIndex(*arena.Node(info.labelDummy).Layer)
	layerWidths[newLayer] = math.Max(layerWidths[newLayer], arena.Node(info.labelDummy).Size.X)
	for _, labelID := range basegraph.Get(&arena.Node(info.labelDummy).Properties, layered.RepresentedLabels) {
		basegraph.Set(&arena.Label(labelID).Properties, includeLabel, true)
	}
}

func swapLabelAndLongEdgeDummies(arena *layered.LGraphArena, labelDummy, longDummy layered.LNodeID) {
	layer1, layer2 := *arena.Node(labelDummy).Layer, *arena.Node(longDummy).Layer
	position1, position2 := arena.NodeIndexInLayer(labelDummy), arena.NodeIndexInLayer(longDummy)
	input1, output1 := inputOutputPorts(arena, labelDummy)
	input2, output2 := inputOutputPorts(arena, longDummy)
	incoming1 := append([]layered.LEdgeID(nil), arena.Port(input1).IncomingEdges...)
	outgoing1 := append([]layered.LEdgeID(nil), arena.Port(output1).OutgoingEdges...)
	incoming2 := append([]layered.LEdgeID(nil), arena.Port(input2).IncomingEdges...)
	outgoing2 := append([]layered.LEdgeID(nil), arena.Port(output2).OutgoingEdges...)

	arena.SetNodeLayerAt(labelDummy, &layer2, position2)
	for _, edgeID := range incoming2 {
		arena.SetEdgeTarget(edgeID, &input1)
	}
	for _, edgeID := range outgoing2 {
		arena.SetEdgeSource(edgeID, &output1)
	}
	arena.SetNodeLayerAt(longDummy, &layer1, position1)
	for _, edgeID := range incoming1 {
		arena.SetEdgeTarget(edgeID, &input2)
	}
	for _, edgeID := range outgoing1 {
		arena.SetEdgeSource(edgeID, &output2)
	}
}

func inputOutputPorts(arena *layered.LGraphArena, nodeID layered.LNodeID) (layered.LPortID, layered.LPortID) {
	var input, output layered.LPortID
	hasInput, hasOutput := false, false
	for _, portID := range arena.Node(nodeID).Ports {
		if !hasInput && len(arena.Port(portID).IncomingEdges) != 0 {
			input, hasInput = portID, true
		}
		if !hasOutput && len(arena.Port(portID).OutgoingEdges) != 0 {
			output, hasOutput = portID, true
		}
	}
	if !hasInput || !hasOutput {
		panic("layered: dummy node has no input/output port")
	}
	return input, output
}

func markDummiesBeforeLabel(arena *layered.LGraphArena, labelDummy layered.LNodeID) {
	current := labelDummy
	for {
		incoming := arena.NodeIncomingEdges(current)
		if len(incoming) == 0 {
			return
		}
		current = arena.EdgeSourceNode(incoming[0])
		if arena.Node(current).Type != layered.LongEdgeNode {
			return
		}
		basegraph.Set(&arena.Node(current).Properties, layered.LongEdgeBeforeLabelDummy, true)
	}
}
