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
)

func overlap(first, second geometry.Rectangle) float64 {
	horizontalOverlap := math.Min(
		math.Abs(first.X-(second.X+second.Width)),
		math.Abs(first.X+first.Width-second.X),
	)
	verticalOverlap := math.Min(
		math.Abs(first.Y-(second.Y+second.Height)),
		math.Abs(first.Y+first.Height-second.Y),
	)
	horizontalCenterDistance := math.Abs(first.Center().X - second.Center().X)
	if horizontalCenterDistance > first.Width/2+second.Width/2 {
		return 1
	}
	verticalCenterDistance := math.Abs(first.Center().Y - second.Center().Y)
	if verticalCenterDistance > first.Height/2+second.Height/2 {
		return 1
	}
	if horizontalCenterDistance == 0 && verticalCenterDistance == 0 {
		return 0
	}
	if horizontalCenterDistance == 0 {
		return verticalOverlap/verticalCenterDistance + 1
	}
	if verticalCenterDistance == 0 {
		return horizontalOverlap/horizontalCenterDistance + 1
	}
	return math.Min(horizontalOverlap/horizontalCenterDistance, verticalOverlap/verticalCenterDistance) + 1
}

func shortestDistance(first, second geometry.Rectangle) float64 {
	rightDistance := second.X - (first.X + first.Width)
	leftDistance := first.X - (second.X + second.Width)
	topDistance := first.Y - (second.Y + second.Height)
	bottomDistance := second.Y - (first.Y + first.Height)
	horizontalDistance := math.Max(leftDistance, rightDistance)
	verticalDistance := math.Max(topDistance, bottomDistance)
	if (fuzzyCompare(horizontalDistance, 0, doubleEpsilon) >= 0) !=
		(fuzzyCompare(verticalDistance, 0, doubleEpsilon) >= 0) {
		return math.Max(verticalDistance, horizontalDistance)
	}
	if fuzzyCompare(horizontalDistance, 0, doubleEpsilon) > 0 {
		return math.Hypot(verticalDistance, horizontalDistance)
	}
	return -math.Hypot(verticalDistance, horizontalDistance)
}

func clipVector(vector geometry.Point, width, height float64) geometry.Point {
	halfWidth, halfHeight := width/2, height/2
	xScale, yScale := 1.0, 1.0
	if math.Abs(vector.X) > halfWidth {
		xScale = halfWidth / math.Abs(vector.X)
	}
	if math.Abs(vector.Y) > halfHeight {
		yScale = halfHeight / math.Abs(vector.Y)
	}
	return vector.Scale(math.Min(xScale, yScale))
}

func rectangleEdges(rect geometry.Rectangle) []*tEdgePoint {
	topLeft := rect.Position()
	topRight := rect.TopRight()
	bottomLeft := rect.BottomLeft()
	bottomRight := rect.BottomRight()
	return []*tEdgePoint{
		{u: topLeft, v: topRight},
		{u: topLeft, v: bottomLeft},
		{u: bottomRight, v: topRight},
		{u: bottomRight, v: bottomLeft},
	}
}

type tEdgePoint struct {
	u, v geometry.Point
}

func segmentDistance(a1, a2, b1, b2, direction geometry.Point) float64 {
	return math.Min(
		traceRays(a1, a2, b1, b2, direction),
		traceRays(b1, b2, a1, a2, direction.Negate()),
	)
}

func traceRays(a1, a2, b1, b2, direction geometry.Point) float64 {
	result := math.Inf(1)
	edgeIntersection, intersects := segmentIntersection(
		a1,
		a2.Sub(a1),
		b1.Add(direction),
		b2.Sub(b1),
	)
	edgeCase := intersects && !fuzzyPointEqual(edgeIntersection, a1) && !fuzzyPointEqual(edgeIntersection, a2)

	intersection, intersects := segmentIntersection(a1, a2.Sub(a1), b1, direction)
	endpointHit := false
	if intersects {
		if fuzzyPointEqual(intersection, a1) == fuzzyPointEqual(intersection, a2) || edgeCase {
			result = math.Min(result, intersection.Sub(b1).Length())
		} else {
			endpointHit = true
		}
	}

	intersection, intersects = segmentIntersection(a1, a2.Sub(a1), b2, direction)
	if intersects && (endpointHit || fuzzyPointEqual(intersection, a1) == fuzzyPointEqual(intersection, a2) || edgeCase) {
		result = math.Min(result, intersection.Sub(b2).Length())
	}
	return result
}

func segmentIntersection(p, r, q, s geometry.Point) (geometry.Point, bool) {
	pq := q.Sub(p)
	pqCrossR := pq.Cross(r)
	rCrossS := r.Cross(s)
	t := pq.Cross(s) / rCrossS
	u := pqCrossR / rCrossS
	if rCrossS == 0 {
		if pqCrossR != 0 {
			return geometry.Point{}, false
		}
		center := q.Add(s.Scale(0.5))
		d1 := p.Distance(center)
		d2 := p.Add(r).Distance(center)
		length := s.Length() * 0.5
		if d1 < d2 && d1 <= length {
			return p, true
		}
		if d2 <= length {
			return p.Add(r), true
		}
		return geometry.Point{}, false
	}
	if t >= 0 && t <= 1 && u >= 0 && u <= 1 {
		return p.Add(r.Scale(t)), true
	}
	return geometry.Point{}, false
}

func fuzzyPointEqual(first, second geometry.Point) bool {
	return math.Abs(first.X-second.X) <= 0.05 && math.Abs(first.Y-second.Y) <= 0.05
}
