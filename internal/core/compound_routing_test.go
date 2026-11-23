// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package core

import (
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
