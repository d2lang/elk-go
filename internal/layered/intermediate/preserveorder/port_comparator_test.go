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

func TestPortComparatorIncomingPortsUseReversePreviousLayerOrder(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	sourceA := arena.AddNode(graphID)
	sourceB := arena.AddNode(graphID)
	target := arena.AddNode(graphID)
	_, incomingA := connectPorts(arena, sourceA, target, 1)
	_, incomingB := connectPorts(arena, sourceB, target, 2)
	ports := []layered.LPortID{incomingA, incomingB}
	comparator := NewPortComparator(arena, []layered.LNodeID{sourceA, sourceB}, nil)
	slices.SortStableFunc(ports, comparator.Compare)
	if want := []layered.LPortID{incomingB, incomingA}; !reflect.DeepEqual(ports, want) {
		t.Fatalf("incoming port order = %v, want %v", ports, want)
	}
}

func TestPortComparatorPreservesSameTargetOrderBug(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	source := arena.AddNode(graphID)
	target := arena.AddNode(graphID)
	first, _ := connectPorts(arena, source, target, 7)
	second, _ := connectPorts(arena, source, target, 2)
	basegraph.Set(&arena.Port(first).Properties, layered.LongEdgeTargetNode, target)
	basegraph.Set(&arena.Port(second).Properties, layered.LongEdgeTargetNode, target)
	comparator := NewPortComparator(arena, nil, map[layered.LNodeID]int{target: 2})
	if got := comparator.Compare(first, second); got != 0 {
		t.Fatalf("same-target comparison = %d, want 0 from ELK 0.8.1 p1-order reuse", got)
	}
	basegraph.Set(&arena.Edge(arena.Port(first).OutgoingEdges[0]).Properties, layered.Reversed, true)
	if got := comparator.Compare(first, second); got != 1 {
		t.Fatalf("reversed edge comparison = %d, want 1", got)
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
