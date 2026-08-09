// Copyright (c) 2015 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"testing"

	"github.com/d2lang/elk-go/internal/layered"
)

func TestNodeRelativePortRanks(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	leftLayer, rightLayer := arena.AddLayer(graphID), arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = []layered.LayerID{leftLayer, rightLayer}
	left := addCountingNode(arena, graphID, leftLayer, layered.EastSide)
	right := addCountingNode(arena, graphID, rightLayer, layered.WestSide)
	second := arena.AddPort()
	arena.SetPortNode(second, &left)
	arena.SetPortSide(second, layered.EastSide)
	connectCountingPorts(arena, arena.Node(left).Ports[0], arena.Node(right).Ports[0])
	connectCountingPorts(arena, second, arena.Node(right).Ports[0])
	order := [][]layered.LNodeID{{left}, {right}}
	distributor := NewBarycenterPortDistributor(NodeRelativePortDistributor, 2)
	InitializeBarycenterPortDistributor(distributor, arena, order)
	distributor.CalculatePortRanks(arena, order[0], layered.PortTypeOutput)
	firstRank := distributor.PortRanks[arena.Port(arena.Node(left).Ports[0]).ID]
	secondRank := distributor.PortRanks[arena.Port(second).ID]
	if firstRank != float32(1.0/3.0) || secondRank != float32(2.0/3.0) {
		t.Fatalf("ranks = %v, %v", firstRank, secondRank)
	}
}

func TestLayerTotalPortRanksAccumulateAcrossNodes(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	leftLayer, rightLayer := arena.AddLayer(graphID), arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = []layered.LayerID{leftLayer, rightLayer}
	first := addCountingNode(arena, graphID, leftLayer, layered.EastSide)
	second := addCountingNode(arena, graphID, leftLayer, layered.EastSide)
	target := addCountingNode(arena, graphID, rightLayer, layered.WestSide)
	connectCountingPorts(arena, arena.Node(first).Ports[0], arena.Node(target).Ports[0])
	connectCountingPorts(arena, arena.Node(second).Ports[0], arena.Node(target).Ports[0])
	order := [][]layered.LNodeID{{first, second}, {target}}
	distributor := NewBarycenterPortDistributor(LayerTotalPortDistributor, 2)
	InitializeBarycenterPortDistributor(distributor, arena, order)
	distributor.CalculatePortRanks(arena, order[0], layered.PortTypeOutput)
	if got := distributor.PortRanks[arena.Port(arena.Node(first).Ports[0]).ID]; got != 1 {
		t.Fatalf("first rank = %v", got)
	}
	if got := distributor.PortRanks[arena.Port(arena.Node(second).Ports[0]).ID]; got != 2 {
		t.Fatalf("second rank = %v", got)
	}
}
