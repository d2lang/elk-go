// Copyright (c) 2010, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package orthogonal

import (
	"math"
	"sort"

	"github.com/d2lang/elk-go/internal/layered"
)

// HyperEdgeSegment is the trunk of an ELK orthogonal hyperedge. Coordinate
// lists are kept sorted and duplicate-free exactly as in ELK 0.8.1.
type HyperEdgeSegment struct {
	strategy *routingDirectionStrategy

	Ports []layered.LPortID
	Mark  int

	RoutingSlot int
	Start       float64
	End         float64

	IncomingCoordinates []float64
	OutgoingCoordinates []float64

	OutgoingDependencies []*HyperEdgeSegmentDependency
	IncomingDependencies []*HyperEdgeSegmentDependency
	OutWeight            int
	CriticalOutWeight    int
	InWeight             int
	CriticalInWeight     int

	SplitPartner *HyperEdgeSegment
	SplitBy      *HyperEdgeSegment
}

func newHyperEdgeSegment(strategy *routingDirectionStrategy) *HyperEdgeSegment {
	return &HyperEdgeSegment{strategy: strategy, Start: math.NaN(), End: math.NaN()}
}

// NewBareSegment is new HyperEdgeSegment(null), used by ELK's self-loop
// RoutingSlotAssigner to reuse the weighted dependency cycle breaker.
func NewBareSegment() *HyperEdgeSegment { return newHyperEdgeSegment(nil) }

// NewTestSegment creates a source-shaped segment without graph ports. It is
// exported for white-box conformance tests and cycle-routing helpers.
func NewTestSegment(incoming, outgoing []float64) *HyperEdgeSegment {
	segment := newHyperEdgeSegment(newRoutingDirectionStrategy(WestToEast))
	segment.IncomingCoordinates = append([]float64(nil), incoming...)
	segment.OutgoingCoordinates = append([]float64(nil), outgoing...)
	sort.Float64s(segment.IncomingCoordinates)
	sort.Float64s(segment.OutgoingCoordinates)
	segment.RecomputeExtent()
	return segment
}

func (segment *HyperEdgeSegment) addPortPositions(
	arena *layered.LGraphArena,
	portID layered.LPortID,
	portSegments map[layered.LPortID]*HyperEdgeSegment,
) {
	portSegments[portID] = segment
	segment.Ports = append(segment.Ports, portID)
	port := arena.Port(portID)
	position := segment.strategy.portPosition(arena, portID)
	if port.Side == segment.strategy.sourcePortSide() {
		segment.IncomingCoordinates = insertSortedUnique(segment.IncomingCoordinates, position)
	} else {
		segment.OutgoingCoordinates = insertSortedUnique(segment.OutgoingCoordinates, position)
	}
	segment.RecomputeExtent()

	// LPort.getConnectedPorts() yields predecessors first, then successors.
	for _, edgeID := range port.IncomingEdges {
		other := *arena.Edge(edgeID).Source
		if _, found := portSegments[other]; !found {
			segment.addPortPositions(arena, other, portSegments)
		}
	}
	for _, edgeID := range port.OutgoingEdges {
		other := *arena.Edge(edgeID).Target
		if _, found := portSegments[other]; !found {
			segment.addPortPositions(arena, other, portSegments)
		}
	}
}

func insertSortedUnique(values []float64, value float64) []float64 {
	for index, stored := range values {
		// HyperEdgeSegment.java deliberately calls Double.floatValue here.
		// Keep the narrowing even though the list itself stores doubles.
		next := float64(float32(stored))
		if next == value {
			return values
		}
		if next > value {
			values = append(values, 0)
			copy(values[index+1:], values[index:])
			values[index] = value
			return values
		}
	}
	return append(values, value)
}

func (segment *HyperEdgeSegment) Length() float64 { return segment.End - segment.Start }

func (segment *HyperEdgeSegment) RepresentsHyperedge() bool {
	return len(segment.IncomingCoordinates)+len(segment.OutgoingCoordinates) > 2
}

func (segment *HyperEdgeSegment) IsDummy() bool {
	return segment.SplitPartner != nil && segment.SplitBy == nil
}

func (segment *HyperEdgeSegment) RecomputeExtent() {
	segment.Start, segment.End = math.NaN(), math.NaN()
	segment.includeExtent(segment.IncomingCoordinates)
	segment.includeExtent(segment.OutgoingCoordinates)
}

func (segment *HyperEdgeSegment) includeExtent(positions []float64) {
	if len(positions) == 0 {
		return
	}
	if math.IsNaN(segment.Start) {
		segment.Start = positions[0]
	} else {
		segment.Start = math.Min(segment.Start, positions[0])
	}
	if math.IsNaN(segment.End) {
		segment.End = positions[len(positions)-1]
	} else {
		segment.End = math.Max(segment.End, positions[len(positions)-1])
	}
}

func (segment *HyperEdgeSegment) simulateSplit() (*HyperEdgeSegment, *HyperEdgeSegment) {
	first := newHyperEdgeSegment(segment.strategy)
	second := newHyperEdgeSegment(segment.strategy)
	first.IncomingCoordinates = append(first.IncomingCoordinates, segment.IncomingCoordinates...)
	first.SplitBy = segment.SplitBy
	first.SplitPartner = second
	first.RecomputeExtent()
	second.OutgoingCoordinates = append(second.OutgoingCoordinates, segment.OutgoingCoordinates...)
	second.SplitPartner = first
	second.RecomputeExtent()
	return first, second
}

func (segment *HyperEdgeSegment) splitAt(position float64) *HyperEdgeSegment {
	partner := newHyperEdgeSegment(segment.strategy)
	segment.SplitPartner, partner.SplitPartner = partner, segment
	partner.OutgoingCoordinates = append(partner.OutgoingCoordinates, segment.OutgoingCoordinates...)
	segment.OutgoingCoordinates = segment.OutgoingCoordinates[:0]
	segment.OutgoingCoordinates = append(segment.OutgoingCoordinates, position)
	partner.IncomingCoordinates = append(partner.IncomingCoordinates, position)
	segment.RecomputeExtent()
	partner.RecomputeExtent()
	for len(segment.IncomingDependencies) > 0 {
		segment.IncomingDependencies[0].Remove()
	}
	for len(segment.OutgoingDependencies) > 0 {
		segment.OutgoingDependencies[0].Remove()
	}
	return partner
}
