// Copyright (c) 2011, 2015 Kiel University and others.
// Copyright (c) 2010, 2020 Kiel University and others.
// Copyright (c) 2022 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package components ports ELK Layered 0.8.1's connected-components split
// and packing stage.
package components

import (
	"math"
	"sort"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// ComponentsProcessor splits an unlayered graph into connected components
// and combines the fully laid-out components again.
type ComponentsProcessor struct {
	graphPlacer graphPlacer
}

func NewComponentsProcessor() *ComponentsProcessor {
	return &ComponentsProcessor{graphPlacer: &simpleRowGraphPlacer{}}
}

func (processor *ComponentsProcessor) Split(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) ([]layered.LGraphID, error) {
	processor.graphPlacer = &simpleRowGraphPlacer{}
	graph := arena.Graph(graphID)
	separate := basegraph.Get(&graph.Properties, layered.SeparateComponents)
	hasExternalPorts := basegraph.Get(&graph.Properties, layered.GraphProperties)&
		layered.GraphPropertyExternalPorts != 0
	compatiblePortConstraints := !basegraph.Get(
		&graph.Properties, layered.PortConstraintsOption).IsOrderFixed()
	if !separate || hasExternalPorts && !compatiblePortConstraints {
		return []layered.LGraphID{graphID}, nil
	}

	for _, nodeID := range graph.LayerlessNodes {
		arena.Node(nodeID).ID = 0
	}
	components := make([]layered.LGraphID, 0)
	for _, nodeID := range graph.LayerlessNodes {
		nodes, externalSides := depthFirstComponent(arena, nodeID, nil, 0)
		if nodes == nil {
			continue
		}
		componentID := arena.AddGraph()
		component := arena.Graph(componentID)
		component.Properties = graph.Properties.Clone()
		basegraph.Set(&component.Properties, layered.ExternalPortConnections,
			externalSides)
		component.Padding = graph.Padding
		basegraph.Unset(&component.Properties, layered.NodeSizeMinimum)
		component.LayerlessNodes = append(component.LayerlessNodes, nodes...)
		for _, member := range nodes {
			arena.Node(member).Graph = componentID
		}
		components = append(components, componentID)
	}
	if hasExternalPorts {
		switch basegraph.Get(&graph.Properties, layered.ComponentOrdering) {
		case layered.ComponentOrderingGroupModelOrder:
			processor.graphPlacer = &componentGroupModelOrderGraphPlacer{}
		case layered.ComponentOrderingModelOrder:
			processor.graphPlacer = &simpleRowGraphPlacer{modelOrder: true}
		default:
			processor.graphPlacer = &componentGroupGraphPlacer{}
		}
	}

	if basegraph.Get(&graph.Properties, layered.ComponentOrdering) != layered.ComponentOrderingNone {
		sort.SliceStable(components, func(left, right int) bool {
			return minimalModelOrder(arena, components[left]) <
				minimalModelOrder(arena, components[right])
		})
	}
	return components, nil
}

func (processor *ComponentsProcessor) Combine(
	arena *layered.LGraphArena,
	components []layered.LGraphID,
	target layered.LGraphID,
) error {
	if processor.graphPlacer == nil {
		processor.graphPlacer = &simpleRowGraphPlacer{}
	}
	return processor.graphPlacer.combine(arena, components, target)
}

func depthFirstComponent(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	nodes []layered.LNodeID,
	externalSides uint8,
) ([]layered.LNodeID, uint8) {
	node := arena.Node(nodeID)
	if node.ID != 0 {
		return nodes, externalSides
	}
	node.ID = 1
	nodes = append(nodes, nodeID)
	if node.Type == layered.ExternalPortNode {
		side := basegraph.Get(&node.Properties, layered.ExternalPortSide)
		externalSides |= uint8(1) << uint8(side)
	}
	for _, portID := range node.Ports {
		port := arena.Port(portID)
		// LPort.getConnectedPorts concatenates predecessors and successors.
		for _, edgeID := range port.IncomingEdges {
			edge := arena.Edge(edgeID)
			if edge.Source == nil || arena.Port(*edge.Source).Node == nil {
				continue
			}
			nodes, externalSides = depthFirstComponent(
				arena, *arena.Port(*edge.Source).Node, nodes, externalSides)
		}
		for _, edgeID := range port.OutgoingEdges {
			edge := arena.Edge(edgeID)
			if edge.Target == nil || arena.Port(*edge.Target).Node == nil {
				continue
			}
			nodes, externalSides = depthFirstComponent(
				arena, *arena.Port(*edge.Target).Node, nodes, externalSides)
		}
	}
	return nodes, externalSides
}

func minimalModelOrder(arena *layered.LGraphArena, graphID layered.LGraphID) int {
	order := math.MaxInt
	for _, nodeID := range arena.Graph(graphID).LayerlessNodes {
		if modelOrder, found := basegraph.TryGet(
			&arena.Node(nodeID).Properties, layered.ModelOrder,
		); found && modelOrder < order {
			order = modelOrder
		}
	}
	return order
}

// MakeLayerless reproduces ElkLayered.layout's final layer cleanup. Component
// placement requires every laid-out graph to be unlayered again.
func MakeLayerless(arena *layered.LGraphArena, graphID layered.LGraphID) {
	graph := arena.Graph(graphID)
	for _, layerID := range graph.Layers {
		layer := arena.Layer(layerID)
		graph.LayerlessNodes = append(graph.LayerlessNodes, layer.Nodes...)
		layer.Nodes = nil
	}
	for _, nodeID := range graph.LayerlessNodes {
		arena.Node(nodeID).Layer = nil
	}
	graph.Layers = nil
}
