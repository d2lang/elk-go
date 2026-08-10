// Copyright (c) 2020 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package preserveorder

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// PortComparator ports ELK 0.12.0's ModelOrderPortComparator behavior used by
// D2's model-order profile. It retains the comparator's transitive ordering
// state because comparisons made earlier in the stable sort affect later ones.
type PortComparator struct {
	Arena                *layered.LGraphArena
	PreviousLayer        []layered.LNodeID
	Ordering             layered.OrderingStrategy
	TargetNodeModelOrder map[layered.LNodeID]int
	PortModelOrder       bool
	BiggerThan           map[layered.LPortID]map[layered.LPortID]struct{}
	SmallerThan          map[layered.LPortID]map[layered.LPortID]struct{}
}

func NewPortComparator(
	arena *layered.LGraphArena,
	previousLayer []layered.LNodeID,
	ordering layered.OrderingStrategy,
	targetNodeModelOrder map[layered.LNodeID]int,
	portModelOrder bool,
) *PortComparator {
	return &PortComparator{
		Arena:                arena,
		PreviousLayer:        append([]layered.LNodeID(nil), previousLayer...),
		Ordering:             ordering,
		TargetNodeModelOrder: targetNodeModelOrder,
		PortModelOrder:       portModelOrder,
		BiggerThan:           make(map[layered.LPortID]map[layered.LPortID]struct{}),
		SmallerThan:          make(map[layered.LPortID]map[layered.LPortID]struct{}),
	}
}

func (comparator *PortComparator) Compare(firstID, secondID layered.LPortID) int {
	if result := comparator.initializeAndRecall(firstID, secondID); result != 0 {
		return result
	}
	first := comparator.Arena.Port(firstID)
	second := comparator.Arena.Port(secondID)
	if first.Side != second.Side {
		result := compareInt(int(first.Side), int(second.Side))
		if result > 0 {
			comparator.update(firstID, secondID, 1)
		} else {
			comparator.update(secondID, firstID, 1)
		}
		return result
	}

	reverseOrder := 1
	if len(first.IncomingEdges) != 0 && len(second.IncomingEdges) != 0 {
		if first.Side == layered.WestSide || first.Side == layered.NorthSide ||
			first.Side == layered.SouthSide {
			reverseOrder = -reverseOrder
		}
		firstSourcePort := *comparator.Arena.Edge(first.IncomingEdges[0]).Source
		secondSourcePort := *comparator.Arena.Edge(second.IncomingEdges[0]).Source
		firstSourceNode := *comparator.Arena.Port(firstSourcePort).Node
		secondSourceNode := *comparator.Arena.Port(secondSourcePort).Node
		if firstSourceNode == secondSourceNode {
			for _, portID := range comparator.Arena.Node(firstSourceNode).Ports {
				if portID == firstSourcePort {
					comparator.update(secondID, firstID, reverseOrder)
					return -reverseOrder
				}
				if portID == secondSourcePort {
					comparator.update(firstID, secondID, reverseOrder)
					return reverseOrder
				}
			}
		}
		firstSource := comparator.Arena.Node(firstSourceNode)
		secondSource := comparator.Arena.Node(secondSourceNode)
		currentNode := comparator.Arena.Node(*first.Node)
		if firstSource.Type == layered.LongEdgeNode &&
			secondSource.Type == layered.LongEdgeNode &&
			firstSource.Layer != nil && secondSource.Layer != nil && currentNode.Layer != nil &&
			comparator.Arena.Layer(*firstSource.Layer).ID == comparator.Arena.Layer(*secondSource.Layer).ID &&
			comparator.Arena.Layer(*firstSource.Layer).ID == comparator.Arena.Layer(*currentNode.Layer).ID {
			inReferenceLayer := comparator.checkReferenceLayer(
				comparator.Arena.Layer(*firstSource.Layer).Nodes,
				firstSourceNode, secondSourceNode,
			)
			if inReferenceLayer != 0 {
				if first.Side == layered.EastSide {
					reverseOrder = -reverseOrder
				}
				if inReferenceLayer > 0 {
					comparator.update(firstID, secondID, reverseOrder)
					return reverseOrder
				}
				comparator.update(secondID, firstID, reverseOrder)
				return -reverseOrder
			}
		}
		inPreviousLayer := comparator.checkReferenceLayer(
			comparator.PreviousLayer, firstSourceNode, secondSourceNode)
		if inPreviousLayer != 0 {
			if inPreviousLayer > 0 {
				comparator.update(firstID, secondID, reverseOrder)
				return reverseOrder
			}
			comparator.update(secondID, firstID, reverseOrder)
			return -reverseOrder
		}
		if comparator.PortModelOrder {
			if result := comparator.checkPortModelOrder(firstID, secondID); result != 0 {
				if result > 0 {
					comparator.update(firstID, secondID, reverseOrder)
					return reverseOrder
				}
				comparator.update(secondID, firstID, reverseOrder)
				return -reverseOrder
			}
		}
	}

	if len(first.OutgoingEdges) != 0 && len(second.OutgoingEdges) != 0 {
		if first.Side == layered.WestSide || first.Side == layered.SouthSide {
			reverseOrder = -reverseOrder
		}
		firstTarget, firstTargetFound := basegraph.TryGet(
			&first.Properties, layered.LongEdgeTargetNode)
		secondTarget, secondTargetFound := basegraph.TryGet(
			&second.Properties, layered.LongEdgeTargetNode)
		if comparator.Ordering == layered.OrderingPreferNodes &&
			firstTargetFound && secondTargetFound &&
			basegraph.Has(&comparator.Arena.Node(firstTarget).Properties, layered.ModelOrder) &&
			basegraph.Has(&comparator.Arena.Node(secondTarget).Properties, layered.ModelOrder) {
			graph := comparator.graph(firstID)
			offset := basegraph.Get(&graph.Properties, layered.MaximumModelOrderNodes)
			firstOrder := CalculateModelOrderOrGroupModelOrder(
				graph,
				&comparator.Arena.Node(firstTarget).Properties,
				&comparator.Arena.Node(secondTarget).Properties,
				offset,
			)
			secondOrder := CalculateModelOrderOrGroupModelOrder(
				graph,
				&comparator.Arena.Node(secondTarget).Properties,
				&comparator.Arena.Node(firstTarget).Properties,
				offset,
			)
			if firstOrder > secondOrder {
				comparator.update(firstID, secondID, reverseOrder)
				return reverseOrder
			}
			comparator.update(secondID, firstID, reverseOrder)
			return -reverseOrder
		}
		if comparator.PortModelOrder {
			if result := comparator.checkPortModelOrder(firstID, secondID); result != 0 {
				if result > 0 {
					comparator.update(firstID, secondID, reverseOrder)
					return reverseOrder
				}
				comparator.update(secondID, firstID, reverseOrder)
				return -reverseOrder
			}
		}

		firstEdge := comparator.Arena.Edge(first.OutgoingEdges[0])
		secondEdge := comparator.Arena.Edge(second.OutgoingEdges[0])
		graph := comparator.graph(firstID)
		firstOrder, secondOrder := 0, 0
		if basegraph.Has(&firstEdge.Properties, layered.ModelOrder) {
			firstOrder = CalculateModelOrderOrGroupModelOrder(
				graph,
				&firstEdge.Properties,
				&secondEdge.Properties,
				len(first.OutgoingEdges)+len(first.IncomingEdges),
			)
		}
		if basegraph.Has(&secondEdge.Properties, layered.ModelOrder) {
			secondOrder = CalculateModelOrderOrGroupModelOrder(
				graph,
				&secondEdge.Properties,
				&firstEdge.Properties,
				len(second.OutgoingEdges)+len(second.IncomingEdges),
			)
		}
		if firstTargetFound && secondTargetFound && firstTarget == secondTarget {
			if firstOrder > secondOrder {
				comparator.update(firstID, secondID, reverseOrder)
				return reverseOrder
			}
			comparator.update(secondID, firstID, reverseOrder)
			return -reverseOrder
		}
		if firstTargetFound {
			if order, found := comparator.TargetNodeModelOrder[firstTarget]; found {
				firstOrder = order
			}
		}
		if secondTargetFound {
			if order, found := comparator.TargetNodeModelOrder[secondTarget]; found {
				secondOrder = order
			}
		}
		if firstOrder > secondOrder {
			comparator.update(firstID, secondID, reverseOrder)
			return reverseOrder
		}
		comparator.update(secondID, firstID, reverseOrder)
		return -reverseOrder
	}

	if len(first.IncomingEdges) != 0 && len(second.OutgoingEdges) != 0 {
		comparator.update(firstID, secondID, reverseOrder)
		return 1
	}
	if len(first.OutgoingEdges) != 0 && len(second.IncomingEdges) != 0 {
		comparator.update(secondID, firstID, reverseOrder)
		return -1
	}
	if basegraph.Has(&first.Properties, layered.ModelOrder) &&
		basegraph.Has(&second.Properties, layered.ModelOrder) {
		if first.Side == layered.WestSide || first.Side == layered.SouthSide {
			reverseOrder = -reverseOrder
		}
		graph := comparator.graph(firstID)
		offset := len(comparator.Arena.Node(*first.Node).Ports)
		firstOrder := CalculateModelOrderOrGroupModelOrder(
			graph, &first.Properties, &second.Properties, offset)
		secondOrder := CalculateModelOrderOrGroupModelOrder(
			graph, &second.Properties, &first.Properties, offset)
		if firstOrder > secondOrder {
			comparator.update(firstID, secondID, reverseOrder)
			return reverseOrder
		}
		comparator.update(secondID, firstID, reverseOrder)
		return -reverseOrder
	}
	comparator.update(secondID, firstID, reverseOrder)
	return -reverseOrder
}

func (comparator *PortComparator) initializeAndRecall(
	first, second layered.LPortID,
) int {
	if _, found := comparator.BiggerThan[first]; !found {
		comparator.BiggerThan[first] = make(map[layered.LPortID]struct{})
	} else if containsPort(comparator.BiggerThan[first], second) {
		return 1
	}
	if _, found := comparator.BiggerThan[second]; !found {
		comparator.BiggerThan[second] = make(map[layered.LPortID]struct{})
	} else if containsPort(comparator.BiggerThan[second], first) {
		return -1
	}
	if _, found := comparator.SmallerThan[first]; !found {
		comparator.SmallerThan[first] = make(map[layered.LPortID]struct{})
	} else if containsPort(comparator.SmallerThan[first], second) {
		return -1
	}
	if _, found := comparator.SmallerThan[second]; !found {
		comparator.SmallerThan[second] = make(map[layered.LPortID]struct{})
	} else if containsPort(comparator.BiggerThan[second], first) {
		// Preserve the source comparator's second biggerThan lookup.
		return 1
	}
	return 0
}

func (comparator *PortComparator) checkPortModelOrder(
	firstID, secondID layered.LPortID,
) int {
	first := comparator.Arena.Port(firstID)
	second := comparator.Arena.Port(secondID)
	if basegraph.Has(&first.Properties, layered.ModelOrder) &&
		basegraph.Has(&second.Properties, layered.ModelOrder) {
		graph := comparator.graph(firstID)
		offset := len(comparator.Arena.Node(*first.Node).Ports)
		return compareInt(
			CalculateModelOrderOrGroupModelOrder(
				graph, &first.Properties, &second.Properties, offset),
			CalculateModelOrderOrGroupModelOrder(
				graph, &second.Properties, &first.Properties, offset),
		)
	}
	return 0
}

func (comparator *PortComparator) graph(portID layered.LPortID) *layered.LGraph {
	nodeID := *comparator.Arena.Port(portID).Node
	return comparator.Arena.Graph(comparator.Arena.NodeGraph(nodeID))
}

func (comparator *PortComparator) update(
	biggerOriginal, smallerOriginal layered.LPortID,
	reverseOrder int,
) {
	bigger, smaller := biggerOriginal, smallerOriginal
	if reverseOrder < 0 {
		bigger, smaller = smallerOriginal, biggerOriginal
	}
	comparator.ensureOrderingEntry(bigger)
	comparator.ensureOrderingEntry(smaller)
	biggerPortBiggerThan := comparator.BiggerThan[bigger]
	smallerPortBiggerThan := comparator.BiggerThan[smaller]
	biggerPortSmallerThan := comparator.SmallerThan[bigger]
	smallerPortSmallerThan := comparator.SmallerThan[smaller]
	biggerPortBiggerThan[smaller] = struct{}{}
	smallerPortSmallerThan[bigger] = struct{}{}
	for verySmall := range smallerPortBiggerThan {
		comparator.ensureOrderingEntry(verySmall)
		biggerPortBiggerThan[verySmall] = struct{}{}
		comparator.SmallerThan[verySmall][bigger] = struct{}{}
		for veryBig := range biggerPortSmallerThan {
			comparator.SmallerThan[verySmall][veryBig] = struct{}{}
		}
	}
	for veryBig := range biggerPortSmallerThan {
		comparator.ensureOrderingEntry(veryBig)
		smallerPortSmallerThan[veryBig] = struct{}{}
		comparator.BiggerThan[veryBig][smaller] = struct{}{}
		for verySmall := range smallerPortBiggerThan {
			comparator.BiggerThan[veryBig][verySmall] = struct{}{}
		}
	}
}

func (comparator *PortComparator) ensureOrderingEntry(port layered.LPortID) {
	if comparator.BiggerThan[port] == nil {
		comparator.BiggerThan[port] = make(map[layered.LPortID]struct{})
	}
	if comparator.SmallerThan[port] == nil {
		comparator.SmallerThan[port] = make(map[layered.LPortID]struct{})
	}
}

func (comparator *PortComparator) checkReferenceLayer(
	layer []layered.LNodeID,
	first, second layered.LNodeID,
) int {
	for _, nodeID := range layer {
		if nodeID == first {
			return -1
		}
		if nodeID == second {
			return 1
		}
	}
	return 0
}

func (comparator *PortComparator) ClearTransitiveOrdering() {
	comparator.BiggerThan = make(map[layered.LPortID]map[layered.LPortID]struct{})
	comparator.SmallerThan = make(map[layered.LPortID]map[layered.LPortID]struct{})
}

func containsPort(values map[layered.LPortID]struct{}, port layered.LPortID) bool {
	_, found := values[port]
	return found
}
