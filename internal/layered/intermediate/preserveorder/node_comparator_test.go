// Copyright (c) 2020 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package preserveorder

import (
	"reflect"
	"slices"
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestNodeComparatorOrdersRealNodesByModel(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	low := arena.AddNode(graphID)
	high := arena.AddNode(graphID)
	basegraph.Set(&arena.Node(low).Properties, layered.ModelOrder, 1)
	basegraph.Set(&arena.Node(high).Properties, layered.ModelOrder, 2)
	order := []layered.LNodeID{high, low}
	comparator := NewNodeComparator(arena, nil, layered.OrderingNodesAndEdges, DummyNodeOver, false)
	slices.SortStableFunc(order, comparator.Compare)
	if want := []layered.LNodeID{low, high}; !reflect.DeepEqual(order, want) {
		t.Fatalf("node order = %v, want %v", order, want)
	}
}

func TestNodeComparatorOrdersDummiesByPreviousLayer(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	sourceA := arena.AddNode(graphID)
	sourceB := arena.AddNode(graphID)
	dummyFromB := arena.AddNode(graphID)
	dummyFromA := arena.AddNode(graphID)
	arena.Node(dummyFromA).Type = layered.LongEdgeNode
	arena.Node(dummyFromB).Type = layered.LongEdgeNode
	connectComparatorNodes(arena, sourceB, dummyFromB, 10)
	connectComparatorNodes(arena, sourceA, dummyFromA, 11)
	order := []layered.LNodeID{dummyFromB, dummyFromA}
	comparator := NewNodeComparator(
		arena, []layered.LNodeID{sourceA, sourceB}, layered.OrderingNodesAndEdges, DummyNodeOver, false)
	slices.SortStableFunc(order, comparator.Compare)
	if want := []layered.LNodeID{dummyFromA, dummyFromB}; !reflect.DeepEqual(order, want) {
		t.Fatalf("dummy order = %v, want %v", order, want)
	}
}

func TestNodeComparatorModelOrderTieBuildsAssociation(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	first := arena.AddNode(graphID)
	second := arena.AddNode(graphID)
	basegraph.Set(&arena.Node(first).Properties, layered.ModelOrder, 7)
	basegraph.Set(&arena.Node(second).Properties, layered.ModelOrder, 7)
	comparator := NewNodeComparator(arena, nil, layered.OrderingNodesAndEdges, DummyNodeOver, false)
	if got := comparator.Compare(first, second); got != -1 {
		t.Fatalf("first tie comparison = %d, want -1", got)
	}
	if got := comparator.Compare(second, first); got != 1 {
		t.Fatalf("reverse remembered association = %d, want 1", got)
	}
}

func connectComparatorNodes(
	arena *layered.LGraphArena,
	source, target layered.LNodeID,
	modelOrder int,
) {
	sourcePort := arena.AddPort()
	targetPort := arena.AddPort()
	arena.SetPortNode(sourcePort, &source)
	arena.SetPortNode(targetPort, &target)
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &sourcePort)
	arena.SetEdgeTarget(edgeID, &targetPort)
	basegraph.Set(&arena.Edge(edgeID).Properties, layered.ModelOrder, modelOrder)
}
