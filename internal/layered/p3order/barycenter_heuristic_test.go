// Copyright (c) 2010, 2015 Kiel University and others.
// Copyright (c) 2016 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"testing"

	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestBarycenterHeuristicRemovesSimpleCrossing(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	leftLayer, rightLayer := arena.AddLayer(graphID), arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = []layered.LayerID{leftLayer, rightLayer}
	leftTop := addCountingNode(arena, graphID, leftLayer, layered.EastSide)
	leftBottom := addCountingNode(arena, graphID, leftLayer, layered.EastSide)
	rightTop := addCountingNode(arena, graphID, rightLayer, layered.WestSide)
	rightBottom := addCountingNode(arena, graphID, rightLayer, layered.WestSide)
	connectCountingPorts(arena, arena.Node(leftTop).Ports[0], arena.Node(rightBottom).Ports[0])
	connectCountingPorts(arena, arena.Node(leftBottom).Ports[0], arena.Node(rightTop).Ports[0])
	order := [][]layered.LNodeID{{leftTop, leftBottom}, {rightTop, rightBottom}}
	distributor := NewBarycenterPortDistributor(LayerTotalPortDistributor, 2)
	InitializeBarycenterPortDistributor(distributor, arena, order)
	resolver := NewForsterConstraintResolver(arena, order)
	InitializeForsterConstraintResolver(resolver, arena, order)
	heuristic := NewBarycenterHeuristic(arena, resolver, javacompat.NewRandom(1), distributor)
	heuristic.MinimizeCrossings(order, 1, true, true)
	if order[1][0] != rightBottom || order[1][1] != rightTop {
		t.Fatalf("right order = %v", order[1])
	}
}

func TestBarycenterFillUnknownPreservesPreorder(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = []layered.LayerID{layerID}
	first := addCountingNode(arena, graphID, layerID, layered.EastSide)
	second := addCountingNode(arena, graphID, layerID, layered.EastSide)
	order := [][]layered.LNodeID{{first, second}}
	distributor := NewBarycenterPortDistributor(NodeRelativePortDistributor, 1)
	InitializeBarycenterPortDistributor(distributor, arena, order)
	resolver := NewForsterConstraintResolver(arena, order)
	InitializeForsterConstraintResolver(resolver, arena, order)
	heuristic := NewBarycenterHeuristic(arena, resolver, javacompat.NewRandom(1), distributor)
	heuristic.fillUnknownBarycenters(order[0], true)
	if got := *resolver.state(arena, first).Barycenter; got != -.5 {
		t.Fatalf("first barycenter = %v", got)
	}
	if got := *resolver.state(arena, second).Barycenter; got != 0 {
		t.Fatalf("second barycenter = %v", got)
	}
}
