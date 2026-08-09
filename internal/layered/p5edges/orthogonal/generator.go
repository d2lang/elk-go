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

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
)

const (
	Tolerance                       = 1e-3
	criticalConflictsDetected       = -1
	conflictThresholdFactor         = 0.5
	criticalConflictThresholdFactor = 0.2
	conflictPenalty                 = 1
	crossingPenalty                 = 16
)

// Generator is ELK 0.8.1's direction-independent orthogonal routing
// generator. One instance is reused for every adjacent layer pair in phase 5.
type Generator struct {
	segmentSplitter           *segmentSplitter
	strategy                  *routingDirectionStrategy
	edgeSpacing               float64
	conflictThreshold         float64
	criticalConflictThreshold float64
}

func NewGenerator(direction RoutingDirection, edgeSpacing float64) *Generator {
	return &Generator{
		strategy:          newRoutingDirectionStrategy(direction),
		edgeSpacing:       edgeSpacing,
		conflictThreshold: conflictThresholdFactor * edgeSpacing,
	}
}

// RouteEdges routes the connected components formed by output ports on the
// selected sides of two adjacent layers. A nil node slice represents a graph
// boundary, just like a null Java iterable.
func (generator *Generator) RouteEdges(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	sourceNodes []layered.LNodeID,
	sourceLayerIndex int,
	targetNodes []layered.LNodeID,
	startPosition float64,
) int {
	_ = sourceLayerIndex // retained because it identifies debug graphs upstream
	portSegments := make(map[layered.LPortID]*HyperEdgeSegment)
	segments := make([]*HyperEdgeSegment, 0)
	generator.createHyperEdgeSegments(arena, sourceNodes, generator.strategy.sourcePortSide(), &segments, portSegments)
	generator.createHyperEdgeSegments(arena, targetNodes, generator.strategy.targetPortSide(), &segments, portSegments)

	generator.criticalConflictThreshold = criticalConflictThresholdFactor * minimumHorizontalSegmentDistance(segments)
	criticalDependencies := 0
	for first := 0; first < len(segments)-1; first++ {
		for second := first + 1; second < len(segments); second++ {
			criticalDependencies += generator.CreateDependencyIfNecessary(segments[first], segments[second])
		}
	}
	random := basegraph.Get(&arena.Graph(graphID).Properties, layered.Random)
	if random == nil {
		random = javacompat.NewRandom(int64(basegraph.Get(&arena.Graph(graphID).Properties, layered.RandomSeed)))
		basegraph.Set(&arena.Graph(graphID).Properties, layered.Random, random)
	}
	if criticalDependencies >= 2 {
		segments = generator.breakCriticalCycles(segments, random)
	}
	BreakNonCriticalCycles(segments, random)
	TopologicalNumbering(segments)

	rankCount := -1
	for _, segment := range segments {
		if math.Abs(segment.Start-segment.End) < Tolerance {
			continue
		}
		if segment.RoutingSlot > rankCount {
			rankCount = segment.RoutingSlot
		}
		generator.strategy.calculateBendPoints(arena, segment, startPosition, generator.edgeSpacing)
	}
	generator.strategy.clearCreatedJunctionPoints()
	return rankCount + 1
}

func (generator *Generator) createHyperEdgeSegments(
	arena *layered.LGraphArena,
	nodes []layered.LNodeID,
	side layered.PortSide,
	segments *[]*HyperEdgeSegment,
	portSegments map[layered.LPortID]*HyperEdgeSegment,
) {
	for _, nodeID := range nodes {
		for _, portID := range arena.Node(nodeID).Ports {
			port := arena.Port(portID)
			if port.Side != side || len(port.OutgoingEdges) == 0 {
				continue
			}
			if _, found := portSegments[portID]; found {
				continue
			}
			segment := newHyperEdgeSegment(generator.strategy)
			*segments = append(*segments, segment)
			segment.addPortPositions(arena, portID, portSegments)
		}
	}
}

func minimumHorizontalSegmentDistance(segments []*HyperEdgeSegment) float64 {
	incoming, outgoing := make([]float64, 0), make([]float64, 0)
	for _, segment := range segments {
		incoming = append(incoming, segment.IncomingCoordinates...)
		outgoing = append(outgoing, segment.OutgoingCoordinates...)
	}
	return math.Min(minimumDifference(incoming), minimumDifference(outgoing))
}

func minimumDifference(values []float64) float64 {
	if len(values) < 2 {
		return math.MaxFloat64
	}
	values = append([]float64(nil), values...)
	sort.Float64s(values)
	minimum := math.MaxFloat64
	previous := values[0]
	for index := 1; index < len(values); index++ {
		current := values[index]
		if current == previous {
			continue
		}
		minimum = math.Min(minimum, current-previous)
		previous = current
	}
	return minimum
}

// CreateDependencyIfNecessary ports the conflict and crossing comparison used
// both during graph construction and after a critical segment split.
func (generator *Generator) CreateDependencyIfNecessary(first, second *HyperEdgeSegment) int {
	if math.Abs(first.Start-first.End) < Tolerance || math.Abs(second.Start-second.End) < Tolerance {
		return 0
	}
	conflictsFirst := generator.countConflicts(first.OutgoingCoordinates, second.IncomingCoordinates)
	conflictsSecond := generator.countConflicts(second.OutgoingCoordinates, first.IncomingCoordinates)
	if conflictsFirst == criticalConflictsDetected || conflictsSecond == criticalConflictsDetected {
		count := 0
		if conflictsFirst == criticalConflictsDetected {
			AddCriticalDependency(second, first)
			count++
		}
		if conflictsSecond == criticalConflictsDetected {
			AddCriticalDependency(first, second)
			count++
		}
		return count
	}

	crossingsFirst := CountCrossings(first.OutgoingCoordinates, second.Start, second.End) +
		CountCrossings(second.IncomingCoordinates, first.Start, first.End)
	crossingsSecond := CountCrossings(second.OutgoingCoordinates, first.Start, first.End) +
		CountCrossings(first.IncomingCoordinates, second.Start, second.End)
	valueFirst := conflictPenalty*conflictsFirst + crossingPenalty*crossingsFirst
	valueSecond := conflictPenalty*conflictsSecond + crossingPenalty*crossingsSecond
	switch {
	case valueFirst < valueSecond:
		AddRegularDependency(first, second, valueSecond-valueFirst)
	case valueFirst > valueSecond:
		AddRegularDependency(second, first, valueFirst-valueSecond)
	case valueFirst > 0 && valueSecond > 0:
		AddRegularDependency(first, second, 0)
		AddRegularDependency(second, first, 0)
	}
	return 0
}

func (generator *Generator) countConflicts(first, second []float64) int {
	if len(first) == 0 || len(second) == 0 {
		return 0
	}
	conflicts, firstIndex, secondIndex := 0, 0, 0
	firstPosition, secondPosition := first[0], second[0]
	for {
		if firstPosition > secondPosition-generator.criticalConflictThreshold &&
			firstPosition < secondPosition+generator.criticalConflictThreshold {
			return criticalConflictsDetected
		}
		if firstPosition > secondPosition-generator.conflictThreshold &&
			firstPosition < secondPosition+generator.conflictThreshold {
			conflicts++
		}
		if firstPosition <= secondPosition && firstIndex+1 < len(first) {
			firstIndex++
			firstPosition = first[firstIndex]
		} else if secondPosition <= firstPosition && secondIndex+1 < len(second) {
			secondIndex++
			secondPosition = second[secondIndex]
		} else {
			break
		}
	}
	return conflicts
}

func CountCrossings(positions []float64, start, end float64) int {
	crossings := 0
	for _, position := range positions {
		if position > end {
			break
		}
		if position >= start {
			crossings++
		}
	}
	return crossings
}

func (generator *Generator) breakCriticalCycles(segments []*HyperEdgeSegment, random *javacompat.Random) []*HyperEdgeSegment {
	dependencies := DetectCycles(segments, true, random)
	if generator.segmentSplitter == nil {
		generator.segmentSplitter = &segmentSplitter{generator: generator}
	}
	generator.segmentSplitter.splitSegments(dependencies, &segments, generator.criticalConflictThreshold)
	return segments
}

// TopologicalNumbering assigns routing slots and then pushes purely
// right-pointing targets as far toward their target layer as possible.
func TopologicalNumbering(segments []*HyperEdgeSegment) {
	sources, rightwardTargets := make([]*HyperEdgeSegment, 0), make([]*HyperEdgeSegment, 0)
	for _, segment := range segments {
		segment.InWeight = len(segment.IncomingDependencies)
		segment.OutWeight = len(segment.OutgoingDependencies)
		if segment.InWeight == 0 {
			sources = append(sources, segment)
		}
		if segment.OutWeight == 0 && len(segment.IncomingCoordinates) == 0 {
			rightwardTargets = append(rightwardTargets, segment)
		}
	}
	maxRank := -1
	for len(sources) > 0 {
		segment := sources[0]
		sources = sources[1:]
		for _, dependency := range segment.OutgoingDependencies {
			target := dependency.Target
			target.RoutingSlot = maxInt(target.RoutingSlot, segment.RoutingSlot+1)
			maxRank = maxInt(maxRank, target.RoutingSlot)
			target.InWeight--
			if target.InWeight == 0 {
				sources = append(sources, target)
			}
		}
	}
	if maxRank <= -1 {
		return
	}
	for _, segment := range rightwardTargets {
		segment.RoutingSlot = maxRank
	}
	for len(rightwardTargets) > 0 {
		segment := rightwardTargets[0]
		rightwardTargets = rightwardTargets[1:]
		for _, dependency := range segment.IncomingDependencies {
			source := dependency.Source
			if len(source.IncomingCoordinates) > 0 {
				continue
			}
			source.RoutingSlot = minInt(source.RoutingSlot, segment.RoutingSlot-1)
			source.OutWeight--
			if source.OutWeight == 0 {
				rightwardTargets = append(rightwardTargets, source)
			}
		}
	}
}

func minInt(first, second int) int {
	if first < second {
		return first
	}
	return second
}

func maxInt(first, second int) int {
	if first > second {
		return first
	}
	return second
}
