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
	"sort"

	"github.com/d2lang/elk-go/internal/geometry"
)

// triangulate is BowyerWatsonTriangulation.triangulate. Slices model the
// source's deterministically iterated GWT sets while explicit equality keeps
// their undirected-edge and unordered-triangle semantics.
func triangulate(vertices []*vertex) []*tEdge {
	topLeft := geometry.Point{X: math.Inf(1), Y: math.Inf(1)}
	bottomRight := geometry.Point{X: math.Inf(-1), Y: math.Inf(-1)}
	for _, vertex := range vertices {
		topLeft.X = math.Min(topLeft.X, vertex.point.X)
		topLeft.Y = math.Min(topLeft.Y, vertex.point.Y)
		bottomRight.X = math.Max(bottomRight.X, vertex.point.X)
		bottomRight.Y = math.Max(bottomRight.Y, vertex.point.Y)
	}
	size := geometry.Point{X: bottomRight.X - topLeft.X, Y: bottomRight.Y - topLeft.Y}
	sa := &vertex{point: geometry.Point{X: topLeft.X - superWiggleRoom, Y: topLeft.Y - size.X - superWiggleRoom}}
	sb := &vertex{point: geometry.Point{X: topLeft.X - superWiggleRoom, Y: bottomRight.Y + size.X + superWiggleRoom}}
	sc := &vertex{point: geometry.Point{X: bottomRight.X + size.Y/2 + superWiggleRoom, Y: topLeft.Y + size.Y/2}}
	superTriangle := newTriangle(sa, sb, sc)
	triangles := []*triangle{superTriangle}

	for _, added := range vertices {
		invalid := make([]*triangle, 0)
		for _, candidate := range triangles {
			if candidate.inCircumcircle(added) {
				invalid = append(invalid, candidate)
			}
		}

		boundary := make([]*tEdge, 0)
		for _, candidate := range invalid {
			for _, edge := range candidate.edges {
				onBoundary := true
				for _, other := range invalid {
					if other != candidate && other.containsEdge(edge) {
						onBoundary = false
					}
				}
				if onBoundary {
					boundary = append(boundary, edge)
				}
			}
		}

		remaining := triangles[:0]
		for _, candidate := range triangles {
			if !containsTrianglePointer(invalid, candidate) {
				remaining = append(remaining, candidate)
			}
		}
		triangles = remaining
		for _, edge := range boundary {
			candidate := newTriangle(added, edge.u, edge.v)
			if !containsEqualTriangle(triangles, candidate) {
				triangles = append(triangles, candidate)
			}
		}
	}

	edges := make([]*tEdge, 0)
	for _, candidate := range triangles {
		for _, edge := range candidate.edges {
			edges = addEdge(edges, edge)
		}
	}
	result := edges[:0]
	for _, edge := range edges {
		if !superTriangle.containsVertex(edge.u) && !superTriangle.containsVertex(edge.v) {
			result = append(result, edge)
		}
	}
	return result
}

func containsTrianglePointer(triangles []*triangle, candidate *triangle) bool {
	for _, present := range triangles {
		if present == candidate {
			return true
		}
	}
	return false
}

func containsEqualTriangle(triangles []*triangle, candidate *triangle) bool {
	for _, present := range triangles {
		if present.containsVertex(candidate.a) && present.containsVertex(candidate.b) && present.containsVertex(candidate.c) {
			return true
		}
	}
	return false
}

func createSpanningTreeForNodes(edges []*tEdge, root *vertex, cost func(*tEdge) float64, nodes []*node) *tree {
	weights := make(map[*tEdge]float64, len(edges))
	for _, edge := range edges {
		weights[edge] = cost(edge)
	}
	edgeList := append([]*tEdge(nil), edges...)
	sort.SliceStable(edgeList, func(i, j int) bool {
		return javaDoubleCompare(weights[edgeList[i]], weights[edgeList[j]]) < 0
	})
	minimum := &tree{node: nodeForVertex(root, nodes)}
	treeNodes := map[*vertex]*tree{root: minimum}
	for len(edgeList) > 0 {
		var nextEdge *tEdge
		var nextNode, nodeInTree *vertex
		minimumWeight := math.Inf(1)
		for _, edge := range edgeList {
			if weights[edge] <= minimumWeight {
				_, uPresent := treeNodes[edge.u]
				_, vPresent := treeNodes[edge.v]
				if uPresent && !vPresent {
					nextNode, nodeInTree, nextEdge = edge.v, edge.u, edge
					break
				}
				if vPresent && !uPresent {
					nextNode, nodeInTree, nextEdge = edge.u, edge.v, edge
					break
				}
			}
		}
		if nextEdge == nil {
			break
		}
		subtree := &tree{node: nodeForVertex(nextNode, nodes)}
		treeNodes[nodeInTree].children = append(treeNodes[nodeInTree].children, subtree)
		treeNodes[nextNode] = subtree
		edgeList = removeEdge(edgeList, nextEdge)
	}
	return minimum
}

func nodeForVertex(wanted *vertex, nodes []*node) *node {
	for _, candidate := range nodes {
		if candidate.original == wanted {
			return candidate
		}
	}
	return nil
}

func javaDoubleCompare(first, second float64) int {
	if first < second {
		return -1
	}
	if first > second {
		return 1
	}
	firstBits, secondBits := math.Float64bits(first), math.Float64bits(second)
	if math.IsNaN(first) {
		if math.IsNaN(second) {
			return 0
		}
		return 1
	}
	if math.IsNaN(second) {
		return -1
	}
	if firstBits == secondBits {
		return 0
	}
	if firstBits>>63 != 0 {
		return -1
	}
	return 1
}
