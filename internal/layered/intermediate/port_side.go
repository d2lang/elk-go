// Copyright (c) 2011, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package intermediate contains the processors run between ELK Layered's five
// main phases.
package intermediate

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// ProcessPortSides makes every port side-fixed. It is the direct counterpart
// of ELK 0.8.1's PortSideProcessor and deliberately visits layerless nodes
// before layered nodes.
func ProcessPortSides(arena *layered.LGraphArena, graphID layered.LGraphID) {
	graph := arena.Graph(graphID)
	for _, nodeID := range graph.LayerlessNodes {
		processNodePortSides(arena, nodeID)
	}
	for _, layerID := range graph.Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			processNodePortSides(arena, nodeID)
		}
	}
}

func processNodePortSides(arena *layered.LGraphArena, nodeID layered.LNodeID) {
	node := arena.Node(nodeID)
	constraints := basegraph.Get(&node.Properties, layered.PortConstraintsOption)
	if constraints.IsSideFixed() {
		for _, portID := range node.Ports {
			if arena.Port(portID).Side == layered.UndefinedSide {
				SetPortSide(arena, portID)
			}
		}
		return
	}
	for _, portID := range node.Ports {
		SetPortSide(arena, portID)
	}
	basegraph.Set(&node.Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedSide)
}

// SetPortSide places input ports west and output ports east. A port connected
// to an external-port dummy inherits that dummy's side instead.
func SetPortSide(arena *layered.LGraphArena, portID layered.LPortID) {
	port := arena.Port(portID)
	if dummyID, found := basegraph.TryGet(&port.Properties, layered.PortDummy); found {
		arena.SetPortSide(portID, basegraph.Get(&arena.Node(dummyID).Properties, layered.ExternalPortSide))
		return
	}
	if len(port.IncomingEdges)-len(port.OutgoingEdges) < 0 {
		arena.SetPortSide(portID, layered.EastSide)
	} else {
		arena.SetPortSide(portID, layered.WestSide)
	}
}
