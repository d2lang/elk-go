// Copyright (c) 2019 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package routing

import (
	"fmt"
	"sort"

	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/layered/intermediate/loops"
	"github.com/d2lang/elk-go/internal/layered/intermediate/loops/ordering"
)

const (
	unconnectedPortPenalty = 1
	connectedPortPenalty   = 3
)

type director struct {
	portPenalties []int
}

// DetermineLoopRoutes chooses the clockwise interval occupied by each loop.
func DetermineLoopRoutes(arena *layered.LGraphArena, holder *loops.Holder) error {
	d := &director{}
	for index, portID := range arena.Node(holder.LNode).Ports {
		arena.Port(portID).ID = index
	}
	for _, loop := range holder.HyperLoops {
		sort.SliceStable(loop.Ports, func(first, second int) bool {
			return arena.Port(loop.Ports[first].LPort).ID < arena.Port(loop.Ports[second].LPort).ID
		})
	}

	for _, loop := range holder.HyperLoops {
		switch loop.Type {
		case loops.OneSide:
			side := arena.Port(loop.Ports[0].LPort).Side
			d.assignBoundaryPorts(arena, loop, side, side)
		case loops.TwoSidesCorner:
			sides := ordering.SortedTwoSideLoopPortSides(loop)
			d.assignBoundaryPorts(arena, loop, sides[0], sides[1])
		case loops.TwoSidesOpposing:
			d.determineOpposing(arena, loop)
		case loops.ThreeSides:
			d.determineThreeSides(arena, loop)
		case loops.FourSides:
			d.determineFourSides(arena, loop)
		default:
			return fmt.Errorf("unsupported self-loop type")
		}
		if loop.LeftmostPort == nil || loop.RightmostPort == nil {
			return fmt.Errorf("self-loop route has no boundary ports")
		}
		current := arena.Port(loop.LeftmostPort.LPort).Side
		target := arena.Port(loop.RightmostPort.LPort).Side
		for current != target {
			loop.OccupiedSides[current] = true
			current = sideRight(current)
		}
		loop.OccupiedSides[current] = true
	}
	return nil
}

func (d *director) determineOpposing(arena *layered.LGraphArena, loop *loops.HyperLoop) {
	sides := loop.PortSides()
	firstLeft := d.lowest(arena, loop, sides[0])
	firstRight := d.highest(arena, loop, sides[1])
	firstPenalty := d.edgePenalty(arena, loop.Holder, firstLeft, firstRight)
	secondLeft := d.lowest(arena, loop, sides[1])
	secondRight := d.highest(arena, loop, sides[0])
	secondPenalty := d.edgePenalty(arena, loop.Holder, secondLeft, secondRight)
	if firstPenalty <= secondPenalty {
		loop.LeftmostPort, loop.RightmostPort = firstLeft, firstRight
	} else {
		loop.LeftmostPort, loop.RightmostPort = secondLeft, secondRight
	}
}

func (d *director) determineThreeSides(arena *layered.LGraphArena, loop *loops.HyperLoop) {
	var left, right layered.PortSide
	switch {
	case !loop.HasPortsOnSide(layered.NorthSide):
		left, right = layered.EastSide, layered.WestSide
	case !loop.HasPortsOnSide(layered.EastSide):
		left, right = layered.SouthSide, layered.NorthSide
	case !loop.HasPortsOnSide(layered.SouthSide):
		left, right = layered.WestSide, layered.EastSide
	default:
		left, right = layered.NorthSide, layered.SouthSide
	}
	d.assignBoundaryPorts(arena, loop, left, right)
}

func (d *director) determineFourSides(arena *layered.LGraphArena, loop *loops.HyperLoop) {
	ports := loop.Ports
	worstLeft, worstRight := ports[len(ports)-1], ports[0]
	worstPenalty := d.edgePenalty(arena, loop.Holder, worstLeft, worstRight)
	for rightIndex := 1; rightIndex < len(ports); rightIndex++ {
		left, right := ports[rightIndex-1], ports[rightIndex]
		penalty := d.edgePenalty(arena, loop.Holder, left, right)
		if penalty > worstPenalty {
			worstLeft, worstRight, worstPenalty = left, right, penalty
		}
	}
	// The selected gap is precisely the part not traversed by the loop.
	loop.LeftmostPort, loop.RightmostPort = worstRight, worstLeft
}

func (d *director) assignBoundaryPorts(
	arena *layered.LGraphArena,
	loop *loops.HyperLoop,
	left, right layered.PortSide,
) {
	loop.LeftmostPort = d.lowest(arena, loop, left)
	loop.RightmostPort = d.highest(arena, loop, right)
}

func (d *director) lowest(arena *layered.LGraphArena, loop *loops.HyperLoop, side layered.PortSide) *loops.Port {
	result := loop.PortsBySide[side][0]
	for _, port := range loop.PortsBySide[side][1:] {
		if arena.Port(port.LPort).ID < arena.Port(result.LPort).ID {
			result = port
		}
	}
	return result
}

func (d *director) highest(arena *layered.LGraphArena, loop *loops.HyperLoop, side layered.PortSide) *loops.Port {
	result := loop.PortsBySide[side][0]
	for _, port := range loop.PortsBySide[side][1:] {
		if arena.Port(port.LPort).ID > arena.Port(result.LPort).ID {
			result = port
		}
	}
	return result
}

func (d *director) edgePenalty(
	arena *layered.LGraphArena,
	holder *loops.Holder,
	left, right *loops.Port,
) int {
	if d.portPenalties == nil {
		d.computePenalties(arena, holder)
	}
	portCount := len(arena.Node(holder.LNode).Ports)
	leftID := arena.Port(left.LPort).ID
	leftOfRightID := arena.Port(right.LPort).ID - 1
	if leftOfRightID < 0 {
		leftOfRightID = portCount - 1
	}
	if leftID <= leftOfRightID {
		return d.portPenalties[leftOfRightID] - d.portPenalties[leftID]
	}
	return d.portPenalties[portCount-1] - d.portPenalties[leftID] + d.portPenalties[leftOfRightID]
}

func (d *director) computePenalties(arena *layered.LGraphArena, holder *loops.Holder) {
	ports := arena.Node(holder.LNode).Ports
	d.portPenalties = make([]int, len(ports))
	total := 0
	for index, portID := range ports {
		port := arena.Port(portID)
		if len(port.IncomingEdges) == 0 && len(port.OutgoingEdges) == 0 {
			total += unconnectedPortPenalty
		} else {
			total += connectedPortPenalty
		}
		d.portPenalties[index] = total
	}
}

func sideRight(side layered.PortSide) layered.PortSide {
	switch side {
	case layered.NorthSide:
		return layered.EastSide
	case layered.EastSide:
		return layered.SouthSide
	case layered.SouthSide:
		return layered.WestSide
	case layered.WestSide:
		return layered.NorthSide
	default:
		return layered.UndefinedSide
	}
}

func sideLeft(side layered.PortSide) layered.PortSide {
	switch side {
	case layered.NorthSide:
		return layered.WestSide
	case layered.WestSide:
		return layered.SouthSide
	case layered.SouthSide:
		return layered.EastSide
	case layered.EastSide:
		return layered.NorthSide
	default:
		return layered.UndefinedSide
	}
}
