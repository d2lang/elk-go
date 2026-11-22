// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package orthogonal_test

import (
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/layered/p5edges"
	"github.com/d2lang/elk-go/internal/layered/p5edges/orthogonal"
)

// routerArena is two layers with one edge from the left node's
// east port to the right node's west port; without a target the edge is
// malformed and routing it panics.
func routerArena(withTarget bool) (*layered.LGraphArena, layered.LGraphID) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	leftLayer, rightLayer := arena.AddLayerToGraph(graphID), arena.AddLayerToGraph(graphID)
	leftNode, rightNode := arena.AddNode(graphID), arena.AddNode(graphID)
	arena.Node(leftNode).Size = geometry.Point{X: 30, Y: 20}
	arena.Node(rightNode).Size = geometry.Point{X: 40, Y: 20}
	arena.SetNodeLayer(leftNode, &leftLayer)
	arena.SetNodeLayer(rightNode, &rightLayer)
	leftPort, rightPort := arena.AddPort(), arena.AddPort()
	arena.SetPortNode(leftPort, &leftNode)
	arena.SetPortNode(rightPort, &rightNode)
	arena.SetPortSide(leftPort, layered.EastSide)
	arena.SetPortSide(rightPort, layered.WestSide)
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &leftPort)
	if withTarget {
		arena.SetEdgeTarget(edgeID, &rightPort)
	}
	return arena, graphID
}

// TestPanickingRouterDropsItsGenerator: an orthogonal edge
// routing that panics while its generator routes (here on an edge without a
// target) must not give the generator back to the pool, whose per-call
// scratch it may have left half-updated. A normal routing gives it back.
func TestPanickingRouterDropsItsGenerator(t *testing.T) {
	orthogonal.DrainGenerators()
	arena, graphID := routerArena(false)
	panicked := func() (recovered any) {
		defer func() { recovered = recover() }()
		_ = (&p5edges.OrthogonalEdgeRouter{}).Process(arena, graphID)
		return nil
	}()
	if panicked == nil {
		t.Fatal("the malformed arena did not make the router panic")
	}
	if pooled := orthogonal.DrainGenerators(); pooled != 0 {
		t.Fatalf("a panicking routing pooled %d generator(s)", pooled)
	}

	arena, graphID = routerArena(true)
	if err := (&p5edges.OrthogonalEdgeRouter{}).Process(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if pooled := orthogonal.DrainGenerators(); pooled != 1 {
		t.Fatalf("a completed routing pooled %d generators, want 1", pooled)
	}
}
