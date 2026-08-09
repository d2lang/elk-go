// Copyright (c) 2011, 2018 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"fmt"
	"sort"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// SortPortLists puts ports into clockwise order starting with the leftmost
// northern port. Sort stability intentionally preserves model order on ties.
func SortPortLists(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	strategy := basegraph.Get(&arena.Graph(graphID).Properties, layered.PortSortingStrategyOption)
	for _, nodeID := range arena.GraphAllNodes(graphID) {
		node := arena.Node(nodeID)
		constraints := basegraph.Get(&node.Properties, layered.PortConstraintsOption)
		if constraints.IsOrderFixed() {
			var sortErr error
			sort.SliceStable(node.Ports, func(firstIndex, secondIndex int) bool {
				comparison, err := compareFixedPorts(arena, node.Ports[firstIndex], node.Ports[secondIndex])
				if err != nil {
					sortErr = err
					return false
				}
				return comparison < 0
			})
			if sortErr != nil {
				return sortErr
			}
		} else if constraints.IsSideFixed() {
			sort.SliceStable(node.Ports, func(firstIndex, secondIndex int) bool {
				return arena.Port(node.Ports[firstIndex]).Side < arena.Port(node.Ports[secondIndex]).Side
			})
			reversePortSide(arena, node.Ports, layered.SouthSide)
			reversePortSide(arena, node.Ports, layered.WestSide)
			if strategy == layered.PortSortingPortDegree {
				sort.SliceStable(node.Ports, func(firstIndex, secondIndex int) bool {
					first, second := arena.Port(node.Ports[firstIndex]), arena.Port(node.Ports[secondIndex])
					if first.Side != second.Side {
						return false
					}
					switch first.Side {
					case layered.EastSide:
						return realPortDegree(arena, node.Ports[firstIndex], false) >
							realPortDegree(arena, node.Ports[secondIndex], false)
					case layered.WestSide:
						return realPortDegree(arena, node.Ports[firstIndex], true) <
							realPortDegree(arena, node.Ports[secondIndex], true)
					default:
						return false
					}
				})
			}
		}
		arena.CachePortSides(nodeID)
	}
	return nil
}

func realPortDegree(arena *layered.LGraphArena, portID layered.LPortID, incoming bool) int {
	edges := arena.Port(portID).OutgoingEdges
	if incoming {
		edges = arena.Port(portID).IncomingEdges
	}
	degree := 0
	for _, edgeID := range edges {
		if !basegraph.Get(&arena.Edge(edgeID).Properties, layered.Reversed) {
			degree++
		}
	}
	return degree
}

func compareFixedPorts(arena *layered.LGraphArena, firstID, secondID layered.LPortID) (int, error) {
	first, second := arena.Port(firstID), arena.Port(secondID)
	if first.Side != second.Side {
		if first.Side < second.Side {
			return -1, nil
		}
		return 1, nil
	}
	var firstPosition, secondPosition float64
	switch first.Side {
	case layered.NorthSide:
		firstPosition, secondPosition = first.Position.X, second.Position.X
	case layered.EastSide:
		firstPosition, secondPosition = first.Position.Y, second.Position.Y
	case layered.SouthSide:
		firstPosition, secondPosition = -first.Position.X, -second.Position.X
	case layered.WestSide:
		firstPosition, secondPosition = -first.Position.Y, -second.Position.Y
	default:
		return 0, fmt.Errorf("layered: cannot sort a port with undefined side")
	}
	if firstPosition < secondPosition {
		return -1, nil
	}
	if firstPosition > secondPosition {
		return 1, nil
	}
	return 0, nil
}

func reversePortSide(arena *layered.LGraphArena, ports []layered.LPortID, side layered.PortSide) {
	lowIndex, highIndex := findPortSideRange(arena, ports, side)
	// Match ELK 0.8.1's PortListSorter.reverse exactly. In particular, a
	// range of two ports is left alone and highIndex is exclusive.
	if highIndex <= lowIndex+2 {
		return
	}
	count := (highIndex - lowIndex) / 2
	for index := 0; index < count; index++ {
		left, right := lowIndex+index, highIndex-index-1
		ports[left], ports[right] = ports[right], ports[left]
	}
}

// findPortSideRange intentionally mirrors ELK 0.8.1's implementation,
// including its use of the low-index port while advancing highIndex and its
// ports.size()-1 upper bound. These details affect the observable assignment
// of implicit FREE ports to edges and thus must not be normalized here.
func findPortSideRange(
	arena *layered.LGraphArena,
	ports []layered.LPortID,
	side layered.PortSide,
) (int, int) {
	if len(ports) == 0 {
		return 0, 0
	}

	currentSide := arena.Port(ports[0]).Side
	lowIndex := 0
	lowerBound := side
	upperBound := side + 1
	for lowIndex < len(ports)-1 && currentSide < lowerBound {
		lowIndex++
		currentSide = arena.Port(ports[lowIndex]).Side
	}
	highIndex := lowIndex
	for highIndex < len(ports)-1 && currentSide < upperBound {
		highIndex++
		currentSide = arena.Port(ports[lowIndex]).Side
	}
	return lowIndex, highIndex
}
