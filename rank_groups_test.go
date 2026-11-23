// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package elk

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
)

func rankNode(id string, w, h float64) *Node {
	return &Node{ID: StringID(id), Width: Float64(w), Height: Float64(h)}
}
func rankEdge(a, b string) *Edge {
	return &Edge{ID: StringID(a + "-" + b), Sources: []ID{StringID(a)}, Targets: []ID{StringID(b)}}
}
func rankRect(n *Node) geometry.Rectangle {
	return geometry.Rectangle{X: Number(n.X), Y: Number(n.Y), Width: Number(n.Width), Height: Number(n.Height)}
}

func TestSameRankAcrossParents(t *testing.T) {
	for _, dir := range []string{"DOWN", "UP", "RIGHT", "LEFT"} {
		t.Run(dir, func(t *testing.T) {
			box := rankNode("merchant", 0, 0)
			checkout, commerce := rankNode("checkout", 76, 43), rankNode("commerce", 113, 65)
			box.Children = []*Node{checkout, commerce}
			channel, payments := rankNode("channel", 23, 18), rankNode("payments", 104, 41)
			g := Graph{ID: StringID("root"), LayoutOptions: LayoutOptions{"elk.algorithm": "layered", "elk.hierarchyHandling": "INCLUDE_CHILDREN", "elk.direction": dir, "d2.rankGroups": [][]string{{"commerce", "channel"}, {"channel", "payments"}}}, Children: []*Node{box, channel, payments}, Edges: []*Edge{rankEdge("checkout", "commerce"), rankEdge("commerce", "channel"), rankEdge("channel", "payments")}}
			if err := Layout(&g); err != nil {
				t.Fatal(err)
			}
			merchant := rankRect(box)
			inside := rankRect(commerce).Move(merchant.Position())
			if merchant.Union(inside) != merchant {
				t.Fatalf("commerce outside merchant: %v, %v", inside, merchant)
			}
			axis := func(r geometry.Rectangle) float64 {
				if dir == "LEFT" || dir == "RIGHT" {
					return r.Center().X
				}
				return r.Center().Y
			}
			for _, r := range []geometry.Rectangle{rankRect(channel), rankRect(payments)} {
				if math.Abs(axis(inside)-axis(r)) > 1e-7 {
					t.Fatalf("rank centers %v, %v", inside, r)
				}
			}
			from := rankRect(checkout).Move(merchant.Position())
			upstream := axis(from)
			if (upstream < axis(inside)) != (dir == "RIGHT" || dir == "DOWN") {
				t.Fatalf("checkout on wrong side: %v vs %v", upstream, axis(inside))
			}
			// Ordinary ELK centers the unequal-sized checkout and commerce
			// across the flow. Ranking the payment handoff must retain that
			// straight incoming connection as well as the two flat edges.
			if dir == "DOWN" || dir == "UP" {
				if from.Center().X != inside.Center().X {
					t.Fatalf("checkout and commerce X centers differ: %v, %v", from, inside)
				}
			} else if from.Center().Y != inside.Center().Y {
				t.Fatalf("checkout and commerce Y centers differ: %v, %v", from, inside)
			}
			for _, e := range g.Edges {
				if len(e.Sections) != 1 || len(e.Sections[0].BendPoints) != 0 {
					t.Fatalf("%s: expected a straight connection: %+v", e.ID.String(), e.Sections)
				}
			}
		})
	}
}

func TestRankedBranchRetainsIncomingAlignmentAcrossBoundary(t *testing.T) {
	checkout, commerce := rankNode("checkout", 76, 43), rankNode("commerce", 113, 65)
	channel, payments := rankNode("channel", 23, 18), rankNode("payments", 104, 41)
	merchant := rankNode("merchant", 0, 0)
	merchant.Children = []*Node{commerce, channel}
	g := Graph{ID: StringID("root"),
		LayoutOptions: LayoutOptions{"elk.direction": "DOWN", "elk.hierarchyHandling": "INCLUDE_CHILDREN", "d2.rankGroups": [][]string{{"commerce", "channel", "payments"}}},
		Children:      []*Node{checkout, merchant, payments},
		Edges:         []*Edge{rankEdge("checkout", "commerce"), rankEdge("commerce", "channel"), rankEdge("channel", "payments")},
	}
	if err := Layout(&g); err != nil {
		t.Fatal(err)
	}
	// Moving channel beside commerce widens merchant. The incoming edge from
	// outside can still meet commerce on its original centerline.
	from, to := rankRect(checkout).Center(), rankRect(commerce).Move(rankRect(merchant).Position()).Center()
	if from.X != to.X || len(g.Edges[0].Sections) != 1 || len(g.Edges[0].Sections[0].BendPoints) != 0 {
		t.Fatalf("straight incoming edge lost: %v to %v, route %+v", from, to, g.Edges[0].Sections)
	}
}

func TestRankedParallelBranchesKeepStraightEdges(t *testing.T) {
	// Both branches narrow as they approach the join. Packing each layer
	// independently would introduce a 70-unit dogleg in the second lane.
	g := Graph{ID: StringID("root"), LayoutOptions: LayoutOptions{"elk.direction": "DOWN", "d2.rankGroups": [][]string{{"b", "d"}}},
		Children: []*Node{rankNode("start", 60, 30), rankNode("a", 110, 30), rankNode("c", 130, 30),
			rankNode("b", 40, 30), rankNode("d", 60, 30), rankNode("end", 60, 30)},
		Edges: []*Edge{rankEdge("start", "a"), rankEdge("start", "c"), rankEdge("a", "b"),
			rankEdge("c", "d"), rankEdge("b", "end"), rankEdge("d", "end")},
	}
	if err := Layout(&g); err != nil {
		t.Fatal(err)
	}
	for _, e := range g.Edges[2:4] {
		if len(e.Sections) != 1 || len(e.Sections[0].BendPoints) != 0 || e.Sections[0].StartPoint.X != e.Sections[0].EndPoint.X {
			t.Fatalf("expected a straight vertical edge for %s", e.ID.String())
		}
	}
}

func TestRankedPipelinesTypedAndJSON(t *testing.T) {
	for _, dir := range []string{"DOWN", "UP", "RIGHT", "LEFT"} {
		t.Run(dir, func(t *testing.T) {
			keys := []string{"elk.algorithm", "elk.direction", "d2.rankGroups"}
			values := []any{"layered", dir, `[["short-report","long-report"]]`}
			opts := PrepareOptions(keys, values)
			names := []string{"push", "lint", "build", "test", "short-report", "long-report"}
			r := NewRequest(len(names)+1, 0, 1, 5)
			defer r.Release()
			root := r.Root("root", opts)
			refs := map[string]NodeRef{}
			g := Graph{ID: StringID("root"), LayoutOptions: LayoutOptions{}}
			for i, k := range keys {
				g.LayoutOptions[k] = values[i]
			}
			for i, name := range names {
				w, h := float64(48+i*3), float64(30+2*i)
				refs[name] = r.Node(root, name, 0, 0, w, h, nil)
				g.Children = append(g.Children, rankNode(name, w, h))
			}
			var labelEdge EdgeRef
			for i, ends := range [][2]string{{"push", "lint"}, {"push", "build"}, {"lint", "short-report"}, {"build", "test"}, {"test", "long-report"}} {
				a, b := ends[0], ends[1]
				e := rankEdge(a, b)
				g.Edges = append(g.Edges, e)
				edge := r.Edge(root, e.ID.String(), []string{a}, []string{b}, nil)
				if i == 2 {
					labelEdge = edge
					r.EdgeLabel(edge, "complete", 0, 0, 54, 17, nil)
					e.Labels = []*Label{{Text: String("complete"), Width: Float64(54), Height: Float64(17)}}
				}
			}
			if err := r.Layout(); err != nil {
				t.Fatal(err)
			}
			if err := Layout(&g); err != nil {
				t.Fatal(err)
			}
			axis := func(name string) float64 {
				x, y, w, h := r.NodeLayout(refs[name])
				if dir == "RIGHT" || dir == "LEFT" {
					return x + w/2
				}
				return y + h/2
			}
			if math.Abs(axis("short-report")-axis("long-report")) > 1e-7 {
				t.Fatal("report centers differ")
			}
			sign := 1.
			if dir == "LEFT" || dir == "UP" {
				sign = -1
			}
			for _, e := range g.Edges {
				if (axis(e.Targets[0].String())-axis(e.Sources[0].String()))*sign <= 0 {
					t.Fatalf("edge %s goes backward", e.ID.String())
				}
			}
			l := g.Edges[2].Labels[0]
			x, y, w, h := r.EdgeLabelBounds(labelEdge, 0)
			if x != Number(l.X) || y != Number(l.Y) || w != Number(l.Width) || h != Number(l.Height) {
				t.Fatalf("typed label %v,%v %vx%v; JSON %+v", x, y, w, h, l)
			}
		})
	}
}

func TestRankedCyclesPortsAndLabels(t *testing.T) {
	for _, dir := range []string{"DOWN", "UP", "RIGHT", "LEFT"} {
		t.Run(dir, func(t *testing.T) {
			nodes := []*Node{rankNode("a", 110, 80), rankNode("b", 76, 72), rankNode("c", 73, 100), rankNode("d", 82, 78)}
			nodes[0].LayoutOptions = LayoutOptions{"elk.portConstraints": "FIXED_POS"}
			nodes[0].Ports = []*Port{{ID: StringID("pa"), X: Float64(0), Y: Float64(15), Width: Float64(0), Height: Float64(0), LayoutOptions: LayoutOptions{"elk.port.side": "WEST"}}}
			g := Graph{ID: StringID("root"), LayoutOptions: LayoutOptions{"elk.direction": dir, "d2.rankGroups": [][]string{{"a", "b", "c", "d"}}}, Children: nodes}
			for i, ends := range [][2]string{{"a", "b"}, {"a", "b"}, {"b", "a"}, {"c", "d"}, {"a", "d"}, {"d", "pa"}, {"a", "a"}} {
				e := rankEdge(ends[0], ends[1])
				e.ID = StringID(string(rune('A' + i)))
				e.Labels = []*Label{{Text: String("label"), Width: Float64(float64(48 + 9*i)), Height: Float64(16)}}
				g.Edges = append(g.Edges, e)
			}
			if err := Layout(&g); err != nil {
				t.Fatal(err)
			}
			var boxes []geometry.Rectangle
			for _, n := range nodes {
				boxes = append(boxes, rankRect(n))
			}
			for _, e := range g.Edges {
				l := e.Labels[0]
				r := geometry.Rectangle{X: Number(l.X), Y: Number(l.Y), Width: Number(l.Width), Height: Number(l.Height)}
				for _, box := range boxes {
					if box.Intersects(r) {
						t.Fatalf("%s label %v overlaps %v", e.ID.String(), r, box)
					}
				}
				boxes = append(boxes, r)
				if len(e.Sections) != 1 {
					t.Fatalf("%s missing route", e.ID.String())
				}
				s := e.Sections[0]
				pts := append([]Point{*s.StartPoint}, s.BendPoints...)
				pts = append(pts, *s.EndPoint)
				for j := 1; j < len(pts); j++ {
					a, b := pts[j-1], pts[j]
					if a.X != b.X && a.Y != b.Y {
						t.Fatalf("nonorthogonal segment %v--%v", a, b)
					}
					for i, n := range nodes {
						if e.Sources[0] == n.ID || e.Targets[0] == n.ID || e.Targets[0] == StringID("pa") && i == 0 {
							continue
						}
						r := rankRect(n)
						if a.X == b.X && min(a.Y, b.Y) < r.MaxY() && max(a.Y, b.Y) > r.Y && r.X < a.X && a.X < r.MaxX() ||
							a.Y == b.Y && min(a.X, b.X) < r.MaxX() && max(a.X, b.X) > r.X && r.Y < a.Y && a.Y < r.MaxY() {
							t.Fatalf("%s cuts through %s", e.ID.String(), n.ID.String())
						}
					}
				}
			}
			pa := nodes[0].Ports[0]
			q := g.Edges[5].Sections[0].EndPoint
			if q.X != Number(nodes[0].X)+Number(pa.X) || q.Y != Number(nodes[0].Y)+Number(pa.Y) {
				t.Fatalf("route misses fixed port: %+v; %+v", q, pa)
			}
		})
	}
}

func TestRankGroupValidationAndNoop(t *testing.T) {
	fixture := func() *Graph {
		return &Graph{ID: StringID("root"), Children: []*Node{rankNode("a", 40, 50), rankNode("b", 51, 43)}, Edges: []*Edge{rankEdge("a", "b")}, LayoutOptions: LayoutOptions{}}
	}
	base := fixture()
	if err := Layout(base); err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{[][]string{}, [][]string{{"a"}}, [][]string{{"b", "b"}}} {
		v := fixture()
		v.LayoutOptions["d2.rankGroups"] = value
		if err := Layout(v); err != nil {
			t.Fatal(err)
		}
		delete(v.LayoutOptions, "d2.rankGroups")
		a, _ := json.Marshal(v)
		b, _ := json.Marshal(base)
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("noop changed layout: %s vs %s", a, b)
		}
	}
	for _, tc := range []struct{ raw, want string }{{`[["a","missing"]]`, "unknown node"}, {`[["root","a"]]`, "leaf node"}, {`[1]`, "array of node ID arrays"}} {
		v := fixture()
		v.LayoutOptions["d2.rankGroups"] = tc.raw
		err := Layout(v)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("option %s: %v", tc.raw, err)
		}
	}
}

func TestRankedContainerBoundariesAndLoop(t *testing.T) {
	for _, dir := range []string{"DOWN", "UP", "RIGHT", "LEFT"} {
		t.Run(dir, func(t *testing.T) {
			g := Graph{ID: StringID("root"), LayoutOptions: LayoutOptions{"elk.algorithm": "layered", "elk.direction": dir, "elk.hierarchyHandling": "INCLUDE_CHILDREN", "d2.rankGroups": [][]string{{"z", "far"}}}}
			box := rankNode("box", 0, 0)
			box.LayoutOptions = LayoutOptions{"elk.padding": "[top=60,left=70,bottom=45,right=55]"}
			for _, s := range []string{"a", "b", "c", "z"} {
				box.Children = append(box.Children, rankNode(s, 38, 48))
			}
			out := rankNode("out", 38, 48)
			g.Children = []*Node{box, out}
			g.Edges = []*Edge{rankEdge("a", "b"), rankEdge("b", "c"), rankEdge("box", "out"), rankEdge("box", "box")}
			path := []string{"i1", "i2", "i3", "i4", "i5", "far"}
			for i, s := range path {
				g.Children = append(g.Children, rankNode(s, 38, 48))
				if i > 0 {
					g.Edges = append(g.Edges, rankEdge(path[i-1], s))
				}
			}
			if err := Layout(&g); err != nil {
				t.Fatal(err)
			}
			b, o := rankRect(box), rankRect(out)
			if dir == "DOWN" && b.MaxY() >= o.Y || dir == "UP" && o.MaxY() >= b.Y || dir == "RIGHT" && b.MaxX() >= o.X || dir == "LEFT" && o.MaxX() >= b.X {
				t.Fatalf("out %v does not follow all of box %v", o, b)
			}
			e := g.Edges[3]
			s := e.Sections[0]
			for _, q := range []*Point{s.StartPoint, s.EndPoint} {
				x, y := q.X, q.Y
				if e.Container != nil && e.Container.String() == "box" {
					x += b.X
					y += b.Y
				}
				if !(x == b.X || x == b.MaxX() || y == b.Y || y == b.MaxY()) || x < b.X || x > b.MaxX() || y < b.Y || y > b.MaxY() {
					t.Fatalf("loop endpoint %v,%v outside resized box %+v", x, y, b)
				}
			}
		})
	}
}

func TestSameRankHonorsLayerExtrema(t *testing.T) {
	for _, kind := range []string{"FIRST", "LAST", "FIRST_SEPARATE", "LAST_SEPARATE"} {
		t.Run(kind, func(t *testing.T) {
			g := Graph{ID: StringID("root"), LayoutOptions: LayoutOptions{"elk.direction": "DOWN", "d2.rankGroups": [][]string{{"x", "y"}}}}
			for _, s := range []string{"a", "b", "c", "x", "y"} {
				g.Children = append(g.Children, rankNode(s, 60, 40))
			}
			g.Children[1].LayoutOptions = LayoutOptions{"elk.layered.layering.layerConstraint": kind}
			g.Edges = []*Edge{rankEdge("a", "b"), rankEdge("b", "c"), rankEdge("x", "y")}
			if err := Layout(&g); err != nil {
				t.Fatal(err)
			}
			b := rankRect(g.Children[1]).Center().Y
			for _, i := range []int{0, 2} {
				v := rankRect(g.Children[i]).Center().Y
				if strings.HasPrefix(kind, "FIRST") && b >= v || strings.HasPrefix(kind, "LAST") && b <= v {
					t.Fatalf("extremum at %v, neighbor at %v", b, v)
				}
			}
		})
	}
	g := Graph{ID: StringID("root"), LayoutOptions: LayoutOptions{"d2.rankGroups": [][]string{{"a", "b"}}}, Children: []*Node{rankNode("a", 60, 40), rankNode("b", 60, 40)}}
	g.Children[0].LayoutOptions = LayoutOptions{"elk.layered.layering.layerConstraint": "FIRST"}
	g.Children[1].LayoutOptions = LayoutOptions{"elk.layered.layering.layerConstraint": "LAST"}
	if err := Layout(&g); err == nil || !strings.Contains(err.Error(), "conflicting layer constraints") {
		t.Fatalf("conflict: %v", err)
	}
}

func TestDistinctFlatEdgesReserveTheirTracks(t *testing.T) {
	g := Graph{ID: StringID("root"), LayoutOptions: LayoutOptions{"elk.direction": "DOWN", "d2.rankGroups": [][]string{{"a", "b", "c", "d", "e"}}}}
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		g.Children = append(g.Children, rankNode(name, 60, 60))
	}
	for i, a := range g.Children {
		for _, b := range g.Children[i+1:] {
			e := rankEdge(a.ID.String(), b.ID.String())
			e.Labels = []*Label{{Text: String("validation"), Width: Float64(170), Height: Float64(21)}}
			g.Edges = append(g.Edges, e)
		}
	}
	if err := Layout(&g); err != nil {
		t.Fatal(err)
	}
	assertDistinctTracks(t, g.Edges)
}

func TestRankedStateTracksAndLabels(t *testing.T) {
	for _, dir := range []string{"DOWN", "UP", "RIGHT", "LEFT"} {
		t.Run(dir, func(t *testing.T) {
			g := Graph{ID: StringID("root"), LayoutOptions: LayoutOptions{"elk.direction": dir, "d2.rankGroups": [][]string{{"playing", "paused"}}}}
			for _, id := range []string{"ready", "playing", "paused", "finished"} {
				g.Children = append(g.Children, rankNode(id, 96, 60))
			}
			for _, row := range [][3]string{{"ready", "playing", "play"}, {"playing", "paused", "pause"}, {"paused", "playing", "resume"}, {"playing", "finished", "finish"}} {
				e := rankEdge(row[0], row[1])
				e.Labels = []*Label{{Text: String(row[2]), Width: Float64(40), Height: Float64(20)}}
				g.Edges = append(g.Edges, e)
			}
			if err := Layout(&g); err != nil {
				t.Fatal(err)
			}
			assertDistinctTracks(t, g.Edges)
			for _, e := range g.Edges {
				l := e.Labels[0]
				r := geometry.Rectangle{X: Number(l.X), Y: Number(l.Y), Width: Number(l.Width), Height: Number(l.Height)}
				for _, other := range g.Edges {
					if other == e {
						continue
					}
					s := other.Sections[0]
					points := append([]Point{*s.StartPoint}, s.BendPoints...)
					points = append(points, *s.EndPoint)
					for i := 1; i < len(points); i++ {
						a, b := points[i-1], points[i]
						if a.X == b.X && r.X < a.X && a.X < r.MaxX() && max(a.Y, b.Y) > r.Y && min(a.Y, b.Y) < r.MaxY() ||
							a.Y == b.Y && r.Y < a.Y && a.Y < r.MaxY() && max(a.X, b.X) > r.X && min(a.X, b.X) < r.MaxX() {
							t.Fatalf("label on %s crosses %s", e.ID.String(), other.ID.String())
						}
					}
				}
			}
		})
	}
}

func assertDistinctTracks(t *testing.T, edges []*Edge) {
	t.Helper()
	var segments [][2]Point
	for _, e := range edges {
		s := e.Sections[0]
		points := append([]Point{*s.StartPoint}, s.BendPoints...)
		points = append(points, *s.EndPoint)
		var current [][2]Point
		for j := 1; j < len(points); j++ {
			a, b := points[j-1], points[j]
			for _, seg := range segments {
				c, d := seg[0], seg[1]
				if a.Y == b.Y && a.Y == c.Y && a.Y == d.Y && min(max(a.X, b.X), max(c.X, d.X)) > max(min(a.X, b.X), min(c.X, d.X)) ||
					a.X == b.X && a.X == c.X && a.X == d.X && min(max(a.Y, b.Y), max(c.Y, d.Y)) > max(min(a.Y, b.Y), min(c.Y, d.Y)) {
					t.Fatalf("%s reuses a routed segment: %v--%v vs %v", e.ID.String(), a, b, seg)
				}
			}
			current = append(current, [2]Point{a, b})
		}
		segments = append(segments, current...)
	}
}

func TestSameRankReciprocalPeerKeepsThroughConnectionsStraight(t *testing.T) {
	for _, dir := range []string{"DOWN", "UP", "RIGHT", "LEFT"} {
		t.Run(dir, func(t *testing.T) {
			ready, playing := rankNode("ready", 89, 66), rankNode("playing", 98, 66)
			paused, finished := rankNode("paused", 96, 66), rankNode("finished", 104, 66)
			g := Graph{ID: StringID("root"), LayoutOptions: LayoutOptions{
				"elk.direction": dir, "d2.rankGroups": [][]string{{"playing", "paused"}},
			}, Children: []*Node{ready, playing, paused, finished},
				Edges: []*Edge{rankEdge("ready", "playing"), rankEdge("playing", "paused"), rankEdge("paused", "playing"), rankEdge("playing", "finished")}}
			for i, text := range []string{"play", "pause", "resume", "finish"} {
				g.Edges[i].Labels = []*Label{{Text: String(text), Width: Float64(float64(len(text)) * 8), Height: Float64(21)}}
			}
			if err := Layout(&g); err != nil {
				t.Fatal(err)
			}
			center := func(n *Node) float64 {
				r := rankRect(n)
				if dir == "UP" || dir == "DOWN" {
					return r.Center().X
				}
				return r.Center().Y
			}
			if center(ready) != center(playing) || center(playing) != center(finished) {
				t.Fatalf("through lane centers: %v, %v, %v", center(ready), center(playing), center(finished))
			}
			for _, e := range []*Edge{g.Edges[0], g.Edges[3]} {
				if len(e.Sections) != 1 || len(e.Sections[0].BendPoints) != 0 {
					t.Fatalf("%s should be straight: %+v", e.ID.String(), e.Sections)
				}
			}
			for _, e := range g.Edges[1:3] {
				if len(e.Sections) != 1 {
					t.Fatalf("missing transition: %s", e.ID.String())
				}
			}
		})
	}
}
