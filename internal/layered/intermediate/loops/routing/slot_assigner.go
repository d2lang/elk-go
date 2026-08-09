// Copyright (c) 2019, 2020 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package routing

import (
	"sort"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/layered/intermediate/loops"
	"github.com/d2lang/elk-go/internal/layered/p5edges/orthogonal"
)

type slotAssigner struct {
	arena        *layered.LGraphArena
	segments     []*orthogonal.HyperEdgeSegment
	loopSegments map[*loops.HyperLoop]*orthogonal.HyperEdgeSegment
	activity     map[*loops.HyperLoop][]bool
}

func AssignRoutingSlots(arena *layered.LGraphArena, holder *loops.Holder) {
	assigner := &slotAssigner{arena: arena}
	labelCrossings := assigner.computeLabelCrossingMatrix(holder)
	assigner.createCrossingGraph(arena, holder, labelCrossings)
	random := basegraph.Get(&arena.Graph(arena.NodeGraph(holder.LNode)).Properties, layered.Random)
	orthogonal.BreakNonCriticalCycles(assigner.segments, random)
	assigner.assignRawSlotsToSegments()
	assigner.assignRawSlotsToLoops(holder)
	assigner.shiftTowardsNode(arena, holder, labelCrossings)
}

func (assigner *slotAssigner) computeLabelCrossingMatrix(holder *loops.Holder) [][]bool {
	labelID := 0
	for _, loop := range holder.HyperLoops {
		if loop.Labels != nil {
			loop.Labels.ID = labelID
			labelID++
		}
	}
	matrix := make([][]bool, labelID)
	for index := range matrix {
		matrix[index] = make([]bool, labelID)
	}
	for first := 0; first < len(holder.HyperLoops); first++ {
		firstLoop := holder.HyperLoops[first]
		if firstLoop.Labels == nil {
			continue
		}
		for second := first + 1; second < len(holder.HyperLoops); second++ {
			secondLoop := holder.HyperLoops[second]
			if secondLoop.Labels == nil {
				continue
			}
			overlap := labelsOverlap(firstLoop, secondLoop)
			matrix[firstLoop.Labels.ID][secondLoop.Labels.ID] = overlap
			matrix[secondLoop.Labels.ID][firstLoop.Labels.ID] = overlap
		}
	}
	return matrix
}

func labelsOverlap(first, second *loops.HyperLoop) bool {
	firstLabels, secondLabels := first.Labels, second.Labels
	if firstLabels == nil || secondLabels == nil || firstLabels.Side != secondLabels.Side ||
		firstLabels.Side == layered.EastSide || firstLabels.Side == layered.WestSide {
		return false
	}
	firstStart, secondStart := firstLabels.Position.X, secondLabels.Position.X
	firstEnd := firstStart + firstLabels.Size.X
	secondEnd := secondStart + secondLabels.Size.X
	return firstStart <= secondEnd && firstEnd >= secondStart
}

func (assigner *slotAssigner) createCrossingGraph(
	arena *layered.LGraphArena,
	holder *loops.Holder,
	labelCrossings [][]bool,
) {
	assigner.segments = make([]*orthogonal.HyperEdgeSegment, 0, len(holder.HyperLoops))
	assigner.loopSegments = make(map[*loops.HyperLoop]*orthogonal.HyperEdgeSegment)
	for _, loop := range holder.HyperLoops {
		segment := orthogonal.NewBareSegment()
		assigner.segments = append(assigner.segments, segment)
		assigner.loopSegments[loop] = segment
	}
	assigner.computeLoopActivity(arena, holder)
	for first := 0; first < len(holder.HyperLoops)-1; first++ {
		for second := first + 1; second < len(holder.HyperLoops); second++ {
			assigner.createDependencies(holder.HyperLoops[first], holder.HyperLoops[second], labelCrossings)
		}
	}
}

func (assigner *slotAssigner) computeLoopActivity(arena *layered.LGraphArena, holder *loops.Holder) {
	assigner.activity = make(map[*loops.HyperLoop][]bool)
	portCount := len(arena.Node(holder.LNode).Ports)
	for _, loop := range holder.HyperLoops {
		activity := make([]bool, portCount)
		assigner.activity[loop] = activity
		index := arena.Port(loop.LeftmostPort.LPort).ID - 1
		target := arena.Port(loop.RightmostPort.LPort).ID
		for index != target {
			index = (index + 1) % portCount
			activity[index] = true
		}
	}
}

func (assigner *slotAssigner) createDependencies(first, second *loops.HyperLoop, matrix [][]bool) {
	firstAboveSecond := assigner.countCrossings(first, second)
	secondAboveFirst := assigner.countCrossings(second, first)
	firstSegment := assigner.loopSegments[first]
	secondSegment := assigner.loopSegments[second]
	switch {
	case firstAboveSecond < secondAboveFirst:
		orthogonal.AddRegularDependency(firstSegment, secondSegment, secondAboveFirst-firstAboveSecond)
	case secondAboveFirst < firstAboveSecond:
		orthogonal.AddRegularDependency(secondSegment, firstSegment, firstAboveSecond-secondAboveFirst)
	case firstAboveSecond != 0 || matrixLabelsOverlap(first, second, matrix):
		orthogonal.AddRegularDependency(firstSegment, secondSegment, 0)
		orthogonal.AddRegularDependency(secondSegment, firstSegment, 0)
	}
}

func (assigner *slotAssigner) countCrossings(upper, lower *loops.HyperLoop) int {
	lowerActivity := assigner.activity[lower]
	result := 0
	for _, port := range upper.Ports {
		// RoutingDirector assigned IDs immediately before this processor.
		if lowerActivity[assigner.arena.Port(port.LPort).ID] {
			result++
		}
	}
	return result
}

func matrixLabelsOverlap(first, second *loops.HyperLoop, matrix [][]bool) bool {
	if first.Labels == nil || second.Labels == nil {
		return false
	}
	return matrix[first.Labels.ID][second.Labels.ID]
}

func (assigner *slotAssigner) assignRawSlotsToSegments() {
	queue := make([]*orthogonal.HyperEdgeSegment, 0)
	for _, segment := range assigner.segments {
		segment.InWeight = len(segment.IncomingDependencies)
		segment.OutWeight = len(segment.OutgoingDependencies)
		if segment.OutWeight == 0 {
			segment.RoutingSlot = 0
			queue = append(queue, segment)
		}
	}
	for len(queue) != 0 {
		segment := queue[0]
		queue = queue[1:]
		nextSlot := segment.RoutingSlot + 1
		for _, dependency := range segment.IncomingDependencies {
			source := dependency.Source
			if nextSlot > source.RoutingSlot {
				source.RoutingSlot = nextSlot
			}
			source.OutWeight--
			if source.OutWeight == 0 {
				queue = append(queue, source)
			}
		}
	}
}

func (assigner *slotAssigner) assignRawSlotsToLoops(holder *loops.Holder) {
	for _, loop := range holder.HyperLoops {
		slot := assigner.loopSegments[loop].RoutingSlot
		for side := layered.NorthSide; side <= layered.WestSide; side++ {
			if loop.OccupiedSides[side] {
				loop.SetRoutingSlot(side, slot)
			}
		}
	}
}

func (assigner *slotAssigner) shiftTowardsNode(
	arena *layered.LGraphArena,
	holder *loops.Holder,
	matrix [][]bool,
) {
	nextFree := make([]int, len(arena.Node(holder.LNode).Ports))
	for side := layered.NorthSide; side <= layered.WestSide; side++ {
		assigner.shiftSide(arena, holder, side, nextFree, matrix)
	}
}

func (assigner *slotAssigner) shiftSide(
	arena *layered.LGraphArena,
	holder *loops.Holder,
	side layered.PortSide,
	nextFree []int,
	matrix [][]bool,
) {
	loopList := make([]*loops.HyperLoop, 0)
	for _, loop := range holder.HyperLoops {
		if loop.OccupiedSides[side] {
			loopList = append(loopList, loop)
		}
	}
	sort.SliceStable(loopList, func(first, second int) bool {
		return loopList[first].RoutingSlot[side] < loopList[second].RoutingSlot[side]
	})
	minPort, maxPort := int(^uint(0)>>1), -int(^uint(0)>>1)-1
	for _, portID := range arena.Node(holder.LNode).Ports {
		port := arena.Port(portID)
		if port.Side == side {
			if port.ID < minPort {
				minPort = port.ID
			}
			if port.ID > maxPort {
				maxPort = port.ID
			}
		}
	}
	if maxPort < minPort {
		for index, loop := range loopList {
			loop.SetRoutingSlot(side, index)
		}
		return
	}

	labelSlots := make([]int, len(matrix))
	for index := range labelSlots {
		labelSlots[index] = -1
	}
	for _, loop := range loopList {
		activity := assigner.activity[loop]
		lowest := 0
		for index := minPort; index <= maxPort; index++ {
			if activity[index] && nextFree[index] > lowest {
				lowest = nextFree[index]
			}
		}
		if loop.Labels != nil {
			conflicts := make(map[int]struct{})
			for other := range matrix {
				if matrix[loop.Labels.ID][other] {
					conflicts[labelSlots[other]] = struct{}{}
				}
			}
			for {
				if _, conflict := conflicts[lowest]; !conflict {
					break
				}
				lowest++
			}
		}
		loop.SetRoutingSlot(side, lowest)
		for index := minPort; index <= maxPort; index++ {
			if activity[index] {
				nextFree[index] = lowest + 1
			}
		}
		if loop.Labels != nil {
			labelSlots[loop.Labels.ID] = lowest
		}
	}
}
