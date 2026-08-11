// Copyright (c) 2020 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

// Package preserveorder ports ELK Layered's stateful model-order comparators.
// Their comparison history is observable and is retained across every call
// made by a sort.
package preserveorder

import (
	"math"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

type LongEdgeOrderingStrategy uint8

const (
	DummyNodeOver LongEdgeOrderingStrategy = iota
	DummyNodeUnder
	LongEdgeEqual
)

func (strategy LongEdgeOrderingStrategy) ReturnValue() int {
	switch strategy {
	case DummyNodeOver:
		return math.MaxInt
	case DummyNodeUnder:
		return math.MinInt
	default:
		return 0
	}
}

// NodeComparator is ELK 0.12.0's ModelOrderNodeComparator for ordinary D2
// model orders. BiggerThan and SmallerThan encode the transitive associations
// accumulated while the sort calls Compare.
type NodeComparator struct {
	Arena         *layered.LGraphArena
	PreviousLayer []layered.LNodeID
	Ordering      layered.OrderingStrategy
	LongEdgeOrder LongEdgeOrderingStrategy
	BeforePorts   bool
	BiggerThan    map[layered.LNodeID]map[layered.LNodeID]struct{}
	SmallerThan   map[layered.LNodeID]map[layered.LNodeID]struct{}
}

func NewNodeComparator(
	arena *layered.LGraphArena,
	previousLayer []layered.LNodeID,
	ordering layered.OrderingStrategy,
	longEdgeOrder LongEdgeOrderingStrategy,
	beforePorts bool,
) *NodeComparator {
	return &NodeComparator{
		Arena:         arena,
		PreviousLayer: append([]layered.LNodeID(nil), previousLayer...),
		Ordering:      ordering,
		LongEdgeOrder: longEdgeOrder,
		BeforePorts:   beforePorts,
		BiggerThan:    make(map[layered.LNodeID]map[layered.LNodeID]struct{}),
		SmallerThan:   make(map[layered.LNodeID]map[layered.LNodeID]struct{}),
	}
}

func (comparator *NodeComparator) Compare(first, second layered.LNodeID) int {
	if result := comparator.initializeAndRecall(first, second); result != 0 {
		return result
	}
	firstNode := comparator.Arena.Node(first)
	secondNode := comparator.Arena.Node(second)
	firstHasModelOrder := basegraph.Has(&firstNode.Properties, layered.ModelOrder)
	secondHasModelOrder := basegraph.Has(&secondNode.Properties, layered.ModelOrder)
	if comparator.Ordering == layered.OrderingPreferEdges ||
		!firstHasModelOrder || !secondHasModelOrder {
		firstSource, firstSourceFound := comparator.incomingSourcePortFromPreviousLayer(first)
		secondSource, secondSourceFound := comparator.incomingSourcePortFromPreviousLayer(second)
		if firstSourceFound && secondSourceFound {
			firstSourceNode := *comparator.Arena.Port(firstSource).Node
			secondSourceNode := *comparator.Arena.Port(secondSource).Node
			if firstSourceNode == secondSourceNode {
				for _, portID := range comparator.Arena.Node(firstSourceNode).Ports {
					if portID == firstSource {
						comparator.update(second, first)
						return -1
					}
					if portID == secondSource {
						comparator.update(first, second)
						return 1
					}
				}
				firstOrder := comparator.connectedEdgeModelOrder(first)
				secondOrder := comparator.connectedEdgeModelOrder(second)
				if firstOrder > secondOrder {
					comparator.update(first, second)
					return 1
				}
				comparator.update(second, first)
				return -1
			}
			for _, previousNode := range comparator.PreviousLayer {
				if previousNode == firstSourceNode {
					comparator.update(second, first)
					return -1
				}
				if previousNode == secondSourceNode {
					comparator.update(first, second)
					return 1
				}
			}
		}

		if firstSourceFound != secondSourceFound {
			if result := comparator.handleHelperDummyNodes(first, second); result != 0 {
				comparator.recordResult(first, second, result)
				return result
			}
			if !firstHasModelOrder || !secondHasModelOrder {
				firstOrder := comparator.connectedEdgeModelOrder(first)
				secondOrder := comparator.connectedEdgeModelOrder(second)
				if firstOrder > secondOrder {
					comparator.update(first, second)
					return 1
				}
				comparator.update(second, first)
				return -1
			}
		}

		if !firstSourceFound && !secondSourceFound {
			if result := comparator.handleHelperDummyNodes(first, second); result != 0 {
				comparator.recordResult(first, second, result)
				return result
			}
		}
	}

	if firstHasModelOrder && secondHasModelOrder {
		graph := comparator.Arena.Graph(comparator.Arena.NodeGraph(first))
		offset := basegraph.Get(&graph.Properties, layered.MaximumModelOrderNodes)
		firstOrder := CalculateModelOrderOrGroupModelOrder(
			graph, &firstNode.Properties, &secondNode.Properties, offset)
		secondOrder := CalculateModelOrderOrGroupModelOrder(
			graph, &secondNode.Properties, &firstNode.Properties, offset)
		if firstOrder > secondOrder {
			comparator.update(first, second)
			return 1
		}
		comparator.update(second, first)
		return -1
	}
	comparator.update(second, first)
	return -1
}

func (comparator *NodeComparator) initializeAndRecall(
	first, second layered.LNodeID,
) int {
	if _, found := comparator.BiggerThan[first]; !found {
		comparator.BiggerThan[first] = make(map[layered.LNodeID]struct{})
	} else if containsNode(comparator.BiggerThan[first], second) {
		return 1
	}
	if _, found := comparator.BiggerThan[second]; !found {
		comparator.BiggerThan[second] = make(map[layered.LNodeID]struct{})
	} else if containsNode(comparator.BiggerThan[second], first) {
		return -1
	}
	if _, found := comparator.SmallerThan[first]; !found {
		comparator.SmallerThan[first] = make(map[layered.LNodeID]struct{})
	} else if containsNode(comparator.SmallerThan[first], second) {
		return -1
	}
	if _, found := comparator.SmallerThan[second]; !found {
		comparator.SmallerThan[second] = make(map[layered.LNodeID]struct{})
	} else if containsNode(comparator.BiggerThan[second], first) {
		// Preserve the source comparator's second biggerThan lookup.
		return 1
	}
	return 0
}

func (comparator *NodeComparator) incomingSourcePortFromPreviousLayer(
	nodeID layered.LNodeID,
) (layered.LPortID, bool) {
	node := comparator.Arena.Node(nodeID)
	if node.Layer == nil {
		return 0, false
	}
	wantedLayer := comparator.Arena.Layer(*node.Layer).ID - 1
	for _, portID := range node.Ports {
		incoming := comparator.Arena.Port(portID).IncomingEdges
		if len(incoming) == 0 {
			continue
		}
		edge := comparator.Arena.Edge(incoming[0])
		if edge.Source == nil || comparator.Arena.Port(*edge.Source).Node == nil {
			continue
		}
		sourceNode := comparator.Arena.Node(*comparator.Arena.Port(*edge.Source).Node)
		if sourceNode.Layer != nil && comparator.Arena.Layer(*sourceNode.Layer).ID == wantedLayer {
			return *edge.Source, true
		}
	}
	return 0, false
}

func (comparator *NodeComparator) connectedEdgeModelOrder(nodeID layered.LNodeID) int {
	for _, portID := range comparator.Arena.Node(nodeID).Ports {
		incoming := comparator.Arena.Port(portID).IncomingEdges
		if len(incoming) != 0 {
			return basegraph.Get(
				&comparator.Arena.Edge(incoming[0]).Properties, layered.ModelOrder)
		}
	}
	return comparator.LongEdgeOrder.ReturnValue()
}

func (comparator *NodeComparator) handleHelperDummyNodes(
	first, second layered.LNodeID,
) int {
	firstNode := comparator.Arena.Node(first)
	secondNode := comparator.Arena.Node(second)
	switch {
	case firstNode.Type == layered.LongEdgeNode && secondNode.Type == layered.NormalNode:
		firstSource, firstTarget, found := comparator.dummyEndpoints(first)
		if !found {
			return 0
		}
		dummyLayer := comparator.nodeLayerID(first)
		if comparator.nodeLayerID(firstSource) != dummyLayer &&
			comparator.nodeLayerID(firstTarget) != dummyLayer {
			return 0
		}
		if firstSource == second || firstTarget == second {
			comparator.update(first, second)
			return 1
		}
		return comparator.Compare(firstSource, second)

	case firstNode.Type == layered.NormalNode && secondNode.Type == layered.LongEdgeNode:
		secondSource, secondTarget, found := comparator.dummyEndpoints(second)
		if !found {
			return 0
		}
		dummyLayer := comparator.nodeLayerID(first)
		if comparator.nodeLayerID(secondSource) != dummyLayer &&
			comparator.nodeLayerID(secondTarget) != dummyLayer {
			return 0
		}
		if secondSource == first || secondTarget == first {
			comparator.update(second, first)
			return -1
		}
		return comparator.Compare(first, secondSource)

	case firstNode.Type == layered.LongEdgeNode && secondNode.Type == layered.LongEdgeNode:
		firstSourcePort, firstTargetPort, firstSource, firstTarget, firstFound :=
			comparator.dummyEndpointPorts(first)
		secondSourcePort, _, secondSource, secondTarget, secondFound :=
			comparator.dummyEndpointPorts(second)
		if !firstFound || !secondFound {
			return 0
		}
		firstLayer := comparator.nodeLayerID(first)
		secondLayer := comparator.nodeLayerID(second)
		firstSourceFeedback := comparator.nodeLayerID(firstSource) == firstLayer
		firstTargetFeedback := comparator.nodeLayerID(firstTarget) == firstLayer
		secondSourceFeedback := comparator.nodeLayerID(secondSource) == secondLayer
		secondTargetFeedback := comparator.nodeLayerID(secondTarget) == secondLayer
		firstReference, secondReference := first, second
		if firstSourceFeedback {
			firstReference = firstSource
		} else if firstTargetFeedback {
			firstReference = firstTarget
		}
		if secondSourceFeedback {
			secondReference = secondSource
		} else if secondTargetFeedback {
			secondReference = secondTarget
		}
		if firstReference == secondReference {
			if comparator.BeforePorts {
				switch {
				case firstSourceFeedback && secondSourceFeedback:
					portComparator := NewPortComparator(
						comparator.Arena,
						comparator.PreviousLayer,
						comparator.Ordering,
						nil,
						secondTargetFeedback,
					)
					if portComparator.Compare(firstSourcePort, secondSourcePort) > 0 {
						comparator.update(second, first)
						return 1
					}
					comparator.update(first, second)
					return -1
				case firstSourceFeedback && secondTargetFeedback:
					comparator.update(second, first)
					return 1
				case firstTargetFeedback && secondSourceFeedback:
					comparator.update(first, second)
					return -1
				case firstTargetFeedback && secondTargetFeedback:
					return 0
				}
			} else {
				for _, portID := range comparator.Arena.Node(firstReference).Ports {
					if portID == firstSourcePort {
						comparator.update(second, first)
						return -1
					}
					if portID == secondSourcePort {
						comparator.update(first, second)
						return 1
					}
				}
			}
		}
		_ = firstTargetPort
		return comparator.Compare(firstReference, secondReference)
	}
	return 0
}

func (comparator *NodeComparator) dummyEndpoints(
	nodeID layered.LNodeID,
) (layered.LNodeID, layered.LNodeID, bool) {
	_, _, source, target, found := comparator.dummyEndpointPorts(nodeID)
	return source, target, found
}

func (comparator *NodeComparator) dummyEndpointPorts(
	nodeID layered.LNodeID,
) (layered.LPortID, layered.LPortID, layered.LNodeID, layered.LNodeID, bool) {
	var incomingPort, outgoingPort layered.LPortID
	incomingFound, outgoingFound := false, false
	for _, portID := range comparator.Arena.Node(nodeID).Ports {
		port := comparator.Arena.Port(portID)
		if !incomingFound && len(port.IncomingEdges) != 0 {
			edge := comparator.Arena.Edge(port.IncomingEdges[0])
			if edge.Source != nil && comparator.Arena.Port(*edge.Source).Node != nil {
				incomingPort, incomingFound = *edge.Source, true
			}
		}
		if !outgoingFound && len(port.OutgoingEdges) != 0 {
			edge := comparator.Arena.Edge(port.OutgoingEdges[0])
			if edge.Target != nil && comparator.Arena.Port(*edge.Target).Node != nil {
				outgoingPort, outgoingFound = *edge.Target, true
			}
		}
	}
	if !incomingFound || !outgoingFound {
		return 0, 0, 0, 0, false
	}
	return incomingPort, outgoingPort,
		*comparator.Arena.Port(incomingPort).Node,
		*comparator.Arena.Port(outgoingPort).Node,
		true
}

func (comparator *NodeComparator) nodeLayerID(nodeID layered.LNodeID) int {
	layerID := comparator.Arena.Node(nodeID).Layer
	if layerID == nil {
		return math.MinInt
	}
	return comparator.Arena.Layer(*layerID).ID
}

func (comparator *NodeComparator) recordResult(
	first, second layered.LNodeID,
	result int,
) {
	if result > 0 {
		comparator.update(first, second)
	} else {
		comparator.update(second, first)
	}
}

func (comparator *NodeComparator) update(bigger, smaller layered.LNodeID) {
	comparator.ensureOrderingEntry(bigger)
	comparator.ensureOrderingEntry(smaller)
	biggerNodeBiggerThan := comparator.BiggerThan[bigger]
	smallerNodeBiggerThan := comparator.BiggerThan[smaller]
	biggerNodeSmallerThan := comparator.SmallerThan[bigger]
	smallerNodeSmallerThan := comparator.SmallerThan[smaller]
	biggerNodeBiggerThan[smaller] = struct{}{}
	smallerNodeSmallerThan[bigger] = struct{}{}
	for verySmall := range smallerNodeBiggerThan {
		comparator.ensureOrderingEntry(verySmall)
		biggerNodeBiggerThan[verySmall] = struct{}{}
		comparator.SmallerThan[verySmall][bigger] = struct{}{}
		for veryBig := range biggerNodeSmallerThan {
			comparator.SmallerThan[verySmall][veryBig] = struct{}{}
		}
	}
	for veryBig := range biggerNodeSmallerThan {
		comparator.ensureOrderingEntry(veryBig)
		smallerNodeSmallerThan[veryBig] = struct{}{}
		comparator.BiggerThan[veryBig][smaller] = struct{}{}
		for verySmall := range smallerNodeBiggerThan {
			comparator.BiggerThan[veryBig][verySmall] = struct{}{}
		}
	}
}

func (comparator *NodeComparator) ensureOrderingEntry(node layered.LNodeID) {
	if comparator.BiggerThan[node] == nil {
		comparator.BiggerThan[node] = make(map[layered.LNodeID]struct{})
	}
	if comparator.SmallerThan[node] == nil {
		comparator.SmallerThan[node] = make(map[layered.LNodeID]struct{})
	}
}

func (comparator *NodeComparator) ClearTransitiveOrdering() {
	comparator.BiggerThan = make(map[layered.LNodeID]map[layered.LNodeID]struct{})
	comparator.SmallerThan = make(map[layered.LNodeID]map[layered.LNodeID]struct{})
}

func containsNode(values map[layered.LNodeID]struct{}, node layered.LNodeID) bool {
	_, found := values[node]
	return found
}

func compareInt(first, second int) int {
	if first < second {
		return -1
	}
	if first > second {
		return 1
	}
	return 0
}
