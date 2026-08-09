// Copyright (c) 2016 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"math"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// PromoteNodes is the arena-backed port of ELK 0.8.1's NodePromotion
// intermediate processor (Nikolov and Tarassov's dummy-reduction heuristic).
func PromoteNodes(arena *layered.LGraphArena, graphID layered.LGraphID) {
	promotion := nodePromotion{arena: arena, graphID: graphID}
	promotion.precalculate()
	promotion.strategy = basegraph.Get(&arena.Graph(graphID).Properties, layered.NodePromotionStrategyOption)
	boundary := basegraph.Get(&arena.Graph(graphID).Properties, layered.NodePromotionMaxIterations)
	always := func(_, _ int) bool { return true }

	switch promotion.strategy {
	case layered.NodePromotionNikolov, layered.NodePromotionNikolovPixel:
		promotion.run(always)
	case layered.NodePromotionNikolovImproved:
		promotion.strategy = layered.NodePromotionNoBoundary
		promotion.run(always)
		newMaximum := 0
		for _, width := range promotion.currentWidth {
			newMaximum = max(newMaximum, width)
		}
		if newMaximum > promotion.maximumWidth {
			promotion.strategy = layered.NodePromotionNikolov
			promotion.run(always)
		}
	case layered.NodePromotionNikolovImprovedPixel:
		promotion.strategy = layered.NodePromotionNoBoundary
		promotion.run(always)
		newMaximum := 0.0
		for _, width := range promotion.currentWidthPixel {
			newMaximum = math.Max(newMaximum, width)
		}
		if newMaximum > promotion.maximumWidthPixel {
			promotion.strategy = layered.NodePromotionNikolovPixel
			promotion.run(always)
		}
	case layered.NodePromotionNodeCountPercentage:
		limit := int(math.Ceil(float64(len(promotion.layers)*boundary) / 100.0))
		promotion.run(func(_, iteration int) bool { return iteration < limit })
	case layered.NodePromotionDummyNodePercentage:
		limit := int(math.Ceil(float64(promotion.dummyNodeCount*boundary) / 100.0))
		promotion.run(func(reduced, _ int) bool { return reduced < limit })
	default:
		// The GraphConfigurator does not schedule NONE, but the Java
		// processor's default arm still runs the unbounded heuristic when
		// invoked directly.
		promotion.run(always)
	}
	promotion.apply()
}

type nodePromotion struct {
	arena   *layered.LGraphArena
	graphID layered.LGraphID

	nodes                  []layered.LNodeID
	nodesWithIncomingEdges []layered.LNodeID
	nodeIndex              map[layered.LNodeID]int
	layers                 []int
	degreeDifference       [][3]int
	currentWidth           []int
	currentWidthPixel      []float64
	maximumWidth           int
	maximumWidthPixel      float64
	dummyNodeCount         int
	maximumHeight          int
	nodeSizeAffix          float64
	dummySize              float64
	strategy               layered.NodePromotionStrategy
}

func (promotion *nodePromotion) precalculate() {
	graph := promotion.arena.Graph(promotion.graphID)
	promotion.nodeSizeAffix = basegraph.Get(&graph.Properties, layered.SpacingNodeNode)
	promotion.dummySize = basegraph.Get(&graph.Properties, layered.SpacingEdgeNodeBetweenLayers)
	promotion.maximumHeight = len(graph.Layers)
	promotion.currentWidth = make([]int, promotion.maximumHeight)
	promotion.currentWidthPixel = make([]float64, promotion.maximumHeight)
	promotion.nodeIndex = make(map[layered.LNodeID]int)

	// ELK numbers layers in reverse and nodes in forward layer order.
	layerRanks := make(map[layered.LayerID]int, len(graph.Layers))
	reversedRank := promotion.maximumHeight - 1
	for _, layerID := range graph.Layers {
		layerRanks[layerID] = reversedRank
		promotion.arena.Layer(layerID).ID = reversedRank
		for _, nodeID := range promotion.arena.Layer(layerID).Nodes {
			promotion.nodeIndex[nodeID] = len(promotion.nodes)
			promotion.nodes = append(promotion.nodes, nodeID)
		}
		reversedRank--
	}

	promotion.layers = make([]int, len(promotion.nodes))
	promotion.degreeDifference = make([][3]int, len(promotion.nodes))
	dummyBaggage := 0
	for _, layerID := range graph.Layers {
		layerRank := layerRanks[layerID]
		incoming, outgoing := 0, 0
		layerNodes := promotion.arena.Layer(layerID).Nodes
		layerWidthPixel := 0.0
		for _, nodeID := range layerNodes {
			index := promotion.nodeIndex[nodeID]
			promotion.layers[index] = layerRank
			node := promotion.arena.Node(nodeID)
			layerWidthPixel += node.Size.Y + promotion.nodeSizeAffix
			inDegree := len(promotion.arena.NodeIncomingEdges(nodeID))
			outDegree := len(promotion.arena.NodeOutgoingEdges(nodeID))
			promotion.degreeDifference[index] = [3]int{outDegree - inDegree, inDegree, outDegree}
			incoming += inDegree
			outgoing += outDegree
			if inDegree > 0 {
				promotion.nodesWithIncomingEdges = append(promotion.nodesWithIncomingEdges, nodeID)
			}
		}
		dummyBaggage -= incoming
		width := len(layerNodes) + dummyBaggage
		layerWidthPixel += float64(dummyBaggage) * promotion.dummySize
		promotion.currentWidth[layerRank] = width
		promotion.currentWidthPixel[layerRank] = layerWidthPixel
		promotion.maximumWidth = max(promotion.maximumWidth, width)
		promotion.maximumWidthPixel = math.Max(promotion.maximumWidthPixel, layerWidthPixel)
		promotion.dummyNodeCount += dummyBaggage
		dummyBaggage += outgoing
	}
}

func (promotion *nodePromotion) run(continueAfter func(reducedDummies, iteration int) bool) {
	iteration := 0
	reducedDummies := 0
	layeringBackup := append([]int(nil), promotion.layers...)
	dummyBackup := promotion.dummyNodeCount
	heightBackup := promotion.maximumHeight
	widthBackup := promotion.currentWidth
	widthPixelBackup := promotion.currentWidthPixel

	for {
		promotions := 0
		for _, nodeID := range promotion.nodesWithIncomingEdges {
			difference, widthNotExceeded := promotion.promote(nodeID)
			apply := true
			if promotion.strategy == layered.NodePromotionNikolov ||
				promotion.strategy == layered.NodePromotionNikolovPixel {
				apply = widthNotExceeded
			}
			if difference < 0 && apply {
				promotions++
				layeringBackup = append(layeringBackup[:0], promotion.layers...)
				promotion.dummyNodeCount += difference
				reducedDummies += dummyBackup - promotion.dummyNodeCount
				// This second addition is present in ELK 0.8.1 and is
				// intentionally retained for bytecode-equivalent behavior.
				dummyBackup = promotion.dummyNodeCount + difference
				heightBackup = promotion.maximumHeight
				widthBackup = append([]int(nil), promotion.currentWidth...)
				widthPixelBackup = append([]float64(nil), promotion.currentWidthPixel...)
			} else {
				promotion.layers = append(promotion.layers[:0], layeringBackup...)
				promotion.dummyNodeCount = dummyBackup
				promotion.currentWidth = append(promotion.currentWidth[:0], widthBackup...)
				promotion.currentWidthPixel = append(promotion.currentWidthPixel[:0], widthPixelBackup...)
				promotion.maximumHeight = heightBackup
			}
		}
		iteration++
		if promotions == 0 || !continueAfter(reducedDummies, iteration) {
			return
		}
	}
}

func (promotion *nodePromotion) promote(nodeID layered.LNodeID) (dummyDifference int, widthNotExceeded bool) {
	widthNotExceeded = true
	index := promotion.nodeIndex[nodeID]
	layerPosition := promotion.layers[index]
	nodeSize := promotion.arena.Node(nodeID).Size.Y + promotion.nodeSizeAffix
	dummiesBuilt := promotion.degreeDifference[index][2]

	promotion.currentWidth[layerPosition] += -1 + dummiesBuilt
	promotion.currentWidthPixel[layerPosition] += -nodeSize + float64(dummiesBuilt)*promotion.dummySize
	layerPosition++
	if layerPosition >= promotion.maximumHeight {
		promotion.maximumHeight++
		promotion.currentWidth = append(promotion.currentWidth, 1)
		promotion.currentWidthPixel = append(promotion.currentWidthPixel, nodeSize)
	} else {
		dummiesReduced := promotion.degreeDifference[index][1]
		promotion.currentWidth[layerPosition] += 1 - dummiesReduced
		promotion.currentWidthPixel[layerPosition] += nodeSize - float64(dummiesReduced)*promotion.dummySize
	}

	if promotion.strategy == layered.NodePromotionNikolov &&
		(promotion.currentWidth[layerPosition] > promotion.maximumWidth ||
			promotion.currentWidth[layerPosition-1] > promotion.maximumWidth) {
		widthNotExceeded = false
	} else if promotion.strategy == layered.NodePromotionNikolovPixel &&
		(promotion.currentWidthPixel[layerPosition] > promotion.maximumWidthPixel ||
			promotion.currentWidthPixel[layerPosition-1] > promotion.maximumWidthPixel) {
		widthNotExceeded = false
	}

	for _, edgeID := range promotion.arena.NodeIncomingEdges(nodeID) {
		predecessor := promotion.arena.EdgeSourceNode(edgeID)
		predecessorIndex := promotion.nodeIndex[predecessor]
		if promotion.layers[predecessorIndex] == layerPosition {
			difference, predecessorWidthNotExceeded := promotion.promote(predecessor)
			dummyDifference += difference
			widthNotExceeded = widthNotExceeded && predecessorWidthNotExceeded
		}
	}
	promotion.layers[index] = layerPosition
	dummyDifference += promotion.degreeDifference[index][0]
	return dummyDifference, widthNotExceeded
}

func (promotion *nodePromotion) apply() {
	// ELK deliberately constructs maximumHeight+1 layers, then removes every
	// empty one after assigning nodes using the reversed ranks.
	newLayers := make([]layered.LayerID, 0, promotion.maximumHeight+1)
	for index := 0; index <= promotion.maximumHeight; index++ {
		layerID := promotion.arena.AddLayer(promotion.graphID)
		promotion.arena.Layer(layerID).ID = promotion.maximumHeight - index
		newLayers = append(newLayers, layerID)
	}
	for _, nodeID := range promotion.nodes {
		index := promotion.nodeIndex[nodeID]
		promotion.arena.SetNodeLayer(nodeID, &newLayers[promotion.maximumHeight-promotion.layers[index]])
	}
	kept := newLayers[:0]
	for _, layerID := range newLayers {
		if len(promotion.arena.Layer(layerID).Nodes) != 0 {
			kept = append(kept, layerID)
		}
	}
	promotion.arena.Graph(promotion.graphID).Layers = kept
}
