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

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
)

const (
	fuzziness       = 0.0001
	doubleEpsilon   = 0.00001
	superWiggleRoom = 50.0
)

type vertex struct {
	point geometry.Point
}

type node struct {
	original *vertex
	vertex   geometry.Point
	rect     geometry.Rectangle
	origin   basegraph.NodeID
}

func newNode(v *vertex, rect geometry.Rectangle, origin basegraph.NodeID) *node {
	return &node{original: v, vertex: v.point, rect: rect, origin: origin}
}

func (n *node) translate(offset geometry.Point) {
	n.vertex = n.vertex.Add(offset)
	n.rect = n.rect.Move(offset)
}

func (n *node) setCenterPosition(position geometry.Point) {
	n.translate(position.Sub(n.vertex))
}

func (n *node) underlap(other *node) float64 {
	horizontalCenterDistance := math.Abs(n.rect.Center().X - other.rect.Center().X)
	verticalCenterDistance := math.Abs(n.rect.Center().Y - other.rect.Center().Y)
	horizontalUnderlap, verticalUnderlap := 0.0, 0.0
	hScale, vScale := 1.0, 1.0
	if horizontalCenterDistance > n.rect.Width/2+other.rect.Width/2 {
		horizontalUnderlap = math.Min(
			math.Abs(n.rect.X-(other.rect.X+other.rect.Width)),
			math.Abs(n.rect.X+n.rect.Width-other.rect.X),
		)
		hScale = 1 - horizontalUnderlap/horizontalCenterDistance
	}
	if verticalCenterDistance > n.rect.Height/2+other.rect.Height/2 {
		verticalUnderlap = math.Min(
			math.Abs(n.rect.Y-(other.rect.Y+other.rect.Height)),
			math.Abs(n.rect.Y+n.rect.Height-other.rect.Y),
		)
		vScale = 1 - verticalUnderlap/verticalCenterDistance
	}
	scale := math.Min(hScale, vScale)
	return (1 - scale) * math.Hypot(horizontalCenterDistance, verticalCenterDistance)
}

func (n *node) distance(other *node, direction geometry.Point) float64 {
	result := math.Inf(1)
	for _, first := range rectangleEdges(n.rect) {
		for _, second := range rectangleEdges(other.rect) {
			result = math.Min(result, segmentDistance(first.u, first.v, second.u, second.v, direction))
		}
	}
	return result
}

func (n *node) touches(other *node) bool {
	return fuzzyCompare(n.rect.X, other.rect.X+other.rect.Width, fuzziness) <= 0 &&
		fuzzyCompare(other.rect.X, n.rect.X+n.rect.Width, fuzziness) <= 0 &&
		fuzzyCompare(n.rect.Y, other.rect.Y+other.rect.Height, fuzziness) <= 0 &&
		fuzzyCompare(other.rect.Y, n.rect.Y+n.rect.Height, fuzziness) <= 0
}

type tEdge struct {
	u *vertex
	v *vertex
}

func sameEdge(left, right *tEdge) bool {
	return left.u == right.u && left.v == right.v || left.u == right.v && left.v == right.u
}

type triangle struct {
	a, b, c *vertex
	edges   []*tEdge
	center  geometry.Point
}

func newTriangle(a, b, c *vertex) *triangle {
	t := &triangle{a: a, b: b, c: c}
	t.edges = []*tEdge{{u: a, v: b}, {u: b, v: c}, {u: c, v: a}}
	ab := b.point.Sub(a.point)
	ac := c.point.Sub(a.point)
	bc := c.point.Sub(b.point)
	e := ab.X*(a.point.X+b.point.X) + ab.Y*(a.point.Y+b.point.Y)
	f := ac.X*(a.point.X+c.point.X) + ac.Y*(a.point.Y+c.point.Y)
	g := 2 * (ab.X*bc.Y - ab.Y*bc.X)
	t.center = geometry.Point{
		X: (ac.Y*e - ab.Y*f) / g,
		Y: (ab.X*f - ac.X*e) / g,
	}
	return t
}

func (t *triangle) containsVertex(candidate *vertex) bool {
	return t.a == candidate || t.b == candidate || t.c == candidate
}

func (t *triangle) containsEdge(candidate *tEdge) bool {
	for _, edge := range t.edges {
		if sameEdge(edge, candidate) {
			return true
		}
	}
	return false
}

func (t *triangle) inCircumcircle(candidate *vertex) bool {
	return fuzzyCompare(t.center.Distance(candidate.point), t.center.Distance(t.a.point), fuzziness) < 0
}

type tree struct {
	node     *node
	children []*tree
}

type sporeGraph struct {
	vertices         []*node
	edges            []*tEdge
	tree             *tree
	preferredRoot    *node
	treeConstruction string
	compaction       string
	cost             func(*tEdge) float64
	orthogonal       bool
}

func fuzzyCompare(a, b, tolerance float64) int {
	if a == b || math.Abs(a-b) <= tolerance || math.IsNaN(a) && math.IsNaN(b) {
		return 0
	}
	if a < b {
		return -1
	}
	return 1
}

func addEdge(edges []*tEdge, edge *tEdge) []*tEdge {
	for _, present := range edges {
		if sameEdge(present, edge) {
			return edges
		}
	}
	return append(edges, edge)
}

func removeEdge(edges []*tEdge, edge *tEdge) []*tEdge {
	for index, candidate := range edges {
		if candidate == edge {
			return append(edges[:index], edges[index+1:]...)
		}
	}
	return edges
}
