// Copyright (c) 2009, 2020 Kiel University and others.
// Copyright (c) 2015, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package box implements org.eclipse.elk.box from ELK 0.8.1.
package box

import (
	"math"
	"sort"

	"github.com/d2lang/elk-go/internal/basicoptions"
	"github.com/d2lang/elk-go/internal/common/elkutil"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
)

const defaultAspectRatio = 1.3

type Provider struct{}

func Register(registry *core.AlgorithmRegistry) error {
	return registry.Register(core.AlgorithmData{
		ID:     "org.eclipse.elk.box",
		Create: func() core.LayoutProvider { return &Provider{} },
		KnownOptions: []string{
			basicoptions.BoxPadding.ID,
			basicoptions.BoxSpacingNodeNode.ID,
			basicoptions.Priority.ID,
			basicoptions.ExpandNodes.ID,
			core.NodeSizeConstraints.ID,
			basicoptions.NodeSizeOptions.ID,
			basicoptions.BoxAspectRatio.ID,
			basicoptions.Interactive.ID,
			core.NodeSizeMinimum.ID,
			basicoptions.PackingMode.ID,
			core.ContentAlignment.ID,
		},
	})
}

func (*Provider) Layout(model *graph.Model, parent graph.NodeID, _ core.ProgressMonitor) error {
	node := model.Node(parent)
	// BoxLayoutProvider narrows spacing through floatValue().
	spacing := float64(float32(graph.Get(&node.Properties, basicoptions.BoxSpacingNodeNode)))
	padding := graph.Get(&node.Properties, basicoptions.BoxPadding)
	expand := graph.Get(&node.Properties, basicoptions.ExpandNodes)
	interactive := graph.Get(&node.Properties, basicoptions.Interactive)
	if graph.Get(&node.Properties, basicoptions.PackingMode) == "SIMPLE" {
		placeBoxesSimple(model, parent, spacing, padding, expand, interactive)
	} else {
		placeBoxesGrouping(model, parent, float32(spacing), padding, expand)
	}
	return nil
}

func placeBoxesSimple(model *graph.Model, parent graph.NodeID, spacing float64, padding geometry.Spacing, expand, interactive bool) {
	boxes := sortBoxes(model, parent, interactive)
	minimum := elkutil.EffectiveMinSizeConstraintFor(model, parent)
	aspect := graph.Get(&model.Node(parent).Properties, basicoptions.BoxAspectRatio)
	if aspect <= 0 {
		aspect = defaultAspectRatio
	}
	size := placeBoxes(model, boxes, spacing, padding, minimum.X, minimum.Y, expand, aspect)
	elkutil.ResizeNode(model, parent, size.X, size.Y, false, true)
}

func sortBoxes(model *graph.Model, parent graph.NodeID, interactive bool) []graph.NodeID {
	boxes := append([]graph.NodeID(nil), model.Node(parent).Children...)
	sort.SliceStable(boxes, func(i, j int) bool {
		first, second := model.Node(boxes[i]), model.Node(boxes[j])
		priorityFirst := graph.Get(&first.Properties, basicoptions.Priority)
		prioritySecond := graph.Get(&second.Properties, basicoptions.Priority)
		if priorityFirst != prioritySecond {
			return priorityFirst > prioritySecond
		}
		if interactive {
			if comparison := javaDoubleCompare(first.Shape.Y, second.Shape.Y); comparison != 0 {
				return comparison < 0
			}
			if comparison := javaDoubleCompare(first.Shape.X, second.Shape.X); comparison != 0 {
				return comparison < 0
			}
		}
		firstArea := first.Shape.Width * first.Shape.Height
		secondArea := second.Shape.Width * second.Shape.Height
		return javaDoubleCompare(firstArea, secondArea) < 0
	})
	return boxes
}

func placeBoxes(
	model *graph.Model,
	boxes []graph.NodeID,
	spacing float64,
	padding geometry.Spacing,
	minimumWidth, minimumHeight float64,
	expand bool,
	aspect float64,
) geometry.Point {
	maxRowWidth, totalArea := 0.0, 0.0
	for _, nodeID := range boxes {
		elkutil.ResizeNodeForConstraints(model, nodeID)
		node := model.Node(nodeID)
		maxRowWidth = math.Max(maxRowWidth, node.Shape.Width)
		totalArea += node.Shape.Width * node.Shape.Height
	}
	mean := totalArea / float64(len(boxes))
	standardDeviation := areaStdDev(model, boxes, mean)
	totalArea += float64(len(boxes)) * standardDeviation
	maxRowWidth = math.Max(maxRowWidth, math.Sqrt(totalArea*aspect)) + padding.Left

	x, y := padding.Left, padding.Top
	highest, broadest := 0.0, padding.Horizontal()
	rowIndices := []int{0}
	rowHeights := make([]float64, 0)
	for index, nodeID := range boxes {
		node := model.Node(nodeID)
		width, height := node.Shape.Width, node.Shape.Height
		if x+width > maxRowWidth {
			if expand {
				rowHeights = append(rowHeights, highest)
				rowIndices = append(rowIndices, index)
			}
			x = padding.Left
			y += highest + spacing
			highest = 0
			broadest = math.Max(broadest, padding.Horizontal()+width)
		}
		node.Shape.SetLocation(x, y)
		broadest = math.Max(broadest, x+width+padding.Right)
		highest = math.Max(highest, height)
		x += width + spacing
	}
	broadest = math.Max(broadest, minimumWidth)
	totalHeight := y + highest + padding.Bottom
	if totalHeight < minimumHeight {
		highest += minimumHeight - totalHeight
		totalHeight = minimumHeight
	}

	if expand {
		x = padding.Left
		rowIndices = append(rowIndices, len(boxes))
		nextRowPosition := 0
		nextRowIndex := rowIndices[nextRowPosition]
		rowHeights = append(rowHeights, highest)
		rowHeightPosition := 0
		rowHeight := 0.0
		for index, nodeID := range boxes {
			if index == nextRowIndex {
				x = padding.Left
				rowHeight = rowHeights[rowHeightPosition]
				rowHeightPosition++
				nextRowPosition++
				nextRowIndex = rowIndices[nextRowPosition]
			}
			node := model.Node(nodeID)
			oldSize := geometry.Point{X: node.Shape.Width, Y: node.Shape.Height}
			node.Shape.Height = rowHeight
			newHeight := rowHeight
			if index+1 == nextRowIndex {
				newWidth := broadest - x - padding.Right
				node.Shape.Width = newWidth
				elkutil.TranslateAligned(model, nodeID, geometry.Point{X: newWidth, Y: newHeight}, oldSize)
			}
			node.Shape.Presence |= graph.HasWidth | graph.HasHeight
			x += node.Shape.Width + spacing
		}
	}
	return geometry.Point{X: broadest, Y: totalHeight}
}

func areaStdDev(model *graph.Model, boxes []graph.NodeID, mean float64) float64 {
	variance := 0.0
	for _, nodeID := range boxes {
		node := model.Node(nodeID)
		difference := node.Shape.Width*node.Shape.Height - mean
		variance += math.Pow(difference, 2)
	}
	return math.Sqrt(variance / float64(len(boxes)-1))
}

type group struct {
	node     *graph.NodeID
	children []*group
	size     geometry.Point
	bottom   []*group
	right    []*group
}

func newLeaf(model *graph.Model, nodeID graph.NodeID) *group {
	model.Node(nodeID).Shape.SetLocation(0, 0)
	copyOfID := nodeID
	return &group{node: &copyOfID}
}

func newGroup(children []*group) *group {
	return &group{children: append([]*group(nil), children...)}
}

func (value *group) width(model *graph.Model) float64 {
	if value.node != nil {
		return model.Node(*value.node).Shape.Width
	}
	return value.size.X
}

func (value *group) height(model *graph.Model) float64 {
	if value.node != nil {
		return model.Node(*value.node).Shape.Height
	}
	return value.size.Y
}

func (value *group) area(model *graph.Model) float64 {
	return value.width(model) * value.height(model)
}

func (value *group) setWidth(model *graph.Model, width float64) {
	if value.node != nil {
		node := model.Node(*value.node)
		node.Shape.Width = width
		node.Shape.Presence |= graph.HasWidth
		return
	}
	delta := width - value.width(model)
	for _, right := range value.right {
		right.setWidth(model, right.width(model)+delta)
	}
}

func (value *group) setHeight(model *graph.Model, height float64) {
	if value.node != nil {
		node := model.Node(*value.node)
		node.Shape.Height = height
		node.Shape.Presence |= graph.HasHeight
		return
	}
	delta := height - value.height(model)
	for _, bottom := range value.bottom {
		bottom.setHeight(model, bottom.height(model)+delta)
	}
}

func (value *group) translate(model *graph.Model, x, y float64) {
	if value.node != nil {
		node := model.Node(*value.node)
		node.Shape.X += x
		node.Shape.Y += y
		node.Shape.Presence |= graph.HasX | graph.HasY
		return
	}
	for _, child := range value.children {
		child.translate(model, x, y)
	}
}

func (value *group) translateInnerNodes(model *graph.Model, x, y float64) {
	if value.node != nil {
		elkutil.Translate(model, *value.node, x, y)
		return
	}
	for _, child := range value.children {
		child.translateInnerNodes(model, x, y)
	}
}

func placeBoxesGrouping(model *graph.Model, parent graph.NodeID, spacing float32, padding geometry.Spacing, expand bool) {
	minimum := graph.Get(&model.Node(parent).Properties, core.NodeSizeMinimum)
	minimum.X = math.Max(minimum.X-padding.Left-padding.Right, 0)
	minimum.Y = math.Max(minimum.Y-padding.Top-padding.Bottom, 0)
	aspect := graph.Get(&model.Node(parent).Properties, basicoptions.BoxAspectRatio)
	if aspect <= 0 {
		aspect = defaultAspectRatio
	}
	groups := make([]*group, 0, len(model.Node(parent).Children))
	for _, child := range model.Node(parent).Children {
		groups = append(groups, newLeaf(model, child))
	}
	var placed []*group
	switch graph.Get(&model.Node(parent).Properties, basicoptions.PackingMode) {
	case "GROUP_INC":
		placed = mergeAndPlaceIncreasing(model, groups, float64(spacing), minimum.X, minimum.Y, expand)
	case "GROUP_DEC":
		placed = mergeAndPlaceDecreasing(model, groups, float64(spacing), minimum.X, minimum.Y, expand)
	default:
		placed = mergeAndPlaceMixed(model, groups, float64(spacing), minimum.X, minimum.Y, expand)
	}
	outer := newGroup(placed)
	size := placeInnerBoxes(model, outer, float64(spacing), padding, minimum.X, minimum.Y, expand, aspect)
	elkutil.ResizeNode(model, parent, size.X, size.Y, false, true)
}

func placeInnerBoxes(
	model *graph.Model,
	container *group,
	spacing float64,
	padding geometry.Spacing,
	minimumWidth, minimumHeight float64,
	expand bool,
	aspect float64,
) geometry.Point {
	maxRowWidth, totalArea := 0.0, 0.0
	for _, value := range container.children {
		if value.node != nil {
			elkutil.ResizeNodeForConstraints(model, *value.node)
		}
		maxRowWidth = math.Max(maxRowWidth, value.width(model))
		totalArea += value.area(model)
	}
	mean := totalArea / float64(len(container.children))
	variance := 0.0
	for _, value := range container.children {
		difference := value.area(model) - mean
		variance += math.Pow(difference, 2)
	}
	standardDeviation := math.Sqrt(variance / float64(len(container.children)-1))
	totalArea += float64(len(container.children)) * standardDeviation
	maxRowWidth = math.Max(maxRowWidth, math.Sqrt(totalArea*aspect)) + padding.Left

	x, y := padding.Left, padding.Top
	highest, broadest := 0.0, padding.Horizontal()
	rowIndices := []int{0}
	rowHeights := make([]float64, 0)
	var last *group
	bottoms := make([]*group, 0)
	for index, value := range container.children {
		width, height := value.width(model), value.height(model)
		if x+width > maxRowWidth {
			if expand {
				rowHeights = append(rowHeights, highest)
				rowIndices = append(rowIndices, index)
				container.right = append(container.right, last)
				bottoms = bottoms[:0]
			}
			x = padding.Left
			y += highest + spacing
			highest = 0
			broadest = math.Max(broadest, padding.Horizontal()+width)
		}
		bottoms = append(bottoms, value)
		value.translate(model, x, y)
		broadest = math.Max(broadest, x+width+padding.Right)
		highest = math.Max(highest, height)
		x += width + spacing
		last = value
	}
	container.bottom = append(container.bottom, bottoms...)
	container.right = append(container.right, bottoms[len(bottoms)-1])
	broadest = math.Max(broadest, minimumWidth)
	totalHeight := y + highest + padding.Bottom
	if totalHeight < minimumHeight {
		highest += minimumHeight - totalHeight
		totalHeight = minimumHeight
	}

	if expand {
		x = padding.Left
		rowIndices = append(rowIndices, len(container.children))
		nextRowPosition := 0
		nextRowIndex := rowIndices[nextRowPosition]
		rowHeights = append(rowHeights, highest)
		rowHeightPosition := 0
		rowHeight := 0.0
		for index, value := range container.children {
			if index == nextRowIndex {
				x = padding.Left
				rowHeight = rowHeights[rowHeightPosition]
				rowHeightPosition++
				nextRowPosition++
				nextRowIndex = rowIndices[nextRowPosition]
			}
			value.setHeight(model, rowHeight)
			if index+1 == nextRowIndex {
				newWidth := broadest - x - padding.Right
				oldWidth := value.width(model)
				value.setWidth(model, newWidth)
				value.translateInnerNodes(model, (newWidth-oldWidth)/2, 0)
			}
			x += value.width(model) + spacing
		}
	}
	return geometry.Point{X: broadest, Y: totalHeight}
}

func mergeAndPlaceDecreasing(model *graph.Model, groups []*group, spacing, minimumWidth, minimumHeight float64, expand bool) []*group {
	sort.SliceStable(groups, func(i, j int) bool {
		return javaDoubleCompare(groups[i].area(model), groups[j].area(model)) > 0
	})
	queue := append([]*group(nil), groups...)
	placed := make([]*group, 0)
	maybe := make([]*group, 0)
	var boxToBeat *group
	collected := 0.0
	for len(queue) > 0 {
		value := queue[0]
		queue = queue[1:]
		if boxToBeat == nil || boxToBeat.area(model)/2 < value.area(model) {
			boxToBeat = value
			placed = append(placed, value)
		} else {
			collected += value.area(model)
			maybe = append(maybe, value)
			if len(maybe) > 1 && (collected > boxToBeat.area(model)/2 || len(queue) == 0) {
				inner := newGroup(maybe)
				innerAspect := boxToBeat.width(model) / boxToBeat.height(model)
				inner.size = placeInnerBoxes(model, inner, spacing, geometry.Spacing{}, minimumWidth, minimumHeight, expand, innerAspect)
				boxToBeat = inner
				placed = append(placed, inner)
				collected = 0
				maybe = maybe[:0]
			}
		}
	}
	return append(placed, maybe...)
}

func mergeAndPlaceMixed(model *graph.Model, groups []*group, spacing, minimumWidth, minimumHeight float64, expand bool) []*group {
	cumulative := make([]float64, len(groups))
	queue := javacompat.NewGWTPriorityQueue(func(first, second *group) int {
		return javaDoubleCompare(first.area(model), second.area(model))
	})
	queue.AddAll(groups)
	index := 0
	placed := make([]*group, 0)
	for !queue.IsEmpty() {
		value, _ := queue.Peek()
		if index > 1 && value.area(model)/2 > cumulative[0] {
			selectionIndex := 0
			for selectionIndex < len(placed)-1 && value.area(model)/2 > cumulative[selectionIndex] {
				selectionIndex++
			}
			selected := append([]*group(nil), placed[:selectionIndex+1]...)
			remaining := append([]*group(nil), placed[selectionIndex+1:]...)
			inner := newGroup(selected)
			innerAspect := value.width(model) / value.height(model)
			inner.size = placeInnerBoxes(model, inner, spacing, geometry.Spacing{}, minimumWidth, minimumHeight, expand, innerAspect)
			queue.Add(inner)
			queue.AddAll(remaining)
			placed = placed[:0]
			index = 0
			clear(cumulative)
		} else {
			value, _ = queue.Poll()
			if index > 0 {
				cumulative[index] = cumulative[index-1]
			}
			cumulative[index] += value.area(model)
			index++
			placed = append(placed, value)
		}
	}
	return placed
}

func mergeAndPlaceIncreasing(model *graph.Model, groups []*group, spacing, minimumWidth, minimumHeight float64, expand bool) []*group {
	sort.SliceStable(groups, func(i, j int) bool {
		return javaDoubleCompare(groups[i].area(model), groups[j].area(model)) < 0
	})
	placed := make([]*group, 0)
	commonArea := 0.0
	for _, value := range groups {
		if len(placed) > 0 && value.area(model) > commonArea*2 {
			merged := newGroup(placed)
			innerAspect := value.width(model) / value.height(model)
			merged.size = placeInnerBoxes(model, merged, spacing, geometry.Spacing{}, minimumWidth, minimumHeight, expand, innerAspect)
			placed = placed[:0]
			commonArea = 0
			placed = append(placed, merged, value)
			commonArea = merged.area(model) + value.area(model)
		} else {
			placed = append(placed, value)
			commonArea += value.area(model)
		}
	}
	return placed
}

func javaDoubleCompare(first, second float64) int {
	if first < second {
		return -1
	}
	if first > second {
		return 1
	}
	firstBits := int64(math.Float64bits(first))
	secondBits := int64(math.Float64bits(second))
	if math.IsNaN(first) {
		firstBits = int64(0x7ff8000000000000)
	}
	if math.IsNaN(second) {
		secondBits = int64(0x7ff8000000000000)
	}
	if firstBits == secondBits {
		return 0
	}
	if firstBits < secondBits {
		return -1
	}
	return 1
}
