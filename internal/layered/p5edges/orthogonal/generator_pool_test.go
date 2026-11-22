// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package orthogonal

import (
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/layered"
)

// TestLargeJunctionSetIsNotKept: emptying the junction-point
// set keeps its storage for the next call only while it is small (a map's
// storage never shrinks, and a pooled generator stays alive indefinitely).
func TestLargeJunctionSetIsNotKept(t *testing.T) {
	for _, size := range []int{maxKeptJunctionPoints, maxKeptJunctionPoints + 1} {
		strategy := newRoutingDirectionStrategy(WestToEast)
		for index := 0; index < size; index++ {
			strategy.createdJunctionPoints[geometry.Point{X: float64(index)}] = struct{}{}
		}
		before := reflect.ValueOf(strategy.createdJunctionPoints).Pointer()
		strategy.clearCreatedJunctionPoints()
		if n := len(strategy.createdJunctionPoints); n != 0 {
			t.Fatalf("%d points: %d left after clearing", size, n)
		}
		kept := reflect.ValueOf(strategy.createdJunctionPoints).Pointer() == before
		if want := size <= maxKeptJunctionPoints; kept != want {
			t.Fatalf("%d points: storage kept = %v, want %v", size, kept, want)
		}
	}
}

// recursiveAddPortPositions is addPortPositions as it was before it was
// made iterative; the iterative one must do exactly what it does.
func recursiveAddPortPositions(
	segment *HyperEdgeSegment,
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
	for _, edgeID := range port.IncomingEdges {
		other := *arena.Edge(edgeID).Source
		if !portSegments.has(other) {
			recursiveAddPortPositions(segment, arena, other, portSegments)
		}
	}
	for _, edgeID := range port.OutgoingEdges {
		other := *arena.Edge(edgeID).Target
		if !portSegments.has(other) {
			recursiveAddPortPositions(segment, arena, other, portSegments)
		}
	}
}

// TestIterativePortWalkMatchesRecursion compares the iterative
// addPortPositions with the recursive original on random graphs of ports
// (random sides and positions, random edges between any two ports, loops
// and parallel edges included), from every port and with random ports
// already taken: the segment's ports, coordinates and extent, and the set.
func TestIterativePortWalkMatchesRecursion(t *testing.T) {
	random := rand.New(rand.NewSource(1))
	for round := 0; round < 2000; round++ {
		arena := layered.NewLGraphArena()
		graphID := arena.AddGraph()
		var ports []layered.LPortID
		for nodes := 1 + random.Intn(5); nodes > 0; nodes-- {
			nodeID := arena.AddNode(graphID)
			arena.Node(nodeID).Position.Y = float64(random.Intn(40))
			for count := 1 + random.Intn(4); count > 0; count-- {
				portID := arena.AddPort()
				arena.SetPortNode(portID, &nodeID)
				arena.SetPortSide(portID, []layered.PortSide{layered.EastSide, layered.WestSide}[random.Intn(2)])
				arena.Port(portID).Position.Y = float64(random.Intn(10))
				ports = append(ports, portID)
			}
		}
		for edges := random.Intn(2 * len(ports)); edges > 0; edges-- {
			source, target := ports[random.Intn(len(ports))], ports[random.Intn(len(ports))]
			edgeID := arena.AddEdge()
			arena.SetEdgeSource(edgeID, &source)
			arena.SetEdgeTarget(edgeID, &target)
		}
		for _, start := range ports {
			var want, got portSet
			for _, portID := range ports {
				if portID != start && random.Intn(6) == 0 {
					want.add(portID)
					got.add(portID)
				}
			}
			strategy := newRoutingDirectionStrategy(WestToEast)
			wantSegment := newHyperEdgeSegment(strategy)
			gotSegment := newHyperEdgeSegment(strategy)
			recursiveAddPortPositions(wantSegment, arena, start, &want)
			gotSegment.addPortPositions(arena, start, &got)
			wantState := fmt.Sprint(wantSegment.Ports, wantSegment.IncomingCoordinates, wantSegment.OutgoingCoordinates,
				wantSegment.Start, wantSegment.End, want.list)
			gotState := fmt.Sprint(gotSegment.Ports, gotSegment.IncomingCoordinates, gotSegment.OutgoingCoordinates,
				gotSegment.Start, gotSegment.End, got.list)
			if wantState != gotState {
				t.Fatalf("round %d from port %d: recursive %s, iterative %s", round, start, wantState, gotState)
			}
		}
	}
}

// TestGeneratorBytesBoundTheSlot: a generator whose recycled
// dependencies or segment lists are large is dropped; the byte sizes used
// are at least the types'.
func TestGeneratorBytesBoundTheSlot(t *testing.T) {
	DrainGenerators()
	generator := NewGenerator(WestToEast, 10)
	for count := 0; count < 1<<16; count++ { // 2.5 MB of recycled dependencies
		generator.alloc.dependency(RegularDependency, 1)
	}
	generator.Release()
	// Only the slot survives collections: an oversized value may go to the
	// sync.Pool under gc, never to the slot.
	runtime.GC()
	runtime.GC()
	if pooled := DrainGenerators(); pooled != 0 {
		t.Fatal("a generator holding 65536 recycled dependencies was kept")
	}
	generator = NewGenerator(WestToEast, 10)
	segment := generator.alloc.segment(generator.strategy)
	segment.IncomingCoordinates = make([]float64, 0, 1<<18) // 2 MB in one segment's list
	generator.Release()
	// Only the slot survives collections: an oversized value may go to the
	// sync.Pool under gc, never to the slot.
	runtime.GC()
	runtime.GC()
	if pooled := DrainGenerators(); pooled != 0 {
		t.Fatal("a generator holding a segment list of 2 MB was kept")
	}
	for _, size := range []struct {
		value any
		bytes int
	}{
		{HyperEdgeSegment{}, 216}, {HyperEdgeSegmentDependency{}, 32}, {portWalkFrame{}, 12},
	} {
		if actual := int(reflect.TypeOf(size.value).Size()); actual > size.bytes {
			t.Errorf("%T takes %d bytes, the bound counts %d", size.value, actual, size.bytes)
		}
	}
}
