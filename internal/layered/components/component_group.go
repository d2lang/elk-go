// Copyright (c) 2012, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package components

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// ELK represents an external-port side set as an EnumSet. The arena stores
// the same set as a bit mask whose bit index is PortSide's ordinal.
const (
	sidesNone  uint8 = 0
	sidesNorth uint8 = 1 << uint8(layered.NorthSide)
	sidesEast  uint8 = 1 << uint8(layered.EastSide)
	sidesSouth uint8 = 1 << uint8(layered.SouthSide)
	sidesWest  uint8 = 1 << uint8(layered.WestSide)

	sidesNorthEast          = sidesNorth | sidesEast
	sidesNorthSouth         = sidesNorth | sidesSouth
	sidesNorthWest          = sidesNorth | sidesWest
	sidesEastSouth          = sidesEast | sidesSouth
	sidesEastWest           = sidesEast | sidesWest
	sidesSouthWest          = sidesSouth | sidesWest
	sidesNorthEastSouth     = sidesNorth | sidesEast | sidesSouth
	sidesNorthEastWest      = sidesNorth | sidesEast | sidesWest
	sidesNorthSouthWest     = sidesNorth | sidesSouth | sidesWest
	sidesEastSouthWest      = sidesEast | sidesSouth | sidesWest
	sidesNorthEastSouthWest = sidesNorth | sidesEast | sidesSouth | sidesWest
)

// componentConstraints is ComponentGroup.CONSTRAINTS. The map is deliberately
// directional: the key is the candidate being inserted and each value is a
// side combination that must not already occur in the group.
var componentConstraints = map[uint8][]uint8{
	sidesNone:               {sidesNorthEastSouthWest},
	sidesWest:               {sidesNorthEastSouthWest, sidesNorthSouthWest},
	sidesEast:               {sidesNorthEastSouth, sidesNorthEastSouthWest},
	sidesNorth:              {sidesNorthEastSouthWest, sidesNorthEastWest},
	sidesSouth:              {sidesEastSouthWest, sidesNorthEastSouthWest},
	sidesNorthSouth:         {sidesEastWest, sidesNorthEastSouthWest, sidesNorthEastWest, sidesEastSouthWest},
	sidesEastWest:           {sidesNorthSouth, sidesNorthSouthWest, sidesNorthEastSouth, sidesNorthEastSouthWest},
	sidesNorthWest:          {sidesNorthWest, sidesNorthEastWest, sidesNorthSouthWest},
	sidesNorthEast:          {sidesNorthEast, sidesNorthEastWest, sidesNorthEastSouth},
	sidesSouthWest:          {sidesSouthWest, sidesEastSouthWest, sidesNorthSouthWest},
	sidesEastSouth:          {sidesEastSouth, sidesEastSouthWest, sidesNorthEastSouth},
	sidesNorthEastWest:      {sidesNorth, sidesNorthSouth, sidesNorthWest, sidesNorthEast, sidesNorthEastSouthWest, sidesNorthEastWest, sidesNorthSouthWest, sidesNorthEastSouth},
	sidesEastSouthWest:      {sidesSouth, sidesNorthSouth, sidesSouthWest, sidesEastSouth, sidesEastSouthWest, sidesNorthSouthWest, sidesNorthEastSouth, sidesNorthEastSouthWest},
	sidesNorthSouthWest:     {sidesWest, sidesEastWest, sidesNorthWest, sidesSouthWest, sidesNorthEastWest, sidesEastSouthWest, sidesNorthSouthWest, sidesNorthEastSouthWest},
	sidesNorthEastSouth:     {sidesEast, sidesEastWest, sidesNorthEast, sidesEastSouth, sidesNorthEastWest, sidesEastSouthWest, sidesNorthEastSouth, sidesNorthEastSouthWest},
	sidesNorthEastSouthWest: {sidesNone, sidesWest, sidesEast, sidesNorth, sidesSouth, sidesNorthSouth, sidesEastWest, sidesNorthEastWest, sidesEastSouthWest, sidesNorthSouthWest, sidesNorthEastSouth, sidesNorthEastSouthWest},
}

type componentGroup struct {
	componentsBySides map[uint8][]layered.LGraphID
	sideOrder         []uint8
}

func newComponentGroup(
	arena *layered.LGraphArena,
	component layered.LGraphID,
) *componentGroup {
	group := &componentGroup{componentsBySides: make(map[uint8][]layered.LGraphID)}
	group.add(arena, component)
	return group
}

func (group *componentGroup) add(
	arena *layered.LGraphArena,
	component layered.LGraphID,
) bool {
	if !group.canAdd(arena, component, nil) {
		return false
	}
	group.insert(arena, component)
	return true
}

func (group *componentGroup) canAdd(
	arena *layered.LGraphArena,
	component layered.LGraphID,
	extraConstraints map[uint8][]uint8,
) bool {
	candidateSides := basegraph.Get(
		&arena.Graph(component).Properties, layered.ExternalPortConnections)
	for _, constraints := range [...][]uint8{
		componentConstraints[candidateSides], extraConstraints[candidateSides],
	} {
		for _, constraint := range constraints {
			if len(group.componentsBySides[constraint]) != 0 {
				return false
			}
		}
	}
	return true
}

func (group *componentGroup) insert(
	arena *layered.LGraphArena,
	component layered.LGraphID,
) {
	sides := basegraph.Get(
		&arena.Graph(component).Properties, layered.ExternalPortConnections)
	if len(group.componentsBySides[sides]) == 0 {
		group.sideOrder = append(group.sideOrder, sides)
	}
	group.componentsBySides[sides] = append(group.componentsBySides[sides], component)
}

func (group *componentGroup) componentsFor(sides uint8) []layered.LGraphID {
	return group.componentsBySides[sides]
}

func (group *componentGroup) components() []layered.LGraphID {
	components := make([]layered.LGraphID, 0)
	for _, sides := range group.sideOrder {
		components = append(components, group.componentsBySides[sides]...)
	}
	return components
}

func (group *componentGroup) portSides() []uint8 {
	return group.sideOrder
}
