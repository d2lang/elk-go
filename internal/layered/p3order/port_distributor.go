// Copyright (c) 2010, 2015 Kiel University and others.
// Copyright (c) 2012, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"sort"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

type PortDistributorKind uint8

const (
	NodeRelativePortDistributor PortDistributorKind = iota
	LayerTotalPortDistributor
)

type BarycenterPortDistributor struct {
	Kind              PortDistributorKind
	PortRanks         []float32
	minimumBarycenter float32
	maximumBarycenter float32
	nodePositions     [][]int
	portBarycenter    []float32
	inLayerPorts      []layered.LPortID
	nPorts            int
}

func NewBarycenterPortDistributor(kind PortDistributorKind, numberOfLayers int) *BarycenterPortDistributor {
	return &BarycenterPortDistributor{Kind: kind, nodePositions: make([][]int, numberOfLayers)}
}

func (distributor *BarycenterPortDistributor) DistributePortsWhileSweeping(
	arena *layered.LGraphArena,
	nodeOrder [][]layered.LNodeID,
	currentIndex int,
	forward bool,
) bool {
	distributor.updateNodePositions(arena, nodeOrder[currentIndex])
	freeLayer := append([]layered.LNodeID(nil), nodeOrder[currentIndex]...)
	side := layered.EastSide
	if forward {
		side = layered.WestSide
	}
	if !isFirstSweepLayer(len(nodeOrder), currentIndex, forward) {
		fixedIndex := currentIndex + 1
		if forward {
			fixedIndex = currentIndex - 1
		}
		fixedLayer := append([]layered.LNodeID(nil), nodeOrder[fixedIndex]...)
		distributor.CalculatePortRanks(arena, fixedLayer, portTypeForSweep(forward))
		for _, nodeID := range freeLayer {
			distributor.distributePorts(arena, nodeID, side)
		}
		distributor.CalculatePortRanks(arena, freeLayer, portTypeForSweep(!forward))
		for _, nodeID := range fixedLayer {
			if arena.Node(nodeID).NestedGraph == nil {
				distributor.distributePorts(arena, nodeID, side.Opposed())
			}
		}
	} else {
		for _, nodeID := range freeLayer {
			distributor.distributePorts(arena, nodeID, side)
		}
	}
	return false
}

func (distributor *BarycenterPortDistributor) CalculatePortRanks(
	arena *layered.LGraphArena,
	layer []layered.LNodeID,
	portType layered.PortType,
) {
	var consumed float32
	for _, nodeID := range layer {
		consumed += distributor.calculateNodePortRanks(arena, nodeID, consumed, portType)
	}
}

func (distributor *BarycenterPortDistributor) calculateNodePortRanks(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	rankSum float32,
	portType layered.PortType,
) float32 {
	ports := arena.Node(nodeID).Ports
	switch distributor.Kind {
	case NodeRelativePortDistributor:
		switch portType {
		case layered.PortTypeInput:
			inputCount, northInputCount := 0, 0
			for _, portID := range ports {
				if len(arena.Port(portID).IncomingEdges) > 0 {
					inputCount++
					if arena.Port(portID).Side == layered.NorthSide {
						northInputCount++
					}
				}
			}
			increment := float32(1) / float32(inputCount+1)
			northPosition := rankSum + float32(northInputCount)*increment
			restPosition := rankSum + 1 - increment
			for _, portID := range inputPorts(arena, nodeID) {
				if arena.Port(portID).Side == layered.NorthSide {
					distributor.PortRanks[arena.Port(portID).ID] = northPosition
					northPosition -= increment
				} else {
					distributor.PortRanks[arena.Port(portID).ID] = restPosition
					restPosition -= increment
				}
			}
		case layered.PortTypeOutput:
			outputCount := 0
			for _, portID := range ports {
				if len(arena.Port(portID).OutgoingEdges) > 0 {
					outputCount++
				}
			}
			increment := float32(1) / float32(outputCount+1)
			position := rankSum + increment
			for _, portID := range outputPorts(arena, nodeID) {
				distributor.PortRanks[arena.Port(portID).ID] = position
				position += increment
			}
		default:
			panic("p3order: undefined port type")
		}
		return 1
	case LayerTotalPortDistributor:
		switch portType {
		case layered.PortTypeInput:
			inputCount, northInputCount := 0, 0
			for _, portID := range ports {
				if len(arena.Port(portID).IncomingEdges) > 0 {
					inputCount++
					if arena.Port(portID).Side == layered.NorthSide {
						northInputCount++
					}
				}
			}
			northPosition := rankSum + float32(northInputCount)
			restPosition := rankSum + float32(inputCount)
			for _, portID := range inputPorts(arena, nodeID) {
				if arena.Port(portID).Side == layered.NorthSide {
					distributor.PortRanks[arena.Port(portID).ID] = northPosition
					northPosition--
				} else {
					distributor.PortRanks[arena.Port(portID).ID] = restPosition
					restPosition--
				}
			}
			return float32(inputCount)
		case layered.PortTypeOutput:
			position := 0
			for _, portID := range outputPorts(arena, nodeID) {
				position++
				distributor.PortRanks[arena.Port(portID).ID] = rankSum + float32(position)
			}
			return float32(position)
		default:
			panic("p3order: undefined port type")
		}
	default:
		panic("p3order: unknown port distributor")
	}
}

func (distributor *BarycenterPortDistributor) distributePorts(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	side layered.PortSide,
) {
	constraints := basegraph.Get(&arena.Node(nodeID).Properties, layered.PortConstraintsOption)
	if constraints.IsOrderFixed() {
		return
	}
	for _, portSide := range []layered.PortSide{side, layered.SouthSide, layered.NorthSide} {
		distributor.distributePortList(arena, nodeID, arena.NodePortsOnSide(nodeID, portSide))
	}
	ports := arena.Node(nodeID).Ports
	sort.SliceStable(ports, func(i, j int) bool {
		first, second := arena.Port(ports[i]), arena.Port(ports[j])
		if first.Side != second.Side {
			return first.Side < second.Side
		}
		firstBarycenter := distributor.portBarycenter[first.ID]
		secondBarycenter := distributor.portBarycenter[second.ID]
		if firstBarycenter == 0 && secondBarycenter == 0 {
			return false
		}
		if firstBarycenter == 0 {
			return true
		}
		if secondBarycenter == 0 {
			return false
		}
		return firstBarycenter < secondBarycenter
	})
	arena.Node(nodeID).Ports = ports
	arena.InvalidatePortSideCache(nodeID)
}

func (distributor *BarycenterPortDistributor) distributePortList(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	ports []layered.LPortID,
) {
	distributor.inLayerPorts = distributor.inLayerPorts[:0]
	distributor.collectPortBarycenters(arena, nodeID, ports)
	if len(distributor.inLayerPorts) > 0 {
		distributor.calculateInLayerPortBarycenters(arena, nodeID)
	}
}

func (distributor *BarycenterPortDistributor) collectPortBarycenters(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	ports []layered.LPortID,
) {
	distributor.minimumBarycenter = 0
	distributor.maximumBarycenter = 0
	absurdlyLarge := float32(2*len(arena.Layer(*arena.Node(nodeID).Layer).Nodes) + 1)
	for _, portID := range ports {
		port := arena.Port(portID)
		northSouth := port.Side == layered.NorthSide || port.Side == layered.SouthSide
		var sum float32
		if northSouth {
			dummyID, found := basegraph.TryGet(&port.Properties, layered.PortDummy)
			if !found {
				continue
			}
			sum += distributor.northSouthPortValue(arena, absurdlyLarge, portID, dummyID)
		} else {
			inLayer := false
			for _, edgeID := range port.OutgoingEdges {
				connected := *arena.Edge(edgeID).Target
				if sameLayer(arena.Node(*arena.Port(connected).Node).Layer, arena.Node(nodeID).Layer) {
					distributor.inLayerPorts = append(distributor.inLayerPorts, portID)
					inLayer = true
					break
				}
				sum += distributor.PortRanks[arena.Port(connected).ID]
			}
			if inLayer {
				continue
			}
			for _, edgeID := range port.IncomingEdges {
				connected := *arena.Edge(edgeID).Source
				if sameLayer(arena.Node(*arena.Port(connected).Node).Layer, arena.Node(nodeID).Layer) {
					distributor.inLayerPorts = append(distributor.inLayerPorts, portID)
					inLayer = true
					break
				}
				sum -= distributor.PortRanks[arena.Port(connected).ID]
			}
			if inLayer {
				continue
			}
		}
		degree := len(arena.PortConnectedEdges(portID))
		if degree > 0 {
			barycenter := sum / float32(degree)
			distributor.portBarycenter[port.ID] = barycenter
			if barycenter < distributor.minimumBarycenter {
				distributor.minimumBarycenter = barycenter
			}
			if barycenter > distributor.maximumBarycenter {
				distributor.maximumBarycenter = barycenter
			}
		} else if northSouth {
			distributor.portBarycenter[port.ID] = sum
		}
	}
}

func (distributor *BarycenterPortDistributor) calculateInLayerPortBarycenters(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
) {
	nodePosition := distributor.positionOf(arena, nodeID) + 1
	layerSize := len(arena.Layer(*arena.Node(nodeID).Layer).Nodes) + 1
	for _, portID := range distributor.inLayerPorts {
		sum, connections := 0, 0
		for _, edgeID := range arena.PortConnectedEdges(portID) {
			connected := arena.EdgeOtherPort(edgeID, portID)
			connectedNode := *arena.Port(connected).Node
			if sameLayer(arena.Node(connectedNode).Layer, arena.Node(nodeID).Layer) {
				sum += distributor.positionOf(arena, connectedNode) + 1
				connections++
			}
		}
		barycenter := float32(sum) / float32(connections)
		port := arena.Port(portID)
		if port.Side == layered.EastSide {
			if barycenter < float32(nodePosition) {
				distributor.portBarycenter[port.ID] = distributor.minimumBarycenter - barycenter
			} else {
				distributor.portBarycenter[port.ID] = distributor.maximumBarycenter + float32(layerSize) - barycenter
			}
		} else if port.Side == layered.WestSide {
			if barycenter < float32(nodePosition) {
				distributor.portBarycenter[port.ID] = distributor.maximumBarycenter + barycenter
			} else {
				distributor.portBarycenter[port.ID] = distributor.minimumBarycenter - (float32(layerSize) - barycenter)
			}
		}
	}
}

func (distributor *BarycenterPortDistributor) northSouthPortValue(
	arena *layered.LGraphArena,
	absurdlyLarge float32,
	portID layered.LPortID,
	dummyID layered.LNodeID,
) float32 {
	input, output := false, false
	for _, dummyPortID := range arena.Node(dummyID).Ports {
		origin, found := basegraph.TryGet(&arena.Port(dummyPortID).Properties, layered.OriginalElement)
		if !found || origin.Kind != layered.OriginLPort || origin.LPort != portID {
			continue
		}
		if len(arena.Port(dummyPortID).OutgoingEdges) > 0 {
			output = true
		} else if len(arena.Port(dummyPortID).IncomingEdges) > 0 {
			input = true
		}
	}
	position := float32(distributor.positionOf(arena, dummyID))
	if input != output {
		if input {
			if arena.Port(portID).Side == layered.NorthSide {
				return -position
			}
			return absurdlyLarge - position
		}
		return position + 1
	}
	if input && output && arena.Port(portID).Side == layered.SouthSide {
		return absurdlyLarge / 2
	}
	return 0
}

func (distributor *BarycenterPortDistributor) positionOf(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
) int {
	return distributor.nodePositions[arena.Layer(*arena.Node(nodeID).Layer).ID][arena.Node(nodeID).ID]
}

func (distributor *BarycenterPortDistributor) updateNodePositions(
	arena *layered.LGraphArena,
	layer []layered.LNodeID,
) {
	for position, nodeID := range layer {
		distributor.nodePositions[arena.Layer(*arena.Node(nodeID).Layer).ID][arena.Node(nodeID).ID] = position
	}
}

func (distributor *BarycenterPortDistributor) InitAtLayerLevel(layerIndex int, order [][]layered.LNodeID) {
	distributor.nodePositions[layerIndex] = make([]int, len(order[layerIndex]))
}

func (distributor *BarycenterPortDistributor) InitAtNodeLevel(
	arena *layered.LGraphArena,
	layerIndex, nodeIndex int,
	order [][]layered.LNodeID,
) {
	nodeID := order[layerIndex][nodeIndex]
	arena.Node(nodeID).ID = nodeIndex
	distributor.nodePositions[layerIndex][nodeIndex] = nodeIndex
}

func (distributor *BarycenterPortDistributor) InitAtPortLevel(
	arena *layered.LGraphArena,
	layerIndex, nodeIndex, portIndex int,
	order [][]layered.LNodeID,
) {
	portID := arena.Node(order[layerIndex][nodeIndex]).Ports[portIndex]
	arena.Port(portID).ID = distributor.nPorts
	distributor.nPorts++
}

func (distributor *BarycenterPortDistributor) InitAfterTraversal() {
	distributor.PortRanks = make([]float32, distributor.nPorts)
	distributor.portBarycenter = make([]float32, distributor.nPorts)
}

func InitializeBarycenterPortDistributor(
	distributor *BarycenterPortDistributor,
	arena *layered.LGraphArena,
	order [][]layered.LNodeID,
) {
	for layerIndex, nodes := range order {
		distributor.InitAtLayerLevel(layerIndex, order)
		if len(nodes) > 0 && arena.Node(nodes[0]).Layer != nil {
			arena.Layer(*arena.Node(nodes[0]).Layer).ID = layerIndex
		}
		for nodeIndex, nodeID := range nodes {
			distributor.InitAtNodeLevel(arena, layerIndex, nodeIndex, order)
			for portIndex := range arena.Node(nodeID).Ports {
				distributor.InitAtPortLevel(arena, layerIndex, nodeIndex, portIndex, order)
			}
		}
	}
	distributor.InitAfterTraversal()
}

func inputPorts(arena *layered.LGraphArena, nodeID layered.LNodeID) []layered.LPortID {
	result := make([]layered.LPortID, 0)
	for _, portID := range arena.Node(nodeID).Ports {
		if len(arena.Port(portID).IncomingEdges) > 0 {
			result = append(result, portID)
		}
	}
	return result
}

func outputPorts(arena *layered.LGraphArena, nodeID layered.LNodeID) []layered.LPortID {
	result := make([]layered.LPortID, 0)
	for _, portID := range arena.Node(nodeID).Ports {
		if len(arena.Port(portID).OutgoingEdges) > 0 {
			result = append(result, portID)
		}
	}
	return result
}

func portTypeForSweep(forward bool) layered.PortType {
	if forward {
		return layered.PortTypeOutput
	}
	return layered.PortTypeInput
}

func isFirstSweepLayer(length, currentIndex int, forward bool) bool {
	if forward {
		return currentIndex == 0
	}
	return currentIndex == length-1
}
