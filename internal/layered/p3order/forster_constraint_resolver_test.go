// Copyright (c) 2016, 2020 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"reflect"
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestForsterConstraintResolverMergesViolatedGroups(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = []layered.LayerID{layerID}
	first := addCountingNode(arena, graphID, layerID, layered.EastSide)
	second := addCountingNode(arena, graphID, layerID, layered.EastSide)
	basegraph.Set(&arena.Node(second).Properties, layered.InLayerSuccessorConstraints, []layered.LNodeID{first})
	order := [][]layered.LNodeID{{first, second}}
	resolver := NewForsterConstraintResolver(arena, order)
	InitializeForsterConstraintResolver(resolver, arena, order)
	resolver.state(arena, first).setBarycenter(0)
	resolver.state(arena, first).SummedWeight = 0
	resolver.state(arena, first).Degree = 1
	resolver.state(arena, second).setBarycenter(1)
	resolver.state(arena, second).SummedWeight = 1
	resolver.state(arena, second).Degree = 1
	nodes := append([]layered.LNodeID(nil), order[0]...)
	resolver.ProcessConstraints(arena, &nodes)
	if !reflect.DeepEqual(nodes, []layered.LNodeID{second, first}) {
		t.Fatalf("nodes = %v", nodes)
	}
	if got := resolver.state(arena, first).Barycenter; got != .5 {
		t.Fatalf("merged barycenter = %v", got)
	}
}
