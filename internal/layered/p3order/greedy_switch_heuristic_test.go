// Copyright (c) 2015 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestTwoSidedGreedySwitchRemovesSimpleCrossing(t *testing.T) {
	testGreedySwitchRemovesSimpleCrossing(t, CrossMinTwoSidedGreedySwitch)
}

func TestOneSidedGreedySwitchRemovesSimpleCrossing(t *testing.T) {
	testGreedySwitchRemovesSimpleCrossing(t, CrossMinOneSidedGreedySwitch)
}

func testGreedySwitchRemovesSimpleCrossing(t *testing.T, crossMinType CrossMinType) {
	t.Helper()
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
	basegraph.Set(&arena.Graph(graphID).Properties, layered.Random, javacompat.NewRandom(1))

	if err := NewLayerSweepCrossingMinimizer(crossMinType).Process(arena, graphID); err != nil {
		t.Fatal(err)
	}
	order := [][]layered.LNodeID{
		append([]layered.LNodeID(nil), arena.Layer(leftLayer).Nodes...),
		append([]layered.LNodeID(nil), arena.Layer(rightLayer).Nodes...),
	}
	counter := NewAllCrossingsCounter(2)
	InitializeAllCrossingsCounter(counter, arena, order)
	if got := counter.CountAllCrossings(arena, order); got != 0 {
		t.Fatalf("greedy-switch crossings = %d, want 0; order = %v", got, order)
	}
}

func TestGreedySwitchActivationMatchesGraphConfigurator(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	for index := 0; index < 2; index++ {
		arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, arena.AddNode(graphID))
	}
	strategy, active := GreedySwitchIntermediateForGraph(arena, graphID)
	if !active || strategy != layered.TwoSidedGreedySwitch {
		t.Fatalf("default greedy switch = (%v, %v), want TWO_SIDED active", strategy, active)
	}

	for len(arena.Graph(graphID).LayerlessNodes) < 40 {
		arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, arena.AddNode(graphID))
	}
	if _, active := GreedySwitchIntermediateForGraph(arena, graphID); active {
		t.Fatal("activation threshold 40 must not activate for exactly 40 nodes")
	}
	basegraph.Set(&arena.Graph(graphID).Properties, layered.GreedySwitchActivationThreshold, 0)
	if _, active := GreedySwitchIntermediateForGraph(arena, graphID); !active {
		t.Fatal("zero activation threshold must force greedy switch")
	}

	basegraph.Set(&arena.Graph(graphID).Properties, layered.Hierarchy, layered.HierarchyIncludeChildren)
	if _, active := GreedySwitchIntermediateForGraph(arena, graphID); active {
		t.Fatal("hierarchical greedy switch defaults to OFF")
	}
}
