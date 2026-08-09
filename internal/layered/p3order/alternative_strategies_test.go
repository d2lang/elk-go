// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"testing"

	"github.com/d2lang/elk-go/internal/layered"
)

func TestInteractiveCrossingMinimizerUsesPreviousVerticalOrder(t *testing.T) {
	arena, graphID, first, second := alternativeOrderingLayer()
	if err := (&InteractiveCrossingMinimizer{}).Process(arena, graphID); err != nil {
		t.Fatal(err)
	}
	nodes := arena.Layer(arena.Graph(graphID).Layers[0]).Nodes
	if len(nodes) != 2 || nodes[0] != second || nodes[1] != first {
		t.Fatalf("interactive order = %v, want [%d %d]", nodes, second, first)
	}
}

func TestNoCrossingMinimizerPreservesPreviousOrder(t *testing.T) {
	arena, graphID, first, second := alternativeOrderingLayer()
	if err := (&NoCrossingMinimizer{}).Process(arena, graphID); err != nil {
		t.Fatal(err)
	}
	nodes := arena.Layer(arena.Graph(graphID).Layers[0]).Nodes
	if len(nodes) != 2 || nodes[0] != first || nodes[1] != second {
		t.Fatalf("preserved order = %v, want [%d %d]", nodes, first, second)
	}
}

func alternativeOrderingLayer() (*layered.LGraphArena, layered.LGraphID, layered.LNodeID, layered.LNodeID) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = append(arena.Graph(graphID).Layers, layerID)
	first, second := arena.AddNode(graphID), arena.AddNode(graphID)
	arena.SetNodeLayer(first, &layerID)
	arena.SetNodeLayer(second, &layerID)
	arena.Node(first).Position.Y = 100
	arena.Node(second).Position.Y = 0
	return arena, graphID, first, second
}
