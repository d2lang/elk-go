// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"testing"

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

func TestJSONShapeCoordinatesRoot(t *testing.T) {
	root := &graphjson.Graph{
		ID:            graphjson.StringID("root"),
		X:             graphjson.Float64(7),
		Y:             graphjson.Float64(11),
		LayoutOptions: graphjson.LayoutOptions{JSONShapeCoords.ID: "ROOT"},
		Children: []*graphjson.Node{{
			ID: graphjson.StringID("p"), X: graphjson.Float64(100), Y: graphjson.Float64(200),
			Labels: []*graphjson.Label{{X: graphjson.Float64(14), Y: graphjson.Float64(25)}},
			Ports: []*graphjson.Port{{
				ID: graphjson.StringID("port"), X: graphjson.Float64(12), Y: graphjson.Float64(23),
				Labels: []*graphjson.Label{{X: graphjson.Float64(1), Y: graphjson.Float64(1)}},
			}},
			Children: []*graphjson.Node{{
				ID: graphjson.StringID("c"), X: graphjson.Float64(10), Y: graphjson.Float64(20),
			}},
		}},
	}

	codec := NewCodec(NewOptionRegistry())
	model, err := codec.Import(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := codec.TransferLayout(model); err != nil {
		t.Fatal(err)
	}

	assertJSONPosition(t, "root", root.X, root.Y, 7, 11)
	parent := root.Children[0]
	assertJSONPosition(t, "parent", parent.X, parent.Y, 107, 211)
	assertJSONPosition(t, "child", parent.Children[0].X, parent.Children[0].Y, 117, 231)
	assertJSONPosition(t, "port", parent.Ports[0].X, parent.Ports[0].Y, 119, 234)
	assertJSONPosition(t, "node label", parent.Labels[0].X, parent.Labels[0].Y, 121, 236)
	assertJSONPosition(t, "port label", parent.Ports[0].Labels[0].X, parent.Ports[0].Labels[0].Y, 120, 235)
}

func TestJSONShapeCoordinatesParentAndInherit(t *testing.T) {
	for _, mode := range []string{"PARENT", "INHERIT"} {
		t.Run(mode, func(t *testing.T) {
			root := &graphjson.Graph{
				ID:            graphjson.StringID("root"),
				X:             graphjson.Float64(7),
				Y:             graphjson.Float64(11),
				LayoutOptions: graphjson.LayoutOptions{JSONShapeCoords.ID: mode},
				Children: []*graphjson.Node{{
					ID: graphjson.StringID("child"), X: graphjson.Float64(100), Y: graphjson.Float64(200),
				}},
			}
			codec := NewCodec(NewOptionRegistry())
			model, err := codec.Import(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := codec.TransferLayout(model); err != nil {
				t.Fatal(err)
			}
			assertJSONPosition(t, "child", root.Children[0].X, root.Children[0].Y, 100, 200)
		})
	}
}

func TestJSONEdgeCoordinateModes(t *testing.T) {
	tests := []struct {
		mode          string
		offsetX       float64
		offsetY       float64
		wantContainer bool
	}{
		{mode: "CONTAINER", wantContainer: true},
		{mode: "PARENT", offsetX: 100, offsetY: 200},
		{mode: "ROOT", offsetX: 107, offsetY: 211},
	}
	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			section := &graphjson.EdgeSection{
				ID:         graphjson.StringID("s"),
				StartPoint: &graphjson.Point{X: 1, Y: 2},
				EndPoint:   &graphjson.Point{X: 5, Y: 6},
				BendPoints: []graphjson.Point{{X: 3, Y: 4}},
			}
			edge := &graphjson.Edge{
				ID:       graphjson.StringID("e"),
				Sources:  []graphjson.ID{graphjson.StringID("a")},
				Targets:  []graphjson.ID{graphjson.StringID("b")},
				Sections: []*graphjson.EdgeSection{section},
				Labels:   []*graphjson.Label{{X: graphjson.Float64(7), Y: graphjson.Float64(8)}},
			}
			root := &graphjson.Graph{
				ID:            graphjson.StringID("root"),
				X:             graphjson.Float64(7),
				Y:             graphjson.Float64(11),
				LayoutOptions: graphjson.LayoutOptions{JSONEdgeCoords.ID: test.mode},
				Children: []*graphjson.Node{{
					ID: graphjson.StringID("p"), X: graphjson.Float64(100), Y: graphjson.Float64(200),
					Children: []*graphjson.Node{
						{ID: graphjson.StringID("a")},
						{ID: graphjson.StringID("b")},
					},
				}},
				Edges: []*graphjson.Edge{edge},
			}

			codec := NewCodec(NewOptionRegistry())
			model, err := codec.Import(root)
			if err != nil {
				t.Fatal(err)
			}
			graph.Set(&model.Edge(0).Properties, JunctionPoints, geometry.Chain{{X: 9, Y: 10}})
			if err := codec.TransferLayout(model); err != nil {
				t.Fatal(err)
			}

			assertJSONPoint(t, "section start", *section.StartPoint, 1+test.offsetX, 2+test.offsetY)
			assertJSONPoint(t, "section bend", section.BendPoints[0], 3+test.offsetX, 4+test.offsetY)
			assertJSONPoint(t, "section end", *section.EndPoint, 5+test.offsetX, 6+test.offsetY)
			assertJSONPosition(t, "edge label", edge.Labels[0].X, edge.Labels[0].Y, 7+test.offsetX, 8+test.offsetY)
			assertJSONPoint(t, "junction", edge.JunctionPoints[0], 9+test.offsetX, 10+test.offsetY)
			if test.wantContainer {
				if edge.Container == nil || edge.Container.String() != "p" {
					t.Fatalf("container = %v, want p", edge.Container)
				}
			} else if edge.Container != nil {
				t.Fatalf("container = %v, want absent", edge.Container)
			}
		})
	}
}

func TestPrimitiveEdgeBendPointsImportedOnce(t *testing.T) {
	root := &graphjson.Graph{
		ID: graphjson.StringID("root"),
		Children: []*graphjson.Node{
			{ID: graphjson.StringID("a")},
			{ID: graphjson.StringID("b")},
		},
		Edges: []*graphjson.Edge{{
			ID:         graphjson.StringID("e"),
			Source:     graphjson.IDPtr(graphjson.StringID("a")),
			Target:     graphjson.IDPtr(graphjson.StringID("b")),
			BendPoints: []graphjson.Point{{X: 1, Y: 2}, {X: 3, Y: 4}},
		}},
	}
	model, err := NewCodec(NewOptionRegistry()).Import(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Sections) != 1 || len(model.Sections[0].BendPoints) != 2 {
		t.Fatalf("primitive bend points = %#v", model.Sections)
	}
}

func assertJSONPosition(t *testing.T, name string, x, y *float64, wantX, wantY float64) {
	t.Helper()
	if x == nil || y == nil || *x != wantX || *y != wantY {
		t.Fatalf("%s = (%v, %v), want (%v, %v)", name, x, y, wantX, wantY)
	}
}

func assertJSONPoint(t *testing.T, name string, point graphjson.Point, wantX, wantY float64) {
	t.Helper()
	if point.X != wantX || point.Y != wantY {
		t.Fatalf("%s = (%v, %v), want (%v, %v)", name, point.X, point.Y, wantX, wantY)
	}
}
