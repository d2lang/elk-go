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

	// The layers' infos are cut, in layer order, from infoBacking when it
	// is large enough (each part is handed out once), else made.
	infoBacking []layerSweepNodeInfo
	infoUsed    int

	// dummies is UseBottomUp's north/south dummy buffer: the workspace's
	// shared one (UseBottomUp calls never nest), or nil to allocate.
	dummies *[]layered.LNodeID
}

func NewLayerSweepTypeDecider(numberOfLayers int) *LayerSweepTypeDecider {
	return newLayerSweepTypeDecider(nil, numberOfLayers, 0)
}

// newLayerSweepTypeDecider is NewLayerSweepTypeDecider with its storage,
// including the infos of totalNodes nodes, taken from ws (nil: allocated
// per layer).
func newLayerSweepTypeDecider(ws *crossMinWorkspace, numberOfLayers, totalNodes int) *LayerSweepTypeDecider {
	decider := ws.deciderSlab().one()
	decider.NodeInfo = ws.infoListSlab().take(numberOfLayers)
	if ws != nil {
		decider.infoBacking = ws.infoSlab().take(totalNodes)
		decider.dummies = &ws.deciderDummies
	}
	return decider
}

func (decider *LayerSweepTypeDecider) InitAtLayerLevel(
	arena *layered.LGraphArena,
	layerIndex int,
	order [][]layered.LNodeID,
) {
	// This deliberately indexes the first node, matching ELK's initializer.
	layerID := *arena.Node(order[layerIndex][0]).Layer
	arena.Layer(layerID).ID = layerIndex
	n := len(order[layerIndex])
	if start := decider.infoUsed; start+n <= len(decider.infoBacking) {
		decider.NodeInfo[layerIndex] = decider.infoBacking[start : start+n : start+n]
		decider.infoUsed += n
		return
	}
	decider.NodeInfo[layerIndex] = make([]layerSweepNodeInfo, n)
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
	if decider.dummies != nil {
		northSouthDummies = (*decider.dummies)[:0]
	}
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

			// NodeOutgoingEdges and NodePortsOnSide walked in place: nothing
			// below changes a port list or an edge list.
			for _, portID := range arena.Node(nodeID).Ports {
				for _, edgeID := range arena.Port(portID).OutgoingEdges {
					value := *decider.nodeInfo(arena, nodeID)
					pathsToRandom += value.randomInfluence
					pathsToHierarchical += value.hierarchicalInfluence
					decider.transferInfoTo(arena, value, arena.EdgeTargetNode(edgeID))
				}
			}

			for _, side := range [2]layered.PortSide{layered.NorthSide, layered.SouthSide} {
				for _, portID := range arena.Node(nodeID).Ports {
					if arena.Port(portID).Side != side {
						continue
					}
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
			for _, portID := range arena.Node(nodeID).Ports {
				for _, edgeID := range arena.Port(portID).OutgoingEdges {
					value := *decider.nodeInfo(arena, nodeID)
					pathsToRandom += value.randomInfluence
					pathsToHierarchical += value.hierarchicalInfluence
					decider.transferInfoTo(arena, value, arena.EdgeTargetNode(edgeID))
				}
			}
		}
		northSouthDummies = northSouthDummies[:0]
	}

	allPaths := pathsToRandom + pathsToHierarchical
	normalized := math.Inf(1)
	if allPaths != 0 {
		normalized = float64(pathsToRandom-pathsToHierarchical) / float64(allPaths)
	}
	if decider.dummies != nil {
		*decider.dummies = northSouthDummies[:0]
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
	east, west := 0, 0
	for _, portID := range arena.Node(parent).Ports {
		switch arena.Port(portID).Side {
		case layered.EastSide:
			east++
		case layered.WestSide:
			west++
		}
	}
	return east < 2 && west < 2
}

func (decider *LayerSweepTypeDecider) hasNoPorts(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	side layered.PortSide,
) bool {
	// NodePortsOnSide and PortConnectedEdges counted in place; no port on
	// the side, or none with an edge, both answer true.
	for _, portID := range arena.Node(nodeID).Ports {
		if port := arena.Port(portID); port.Side == side && len(port.IncomingEdges)+len(port.OutgoingEdges) > 0 {
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
