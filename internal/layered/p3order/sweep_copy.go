// Copyright (c) 2016 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"cmp"
	"sort"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// SweepCopy stores the node order and each node's current port order. Port
// orders are indexed by layer and position, just as in ELK 0.8.1; this means
// they travel with a candidate sweep order rather than with arena IDs.
type SweepCopy struct {
	nodeOrder  [][]layered.LNodeID
	portOrders [][][]layered.LPortID
}

func NewSweepCopy(arena *layered.LGraphArena, nodeOrder [][]layered.LNodeID) *SweepCopy {
	copy := &SweepCopy{
		nodeOrder:  cloneNodeOrder(nodeOrder),
		portOrders: make([][][]layered.LPortID, len(nodeOrder)),
	}
	for layerIndex, nodes := range nodeOrder {
		copy.portOrders[layerIndex] = make([][]layered.LPortID, len(nodes))
		for nodeIndex, nodeID := range nodes {
			copy.portOrders[layerIndex][nodeIndex] = append([]layered.LPortID(nil), arena.Node(nodeID).Ports...)
		}
	}
	return copy
}

func (copy *SweepCopy) Clone() *SweepCopy {
	if copy == nil {
		return nil
	}
	result := &SweepCopy{
		nodeOrder:  cloneNodeOrder(copy.nodeOrder),
		portOrders: make([][][]layered.LPortID, len(copy.portOrders)),
	}
	for layerIndex := range copy.portOrders {
		result.portOrders[layerIndex] = make([][]layered.LPortID, len(copy.portOrders[layerIndex]))
		for nodeIndex := range copy.portOrders[layerIndex] {
			result.portOrders[layerIndex][nodeIndex] = append(
				[]layered.LPortID(nil), copy.portOrders[layerIndex][nodeIndex]...,
			)
		}
	}
	return result
}

// Nodes returns the saved order. Callers must treat it as read-only, matching
// SweepCopy.nodes()'s contract in Java.
func (copy *SweepCopy) Nodes() [][]layered.LNodeID { return copy.nodeOrder }

func (copy *SweepCopy) TransferNodeAndPortOrdersToGraph(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	setPortConstraints bool,
) {
	northSouthDummies := make([]layered.LNodeID, 0)
	updatePortOrder := make([]layered.LNodeID, 0)
	seenUpdate := make(map[layered.LNodeID]bool)
	layers := append([]layered.LayerID(nil), arena.Graph(graphID).Layers...)
	for layerIndex, layerID := range layers {
		northSouthDummies = northSouthDummies[:0]
		for nodeIndex := range arena.Layer(layerID).Nodes {
			nodeID := copy.nodeOrder[layerIndex][nodeIndex]
			arena.Node(nodeID).ID = nodeIndex
			if arena.Node(nodeID).Type == layered.NorthSouthPortNode {
				northSouthDummies = append(northSouthDummies, nodeID)
			}
			arena.Layer(layerID).Nodes[nodeIndex] = nodeID
			arena.Node(nodeID).Ports = append([]layered.LPortID(nil), copy.portOrders[layerIndex][nodeIndex]...)
			arena.InvalidatePortSideCache(nodeID)
			if setPortConstraints {
				constraints := basegraph.Get(&arena.Node(nodeID).Properties, layered.PortConstraintsOption)
				if !constraints.IsOrderFixed() {
					basegraph.Set(&arena.Node(nodeID).Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedOrder)
				}
			}
		}

		for _, dummyID := range northSouthDummies {
			originID, ok := basegraph.TryGet(&arena.Node(dummyID).Properties, layered.InLayerLayoutUnit)
			if !ok {
				continue
			}
			copy.assertCorrectPortSides(arena, dummyID, originID)
			for _, nodeID := range []layered.LNodeID{originID, dummyID} {
				if !seenUpdate[nodeID] {
					seenUpdate[nodeID] = true
					updatePortOrder = append(updatePortOrder, nodeID)
				}
			}
		}
	}

	for _, nodeID := range updatePortOrder {
		ports := arena.Node(nodeID).Ports
		sort.SliceStable(ports, func(i, j int) bool {
			return comparePortsCombined(arena, ports[i], ports[j]) < 0
		})
		arena.Node(nodeID).Ports = ports
		arena.CachePortSides(nodeID)
	}
}

func (copy *SweepCopy) assertCorrectPortSides(
	arena *layered.LGraphArena,
	dummyID, originID layered.LNodeID,
) {
	if len(arena.Node(dummyID).Ports) == 0 {
		return
	}
	dummyPortID := arena.Node(dummyID).Ports[0]
	origin, ok := basegraph.TryGet(&arena.Port(dummyPortID).Properties, layered.OriginalElement)
	if !ok || origin.Kind != layered.OriginLPort {
		return
	}
	for _, portID := range arena.Node(originID).Ports {
		if portID != origin.LPort {
			continue
		}
		port := arena.Port(portID)
		switch {
		case port.Side == layered.NorthSide && arena.Node(dummyID).ID > arena.Node(originID).ID:
			arena.SetPortSide(portID, layered.SouthSide)
			if port.ExplicitAnchor {
				port.Anchor.Y = port.Size.Y - port.Anchor.Y
			}
		case port.Side == layered.SouthSide && arena.Node(originID).ID > arena.Node(dummyID).ID:
			arena.SetPortSide(portID, layered.NorthSide)
			if port.ExplicitAnchor {
				port.Anchor.Y = -(port.Size.Y - port.Anchor.Y)
			}
		}
		return
	}
}

func cloneNodeOrder(order [][]layered.LNodeID) [][]layered.LNodeID {
	if order == nil {
		return nil
	}
	result := make([][]layered.LNodeID, len(order))
	for index := range order {
		result[index] = append([]layered.LNodeID(nil), order[index]...)
	}
	return result
}

func comparePortsCombined(arena *layered.LGraphArena, firstID, secondID layered.LPortID) int {
	first, second := arena.Port(firstID), arena.Port(secondID)
	if first.Side != second.Side {
		return cmp.Compare(first.Side, second.Side)
	}
	if first.Node == nil {
		return 0
	}
	constraints := basegraph.Get(&arena.Node(*first.Node).Properties, layered.PortConstraintsOption)
	if !constraints.IsOrderFixed() {
		return 0
	}
	if constraints == layered.PortConstraintsFixedOrder {
		firstIndex, firstPresent := rawInt(&first.Properties, "org.eclipse.elk.port.index")
		secondIndex, secondPresent := rawInt(&second.Properties, "org.eclipse.elk.port.index")
		if firstPresent && secondPresent && firstIndex != secondIndex {
			return cmp.Compare(firstIndex, secondIndex)
		}
	}
	switch first.Side {
	case layered.NorthSide:
		return compareFloat(first.Position.X, second.Position.X)
	case layered.EastSide:
		return compareFloat(first.Position.Y, second.Position.Y)
	case layered.SouthSide:
		return compareFloat(second.Position.X, first.Position.X)
	case layered.WestSide:
		return compareFloat(second.Position.Y, first.Position.Y)
	default:
		panic("p3order: fixed-order port has undefined side")
	}
}

func rawInt(properties rawPropertyMap, id string) (int, bool) {
	value, ok := properties.Raw(id)
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	default:
		return 0, false
	}
}

func compareFloat(first, second float64) int {
	if first < second {
		return -1
	}
	if first > second {
		return 1
	}
	return 0
}
