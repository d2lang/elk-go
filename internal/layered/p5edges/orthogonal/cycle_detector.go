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

	"github.com/d2lang/elk-go/internal/javacompat"
)

// DetectCycles ports HyperEdgeCycleDetector.detectCycles. When criticalOnly
// is false, critical arcs are still protected from reversal.
func DetectCycles(segments []*HyperEdgeSegment, criticalOnly bool, random *javacompat.Random) []*HyperEdgeSegmentDependency {
	return detectCycles(segments, criticalOnly, random, &cycleScratch{})
}

// cycleScratch is the working storage of detectCycles; a generator keeps one
// for its RouteEdges calls. Every list is emptied before it is filled.
type cycleScratch struct {
	sources, sinks, order, maximum []*HyperEdgeSegment
	result                         []*HyperEdgeSegmentDependency
}

// detectCycles is DetectCycles with its lists in scratch; the result is
// valid until scratch is used again.
func detectCycles(
	segments []*HyperEdgeSegment,
	criticalOnly bool,
	random *javacompat.Random,
	scratch *cycleScratch,
) []*HyperEdgeSegmentDependency {
	if random == nil {
		random = javacompat.NewRandom(1)
	}
	sources, sinks := initializeCycleDetector(segments, criticalOnly, scratch.sources[:0], scratch.sinks[:0])
	computeLinearOrderingMarks(segments, sources, sinks, criticalOnly, random, scratch)

	result := scratch.result[:0]
	for _, source := range segments {
		for _, dependency := range source.OutgoingDependencies {
			if (!criticalOnly || dependency.Type == CriticalDependency) && source.Mark > dependency.Target.Mark {
				result = append(result, dependency)
			}
		}
	}
	scratch.result = result
	return result
}

func initializeCycleDetector(
	segments []*HyperEdgeSegment,
	criticalOnly bool,
	sources, sinks []*HyperEdgeSegment,
) ([]*HyperEdgeSegment, []*HyperEdgeSegment) {
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
	scratch *cycleScratch,
) {
	// The source and sink queues are walked by head index (the order of
	// taking the first element each time); they and the other lists are
	// handed back to scratch for the next call.
	sourceHead, sinkHead := 0, 0
	// TreeSet orders the initial negative marks, so its stable iteration order
	// is the reverse of the input list. Mutating marks does not rebalance Java's
	// tree; we likewise retain this frozen order.
	unprocessedOrder := append(scratch.order[:0], segments...)
	// GoStableSort is sort.SliceStable without reflection (same comparisons,
	// same permutation).
	javacompat.GoStableSort(unprocessedOrder, func(first, second *HyperEdgeSegment) bool { return first.Mark < second.Mark })
	// The set of unprocessed segments is a flag on each segment plus a count
	// (membership tests and removals only).
	unprocessed := 0
	for _, segment := range segments {
		if !segment.unprocessed {
			segment.unprocessed = true
			unprocessed++
		}
	}
	markBase := len(segments)
	nextSinkMark, nextSourceMark := markBase-1, markBase+1

	maxSegments := scratch.maximum[:0]
	for unprocessed > 0 {
		for sinkHead < len(sinks) {
			sink := sinks[sinkHead]
			sinkHead++
			if !sink.unprocessed {
				continue
			}
			sink.unprocessed = false
			unprocessed--
			sink.Mark = nextSinkMark
			nextSinkMark--
			sources, sinks = updateNeighbors(sink, sources, sinks, criticalOnly)
		}
		for sourceHead < len(sources) {
			source := sources[sourceHead]
			sourceHead++
			if !source.unprocessed {
				continue
			}
			source.unprocessed = false
			unprocessed--
			source.Mark = nextSourceMark
			nextSourceMark++
			sources, sinks = updateNeighbors(source, sources, sinks, criticalOnly)
		}

		maxOutflow := math.MinInt
		maxSegments = maxSegments[:0]
		for _, segment := range unprocessedOrder {
			if !segment.unprocessed {
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
			if selected.unprocessed {
				selected.unprocessed = false
				unprocessed--
			}
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
	scratch.sources, scratch.sinks = sources[:0], sinks[:0]
	scratch.order, scratch.maximum = unprocessedOrder[:0], maxSegments[:0]
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
	breakNonCriticalCycles(segments, random, &cycleScratch{})
}

// breakNonCriticalCycles is BreakNonCriticalCycles with the detector's lists
// in scratch (removing or reversing a dependency changes no list of it).
func breakNonCriticalCycles(segments []*HyperEdgeSegment, random *javacompat.Random, scratch *cycleScratch) {
	for _, dependency := range detectCycles(segments, false, random, scratch) {
		if dependency.Weight == 0 {
			dependency.Remove()
		} else {
			dependency.Reverse()
		}
	}
}
