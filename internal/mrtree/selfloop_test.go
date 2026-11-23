// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package mrtree_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	elk "github.com/d2lang/elk-go"
	"github.com/d2lang/elk-go/internal/geometry"
)

func TestMrTreePlacesNodesAndComponentsAroundLoops(t *testing.T) {
	for _, direction := range []string{"DOWN", "UP", "LEFT", "RIGHT"} {
		for _, compact := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/compact=%t", direction, compact), func(t *testing.T) {
				g := &elk.Graph{ID: elk.StringID("root"), LayoutOptions: elk.LayoutOptions{
					"elk.algorithm": "mrtree", "elkgo.routeSelfLoops": true, "elk.direction": direction,
					"elk.spacing.nodeSelfLoop": 50, "elk.mrtree.compaction": compact,
				}}
				for _, id := range []string{"rootNode", "branch", "sibling", "leaf", "standalone"} {
					g.Children = append(g.Children, &elk.Node{ID: elk.StringID(id), Width: elk.Float64(53), Height: elk.Float64(66)})
				}
				connect := func(source, target string) *elk.Edge {
					e := &elk.Edge{ID: elk.IntID(int64(len(g.Edges))), Sources: []elk.ID{elk.StringID(source)}, Targets: []elk.ID{elk.StringID(target)}}
					g.Edges = append(g.Edges, e)
					return e
				}
				connect("rootNode", "branch")
				connect("rootNode", "sibling")
				connect("branch", "leaf")
				connect("rootNode", "leaf")
				for _, owner := range []string{"branch", "branch", "standalone"} {
					e := connect(owner, owner)
					e.Labels = []*elk.Label{{Text: elk.String("retry this operation"), Width: elk.Float64(130), Height: elk.Float64(18)}}
				}
				if err := elk.Layout(g); err != nil {
					t.Fatal(err)
				}
				boxes := make(map[elk.ID]geometry.Rectangle)
				root := geometry.Rectangle{Width: elk.Number(g.Width), Height: elk.Number(g.Height)}
				for _, node := range g.Children {
					box := geometry.Rectangle{X: elk.Number(node.X), Y: elk.Number(node.Y), Width: elk.Number(node.Width), Height: elk.Number(node.Height)}
					if box.Width != 53 || box.Height != 66 || root.Union(box) != root {
						t.Fatalf("node %s has invalid bounds %+v in root %+v", node.ID, box, root)
					}
					boxes[node.ID] = box
				}
				for _, e := range g.Edges {
					if len(e.Sections) != 1 {
						t.Fatalf("edge %s to %s has %d sections", e.Sources[0], e.Targets[0], len(e.Sections))
					}
					s := e.Sections[0]
					attached := func(p *elk.Point, id elk.ID) bool {
						box := boxes[id]
						return p != nil && p.X >= box.X && p.X <= box.MaxX() && p.Y >= box.Y && p.Y <= box.MaxY() &&
							(p.X == box.X || p.X == box.MaxX() || p.Y == box.Y || p.Y == box.MaxY())
					}
					if !attached(s.StartPoint, e.Sources[0]) || !attached(s.EndPoint, e.Targets[0]) {
						t.Fatalf("edge %s to %s detached from visible node: %v to %v", e.Sources[0], e.Targets[0], s.StartPoint, s.EndPoint)
					}
					if e.Sources[0] != e.Targets[0] {
						continue
					}
					if len(s.BendPoints) < 2 {
						t.Fatalf("self-loop missing detour: %+v", s)
					}
					l := e.Labels[0]
					labelBox := geometry.Rectangle{X: elk.Number(l.X), Y: elk.Number(l.Y), Width: *l.Width, Height: *l.Height}
					if root.Union(labelBox) != root {
						t.Fatalf("loop label %+v escapes graph %+v", labelBox, root)
					}
					for id, box := range boxes {
						if labelBox.Intersects(box) {
							t.Fatalf("loop label overlaps node %s: %+v intersects %+v", id, labelBox, box)
						}
					}
					points := append([]elk.Point{*s.StartPoint}, s.BendPoints...)
					points = append(points, *s.EndPoint)
					for i := 1; i < len(points); i++ {
						a, b := points[i-1], points[i]
						segment := geometry.Rectangle{X: math.Min(a.X, b.X), Y: math.Min(a.Y, b.Y), Width: math.Abs(b.X - a.X), Height: math.Abs(b.Y - a.Y)}
						if root.Union(segment) != root {
							t.Fatalf("loop segment %v to %v escapes root %+v", a, b, root)
						}
						for id, box := range boxes {
							if segment.Intersects(box) {
								t.Fatalf("loop on %s crosses %s: %v to %v intersects %+v", e.Sources[0], id, a, b, box)
							}
						}
					}
				}
			})
		}
	}
}

func TestMrTreeNestedSelfLoopCoordinates(t *testing.T) {
	node := func(id string) *elk.Node {
		return &elk.Node{ID: elk.StringID(id), Width: elk.Float64(60), Height: elk.Float64(40)}
	}
	loop := func(id string) *elk.Edge {
		return &elk.Edge{ID: elk.StringID(id + "-loop"), Sources: []elk.ID{elk.StringID(id)}, Targets: []elk.ID{elk.StringID(id)}}
	}
	child := node("child")
	outer := &elk.Node{ID: elk.StringID("outer"), LayoutOptions: elk.LayoutOptions{"elk.algorithm": "mrtree"},
		Children: []*elk.Node{child, node("innerNeighbor")}, Edges: []*elk.Edge{loop("child")}}
	root := &elk.Graph{ID: elk.StringID("root"), LayoutOptions: elk.LayoutOptions{"elk.algorithm": "mrtree", "elkgo.routeSelfLoops": true},
		Children: []*elk.Node{outer, node("outerNeighbor")}, Edges: []*elk.Edge{loop("outer")}}
	if err := elk.Layout(root); err != nil {
		t.Fatal(err)
	}
	for _, parent := range []*elk.Node{root, outer} {
		owner := parent.Children[0]
		if len(parent.Edges[0].Sections) != 1 {
			t.Fatalf("loop on %s missing under %s", owner.ID, parent.ID)
		}
		section := parent.Edges[0].Sections[0]
		left, top, right, bottom := *owner.X, *owner.Y, *owner.X+*owner.Width, *owner.Y+*owner.Height
		for _, end := range []*elk.Point{section.StartPoint, section.EndPoint} {
			if end == nil || end.X < left || end.X > right || end.Y < top || end.Y > bottom ||
				(end.X != left && end.X != right && end.Y != top && end.Y != bottom) {
				t.Fatalf("loop on %s endpoint %v outside its border in %s coordinates (box %v,%v..%v,%v)", owner.ID, end, parent.ID, left, top, right, bottom)
			}
		}
		if len(section.BendPoints) < 2 {
			t.Fatalf("loop on %s has no exterior detour", owner.ID)
		}
		for _, bend := range section.BendPoints {
			if bend.X < 0 || bend.X > *parent.Width || bend.Y < 0 || bend.Y > *parent.Height {
				t.Fatalf("loop on %s escapes parent %s: %v", owner.ID, parent.ID, bend)
			}
		}
	}
}

func TestMrTreeLoopMustFitFixedParent(t *testing.T) {
	input := []byte(`{"id":"limited","width":80,"height":70,"layoutOptions":{"elk.algorithm":"mrtree","elkgo.routeSelfLoops":true,"elk.nodeSize.fixedGraphSize":true},"children":[{"id":"a","width":45,"height":30}],"edges":[{"id":"e","sources":["a"],"targets":["a"],"labels":[{"text":"retry","width":130,"height":18}]}]}`)
	if _, err := elk.LayoutJSON(input); err == nil || !strings.Contains(err.Error(), "exceeding fixed parent \"limited\"") {
		t.Fatalf("expected fixed-parent overflow with graph ID; got %v", err)
	}
}
