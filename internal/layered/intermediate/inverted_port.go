// Copyright (c) 2011, 2019 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// ProcessInvertedPorts inserts same-layer long-edge dummies for input ports on
// the east side and output ports on the west side.
func ProcessInvertedPorts(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	layers := append([]layered.LayerID(nil), arena.Graph(graphID).Layers...)
	var currentLayer *layered.LayerID
	unassigned := make([]layered.LNodeID, 0)
	for _, layerID := range layers {
		previousLayer := currentLayer
		layerCopy := layerID
		currentLayer = &layerCopy
		for _, nodeID := range unassigned {
			arena.SetNodeLayer(nodeID, previousLayer)
		}
		unassigned = unassigned[:0]

		nodes := append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...)
		for _, nodeID := range nodes {
			node := arena.Node(nodeID)
			if node.Type != layered.NormalNode ||
				!basegraph.Get(&node.Properties, layered.PortConstraintsOption).IsSideFixed() {
				continue
			}
			ports := append([]layered.LPortID(nil), node.Ports...)
			for _, portID := range ports {
				port := arena.Port(portID)
				if port.Side != layered.EastSide || len(port.IncomingEdges) == 0 {
					continue
				}
				for _, edgeID := range append([]layered.LEdgeID(nil), port.IncomingEdges...) {
					createEastInvertedPortDummy(arena, graphID, portID, edgeID, &unassigned)
				}
			}
			for _, portID := range ports {
				port := arena.Port(portID)
				if port.Side != layered.WestSide || len(port.OutgoingEdges) == 0 {
					continue
				}
				for _, edgeID := range append([]layered.LEdgeID(nil), port.OutgoingEdges...) {
					createWestInvertedPortDummy(arena, graphID, portID, edgeID, &unassigned)
				}
			}
		}
	}
	for _, nodeID := range unassigned {
		arena.SetNodeLayer(nodeID, currentLayer)
	}
	return nil
}

func createEastInvertedPortDummy(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	eastPort layered.LPortID,
	edgeID layered.LEdgeID,
	unassigned *[]layered.LNodeID,
) {
	edge := arena.Edge(edgeID)
	if edge.Source == nil || edge.Target == nil || *edge.Target != eastPort ||
		arena.EdgeSourceNode(edgeID) == *arena.Port(eastPort).Node {
		return
	}
	dummy, input, output := newInvertedPortDummy(arena, graphID, edgeID)
	*unassigned = append(*unassigned, dummy)
	arena.SetEdgeTarget(edgeID, &input)
	dummyEdge := copyInvertedDummyEdge(arena, edgeID)
	arena.SetEdgeSource(dummyEdge, &output)
	arena.SetEdgeTarget(dummyEdge, &eastPort)
	setInvertedLongEdgeEndpoints(arena, dummy, input, output)
	moveInvertedPortHeadLabels(arena, edgeID, dummyEdge, false)
}

func createWestInvertedPortDummy(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	westPort layered.LPortID,
	edgeID layered.LEdgeID,
	unassigned *[]layered.LNodeID,
) {
	edge := arena.Edge(edgeID)
	if edge.Source == nil || edge.Target == nil || *edge.Source != westPort ||
		arena.EdgeTargetNode(edgeID) == *arena.Port(westPort).Node {
		return
	}
	originalTarget := *edge.Target
	dummy, input, output := newInvertedPortDummy(arena, graphID, edgeID)
	*unassigned = append(*unassigned, dummy)
	arena.SetEdgeTarget(edgeID, &input)
	dummyEdge := copyInvertedDummyEdge(arena, edgeID)
	arena.SetEdgeSource(dummyEdge, &output)
	arena.SetEdgeTarget(dummyEdge, &originalTarget)
	moveInvertedPortHeadLabels(arena, edgeID, dummyEdge, true)
	setInvertedLongEdgeEndpoints(arena, dummy, input, output)
}

func newInvertedPortDummy(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	edgeID layered.LEdgeID,
) (layered.LNodeID, layered.LPortID, layered.LPortID) {
	dummy := arena.AddNode(graphID)
	arena.Node(dummy).Type = layered.LongEdgeNode
	basegraph.Set(&arena.Node(dummy).Properties, layered.OriginalElement, layered.LEdgeOrigin(edgeID))
	basegraph.Set(&arena.Node(dummy).Properties, layered.PortConstraintsOption,
		layered.PortConstraintsFixedPosition)
	input := arena.AddPort()
	arena.SetPortNode(input, &dummy)
	arena.SetPortSide(input, layered.WestSide)
	output := arena.AddPort()
	arena.SetPortNode(output, &dummy)
	arena.SetPortSide(output, layered.EastSide)
	return dummy, input, output
}

func copyInvertedDummyEdge(arena *layered.LGraphArena, original layered.LEdgeID) layered.LEdgeID {
	dummy := arena.AddEdge()
	arena.Edge(dummy).Properties.CopyFrom(&arena.Edge(original).Properties)
	basegraph.Unset(&arena.Edge(dummy).Properties, layered.JunctionPoints)
	return dummy
}

func moveInvertedPortHeadLabels(
	arena *layered.LGraphArena,
	original, dummy layered.LEdgeID,
	requireFreshOrigin bool,
) {
	remaining := make([]layered.LLabelID, 0, len(arena.Edge(original).Labels))
	for _, labelID := range arena.Edge(original).Labels {
		if basegraph.Get(&arena.Label(labelID).Properties, layered.EdgeLabelsPlacement) !=
			layered.EdgeLabelPlacementHead {
			remaining = append(remaining, labelID)
			continue
		}
		if _, found := basegraph.TryGet(&arena.Label(labelID).Properties, layered.EndLabelEdge); !found || requireFreshOrigin {
			basegraph.Set(&arena.Label(labelID).Properties, layered.EndLabelEdge, original)
		}
		arena.Edge(dummy).Labels = append(arena.Edge(dummy).Labels, labelID)
	}
	arena.Edge(original).Labels = remaining
}

func setInvertedLongEdgeEndpoints(
	arena *layered.LGraphArena,
	dummy layered.LNodeID,
	input, output layered.LPortID,
) {
	if len(arena.Port(input).IncomingEdges) == 0 || len(arena.Port(output).OutgoingEdges) == 0 {
		return
	}
	sourcePort := *arena.Edge(arena.Port(input).IncomingEdges[0]).Source
	targetPort := *arena.Edge(arena.Port(output).OutgoingEdges[0]).Target
	sourceNode := *arena.Port(sourcePort).Node
	targetNode := *arena.Port(targetPort).Node
	if arena.Node(sourceNode).Type == layered.LongEdgeNode {
		if value, found := basegraph.TryGet(&arena.Node(sourceNode).Properties, layered.LongEdgeSource); found {
			basegraph.Set(&arena.Node(dummy).Properties, layered.LongEdgeSource, value)
		} else {
			basegraph.Unset(&arena.Node(dummy).Properties, layered.LongEdgeSource)
		}
	} else {
		basegraph.Set(&arena.Node(dummy).Properties, layered.LongEdgeSource, sourcePort)
	}
	if arena.Node(targetNode).Type == layered.LongEdgeNode {
		if value, found := basegraph.TryGet(&arena.Node(targetNode).Properties, layered.LongEdgeTarget); found {
			basegraph.Set(&arena.Node(dummy).Properties, layered.LongEdgeTarget, value)
		} else {
			basegraph.Unset(&arena.Node(dummy).Properties, layered.LongEdgeTarget)
		}
	} else {
		basegraph.Set(&arena.Node(dummy).Properties, layered.LongEdgeTarget, targetPort)
	}
}
