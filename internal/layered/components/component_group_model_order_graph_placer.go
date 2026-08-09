// Copyright (c) 2022 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package components

import (
	"fmt"
	"math"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// componentGroupModelOrderGraphPlacer is ELK 0.8.1's
// ComponentGroupModelOrderGraphPlacer. Unlike the base placer, it only tries
// to append to the latest group and advances groups in layout direction.
type componentGroupModelOrderGraphPlacer struct {
	sectorPlacer    componentGroupGraphPlacer
	componentGroups []*modelOrderComponentGroup
}

func (placer *componentGroupModelOrderGraphPlacer) combine(
	arena *layered.LGraphArena,
	components []layered.LGraphID,
	targetID layered.LGraphID,
) error {
	placer.componentGroups = nil
	for _, componentID := range components {
		if componentID == targetID {
			return fmt.Errorf("components: target graph cannot be a component group member")
		}
	}
	target := arena.Graph(targetID)
	target.LayerlessNodes = nil
	if len(components) == 0 {
		target.Size = geometry.Point{}
		return nil
	}

	firstComponent := arena.Graph(components[0])
	target.Properties = firstComponent.Properties.Clone()
	for _, componentID := range components {
		placer.addComponent(arena, componentID)
	}

	spaceBlockedBySouthEdges := geometry.Point{}
	spaceBlockedByComponents := geometry.Point{}
	offset := geometry.Point{}
	maxSize := geometry.Point{}
	spacing := basegraph.Get(
		&firstComponent.Properties, layered.SpacingComponentComponent)
	direction := basegraph.Get(&target.Properties, layered.DirectionOption)
	for _, group := range placer.componentGroups {
		if direction == layered.DirectionRight || direction == layered.DirectionLeft {
			offset.X = spaceBlockedBySouthEdges.X
			for _, sides := range group.group.portSides() {
				if sides&sidesNorth != 0 {
					offset.X = spaceBlockedByComponents.X
					break
				}
			}
		} else if direction == layered.DirectionDown || direction == layered.DirectionUp {
			offset.Y = spaceBlockedBySouthEdges.Y
			for _, sides := range group.group.portSides() {
				if sides&sidesWest != 0 {
					offset.Y = spaceBlockedByComponents.Y
					break
				}
			}
		}

		groupSize := placer.sectorPlacer.placeComponents(arena, group.group, spacing)
		offsetGraphs(arena, group.group.components(), offset)
		if direction == layered.DirectionRight || direction == layered.DirectionLeft {
			spaceBlockedByComponents.X = offset.X + groupSize.X
			maxSize.X = math.Max(maxSize.X, spaceBlockedByComponents.X)
			for _, sides := range group.group.portSides() {
				if sides&sidesSouth != 0 {
					spaceBlockedBySouthEdges.X = offset.X + groupSize.X
					break
				}
			}
			spaceBlockedByComponents.Y = offset.Y + groupSize.Y
			offset.Y = spaceBlockedByComponents.Y
			maxSize.Y = math.Max(maxSize.Y, offset.Y)
		} else if direction == layered.DirectionDown || direction == layered.DirectionUp {
			spaceBlockedByComponents.Y = offset.Y + groupSize.Y
			maxSize.Y = math.Max(maxSize.Y, spaceBlockedByComponents.Y)
			for _, sides := range group.group.portSides() {
				if sides&sidesEast != 0 {
					spaceBlockedBySouthEdges.Y = offset.Y + groupSize.Y
					break
				}
			}
			spaceBlockedByComponents.X = offset.X + groupSize.X
			offset.X = spaceBlockedByComponents.X
			maxSize.X = math.Max(maxSize.X, offset.X)
		}
	}
	target.Size = geometry.Point{X: maxSize.X - spacing, Y: maxSize.Y - spacing}

	if basegraph.Get(&firstComponent.Properties, connectedComponentsCompaction) &&
		basegraph.Get(&firstComponent.Properties, layered.EdgeRoutingOption) ==
			layered.EdgeRoutingOrthogonal {
		if err := compactExternalComponentGroups(
			arena, components, targetID, spacing); err != nil {
			return err
		}
	}
	for _, group := range placer.componentGroups {
		moveGraphs(arena, targetID, group.group.components(), geometry.Point{})
	}
	return nil
}

func (placer *componentGroupModelOrderGraphPlacer) addComponent(
	arena *layered.LGraphArena,
	component layered.LGraphID,
) {
	if len(placer.componentGroups) != 0 {
		last := placer.componentGroups[len(placer.componentGroups)-1]
		if last.add(arena, component) {
			return
		}
	}
	placer.componentGroups = append(
		placer.componentGroups, newModelOrderComponentGroup(arena, component))
}
