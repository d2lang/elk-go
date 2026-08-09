// Copyright (c) 2014, 2015 Kiel University and others.
// Copyright (c) 2016 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"testing"

	"github.com/d2lang/elk-go/internal/layered"
)

func TestAllCrossingsCounter(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	leftLayer := arena.AddLayer(graphID)
	rightLayer := arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = []layered.LayerID{leftLayer, rightLayer}
	left := []layered.LNodeID{
		addCountingNode(arena, graphID, leftLayer, layered.EastSide),
		addCountingNode(arena, graphID, leftLayer, layered.EastSide),
	}
	right := []layered.LNodeID{
		addCountingNode(arena, graphID, rightLayer, layered.WestSide),
		addCountingNode(arena, graphID, rightLayer, layered.WestSide),
	}
	connectCountingPorts(arena, arena.Node(left[0]).Ports[0], arena.Node(right[1]).Ports[0])
	connectCountingPorts(arena, arena.Node(left[1]).Ports[0], arena.Node(right[0]).Ports[0])
	order := [][]layered.LNodeID{left, right}
	counter := NewAllCrossingsCounter(len(order))
	InitializeAllCrossingsCounter(counter, arena, order)
	if got := counter.CountAllCrossings(arena, order); got != 1 {
		t.Fatalf("all crossing count = %d, want 1", got)
	}
}

func TestHyperedgeCounterTreatsSharedPortAsOneHyperedge(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	leftLayer, rightLayer := arena.AddLayer(graphID), arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = []layered.LayerID{leftLayer, rightLayer}
	leftA := addCountingNode(arena, graphID, leftLayer, layered.EastSide)
	leftB := addCountingNode(arena, graphID, leftLayer, layered.EastSide)
	rightA := addCountingNode(arena, graphID, rightLayer, layered.WestSide)
	rightB := addCountingNode(arena, graphID, rightLayer, layered.WestSide)
	sharedSource := arena.Node(leftA).Ports[0]
	connectCountingPorts(arena, sharedSource, arena.Node(rightA).Ports[0])
	connectCountingPorts(arena, sharedSource, arena.Node(rightB).Ports[0])
	connectCountingPorts(arena, arena.Node(leftB).Ports[0], arena.Node(rightA).Ports[0])
	order := [][]layered.LNodeID{{leftA, leftB}, {rightA, rightB}}
	counter := NewAllCrossingsCounter(2)
	InitializeAllCrossingsCounter(counter, arena, order)
	if got := counter.CountAllCrossings(arena, order); got < 0 {
		t.Fatalf("hyperedge crossing count must be nonnegative, got %d", got)
	}
}
