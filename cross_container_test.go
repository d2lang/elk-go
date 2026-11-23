// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package elk_test

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"

	elk "github.com/d2lang/elk-go"
	"github.com/d2lang/elk-go/internal/geometry"
)

const hierarchyRouting = "elkgo.routeCrossContainerEdges"

func hierarchyFixture(algorithm string, edges ...*elk.Edge) *elk.Graph {
	outer := &elk.Node{ID: elk.StringID("outer"), Children: []*elk.Node{{
		ID: elk.StringID("inner"), Children: []*elk.Node{{
			ID: elk.StringID("target"), Width: elk.Float64(80), Height: elk.Float64(90),
		}},
	}}}
	for _, container := range []*elk.Node{outer, outer.Children[0]} {
		container.LayoutOptions = elk.LayoutOptions{"elk.padding": "[top=35,left=24,bottom=24,right=24]"}
		container.Labels = []*elk.Label{{Text: elk.String(container.ID.String()), Width: elk.Float64(45), Height: elk.Float64(16)}}
	}
	return &elk.Graph{
		ID: elk.StringID("root"),
		LayoutOptions: elk.LayoutOptions{
			"elk.algorithm": algorithm, "elk.hierarchyHandling": "INCLUDE_CHILDREN",
			"elk.json.shapeCoords": "ROOT", "elk.json.edgeCoords": "ROOT", hierarchyRouting: true,
		},
		Children: []*elk.Node{outer, {ID: elk.StringID("external"), Width: elk.Float64(68), Height: elk.Float64(40)}},
		Edges:    edges,
	}
}

func hierarchyEdge(id, source, target string) *elk.Edge {
	return &elk.Edge{ID: elk.StringID(id), Sources: []elk.ID{elk.StringID(source)}, Targets: []elk.ID{elk.StringID(target)}}
}

func nodeBounds(n *elk.Node) geometry.Rectangle {
	return geometry.Rectangle{X: elk.Number(n.X), Y: elk.Number(n.Y), Width: elk.Number(n.Width), Height: elk.Number(n.Height)}
}

func checkOnBorder(t *testing.T, point elk.Point, box geometry.Rectangle) {
	t.Helper()
	const tolerance = 1e-6
	xOK := point.X >= box.X-tolerance && point.X <= box.MaxX()+tolerance
	yOK := point.Y >= box.Y-tolerance && point.Y <= box.MaxY()+tolerance
	xEdge := math.Abs(point.X-box.X) < tolerance || math.Abs(point.X-box.MaxX()) < tolerance
	yEdge := math.Abs(point.Y-box.Y) < tolerance || math.Abs(point.Y-box.MaxY()) < tolerance
	if !(xOK && yOK && (xEdge || yEdge)) {
		t.Fatalf("endpoint %+v misses shape border %+v", point, box)
	}
}

func hierarchySection(t *testing.T, edge *elk.Edge) *elk.EdgeSection {
	t.Helper()
	if len(edge.Sections) != 1 {
		t.Fatalf("edge %s has %d sections", edge.ID, len(edge.Sections))
	}
	s := edge.Sections[0]
	if s.IncomingShape == nil || *s.IncomingShape != edge.Sources[0] || s.OutgoingShape == nil || *s.OutgoingShape != edge.Targets[0] {
		t.Fatalf("section does not identify the requested endpoints: %+v", s)
	}
	if s.StartPoint == nil || s.EndPoint == nil {
		t.Fatal("section lacks endpoints")
	}
	prev := *s.StartPoint
	for _, point := range append(append([]elk.Point(nil), s.BendPoints...), *s.EndPoint) {
		length := math.Hypot(point.X-prev.X, point.Y-prev.Y)
		if length == 0 || math.IsNaN(length) || math.IsInf(length, 0) {
			t.Fatalf("invalid segment %+v -> %+v", prev, point)
		}
		prev = point
	}
	return s
}

func TestCrossContainerPlacementAndPorts(t *testing.T) {
	for _, algorithm := range []string{"force", "stress", "mrtree", "radial"} {
		t.Run(algorithm, func(t *testing.T) {
			// Both edges have the same node owners. They must retain their
			// distinct ports when projected to the outer placement graph.
			g := hierarchyFixture(algorithm, hierarchyEdge("first", "external", "id"), hierarchyEdge("second", "external", "email"))
			outer, external := g.Children[0], g.Children[1]
			inner := outer.Children[0]
			table := inner.Children[0]
			table.LayoutOptions = elk.LayoutOptions{"elk.portConstraints": "FIXED_POS"}
			for i, name := range []string{"id", "email"} {
				table.Ports = append(table.Ports, &elk.Port{
					ID: elk.StringID(name), X: elk.Float64(0), Y: elk.Float64(20 + float64(i)*48),
					LayoutOptions: elk.LayoutOptions{"elk.port.side": "WEST"},
				})
			}
			if err := elk.Layout(g); err != nil {
				t.Fatal(err)
			}
			if nodeBounds(outer).Intersects(nodeBounds(external)) {
				t.Fatalf("external %+v overlaps the container %+v", nodeBounds(external), nodeBounds(outer))
			}
			for _, parent := range []*elk.Node{outer, inner} {
				box, child := nodeBounds(parent), nodeBounds(parent.Children[0])
				if child.X < box.X || child.Y < box.Y || child.MaxX() > box.MaxX() || child.MaxY() > box.MaxY() {
					t.Fatalf("parent %+v does not enclose child %+v", box, child)
				}
			}
			for i, edge := range g.Edges {
				s := hierarchySection(t, edge)
				checkOnBorder(t, *s.StartPoint, nodeBounds(external))
				want := elk.Point{X: elk.Number(table.X), Y: elk.Number(table.Y) + 20 + float64(i)*48}
				if math.Hypot(s.EndPoint.X-want.X, s.EndPoint.Y-want.Y) > 1e-6 {
					t.Errorf("edge %s ends at %+v, want row port %+v", edge.ID, s.EndPoint, want)
				}
			}
		})
	}
}

func TestCrossContainerAncestorEndpoints(t *testing.T) {
	for _, algorithm := range []string{"force", "stress", "mrtree", "radial"} {
		for _, reverse := range []bool{false, true} {
			source, target := "outer", "target"
			if reverse {
				source, target = target, source
			}
			t.Run(algorithm+"/"+source, func(t *testing.T) {
				g := hierarchyFixture(algorithm, hierarchyEdge("link", source, target))
				g.Children = g.Children[:1]
				outer := g.Children[0]
				inner := outer.Children[0]
				if err := elk.Layout(g); err != nil {
					t.Fatal(err)
				}
				s := hierarchySection(t, g.Edges[0])
				outerEnd, innerEnd := s.StartPoint, s.EndPoint
				if reverse {
					outerEnd, innerEnd = innerEnd, outerEnd
				}
				checkOnBorder(t, *outerEnd, nodeBounds(outer))
				checkOnBorder(t, *innerEnd, nodeBounds(inner.Children[0]))
				for _, container := range []*elk.Node{outer, inner} {
					label := container.Labels[0]
					title := geometry.Rectangle{X: elk.Number(label.X), Y: elk.Number(label.Y), Width: elk.Number(label.Width), Height: elk.Number(label.Height)}
					prev := *s.StartPoint
					for _, p := range append(append([]elk.Point(nil), s.BendPoints...), *s.EndPoint) {
						segment := geometry.Rectangle{X: math.Min(prev.X, p.X), Y: math.Min(prev.Y, p.Y), Width: math.Abs(p.X - prev.X), Height: math.Abs(p.Y - prev.Y)}
						if segment.Intersects(title) {
							t.Errorf("segment %+v -> %+v crosses %s title %+v", prev, p, container.ID, title)
						}
						prev = p
					}
				}
			})
		}
	}
}

func TestCrossContainerOptInPreservesDefaultAndLayered(t *testing.T) {
	for _, algorithm := range []string{"force", "stress", "mrtree", "radial", "layered"} {
		makeGraph := func() *elk.Graph { return hierarchyFixture(algorithm, hierarchyEdge("e", "external", "target")) }
		without, with := makeGraph(), makeGraph()
		delete(without.LayoutOptions, hierarchyRouting)
		if algorithm != "layered" {
			with.LayoutOptions[hierarchyRouting] = false
		}
		for _, g := range []*elk.Graph{without, with} {
			if err := elk.Layout(g); err != nil {
				t.Fatal(err)
			}
		}
		if algorithm != "layered" && len(without.Edges[0].Sections) != 0 {
			t.Fatalf("unmodified %s started routing hierarchy edges", algorithm)
		}
		if !reflect.DeepEqual(with.Children, without.Children) || !reflect.DeepEqual(with.Edges, without.Edges) {
			t.Errorf("option changed %s reference output", algorithm)
		}
	}
}

// The edge is declared on "sheet" but contained in "scope". Both are offset
// from the root, so all three edge coordinate modes have distinct origins.
func TestCrossContainerEdgeCoordinateModes(t *testing.T) {
	for _, mode := range []string{"ROOT", "PARENT", "CONTAINER"} {
		t.Run(mode, func(t *testing.T) {
			scope := hierarchyFixture("mrtree")
			scope.ID = elk.StringID("scope")
			delete(scope.LayoutOptions, hierarchyRouting)
			delete(scope.LayoutOptions, "elk.json.edgeCoords")
			table := scope.Children[0].Children[0].Children[0]
			table.LayoutOptions = elk.LayoutOptions{"elk.portConstraints": "FIXED_POS"}
			table.Ports = []*elk.Port{{
				ID: elk.StringID("field"), X: elk.Float64(80), Y: elk.Float64(65),
				LayoutOptions: elk.LayoutOptions{"elk.port.side": "EAST"},
			}}
			sheet := &elk.Node{
				ID: elk.StringID("sheet"), Children: []*elk.Node{scope}, Edges: []*elk.Edge{hierarchyEdge("e", "field", "external"), hierarchyEdge("loop", "external", "external")},
				LayoutOptions: elk.LayoutOptions{"elk.algorithm": "box"},
			}
			g := &elk.Graph{
				ID: elk.StringID("canvas"), Children: []*elk.Node{sheet},
				LayoutOptions: elk.LayoutOptions{
					"elk.algorithm": "box", "elk.json.shapeCoords": "ROOT", "elk.json.edgeCoords": mode,
					hierarchyRouting: true, "elkgo.routeSelfLoops": true,
				},
			}
			if err := elk.Layout(g); err != nil {
				t.Fatal(err)
			}
			edge := sheet.Edges[0]
			s := hierarchySection(t, edge)
			if mode == "CONTAINER" && (edge.Container == nil || *edge.Container != scope.ID) {
				t.Fatalf("container %v, want scope", edge.Container)
			}
			var offset geometry.Point
			switch mode {
			case "PARENT":
				offset = nodeBounds(sheet).Position()
			case "CONTAINER":
				offset = nodeBounds(scope).Position()
			}
			start := geometry.Point(*s.StartPoint).Add(offset)
			port := table.Ports[0]
			want := geometry.Point{X: elk.Number(port.X), Y: elk.Number(port.Y)}
			if start.Distance(want) > 1e-6 {
				t.Fatalf("absolute start %+v misses port at %+v (offset %+v)", start, want, offset)
			}
			end := geometry.Point(*s.EndPoint).Add(offset)
			checkOnBorder(t, elk.Point(end), nodeBounds(scope.Children[1]))
			// Feed the exported graph back through the public JSON API. Both
			// routes and the fixed named port must stay in their own frames.
			if len(sheet.Edges[1].Sections) == 0 {
				t.Fatal("missing native self-loop in mixed fixture")
			}
			input, err := json.Marshal(g)
			if err != nil {
				t.Fatal(err)
			}
			output, err := elk.LayoutJSON(input)
			if err != nil {
				t.Fatal(err)
			}
			var next elk.Graph
			if err := json.Unmarshal(output, &next); err != nil {
				t.Fatal(err)
			}
			nextSheet := next.Children[0]
			nextPort := nextSheet.Children[0].Children[0].Children[0].Children[0].Ports[0]
			if *port.X != *nextPort.X || *port.Y != *nextPort.Y || !reflect.DeepEqual(edge.Sections, nextSheet.Edges[0].Sections) || !reflect.DeepEqual(sheet.Edges[1].Sections, nextSheet.Edges[1].Sections) {
				t.Fatalf("re-layout changed the port, cross-container route or self-loop\nbefore %s\nafter %s", input, output)
			}
		})
	}
}

// A route synthesized only after layout would leave outer in a disconnected
// component. Mr. Tree must see all three ranks while positioning the boxes.
func TestCrossContainerEdgeInfluencesRanks(t *testing.T) {
	g := hierarchyFixture("mrtree", hierarchyEdge("first", "top", "external"), hierarchyEdge("second", "external", "target"))
	g.LayoutOptions["elk.direction"] = "DOWN"
	top := &elk.Node{ID: elk.StringID("top"), Width: elk.Float64(60), Height: elk.Float64(60)}
	g.Children = append(g.Children, top)
	if err := elk.Layout(g); err != nil {
		t.Fatal(err)
	}
	outer, external := nodeBounds(g.Children[0]), nodeBounds(g.Children[1])
	if external.Y <= nodeBounds(top).MaxY() || outer.Y <= external.MaxY() {
		t.Fatalf("expected ranks top -> external -> outer, got top=%v external=%v outer=%v", nodeBounds(top), external, outer)
	}
	hierarchySection(t, g.Edges[1])
}

func TestCrossContainerParallelLabels(t *testing.T) {
	g := hierarchyFixture("mrtree", hierarchyEdge("e1", "external", "target"), hierarchyEdge("e2", "external", "target"))
	for i, value := range []string{"read", "write"} {
		g.Edges[i].Labels = []*elk.Label{{Text: elk.String(value), Width: elk.Float64(40), Height: elk.Float64(12)}}
	}
	if err := elk.Layout(g); err != nil {
		t.Fatal(err)
	}
	first, second := hierarchySection(t, g.Edges[0]), hierarchySection(t, g.Edges[1])
	if *first.StartPoint == *second.StartPoint && *first.EndPoint == *second.EndPoint && reflect.DeepEqual(first.BendPoints, second.BendPoints) {
		t.Fatal("parallel connections have identical paths")
	}
	bounds := func(label *elk.Label) geometry.Rectangle {
		return geometry.Rectangle{X: elk.Number(label.X), Y: elk.Number(label.Y), Width: elk.Number(label.Width), Height: elk.Number(label.Height)}
	}
	if bounds(g.Edges[0].Labels[0]).Intersects(bounds(g.Edges[1].Labels[0])) {
		t.Fatal("parallel edge labels overlap")
	}
}

// Two outside nodes connect to different descendants of the same container.
// Radial must place both outside the container even though the projected
// connections both point into the same subtree.
func TestCrossContainerConvergingRadialEdges(t *testing.T) {
	g := hierarchyFixture("radial", hierarchyEdge("one", "external", "target"), hierarchyEdge("two", "other", "second"))
	outer := g.Children[0]
	inner := outer.Children[0]
	second := &elk.Node{ID: elk.StringID("second"), Width: elk.Float64(55), Height: elk.Float64(40)}
	inner.Children = append(inner.Children, second)
	g.Children = append(g.Children, &elk.Node{ID: elk.StringID("other"), Width: elk.Float64(60), Height: elk.Float64(40)})
	if err := elk.Layout(g); err != nil {
		t.Fatal(err)
	}
	for i, edge := range g.Edges {
		external := g.Children[i+1]
		for _, sibling := range g.Children[:i+1] {
			if nodeBounds(external).Intersects(nodeBounds(sibling)) {
				t.Fatalf("%s overlaps %s", external.ID, sibling.ID)
			}
		}
		s := hierarchySection(t, edge)
		checkOnBorder(t, *s.StartPoint, nodeBounds(external))
		checkOnBorder(t, *s.EndPoint, nodeBounds(inner.Children[i]))
	}
}

// An unrelated first child is a separate Radial tree. Its input order must
// neither select it as the only tree nor leave other components at its origin.
func TestCrossContainerRadialDisconnectedFirstChild(t *testing.T) {
	g := hierarchyFixture("radial", hierarchyEdge("link", "external", "target"))
	unrelated := &elk.Node{ID: elk.StringID("unrelated"), Width: elk.Float64(65), Height: elk.Float64(50)}
	g.Children = append([]*elk.Node{unrelated}, g.Children...)
	if err := elk.Layout(g); err != nil {
		t.Fatal(err)
	}
	for i, node := range g.Children {
		for _, sibling := range g.Children[:i] {
			if nodeBounds(node).Intersects(nodeBounds(sibling)) {
				t.Fatalf("%s overlaps %s", node.ID, sibling.ID)
			}
		}
	}
	s := hierarchySection(t, g.Edges[0])
	checkOnBorder(t, *s.StartPoint, nodeBounds(g.Children[2]))
	checkOnBorder(t, *s.EndPoint, nodeBounds(g.Children[1].Children[0].Children[0]))
}
