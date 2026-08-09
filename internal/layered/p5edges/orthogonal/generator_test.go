// Copyright (c) 2010, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package orthogonal

import (
	"reflect"
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestHyperEdgeSegmentMaintainsSortedUniqueCoordinatesAndSplits(t *testing.T) {
	segment := NewTestSegment([]float64{5, 0, 5}, []float64{9, 3})
	// Test constructors normalize synthetic input; graph construction uses the
	// same insertion rule incrementally.
	if got, want := segment.IncomingCoordinates, []float64{0, 5, 5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("incoming coordinates = %v, want %v", got, want)
	}
	segment.IncomingCoordinates = insertSortedUnique(segment.IncomingCoordinates[:2], 5)
	if got, want := segment.IncomingCoordinates, []float64{0, 5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("insertSortedUnique = %v, want %v", got, want)
	}
	segment.RecomputeExtent()
	partner := segment.splitAt(4)
	if got, want := segment.OutgoingCoordinates, []float64{4}; !reflect.DeepEqual(got, want) {
		t.Fatalf("first outgoing = %v, want %v", got, want)
	}
	if got, want := partner.OutgoingCoordinates, []float64{3, 9}; !reflect.DeepEqual(got, want) {
		t.Fatalf("partner outgoing = %v, want %v", got, want)
	}
	if segment.SplitPartner != partner || partner.SplitPartner != segment || !partner.IsDummy() {
		t.Fatal("split partner linkage does not match ELK")
	}
}

func TestCycleDetectorFindsLowerWeightArcInRegularTwoCycle(t *testing.T) {
	first, second := NewTestSegment(nil, nil), NewTestSegment(nil, nil)
	AddRegularDependency(first, second, 1)
	AddRegularDependency(second, first, 2)
	cycles := DetectCycles([]*HyperEdgeSegment{first, second}, false, javacompat.NewRandom(1))
	if len(cycles) != 1 || cycles[0].Weight != 1 {
		t.Fatalf("cycles = %#v, want one weight-1 dependency", cycles)
	}
}

func TestCycleDetectorCriticalOnlyIgnoresRegularDependencies(t *testing.T) {
	first, second := NewTestSegment(nil, nil), NewTestSegment(nil, nil)
	AddRegularDependency(first, second, 1)
	AddRegularDependency(second, first, 1)
	if cycles := DetectCycles([]*HyperEdgeSegment{first, second}, true, javacompat.NewRandom(1)); len(cycles) != 0 {
		t.Fatalf("critical-only cycles = %d, want 0", len(cycles))
	}
}

func TestCycleDetectorReturnsCriticalDependency(t *testing.T) {
	first, second := NewTestSegment(nil, nil), NewTestSegment(nil, nil)
	AddCriticalDependency(first, second)
	AddCriticalDependency(second, first)
	cycles := DetectCycles([]*HyperEdgeSegment{first, second}, true, javacompat.NewRandom(1))
	if len(cycles) != 1 || cycles[0].Type != CriticalDependency {
		t.Fatalf("cycles = %#v, want one critical dependency", cycles)
	}
}

func TestCreateDependencyMakesSymmetricZeroWeightTwoCycle(t *testing.T) {
	generator := NewGenerator(WestToEast, 10)
	first := NewTestSegment([]float64{0}, []float64{5})
	second := NewTestSegment([]float64{5}, []float64{0})
	if critical := generator.CreateDependencyIfNecessary(first, second); critical != 0 {
		t.Fatalf("critical dependencies = %d, want 0", critical)
	}
	if len(first.OutgoingDependencies) != 1 || len(second.OutgoingDependencies) != 1 ||
		first.OutgoingDependencies[0].Weight != 0 || second.OutgoingDependencies[0].Weight != 0 ||
		first.OutgoingDependencies[0].Target != second || second.OutgoingDependencies[0].Target != first {
		t.Fatal("symmetric penalties did not create ELK's zero-weight two-cycle")
	}
}

func TestBreakNonCriticalCyclesReversesPositiveWeightDependency(t *testing.T) {
	first := NewTestSegment([]float64{0}, []float64{1})
	second := NewTestSegment([]float64{1}, []float64{0})
	toReverse := AddRegularDependency(first, second, 1)
	AddRegularDependency(second, first, 2)
	BreakNonCriticalCycles([]*HyperEdgeSegment{first, second}, javacompat.NewRandom(1))
	if toReverse.Source != second || toReverse.Target != first {
		t.Fatalf("dependency = %p -> %p, want second -> first", toReverse.Source, toReverse.Target)
	}
}

func TestTopologicalNumberingAssignsIncreasingSlots(t *testing.T) {
	first := NewTestSegment([]float64{0}, []float64{4})
	second := NewTestSegment([]float64{5}, []float64{9})
	AddRegularDependency(first, second, 1)
	TopologicalNumbering([]*HyperEdgeSegment{first, second})
	if first.RoutingSlot != 0 || second.RoutingSlot != 1 {
		t.Fatalf("slots = %d, %d; want 0, 1", first.RoutingSlot, second.RoutingSlot)
	}
}

func TestSegmentSplitterPrefersRegularEdgeOverHyperedge(t *testing.T) {
	generator := NewGenerator(WestToEast, 10)
	splitter := &segmentSplitter{generator: generator}
	hyperedge := NewTestSegment([]float64{0, 6}, []float64{3})
	regular := NewTestSegment([]float64{2}, []float64{5})
	dependency := AddCriticalDependency(hyperedge, regular)
	segments := []*HyperEdgeSegment{hyperedge, regular}
	splitter.splitSegments([]*HyperEdgeSegmentDependency{dependency}, &segments, 0.5)
	if len(segments) != 3 {
		t.Fatalf("segments = %d, want 3", len(segments))
	}
	if regular.SplitPartner == nil || hyperedge.SplitPartner != nil {
		t.Fatal("splitter did not prefer splitting the regular edge")
	}
	if !hasDependency(regular, hyperedge, CriticalDependency) ||
		!hasDependency(hyperedge, regular.SplitPartner, CriticalDependency) {
		t.Fatal("splitter did not install the two critical ordering dependencies")
	}
}

func hasDependency(source, target *HyperEdgeSegment, kind DependencyType) bool {
	for _, dependency := range source.OutgoingDependencies {
		if dependency.Target == target && dependency.Type == kind {
			return true
		}
	}
	return false
}

func TestGeneratorRoutesEdgeBetweenLayers(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	leftLayer := arena.AddLayerToGraph(graphID)
	rightLayer := arena.AddLayerToGraph(graphID)
	leftNode, rightNode := arena.AddNode(graphID), arena.AddNode(graphID)
	arena.SetNodeLayer(leftNode, &leftLayer)
	arena.SetNodeLayer(rightNode, &rightLayer)
	arena.Node(rightNode).Position.Y = 30
	leftPort, rightPort := arena.AddPort(), arena.AddPort()
	arena.SetPortNode(leftPort, &leftNode)
	arena.SetPortNode(rightPort, &rightNode)
	arena.SetPortSide(leftPort, layered.EastSide)
	arena.SetPortSide(rightPort, layered.WestSide)
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &leftPort)
	arena.SetEdgeTarget(edgeID, &rightPort)

	slots := NewGenerator(WestToEast, 10).RouteEdges(
		arena, graphID, []layered.LNodeID{leftNode}, 0, []layered.LNodeID{rightNode}, 20,
	)
	if slots != 1 {
		t.Fatalf("slots = %d, want 1", slots)
	}
	if got, want := arena.Edge(edgeID).BendPoints, (geometry.Chain{{X: 20}, {X: 20, Y: 30}}); !reflect.DeepEqual(got, want) {
		t.Fatalf("bend points = %v, want %v", got, want)
	}
}

func TestDirectionStrategiesUseExpectedPortSides(t *testing.T) {
	tests := []struct {
		direction      RoutingDirection
		source, target layered.PortSide
	}{
		{WestToEast, layered.EastSide, layered.WestSide},
		{NorthToSouth, layered.SouthSide, layered.NorthSide},
		{SouthToNorth, layered.NorthSide, layered.SouthSide},
	}
	for _, test := range tests {
		strategy := newRoutingDirectionStrategy(test.direction)
		if strategy.sourcePortSide() != test.source || strategy.targetPortSide() != test.target {
			t.Fatalf("direction %d sides = %v/%v, want %v/%v", test.direction,
				strategy.sourcePortSide(), strategy.targetPortSide(), test.source, test.target)
		}
	}
}
