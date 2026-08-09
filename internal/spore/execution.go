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

func growTree(root *tree) bool {
	overlapsExisted := false
	var growAt func(*tree)
	growAt = func(parent *tree) {
		for _, child := range parent.children {
			child.node.translate(parent.node.vertex.Sub(parent.node.original.point))
			factor := overlap(parent.node.rect, child.node.rect)
			if factor > 1 {
				overlapsExisted = true
			}
			child.node.setCenterPosition(parent.node.vertex.Add(
				child.node.original.point.Sub(parent.node.original.point).Scale(factor),
			))
			growAt(child)
		}
	}
	growAt(root)
	return overlapsExisted
}

func compactTree(root *tree, orthogonal bool) {
	var compactAt func(*tree)
	compactAt = func(parent *tree) {
		for _, child := range parent.children {
			compactAt(child)
		}
		for _, child := range parent.children {
			vector := parent.node.vertex.Sub(child.node.vertex)
			if orthogonal {
				parentRect, childRect := parent.node.rect, child.node.rect
				if math.Abs(vector.X) >= math.Abs(vector.Y) {
					vector.Y = 0
					if childRect.Y+childRect.Height > parentRect.Y && childRect.Y < parentRect.Y+parentRect.Height {
						distance := math.Max(
							parentRect.X-(childRect.X+childRect.Width),
							childRect.X-(parentRect.X+parentRect.Width),
						)
						vector = vector.ScaleToLength(distance)
					}
				} else {
					vector.X = 0
					if childRect.X+childRect.Width > parentRect.X && childRect.X < parentRect.X+parentRect.Width {
						distance := math.Max(
							parentRect.Y-(childRect.Y+childRect.Height),
							childRect.Y-(parentRect.Y+parentRect.Height),
						)
						vector = vector.ScaleToLength(distance)
					}
				}
			} else {
				vector = vector.ScaleToLength(parent.node.underlap(child.node))
			}
			minimum := getMinUnderlap(root, child, vector.Length(), vector)
			vector = vector.ScaleToLength(minimum)
			translateSubtree(child, vector)
		}
	}
	compactAt(root)
}

func getMinUnderlap(entire, subtree *tree, current float64, vector geometry.Point) float64 {
	minimum := math.Min(current, minUnderlapWithSubtree(entire.node, subtree, current, vector))
	for _, child := range entire.children {
		if child != subtree {
			minimum = math.Min(minimum, getMinUnderlap(child, subtree, minimum, vector))
		}
	}
	return minimum
}

func minUnderlapWithSubtree(outside *node, subtree *tree, current float64, vector geometry.Point) float64 {
	minimum := current
	for _, child := range subtree.children {
		candidate := child.node
		if outside.touches(candidate) {
			if fuzzyCompare(candidate.rect.X, outside.rect.X+outside.rect.Width, fuzziness) == 0 && vector.X < 0 ||
				fuzzyCompare(candidate.rect.X+candidate.rect.Width, outside.rect.X, fuzziness) == 0 && vector.X > 0 ||
				fuzzyCompare(candidate.rect.Y, outside.rect.Y+outside.rect.Height, fuzziness) == 0 && vector.Y < 0 ||
				fuzzyCompare(candidate.rect.Y+candidate.rect.Height, outside.rect.Y, fuzziness) == 0 && vector.Y > 0 {
				minimum = 0
				break
			}
		} else {
			minimum = math.Min(minimum, outside.distance(candidate, vector))
		}
		minimum = math.Min(minimum, minUnderlapWithSubtree(outside, child, minimum, vector))
	}
	return minimum
}

func translateSubtree(subtree *tree, vector geometry.Point) {
	subtree.node.translate(vector)
	for _, child := range subtree.children {
		translateSubtree(child, vector)
	}
}
