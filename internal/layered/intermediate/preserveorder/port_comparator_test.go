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

func TestPortComparatorIncomingPortsUsePreviousLayerOrder(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	sourceA := arena.AddNode(graphID)
	sourceB := arena.AddNode(graphID)
	target := arena.AddNode(graphID)
	_, incomingA := connectPorts(arena, sourceA, target, 1)
	_, incomingB := connectPorts(arena, sourceB, target, 2)
	ports := []layered.LPortID{incomingA, incomingB}
	comparator := NewPortComparator(
		arena, []layered.LNodeID{sourceA, sourceB}, layered.OrderingNodesAndEdges, nil, false)
	slices.SortStableFunc(ports, comparator.Compare)
	if want := []layered.LPortID{incomingA, incomingB}; !reflect.DeepEqual(ports, want) {
		t.Fatalf("incoming port order = %v, want %v", ports, want)
	}
}

func TestPortComparatorUsesBothSameTargetEdgeOrders(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	source := arena.AddNode(graphID)
	target := arena.AddNode(graphID)
	first, _ := connectPorts(arena, source, target, 7)
	second, _ := connectPorts(arena, source, target, 2)
	basegraph.Set(&arena.Port(first).Properties, layered.LongEdgeTargetNode, target)
	basegraph.Set(&arena.Port(second).Properties, layered.LongEdgeTargetNode, target)
	comparator := NewPortComparator(
		arena, nil, layered.OrderingNodesAndEdges, map[layered.LNodeID]int{target: 2}, false)
	if got := comparator.Compare(first, second); got != 1 {
		t.Fatalf("same-target comparison = %d, want 1", got)
	}
	comparator.ClearTransitiveOrdering()
	if got := comparator.Compare(second, first); got != -1 {
		t.Fatalf("reverse same-target comparison = %d, want -1", got)
	}
}

func TestPortComparatorReversesIncomingWestPortOrder(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	sourceA := arena.AddNode(graphID)
	sourceB := arena.AddNode(graphID)
	target := arena.AddNode(graphID)
	_, incomingA := connectPorts(arena, sourceA, target, 1)
	_, incomingB := connectPorts(arena, sourceB, target, 2)
	arena.SetPortSide(incomingA, layered.WestSide)
	arena.SetPortSide(incomingB, layered.WestSide)
	ports := []layered.LPortID{incomingA, incomingB}
	comparator := NewPortComparator(
		arena, []layered.LNodeID{sourceA, sourceB}, layered.OrderingNodesAndEdges, nil, false)
	slices.SortStableFunc(ports, comparator.Compare)
	if want := []layered.LPortID{incomingB, incomingA}; !reflect.DeepEqual(ports, want) {
		t.Fatalf("west incoming port order = %v, want %v", ports, want)
	}
}

func connectPorts(
	arena *layered.LGraphArena,
	source, target layered.LNodeID,
	modelOrder int,
) (layered.LPortID, layered.LPortID) {
	sourcePort := arena.AddPort()
	targetPort := arena.AddPort()
	arena.SetPortNode(sourcePort, &source)
	arena.SetPortNode(targetPort, &target)
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &sourcePort)
	arena.SetEdgeTarget(edgeID, &targetPort)
	basegraph.Set(&arena.Edge(edgeID).Properties, layered.ModelOrder, modelOrder)
	return sourcePort, targetPort
}
