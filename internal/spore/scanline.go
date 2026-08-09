// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package spore

import "sort"

type timestamp struct {
	node *node
	low  bool
}

func scanlineOverlaps(nodes []*node) []*tEdge {
	timestamps := make([]timestamp, 0, len(nodes)*2)
	for _, candidate := range nodes {
		timestamps = append(timestamps, timestamp{node: candidate, low: true}, timestamp{node: candidate})
	}
	sort.SliceStable(timestamps, func(i, j int) bool {
		first, second := timestamps[i], timestamps[j]
		firstY := first.node.rect.Y
		if !first.low {
			firstY += first.node.rect.Height
		}
		secondY := second.node.rect.Y
		if !second.low {
			secondY += second.node.rect.Height
		}
		if compared := javaDoubleCompare(firstY, secondY); compared != 0 {
			return compared < 0
		}
		// The 0.8.1 comparator deliberately processes an ending border before
		// a beginning border at the same coordinate.
		return !first.low && second.low
	})

	active := make([]*node, 0)
	edges := make([]*tEdge, 0)
	for _, event := range timestamps {
		if !event.low {
			for index, candidate := range active {
				if candidate == event.node {
					active = append(active[:index], active[index+1:]...)
					break
				}
			}
			continue
		}

		insert := sort.Search(len(active), func(index int) bool {
			return compareScanlineNodes(active[index], event.node) >= 0
		})
		active = append(active, nil)
		copy(active[insert+1:], active[insert:])
		active[insert] = event.node
		overlapsFound := false
		for _, other := range active {
			if scanlineOverlap(event.node, other) {
				edges = addEdge(edges, &tEdge{u: event.node.original, v: other.original})
				overlapsFound = true
			} else if overlapsFound {
				break
			}
		}
	}
	return edges
}

func compareScanlineNodes(first, second *node) int {
	if compared := javaDoubleCompare(first.rect.X, second.rect.X); compared != 0 {
		return compared
	}
	if compared := javaDoubleCompare(first.original.point.X, second.original.point.X); compared != 0 {
		return compared
	}
	return javaDoubleCompare(first.original.point.Y, second.original.point.Y)
}

func scanlineOverlap(first, second *node) bool {
	if first == nil || second == nil || first == second {
		return false
	}
	return fuzzyCompare(first.rect.X, second.rect.X+second.rect.Width, fuzziness) < 0 &&
		fuzzyCompare(second.rect.X, first.rect.X+first.rect.Width, fuzziness) < 0
}
