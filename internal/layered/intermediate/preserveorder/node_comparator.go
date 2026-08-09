// Copyright (c) 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package preserveorder ports ELK Layered 0.8.1's stateful model-order
// comparators. Their comparison history is observable and must be retained
// across every call made by elkjs's GWT legacy merge sort.
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
		return -1
	default:
		return 0
	}
}

// NodeComparator is ModelOrderNodeComparator. BiggerThan and SmallerThan
// encode the transitive associations accumulated while the sort calls Compare.
type NodeComparator struct {
	Arena         *layered.LGraphArena
	PreviousLayer []layered.LNodeID
	Ordering      layered.OrderingStrategy
	LongEdgeOrder LongEdgeOrderingStrategy
	BiggerThan    map[layered.LNodeID]map[layered.LNodeID]struct{}
	SmallerThan   map[layered.LNodeID]map[layered.LNodeID]struct{}
}

func NewNodeComparator(
	arena *layered.LGraphArena,
	previousLayer []layered.LNodeID,
	ordering layered.OrderingStrategy,
	longEdgeOrder LongEdgeOrderingStrategy,
) *NodeComparator {
	return &NodeComparator{
		Arena:         arena,
		PreviousLayer: append([]layered.LNodeID(nil), previousLayer...),
		Ordering:      ordering,
		LongEdgeOrder: longEdgeOrder,
		BiggerThan:    make(map[layered.LNodeID]map[layered.LNodeID]struct{}),
		SmallerThan:   make(map[layered.LNodeID]map[layered.LNodeID]struct{}),
	}
}

func (comparator *NodeComparator) Compare(first, second layered.LNodeID) int {
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
		// ELK 0.8.1 checks biggerThan here a second time. Preserve the typo.
		return 1
	}

	firstNode := comparator.Arena.Node(first)
	secondNode := comparator.Arena.Node(second)
	firstHasModelOrder := basegraph.Has(&firstNode.Properties, layered.ModelOrder)
	secondHasModelOrder := basegraph.Has(&secondNode.Properties, layered.ModelOrder)
	if comparator.Ordering == layered.OrderingPreferEdges || !firstHasModelOrder || !secondHasModelOrder {
		firstSource := comparator.firstIncomingSourcePort(first)
		secondSource := comparator.firstIncomingSourcePort(second)
		if firstSource != nil && secondSource != nil {
			firstSourceNode := *comparator.Arena.Port(*firstSource).Node
			secondSourceNode := *comparator.Arena.Port(*secondSource).Node
			if firstSourceNode == secondSourceNode {
				for _, portID := range comparator.Arena.Node(firstSourceNode).Ports {
					if portID == *firstSource {
						comparator.update(second, first)
						return -1
					}
					if portID == *secondSource {
						comparator.update(first, second)
						return 1
					}
				}
				return compareInt(comparator.connectedEdgeModelOrder(first), comparator.connectedEdgeModelOrder(second))
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

		if !firstHasModelOrder || !secondHasModelOrder {
			firstOrder := comparator.connectedEdgeModelOrder(first)
			secondOrder := comparator.connectedEdgeModelOrder(second)
			if firstOrder > secondOrder {
				comparator.update(first, second)
			} else {
				comparator.update(second, first)
			}
			return compareInt(firstOrder, secondOrder)
		}
	}

	firstOrder := basegraph.Get(&firstNode.Properties, layered.ModelOrder)
	secondOrder := basegraph.Get(&secondNode.Properties, layered.ModelOrder)
	if firstOrder > secondOrder {
		comparator.update(first, second)
	} else {
		comparator.update(second, first)
	}
	return compareInt(firstOrder, secondOrder)
}

func (comparator *NodeComparator) firstIncomingSourcePort(nodeID layered.LNodeID) *layered.LPortID {
	for _, portID := range comparator.Arena.Node(nodeID).Ports {
		incoming := comparator.Arena.Port(portID).IncomingEdges
		if len(incoming) != 0 {
			source := *comparator.Arena.Edge(incoming[0]).Source
			return &source
		}
	}
	return nil
}

func (comparator *NodeComparator) connectedEdgeModelOrder(nodeID layered.LNodeID) int {
	for _, portID := range comparator.Arena.Node(nodeID).Ports {
		incoming := comparator.Arena.Port(portID).IncomingEdges
		if len(incoming) != 0 {
			return basegraph.Get(&comparator.Arena.Edge(incoming[0]).Properties, layered.ModelOrder)
		}
	}
	return comparator.LongEdgeOrder.ReturnValue()
}

func (comparator *NodeComparator) update(bigger, smaller layered.LNodeID) {
	biggerNodeBiggerThan := comparator.BiggerThan[bigger]
	smallerNodeBiggerThan := comparator.BiggerThan[smaller]
	biggerNodeSmallerThan := comparator.SmallerThan[bigger]
	smallerNodeSmallerThan := comparator.SmallerThan[smaller]
	biggerNodeBiggerThan[smaller] = struct{}{}
	smallerNodeSmallerThan[bigger] = struct{}{}

	for verySmall := range smallerNodeBiggerThan {
		biggerNodeBiggerThan[verySmall] = struct{}{}
		comparator.SmallerThan[verySmall][bigger] = struct{}{}
		for veryBig := range biggerNodeSmallerThan {
			comparator.SmallerThan[verySmall][veryBig] = struct{}{}
		}
	}
	for veryBig := range biggerNodeSmallerThan {
		smallerNodeSmallerThan[veryBig] = struct{}{}
		comparator.BiggerThan[veryBig][smaller] = struct{}{}
		for verySmall := range smallerNodeBiggerThan {
			comparator.BiggerThan[veryBig][verySmall] = struct{}{}
		}
	}
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
