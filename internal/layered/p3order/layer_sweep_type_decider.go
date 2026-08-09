// Copyright (c) 2016 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"math"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// layerSweepNodeInfo is LayerSweepTypeDecider.NodeInfo. The values count
// paths rather than merely edges, hence transfer adds all three fields.
type layerSweepNodeInfo struct {
	connectedEdges        int
	hierarchicalInfluence int
	randomInfluence       int
}

func (info *layerSweepNodeInfo) transfer(other layerSweepNodeInfo) {
	info.hierarchicalInfluence += other.hierarchicalInfluence
	info.randomInfluence += other.randomInfluence
	info.connectedEdges += other.connectedEdges
}

// LayerSweepTypeDecider chooses between bottom-up and cross-hierarchical
// sweeping for one graph. InitAtLayerLevel and InitAtNodeLevel must be called
// in IInitializable traversal order before UseBottomUp.
type LayerSweepTypeDecider struct {
	NodeInfo [][]layerSweepNodeInfo
}

func NewLayerSweepTypeDecider(numberOfLayers int) *LayerSweepTypeDecider {
	return &LayerSweepTypeDecider{NodeInfo: make([][]layerSweepNodeInfo, numberOfLayers)}
}

func (decider *LayerSweepTypeDecider) InitAtLayerLevel(
	arena *layered.LGraphArena,
	layerIndex int,
	order [][]layered.LNodeID,
) {
	// This deliberately indexes the first node, matching ELK's initializer.
	layerID := *arena.Node(order[layerIndex][0]).Layer
	arena.Layer(layerID).ID = layerIndex
	decider.NodeInfo[layerIndex] = make([]layerSweepNodeInfo, len(order[layerIndex]))
}

func (decider *LayerSweepTypeDecider) InitAtNodeLevel(
	arena *layered.LGraphArena,
	layerIndex, nodeIndex int,
	order [][]layered.LNodeID,
) {
	nodeID := order[layerIndex][nodeIndex]
	arena.Node(nodeID).ID = nodeIndex
	decider.NodeInfo[layerIndex][nodeIndex] = layerSweepNodeInfo{}
}

func (decider *LayerSweepTypeDecider) UseBottomUp(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	parent *layered.LNodeID,
	crossMinDeterministic bool,
	order [][]layered.LNodeID,
) bool {
	boundary := basegraph.Get(&arena.Graph(graphID).Properties, layered.HierarchicalSweepiness)
	if boundary < -1 || parent == nil || decider.fixedPortOrder(arena, *parent) ||
		decider.fewerThanTwoInOutEdges(arena, *parent) {
		return true
	}
	if crossMinDeterministic {
		return false
	}

	pathsToRandom, pathsToHierarchical := 0, 0
	northSouthDummies := make([]layered.LNodeID, 0)
	for _, layer := range order {
		for _, nodeID := range layer {
			if arena.Node(nodeID).Type == layered.NorthSouthPortNode {
				northSouthDummies = append(northSouthDummies, nodeID)
				continue
			}

			current := decider.nodeInfo(arena, nodeID)
			if arena.Node(nodeID).Type == layered.ExternalPortNode {
				current.hierarchicalInfluence = 1
				if decider.isEasternDummy(arena, nodeID) {
					pathsToHierarchical += current.connectedEdges
				}
			} else if decider.hasNoPorts(arena, nodeID, layered.WestSide) {
				current.randomInfluence = 1
			} else if decider.hasNoPorts(arena, nodeID, layered.EastSide) {
				pathsToRandom += current.connectedEdges
			}

			for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
				value := *decider.nodeInfo(arena, nodeID)
				pathsToRandom += value.randomInfluence
				pathsToHierarchical += value.hierarchicalInfluence
				decider.transferInfoTo(arena, value, arena.EdgeTargetNode(edgeID))
			}

			for _, side := range []layered.PortSide{layered.NorthSide, layered.SouthSide} {
				for _, portID := range arena.NodePortsOnSide(nodeID, side) {
					dummyID, found := basegraph.TryGet(&arena.Port(portID).Properties, layered.PortDummy)
					if !found {
						continue
					}
					value := *decider.nodeInfo(arena, nodeID)
					pathsToRandom += value.randomInfluence
					pathsToHierarchical += value.hierarchicalInfluence
					decider.transferInfoTo(arena, value, dummyID)
				}
			}
		}

		for _, nodeID := range northSouthDummies {
			for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
				value := *decider.nodeInfo(arena, nodeID)
				pathsToRandom += value.randomInfluence
				pathsToHierarchical += value.hierarchicalInfluence
				decider.transferInfoTo(arena, value, arena.EdgeTargetNode(edgeID))
			}
		}
		northSouthDummies = northSouthDummies[:0]
	}

	allPaths := pathsToRandom + pathsToHierarchical
	normalized := math.Inf(1)
	if allPaths != 0 {
		normalized = float64(pathsToRandom-pathsToHierarchical) / float64(allPaths)
	}
	return normalized >= boundary
}

func (decider *LayerSweepTypeDecider) fixedPortOrder(arena *layered.LGraphArena, parent layered.LNodeID) bool {
	return basegraph.Get(&arena.Node(parent).Properties, layered.PortConstraintsOption).IsOrderFixed()
}

func (decider *LayerSweepTypeDecider) fewerThanTwoInOutEdges(
	arena *layered.LGraphArena,
	parent layered.LNodeID,
) bool {
	return len(arena.NodePortsOnSide(parent, layered.EastSide)) < 2 &&
		len(arena.NodePortsOnSide(parent, layered.WestSide)) < 2
}

func (decider *LayerSweepTypeDecider) hasNoPorts(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	side layered.PortSide,
) bool {
	ports := arena.NodePortsOnSide(nodeID, side)
	if len(ports) == 0 {
		return true
	}
	for _, portID := range ports {
		if len(arena.PortConnectedEdges(portID)) > 0 {
			return false
		}
	}
	return true
}

func (decider *LayerSweepTypeDecider) isEasternDummy(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
) bool {
	origin, found := basegraph.TryGet(&arena.Node(nodeID).Properties, layered.OriginalElement)
	if !found || origin.Kind != layered.OriginLPort {
		panic("p3order: external port dummy has no LPort origin")
	}
	return arena.Port(origin.LPort).Side == layered.EastSide
}

func (decider *LayerSweepTypeDecider) transferInfoTo(
	arena *layered.LGraphArena,
	current layerSweepNodeInfo,
	target layered.LNodeID,
) {
	targetInfo := decider.nodeInfo(arena, target)
	targetInfo.transfer(current)
	targetInfo.connectedEdges++
}

func (decider *LayerSweepTypeDecider) nodeInfo(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
) *layerSweepNodeInfo {
	layerID := *arena.Node(nodeID).Layer
	return &decider.NodeInfo[arena.Layer(layerID).ID][arena.Node(nodeID).ID]
}
