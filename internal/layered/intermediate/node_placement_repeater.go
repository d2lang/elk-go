// Copyright (c) 2026 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// PrepareNodePlacementRepeat fixes the node sizes produced by network-simplex
// flexibility, disables flexibility for the second run, and recomputes ports,
// labels, and margins for those fixed sizes. The selected phase-4 placer is
// invoked by layeredlayout to keep the phase packages acyclic.
func PrepareNodePlacementRepeat(arena *layered.LGraphArena, graphID layered.LGraphID) {
	graph := arena.Graph(graphID)
	basegraph.Unset(&graph.Properties, layered.NetworkSimplexNodeFlexibilityDefault)
	for _, layerID := range graph.Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			node := arena.Node(nodeID)
			basegraph.Unset(&node.Properties, layered.NetworkSimplexNodeFlexibility)
			basegraph.Unset(&node.Properties, layered.NodeSizeConstraints)
			// The source node-size calculator leaves FIXED_ORDER unchanged.
			// Our first sizing pass promotes it to FIXED_POS for the arena
			// pipeline, so restore the imported constraint before repeating the
			// source calculation.
			basegraph.Set(
				&node.Properties,
				layered.PortConstraintsOption,
				basegraph.Get(&node.Properties, layered.OriginalPortConstraints),
			)
			node.Position.Y = 0
		}
	}

	ProcessLabelAndNodeSizes(arena, graphID)
	CalculateInnermostNodeMargins(arena, graphID)
}
