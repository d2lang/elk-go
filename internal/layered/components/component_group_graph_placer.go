// Copyright (c) 2012, 2015 Kiel University and others.
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

// componentGroupGraphPlacer is ELK 0.8.1's ComponentGroupGraphPlacer. It
// packs disconnected components whose external-port dummies constrain which
// sectors can share a placement group.
type componentGroupGraphPlacer struct {
	componentGroups []*componentGroup
}

func (placer *componentGroupGraphPlacer) combine(
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

	offset := geometry.Point{}
	spacing := basegraph.Get(
		&firstComponent.Properties, layered.SpacingComponentComponent)
	for _, group := range placer.componentGroups {
		groupSize := placer.placeComponents(arena, group, spacing)
		offsetGraphs(arena, group.components(), offset)
		offset = offset.Add(groupSize)
	}
	target.Size = geometry.Point{X: offset.X - spacing, Y: offset.Y - spacing}

	if basegraph.Get(&firstComponent.Properties, connectedComponentsCompaction) &&
		basegraph.Get(&firstComponent.Properties, layered.EdgeRoutingOption) ==
			layered.EdgeRoutingOrthogonal {
		if err := compactExternalComponentGroups(
			arena, components, targetID, spacing); err != nil {
			return err
		}
	}
	for _, group := range placer.componentGroups {
		moveGraphs(arena, targetID, group.components(), geometry.Point{})
	}
	return nil
}

func (placer *componentGroupGraphPlacer) addComponent(
	arena *layered.LGraphArena,
	component layered.LGraphID,
) {
	for _, group := range placer.componentGroups {
		if group.add(arena, component) {
			return
		}
	}
	placer.componentGroups = append(
		placer.componentGroups, newComponentGroup(arena, component))
}

func (placer *componentGroupGraphPlacer) placeComponents(
	arena *layered.LGraphArena,
	group *componentGroup,
	spacing float64,
) geometry.Point {
	sizeC := placer.placeComponentsInRows(arena, group.componentsFor(sidesNone), spacing)
	sizeN := placer.placeComponentsHorizontally(arena, group.componentsFor(sidesNorth), spacing)
	sizeS := placer.placeComponentsHorizontally(arena, group.componentsFor(sidesSouth), spacing)
	sizeW := placer.placeComponentsVertically(arena, group.componentsFor(sidesWest), spacing)
	sizeE := placer.placeComponentsVertically(arena, group.componentsFor(sidesEast), spacing)
	sizeNW := placer.placeComponentsHorizontally(arena, group.componentsFor(sidesNorthWest), spacing)
	sizeNE := placer.placeComponentsHorizontally(arena, group.componentsFor(sidesNorthEast), spacing)
	sizeSW := placer.placeComponentsHorizontally(arena, group.componentsFor(sidesSouthWest), spacing)
	sizeSE := placer.placeComponentsHorizontally(arena, group.componentsFor(sidesEastSouth), spacing)
	sizeWE := placer.placeComponentsVertically(arena, group.componentsFor(sidesEastWest), spacing)
	sizeNS := placer.placeComponentsHorizontally(arena, group.componentsFor(sidesNorthSouth), spacing)
	sizeNWE := placer.placeComponentsHorizontally(arena, group.componentsFor(sidesNorthEastWest), spacing)
	sizeSWE := placer.placeComponentsHorizontally(arena, group.componentsFor(sidesEastSouthWest), spacing)
	sizeWNS := placer.placeComponentsVertically(arena, group.componentsFor(sidesNorthSouthWest), spacing)
	sizeENS := placer.placeComponentsVertically(arena, group.componentsFor(sidesNorthEastSouth), spacing)
	sizeNESW := placer.placeComponentsHorizontally(arena, group.componentsFor(sidesNorthEastSouthWest), spacing)

	colLeftWidth := max4(sizeNW.X, sizeW.X, sizeSW.X, sizeWNS.X)
	colMidWidth := max4(sizeN.X, sizeC.X, sizeS.X, sizeNESW.X)
	colNSWidth := sizeNS.X
	colRightWidth := max4(sizeNE.X, sizeE.X, sizeSE.X, sizeENS.X)
	rowTopHeight := max4(sizeNW.Y, sizeN.Y, sizeNE.Y, sizeNWE.Y)
	rowMidHeight := max4(sizeW.Y, sizeC.Y, sizeE.Y, sizeNESW.Y)
	rowWEHeight := sizeWE.Y
	rowBottomHeight := max4(sizeSW.Y, sizeS.Y, sizeSE.Y, sizeSWE.Y)

	offsetGraphs(arena, group.componentsFor(sidesNone), geometry.Point{
		X: colLeftWidth + colNSWidth,
		Y: rowTopHeight + rowWEHeight,
	})
	offsetGraphs(arena, group.componentsFor(sidesNorthEastSouthWest), geometry.Point{
		X: colLeftWidth + colNSWidth,
		Y: rowTopHeight + rowWEHeight,
	})
	offsetGraphs(arena, group.componentsFor(sidesNorth), geometry.Point{
		X: colLeftWidth + colNSWidth,
	})
	offsetGraphs(arena, group.componentsFor(sidesSouth), geometry.Point{
		X: colLeftWidth + colNSWidth,
		Y: rowTopHeight + rowWEHeight + rowMidHeight,
	})
	offsetGraphs(arena, group.componentsFor(sidesWest), geometry.Point{
		Y: rowTopHeight + rowWEHeight,
	})
	offsetGraphs(arena, group.componentsFor(sidesEast), geometry.Point{
		X: colLeftWidth + colNSWidth + colMidWidth,
		Y: rowTopHeight + rowWEHeight,
	})
	offsetGraphs(arena, group.componentsFor(sidesNorthEast), geometry.Point{
		X: colLeftWidth + colNSWidth + colMidWidth,
	})
	offsetGraphs(arena, group.componentsFor(sidesSouthWest), geometry.Point{
		Y: rowTopHeight + rowWEHeight + rowMidHeight,
	})
	offsetGraphs(arena, group.componentsFor(sidesEastSouth), geometry.Point{
		X: colLeftWidth + colNSWidth + colMidWidth,
		Y: rowTopHeight + rowWEHeight + rowMidHeight,
	})
	offsetGraphs(arena, group.componentsFor(sidesEastWest), geometry.Point{
		Y: rowTopHeight,
	})
	offsetGraphs(arena, group.componentsFor(sidesNorthSouth), geometry.Point{
		X: colLeftWidth,
	})
	offsetGraphs(arena, group.componentsFor(sidesEastSouthWest), geometry.Point{
		Y: rowTopHeight + rowWEHeight + rowMidHeight,
	})
	offsetGraphs(arena, group.componentsFor(sidesNorthEastSouth), geometry.Point{
		X: colLeftWidth + colNSWidth + colMidWidth,
	})

	return geometry.Point{
		X: max4(colLeftWidth+colMidWidth+colNSWidth+colRightWidth,
			sizeWE.X, sizeNWE.X, sizeSWE.X),
		Y: max4(rowTopHeight+rowMidHeight+rowWEHeight+rowBottomHeight,
			sizeNS.Y, sizeWNS.Y, sizeENS.Y),
	}
}

func (*componentGroupGraphPlacer) placeComponentsHorizontally(
	arena *layered.LGraphArena,
	components []layered.LGraphID,
	spacing float64,
) geometry.Point {
	size := geometry.Point{}
	for _, componentID := range components {
		component := arena.Graph(componentID)
		offsetGraph(arena, componentID, geometry.Point{X: size.X})
		size.X += component.Size.X + spacing
		size.Y = math.Max(size.Y, component.Size.Y)
	}
	if size.Y > 0 {
		size.Y += spacing
	}
	return size
}

func (*componentGroupGraphPlacer) placeComponentsVertically(
	arena *layered.LGraphArena,
	components []layered.LGraphID,
	spacing float64,
) geometry.Point {
	size := geometry.Point{}
	for _, componentID := range components {
		component := arena.Graph(componentID)
		offsetGraph(arena, componentID, geometry.Point{Y: size.Y})
		size.Y += component.Size.Y + spacing
		size.X = math.Max(size.X, component.Size.X)
	}
	if size.X > 0 {
		size.X += spacing
	}
	return size
}

func (*componentGroupGraphPlacer) placeComponentsInRows(
	arena *layered.LGraphArena,
	components []layered.LGraphID,
	spacing float64,
) geometry.Point {
	if len(components) == 0 {
		return geometry.Point{}
	}
	maxRowWidth, totalArea := 0.0, 0.0
	for _, componentID := range components {
		size := arena.Graph(componentID).Size
		maxRowWidth = math.Max(maxRowWidth, size.X)
		totalArea += size.X * size.Y
	}
	// The float cast is in the Java source and survives GWT.
	areaWidth := float64(float32(math.Sqrt(totalArea))) * basegraph.Get(
		&arena.Graph(components[0]).Properties, layered.AspectRatio)
	maxRowWidth = math.Max(maxRowWidth, areaWidth)

	xPosition, yPosition, highestBox, broadestRow := 0.0, 0.0, 0.0, spacing
	for _, componentID := range components {
		size := arena.Graph(componentID).Size
		if xPosition+size.X > maxRowWidth {
			xPosition = 0
			yPosition += highestBox + spacing
			highestBox = 0
		}
		offsetGraph(arena, componentID, geometry.Point{X: xPosition, Y: yPosition})
		broadestRow = math.Max(broadestRow, xPosition+size.X)
		highestBox = math.Max(highestBox, size.Y)
		xPosition += size.X + spacing
	}
	return geometry.Point{
		X: broadestRow + spacing,
		Y: yPosition + highestBox + spacing,
	}
}

func offsetGraphs(
	arena *layered.LGraphArena,
	graphs []layered.LGraphID,
	offset geometry.Point,
) {
	for _, graphID := range graphs {
		offsetGraph(arena, graphID, offset)
	}
}

func max4(first, second, third, fourth float64) float64 {
	return math.Max(math.Max(first, second), math.Max(third, fourth))
}
