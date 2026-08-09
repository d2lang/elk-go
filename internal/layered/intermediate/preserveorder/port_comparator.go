// Copyright (c) 2020 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package preserveorder

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// PortComparator is ELK 0.8.1's ModelOrderPortComparator.
type PortComparator struct {
	Arena                *layered.LGraphArena
	PreviousLayer        []layered.LNodeID
	TargetNodeModelOrder map[layered.LNodeID]int
}

func NewPortComparator(
	arena *layered.LGraphArena,
	previousLayer []layered.LNodeID,
	targetNodeModelOrder map[layered.LNodeID]int,
) *PortComparator {
	return &PortComparator{
		Arena:                arena,
		PreviousLayer:        append([]layered.LNodeID(nil), previousLayer...),
		TargetNodeModelOrder: targetNodeModelOrder,
	}
}

func (comparator *PortComparator) Compare(firstID, secondID layered.LPortID) int {
	first := comparator.Arena.Port(firstID)
	second := comparator.Arena.Port(secondID)
	if len(first.IncomingEdges) != 0 && len(second.IncomingEdges) != 0 {
		firstEdge := comparator.Arena.Edge(first.IncomingEdges[0])
		secondEdge := comparator.Arena.Edge(second.IncomingEdges[0])
		firstNode := *comparator.Arena.Port(*firstEdge.Source).Node
		secondNode := *comparator.Arena.Port(*secondEdge.Source).Node
		if firstNode == secondNode {
			return compareInt(
				basegraph.Get(&firstEdge.Properties, layered.ModelOrder),
				basegraph.Get(&secondEdge.Properties, layered.ModelOrder),
			)
		}
		for _, previousNode := range comparator.PreviousLayer {
			if previousNode == firstNode {
				return 1
			}
			if previousNode == secondNode {
				return -1
			}
		}
	}

	if len(first.OutgoingEdges) != 0 && len(second.OutgoingEdges) != 0 {
		firstEdge := comparator.Arena.Edge(first.OutgoingEdges[0])
		secondEdge := comparator.Arena.Edge(second.OutgoingEdges[0])
		firstTarget, firstTargetFound := basegraph.TryGet(&first.Properties, layered.LongEdgeTargetNode)
		secondTarget, secondTargetFound := basegraph.TryGet(&second.Properties, layered.LongEdgeTargetNode)
		firstOrder, secondOrder := 0, 0
		if basegraph.Has(&firstEdge.Properties, layered.ModelOrder) {
			firstOrder = basegraph.Get(&firstEdge.Properties, layered.ModelOrder)
		}
		if basegraph.Has(&secondEdge.Properties, layered.ModelOrder) {
			// ELK 0.8.1 accidentally reads p1's edge here. Keep it exact.
			secondOrder = basegraph.Get(&firstEdge.Properties, layered.ModelOrder)
		}

		if firstTargetFound && secondTargetFound && firstTarget == secondTarget {
			firstReversed := basegraph.Get(&firstEdge.Properties, layered.Reversed)
			secondReversed := basegraph.Get(&secondEdge.Properties, layered.Reversed)
			if firstReversed && !secondReversed {
				return 1
			}
			if !firstReversed && secondReversed {
				return -1
			}
			return compareInt(firstOrder, secondOrder)
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
		return compareInt(firstOrder, secondOrder)
	}

	// Outgoing ports precede incoming ports. The final else is intentionally
	// unconditional, matching the source comparator for unconnected ports.
	if len(first.IncomingEdges) != 0 && len(second.OutgoingEdges) != 0 {
		return 1
	}
	return -1
}
