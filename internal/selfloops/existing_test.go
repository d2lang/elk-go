// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package selfloops

import (
	"reflect"
	"testing"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

func TestSuppliedLoopKeepsGeometryAndTopology(t *testing.T) {
	m := graph.NewModel()
	graph.Set(&m.Node(m.Root).Properties, enabled, true)
	node := m.AddNode(&m.Root)
	m.Node(node).Shape.SetLocation(100, 200)
	m.Node(node).Shape.SetDimensions(80, 60)
	id := m.AddEdge(&m.Root)
	m.AddSource(id, graph.NodeRef(node))
	m.AddTarget(id, graph.NodeRef(node))
	first, last := m.AddSection(id), m.AddSection(id)
	s := m.Section(first)
	s.Identifier, s.IdentifierPresent = "custom", true
	s.Start, s.End = geometry.Point{X: 100, Y: 220}, geometry.Point{X: 20, Y: 230}
	s.StartPresent, s.EndPresent = true, true
	s.BendPoints = geometry.Chain{{X: 20, Y: 220}}
	s.OutgoingSections = []graph.SectionID{last}
	s = m.Section(last)
	s.IncomingSections = []graph.SectionID{first}
	s.Start, s.End = geometry.Point{X: 20, Y: 230}, geometry.Point{X: 100, Y: 240}
	s.StartPresent, s.EndPresent = true, true
	s.BendPoints = geometry.Chain{{X: 20, Y: 240}}
	graph.Set(&m.Edge(id).Properties, core.JunctionPoints, geometry.Chain{{X: 20, Y: 230}})
	label := m.AddLabel(graph.EdgeElementRef(id), "custom label")
	m.Label(label).Shape.SetLocation(10, 222)
	m.Label(label).Shape.SetDimensions(20, 16)

	p := Collect(m, m.Root)[node]
	p.Arrange([]Side{North})
	if p.Left != 90 || p.Right != 0 || p.Top != 0 || p.Bottom != 0 {
		t.Fatalf("supplied loop margins: left %g, right %g, top %g, bottom %g", p.Left, p.Right, p.Top, p.Bottom)
	}
	p.TranslateExisting(m, 110, 205)
	p.Write(m, 243, 51)
	p.Write(m, 243, 51)
	if firstSection := m.Section(first); firstSection.Identifier != "custom" || !reflect.DeepEqual(firstSection.OutgoingSections, []graph.SectionID{last}) ||
		!reflect.DeepEqual(m.Section(last).IncomingSections, []graph.SectionID{first}) || !reflect.DeepEqual(m.Edge(id).Sections, []graph.SectionID{first, last}) {
		t.Fatal("supplied sections lost their identities or topology")
	}
	want := geometry.Chain{{X: 243, Y: 71}, {X: 163, Y: 71}, {X: 163, Y: 81}, {X: 163, Y: 81}, {X: 163, Y: 91}, {X: 243, Y: 91}}
	if got := append(m.SectionChain(first), m.SectionChain(last)...); !reflect.DeepEqual(got, want) {
		t.Fatalf("translated supplied route = %v, want %v", got, want)
	}
	if got := m.Label(label).Shape.Position(); got != (geometry.Point{X: 153, Y: 73}) {
		t.Errorf("translated label = %v", got)
	}
	if got := graph.Get(&m.Edge(id).Properties, core.JunctionPoints); !reflect.DeepEqual(got, geometry.Chain{{X: 163, Y: 81}}) {
		t.Errorf("translated junction = %v", got)
	}
}
