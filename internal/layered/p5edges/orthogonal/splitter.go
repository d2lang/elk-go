// Copyright (c) 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package orthogonal

import "sort"

type segmentSplitter struct{ generator *Generator }

type freeArea struct {
	start float64
	end   float64
	size  float64
}

type areaRating struct {
	dependencies int
	crossings    int
}

func (splitter *segmentSplitter) splitSegments(
	dependencies []*HyperEdgeSegmentDependency,
	segments *[]*HyperEdgeSegment,
	criticalThreshold float64,
) {
	if len(dependencies) == 0 {
		return
	}
	areas := findFreeAreas(*segments, criticalThreshold)
	selected := decideWhichSegmentsToSplit(dependencies)
	// Stream.sorted is stable, preserving LinkedHashSet insertion order on ties.
	sort.SliceStable(selected, func(i, j int) bool { return selected[i].Length() < selected[j].Length() })
	for _, segment := range selected {
		splitter.split(segment, segments, &areas, criticalThreshold)
	}
}

func findFreeAreas(segments []*HyperEdgeSegment, criticalThreshold float64) []freeArea {
	coordinates := make([]float64, 0)
	for _, segment := range segments {
		coordinates = append(coordinates, segment.IncomingCoordinates...)
		coordinates = append(coordinates, segment.OutgoingCoordinates...)
	}
	sort.Float64s(coordinates)
	areas := make([]freeArea, 0)
	for index := 1; index < len(coordinates); index++ {
		if coordinates[index]-coordinates[index-1] >= 2*criticalThreshold {
			start := coordinates[index-1] + criticalThreshold
			end := coordinates[index] - criticalThreshold
			areas = append(areas, freeArea{start: start, end: end, size: end - start})
		}
	}
	return areas
}

func decideWhichSegmentsToSplit(dependencies []*HyperEdgeSegmentDependency) []*HyperEdgeSegment {
	selected := make([]*HyperEdgeSegment, 0)
	seen := make(map[*HyperEdgeSegment]bool)
	for _, dependency := range dependencies {
		source, target := dependency.Source, dependency.Target
		if seen[source] || seen[target] {
			continue
		}
		segment, cause := source, target
		if source.RepresentsHyperedge() && !target.RepresentsHyperedge() {
			segment, cause = target, source
		}
		selected = append(selected, segment)
		seen[segment] = true
		segment.SplitBy = cause
	}
	return selected
}

func (splitter *segmentSplitter) split(
	segment *HyperEdgeSegment,
	segments *[]*HyperEdgeSegment,
	areas *[]freeArea,
	criticalThreshold float64,
) {
	position := splitter.computePositionToSplitAndUpdateFreeAreas(segment, areas, criticalThreshold)
	*segments = append(*segments, segment.splitAt(position))
	splitter.updateDependencies(segment, *segments)
}

func (splitter *segmentSplitter) updateDependencies(segment *HyperEdgeSegment, segments []*HyperEdgeSegment) {
	cause, partner := segment.SplitBy, segment.SplitPartner
	AddCriticalDependency(segment, cause)
	AddCriticalDependency(cause, partner)
	for _, other := range segments {
		if other != cause && other != segment && other != partner {
			splitter.generator.CreateDependencyIfNecessary(other, segment)
			splitter.generator.CreateDependencyIfNecessary(other, partner)
		}
	}
}

func (splitter *segmentSplitter) computePositionToSplitAndUpdateFreeAreas(
	segment *HyperEdgeSegment,
	areas *[]freeArea,
	criticalThreshold float64,
) float64 {
	first, last := -1, -1
	for index, area := range *areas {
		if area.start > segment.End {
			break
		}
		if area.end >= segment.Start {
			if first < 0 {
				first = index
			}
			last = index
		}
	}
	position := center(segment.Start, segment.End)
	if first >= 0 {
		best := splitter.chooseBestAreaIndex(segment, *areas, first, last)
		position = center((*areas)[best].start, (*areas)[best].end)
		useArea(areas, best, criticalThreshold)
	}
	return position
}

func (splitter *segmentSplitter) chooseBestAreaIndex(
	segment *HyperEdgeSegment,
	areas []freeArea,
	from, to int,
) int {
	bestIndex := from
	if from < to {
		first, partner := segment.simulateSplit()
		bestArea := areas[bestIndex]
		bestRating := splitter.rateArea(segment, first, partner, bestArea)
		for index := from + 1; index <= to; index++ {
			area := areas[index]
			rating := splitter.rateArea(segment, first, partner, area)
			if isBetterArea(area, rating, bestArea, bestRating) {
				bestArea, bestRating = area, rating
				// ELK 0.8.1 does not update bestIndex here. This source bug is
				// observable and therefore intentionally retained for parity.
			}
		}
	}
	return bestIndex
}

func (splitter *segmentSplitter) rateArea(
	segment, first, partner *HyperEdgeSegment,
	area freeArea,
) areaRating {
	areaCenter := center(area.start, area.end)
	first.OutgoingCoordinates = append(first.OutgoingCoordinates[:0], areaCenter)
	partner.IncomingCoordinates = append(partner.IncomingCoordinates[:0], areaCenter)
	rating := areaRating{}
	for _, dependency := range segment.IncomingDependencies {
		splitter.updateConsideringBothOrderings(&rating, first, dependency.Source)
		splitter.updateConsideringBothOrderings(&rating, partner, dependency.Source)
	}
	for _, dependency := range segment.OutgoingDependencies {
		splitter.updateConsideringBothOrderings(&rating, first, dependency.Target)
		splitter.updateConsideringBothOrderings(&rating, partner, dependency.Target)
	}
	rating.dependencies += 2
	rating.crossings += countCrossingsForSingleOrdering(first, segment.SplitBy)
	rating.crossings += countCrossingsForSingleOrdering(segment.SplitBy, partner)
	return rating
}

func (*segmentSplitter) updateConsideringBothOrderings(
	rating *areaRating,
	first, second *HyperEdgeSegment,
) {
	firstLeft := countCrossingsForSingleOrdering(first, second)
	secondLeft := countCrossingsForSingleOrdering(second, first)
	if firstLeft == secondLeft {
		if firstLeft > 0 {
			rating.dependencies += 2
			rating.crossings += firstLeft
		}
	} else {
		rating.dependencies++
		rating.crossings += minInt(firstLeft, secondLeft)
	}
}

func countCrossingsForSingleOrdering(left, right *HyperEdgeSegment) int {
	return CountCrossings(left.OutgoingCoordinates, right.Start, right.End) +
		CountCrossings(right.IncomingCoordinates, left.Start, left.End)
}

func isBetterArea(current freeArea, currentRating areaRating, best freeArea, bestRating areaRating) bool {
	if currentRating.crossings < bestRating.crossings {
		return true
	}
	if currentRating.crossings == bestRating.crossings {
		if currentRating.dependencies < bestRating.dependencies {
			return true
		}
		return currentRating.dependencies == bestRating.dependencies && current.size > best.size
	}
	return false
}

func useArea(areas *[]freeArea, index int, criticalThreshold float64) {
	old := (*areas)[index]
	copy((*areas)[index:], (*areas)[index+1:])
	*areas = (*areas)[:len(*areas)-1]
	if old.size/2 < criticalThreshold {
		return
	}
	oldCenter := center(old.start, old.end)
	replacements := make([]freeArea, 0, 2)
	firstEnd := oldCenter - criticalThreshold
	if old.start <= firstEnd {
		replacements = append(replacements, freeArea{start: old.start, end: firstEnd, size: firstEnd - old.start})
	}
	secondStart := oldCenter + criticalThreshold
	if secondStart <= old.end {
		replacements = append(replacements, freeArea{start: secondStart, end: old.end, size: old.end - secondStart})
	}
	if len(replacements) > 0 {
		*areas = append(*areas, make([]freeArea, len(replacements))...)
		copy((*areas)[index+len(replacements):], (*areas)[index:len(*areas)-len(replacements)])
		copy((*areas)[index:], replacements)
	}
}

func center(first, second float64) float64 { return (first + second) / 2 }
