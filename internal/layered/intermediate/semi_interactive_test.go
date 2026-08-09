// Copyright (c) 2016, 2020 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestSemiInteractiveCrossingConstrainsPositionedNormalNodes(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayerToGraph(graphID)
	first := arena.AddNode(graphID)
	second := arena.AddNode(graphID)
	unpositioned := arena.AddNode(graphID)
	dummy := arena.AddNode(graphID)
	arena.Node(dummy).Type = layered.LongEdgeNode
	for _, nodeID := range []layered.LNodeID{first, second, unpositioned, dummy} {
		arena.SetNodeLayer(nodeID, &layerID)
	}
	basegraph.Set(&arena.Node(first).Properties, layered.PositionOption, geometry.Point{Y: 90})
	basegraph.Set(&arena.Node(second).Properties, layered.PositionOption, geometry.Point{Y: 10})
	basegraph.Set(&arena.Node(dummy).Properties, layered.PositionOption, geometry.Point{Y: 5})

	ProcessSemiInteractiveCrossing(arena, graphID)
	if got := basegraph.Get(&arena.Node(second).Properties, layered.InLayerSuccessorConstraints); len(got) != 1 || got[0] != first {
		t.Fatalf("successors = %v, want [%d]", got, first)
	}
	if got := basegraph.Get(&arena.Node(first).Properties, layered.InLayerSuccessorConstraints); len(got) != 0 {
		t.Fatalf("last positioned node successors = %v", got)
	}
	if !basegraph.Get(&arena.Graph(graphID).Properties, layered.InLayerSuccessorConstraintsBetweenNonDummies) {
		t.Fatal("non-dummy constraint flag was not set")
	}
}
