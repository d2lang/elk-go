// Copyright (c) 2016 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"testing"

	"github.com/d2lang/elk-go/internal/layered"
)

func TestGreedyPortDistributorSwapsCrossingPorts(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	leftLayer, rightLayer := arena.AddLayer(graphID), arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = []layered.LayerID{leftLayer, rightLayer}
	leftTop := addCountingNode(arena, graphID, leftLayer, layered.EastSide)
	leftBottom := addCountingNode(arena, graphID, leftLayer, layered.EastSide)
	right := arena.AddNode(graphID)
	arena.SetNodeLayer(right, &rightLayer)
	rightFirst, rightSecond := arena.AddPort(), arena.AddPort()
	arena.SetPortNode(rightFirst, &right)
	arena.SetPortNode(rightSecond, &right)
	arena.SetPortSide(rightFirst, layered.WestSide)
	arena.SetPortSide(rightSecond, layered.WestSide)
	connectCountingPorts(arena, arena.Node(leftTop).Ports[0], rightFirst)
	connectCountingPorts(arena, arena.Node(leftBottom).Ports[0], rightSecond)
	order := [][]layered.LNodeID{{leftTop, leftBottom}, {right}}
	distributor := NewGreedyPortDistributor()
	InitializeGreedyPortDistributor(distributor, arena, order)

	if !distributor.DistributePortsWhileSweeping(arena, order, 1, true) {
		t.Fatal("expected a crossing-reducing port swap")
	}
	if got := arena.Node(right).Ports; got[0] != rightSecond || got[1] != rightFirst {
		t.Fatalf("right port order = %v, want [%v %v]", got, rightSecond, rightFirst)
	}
	if distributor.DistributePortsWhileSweeping(arena, order, 1, true) {
		t.Fatal("already optimal port order must remain unchanged")
	}
}
