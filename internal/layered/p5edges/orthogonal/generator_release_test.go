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

// fanOutGraph is one source port with edges to two targets at different
// heights: RouteEdges makes one hyperedge segment and a junction point where
// the second edge branches off.
func fanOutGraph() (*layered.LGraphArena, layered.LGraphID, []layered.LNodeID, []layered.LNodeID, []layered.LEdgeID) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	leftLayer := arena.AddLayerToGraph(graphID)
	rightLayer := arena.AddLayerToGraph(graphID)
	source := arena.AddNode(graphID)
	arena.SetNodeLayer(source, &leftLayer)
	sourcePort := arena.AddPort()
	arena.SetPortNode(sourcePort, &source)
	arena.SetPortSide(sourcePort, layered.EastSide)
	var targets []layered.LNodeID
	var edges []layered.LEdgeID
	for index, y := range []float64{30, 60} {
		target := arena.AddNode(graphID)
		arena.SetNodeLayer(target, &rightLayer)
		arena.Node(target).Position.Y = y
		targetPort := arena.AddPort()
		arena.SetPortNode(targetPort, &target)
		arena.SetPortSide(targetPort, layered.WestSide)
		edge := arena.AddEdge()
		arena.SetEdgeSource(edge, &sourcePort)
		arena.SetEdgeTarget(edge, &targetPort)
		targets = append(targets, target)
		edges = append(edges, edge)
		_ = index
	}
	return arena, graphID, []layered.LNodeID{source}, targets, edges
}

func junctionPoints(t *testing.T, generator *Generator) [][]geometry.Point {
	arena, graphID, left, right, edges := fanOutGraph()
	generator.RouteEdges(arena, graphID, left, 0, right, 20)
	var result [][]geometry.Point
	for _, edge := range edges {
		result = append(result, basegraph.Get(&arena.Edge(edge).Properties, layered.JunctionPoints))
	}
	return result
}

// TestReleasedGeneratorForgetsJunctionPoints: a generator
// handed out by NewGenerator must start without remembered junction points.
// RouteEdges clears its set only at its end, so a RouteEdges call that
// panics after recording a junction point (the edge router's deferred
// Release then pools the generator) leaves the set filled; the state is
// simulated here by recording the point by hand before Release.
func TestReleasedGeneratorForgetsJunctionPoints(t *testing.T) {
	want := junctionPoints(t, NewGenerator(WestToEast, 10))
	if len(want[0])+len(want[1]) == 0 {
		t.Fatalf("scenario makes no junction point: %v", want)
	}
	// A generator whose RouteEdges was interrupted after it recorded the
	// junction points it had made so far.
	interrupted := NewGenerator(WestToEast, 10)
	for _, points := range want {
		for _, point := range points {
			interrupted.strategy.createdJunctionPoints[point] = struct{}{}
		}
	}
	interrupted.Release()
	got := junctionPoints(t, NewGenerator(WestToEast, 10))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("junction points after a reused generator = %v, fresh generator = %v", got, want)
	}
}
