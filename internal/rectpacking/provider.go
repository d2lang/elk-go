// Copyright (c) 2018, 2020 Kiel University and others.
// Copyright (c) 2020 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package rectpacking ports org.eclipse.elk.alg.rectpacking from ELK 0.8.1.
package rectpacking

import (
	"math"
	"sort"

	"github.com/d2lang/elk-go/internal/common/elkutil"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

type Provider struct{}

func Register(registry *core.AlgorithmRegistry) error {
	return registry.Register(core.AlgorithmData{
		ID:     "org.eclipse.elk.rectpacking",
		Create: func() core.LayoutProvider { return &Provider{} },
		KnownOptions: []string{
			AspectRatio.ID, ExpandNodes.ID, Padding.ID, SpacingNodeNode.ID,
			OptimizationGoal.ID, LastPlaceShift.ID, OnlyFirstIteration.ID,
			RowCompaction.ID, ExpandToAspect.ID, Interactive.ID,
			DesiredPosition.ID, CurrentPosition.ID, TargetWidth.ID,
		},
	})
}

func (*Provider) Layout(model *graph.Model, parent graph.NodeID, _ core.ProgressMonitor) error {
	parentNode := model.Node(parent)
	rectangles := append([]graph.NodeID(nil), parentNode.Children...)
	for _, id := range rectangles {
		model.Node(id).Shape.SetLocation(0, 0)
	}

	aspectRatio := graph.Get(&parentNode.Properties, AspectRatio)
	goal := graph.Get(&parentNode.Properties, OptimizationGoal)
	lastPlaceShift := graph.Get(&parentNode.Properties, LastPlaceShift)
	onlyFirst := graph.Get(&parentNode.Properties, OnlyFirstIteration)
	expandNodes := graph.Get(&parentNode.Properties, ExpandNodes)
	padding := graph.Get(&parentNode.Properties, Padding)
	spacing := graph.Get(&parentNode.Properties, SpacingNodeNode)
	compact := graph.Get(&parentNode.Properties, RowCompaction)
	expandToAspect := graph.Get(&parentNode.Properties, ExpandToAspect)
	interactive := graph.Get(&parentNode.Properties, Interactive)
	targetWidth := graph.Get(&parentNode.Properties, TargetWidth)

	if interactive {
		rectangles = interactiveOrder(model, rectangles)
		// ELK mutates the parent's child EList in interactive mode.
		parentNode.Children = append(parentNode.Children[:0], rectangles...)
		for index, id := range rectangles {
			graph.Set(&model.Node(id).Properties, CurrentPosition, index)
		}
	}

	minimum := elkutil.EffectiveMinSizeConstraintFor(model, parent)
	minimum.X -= padding.Horizontal()
	minimum.Y -= padding.Vertical()
	maxWidth := minimum.X
	drawing := drawingData{dar: aspectRatio, width: targetWidth, option: wholeDrawing}
	if targetWidth < 0 || targetWidth < minimum.X {
		drawing = approximate(model, rectangles, spacing, padding, aspectRatio, goal, lastPlaceShift)
	}
	minimum.X += padding.Horizontal()
	minimum.Y += padding.Vertical()
	if !onlyFirst {
		for _, id := range rectangles {
			model.Node(id).Shape.SetLocation(0, 0)
		}
		maxWidth = math.Max(minimum.X, drawing.width)
		drawing = placeAndCompact(model, rectangles, maxWidth, minimum, padding, spacing, aspectRatio, expandNodes, expandToAspect, compact)
	}

	for _, id := range rectangles {
		node := model.Node(id)
		node.Shape.SetLocation(node.Shape.X+padding.Left, node.Shape.Y+padding.Top)
	}
	elkutil.ResizeNode(model, parent, drawing.width+padding.Horizontal(), drawing.height+padding.Vertical(), false, true)
	return nil
}

func interactiveOrder(model *graph.Model, rectangles []graph.NodeID) []graph.NodeID {
	result := make([]graph.NodeID, 0, len(rectangles))
	fixed := make([]graph.NodeID, 0)
	for _, id := range rectangles {
		if graph.Has(&model.Node(id).Properties, DesiredPosition) {
			fixed = append(fixed, id)
		} else {
			result = append(result, id)
		}
	}
	sort.SliceStable(fixed, func(i, j int) bool {
		a := graph.Get(&model.Node(fixed[i]).Properties, DesiredPosition)
		b := graph.Get(&model.Node(fixed[j]).Properties, DesiredPosition)
		if a == b {
			// The Java comparator returns -1 here. Stable insertion in reverse
			// order is its deterministic GWT result.
			return true
		}
		return a < b
	})
	for _, id := range fixed {
		position := graph.Get(&model.Node(id).Properties, DesiredPosition)
		if position < 0 {
			position = 0
		}
		if position > len(result) {
			position = len(result)
		}
		result = append(result, 0)
		copy(result[position+1:], result[position:])
		result[position] = id
	}
	return result
}

type placement uint8

const (
	wholeDrawing placement = iota
	lastRight
	lastBelow
	wholeRight
	wholeBelow
)

type drawingData struct {
	dar, width, height, area, aspect, scale float64
	option                                  placement
	nextX, nextY                            float64
}

func newDrawing(dar, width, height float64, option placement, x, y float64) drawingData {
	result := drawingData{dar: dar, width: width, height: height, option: option, nextX: x, nextY: y}
	if width > 0 && height > 0 {
		result.area = width * height
		result.aspect = width / height
		result.scale = math.Min(dar/width, 1/height)
	}
	return result
}

func approximate(model *graph.Model, rectangles []graph.NodeID, spacing float64, padding geometry.Spacing, dar float64, goal string, shift bool) drawingData {
	first := model.Node(rectangles[0])
	first.Shape.SetLocation(0, 0)
	placed := []graph.NodeID{rectangles[0]}
	last := rectangles[0]
	current := newDrawing(dar, first.Shape.Width, first.Shape.Height, wholeDrawing, 0, 0)
	for _, id := range rectangles[1:] {
		candidates := []drawingData{
			candidate(model, id, last, current, placed, spacing, shift, lastRight),
			candidate(model, id, last, current, placed, spacing, shift, lastBelow),
			candidate(model, id, last, current, placed, spacing, shift, wholeRight),
			candidate(model, id, last, current, placed, spacing, shift, wholeBelow),
		}
		best := bestCandidate(model, candidates, id, last, padding, goal)
		model.Node(id).Shape.SetLocation(best.nextX, best.nextY)
		best.option = wholeDrawing
		current = best
		last = id
		placed = append(placed, id)
	}
	return current
}

func candidate(model *graph.Model, id, last graph.NodeID, drawing drawingData, placed []graph.NodeID, spacing float64, shift bool, option placement) drawingData {
	node, previous := model.Node(id), model.Node(last)
	x, y := 0.0, 0.0
	width, height := drawing.width, drawing.height
	switch option {
	case lastRight:
		x = previous.Shape.X + previous.Shape.Width + spacing
		y = previous.Shape.Y
		if shift {
			y = shiftedY(model, x, placed, previous, spacing)
		}
		width = math.Max(width, x+node.Shape.Width)
		height = math.Max(height, y+node.Shape.Height)
	case lastBelow:
		y = previous.Shape.Y + previous.Shape.Height + spacing
		x = previous.Shape.X
		if shift {
			x = shiftedX(model, y, placed, previous, spacing)
		}
		width = math.Max(width, x+node.Shape.Width)
		height = math.Max(height, y+node.Shape.Height)
	case wholeRight:
		x = drawing.width + spacing
		width = drawing.width + spacing + node.Shape.Width
		height = math.Max(drawing.height, node.Shape.Height)
	case wholeBelow:
		y = drawing.height + spacing
		width = math.Max(drawing.width, node.Shape.Width)
		height = drawing.height + spacing + node.Shape.Height
	}
	return newDrawing(drawing.dar, width, height, option, x, y)
}

func shiftedY(model *graph.Model, x float64, placed []graph.NodeID, last *graph.Node, spacing float64) float64 {
	found := false
	closestBottom := 0.0
	for _, id := range placed {
		node := model.Node(id)
		bottom := node.Shape.Y + node.Shape.Height
		if x < node.Shape.X+node.Shape.Width+spacing {
			if !found || last.Shape.Y-bottom < last.Shape.Y-closestBottom {
				closestBottom = bottom
				found = true
			}
		}
	}
	if !found {
		return 0
	}
	return closestBottom + spacing
}

func shiftedX(model *graph.Model, y float64, placed []graph.NodeID, last *graph.Node, spacing float64) float64 {
	found := false
	closestRight := 0.0
	for _, id := range placed {
		node := model.Node(id)
		right := node.Shape.X + node.Shape.Width
		if y < node.Shape.Y+node.Shape.Height+spacing {
			if !found || last.Shape.X-right < last.Shape.X-closestRight {
				closestRight = right
				found = true
			}
		}
	}
	if !found {
		return 0
	}
	return closestRight + spacing
}

func bestCandidate(model *graph.Model, candidates []drawingData, id, last graph.NodeID, padding geometry.Spacing, goal string) drawingData {
	filters := []func([]drawingData) []drawingData{}
	scale := func(values []drawingData) []drawingData {
		best := math.Inf(-1)
		for _, value := range values {
			best = math.Max(best, math.Min(value.dar/(value.width+padding.Horizontal()), 1/(value.height+padding.Vertical())))
		}
		result := values[:0]
		for _, value := range values {
			if math.Min(value.dar/(value.width+padding.Horizontal()), 1/(value.height+padding.Vertical())) == best {
				result = append(result, value)
			}
		}
		return result
	}
	area := func(values []drawingData) []drawingData {
		best := math.Inf(1)
		for _, value := range values {
			best = math.Min(best, (value.width+padding.Horizontal())*(value.height+padding.Vertical()))
		}
		result := values[:0]
		for _, value := range values {
			if (value.width+padding.Horizontal())*(value.height+padding.Vertical()) == best {
				result = append(result, value)
			}
		}
		return result
	}
	aspect := func(values []drawingData) []drawingData {
		best := math.Inf(1)
		for _, value := range values {
			best = math.Min(best, math.Abs((value.width+padding.Horizontal())/(value.height+padding.Vertical())-value.dar))
		}
		result := values[:0]
		for _, value := range values {
			if math.Abs((value.width+padding.Horizontal())/(value.height+padding.Vertical())-value.dar) == best {
				result = append(result, value)
			}
		}
		return result
	}
	switch goal {
	case "ASPECT_RATIO_DRIVEN":
		filters = append(filters, aspect, area, scale)
	case "AREA_DRIVEN":
		filters = append(filters, area, scale, aspect)
	default:
		filters = append(filters, scale, area, aspect)
	}
	for _, filter := range filters {
		if len(candidates) > 1 {
			candidates = filter(candidates)
		}
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	if len(candidates) != 2 {
		return candidates[0]
	}
	a, b := candidates[0], candidates[1]
	aBelow := a.option == lastBelow || a.option == wholeBelow
	bBelow := b.option == lastBelow || b.option == wholeBelow
	aRight := a.option == lastRight || a.option == wholeRight
	bRight := b.option == lastRight || b.option == wholeRight
	if aBelow && bBelow {
		if a.option == wholeBelow {
			return a
		}
		return b
	}
	if aRight && bRight {
		if a.option == wholeRight {
			return a
		}
		return b
	}
	if (a.option == lastRight || a.option == lastBelow) && (b.option == lastRight || b.option == lastBelow) {
		right, below := a, b
		if b.option == lastRight {
			right, below = b, a
		}
		previous, node := model.Node(last), model.Node(id)
		rightHeight := math.Max(previous.Shape.Y+previous.Shape.Height, right.nextY+node.Shape.Height) - math.Min(previous.Shape.Y, right.nextY)
		rightArea := (right.nextX + node.Shape.Width - previous.Shape.X) * rightHeight
		belowWidth := math.Max(previous.Shape.X+previous.Shape.Width, below.nextX+node.Shape.Width) - math.Min(previous.Shape.X, below.nextX)
		belowArea := belowWidth * (below.nextY + node.Shape.Height - previous.Shape.Y)
		if rightArea <= belowArea {
			return right
		}
		return below
	}
	return a
}
