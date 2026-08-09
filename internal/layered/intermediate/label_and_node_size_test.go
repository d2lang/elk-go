// Copyright (c) 2010, 2020 Kiel University and others.
// Copyright (c) 2017, 2019 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestProcessLabelAndNodeSizesDistributesOrderedPorts(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	nodeID := arena.AddNode(graphID)
	arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, nodeID)
	node := arena.Node(nodeID)
	node.Size.X, node.Size.Y = 90, 45
	basegraph.Set(&node.Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedOrder)

	for range 3 {
		portID := arena.AddPort()
		arena.SetPortNode(portID, &nodeID)
		arena.SetPortSide(portID, layered.SouthSide)
	}
	// Clockwise LNode order on SOUTH is right-to-left.
	node.Ports[0], node.Ports[2] = node.Ports[2], node.Ports[0]

	ProcessLabelAndNodeSizes(arena, graphID)
	want := []float64{67.5, 45, 22.5}
	for index, portID := range node.Ports {
		port := arena.Port(portID)
		if port.Position.X != want[index] || port.Position.Y != 45 {
			t.Fatalf("port %d position = %+v, want (%v, 45)", index, port.Position, want[index])
		}
	}
	if got := basegraph.Get(&node.Properties, layered.PortConstraintsOption); got != layered.PortConstraintsFixedPosition {
		t.Fatalf("constraints = %v, want FIXED_POS", got)
	}
}

func TestProcessLabelAndNodeSizesPreservesFixedPositions(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	nodeID := arena.AddNode(graphID)
	arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, nodeID)
	arena.Node(nodeID).Size.X = 40
	arena.Node(nodeID).Size.Y = 30
	basegraph.Set(&arena.Node(nodeID).Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedPosition)
	portID := arena.AddPort()
	arena.SetPortNode(portID, &nodeID)
	arena.SetPortSide(portID, layered.EastSide)
	arena.Port(portID).Position.X = 17
	arena.Port(portID).Position.Y = 23

	ProcessLabelAndNodeSizes(arena, graphID)
	if arena.Port(portID).Position.X != 40 || arena.Port(portID).Position.Y != 23 {
		t.Fatalf("fixed position border snap = %+v, want (40, 23)", arena.Port(portID).Position)
	}
}
