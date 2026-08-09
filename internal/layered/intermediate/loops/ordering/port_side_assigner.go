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

// AssignPortSides ports ELK 0.8.1's PortSideAssigner.
func AssignPortSides(arena *layered.LGraphArena, holder *loops.Holder) error {
	switch basegraph.Get(&arena.Node(holder.LNode).Properties, layered.SelfLoopDistribution) {
	case layered.SelfLoopDistributionNorth:
		for _, loop := range holder.HyperLoops {
			for _, port := range loop.Ports {
				if port.Hidden {
					arena.SetPortSide(port.LPort, layered.NorthSide)
				}
			}
		}

	case layered.SelfLoopDistributionNorthSouth:
		northPorts, southPorts := 0, 0
		for _, loop := range holder.HyperLoops {
			hidden := hiddenPorts(loop)
			side := layered.NorthSide
			if northPorts <= southPorts {
				northPorts += len(hidden)
			} else {
				side = layered.SouthSide
				southPorts += len(hidden)
			}
			for _, port := range hidden {
				arena.SetPortSide(port.LPort, side)
			}
		}

	case layered.SelfLoopDistributionEqually:
		assignToAllSides(arena, holder)

	default:
		return fmt.Errorf("unsupported self-loop distribution strategy")
	}

	for _, portID := range holder.PortOrder {
		if arena.Port(portID).Side == layered.UndefinedSide {
			return fmt.Errorf("self-loop port %d remains unassigned", portID)
		}
	}
	return nil
}

type assignmentTarget struct {
	first, second layered.PortSide
}

var assignmentTargets = []assignmentTarget{
	{layered.NorthSide, layered.NorthSide},
	{layered.SouthSide, layered.SouthSide},
	{layered.EastSide, layered.EastSide},
	{layered.WestSide, layered.WestSide},
	{layered.WestSide, layered.NorthSide},
	{layered.NorthSide, layered.EastSide},
	{layered.SouthSide, layered.WestSide},
	{layered.EastSide, layered.SouthSide},
}

func assignToAllSides(arena *layered.LGraphArena, holder *loops.Holder) {
	sortedLoops := append([]*loops.HyperLoop(nil), holder.HyperLoops...)
	sort.SliceStable(sortedLoops, func(first, second int) bool {
		return len(sortedLoops[first].Ports) > len(sortedLoops[second].Ports)
	})
	for index, loop := range sortedLoops {
		assignToTarget(arena, loop, assignmentTargets[index%len(assignmentTargets)])
	}
}

func assignToTarget(arena *layered.LGraphArena, loop *loops.HyperLoop, target assignmentTarget) {
	// Java sorts the loop's actual list here. List.sort is stable.
	if target.first != target.second {
		sort.SliceStable(loop.Ports, func(first, second int) bool {
			firstPort := arena.Port(loop.Ports[first].LPort)
			secondPort := arena.Port(loop.Ports[second].LPort)
			return len(firstPort.IncomingEdges)-len(firstPort.OutgoingEdges) <
				len(secondPort.IncomingEdges)-len(secondPort.OutgoingEdges)
		})
	}
	split := len(loop.Ports) / 2
	for index, port := range loop.Ports {
		if !port.Hidden {
			continue
		}
		side := target.second
		if index < split {
			side = target.first
		}
		arena.SetPortSide(port.LPort, side)
	}
}

func hiddenPorts(loop *loops.HyperLoop) []*loops.Port {
	result := make([]*loops.Port, 0, len(loop.Ports))
	for _, port := range loop.Ports {
		if port.Hidden {
			result = append(result, port)
		}
	}
	return result
}
