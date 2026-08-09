// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p2layers

import (
	"testing"

	"github.com/d2lang/elk-go/internal/layered"
)

func TestAlternativeLayerersProduceForwardSourceShapedChain(t *testing.T) {
	tests := []struct {
		name    string
		layerer layered.LayoutProcessor
	}{
		{"longest-path", &LongestPathLayerer{}},
		{"coffman-graham", &CoffmanGrahamLayerer{}},
		{"interactive", &InteractiveLayerer{}},
		{"stretch-width", &StretchWidthLayerer{}},
		{"min-width", &MinWidthLayerer{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			arena, graphID, edges := alternativeLayeringChain()
			if err := test.layerer.Process(arena, graphID); err != nil {
				t.Fatal(err)
			}
			if len(arena.Graph(graphID).LayerlessNodes) != 0 {
				t.Fatal("layerless nodes were not cleared")
			}
			for _, edgeID := range edges {
				source := arena.EdgeSourceNode(edgeID)
				target := arena.EdgeTargetNode(edgeID)
				if arena.LayerIndex(*arena.Node(source).Layer) >= arena.LayerIndex(*arena.Node(target).Layer) {
					t.Fatalf("edge %d does not advance a layer", edgeID)
				}
			}
		})
	}
}

func alternativeLayeringChain() (*layered.LGraphArena, layered.LGraphID, []layered.LEdgeID) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	nodes := []layered.LNodeID{arena.AddNode(graphID), arena.AddNode(graphID), arena.AddNode(graphID)}
	ports := make([]layered.LPortID, len(nodes))
	for index, nodeID := range nodes {
		arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, nodeID)
		arena.Node(nodeID).Position.X = float64(index * 100)
		arena.Node(nodeID).Size.X = 30
		arena.Node(nodeID).Size.Y = 20
		ports[index] = arena.AddPort()
		arena.SetPortNode(ports[index], &nodeID)
	}
	edges := make([]layered.LEdgeID, 0, len(nodes)-1)
	for index := 0; index+1 < len(nodes); index++ {
		edgeID := arena.AddEdge()
		source, target := ports[index], ports[index+1]
		arena.SetEdgeSource(edgeID, &source)
		arena.SetEdgeTarget(edgeID, &target)
		edges = append(edges, edgeID)
	}
	return arena, graphID, edges
}
