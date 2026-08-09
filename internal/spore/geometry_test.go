// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package spore

import (
	"math"
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
)

func TestOverlapMovesRectanglesUntilTheyTouch(t *testing.T) {
	first := geometry.Rectangle{X: 0, Y: 0, Width: 40, Height: 80}
	rectangles := []geometry.Rectangle{
		{X: 0, Y: 0, Width: 70, Height: 20},
		{X: 10, Y: 50, Width: 70, Height: 20},
		{X: -40, Y: 30, Width: 70, Height: 20},
		{X: -60, Y: 70, Width: 70, Height: 20},
		{X: -10, Y: 70, Width: 70, Height: 20},
		{X: -20, Y: -10, Width: 70, Height: 20},
		{X: 10, Y: 20, Width: 20, Height: 20},
		{X: -20, Y: -20, Width: 100, Height: 120},
		{X: 0, Y: -0.001, Width: 40, Height: 80},
	}
	for _, second := range rectangles {
		factor := overlap(first, second)
		direction := second.Center().Sub(first.Center())
		newCenter := first.Center().Add(direction.Scale(factor))
		moved := second.Move(newCenter.Sub(second.Center()))
		if distance := shortestDistance(first, moved); distance < -doubleEpsilon {
			t.Fatalf("rect %+v still overlaps by %v after factor %v", second, distance, factor)
		}
	}
}

func TestNodeUnderlapMatchesDirectionalDistance(t *testing.T) {
	firstRect := geometry.Rectangle{X: 0, Y: 0, Width: 20, Height: 60}
	secondRects := []geometry.Rectangle{
		{X: 40, Y: 20, Width: 20, Height: 20},
		{X: 30, Y: 70, Width: 20, Height: 20},
		{X: -40, Y: 40, Width: 20, Height: 20},
		{X: 20, Y: 0, Width: 20, Height: 20},
	}
	for _, secondRect := range secondRects {
		firstVertex := &vertex{point: firstRect.Center()}
		secondVertex := &vertex{point: secondRect.Center()}
		first := newNode(firstVertex, firstRect, 0)
		second := newNode(secondVertex, secondRect, 1)
		underlap := first.underlap(second)
		direction := first.vertex.Sub(second.vertex)
		if distance := first.distance(second, direction); math.Abs(underlap-distance) > 1e-7 {
			t.Fatalf("underlap %v != directional distance %v for %+v", underlap, distance, secondRect)
		}
		second.translate(direction.ScaleToLength(underlap))
		if distance := shortestDistance(first.rect, second.rect); math.Abs(distance) > doubleEpsilon {
			t.Fatalf("distance after compaction = %v for %+v", distance, secondRect)
		}
	}
}

func TestScanlineFindsWideRectangleOverlap(t *testing.T) {
	rectangles := []geometry.Rectangle{
		{X: 0, Y: 30, Width: 160, Height: 20},
		{X: 150, Y: 40, Width: 160, Height: 20},
		{X: 150, Y: 0, Width: 20, Height: 20},
		{X: 150, Y: 70, Width: 20, Height: 20},
	}
	nodes := make([]*node, len(rectangles))
	for index, rect := range rectangles {
		original := &vertex{point: rect.Center()}
		nodes[index] = newNode(original, rect, 0)
	}
	edges := scanlineOverlaps(nodes)
	if len(edges) != 1 || !sameEdge(edges[0], &tEdge{u: nodes[0].original, v: nodes[1].original}) {
		t.Fatalf("scanline edges = %#v, want only the two wide rectangles", edges)
	}
}

func TestBowyerWatsonProducesConnectedUniqueGraph(t *testing.T) {
	points := []geometry.Point{{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 20, Y: 80}, {X: 70, Y: 60}, {X: 40, Y: 30}}
	vertices := make([]*vertex, len(points))
	for index, point := range points {
		vertices[index] = &vertex{point: point}
	}
	edges := triangulate(vertices)
	for index, edge := range edges {
		for _, other := range edges[index+1:] {
			if sameEdge(edge, other) {
				t.Fatal("triangulation contains a duplicate undirected edge")
			}
		}
	}
	seen := map[*vertex]bool{vertices[0]: true}
	for changed := true; changed; {
		changed = false
		for _, edge := range edges {
			if seen[edge.u] && !seen[edge.v] {
				seen[edge.v], changed = true, true
			}
			if seen[edge.v] && !seen[edge.u] {
				seen[edge.u], changed = true, true
			}
		}
	}
	if len(seen) != len(vertices) {
		t.Fatalf("triangulation reached %d/%d vertices", len(seen), len(vertices))
	}
}
