// Copyright (c) 2018, 2020 Kiel University and others.
// Copyright (c) 2020 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package rectpacking

import (
	"math"

	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

type blockRow struct {
	x, y, width, height, spacing float64
	nodes                        []graph.NodeID
}

func newBlockRow(x, y, spacing float64) *blockRow { return &blockRow{x: x, y: y, spacing: spacing} }

func (row *blockRow) add(model *graph.Model, id graph.NodeID) {
	node := model.Node(id)
	node.Shape.SetLocation(row.x+row.width+func() float64 {
		if len(row.nodes) > 0 {
			return row.spacing
		}
		return 0
	}(), row.y)
	row.height = math.Max(row.height, node.Shape.Height)
	if len(row.nodes) > 0 {
		row.width += row.spacing
	}
	row.width += node.Shape.Width
	row.nodes = append(row.nodes, id)
}

func (row *blockRow) remove(model *graph.Model, id graph.NodeID, update bool) {
	for index, candidate := range row.nodes {
		if candidate == id {
			row.nodes = append(row.nodes[:index], row.nodes[index+1:]...)
			break
		}
	}
	if update {
		row.update(model)
	}
}

func (row *blockRow) update(model *graph.Model) {
	width, height := 0.0, 0.0
	for _, id := range row.nodes {
		node := model.Node(id)
		node.Shape.SetLocation(row.x+width, row.y)
		width += node.Shape.Width + row.spacing
		height = math.Max(height, node.Shape.Height+row.spacing)
	}
	row.width = width - row.spacing
	row.height = height - row.spacing
}

func (row *blockRow) expand(model *graph.Model, width, additionalHeight float64, index int) {
	additionalWidth := (width - row.width) / float64(len(row.nodes))
	row.height += additionalHeight
	row.width = width
	for i, id := range row.nodes {
		node := model.Node(id)
		node.Shape.SetLocation(node.Shape.X+float64(i)*additionalWidth, node.Shape.Y+float64(index)*additionalHeight)
		node.Shape.SetDimensions(node.Shape.Width+additionalWidth, row.height)
	}
}

type block struct {
	model                                        *graph.Model
	children                                     []graph.NodeID
	rows                                         []*blockRow
	x, y, width, height                          float64
	minWidth, minHeight, maxHeight               float64
	smallestWidth, smallestHeight, averageHeight float64
	parent                                       *rectRow
	stack                                        *blockStack
	spacing                                      float64
	fixed, positionFixed                         bool
}

func newBlock(model *graph.Model, x, y float64, parent *rectRow, spacing float64) *block {
	return &block{model: model, x: x, y: y, parent: parent, spacing: spacing, smallestWidth: math.Inf(1), smallestHeight: math.Inf(1)}
}

func (value *block) lastRow() *blockRow { return value.rows[len(value.rows)-1] }

func (value *block) add(id graph.NodeID) {
	if len(value.rows) == 0 {
		value.rows = append(value.rows, newBlockRow(value.x, value.y, value.spacing))
	}
	value.children = append(value.children, id)
	value.lastRow().add(value.model, id)
	value.adjustAdd(id)
}

func (value *block) addNewRow(id graph.NodeID) {
	value.children = append(value.children, id)
	last := value.lastRow()
	value.rows = append(value.rows, newBlockRow(value.x, last.y+last.height+value.spacing, value.spacing))
	value.lastRow().add(value.model, id)
	value.adjustAdd(id)
}

func (value *block) remove(id graph.NodeID) {
	for index, candidate := range value.children {
		if candidate == id {
			value.children = append(value.children[:index], value.children[index+1:]...)
			break
		}
	}
	for index, row := range value.rows {
		found := false
		for _, candidate := range row.nodes {
			if candidate == id {
				found = true
				break
			}
		}
		if found {
			row.remove(value.model, id, true)
			if len(row.nodes) == 0 {
				value.rows = append(value.rows[:index], value.rows[index+1:]...)
			}
			break
		}
	}
	value.adjustRemove()
}

func (value *block) setLocation(x, y float64) {
	dx, dy := x-value.x, y-value.y
	for _, id := range value.children {
		node := value.model.Node(id)
		node.Shape.SetLocation(node.Shape.X+dx, node.Shape.Y+dy)
	}
	for _, row := range value.rows {
		row.x += dx
		row.y += dy
	}
	value.x, value.y = x, y
}

func (value *block) adjustAdd(id graph.NodeID) {
	node := value.model.Node(id)
	value.smallestWidth = math.Min(value.smallestWidth, node.Shape.Width)
	value.width = math.Max(value.width, value.lastRow().width)
	value.minWidth = math.Max(value.minWidth, node.Shape.Width+func() float64 {
		if len(value.children) == 1 {
			return 0
		}
		return value.spacing
	}())
	value.smallestHeight = math.Min(value.smallestHeight, node.Shape.Height)
	if len(value.children) > 1 {
		value.maxHeight += value.spacing
	}
	value.maxHeight += node.Shape.Height
	value.minHeight = math.Max(value.minHeight, node.Shape.Height)
	totalHeight := float64(max(0, len(value.rows)-1)) * value.spacing
	for _, row := range value.rows {
		totalHeight += row.height
	}
	value.height = totalHeight
	value.averageHeight = value.maxHeight/float64(len(value.children)) - value.spacing*(float64(len(value.children)-1)/float64(len(value.children)))
	value.parent.notify()
}

func (value *block) adjustRemove() {
	newWidth, newHeight := 0.0, 0.0
	for index, row := range value.rows {
		newWidth = math.Max(newWidth, row.width)
		newHeight += row.height
		if index > 0 {
			newHeight += value.spacing
		}
	}
	value.width, value.height = newWidth, newHeight
	value.minWidth, value.minHeight, value.maxHeight = 0, 0, 0
	value.smallestWidth, value.smallestHeight = math.Inf(1), math.Inf(1)
	for _, id := range value.children {
		node := value.model.Node(id)
		value.smallestWidth = math.Min(value.smallestWidth, node.Shape.Width)
		value.minWidth = math.Max(value.minWidth, node.Shape.Width)
		value.minHeight = math.Max(value.minHeight, node.Shape.Height)
		value.smallestHeight = math.Min(value.smallestHeight, node.Shape.Height)
		value.maxHeight += node.Shape.Height + value.spacing
	}
	if len(value.children) > 0 {
		value.averageHeight = value.maxHeight/float64(len(value.children)) - value.spacing*(float64(len(value.children)-1)/float64(len(value.children)))
	}
	value.parent.notify()
}

func (value *block) layoutIn(width float64, place bool) (float64, float64) {
	currentX, currentY := 0.0, value.y
	currentWidth, currentHeight, maxHeight, widthInRow := 0.0, 0.0, 0.0, 0.0
	rowIndex, index := 0, 0
	if place {
		value.rows = []*blockRow{newBlockRow(value.x, value.y, value.spacing)}
	}
	for _, id := range value.children {
		node := value.model.Node(id)
		space := 0.0
		if index > 0 {
			space = value.spacing
		}
		if currentX+node.Shape.Width+space > width && maxHeight > 0 {
			currentX = 0
			currentY += maxHeight + value.spacing
			currentWidth = math.Max(currentWidth, widthInRow)
			currentHeight += maxHeight + value.spacing
			maxHeight, widthInRow, index = 0, 0, 0
			if place {
				rowIndex++
				value.rows = append(value.rows, newBlockRow(value.x, currentY, value.spacing))
			}
		}
		space = 0
		if index > 0 {
			space = value.spacing
		}
		widthInRow += node.Shape.Width + space
		maxHeight = math.Max(maxHeight, node.Shape.Height)
		if place {
			value.rows[rowIndex].add(value.model, id)
		}
		currentX += node.Shape.Width + space
		index++
	}
	currentWidth = math.Max(currentWidth, widthInRow)
	currentHeight += maxHeight
	if place {
		value.width, value.height = currentWidth, currentHeight
		value.parent.notify()
	}
	return currentWidth, currentHeight
}

func (value *block) placeIn(width float64) bool {
	oldW, oldH := value.width, value.height
	w, h := value.layoutIn(width, true)
	return w != oldW || h != oldH
}

func (value *block) heightForWidth(width float64) float64 {
	_, height := value.layoutIn(width, false)
	return height
}

func (value *block) widthForHeight(height float64) float64 {
	if value.maxHeight <= height {
		return value.minWidth
	}
	if width, h := value.layoutIn(value.minWidth, false); width <= value.minWidth && h <= height {
		return value.minWidth
	}
	upper, lower, viable := value.width, value.minWidth, value.width
	trial := (upper-lower)/2 + lower
	for lower+1 < upper {
		width, h := value.layoutIn(trial, false)
		if width <= trial && h <= height {
			viable, upper = trial, trial
		} else {
			lower = trial
		}
		trial = (upper-lower)/2 + lower
	}
	return viable
}

func (value *block) expand(additionalWidth, additionalHeight float64) {
	width := value.width + additionalWidth
	value.width, value.height = width, value.height+additionalHeight
	perRow := additionalHeight / float64(len(value.rows))
	for index, row := range value.rows {
		row.expand(value.model, width, perRow, index)
	}
}

type blockStack struct {
	blocks                       []*block
	x, y, width, height, spacing float64
}

func newBlockStack(x, y, spacing float64) *blockStack {
	return &blockStack{x: x, y: y, spacing: spacing}
}
func (stack *blockStack) add(value *block) {
	value.stack = stack
	stack.width = math.Max(stack.width, value.width)
	if len(stack.blocks) > 0 {
		stack.height += stack.spacing
	}
	stack.height += value.height
	stack.blocks = append(stack.blocks, value)
}
func (stack *blockStack) update() {
	stack.width, stack.height = 0, 0
	for index, value := range stack.blocks {
		stack.width = math.Max(stack.width, value.width)
		if index > 0 {
			stack.height += stack.spacing
		}
		stack.height += value.height
	}
}
func (stack *blockStack) setLocation(x, y float64) {
	dx, dy := x-stack.x, y-stack.y
	for _, value := range stack.blocks {
		value.setLocation(value.x+dx, value.y+dy)
	}
	stack.x, stack.y = x, y
}
func (stack *blockStack) minimumWidth() float64 {
	result := 0.0
	for _, value := range stack.blocks {
		result = math.Max(result, value.minWidth)
	}
	return result
}
func (stack *blockStack) widthForHeight(height float64) float64 {
	if len(stack.blocks) == 1 {
		return stack.blocks[0].widthForHeight(height)
	}
	lower, upper, viable := stack.minimumWidth(), stack.width, stack.width
	trial := (upper-lower)/2 + lower
	for lower+1 < upper {
		total := 0.0
		for _, value := range stack.blocks {
			total += value.heightForWidth(trial)
		}
		if total < height {
			viable, upper = trial, trial
		} else {
			lower = trial
		}
		trial = (upper-lower)/2 + lower
	}
	return viable
}
func (stack *blockStack) placeIn(width float64) {
	currentY, currentHeight, currentWidth := stack.y, 0.0, 0.0
	for _, value := range stack.blocks {
		value.setLocation(stack.x, currentY)
		value.placeIn(width)
		currentWidth = math.Max(currentWidth, value.width)
		currentY += value.height + stack.spacing
		currentHeight = currentY
	}
	stack.width, stack.height = currentWidth, currentHeight
}
func (stack *blockStack) expand(additionalWidth, additionalHeight float64) {
	perBlock := additionalHeight / float64(len(stack.blocks))
	for index, value := range stack.blocks {
		value.setLocation(value.x, value.y+float64(index)*perBlock)
		value.expand(stack.width-value.width+additionalWidth, perBlock)
	}
}

type rectRow struct {
	y, width, height, spacing float64
	blocks                    []*block
	stacks                    []*blockStack
}

func newRectRow(y, spacing float64) *rectRow { return &rectRow{y: y, spacing: spacing} }
func (row *rectRow) notify() {
	row.width = 0
	row.height = math.Inf(-1)
	for index, value := range row.blocks {
		if index > 0 {
			row.width += row.spacing
		}
		row.width += value.width
		row.height = math.Max(row.height, value.height)
	}
}
func (row *rectRow) add(value *block) {
	row.height = math.Max(row.height, value.height)
	if len(row.blocks) > 0 {
		row.width += row.spacing
	}
	row.width += value.width
	row.blocks = append(row.blocks, value)
}
func (row *rectRow) remove(value *block) {
	for index, candidate := range row.blocks {
		if candidate == value {
			row.blocks = append(row.blocks[:index], row.blocks[index+1:]...)
			break
		}
	}
	row.width -= value.width
	if len(row.blocks) > 0 {
		row.width -= row.spacing
	}
	row.height = math.SmallestNonzeroFloat64
	for _, candidate := range row.blocks {
		row.height = math.Max(row.height, candidate.height)
	}
}
func (row *rectRow) setY(y float64) {
	diff := y - row.y
	for _, stack := range row.stacks {
		stack.setLocation(stack.x, stack.y+diff)
	}
	row.y = y
}
func (row *rectRow) expand(model *graph.Model, width, additionalHeight float64) {
	perStack := (width - row.width) / float64(len(row.stacks))
	for index, stack := range row.stacks {
		stack.setLocation(stack.x+float64(index)*perStack, stack.y)
		stack.expand(perStack, row.height-stack.height+additionalHeight)
	}
}

func initialPlacement(model *graph.Model, rectangles []graph.NodeID, width, spacing float64, rowHeightReevaluation bool) []*rectRow {
	rows := make([]*rectRow, 0)
	row := newRectRow(0, spacing)
	row.add(newBlock(model, 0, 0, row, spacing))
	currentWidth, drawingHeight := 0.0, 0.0
	for _, id := range rectangles {
		value := row.blocks[len(row.blocks)-1]
		node := model.Node(id)
		potential := currentWidth + node.Shape.Width
		if len(row.blocks[0].children) > 0 {
			potential += spacing
		}
		if potential > width || graph.Get(&node.Properties, InNewRow) {
			currentWidth = 0
			drawingHeight += row.height + spacing
			rows = append(rows, row)
			row = newRectRow(drawingHeight, spacing)
			value = newBlock(model, 0, row.y, row, spacing)
			row.add(value)
		}
		if len(value.children) == 0 || !rowHeightReevaluation && similarHeight(model, value, id) {
			value.add(id)
		} else {
			value = newBlock(model, value.x+value.width+spacing, row.y, row, spacing)
			row.add(value)
			value.add(id)
		}
		currentWidth = node.Shape.X + node.Shape.Width
	}
	return append(rows, row)
}

func similarHeight(model *graph.Model, value *block, id graph.NodeID) bool {
	height := model.Node(id).Shape.Height
	if height >= value.smallestHeight && height <= value.minHeight {
		return true
	}
	return value.averageHeight*.5 <= height && value.averageHeight*1.5 >= height
}

func placeRectInBlock(row *rectRow, value *block, id graph.NodeID, width, spacing float64) bool {
	node := value.model.Node(id)
	if !similarHeight(value.model, value, id) {
		return false
	}
	last := value.lastRow()
	if last.x+last.width+node.Shape.Width+spacing <= width &&
		(last.y-row.y+node.Shape.Height <= row.height || len(row.blocks) == 1) {
		value.add(id)
		return true
	}
	if value.x+node.Shape.Width <= width &&
		value.y+value.height+node.Shape.Height+spacing <= row.y+row.height {
		value.addNewRow(id)
		return true
	}
	return false
}

func resetCoordinates(model *graph.Model, rectangles []graph.NodeID) {
	for _, id := range rectangles {
		model.Node(id).Shape.SetLocation(0, 0)
	}
}

func rectangleDimensions(model *graph.Model, rectangles []graph.NodeID) geometry.Point {
	result := geometry.Point{}
	for _, id := range rectangles {
		node := model.Node(id)
		result.X = math.Max(result.X, node.Shape.X+node.Shape.Width)
		result.Y = math.Max(result.Y, node.Shape.Y+node.Shape.Height)
	}
	return result
}

func rowsDimensions(rows []*rectRow, spacing float64) geometry.Point {
	result := geometry.Point{}
	for index, row := range rows {
		result.X = math.Max(result.X, row.width)
		if index > 0 {
			result.Y += spacing
		}
		result.Y += row.height
	}
	return result
}

func rowsHeight(rows []*rectRow, spacing float64) float64 {
	return rowsDimensions(rows, spacing).Y
}

func finishPacking(size, minimum geometry.Point, padding geometry.Spacing, dar float64) drawingData {
	width := math.Max(size.X, minimum.X-padding.Horizontal())
	height := math.Max(size.Y, minimum.Y-padding.Vertical())
	additionalHeight := height - size.Y
	// SimplePlacement and NoPlacement in ELK 0.12 assign height plus the
	// additional height. The compactor uses size.y + additionalHeight, which
	// is simply height; callers select the appropriate behavior below.
	drawing := newDrawing(dar, width, height+additionalHeight, wholeDrawing, 0, 0)
	drawing.additionalHeight = additionalHeight
	return drawing
}

func simplePlacement(
	model *graph.Model,
	rectangles []graph.NodeID,
	targetWidth float64,
	minimum geometry.Point,
	padding geometry.Spacing,
	spacing, dar float64,
) (drawingData, []*rectRow) {
	resetCoordinates(model, rectangles)
	rows := initialPlacement(model, rectangles, targetWidth, spacing, false)
	for _, row := range rows {
		for _, value := range row.blocks {
			stack := newBlockStack(value.x, value.y, spacing)
			stack.add(value)
			row.stacks = append(row.stacks, stack)
		}
	}
	return finishPacking(rowsDimensions(rows, spacing), minimum, padding, dar), rows
}

func noPlacement(
	model *graph.Model,
	rectangles []graph.NodeID,
	minimum geometry.Point,
	padding geometry.Spacing,
	dar float64,
) drawingData {
	return finishPacking(rectangleDimensions(model, rectangles), minimum, padding, dar)
}

func compactPlacement(
	model *graph.Model,
	rectangles []graph.NodeID,
	targetWidth float64,
	minimum geometry.Point,
	padding geometry.Spacing,
	spacing, dar float64,
	rowHeightReevaluation bool,
	iterations int,
) (drawingData, []*rectRow) {
	drawing, rows, changes := compactOnce(
		model, rectangles, targetWidth, minimum, padding, spacing, dar, rowHeightReevaluation,
	)
	for iterations > 1 {
		positions := make([]geometry.Rectangle, len(rectangles))
		for index, id := range rectangles {
			positions[index] = model.Node(id).Shape.Rectangle()
		}

		candidateWidth := targetWidth
		ratio := (drawing.width + padding.Horizontal()) / (drawing.height + padding.Vertical())
		if len(rectangles) > 1 && !math.IsInf(changes.minIncrease, 1) && ratio < dar {
			candidateWidth = targetWidth + changes.minIncrease
		} else if len(rectangles) > 1 && !math.IsInf(changes.minDecrease, 1) && ratio > dar {
			candidateWidth = math.Max(minimum.X, targetWidth-changes.minDecrease)
		}

		candidate, candidateRows, candidateChanges := compactOnce(
			model, rectangles, candidateWidth, minimum, padding, spacing, dar, rowHeightReevaluation,
		)
		if candidate.scale >= drawing.scale && !math.IsNaN(candidate.scale) {
			drawing, rows, changes = candidate, candidateRows, candidateChanges
		} else {
			for index, id := range rectangles {
				node := model.Node(id)
				position := positions[index]
				node.Shape.SetLocation(position.X, position.Y)
				node.Shape.SetDimensions(position.Width, position.Height)
			}
		}
		iterations--
	}
	return drawing, rows
}

type rowWidthChanges struct {
	minIncrease, maxIncrease float64
	minDecrease, maxDecrease float64
}

func compactOnce(
	model *graph.Model,
	rectangles []graph.NodeID,
	targetWidth float64,
	minimum geometry.Point,
	padding geometry.Spacing,
	spacing, dar float64,
	rowHeightReevaluation bool,
) (drawingData, []*rectRow, rowWidthChanges) {
	resetCoordinates(model, rectangles)
	rows := initialPlacement(model, rectangles, targetWidth, spacing, rowHeightReevaluation)
	changes := rowWidthChanges{minIncrease: math.Inf(1), minDecrease: math.Inf(1)}
	for index := 0; index < len(rows); index++ {
		row := rows[index]
		if index > 0 {
			row.setY(rows[index-1].y + rows[index-1].height + spacing)
		}
		_, repeat := compact(index, &rows, targetWidth, spacing, rowHeightReevaluation)
		if repeat {
			for _, value := range row.blocks {
				value.fixed = false
				value.positionFixed = false
				value.adjustRemove()
			}
			row.stacks = nil
			row.width = targetWidth
			index--
			continue
		}
		adjustRow(row, spacing)
		if index+1 < len(rows) {
			increase := row.width + spacing + rows[index+1].blocks[0].width - targetWidth
			// Preserve ELK 0.12's observable update order, including the two
			// cross-field references in RowFillingAndCompaction.
			changes.maxIncrease = math.Max(increase, changes.maxDecrease)
			changes.minIncrease = math.Min(increase, changes.minDecrease)
			if len(row.stacks) != 0 {
				decrease := row.stacks[len(row.stacks)-1].width
				if len(row.stacks) > 1 {
					decrease += spacing
				}
				changes.maxDecrease = math.Max(changes.maxDecrease, decrease)
				changes.minDecrease = math.Min(changes.maxDecrease, decrease)
			}
		}
	}
	if len(rows) == 1 && len(rows[0].stacks) != 0 {
		lastStack := rows[0].stacks[len(rows[0].stacks)-1]
		if len(lastStack.blocks) != 0 {
			lastBlock := lastStack.blocks[len(lastStack.blocks)-1]
			for _, blockRow := range lastBlock.rows {
				decrease := lastBlock.width - blockRow.width
				changes.maxDecrease = math.Max(changes.maxDecrease, decrease)
				changes.minDecrease = math.Min(changes.minDecrease, decrease)
				increase := blockRow.width + spacing
				changes.maxIncrease = math.Max(changes.maxIncrease, increase)
				changes.minIncrease = math.Min(changes.minIncrease, increase)
			}
		}
	}
	size := rowsDimensions(rows, spacing)
	width := math.Max(size.X, minimum.X-padding.Horizontal())
	height := math.Max(size.Y, minimum.Y-padding.Vertical())
	drawing := newDrawing(dar, width, height, wholeDrawing, 0, 0)
	drawing.additionalHeight = height - size.Y
	return drawing, rows, changes
}

func expandRows(model *graph.Model, rows []*rectRow, width, additionalHeight float64) {
	if len(rows) == 0 {
		return
	}
	perRow := additionalHeight / float64(len(rows))
	for index, row := range rows {
		row.setY(row.y + perRow*float64(index))
		row.expand(model, width, perRow)
	}
}

func adjustRow(row *rectRow, spacing float64) {
	row.height, row.width = 0, 0
	for index, stack := range row.stacks {
		stack.update()
		row.height = math.Max(row.height, stack.height)
		if index > 0 {
			row.width += spacing
		}
		row.width += stack.width
	}
}

func nextBlock(rows []*rectRow, row *rectRow, blockIndex, nextRowIndex int) *block {
	if blockIndex < len(row.blocks)-1 {
		return row.blocks[blockIndex+1]
	}
	if nextRowIndex < len(rows) && len(rows[nextRowIndex].blocks) > 0 {
		return rows[nextRowIndex].blocks[0]
	}
	return nil
}

func compact(rowIndex int, rows *[]*rectRow, width, spacing float64, rowHeightReevaluation bool) (bool, bool) {
	changed := false
	compactAgain := false
	nextRowIndex := rowIndex + 1
	row := (*rows)[rowIndex]
	var currentStack *blockStack
	for blockIndex := 0; blockIndex < len(row.blocks); blockIndex++ {
		value := row.blocks[blockIndex]
		if value.fixed {
			continue
		}
		if len(value.children) == 0 {
			row.remove(value)
			blockIndex--
			changed = true
			continue
		}
		if !value.positionFixed {
			if currentStack != nil {
				currentStack.update()
			}
			x := 0.0
			if currentStack != nil {
				x = currentStack.x + currentStack.width + spacing
			}
			currentStack = newBlockStack(x, row.y, spacing)
			value.setLocation(currentStack.x+currentStack.width, row.y)
			row.stacks = append(row.stacks, currentStack)
			currentStack.add(value)
			value.positionFixed = true
		}
		next := nextBlock(*rows, row, blockIndex, nextRowIndex)
		fromNext := next != nil && next.parent != row
		if next != nil {
			if len(next.children) > 0 && !graph.Get(&modelNode(next).Properties, InNewRow) {
				value.placeIn(width - value.x)
				value.stack.update()
				changed = absorb(row, value, next, width, spacing) || changed
			} else {
				// ELK 0.12 takes this branch both for an empty next block and
				// for an inNewRow boundary. It calls remove on the current row
				// even when the block belongs to the following row, then stops
				// compacting this row.
				row.remove(next)
				break
			}
			if len(next.children) == 0 {
				if nextRowIndex < len(*rows) {
					(*rows)[nextRowIndex].remove(next)
				}
				next = nil
				for len(*rows) > nextRowIndex && len((*rows)[nextRowIndex].blocks) == 0 {
					*rows = append((*rows)[:nextRowIndex], (*rows)[nextRowIndex+1:]...)
				}
			}
			if next == nil {
				blockIndex--
				continue
			}
			if !graph.Get(&modelNode(next).Properties, InNewRow) &&
				placeBelow(rows, row, value, next, fromNext, width, nextRowIndex, spacing) {
				changed = true
				continue
			}
			if fromNext {
				oldHeight := row.height
				nextMinHeight := next.minHeight
				if !graph.Get(&modelNode(next).Properties, InNewRow) &&
					placeBeside(rows, row, value, next, width, nextRowIndex, spacing, rowHeightReevaluation) {
					changed = true
					if oldHeight < nextMinHeight {
						compactAgain = true
						next.parent = row
						break
					}
					continue
				}
				if useRowHeight(row, value) {
					value.fixed = true
					changed = true
					continue
				}
			} else if useRowHeight(row, value) {
				value.fixed = true
				changed = true
				continue
			}
			if changed {
				continue
			}
		}
		if useRowHeight(row, value) {
			value.fixed = true
			changed = true
			if next != nil {
				next.positionFixed = false
			}
		} else {
			value.stack.update()
		}
	}
	return changed, compactAgain
}

func modelNode(value *block) *graph.Node {
	return value.model.Node(value.children[0])
}

func useRowHeight(row *rectRow, value *block) bool {
	old := value.stack.width
	if value.height < row.height {
		target := value.stack.widthForHeight(row.height)
		if value.stack.width > target {
			value.stack.placeIn(target)
			return old != value.stack.width
		}
	}
	return false
}

func absorb(row *rectRow, value, next *block, width, spacing float64) bool {
	changed := false
	for len(next.children) > 0 {
		id := next.children[0]
		if graph.Get(&next.model.Node(id).Properties, InNewRow) {
			break
		}
		if !placeRectInBlock(row, value, id, width, spacing) {
			break
		}
		changed = true
		next.remove(id)
	}
	if len(next.children) == 0 {
		next.parent.remove(next)
	}
	if changed {
		value.stack.update()
	}
	return changed
}

func placeBelow(rows *[]*rectRow, row *rectRow, value, next *block, fromNext bool, width float64, nextRowIndex int, spacing float64) bool {
	remaining := width - value.x
	currentMinHeight := value.y - row.y + value.heightForWidth(remaining)
	if next.minWidth+spacing > remaining {
		return false
	}
	nextMinHeight := next.heightForWidth(remaining)
	if currentMinHeight+spacing+nextMinHeight > row.height {
		return false
	}
	value.placeIn(remaining)
	value.fixed = true
	next.placeIn(remaining)
	next.setLocation(value.x, value.y+value.height+spacing)
	next.positionFixed = true
	value.stack.add(next)
	if fromNext {
		oldParent := next.parent
		row.add(next)
		next.parent = row
		if nextRowIndex < len(*rows) {
			oldParent.remove(next)
			if len((*rows)[nextRowIndex].blocks) == 0 {
				*rows = append((*rows)[:nextRowIndex], (*rows)[nextRowIndex+1:]...)
			}
		}
	}
	return true
}

func placeBeside(rows *[]*rectRow, row *rectRow, value, next *block, width float64, nextRowIndex int, spacing float64, rowHeightReevaluation bool) bool {
	currentMinWidth := value.stack.widthForHeight(row.y + row.height - value.stack.y)
	shouldReevaluate := next.minHeight > row.height && rowHeightReevaluation
	targetNext := width - (value.stack.x + currentMinWidth - spacing)
	nextHeight := next.heightForWidth(targetNext)
	if shouldReevaluate && nextHeight > next.minHeight {
		return false
	}
	if shouldReevaluate {
		potentialWidth := 0.0
		for _, stack := range row.stacks {
			potentialWidth += stack.widthForHeight(next.minHeight) + spacing
		}
		targetNext = width - potentialWidth
	}
	if targetNext < next.minWidth {
		return false
	}
	lastRowOptimization := nextRowIndex == len(*rows)-1 && targetNext >= (*rows)[nextRowIndex].width
	if !shouldReevaluate && nextHeight > row.height && !lastRowOptimization {
		return false
	}
	if lastRowOptimization && nextHeight > row.height {
		value.height = nextHeight
		value.placeIn(value.widthForHeight(nextHeight))
	} else {
		value.stack.placeIn(currentMinWidth)
		value.fixed = true
	}
	next.placeIn(width - (value.x + value.width))
	next.setLocation(value.stack.x+value.stack.width, row.y)
	oldParent := next.parent
	row.add(next)
	next.parent = row
	if nextRowIndex < len(*rows) {
		oldParent.remove(next)
		if len((*rows)[nextRowIndex].blocks) == 0 {
			*rows = append((*rows)[:nextRowIndex], (*rows)[nextRowIndex+1:]...)
		}
	}
	return true
}
