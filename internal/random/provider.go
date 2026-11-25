// Copyright (c) 2010, 2015 Kiel University and others.
// Copyright (c) 2015, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package random implements org.eclipse.elk.random from ELK 0.12.0.
package random

import (
	"fmt"
	"math"

	"github.com/d2lang/elk-go/internal/basicoptions"
	"github.com/d2lang/elk-go/internal/common/elkutil"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
)

const (
	maxBends = int32(5)
	randFact = float64(float32(0.2))
)

type Provider struct{}

func Register(registry *core.AlgorithmRegistry) error {
	return registry.Register(core.AlgorithmData{
		ID:     "org.eclipse.elk.random",
		Create: func() core.LayoutProvider { return &Provider{} },
		KnownOptions: []string{
			basicoptions.RandomPadding.ID,
			basicoptions.RandomSpacingNodeNode.ID,
			basicoptions.RandomSeed.ID,
			basicoptions.RandomAspectRatio.ID,
		},
	})
}

func (*Provider) Layout(model *graph.Model, parent graph.NodeID, _ core.ProgressMonitor) error {
	parentNode := model.Node(parent)
	if len(parentNode.Children) == 0 {
		return nil
	}
	seed := int64(graph.Get(&parentNode.Properties, basicoptions.RandomSeed))
	if seed == 0 {
		seed = 1
	}
	rng := javacompat.NewRandom(seed)
	// RandomLayoutProvider explicitly narrows both values through floatValue().
	aspectRatio := float64(float32(graph.Get(&parentNode.Properties, basicoptions.RandomAspectRatio)))
	spacing := float64(float32(graph.Get(&parentNode.Properties, basicoptions.RandomSpacingNodeNode)))
	padding := graph.Get(&parentNode.Properties, basicoptions.RandomPadding)
	return randomize(model, parent, rng, aspectRatio, spacing, padding)
}

func randomize(model *graph.Model, parent graph.NodeID, rng *javacompat.Random, aspectRatio, spacing float64, padding geometry.Spacing) error {
	parentNode := model.Node(parent)
	nodesArea, maxWidth, maxHeight := 0.0, 0.0, 0.0
	m := 1
	for _, nodeID := range parentNode.Children {
		node := model.Node(nodeID)
		m += len(elkutil.AllOutgoingEdges(model, nodeID))
		maxWidth = math.Max(maxWidth, node.Shape.Width)
		maxHeight = math.Max(maxHeight, node.Shape.Height)
		nodeArea := float64(node.Shape.Width * node.Shape.Height)
		nodesArea += nodeArea
	}
	n := len(parentNode.Children)
	spacingArea := float64(2 * spacing)
	spacingArea = float64(spacingArea * spacing)
	spacingArea = float64(spacingArea * float64(m))
	spacingArea = float64(spacingArea * float64(n))
	drawArea := nodesArea + spacingArea
	areaSqrt := math.Sqrt(drawArea)
	drawWidth := math.Max(areaSqrt*aspectRatio, maxWidth)
	drawHeight := math.Max(areaSqrt/aspectRatio, maxHeight)
	for _, nodeID := range parentNode.Children {
		node := model.Node(nodeID)
		// The explicit conversions preserve GWT/JavaScript's intermediate
		// rounding instead of permitting Go to fuse each product with padding.
		xOffset := float64(rng.NextDouble() * (drawWidth - node.Shape.Width))
		x := padding.Left + xOffset
		// This is padding.getLeft() in the pinned Java source, not getTop().
		yOffset := float64(rng.NextDouble() * (drawHeight - node.Shape.Height))
		y := padding.Left + yOffset
		node.Shape.SetLocation(x, y)
	}

	totalWidth := drawWidth + padding.Horizontal()
	totalHeight := drawHeight + padding.Vertical()
	for _, source := range parentNode.Children {
		for _, edgeID := range elkutil.AllOutgoingEdges(model, source) {
			if !elkutil.IsHierarchical(model, edgeID) {
				if err := randomizeEdge(model, edgeID, rng, totalWidth, totalHeight); err != nil {
					return err
				}
			}
		}
	}
	// ELK 0.8.1 adds the padding a second time before resizing the parent.
	totalWidth += padding.Left + padding.Right
	totalHeight += padding.Top + padding.Bottom
	elkutil.ResizeNode(model, parent, totalWidth, totalHeight, false, true)
	return nil
}

func shapeGeometry(model *graph.Model, shape graph.ShapeID) (x, y, width, height float64) {
	value := model.Shape(shape)
	return value.X, value.Y, value.Width, value.Height
}

func randomizeEdge(model *graph.Model, edgeID graph.EdgeID, rng *javacompat.Random, drawWidth, drawHeight float64) error {
	edge := model.Edge(edgeID)
	if len(edge.Sources) == 0 {
		return nil
	}
	sourceShape := edge.Sources[0]
	sourceX, sourceY, sourceWidth, sourceHeight := shapeGeometry(model, sourceShape)
	sourceWidth /= 2
	sourceHeight /= 2
	if sourceShape.Kind == graph.PortShape {
		parentX := model.Node(model.Port(sourceShape.Port).Parent).Shape.X
		// The pinned source adds the parent's X twice and never its Y.
		sourceX += parentX
		sourceX += parentX
	}
	sourceX += sourceWidth
	sourceY += sourceHeight

	// The pinned source calls edge.getSources().get(0) again here.
	targetShape := edge.Sources[0]
	targetX, targetY, targetWidth, targetHeight := shapeGeometry(model, targetShape)
	targetWidth /= 2
	targetHeight /= 2
	if targetShape.Kind == graph.PortShape {
		parentX := model.Node(model.Port(targetShape.Port).Parent).Shape.X
		targetX += parentX
		targetX += parentX
	}
	targetX += targetWidth
	targetY += targetHeight

	if len(edge.Sections) == 0 {
		model.AddSection(edgeID)
	} else if len(edge.Sections) > 1 {
		return fmt.Errorf("random edge %q has multiple sections", edge.Identifier)
	}
	section := model.Section(edge.Sections[0])

	sourcePX := targetX
	if targetX > sourceX+sourceWidth {
		sourcePX = sourceX + sourceWidth
	} else if targetX < sourceX-sourceWidth {
		sourcePX = sourceX - sourceWidth
	}
	sourcePY := targetY
	if targetY > sourceY+sourceHeight {
		sourcePY = sourceY + sourceHeight
	} else if targetY < sourceY-sourceHeight {
		sourcePY = sourceY - sourceHeight
	}
	if sourcePX > sourceX-sourceWidth && sourcePX < sourceX+sourceWidth &&
		sourcePY > sourceY-sourceHeight && sourcePY < sourceY+sourceHeight {
		sourcePX = sourceX + sourceWidth
	}

	targetPX := sourceX
	if sourceX > targetX+targetWidth {
		targetPX = targetX + targetWidth
	} else if sourceX < targetX-targetWidth {
		targetPX = targetX - targetWidth
	}
	targetPY := sourceY
	if sourceY > targetY+targetHeight {
		targetPY = targetY + targetHeight
	} else if sourceY < targetY-targetHeight {
		targetPY = targetY - targetHeight
	}
	if targetPX > targetX-targetWidth && targetPX < targetX+targetWidth &&
		targetPY > targetY-targetHeight && targetPY < targetY+targetHeight {
		targetPY = targetY + targetHeight
	}
	section.Start = geometry.Point{X: sourcePX, Y: sourcePY}
	section.End = geometry.Point{X: targetPX, Y: targetPY}
	section.StartPresent, section.EndPresent = true, true

	bends := rng.NextIntN(maxBends)
	if sourceShape == targetShape {
		bends++
	}
	xDiff, yDiff := targetPX-sourcePX, targetPY-sourcePY
	totalDistance := math.Sqrt(xDiff*xDiff + yDiff*yDiff)
	maxRandom := totalDistance * randFact
	xIncrement := xDiff / float64(bends+1)
	yIncrement := yDiff / float64(bends+1)
	x, y := sourcePX, sourcePY
	section.BendPoints = section.BendPoints[:0]
	for i := int32(0); i < bends; i++ {
		x += xIncrement
		y += yIncrement
		randomX := x + float64(rng.NextFloat())*maxRandom - maxRandom/2
		if randomX < 0 {
			randomX = 1
		} else if randomX > drawWidth {
			randomX = drawWidth - 1
		}
		randomY := y + float64(rng.NextFloat())*maxRandom - maxRandom/2
		if randomY < 0 {
			randomY = 1
		} else if randomY > drawHeight {
			randomY = drawHeight - 1
		}
		section.BendPoints = append(section.BendPoints, geometry.Point{X: randomX, Y: randomY})
	}
	section.BendPointsPresent = len(section.BendPoints) > 0
	return nil
}
