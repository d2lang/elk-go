// Copyright (c) 2012, 2020 Kiel University and others.
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
	"sort"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

var (
	priority                      = basegraph.PropertyWithDefault("org.eclipse.elk.priority", func() int { return 0 })
	connectedComponentsCompaction = basegraph.PropertyWithDefault(
		"org.eclipse.elk.layered.compaction.connectedComponents", func() bool { return false })
)

// simpleRowGraphPlacer ports ELK 0.12.0's SimpleRowGraphPlacer and, when
// modelOrder is set, ModelOrderRowGraphPlacer.
type simpleRowGraphPlacer struct {
	modelOrder bool
}

func (placer *simpleRowGraphPlacer) combine(
	arena *layered.LGraphArena,
	components []layered.LGraphID,
	targetID layered.LGraphID,
) error {
	target := arena.Graph(targetID)
	if len(components) == 1 {
		sourceID := components[0]
		if sourceID != targetID {
			source := arena.Graph(sourceID)
			target.LayerlessNodes = nil
			moveGraph(arena, targetID, sourceID, geometry.Point{})
			target.Properties = source.Properties.Clone()
			target.Padding = source.Padding
			target.Size = source.Size
		}
		return nil
	}
	if len(components) == 0 {
		target.LayerlessNodes = nil
		target.Size = geometry.Point{}
		return nil
	}
	for _, component := range components {
		if component == targetID {
			return fmt.Errorf("components: target graph cannot be one of several components")
		}
	}

	if !placer.modelOrder && basegraph.Get(
		&target.Properties, layered.ComponentOrdering,
	) == layered.ComponentOrderingNone {
		for _, componentID := range components {
			component := arena.Graph(componentID)
			componentPriority := 0
			for _, nodeID := range component.LayerlessNodes {
				componentPriority += basegraph.Get(&arena.Node(nodeID).Properties, priority)
			}
			component.ID = componentPriority
		}
		// Collections.sort is stable. The stable tie behavior is observable for
		// equal-size components and therefore must not be replaced by sort.Slice.
		sort.SliceStable(components, func(left, right int) bool {
			first, second := arena.Graph(components[left]), arena.Graph(components[right])
			if first.ID != second.ID {
				return first.ID > second.ID
			}
			firstArea := first.Size.X * first.Size.Y
			secondArea := second.Size.X * second.Size.Y
			return firstArea < secondArea
		})
	}

	firstComponent := arena.Graph(components[0])
	target.LayerlessNodes = nil
	target.Properties = firstComponent.Properties.Clone()

	maxRowWidth, totalArea := 0.0, 0.0
	for _, componentID := range components {
		size := arena.Graph(componentID).Size
		maxRowWidth = math.Max(maxRowWidth, size.X)
		totalArea += size.X * size.Y
	}
	// The float cast in the Java source is intentional and survives GWT.
	areaWidth := float64(float32(math.Sqrt(totalArea))) *
		basegraph.Get(&target.Properties, layered.AspectRatio)
	maxRowWidth = math.Max(maxRowWidth, areaWidth)
	componentSpacing := basegraph.Get(
		&target.Properties, layered.SpacingComponentComponent)

	if placer.modelOrder {
		placeComponentsInModelOrder(
			arena, components, target, maxRowWidth, componentSpacing)
	} else {
		placeComponentsInRows(arena, components, target, maxRowWidth, componentSpacing)
	}

	if basegraph.Get(&firstComponent.Properties, connectedComponentsCompaction) {
		compactedSize, err := compactConnectedComponents(
			arena, components, componentSpacing)
		if err != nil {
			return err
		}
		target.Size = compactedSize
	}
	moveGraphs(arena, targetID, components, geometry.Point{})
	return nil
}

func placeComponentsInRows(
	arena *layered.LGraphArena,
	components []layered.LGraphID,
	target *layered.LGraph,
	maxRowWidth, componentSpacing float64,
) {
	xPosition, yPosition := 0.0, 0.0
	highestBox, broadestRow := 0.0, componentSpacing
	for _, componentID := range components {
		component := arena.Graph(componentID)
		if xPosition+component.Size.X > maxRowWidth {
			xPosition = 0
			yPosition += highestBox + componentSpacing
			highestBox = 0
		}
		offsetGraph(arena, componentID, geometry.Point{
			X: xPosition + component.Offset.X,
			Y: yPosition + component.Offset.Y,
		})
		component.Offset = geometry.Point{}
		broadestRow = math.Max(broadestRow, xPosition+component.Size.X)
		highestBox = math.Max(highestBox, component.Size.Y)
		xPosition += component.Size.X + componentSpacing
	}
	target.Size = geometry.Point{X: broadestRow, Y: yPosition + highestBox}
}

// placeComponentsInModelOrder is ELK 0.12.0's ModelOrderRowGraphPlacer.
// Components arrive sorted by their smallest node model order. External-port
// sides then constrain row breaks without reordering the components.
func placeComponentsInModelOrder(
	arena *layered.LGraphArena,
	components []layered.LGraphID,
	target *layered.LGraph,
	maxRowWidth, componentSpacing float64,
) {
	xPosition, yPosition := 0.0, 0.0
	highestBox, broadestRow := 0.0, componentSpacing
	startXOfRow := 0.0
	var lastComponent *layered.LGraph
	for _, componentID := range components {
		component := arena.Graph(componentID)
		sides := basegraph.Get(&component.Properties, layered.ExternalPortConnections)
		lastHasEast := lastComponent != nil && hasExternalPortSide(
			basegraph.Get(&lastComponent.Properties, layered.ExternalPortConnections),
			layered.EastSide,
		)
		if xPosition+component.Size.X > maxRowWidth &&
			!hasExternalPortSide(sides, layered.NorthSide) ||
			lastHasEast || hasExternalPortSide(sides, layered.WestSide) {
			xPosition = startXOfRow
			yPosition += highestBox + componentSpacing
			highestBox = 0
		}
		if hasExternalPortSide(sides, layered.NorthSide) {
			xPosition = broadestRow + componentSpacing
		}
		offsetGraph(arena, componentID, geometry.Point{
			X: xPosition + component.Offset.X,
			Y: yPosition + component.Offset.Y,
		})
		broadestRow = math.Max(broadestRow, xPosition+component.Size.X)
		if hasExternalPortSide(sides, layered.SouthSide) {
			startXOfRow = math.Max(startXOfRow, xPosition+component.Size.X+componentSpacing)
		}
		component.Offset = geometry.Point{}
		highestBox = math.Max(highestBox, component.Size.Y)
		xPosition += component.Size.X + componentSpacing
		lastComponent = component
	}
	target.Size = geometry.Point{X: broadestRow, Y: yPosition + highestBox}
}

func hasExternalPortSide(sides uint8, side layered.PortSide) bool {
	return sides&(uint8(1)<<uint8(side)) != 0
}
