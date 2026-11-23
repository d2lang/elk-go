// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"slices"
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

func TestCompoundRouteClearsSiblingAndRetainsInsetPort(t *testing.T) {
	m := graph.NewModel()
	root := m.Root
	m.Node(root).Shape.SetDimensions(320, 200)
	add := func(parent graph.NodeID, x, y, w, h float64) graph.NodeID {
		id := m.AddNode(&parent)
		m.Node(id).Shape.SetLocation(x, y)
		m.Node(id).Shape.SetDimensions(w, h)
		return id
	}
	source := add(root, 20, 70, 40, 40)
	wall := add(root, 100, 40, 48, 115)
	container := add(root, 175, 25, 125, 140)
	target := add(container, 20, 45, 68, 65)
	port := m.AddPort(target)
	m.Port(port).Shape.SetLocation(0, 17)
	m.Port(port).Shape.SetDimensions(8, 6)
	graph.Set(&m.Port(port).Properties, PortSide, "WEST")
	graph.Set(&m.Port(port).Properties, PortAnchor, geometry.Point{X: 3, Y: 2})

	id := m.AddEdge(&root)
	m.AddSource(id, graph.NodeRef(source))
	m.AddTarget(id, graph.PortRef(port))
	if err := routeCompoundEdge(m, id, .5, nil); err != nil {
		t.Fatal(err)
	}
	path := m.SectionChain(m.Edge(id).Sections[0])
	want := geometry.Point{X: 198, Y: 89}
	if path[len(path)-1] != want {
		t.Fatalf("port attachment = %v, want %v", path[len(path)-1], want)
	}
	before := path[len(path)-2]
	if before.Y != want.Y || want.X-before.X < 16 {
		t.Fatalf("route %v needs a clear, level approach to the west port", path)
	}
	obstacle := m.Node(wall).Shape.Rectangle()
	for i, p := range path[1:] {
		q := path[i]
		if p.X == q.X {
			if p.X >= obstacle.X && p.X <= obstacle.MaxX() && max(p.Y, q.Y) >= obstacle.Y && min(p.Y, q.Y) <= obstacle.MaxY() {
				t.Fatalf("route %v touches intervening node %+v", path, obstacle)
			}
		} else if p.Y == q.Y {
			if p.Y >= obstacle.Y && p.Y <= obstacle.MaxY() && max(p.X, q.X) >= obstacle.X && min(p.X, q.X) <= obstacle.MaxX() {
				t.Fatalf("route %v touches intervening node %+v", path, obstacle)
			}
		} else {
			t.Fatalf("route has a diagonal segment %v to %v", q, p)
		}
	}
}

func TestCompoundPathRemovesRadialRoundoff(t *testing.T) {
	// Radial placed an external node at a height numerically just below
	// the center of its nested target. Preserve the exact endpoints.
	start := geometry.Point{X: 453.10352763219259, Y: 144.99999999999994}
	end := geometry.Point{X: 165, Y: 145}
	path := geometry.Chain{start, {X: 170, Y: start.Y}, {X: 170, Y: end.Y}, end}
	if got := compactCompoundPath(path); !slices.Equal(got, geometry.Chain{start, end}) {
		t.Fatalf("aligned route has a spurious bend: %v", got)
	}
}

func TestCompoundPortLeadsFitNarrowGap(t *testing.T) {
	m := graph.NewModel()
	root := m.Root
	m.Node(root).Shape.SetDimensions(160, 120)
	src := m.AddNode(&root)
	m.Node(src).Shape.SetLocation(10, 20)
	m.Node(src).Shape.SetDimensions(30, 50)
	outer := m.AddNode(&root)
	m.Node(outer).Shape.SetLocation(50, 10)
	m.Node(outer).Shape.SetDimensions(80, 90)
	dst := m.AddNode(&outer)
	m.Node(dst).Shape.SetLocation(15, 10)
	m.Node(dst).Shape.SetDimensions(40, 50)
	port := func(owner graph.NodeID, side string, x, y float64) graph.ShapeID {
		id := m.AddPort(owner)
		m.Port(id).Shape.SetLocation(x, y)
		graph.Set(&m.Port(id).Properties, PortSide, side)
		return graph.PortRef(id)
	}
	id := m.AddEdge(&root)
	m.AddSource(id, port(src, "EAST", 30, 25))
	m.AddTarget(id, port(dst, "WEST", 0, 27))
	if err := routeCompoundEdge(m, id, .5, nil); err != nil {
		t.Fatal(err)
	}
	path := m.SectionChain(m.Edge(id).Sections[0])
	if path.Length() > 27.000001 {
		t.Fatalf("route takes a detour beyond the 25-by-2 gap: %v", path)
	}
	for i := 2; i < len(path); i++ {
		if path[i-1].Sub(path[i-2]).Dot(path[i].Sub(path[i-1])) < 0 {
			t.Fatalf("route doubles back in narrow gap: %v", path)
		}
	}
}
