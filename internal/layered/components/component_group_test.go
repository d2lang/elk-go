// Copyright (c) 2012, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package components

import (
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestComponentGroupValidConstraints(t *testing.T) {
	tests := []struct {
		name  string
		sides []uint8
	}{
		{
			name: "cardinal and center",
			sides: []uint8{
				sidesNorth, sidesNorth, sidesSouth, sidesSouth,
				sidesWest, sidesWest, sidesEast, sidesEast,
				sidesNone, sidesNone,
			},
		},
		{
			name: "horizontal and center",
			sides: []uint8{
				sidesEastWest, sidesEastWest, sidesWest, sidesWest,
				sidesEast, sidesEast, sidesNone, sidesNone,
			},
		},
		{
			name: "vertical and center",
			sides: []uint8{
				sidesNorthSouth, sidesNorthSouth, sidesNorth, sidesNorth,
				sidesSouth, sidesSouth, sidesNone, sidesNone,
			},
		},
		{
			name: "all sides and corners",
			sides: []uint8{
				sidesNorthEastSouthWest, sidesNorthWest, sidesNorthEast,
				sidesSouthWest, sidesEastSouth,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			arena := layered.NewLGraphArena()
			group := &componentGroup{componentsBySides: make(map[uint8][]layered.LGraphID)}
			for index, sides := range test.sides {
				if !group.add(arena, componentWithSides(arena, sides)) {
					t.Fatalf("component %d with sides %#x was rejected", index, sides)
				}
			}
		})
	}
}

func TestComponentGroupInvalidConstraints(t *testing.T) {
	tests := []struct {
		name       string
		initial    []uint8
		rejections []uint8
	}{
		{
			name:    "horizontal rejects vertical spans",
			initial: []uint8{sidesEastWest},
			rejections: []uint8{
				sidesNorthSouth, sidesNorthSouthWest, sidesNorthEastSouth,
			},
		},
		{
			name:    "vertical rejects horizontal spans",
			initial: []uint8{sidesNorthSouth},
			rejections: []uint8{
				sidesEastWest, sidesNorthEastWest, sidesEastSouthWest,
			},
		},
		{
			name: "four corners reject three-side spans",
			initial: []uint8{
				sidesNorthWest, sidesNorthEast, sidesSouthWest, sidesEastSouth,
			},
			rejections: []uint8{
				sidesNorthEastSouth, sidesNorthEastWest,
				sidesEastSouthWest, sidesNorthSouthWest,
			},
		},
		{
			name:    "all sides rejects every non-corner conflict",
			initial: []uint8{sidesNorthEastSouthWest},
			rejections: []uint8{
				sidesNone, sidesNorthSouth, sidesEastWest,
				sidesNorthEastSouth, sidesNorthEastWest,
				sidesEastSouthWest, sidesNorthSouthWest,
				sidesNorthEastSouthWest,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			arena := layered.NewLGraphArena()
			group := &componentGroup{componentsBySides: make(map[uint8][]layered.LGraphID)}
			for _, sides := range test.initial {
				if !group.add(arena, componentWithSides(arena, sides)) {
					t.Fatalf("initial component with sides %#x was rejected", sides)
				}
			}
			for _, sides := range test.rejections {
				if group.add(arena, componentWithSides(arena, sides)) {
					t.Fatalf("component with sides %#x was accepted", sides)
				}
			}
		})
	}
}

func componentWithSides(arena *layered.LGraphArena, sides uint8) layered.LGraphID {
	graphID := arena.AddGraph()
	basegraph.Set(&arena.Graph(graphID).Properties,
		layered.ExternalPortConnections, sides)
	return graphID
}
