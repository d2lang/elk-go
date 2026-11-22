// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package orthogonal

import (
	"reflect"
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// reuseScenario builds two layers whose ports share y coordinates:
// edges 10->30 and 30->10 conflict critically in both directions (so one of
// their segments is split), and port 50 fans out to 50 and 70 (a hyperedge
// segment, so a junction point is recorded).
func reuseScenario() (*layered.LGraphArena, layered.LGraphID, []layered.LNodeID, []layered.LNodeID, []layered.LEdgeID) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	leftLayer := arena.AddLayerToGraph(graphID)
	rightLayer := arena.AddLayerToGraph(graphID)
	left := arena.AddNode(graphID)
	arena.SetNodeLayer(left, &leftLayer)
	right := arena.AddNode(graphID)
	arena.SetNodeLayer(right, &rightLayer)
	arena.Node(right).Position.X = 100
	ports := func(node layered.LNodeID, side layered.PortSide, ys ...float64) map[float64]layered.LPortID {
		made := map[float64]layered.LPortID{}
		for _, y := range ys {
			port := arena.AddPort()
			arena.SetPortNode(port, &node)
			arena.SetPortSide(port, side)
			arena.Port(port).Position.Y = y
			made[y] = port
		}
		return made
	}
	out := ports(left, layered.EastSide, 10, 30, 50)
	in := ports(right, layered.WestSide, 10, 30, 50, 70)
	var edges []layered.LEdgeID
	for _, pair := range [][2]float64{{10, 30}, {30, 10}, {50, 50}, {50, 70}} {
		source, target := out[pair[0]], in[pair[1]]
		edge := arena.AddEdge()
		arena.SetEdgeSource(edge, &source)
		arena.SetEdgeTarget(edge, &target)
		edges = append(edges, edge)
	}
	return arena, graphID, []layered.LNodeID{left}, []layered.LNodeID{right}, edges
}

type routeResult struct {
	slots     int
	bends     [][]geometry.Point
	junctions [][]geometry.Point
}

func routeScenario(t *testing.T, generator *Generator) routeResult {
	t.Helper()
	arena, graphID, left, right, edges := reuseScenario()
	result := routeResult{slots: generator.RouteEdges(arena, graphID, left, 0, right, 20)}
	if n := len(generator.strategy.createdJunctionPoints); n != 0 {
		t.Fatalf("RouteEdges returned with %d remembered junction points", n)
	}
	for _, edge := range edges {
		result.bends = append(result.bends, append([]geometry.Point(nil), arena.Edge(edge).BendPoints...))
		result.junctions = append(result.junctions, basegraph.Get(&arena.Edge(edge).Properties, layered.JunctionPoints))
	}
	return result
}

// TestPooledGeneratorStartsClean pins the invariant
// that makes pooling the routing generator safe: RouteEdges cannot return
// (and cannot unwind from its bend-point loop) with junction points remembered, so a
// generator handed out again starts with an empty set and routes exactly as
// a new one. The scenario takes both the split-partner path and the
// junction-point path in one call.
func TestPooledGeneratorStartsClean(t *testing.T) {
	fresh := &Generator{strategy: newRoutingDirectionStrategy(WestToEast)}
	fresh.edgeSpacing = 10
	fresh.conflictThreshold = conflictThresholdFactor * 10
	want := routeScenario(t, fresh)

	split, junctions := false, 0
	for _, bends := range want.bends {
		if len(bends) == 4 {
			split = true
		}
	}
	for _, points := range want.junctions {
		junctions += len(points)
	}
	if !split || junctions == 0 {
		t.Fatalf("scenario does not take both paths: split=%v junctions=%d bends=%v", split, junctions, want.bends)
	}

	for round := 0; round < 3; round++ {
		generator := NewGenerator(WestToEast, 10)
		if n := len(generator.strategy.createdJunctionPoints); n != 0 {
			t.Fatalf("round %d: NewGenerator handed out %d remembered junction points", round, n)
		}
		got := routeScenario(t, generator)
		generator.Release()
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("round %d: pooled generator routed %+v, a new one %+v", round, got, want)
		}
	}
}
