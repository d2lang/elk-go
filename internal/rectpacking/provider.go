// Copyright (c) 2018, 2020 Kiel University and others.
// Copyright (c) 2020 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package rectpacking ports org.eclipse.elk.alg.rectpacking from ELK 0.12.0.
package rectpacking

import (
	"fmt"
	"math"
	"sort"

	"github.com/d2lang/elk-go/internal/basicoptions"
	"github.com/d2lang/elk-go/internal/box"
	"github.com/d2lang/elk-go/internal/common/elkutil"
	"github.com/d2lang/elk-go/internal/common/nodespacing"
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
			AspectRatio.ID,
			basicoptions.NodeSizeFixedGraphSize.ID,
			Padding.ID,
			SpacingNodeNode.ID,
			core.ContentAlignment.ID,
			core.NodeSizeConstraints.ID,
			core.NodeSizeMinimum.ID,
			basicoptions.NodeSizeOptions.ID,
			"org.eclipse.elk.nodeLabels.placement",
			OmitNodeMicroLayout.ID,
			"org.eclipse.elk.portLabels.placement",
			OptimizationGoal.ID,
			LastPlaceShift.ID,
			TargetWidth.ID,
			WidthApproximationStrategy.ID,
			PackingStrategy.ID,
			RowHeightReevaluation.ID,
			CompactionIterations.ID,
			WhiteSpaceEliminationStrategy.ID,
			Interactive.ID,
			core.InteractiveLayout.ID,
			DesiredPosition.ID,
			CurrentPosition.ID,
			InNewRow.ID,
			TryBox.ID,
			OrderBySize.ID,
		},
	})
}

func (*Provider) Layout(model *graph.Model, parent graph.NodeID, monitor core.ProgressMonitor) error {
	parentNode := model.Node(parent)
	rectangles := append([]graph.NodeID(nil), parentNode.Children...)
	padding := graph.Get(&parentNode.Properties, Padding)
	spacing := graph.Get(&parentNode.Properties, SpacingNodeNode)

	// RectPackingLayoutProvider runs node micro layout both before packing and
	// after optional whitespace expansion, since expansion can change child
	// dimensions. The Go node-spacing utility is the matching common phase.
	if !graph.Get(&parentNode.Properties, OmitNodeMicroLayout) {
		nodespacing.CalculateLabelAndNodeSizes(model, parent, nil)
	}

	// ELK 0.12's trybox special case deliberately falls back to Box only when
	// no consecutive triple is vertically stackable.
	stackable := false
	if graph.Get(&parentNode.Properties, TryBox) && len(rectangles) >= 3 {
		first, second := rectangles[0], rectangles[1]
		for index := 2; index < len(rectangles); index++ {
			third := rectangles[index]
			firstHeight := model.Node(first).Shape.Height
			secondHeight := model.Node(second).Shape.Height
			thirdHeight := model.Node(third).Shape.Height
			if firstHeight >= secondHeight+thirdHeight+spacing ||
				thirdHeight >= firstHeight+secondHeight+spacing {
				stackable = true
				break
			}
			first, second = second, third
		}
	} else {
		stackable = true
	}
	if !stackable {
		priority := len(rectangles)
		for _, id := range rectangles {
			graph.Set(&model.Node(id).Properties, basicoptions.Priority, priority)
			priority--
		}
		return (&box.Provider{}).Layout(model, parent, monitor)
	}

	// ELK's phase-independent processors run in this order before width
	// approximation: minimum-size preprocessing, size ordering, interaction.
	minimum := elkutil.EffectiveMinSizeConstraintFor(model, parent)
	if graph.Get(&parentNode.Properties, OrderBySize) {
		sort.SliceStable(rectangles, func(i, j int) bool {
			return model.Node(rectangles[i]).Shape.Height > model.Node(rectangles[j]).Shape.Height
		})
		parentNode.Children = append(parentNode.Children[:0], rectangles...)
	}

	if graph.Get(&parentNode.Properties, Interactive) {
		var err error
		rectangles, err = interactiveOrder(model, rectangles)
		if err != nil {
			return err
		}
		// ELK mutates the parent's child EList in interactive mode.
		parentNode.Children = append(parentNode.Children[:0], rectangles...)
		for index, id := range rectangles {
			graph.Set(&model.Node(id).Properties, CurrentPosition, index)
		}
	}

	aspectRatio := graph.Get(&parentNode.Properties, AspectRatio)
	var targetWidth float64
	switch graph.Get(&parentNode.Properties, WidthApproximationStrategy) {
	case "TARGET_WIDTH":
		if !graph.Has(&parentNode.Properties, TargetWidth) {
			return fmt.Errorf("rectpacking TARGET_WIDTH requires %s", TargetWidth.ID)
		}
		targetWidth = graph.Get(&parentNode.Properties, TargetWidth)
	case "GREEDY":
		if len(rectangles) == 0 {
			targetWidth = 0
		} else {
			drawing := approximate(
				model, rectangles, spacing, padding, aspectRatio,
				graph.Get(&parentNode.Properties, OptimizationGoal),
				graph.Get(&parentNode.Properties, LastPlaceShift),
			)
			targetWidth = drawing.width
		}
	default:
		return fmt.Errorf("unsupported rectpacking width approximation strategy %q",
			graph.Get(&parentNode.Properties, WidthApproximationStrategy))
	}
	// MinSizePostProcessor clamps the internal target width to the unadjusted
	// minimum width. Padding is removed later when P2 calculates dimensions.
	targetWidth = math.Max(targetWidth, minimum.X)

	packingStrategy := graph.Get(&parentNode.Properties, PackingStrategy)
	var drawing drawingData
	var rows []*rectRow
	switch packingStrategy {
	case "COMPACTION":
		drawing, rows = compactPlacement(
			model, rectangles, targetWidth, minimum, padding, spacing, aspectRatio,
			graph.Get(&parentNode.Properties, RowHeightReevaluation),
			graph.Get(&parentNode.Properties, CompactionIterations),
		)
	case "SIMPLE":
		drawing, rows = simplePlacement(model, rectangles, targetWidth, minimum, padding, spacing, aspectRatio)
	case "NONE":
		drawing = noPlacement(model, rectangles, minimum, padding, aspectRatio)
	default:
		return fmt.Errorf("unsupported rectpacking packing strategy %q", packingStrategy)
	}

	whiteSpaceStrategy := graph.Get(&parentNode.Properties, WhiteSpaceEliminationStrategy)
	if whiteSpaceStrategy != "NONE" {
		if rows == nil {
			return fmt.Errorf("rectpacking whitespace elimination requires a packing strategy that produces rows")
		}
		if whiteSpaceStrategy == "TO_ASPECT_RATIO" {
			if drawing.width/drawing.height < aspectRatio {
				drawing.width = drawing.height * aspectRatio
			} else {
				drawing.additionalHeight += drawing.width/aspectRatio - drawing.height
				drawing.height += drawing.additionalHeight
			}
		} else if whiteSpaceStrategy != "EQUAL_BETWEEN_STRUCTURES" {
			return fmt.Errorf("unsupported rectpacking whitespace elimination strategy %q", whiteSpaceStrategy)
		}
		expandRows(model, rows, drawing.width, drawing.additionalHeight)
	}

	// Content alignment is applied to the unpadded drawing before every child
	// is shifted by the requested top/left padding.
	realSize := rectangleDimensions(model, rectangles)
	elkutil.TranslateAligned(model, parent,
		geometry.Point{X: drawing.width, Y: drawing.height}, realSize)
	for _, id := range rectangles {
		node := model.Node(id)
		node.Shape.SetLocation(node.Shape.X+padding.Left, node.Shape.Y+padding.Top)
	}
	if !graph.Get(&parentNode.Properties, basicoptions.NodeSizeFixedGraphSize) {
		elkutil.ResizeNode(model, parent, drawing.width+padding.Horizontal(), drawing.height+padding.Vertical(), false, true)
	}
	if !graph.Get(&parentNode.Properties, OmitNodeMicroLayout) {
		nodespacing.CalculateLabelAndNodeSizes(model, parent, nil)
	}
	return nil
}

func interactiveOrder(model *graph.Model, rectangles []graph.NodeID) ([]graph.NodeID, error) {
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
			return nil, fmt.Errorf("rectpacking desired position %d is negative", position)
		}
		if position > len(result) {
			position = len(result)
		}
		result = append(result, 0)
		copy(result[position+1:], result[position:])
		result[position] = id
	}
	return result, nil
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
	additionalHeight                        float64
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
