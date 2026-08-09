// Copyright (c) 2016 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package wrapping

import (
	"math"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// InsertDummies inserts the two in-layer and intervening long-edge dummies
// used to route one wrapping edge back across the drawing.
func InsertDummies(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	originalEdge layered.LEdgeID,
	offsetFirstInLayerDummy int,
) []layered.LEdgeID {
	graph := arena.Graph(graphID)
	individual := layered.IndividualSpacings{}
	basegraph.Set(&individual.Properties, layered.SpacingEdgeNode,
		basegraph.Get(&graph.Properties, layered.SpacingEdgeNode)+
			basegraph.Get(&graph.Properties, layered.WrappingAdditionalEdgeSpacing))

	edgeID := originalEdge
	targetPort := *arena.Edge(edgeID).Target
	sourceNode := arena.EdgeSourceNode(edgeID)
	targetNode := arena.EdgeTargetNode(edgeID)
	sourceIndex := arena.LayerIndex(*arena.Node(sourceNode).Layer)
	targetIndex := arena.LayerIndex(*arena.Node(targetNode).Layer)
	created := make([]layered.LEdgeID, 0, targetIndex-sourceIndex+1)

	for index := sourceIndex; index <= targetIndex; index++ {
		layerID := graph.Layers[index]
		dummyID := arena.AddNode(graphID)
		dummy := arena.Node(dummyID)
		dummy.Type = layered.LongEdgeNode
		basegraph.Set(&dummy.Properties, layered.OriginalElement, layered.LEdgeOrigin(edgeID))
		basegraph.Set(&dummy.Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedPosition)
		basegraph.Set(&dummy.Properties, layered.SpacingIndividual, individual)
		if index == sourceIndex {
			arena.SetNodeLayerAt(dummyID, &layerID, len(arena.Layer(layerID).Nodes)-offsetFirstInLayerDummy)
		} else {
			arena.SetNodeLayer(dummyID, &layerID)
		}

		thickness := basegraph.Get(&arena.Edge(edgeID).Properties, layered.EdgeThickness)
		if thickness < 0 {
			thickness = 0
			basegraph.Set(&arena.Edge(edgeID).Properties, layered.EdgeThickness, thickness)
		}
		dummy.Size.Y = thickness
		portPosition := math.Floor(thickness / 2)

		input := arena.AddPort()
		arena.SetPortSide(input, layered.WestSide)
		arena.SetPortNode(input, &dummyID)
		arena.Port(input).Position.Y = portPosition
		output := arena.AddPort()
		arena.SetPortSide(output, layered.EastSide)
		arena.SetPortNode(output, &dummyID)
		arena.Port(output).Position.Y = portPosition

		arena.SetEdgeTarget(edgeID, &input)
		newEdgeID := arena.AddEdge()
		arena.Edge(newEdgeID).Properties.CopyFrom(&arena.Edge(edgeID).Properties)
		basegraph.Unset(&arena.Edge(newEdgeID).Properties, layered.JunctionPoints)
		arena.SetEdgeSource(newEdgeID, &output)
		arena.SetEdgeTarget(newEdgeID, &targetPort)
		setDummyProperties(arena, dummyID, edgeID, newEdgeID)
		created = append(created, newEdgeID)
		edgeID = newEdgeID
	}
	return created
}

func setDummyProperties(
	arena *layered.LGraphArena,
	dummyID layered.LNodeID,
	inEdgeID layered.LEdgeID,
	outEdgeID layered.LEdgeID,
) {
	sourceNode := arena.EdgeSourceNode(inEdgeID)
	if arena.Node(sourceNode).Type == layered.LongEdgeNode {
		if source, found := basegraph.TryGet(&arena.Node(sourceNode).Properties, layered.LongEdgeSource); found {
			basegraph.Set(&arena.Node(dummyID).Properties, layered.LongEdgeSource, source)
		}
		if target, found := basegraph.TryGet(&arena.Node(sourceNode).Properties, layered.LongEdgeTarget); found {
			basegraph.Set(&arena.Node(dummyID).Properties, layered.LongEdgeTarget, target)
		}
		return
	}
	basegraph.Set(&arena.Node(dummyID).Properties, layered.LongEdgeSource, *arena.Edge(inEdgeID).Source)
	basegraph.Set(&arena.Node(dummyID).Properties, layered.LongEdgeTarget, *arena.Edge(outEdgeID).Target)
}
