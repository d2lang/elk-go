// Copyright (c) 2010, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package orthogonal

import (
	"math"
	"sort"

	"github.com/d2lang/elk-go/internal/javacompat"
)

// DetectCycles ports HyperEdgeCycleDetector.detectCycles. When criticalOnly
// is false, critical arcs are still protected from reversal.
func DetectCycles(segments []*HyperEdgeSegment, criticalOnly bool, random *javacompat.Random) []*HyperEdgeSegmentDependency {
	if random == nil {
		random = javacompat.NewRandom(1)
	}
	sources, sinks := initializeCycleDetector(segments, criticalOnly)
	computeLinearOrderingMarks(segments, sources, sinks, criticalOnly, random)

	result := make([]*HyperEdgeSegmentDependency, 0)
	for _, source := range segments {
		for _, dependency := range source.OutgoingDependencies {
			if (!criticalOnly || dependency.Type == CriticalDependency) && source.Mark > dependency.Target.Mark {
				result = append(result, dependency)
			}
		}
	}
	return result
}

func initializeCycleDetector(segments []*HyperEdgeSegment, criticalOnly bool) (sources, sinks []*HyperEdgeSegment) {
	nextMark := -1
	for _, segment := range segments {
		segment.Mark = nextMark
		nextMark--
		criticalIn, criticalOut := 0, 0
		for _, dependency := range segment.IncomingDependencies {
			if dependency.Type == CriticalDependency {
				criticalIn += dependency.Weight
			}
		}
		for _, dependency := range segment.OutgoingDependencies {
			if dependency.Type == CriticalDependency {
				criticalOut += dependency.Weight
			}
		}
		segment.CriticalInWeight, segment.CriticalOutWeight = criticalIn, criticalOut
		segment.InWeight, segment.OutWeight = criticalIn, criticalOut
		if !criticalOnly {
			segment.InWeight, segment.OutWeight = 0, 0
			for _, dependency := range segment.IncomingDependencies {
				segment.InWeight += dependency.Weight
			}
			for _, dependency := range segment.OutgoingDependencies {
				segment.OutWeight += dependency.Weight
			}
		}
		if segment.OutWeight == 0 {
			sinks = append(sinks, segment)
		} else if segment.InWeight == 0 {
			sources = append(sources, segment)
		}
	}
	return sources, sinks
}

func computeLinearOrderingMarks(
	segments, sources, sinks []*HyperEdgeSegment,
	criticalOnly bool,
	random *javacompat.Random,
) {
	// TreeSet orders the initial negative marks, so its stable iteration order
	// is the reverse of the input list. Mutating marks does not rebalance Java's
	// tree; we likewise retain this frozen order.
	unprocessedOrder := append([]*HyperEdgeSegment(nil), segments...)
	sort.SliceStable(unprocessedOrder, func(i, j int) bool { return unprocessedOrder[i].Mark < unprocessedOrder[j].Mark })
	unprocessed := make(map[*HyperEdgeSegment]bool, len(segments))
	for _, segment := range segments {
		unprocessed[segment] = true
	}
	markBase := len(segments)
	nextSinkMark, nextSourceMark := markBase-1, markBase+1

	for len(unprocessed) > 0 {
		for len(sinks) > 0 {
			sink := sinks[0]
			sinks = sinks[1:]
			if !unprocessed[sink] {
				continue
			}
			delete(unprocessed, sink)
			sink.Mark = nextSinkMark
			nextSinkMark--
			sources, sinks = updateNeighbors(sink, sources, sinks, criticalOnly)
		}
		for len(sources) > 0 {
			source := sources[0]
			sources = sources[1:]
			if !unprocessed[source] {
				continue
			}
			delete(unprocessed, source)
			source.Mark = nextSourceMark
			nextSourceMark++
			sources, sinks = updateNeighbors(source, sources, sinks, criticalOnly)
		}

		maxOutflow := math.MinInt
		maxSegments := make([]*HyperEdgeSegment, 0)
		for _, segment := range unprocessedOrder {
			if !unprocessed[segment] {
				continue
			}
			if !criticalOnly && segment.CriticalOutWeight > 0 && segment.CriticalInWeight <= 0 {
				maxSegments = append(maxSegments[:0], segment)
				break
			}
			outflow := segment.OutWeight - segment.InWeight
			if outflow >= maxOutflow {
				if outflow > maxOutflow {
					maxSegments = maxSegments[:0]
					maxOutflow = outflow
				}
				maxSegments = append(maxSegments, segment)
			}
		}
		if len(maxSegments) > 0 {
			selected := maxSegments[random.NextIntN(int32(len(maxSegments)))]
			delete(unprocessed, selected)
			selected.Mark = nextSourceMark
			nextSourceMark++
			sources, sinks = updateNeighbors(selected, sources, sinks, criticalOnly)
		}
	}

	shiftBase := len(segments) + 1
	for _, segment := range segments {
		if segment.Mark < markBase {
			segment.Mark += shiftBase
		}
	}
}

func updateNeighbors(
	segment *HyperEdgeSegment,
	sources, sinks []*HyperEdgeSegment,
	criticalOnly bool,
) ([]*HyperEdgeSegment, []*HyperEdgeSegment) {
	for _, dependency := range segment.OutgoingDependencies {
		if criticalOnly && dependency.Type != CriticalDependency {
			continue
		}
		target := dependency.Target
		if target.Mark < 0 && dependency.Weight > 0 {
			target.InWeight -= dependency.Weight
			if dependency.Type == CriticalDependency {
				target.CriticalInWeight -= dependency.Weight
			}
			if target.InWeight <= 0 && target.OutWeight > 0 {
				sources = append(sources, target)
			}
		}
	}
	for _, dependency := range segment.IncomingDependencies {
		if criticalOnly && dependency.Type != CriticalDependency {
			continue
		}
		source := dependency.Source
		if source.Mark < 0 && dependency.Weight > 0 {
			source.OutWeight -= dependency.Weight
			if dependency.Type == CriticalDependency {
				source.CriticalOutWeight -= dependency.Weight
			}
			if source.OutWeight <= 0 && source.InWeight > 0 {
				sinks = append(sinks, source)
			}
		}
	}
	return sources, sinks
}

// BreakNonCriticalCycles removes zero-weight two-cycle arcs and reverses all
// other feedback arcs selected by ELK's cycle detector.
func BreakNonCriticalCycles(segments []*HyperEdgeSegment, random *javacompat.Random) {
	for _, dependency := range DetectCycles(segments, false, random) {
		if dependency.Weight == 0 {
			dependency.Remove()
		} else {
			dependency.Reverse()
		}
	}
}
