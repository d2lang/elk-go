// Copyright (c) 2019 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package ordering

import (
	"fmt"
	"sort"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/layered/intermediate/loops"
)

type portSideArea uint8

const (
	areaStart portSideArea = iota
	areaMiddle
	areaEnd
)

type addMode uint8

const (
	appendPorts addMode = iota
	prependPorts
)

type restorer struct {
	loopsByType [5][]*loops.HyperLoop
	targetAreas [5][3][]*loops.Port
}

// RestorePorts ports ELK 0.8.1's PortRestorer, including its historical
// split-index predicate and the precise burst order used to merge regular and
// restored ports.
func RestorePorts(arena *layered.LGraphArena, holder *loops.Holder) error {
	r := &restorer{}
	for _, loop := range holder.HyperLoops {
		r.loopsByType[loop.Type] = append(r.loopsByType[loop.Type], loop)
	}
	if err := r.processOneSideLoops(arena, holder); err != nil {
		return err
	}
	r.processTwoSideCornerLoops()
	r.processThreeSideLoops()
	r.processFourSideLoops()
	r.processTwoSideOpposingLoops()
	r.restore(arena, holder)
	for side := layered.UndefinedSide; side <= layered.WestSide; side++ {
		for area := areaStart; area <= areaEnd; area++ {
			for _, port := range r.targetAreas[side][area] {
				port.Hidden = false
			}
		}
	}
	holder.PortsHidden = false
	return nil
}

func (r *restorer) processOneSideLoops(arena *layered.LGraphArena, holder *loops.Holder) error {
	ordering := basegraph.Get(&arena.Node(holder.LNode).Properties, layered.SelfLoopOrdering)
	if ordering == layered.SelfLoopOrderingReverseStacked {
		oneSided := r.loopsByType[loops.OneSide]
		for left, right := 0, len(oneSided)-1; left < right; left, right = left+1, right-1 {
			oneSided[left], oneSided[right] = oneSided[right], oneSided[left]
		}
	}
	for _, loop := range r.loopsByType[loops.OneSide] {
		side := arena.Port(loop.Ports[0].LPort).Side
		sortedPorts := append([]*loops.Port(nil), loop.Ports...)
		sort.SliceStable(sortedPorts, func(first, second int) bool {
			return sortedPorts[first].NetFlow() < sortedPorts[second].NetFlow()
		})
		switch ordering {
		case layered.SelfLoopOrderingSequenced:
			r.addToTargetArea(sortedPorts, side, areaMiddle, appendPorts)
		case layered.SelfLoopOrderingStacked, layered.SelfLoopOrderingReverseStacked:
			split := computePortListSplitIndex(sortedPorts)
			r.addToTargetArea(sortedPorts[:split], side, areaMiddle, prependPorts)
			r.addToTargetArea(sortedPorts[split:], side, areaMiddle, appendPorts)
		default:
			return fmt.Errorf("unsupported self-loop ordering strategy")
		}
	}
	return nil
}

func computePortListSplitIndex(ports []*loops.Port) int {
	positive := 0
	for positive < len(ports) && ports[positive].NetFlow() <= 0 {
		positive++
	}
	if positive > 0 && positive < len(ports)-1 {
		return positive
	}

	// The 0.8.1 source says non-negative but tests > 0 here as well, and its
	// second bound references positive. Both quirks are observable.
	nonNegative := 0
	for nonNegative < len(ports) && ports[nonNegative].NetFlow() <= 0 {
		nonNegative++
	}
	if nonNegative > 0 && positive < len(ports)-1 {
		return nonNegative
	}
	return len(ports) / 2
}

func (r *restorer) processTwoSideCornerLoops() {
	for _, loop := range r.loopsByType[loops.TwoSidesCorner] {
		sides := SortedTwoSideLoopPortSides(loop)
		r.addLoopToTargetArea(loop, sides[0], areaEnd, prependPorts)
		r.addLoopToTargetArea(loop, sides[1], areaStart, appendPorts)
	}
}

func (r *restorer) processTwoSideOpposingLoops() {
	for _, loop := range r.loopsByType[loops.TwoSidesOpposing] {
		sides := SortedTwoSideLoopPortSides(loop)
		r.addLoopToTargetArea(loop, sides[0], areaEnd, prependPorts)
		r.addLoopToTargetArea(loop, sides[1], areaStart, appendPorts)
	}
}

func SortedTwoSideLoopPortSides(loop *loops.HyperLoop) [2]layered.PortSide {
	sidesList := loop.PortSides()
	sides := [2]layered.PortSide{sidesList[0], sidesList[1]}
	if sides[0] == layered.NorthSide && sides[1] == layered.WestSide {
		sides[0], sides[1] = sides[1], sides[0]
	}
	return sides
}

var (
	constellationNES = [3]layered.PortSide{layered.NorthSide, layered.EastSide, layered.SouthSide}
	constellationESW = [3]layered.PortSide{layered.EastSide, layered.SouthSide, layered.WestSide}
	constellationSWN = [3]layered.PortSide{layered.SouthSide, layered.WestSide, layered.NorthSide}
	constellationWNE = [3]layered.PortSide{layered.WestSide, layered.NorthSide, layered.EastSide}
)

func (r *restorer) processThreeSideLoops() {
	for _, loop := range r.loopsByType[loops.ThreeSides] {
		sides := determineLoopConstellation(loop)
		r.addLoopToTargetArea(loop, sides[0], areaEnd, prependPorts)
		r.addLoopToTargetArea(loop, sides[1], areaMiddle, appendPorts)
		r.addLoopToTargetArea(loop, sides[2], areaStart, appendPorts)
	}
}

func determineLoopConstellation(loop *loops.HyperLoop) [3]layered.PortSide {
	switch {
	case !loop.HasPortsOnSide(layered.NorthSide):
		return constellationESW
	case !loop.HasPortsOnSide(layered.EastSide):
		return constellationSWN
	case !loop.HasPortsOnSide(layered.SouthSide):
		return constellationWNE
	default:
		return constellationNES
	}
}

func (r *restorer) processFourSideLoops() {
	for _, loop := range r.loopsByType[loops.FourSides] {
		for _, side := range loop.PortSides() {
			r.addLoopToTargetArea(loop, side, areaMiddle, appendPorts)
		}
	}
}

func (r *restorer) addLoopToTargetArea(
	loop *loops.HyperLoop,
	side layered.PortSide,
	area portSideArea,
	mode addMode,
) {
	r.addToTargetArea(loop.PortsBySide[side], side, area, mode)
}

func (r *restorer) addToTargetArea(
	ports []*loops.Port,
	side layered.PortSide,
	area portSideArea,
	mode addMode,
) {
	hidden := make([]*loops.Port, 0, len(ports))
	for _, port := range ports {
		if port.Hidden {
			hidden = append(hidden, port)
		}
	}
	for left, right := 0, len(hidden)-1; left < right; left, right = left+1, right-1 {
		hidden[left], hidden[right] = hidden[right], hidden[left]
	}
	if mode == prependPorts {
		hidden = append(hidden, r.targetAreas[side][area]...)
		r.targetAreas[side][area] = hidden
	} else {
		r.targetAreas[side][area] = append(r.targetAreas[side][area], hidden...)
	}
}

func (r *restorer) restore(arena *layered.LGraphArena, holder *loops.Holder) {
	node := arena.Node(holder.LNode)
	oldPorts := append([]layered.LPortID(nil), node.Ports...)
	newPorts := make([]layered.LPortID, 0, len(oldPorts)+len(holder.Ports))
	nextOld := 0

	addTarget := func(side layered.PortSide, area portSideArea) {
		for _, slPort := range r.targetAreas[side][area] {
			portID := slPort.LPort
			nodeCopy := holder.LNode
			arena.Port(portID).Node = &nodeCopy
			newPorts = append(newPorts, portID)
		}
	}
	addOldThat := func(condition func(layered.LPortID) bool) {
		for nextOld < len(oldPorts) {
			portID := oldPorts[nextOld]
			if !condition(portID) {
				return
			}
			newPorts = append(newPorts, portID)
			nextOld++
		}
	}

	addTarget(layered.NorthSide, areaStart)
	addOldThat(func(port layered.LPortID) bool {
		return arena.Port(port).Side == layered.NorthSide && r.hasWestConnections(arena, port)
	})
	addTarget(layered.NorthSide, areaMiddle)
	addOldThat(func(port layered.LPortID) bool { return arena.Port(port).Side == layered.NorthSide })
	addTarget(layered.NorthSide, areaEnd)

	addTarget(layered.EastSide, areaStart)
	addTarget(layered.EastSide, areaMiddle)
	addOldThat(func(port layered.LPortID) bool { return arena.Port(port).Side == layered.EastSide })
	addTarget(layered.EastSide, areaEnd)

	addTarget(layered.SouthSide, areaStart)
	addOldThat(func(port layered.LPortID) bool {
		return arena.Port(port).Side == layered.SouthSide && r.hasEastConnections(arena, port)
	})
	addTarget(layered.SouthSide, areaMiddle)
	addOldThat(func(port layered.LPortID) bool { return arena.Port(port).Side == layered.SouthSide })
	addTarget(layered.SouthSide, areaEnd)

	addTarget(layered.WestSide, areaStart)
	addOldThat(func(port layered.LPortID) bool { return arena.Port(port).Side == layered.WestSide })
	addTarget(layered.WestSide, areaMiddle)
	addTarget(layered.WestSide, areaEnd)

	node.Ports = newPorts
	arena.InvalidatePortSideCache(holder.LNode)
}

func (r *restorer) hasWestConnections(arena *layered.LGraphArena, portID layered.LPortID) bool {
	sides := northSouthPortConnectionSides(arena, portID)
	return sides[layered.WestSide] || sides[layered.WestSide] && sides[layered.EastSide]
}

func (r *restorer) hasEastConnections(arena *layered.LGraphArena, portID layered.LPortID) bool {
	return northSouthPortConnectionSides(arena, portID)[layered.EastSide]
}

func northSouthPortConnectionSides(arena *layered.LGraphArena, portID layered.LPortID) [5]bool {
	var result [5]bool
	dummyID, found := basegraph.TryGet(&arena.Port(portID).Properties, layered.PortDummy)
	if !found {
		return result
	}
	for _, dummyPortID := range arena.Node(dummyID).Ports {
		origin, found := basegraph.TryGet(&arena.Port(dummyPortID).Properties, layered.OriginalElement)
		if found && origin.Kind == layered.OriginLPort && origin.LPort == portID &&
			len(arena.PortConnectedEdges(dummyPortID)) != 0 {
			result[arena.Port(dummyPortID).Side] = true
		}
	}
	return result
}
