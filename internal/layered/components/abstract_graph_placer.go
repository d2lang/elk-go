// Copyright (c) 2012, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package components

import (
	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// graphPlacer is the arena-backed counterpart of AbstractGraphPlacer.
type graphPlacer interface {
	combine(*layered.LGraphArena, []layered.LGraphID, layered.LGraphID) error
}

func moveGraphs(
	arena *layered.LGraphArena,
	destination layered.LGraphID,
	sources []layered.LGraphID,
	offset geometry.Point,
) {
	for _, source := range sources {
		moveGraph(arena, destination, source, offset)
	}
}

// moveGraph applies both the supplied offset and the source graph's own
// offset. Like the Java method, it must only be called once for a source.
func moveGraph(
	arena *layered.LGraphArena,
	destination, source layered.LGraphID,
	offset geometry.Point,
) {
	sourceGraph := arena.Graph(source)
	graphOffset := sourceGraph.Offset.Add(offset)
	for _, nodeID := range sourceGraph.LayerlessNodes {
		offsetNodeAndOutgoingEdges(arena, nodeID, graphOffset)
		arena.Graph(destination).LayerlessNodes = append(
			arena.Graph(destination).LayerlessNodes, nodeID)
		arena.Node(nodeID).Graph = destination
	}
}

func offsetGraph(arena *layered.LGraphArena, graphID layered.LGraphID, offset geometry.Point) {
	for _, nodeID := range arena.Graph(graphID).LayerlessNodes {
		offsetNodeAndOutgoingEdges(arena, nodeID, offset)
	}
}

func offsetNodeAndOutgoingEdges(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	offset geometry.Point,
) {
	node := arena.Node(nodeID)
	node.Position = node.Position.Add(offset)
	for _, portID := range node.Ports {
		for _, edgeID := range arena.Port(portID).OutgoingEdges {
			edge := arena.Edge(edgeID)
			edge.BendPoints = edge.BendPoints.Offset(offset)
			if junctionPoints, found := basegraph.TryGet(
				&edge.Properties, layered.JunctionPoints,
			); found {
				basegraph.Set(&edge.Properties, layered.JunctionPoints,
					junctionPoints.Offset(offset))
			}
			for _, labelID := range edge.Labels {
				label := arena.Label(labelID)
				label.Position = label.Position.Add(offset)
			}
		}
	}
}
