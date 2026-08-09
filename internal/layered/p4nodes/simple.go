// Copyright (c) 2010, 2015 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// SimpleNodePlacer stacks each layer and centers it around the tallest layer.
type SimpleNodePlacer struct{}

func (*SimpleNodePlacer) ProcessorConfiguration(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) layered.ProcessorConfiguration {
	var configuration layered.ProcessorConfiguration
	if basegraph.Get(&arena.Graph(graphID).Properties, layered.GraphProperties)&layered.GraphPropertyExternalPorts != 0 {
		configuration.AddBefore(layered.PhaseEdgeRouting, layered.HierarchicalPortPositionProcessor)
	}
	return configuration
}

func (*SimpleNodePlacer) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	maximumHeight := 0.0
	for _, layerID := range arena.Graph(graphID).Layers {
		layer := arena.Layer(layerID)
		layer.Size.Y = 0
		var previous *layered.LNodeID
		for _, nodeID := range layer.Nodes {
			node := arena.Node(nodeID)
			if previous != nil {
				layer.Size.Y += layered.VerticalSpacing(arena, nodeID, *previous)
			}
			layer.Size.Y += node.Margin.Top + node.Size.Y + node.Margin.Bottom
			copyID := nodeID
			previous = &copyID
		}
		if layer.Size.Y > maximumHeight {
			maximumHeight = layer.Size.Y
		}
	}
	for _, layerID := range arena.Graph(graphID).Layers {
		layer := arena.Layer(layerID)
		position := (maximumHeight - layer.Size.Y) / 2
		var previous *layered.LNodeID
		for _, nodeID := range layer.Nodes {
			node := arena.Node(nodeID)
			if previous != nil {
				position += layered.VerticalSpacing(arena, nodeID, *previous)
			}
			position += node.Margin.Top
			node.Position.Y = position
			position += node.Size.Y + node.Margin.Bottom
			copyID := nodeID
			previous = &copyID
		}
	}
	return nil
}
