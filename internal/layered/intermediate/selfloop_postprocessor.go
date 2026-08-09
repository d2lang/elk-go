// Copyright (c) 2018, 2019 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/layered/intermediate/loops"
)

// PostprocessSelfLoops reattaches the routed edges and translates node-local
// bend points and labels into graph coordinates.
func PostprocessSelfLoops(arena *layered.LGraphArena, graphID layered.LGraphID) {
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			node := arena.Node(nodeID)
			if node.Type != layered.NormalNode {
				continue
			}
			holder, found := basegraph.TryGet(&node.Properties, loops.HolderProperty)
			if !found {
				continue
			}
			for _, loop := range holder.HyperLoops {
				for _, edge := range loop.Edges {
					source, target := edge.Source.LPort, edge.Target.LPort
					arena.SetEdgeSource(edge.LEdge, &source)
					arena.SetEdgeTarget(edge.LEdge, &target)
					arena.Edge(edge.LEdge).BendPoints.Offset(node.Position)
				}
				if loop.Labels != nil {
					loop.Labels.ApplyPlacement(arena, node.Position)
				}
			}
		}
	}
}
