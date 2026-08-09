// Copyright (c) 2022 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package components

import "github.com/d2lang/elk-go/internal/layered"

// modelOrderConstraints is ModelOrderComponentGroup.MODEL_ORDER_CONSTRAINTS.
// It contains only the constraints in addition to componentConstraints.
var modelOrderConstraints = map[uint8][]uint8{
	sidesNorth: {
		sidesNone, sidesEast, sidesSouth, sidesWest, sidesNorthEast,
		sidesEastSouth, sidesSouthWest, sidesEastWest, sidesEastSouthWest,
		sidesNorthSouthWest, sidesNorthEastSouth,
	},
	sidesWest: {
		sidesNone, sidesEast, sidesSouth, sidesNorthEast, sidesEastSouth,
		sidesSouthWest, sidesEastSouthWest, sidesNorthEastSouth,
	},
	sidesNorthEast: {
		sidesNone, sidesEast, sidesSouth, sidesWest, sidesNorthEast,
		sidesEastSouth, sidesSouthWest, sidesEastWest, sidesEastSouthWest,
		sidesNorthSouthWest, sidesNorthEastSouth, sidesNorthEastSouthWest,
	},
	sidesNorthWest: {
		sidesNone, sidesNorth, sidesEast, sidesSouth, sidesWest, sidesNorthEast,
		sidesEastSouth, sidesSouthWest, sidesEastWest, sidesEastSouthWest,
		sidesNorthEastSouth, sidesNorthEastSouthWest,
	},
	sidesNorthSouthWest: {
		sidesNone, sidesEast, sidesSouth, sidesNorthEast, sidesEastSouth,
	},
	sidesNorthEastWest: {
		sidesNone, sidesEast, sidesSouth, sidesWest, sidesEastSouth,
		sidesSouthWest, sidesEastWest, sidesEastSouthWest,
	},
	sidesNone: {
		sidesEast, sidesSouth, sidesEastSouth, sidesSouthWest,
		sidesEastSouthWest, sidesNorthEastSouth,
	},
	sidesEast: {
		sidesSouth, sidesEastSouth, sidesSouthWest, sidesEastSouthWest,
		sidesNorthSouthWest,
	},
	sidesSouth: {
		sidesEastSouth, sidesNorthSouthWest, sidesNorthEastSouth,
	},
	sidesNorthSouth: {
		sidesEast, sidesSouth, sidesEastSouth, sidesSouthWest,
		sidesEastSouthWest, sidesNorthEastSouth, sidesNone, sidesNorth,
	},
	sidesEastWest: {
		sidesEast, sidesSouth, sidesEastSouth, sidesSouthWest,
		sidesEastSouthWest, sidesNone, sidesWest,
	},
	sidesSouthWest: {
		sidesSouth, sidesEastSouth, sidesSouthWest, sidesEastWest,
	},
	sidesNorthEastSouth: {
		sidesSouth, sidesSouthWest,
	},
	sidesNorthEastSouthWest: {
		sidesEastSouth, sidesSouthWest,
	},
}

type modelOrderComponentGroup struct {
	group          *componentGroup
	componentOrder []layered.LGraphID
}

func newModelOrderComponentGroup(
	arena *layered.LGraphArena,
	component layered.LGraphID,
) *modelOrderComponentGroup {
	group := &modelOrderComponentGroup{
		group: &componentGroup{componentsBySides: make(map[uint8][]layered.LGraphID)},
	}
	group.add(arena, component)
	// The source constructor appends the initial component a second time. The
	// list is not consumed by the 0.8.1 placer, but preserving it keeps the
	// translated object state source-shaped.
	group.componentOrder = append(group.componentOrder, component)
	return group
}

func (group *modelOrderComponentGroup) add(
	arena *layered.LGraphArena,
	component layered.LGraphID,
) bool {
	if !group.group.canAdd(arena, component, modelOrderConstraints) {
		return false
	}
	group.group.insert(arena, component)
	group.componentOrder = append(group.componentOrder, component)
	return true
}
