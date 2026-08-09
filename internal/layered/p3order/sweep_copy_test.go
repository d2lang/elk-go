// Copyright (c) 2016 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"reflect"
	"testing"

	"github.com/d2lang/elk-go/internal/layered"
)

func TestSweepCopyRestoresNodeAndPortOrder(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = []layered.LayerID{layerID}
	first := addCountingNode(arena, graphID, layerID, layered.EastSide)
	second := addCountingNode(arena, graphID, layerID, layered.EastSide)
	extra := arena.AddPort()
	arena.SetPortNode(extra, &first)
	arena.SetPortSide(extra, layered.EastSide)
	wantPorts := append([]layered.LPortID(nil), arena.Node(first).Ports...)
	copy := NewSweepCopy(arena, [][]layered.LNodeID{{first, second}})

	arena.Layer(layerID).Nodes[0], arena.Layer(layerID).Nodes[1] = second, first
	arena.Node(first).Ports[0], arena.Node(first).Ports[1] = arena.Node(first).Ports[1], arena.Node(first).Ports[0]
	copy.TransferNodeAndPortOrdersToGraph(arena, graphID, false)
	if got := arena.Layer(layerID).Nodes; !reflect.DeepEqual(got, []layered.LNodeID{first, second}) {
		t.Fatalf("nodes = %v", got)
	}
	if got := arena.Node(first).Ports; !reflect.DeepEqual(got, wantPorts) {
		t.Fatalf("ports = %v, want %v", got, wantPorts)
	}
}

func TestSweepCopyCloneIsIndependent(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayer(graphID)
	nodeID := addCountingNode(arena, graphID, layerID, layered.EastSide)
	original := NewSweepCopy(arena, [][]layered.LNodeID{{nodeID}})
	clone := original.Clone()
	clone.nodeOrder[0][0] = 99
	clone.portOrders[0][0][0] = 99
	if original.nodeOrder[0][0] != nodeID || original.portOrders[0][0][0] != arena.Node(nodeID).Ports[0] {
		t.Fatal("clone aliases original")
	}
}
