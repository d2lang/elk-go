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

	// unprocessed is the cycle detector's membership flag (false outside
	// computeLinearOrderingMarks).
	unprocessed bool
}

func newHyperEdgeSegment(strategy *routingDirectionStrategy) *HyperEdgeSegment {
	if strategy != nil && strategy.alloc != nil {
		return strategy.alloc.segment(strategy)
	}
	return &HyperEdgeSegment{strategy: strategy, Start: math.NaN(), End: math.NaN()}
}

// segmentAlloc recycles the segments and dependencies of a generator's
// RouteEdges calls. Nothing refers to them once a call has returned (the
// generator's own buffers are emptied at the start of the next call), so each
// call takes them from the start again. A handed-out object is reset to
// exactly the state a new one has, except that its slices keep their storage
// with length zero; that storage belonged to the object alone, and nothing
// distinguishes an empty slice from a nil one here.
type segmentAlloc struct {
	segments         []*HyperEdgeSegment
	usedSegments     int
	dependencies     []*HyperEdgeSegmentDependency
	usedDependencies int
}

// restart makes every object available again.
func (alloc *segmentAlloc) restart() {
	alloc.usedSegments, alloc.usedDependencies = 0, 0
}

func (alloc *segmentAlloc) segment(strategy *routingDirectionStrategy) *HyperEdgeSegment {
	if alloc.usedSegments == len(alloc.segments) {
		alloc.segments = append(alloc.segments, &HyperEdgeSegment{})
	}
	segment := alloc.segments[alloc.usedSegments]
	alloc.usedSegments++
	*segment = HyperEdgeSegment{
		strategy: strategy, Start: math.NaN(), End: math.NaN(),
		Ports:                segment.Ports[:0],
		IncomingCoordinates:  segment.IncomingCoordinates[:0],
		OutgoingCoordinates:  segment.OutgoingCoordinates[:0],
		OutgoingDependencies: clearDependencies(segment.OutgoingDependencies),
		IncomingDependencies: clearDependencies(segment.IncomingDependencies),
	}
	return segment
}

func clearDependencies(values []*HyperEdgeSegmentDependency) []*HyperEdgeSegmentDependency {
	for index := range values {
		values[index] = nil
	}
	return values[:0]
}

func (alloc *segmentAlloc) dependency(kind DependencyType, weight int) *HyperEdgeSegmentDependency {
	if alloc.usedDependencies == len(alloc.dependencies) {
		alloc.dependencies = append(alloc.dependencies, &HyperEdgeSegmentDependency{})
	}
	dependency := alloc.dependencies[alloc.usedDependencies]
	alloc.usedDependencies++
	*dependency = HyperEdgeSegmentDependency{Type: kind, Weight: weight}
	return dependency
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

// addPortPositions adds portID, and every port connected to it through edges
// that is in no segment yet, to segment. It is ELK's recursive
// addPortPositions made iterative, so that long chains of connected ports
// need no deeper call stack (the strategy keeps the pending calls). Every
// read and write happens in the recursion's order: a call adds its port
// (membership, port list, coordinate, extent), then walks the port's
// incoming edges' sources and then its outgoing edges' targets in order (as
// LPort.getConnectedPorts yields predecessors first, then successors), a
// call made for a port not added yet completing before the next edge is
// looked at. The edge lists are read again at each step instead of being
// kept: adding ports changes no edge list.
func (segment *HyperEdgeSegment) addPortPositions(
	arena *layered.LGraphArena,
	portID layered.LPortID,
	portSegments *portSet,
) {
	strategy := segment.strategy
	frames := append(strategy.portWalk[:0], portWalkFrame{port: portID})
	segment.addPortPosition(arena, portID, portSegments)
	for len(frames) > 0 {
		frame := &frames[len(frames)-1]
		port := arena.Port(frame.port)
		edges := port.IncomingEdges
		if frame.list != 0 {
			edges = port.OutgoingEdges
		}
		if int(frame.edge) >= len(edges) {
			if frame.list == 0 {
				frame.list, frame.edge = 1, 0
			} else {
				frames = frames[:len(frames)-1]
			}
			continue
		}
		edgeID := edges[frame.edge]
		frame.edge++
		var other layered.LPortID
		if frame.list == 0 {
			other = *arena.Edge(edgeID).Source
		} else {
			other = *arena.Edge(edgeID).Target
		}
		if !portSegments.has(other) {
			// The call addPortPositions(other).
			segment.addPortPosition(arena, other, portSegments)
			frames = append(frames, portWalkFrame{port: other})
		}
	}
	strategy.portWalk = frames[:0]
}

// portWalkFrame is one pending call of addPortPositions: its port, which of
// the port's edge lists is walked (0: incoming, 1: outgoing) and the next
// edge's index in it.
type portWalkFrame struct {
	port layered.LPortID
	list int32
	edge int32
}

// addPortPosition is what a call of addPortPositions does to its own port.
func (segment *HyperEdgeSegment) addPortPosition(
	arena *layered.LGraphArena,
	portID layered.LPortID,
	portSegments *portSet,
) {
	portSegments.add(portID)
	segment.Ports = append(segment.Ports, portID)
	port := arena.Port(portID)
	position := segment.strategy.portPosition(arena, portID)
	if port.Side == segment.strategy.sourcePortSide() {
		segment.IncomingCoordinates = insertSortedUnique(segment.IncomingCoordinates, position)
	} else {
		segment.OutgoingCoordinates = insertSortedUnique(segment.OutgoingCoordinates, position)
	}
	segment.RecomputeExtent()
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
