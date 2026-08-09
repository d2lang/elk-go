// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p1cycles

import (
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestAlternativeCycleBreakersRemoveSourceShapedCycle(t *testing.T) {
	tests := []struct {
		name    string
		breaker layered.LayoutProcessor
	}{
		{"depth-first", &DepthFirstCycleBreaker{}},
		{"interactive", &InteractiveCycleBreaker{}},
		{"model-order", &ModelOrderCycleBreaker{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			arena, graphID := alternativeCycleGraph()
			if err := test.breaker.Process(arena, graphID); err != nil {
				t.Fatal(err)
			}
			if hasCycle(arena, graphID) {
				t.Fatal("cycle remained after cycle breaking")
			}
		})
	}
}

func alternativeCycleGraph() (*layered.LGraphArena, layered.LGraphID) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	nodes := []layered.LNodeID{arena.AddNode(graphID), arena.AddNode(graphID), arena.AddNode(graphID)}
	ports := make([]layered.LPortID, len(nodes))
	for index, nodeID := range nodes {
		arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, nodeID)
		arena.Node(nodeID).Position.X = float64(index * 100)
		arena.Node(nodeID).Size.X = 30
		basegraph.Set(&arena.Node(nodeID).Properties, layered.ModelOrder, index)
		ports[index] = arena.AddPort()
		arena.SetPortNode(ports[index], &nodeID)
	}
	for index := range nodes {
		edgeID := arena.AddEdge()
		source, target := ports[index], ports[(index+1)%len(nodes)]
		arena.SetEdgeSource(edgeID, &source)
		arena.SetEdgeTarget(edgeID, &target)
	}
	return arena, graphID
}
