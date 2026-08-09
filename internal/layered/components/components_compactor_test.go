// Copyright (c) 2016 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package components

import (
	"reflect"
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestRectilinearConvexHullSplitsRectangle(t *testing.T) {
	points := []hullPoint{
		newHullPoint(0, 0), newHullPoint(0, 10),
		newHullPoint(20, 0), newHullPoint(20, 10),
	}
	want := []geometry.Rectangle{{X: 0, Y: 0, Width: 20, Height: 10}}
	if got := splitRectilinearHull(points); !reflect.DeepEqual(got, want) {
		t.Fatalf("rectilinear hull = %#v, want %#v", got, want)
	}
}

func TestRectilinearConvexHullPreservesOrthogonalLShape(t *testing.T) {
	points := []hullPoint{
		newHullPoint(0, 0), newHullPoint(0, 10),
		newHullPoint(30, 0), newHullPoint(30, 10),
		newHullPoint(0, 10), newHullPoint(0, 30),
		newHullPoint(10, 10), newHullPoint(10, 30),
	}
	want := []geometry.Rectangle{
		{X: 0, Y: 0, Width: 10, Height: 30},
		{X: 10, Y: 0, Width: 20, Height: 10},
	}
	if got := splitRectilinearHull(points); !reflect.DeepEqual(got, want) {
		t.Fatalf("rectilinear hull = %#v, want %#v", got, want)
	}
}

func TestConnectedComponentsCompactorClosesSimpleRowGap(t *testing.T) {
	arena := layered.NewLGraphArena()
	firstGraph, secondGraph := arena.AddGraph(), arena.AddGraph()
	first := addComponentNode(arena, firstGraph)
	second := addComponentNode(arena, secondGraph)
	arena.Node(first).Size = geometry.Point{X: 30, Y: 20}
	arena.Node(second).Position = geometry.Point{X: 100}
	arena.Node(second).Size = geometry.Point{X: 10, Y: 40}

	size, err := compactConnectedComponents(
		arena, []layered.LGraphID{firstGraph, secondGraph}, 15)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := arena.Node(first).Position, (geometry.Point{}); got != want {
		t.Fatalf("first position = %v, want %v", got, want)
	}
	if got, want := arena.Node(second).Position, (geometry.Point{X: 45}); got != want {
		t.Fatalf("second position = %v, want %v", got, want)
	}
	if want := (geometry.Point{X: 55, Y: 40}); size != want {
		t.Fatalf("compacted size = %v, want %v", size, want)
	}
}
