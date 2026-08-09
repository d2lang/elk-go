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

// simpleRowGraphPlacer is ELK 0.8.1's SimpleRowGraphPlacer. It is used when
// none of the components contains an external-port dummy.
type simpleRowGraphPlacer struct{}

func (*simpleRowGraphPlacer) combine(
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

	if componentOrdering(&target.Properties) == componentOrderingNone {
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
