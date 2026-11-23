// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package selfloops

import (
	"testing"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

func TestConcentricLabelsAndAnchors(t *testing.T) {
	model := graph.NewModel()
	node := model.AddNode(&model.Root)
	model.Node(node).Shape.SetDimensions(120, 60)
	for i := 0; i < 3; i++ {
		id := model.AddEdge(&model.Root)
		model.AddSource(id, graph.NodeRef(node))
		model.AddTarget(id, graph.NodeRef(node))
		label := model.AddLabel(graph.EdgeElementRef(id), "retry")
		model.Label(label).Shape.SetDimensions(120, 22)
	}
	if plans := Collect(model, model.Root); plans != nil {
		t.Fatal("extension should be disabled by default")
	}
	graph.Set(&model.Node(model.Root).Properties, enabled, true)
	p := Collect(model, model.Root)[node]
	p.Arrange([]Side{East})
	right := p.Right
	p.Arrange([]Side{West})
	if p.Left != right || p.Right != 0 {
		t.Fatalf("replanned margins: %+v", p)
	}
	p.Write(model, 40, 70)
	for i := range model.Edges {
		s := model.Section(model.Edges[i].Sections[0])
		if s.Start.X != 40 || s.End.X != 40 {
			t.Errorf("loop %d lost shape attachments", i)
		}
		got := model.Label(model.Edges[i].Labels[0]).Shape
		if got.X+got.Width >= 40 || got.X < 40-p.Left {
			t.Errorf("label outside reserved lane: %+v", got)
		}
		if i > 0 {
			prev := model.Section(model.Edges[i-1].Sections[0])
			prevLabel := model.Label(model.Edges[i-1].Labels[0]).Shape
			if s.Start.Y >= prev.Start.Y || s.End.Y <= prev.End.Y || got.X+got.Width >= prevLabel.X {
				t.Errorf("loop %d does not have distinct anchors and label space", i)
			}
		}
	}
}

func TestPortRoutes(t *testing.T) {
	for _, pair := range [][2]Side{{West, East}, {East, North}, {South, West}, {West, West}} {
		model := graph.NewModel()
		graph.Set(&model.Node(model.Root).Properties, enabled, true)
		node := model.AddNode(&model.Root)
		model.Node(node).Shape.SetDimensions(100, 50)
		anchors := []geometry.Point{{X: 50}, {X: 100, Y: 25}, {X: 50, Y: 50}, {Y: 25}}
		names := []string{"NORTH", "EAST", "SOUTH", "WEST"}
		makePort := func(side Side) graph.ShapeID {
			id := model.AddPort(node)
			v := anchors[side]
			model.Port(id).Shape.SetLocation(v.X, v.Y)
			graph.Set(&model.Port(id).Properties, core.PortSide, names[side])
			return graph.PortRef(id)
		}
		src := makePort(pair[0])
		dst := src
		if pair[0] != pair[1] {
			dst = makePort(pair[1])
		}
		id := model.AddEdge(&model.Root)
		model.AddSource(id, src)
		model.AddTarget(id, dst)
		p := Collect(model, model.Root)[node]
		p.Write(model, 0, 0)
		s := model.Section(model.Edge(id).Sections[0])
		if s.Start != anchors[pair[0]] || s.End != anchors[pair[1]] || len(s.BendPoints) < 2 || *s.IncomingShape != src || *s.OutgoingShape != dst {
			t.Fatalf("ports %v: %+v", pair, s)
		}
	}
}

func TestOuterArmsClearInnerLabel(t *testing.T) {
	for _, side := range []Side{North, East, South, West} {
		m := graph.NewModel()
		graph.Set(&m.Node(m.Root).Properties, enabled, true)
		graph.Set(&m.Node(m.Root).Properties, loopSpacing, 50.)
		n := m.AddNode(&m.Root)
		m.Node(n).Shape.SetDimensions(72, 66)
		for _, label := range []string{"heartbeat", "message"} {
			e := m.AddEdge(&m.Root)
			m.AddSource(e, graph.NodeRef(n))
			m.AddTarget(e, graph.NodeRef(n))
			l := m.AddLabel(graph.EdgeElementRef(e), label)
			m.Label(l).Shape.SetDimensions(90, 90)
		}
		p := Collect(m, m.Root)[n]
		p.Arrange([]Side{side})
		p.Write(m, 0, 0)
		inner := m.Label(0).Shape.Rectangle()
		outer := m.SectionChain(m.Edge(1).Sections[0])
		for i := 1; i < len(outer); i++ {
			if entersBox(outer[i-1], outer[i], inner) {
				t.Fatalf("side %d: outer loop segment %v to %v crosses inner label %+v", side, outer[i-1], outer[i], inner)
			}
		}
		center := m.Label(1).Shape.Rectangle().Center()
		if center.Sub(outer.PointAt(outer.Length()/2)).Length() > 1e-8 {
			t.Errorf("outer label lost its midpoint on side %d", side)
		}
	}
}
