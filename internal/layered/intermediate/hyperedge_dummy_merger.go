// Copyright (c) 2010, 2018 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"fmt"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// HyperedgeDummyMerger joins adjacent long-edge dummies that belong to the
// same hyperedge, subject to the source/target label-dummy restrictions from
// ELK 0.8.1.
type HyperedgeDummyMerger struct{}

func (*HyperedgeDummyMerger) Process(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) error {
	identifyHyperedges(arena, graphID)
	for _, layerID := range arena.Graph(graphID).Layers {
		nodes := arena.Layer(layerID).Nodes
		if len(nodes) == 0 {
			continue
		}
		var lastNode layered.LNodeID
		var lastType layered.NodeType
		hasLast := false
		for nodeIndex := 0; nodeIndex < len(nodes); nodeIndex++ {
			currentNode := nodes[nodeIndex]
			currentType := arena.Node(currentNode).Type
			if currentType == layered.LongEdgeNode && hasLast && lastType == layered.LongEdgeNode {
				state, err := checkHyperedgeMergeAllowed(arena, currentNode, lastNode)
				if err != nil {
					return err
				}
				if state.allowMerge {
					if err := mergeHyperedgeDummyNodes(
						arena, currentNode, lastNode, state.sameSource, state.sameTarget,
					); err != nil {
						return err
					}
					nodes = append(nodes[:nodeIndex], nodes[nodeIndex+1:]...)
					arena.Layer(layerID).Nodes = nodes
					nodeIndex--
					currentNode = lastNode
					currentType = lastType
				}
			}
			lastNode, lastType, hasLast = currentNode, currentType, true
		}
	}
	return nil
}

// MergeHyperedgeDummies is the function-shaped API used by the intermediate
// processor dispatcher.
func MergeHyperedgeDummies(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	return (&HyperedgeDummyMerger{}).Process(arena, graphID)
}

type hyperedgeMergeState struct {
	allowMerge bool
	sameSource bool
	sameTarget bool
}

func checkHyperedgeMergeAllowed(
	arena *layered.LGraphArena,
	currentNode, lastNode layered.LNodeID,
) (hyperedgeMergeState, error) {
	current := arena.Node(currentNode)
	last := arena.Node(lastNode)
	if len(current.Ports) == 0 || len(last.Ports) == 0 {
		return hyperedgeMergeState{}, fmt.Errorf("layered: long-edge dummy has no ports")
	}
	currentSource, currentHasSource := basegraph.TryGet(&current.Properties, layered.LongEdgeSource)
	lastSource, lastHasSource := basegraph.TryGet(&last.Properties, layered.LongEdgeSource)
	currentTarget, currentHasTarget := basegraph.TryGet(&current.Properties, layered.LongEdgeTarget)
	lastTarget, lastHasTarget := basegraph.TryGet(&last.Properties, layered.LongEdgeTarget)
	sameSource := currentHasSource && lastHasSource && currentSource == lastSource
	sameTarget := currentHasTarget && lastHasTarget && currentTarget == lastTarget
	currentHasLabels := basegraph.Get(&current.Properties, layered.LongEdgeHasLabelDummies)
	lastHasLabels := basegraph.Get(&last.Properties, layered.LongEdgeHasLabelDummies)
	if !currentHasLabels && !lastHasLabels {
		return hyperedgeMergeState{
			allowMerge: arena.Port(current.Ports[0]).ID == arena.Port(last.Ports[0]).ID,
			sameSource: sameSource,
			sameTarget: sameTarget,
		}, nil
	}
	eligibleForSource := (!currentHasLabels ||
		basegraph.Get(&current.Properties, layered.LongEdgeBeforeLabelDummy)) &&
		(!lastHasLabels || basegraph.Get(&last.Properties, layered.LongEdgeBeforeLabelDummy))
	eligibleForTarget := (!currentHasLabels ||
		!basegraph.Get(&current.Properties, layered.LongEdgeBeforeLabelDummy)) &&
		(!lastHasLabels || !basegraph.Get(&last.Properties, layered.LongEdgeBeforeLabelDummy))
	return hyperedgeMergeState{
		allowMerge: sameSource && eligibleForSource || sameTarget && eligibleForTarget,
		sameSource: sameSource,
		sameTarget: sameTarget,
	}, nil
}

func mergeHyperedgeDummyNodes(
	arena *layered.LGraphArena,
	mergeSource, mergeTarget layered.LNodeID,
	keepSource, keepTarget bool,
) error {
	inputs := arena.NodePortsOnSide(mergeTarget, layered.WestSide)
	outputs := arena.NodePortsOnSide(mergeTarget, layered.EastSide)
	if len(inputs) == 0 || len(outputs) == 0 {
		return fmt.Errorf("layered: long-edge merge target %d has no west/east ports", mergeTarget)
	}
	input, output := inputs[0], outputs[0]
	for _, portID := range arena.Node(mergeSource).Ports {
		for len(arena.Port(portID).IncomingEdges) > 0 {
			edgeID := arena.Port(portID).IncomingEdges[0]
			arena.SetEdgeTarget(edgeID, &input)
		}
		for len(arena.Port(portID).OutgoingEdges) > 0 {
			edgeID := arena.Port(portID).OutgoingEdges[0]
			arena.SetEdgeSource(edgeID, &output)
		}
	}
	if !keepSource {
		basegraph.Unset(&arena.Node(mergeTarget).Properties, layered.LongEdgeSource)
	}
	if !keepTarget {
		basegraph.Unset(&arena.Node(mergeTarget).Properties, layered.LongEdgeTarget)
	}
	return nil
}

func identifyHyperedges(arena *layered.LGraphArena, graphID layered.LGraphID) {
	ports := make([]layered.LPortID, 0)
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			ports = append(ports, arena.Node(nodeID).Ports...)
		}
	}
	for _, portID := range ports {
		arena.Port(portID).ID = -1
	}
	hyperedgeIndex := 0
	for _, portID := range ports {
		if arena.Port(portID).ID == -1 {
			identifyHyperedgeDFS(arena, portID, hyperedgeIndex)
			hyperedgeIndex++
		}
	}
}

func identifyHyperedgeDFS(
	arena *layered.LGraphArena,
	portID layered.LPortID,
	hyperedgeIndex int,
) {
	port := arena.Port(portID)
	port.ID = hyperedgeIndex
	for _, edgeID := range port.IncomingEdges {
		edge := arena.Edge(edgeID)
		if edge.Source != nil && arena.Port(*edge.Source).ID == -1 {
			identifyHyperedgeDFS(arena, *edge.Source, hyperedgeIndex)
		}
	}
	for _, edgeID := range port.OutgoingEdges {
		edge := arena.Edge(edgeID)
		if edge.Target != nil && arena.Port(*edge.Target).ID == -1 {
			identifyHyperedgeDFS(arena, *edge.Target, hyperedgeIndex)
		}
	}
	if port.Node == nil || arena.Node(*port.Node).Type != layered.LongEdgeNode {
		return
	}
	for _, otherPort := range arena.Node(*port.Node).Ports {
		if otherPort != portID && arena.Port(otherPort).ID == -1 {
			identifyHyperedgeDFS(arena, otherPort, hyperedgeIndex)
		}
	}
}
