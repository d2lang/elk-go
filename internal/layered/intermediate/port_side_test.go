// Copyright (c) 2011, 2015 Kiel University and others.
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

func TestProcessPortSidesUsesNetFlowAndFixesConstraints(t *testing.T) {
	arena, graphID := layered.NewLGraphArena(), layered.LGraphID(0)
	arena.AddGraph()
	source := arena.AddNode(graphID)
	target := arena.AddNode(graphID)
	arena.Graph(graphID).LayerlessNodes = []layered.LNodeID{source, target}
	sourcePort, targetPort := arena.AddPort(), arena.AddPort()
	arena.SetPortNode(sourcePort, &source)
	arena.SetPortNode(targetPort, &target)
	edge := arena.AddEdge()
	arena.SetEdgeSource(edge, &sourcePort)
	arena.SetEdgeTarget(edge, &targetPort)

	ProcessPortSides(arena, graphID)

	if got := arena.Port(sourcePort).Side; got != layered.EastSide {
		t.Fatalf("source side = %v, want EAST", got)
	}
	if got := arena.Port(targetPort).Side; got != layered.WestSide {
		t.Fatalf("target side = %v, want WEST", got)
	}
	for _, nodeID := range []layered.LNodeID{source, target} {
		if got := basegraph.Get(&arena.Node(nodeID).Properties, layered.PortConstraintsOption); got != layered.PortConstraintsFixedSide {
			t.Fatalf("node %d constraints = %v, want FIXED_SIDE", nodeID, got)
		}
	}
}

func TestProcessPortSidesPreservesFixedAndInheritsDummySide(t *testing.T) {
	arena, graphID := layered.NewLGraphArena(), layered.LGraphID(0)
	arena.AddGraph()
	node := arena.AddNode(graphID)
	dummy := arena.AddNode(graphID)
	arena.Graph(graphID).LayerlessNodes = []layered.LNodeID{node, dummy}
	basegraph.Set(&arena.Node(node).Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedPosition)
	basegraph.Set(&arena.Node(dummy).Properties, layered.ExternalPortSide, layered.NorthSide)
	fixed, inherited := arena.AddPort(), arena.AddPort()
	arena.SetPortNode(fixed, &node)
	arena.SetPortNode(inherited, &node)
	arena.SetPortSide(fixed, layered.SouthSide)
	basegraph.Set(&arena.Port(inherited).Properties, layered.PortDummy, dummy)

	ProcessPortSides(arena, graphID)

	if got := arena.Port(fixed).Side; got != layered.SouthSide {
		t.Fatalf("fixed side = %v, want SOUTH", got)
	}
	if got := arena.Port(inherited).Side; got != layered.NorthSide {
		t.Fatalf("dummy side = %v, want NORTH", got)
	}
}
