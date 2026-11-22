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
//
// All layers and port lists live in three flat arrays that are reused when a
// copy is refilled; every sub-slice has its capacity capped at its length.
type SweepCopy struct {
	nodeOrder  [][]layered.LNodeID
	portOrders [][][]layered.LPortID

	nodes     []layered.LNodeID
	ports     []layered.LPortID
	nodePorts [][]layered.LPortID

	ws *crossMinWorkspace // storage for the arrays (nil: allocated)
}

func NewSweepCopy(arena *layered.LGraphArena, nodeOrder [][]layered.LNodeID) *SweepCopy {
	copy := &SweepCopy{}
	copy.fill(arena, nodeOrder)
	return copy
}

// fill makes copy a snapshot of nodeOrder and the nodes' current port lists.
func (copy *SweepCopy) fill(arena *layered.LGraphArena, nodeOrder [][]layered.LNodeID) {
	totalNodes, totalPorts := 0, 0
	for _, nodes := range nodeOrder {
		totalNodes += len(nodes)
		for _, nodeID := range nodes {
			totalPorts += len(arena.Node(nodeID).Ports)
		}
	}
	copy.reserve(len(nodeOrder), totalNodes, totalPorts)
	nodeOffset, portOffset := 0, 0
	for layerIndex, nodes := range nodeOrder {
		layerNodes := copy.nodes[nodeOffset : nodeOffset+len(nodes) : nodeOffset+len(nodes)]
		builtinCopy(layerNodes, nodes)
		layerPorts := copy.nodePorts[nodeOffset : nodeOffset+len(nodes) : nodeOffset+len(nodes)]
		for nodeIndex, nodeID := range nodes {
			ports := arena.Node(nodeID).Ports
			saved := copy.ports[portOffset : portOffset+len(ports) : portOffset+len(ports)]
			builtinCopyPorts(saved, ports)
			layerPorts[nodeIndex] = saved
			portOffset += len(ports)
		}
		copy.nodeOrder[layerIndex] = layerNodes
		copy.portOrders[layerIndex] = layerPorts
		nodeOffset += len(nodes)
	}
}

// fillFrom makes copy an independent duplicate of other.
func (copy *SweepCopy) fillFrom(other *SweepCopy) {
	totalNodes, totalPorts := 0, 0
	for layerIndex, nodes := range other.nodeOrder {
		totalNodes += len(nodes)
		for nodeIndex := range other.portOrders[layerIndex] {
			totalPorts += len(other.portOrders[layerIndex][nodeIndex])
		}
	}
	copy.reserve(len(other.nodeOrder), totalNodes, totalPorts)
	nodeOffset, portOffset := 0, 0
	for layerIndex, nodes := range other.nodeOrder {
		layerNodes := copy.nodes[nodeOffset : nodeOffset+len(nodes) : nodeOffset+len(nodes)]
		builtinCopy(layerNodes, nodes)
		otherPorts := other.portOrders[layerIndex]
		layerPorts := copy.nodePorts[nodeOffset : nodeOffset+len(otherPorts) : nodeOffset+len(otherPorts)]
		for nodeIndex, ports := range otherPorts {
			saved := copy.ports[portOffset : portOffset+len(ports) : portOffset+len(ports)]
			builtinCopyPorts(saved, ports)
			layerPorts[nodeIndex] = saved
			portOffset += len(ports)
		}
		copy.nodeOrder[layerIndex] = layerNodes
		copy.portOrders[layerIndex] = layerPorts
		nodeOffset += len(nodes)
	}
}

// reserve gives the copy arrays of the given lengths; fill and fillFrom
// overwrite every element before anything reads it.
func (copy *SweepCopy) reserve(layers, nodes, ports int) {
	if ws := copy.ws; ws != nil {
		if cap(copy.nodeOrder) < layers {
			copy.nodeOrder = ws.copyLayers.take(layers)
			copy.portOrders = ws.copyPortLayers.take(layers)
		}
		if cap(copy.nodes) < nodes {
			copy.nodes = ws.copyNodes.take(nodes)
			copy.nodePorts = ws.copyPortLists.take(nodes)
		}
		if cap(copy.ports) < ports {
			copy.ports = ws.copyPorts.take(ports)
		}
	} else {
		if cap(copy.nodeOrder) < layers {
			copy.nodeOrder = make([][]layered.LNodeID, layers)
			copy.portOrders = make([][][]layered.LPortID, layers)
		}
		if cap(copy.nodes) < nodes {
			copy.nodes = make([]layered.LNodeID, nodes)
			copy.nodePorts = make([][]layered.LPortID, nodes)
		}
		if cap(copy.ports) < ports {
			copy.ports = make([]layered.LPortID, ports)
		}
	}
	copy.nodeOrder, copy.portOrders = copy.nodeOrder[:layers], copy.portOrders[:layers]
	copy.nodes, copy.nodePorts = copy.nodes[:nodes], copy.nodePorts[:nodes]
	copy.ports = copy.ports[:ports]
}

func builtinCopy(destination, source []layered.LNodeID)      { copy(destination, source) }
func builtinCopyPorts(destination, source []layered.LPortID) { copy(destination, source) }

func (copy *SweepCopy) Clone() *SweepCopy {
	if copy == nil {
		return nil
	}
	result := &SweepCopy{}
	result.fillFrom(copy)
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
	// The graph's layer list is not changed below (only the layers' node
	// lists and the nodes), so it is walked in place. Every node gets a new
	// port list, as before: the lists are cut from one array, each with its
	// capacity capped at its length, so no two lists share an element (an
	// empty list stays nil, as append left it).
	layers := arena.Graph(graphID).Layers
	total := 0
	for layerIndex, layerID := range layers {
		for nodeIndex := range arena.Layer(layerID).Nodes {
			total += len(copy.portOrders[layerIndex][nodeIndex])
		}
	}
	backing := make([]layered.LPortID, total)
	for layerIndex, layerID := range layers {
		northSouthDummies = northSouthDummies[:0]
		for nodeIndex := range arena.Layer(layerID).Nodes {
			nodeID := copy.nodeOrder[layerIndex][nodeIndex]
			arena.Node(nodeID).ID = nodeIndex
			if arena.Node(nodeID).Type == layered.NorthSouthPortNode {
				northSouthDummies = append(northSouthDummies, nodeID)
			}
			arena.Layer(layerID).Nodes[nodeIndex] = nodeID
			var ports []layered.LPortID
			if saved := copy.portOrders[layerIndex][nodeIndex]; len(saved) > 0 {
				ports = backing[:len(saved):len(saved)]
				backing = backing[len(saved):]
				builtinCopyPorts(ports, saved)
			}
			arena.Node(nodeID).Ports = ports
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
		firstIndex, firstPresent := rawInt(&first.Properties, portIndexKey)
		secondIndex, secondPresent := rawInt(&second.Properties, portIndexKey)
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

var portIndexKey = basegraph.InternPropertyKey("org.eclipse.elk.port.index")

func rawInt(properties *basegraph.PropertyMap, key basegraph.PropertyKey) (int, bool) {
	value, ok := properties.RawKey(key)
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
