// Copyright (c) 2016 Kiel University and others.
// Copyright (c) 2016, 2018 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"testing"

	"github.com/d2lang/elk-go/internal/layered"
)

func TestCrossingsCounterBetweenLayers(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	leftLayer := arena.AddLayer(graphID)
	rightLayer := arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = append(arena.Graph(graphID).Layers, leftLayer, rightLayer)

	leftNodes := []layered.LNodeID{
		addCountingNode(arena, graphID, leftLayer, layered.EastSide),
		addCountingNode(arena, graphID, leftLayer, layered.EastSide),
	}
	rightNodes := []layered.LNodeID{
		addCountingNode(arena, graphID, rightLayer, layered.WestSide),
		addCountingNode(arena, graphID, rightLayer, layered.WestSide),
	}
	connectCountingPorts(arena, arena.Node(leftNodes[0]).Ports[0], arena.Node(rightNodes[1]).Ports[0])
	connectCountingPorts(arena, arena.Node(leftNodes[1]).Ports[0], arena.Node(rightNodes[0]).Ports[0])

	counter := NewCrossingsCounter(make([]int, len(arena.Ports)))
	if got := counter.CountCrossingsBetweenLayers(arena, leftNodes, rightNodes); got != 1 {
		t.Fatalf("crossing count = %d, want 1", got)
	}
}

func TestCrossingsCounterBetweenLayersNoCross(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	leftLayer := arena.AddLayer(graphID)
	rightLayer := arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = append(arena.Graph(graphID).Layers, leftLayer, rightLayer)
	left := []layered.LNodeID{
		addCountingNode(arena, graphID, leftLayer, layered.EastSide),
		addCountingNode(arena, graphID, leftLayer, layered.EastSide),
	}
	right := []layered.LNodeID{
		addCountingNode(arena, graphID, rightLayer, layered.WestSide),
		addCountingNode(arena, graphID, rightLayer, layered.WestSide),
	}
	connectCountingPorts(arena, arena.Node(left[0]).Ports[0], arena.Node(right[0]).Ports[0])
	connectCountingPorts(arena, arena.Node(left[1]).Ports[0], arena.Node(right[1]).Ports[0])
	counter := NewCrossingsCounter(make([]int, len(arena.Ports)))
	if got := counter.CountCrossingsBetweenLayers(arena, left, right); got != 0 {
		t.Fatalf("crossing count = %d, want 0", got)
	}
}

func addCountingNode(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	layerID layered.LayerID,
	side layered.PortSide,
) layered.LNodeID {
	nodeID := arena.AddNode(graphID)
	node := arena.Node(nodeID)
	node.ID = len(arena.Layer(layerID).Nodes)
	arena.SetNodeLayer(nodeID, &layerID)
	portID := arena.AddPort()
	arena.Port(portID).ID = len(arena.Ports) - 1
	arena.SetPortNode(portID, &nodeID)
	arena.SetPortSide(portID, side)
	return nodeID
}

func connectCountingPorts(arena *layered.LGraphArena, source, target layered.LPortID) layered.LEdgeID {
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &source)
	arena.SetEdgeTarget(edgeID, &target)
	return edgeID
}
