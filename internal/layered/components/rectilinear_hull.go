// Copyright (c) 2016 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package components

import (
	"sort"

	"github.com/d2lang/elk-go/internal/geometry"
)

// The declaration order is the one used by Point.Quadrant in ELK 0.8.1. It
// is observable in RIGHT_SPECIAL_ORDER below.
type hullQuadrant uint8

const (
	hullQ1 hullQuadrant = iota
	hullQ4
	hullQ2
	hullQ3
)

func (quadrant hullQuadrant) upper() bool {
	return quadrant == hullQ1 || quadrant == hullQ2
}

func (quadrant hullQuadrant) left() bool {
	return quadrant == hullQ1 || quadrant == hullQ4
}

func hullBothLeftOrRight(first, second hullQuadrant) bool {
	return first == hullQ1 && second == hullQ4 ||
		first == hullQ4 && second == hullQ1 ||
		first == hullQ3 && second == hullQ2 ||
		first == hullQ2 && second == hullQ3
}

func hullOneLeftOneRight(first, second hullQuadrant) bool {
	return first == hullQ1 && (second == hullQ2 || second == hullQ3) ||
		first == hullQ4 && (second == hullQ3 || second == hullQ2)
}

type hullPoint struct {
	x, y     float64
	quadrant hullQuadrant
	convex   bool
}

func newHullPoint(x, y float64) hullPoint {
	return hullPoint{x: x, y: y, convex: true}
}

func quadrantHullPoint(point hullPoint, quadrant hullQuadrant) hullPoint {
	point.quadrant = quadrant
	point.convex = true
	return point
}

type hullPointOrder func(first, second hullPoint) int

func stableHullSort(points []hullPoint, order hullPointOrder) []hullPoint {
	copyPoints := append([]hullPoint(nil), points...)
	sort.SliceStable(copyPoints, func(first, second int) bool {
		return order(copyPoints[first], copyPoints[second]) < 0
	})
	return copyPoints
}

func compareFloat(first, second float64) int {
	switch {
	case first < second:
		return -1
	case first > second:
		return 1
	default:
		return 0
	}
}

func rightHighFirst(first, second hullPoint) int {
	if first.x == second.x {
		return compareFloat(second.y, first.y)
	}
	return compareFloat(first.x, second.x)
}

func rightLowFirst(first, second hullPoint) int {
	if first.x == second.x {
		return compareFloat(first.y, second.y)
	}
	return compareFloat(first.x, second.x)
}

func leftHighFirst(first, second hullPoint) int {
	if first.x == second.x {
		return compareFloat(second.y, first.y)
	}
	return compareFloat(second.x, first.x)
}

func leftLowFirst(first, second hullPoint) int {
	if first.x == second.x {
		return compareFloat(first.y, second.y)
	}
	return compareFloat(second.x, first.x)
}

func maximalHullElements(
	points []hullPoint,
	quadrant hullQuadrant,
	order hullPointOrder,
) []hullPoint {
	ordered := stableHullSort(points, order)
	result := make([]hullPoint, 0, len(ordered))
	upper := quadrant == hullQ1 || quadrant == hullQ2
	var maximalY float64
	if upper {
		maximalY = positiveInfinity
	} else {
		maximalY = negativeInfinity
	}
	for _, point := range ordered {
		if upper && point.y < maximalY || !upper && point.y > maximalY {
			result = append(result, quadrantHullPoint(point, quadrant))
			maximalY = point.y
		}
	}
	return result
}

func addConcaveHullCorners(points []hullPoint, quadrant hullQuadrant) []hullPoint {
	if len(points) == 0 {
		return nil
	}
	result := make([]hullPoint, 0, len(points)*2-1)
	last := points[0]
	result = append(result, last)
	for _, next := range points[1:] {
		result = append(result, hullPoint{
			x: next.x, y: last.y, quadrant: quadrant, convex: false,
		})
		result = append(result, next)
		last = next
	}
	return result
}

func reverseHullPoints(points []hullPoint) []hullPoint {
	result := make([]hullPoint, len(points))
	for index := range points {
		result[len(points)-1-index] = points[index]
	}
	return result
}

// splitRectilinearHull is a source-shaped port of
// RectilinearConvexHull.of(points).splitIntoRectangles().
func splitRectilinearHull(points []hullPoint) []geometry.Rectangle {
	if len(points) == 0 {
		return nil
	}

	var xMin1, xMin2 *hullPoint
	for index := range points {
		point := &points[index]
		// Equality is intentional: the two extrema may be equal.
		if xMin1 == nil || point.x <= xMin1.x {
			xMin2 = xMin1
			xMin1 = point
		}
	}

	q1 := addConcaveHullCorners(
		maximalHullElements(points, hullQ1, rightLowFirst), hullQ1)
	q4 := addConcaveHullCorners(
		maximalHullElements(points, hullQ4, rightHighFirst), hullQ4)
	q2 := addConcaveHullCorners(
		maximalHullElements(points, hullQ2, leftLowFirst), hullQ2)
	q3 := addConcaveHullCorners(
		maximalHullElements(points, hullQ3, leftHighFirst), hullQ3)

	hull := make([]hullPoint, 0, len(q1)+len(q2)+len(q3)+len(q4))
	hull = append(hull, q1...)
	hull = append(hull, reverseHullPoints(q2)...)
	hull = append(hull, q3...)
	hull = append(hull, reverseHullPoints(q4)...)

	rightSpecialOrder := func(first, second hullPoint) int {
		if first.x != second.x {
			return compareFloat(first.x, second.x)
		}
		if first.quadrant == second.quadrant ||
			hullBothLeftOrRight(first.quadrant, second.quadrant) {
			value := -1
			if first.quadrant.left() {
				value = 1
			}
			if first.convex && !second.convex {
				return value
			}
			if !first.convex && second.convex {
				return -value
			}
		}
		return int(first.quadrant) - int(second.quadrant)
	}
	hull = stableHullSort(hull, rightSpecialOrder)

	lastX := xMin1.x
	if xMin2 != nil && xMin2.x < lastX {
		lastX = xMin2.x
	}
	var minY, maxY *hullPoint
	var queued *geometry.Rectangle
	var queuedPoint *hullPoint
	rectangles := make([]geometry.Rectangle, 0)
	for index := range hull {
		point := &hull[index]
		if queued != nil &&
			(point.x != queuedPoint.x ||
				hullOneLeftOneRight(queuedPoint.quadrant, point.quadrant)) {
			rectangles = append(rectangles, *queued)
			lastX = queued.X + queued.Width
			queued = nil
			queuedPoint = nil
		}

		if point.quadrant.upper() {
			minY = point
		} else {
			maxY = point
		}

		queueRectangle := point.quadrant == hullQ1 && !point.convex ||
			point.quadrant == hullQ2 && point.convex ||
			point.quadrant == hullQ3 && point.convex ||
			point.quadrant == hullQ4 && !point.convex
		if queueRectangle && minY != nil && maxY != nil {
			rectangle := geometry.Rectangle{
				X: lastX, Y: minY.y,
				Width: point.x - lastX, Height: maxY.y - minY.y,
			}
			queued = &rectangle
			queuedPoint = point
		}
	}
	if queued != nil {
		rectangles = append(rectangles, *queued)
	}
	return rectangles
}
