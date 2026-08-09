// Copyright (c) 2021 Kiel University and others.
// Copyright (c) 2022 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestModelOrderBarycenterKeepsRealNodeOrder(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = []layered.LayerID{layerID}
	first := addCountingNode(arena, graphID, layerID, layered.EastSide)
	dummy := addCountingNode(arena, graphID, layerID, layered.EastSide)
	second := addCountingNode(arena, graphID, layerID, layered.EastSide)
	basegraph.Set(&arena.Node(first).Properties, layered.ModelOrder, 0)
	basegraph.Set(&arena.Node(second).Properties, layered.ModelOrder, 1)
	order := [][]layered.LNodeID{{first, dummy, second}}
	distributor := NewBarycenterPortDistributor(NodeRelativePortDistributor, 1)
	InitializeBarycenterPortDistributor(distributor, arena, order)
	resolver := NewForsterConstraintResolver(arena, order)
	InitializeForsterConstraintResolver(resolver, arena, order)
	heuristic := NewBarycenterHeuristic(arena, resolver, javacompat.NewRandom(1), distributor)
	heuristic.ModelOrder = NewModelOrderBarycenterHeuristic(true)
	resolver.state(arena, first).Barycenter = floatPtr(2)
	resolver.state(arena, dummy).Barycenter = floatPtr(0)
	resolver.state(arena, second).Barycenter = floatPtr(1)
	heuristic.ModelOrder.sort(heuristic, order[0])
	firstIndex, secondIndex := -1, -1
	for index, nodeID := range order[0] {
		if nodeID == first {
			firstIndex = index
		} else if nodeID == second {
			secondIndex = index
		}
	}
	if firstIndex >= secondIndex {
		t.Fatalf("model nodes reversed: %v", order[0])
	}
}
