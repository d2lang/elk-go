// Copyright (c) 2014, 2015 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"math"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// InteractiveNodePlacer keeps existing vertical coordinates where possible
// and shifts nodes down only enough to eliminate overlaps.
type InteractiveNodePlacer struct{}

func (*InteractiveNodePlacer) ProcessorConfiguration(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) layered.ProcessorConfiguration {
	var configuration layered.ProcessorConfiguration
	if basegraph.Get(&arena.Graph(graphID).Properties, layered.GraphProperties)&layered.GraphPropertyExternalPorts != 0 {
		configuration.AddBefore(layered.PhaseEdgeRouting, layered.HierarchicalPortPositionProcessor)
	}
	return configuration
}

func (*InteractiveNodePlacer) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	for _, layerID := range arena.Graph(graphID).Layers {
		minimumValidY := math.Inf(-1)
		previousType := layered.NormalNode
		for _, nodeID := range arena.Layer(layerID).Nodes {
			node := arena.Node(nodeID)
			nodeType := node.Type
			if nodeType != layered.NormalNode {
				original, found := basegraph.TryGet(&node.Properties, layered.OriginalDummyNodePosition)
				if found {
					node.Position.Y = original
				} else {
					minimumValidY = math.Max(minimumValidY, 0)
					node.Position.Y = minimumValidY + layered.VerticalSpacingByType(arena, graphID, nodeType, previousType)
				}
			}
			spacing := layered.VerticalSpacingByType(arena, graphID, nodeType, previousType)
			minimumPosition := minimumValidY + spacing + node.Margin.Top
			if node.Position.Y < minimumPosition {
				node.Position.Y = minimumPosition
			}
			minimumValidY = node.Position.Y + node.Size.Y + node.Margin.Bottom
			previousType = nodeType
		}
	}
	return nil
}
