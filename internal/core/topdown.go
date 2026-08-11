// Copyright (c) 2022 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"fmt"
	"math"

	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

var topdownMargins = graph.CloneablePropertyWithDefault("org.eclipse.elk.margins", func() geometry.Spacing {
	return geometry.Spacing{}
})

// layoutTopdown is the top-down branch of ELK 0.12's recursive graph layout
// engine. It sizes compound children before laying out their parent, then
// records the scale and coordinate compensation required by hierarchical
// nodes before recursion continues into those children.
func (engine *Engine) layoutTopdown(
	model *graph.Model,
	parent graph.NodeID,
	algorithm *AlgorithmData,
	monitor ProgressMonitor,
) error {
	node := model.Node(parent)
	nodeType, found := graph.TryGet(&node.Properties, TopdownNodeType)
	if !found || nodeType == "" {
		return fmt.Errorf("%s has not been assigned a top-down node type.", nodePath(model, parent))
	}

	if nodeType == "HIERARCHICAL_NODE" || nodeType == "ROOT_NODE" {
		for _, childID := range node.Children {
			child := model.Node(childID)
			if len(child.Children) == 0 {
				continue
			}
			if graph.Has(&child.Properties, TopdownSizeApproximator) {
				return fmt.Errorf("topdown size approximators cannot be constructed through LayoutJSON")
			}
			padding := graph.Get(&child.Properties, Padding)
			width := graph.Get(&child.Properties, TopdownHierarchicalNodeWidth)
			aspect := graph.Get(&child.Properties, TopdownHierarchicalNodeAspectRatio)
			child.Shape.SetDimensions(
				math.Max(child.Shape.Width, width+padding.Horizontal()),
				math.Max(child.Shape.Height, width/aspect+padding.Vertical()),
			)
		}
	}

	padding := graph.Get(&node.Properties, Padding)
	availableWidth := node.Shape.Width - padding.Horizontal()
	availableHeight := node.Shape.Height - padding.Vertical()
	graph.Set(&node.Properties, AspectRatio, availableWidth/availableHeight)

	if err := engine.executeAlgorithm(model, parent, algorithm, monitor); err != nil {
		return err
	}

	if nodeType == "ROOT_NODE" {
		computeChildAreaDimensions(model, parent)
		width := graph.Get(&node.Properties, ChildAreaWidth)
		height := graph.Get(&node.Properties, ChildAreaHeight)
		node.Shape.SetDimensions(padding.Left+width+padding.Right, padding.Top+height+padding.Bottom)
	}

	if nodeType != "HIERARCHICAL_NODE" {
		return nil
	}
	if availableWidth < 0 || availableHeight < 0 {
		return fmt.Errorf("The size defined by the parent parallel node is too small for the space provided by the paddings of the child hierarchical node. %s",
			nodePath(model, parent))
	}
	if !graph.Has(&node.Properties, ChildAreaWidth) && !graph.Has(&node.Properties, ChildAreaHeight) {
		computeChildAreaDimensions(model, parent)
	}
	desiredWidth := graph.Get(&node.Properties, ChildAreaWidth)
	desiredHeight := graph.Get(&node.Properties, ChildAreaHeight)
	scaleX := availableWidth / desiredWidth
	scaleY := availableHeight / desiredHeight
	scale := math.Min(scaleX, math.Min(scaleY, graph.Get(&node.Properties, TopdownScaleCap)))
	graph.Set(&node.Properties, TopdownScaleFactor, scale)

	var alignmentX, alignmentY float64
	alignment := graph.Get(&node.Properties, ContentAlignment)
	if scale < scaleX {
		if containsOption(alignment, "H_CENTER") {
			alignmentX = (availableWidth/2 - desiredWidth*scale/2) / scale
		} else if containsOption(alignment, "H_RIGHT") {
			alignmentX = (availableWidth - desiredWidth*scale) / scale
		}
	}
	if scale < scaleY {
		if containsOption(alignment, "V_CENTER") {
			alignmentY = (availableHeight/2 - desiredHeight*scale/2) / scale
		} else if containsOption(alignment, "V_BOTTOM") {
			alignmentY = (availableHeight - desiredHeight*scale) / scale
		}
	}
	offset := geometry.Point{
		X: alignmentX + padding.Left/scale - padding.Left,
		Y: alignmentY + padding.Top/scale - padding.Top,
	}
	translateTopdownContent(model, parent, offset)
	return nil
}

func containsOption(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func translateTopdownContent(model *graph.Model, parent graph.NodeID, offset geometry.Point) {
	node := model.Node(parent)
	for _, childID := range node.Children {
		child := model.Node(childID)
		child.Shape.SetLocation(child.Shape.X+offset.X, child.Shape.Y+offset.Y)
	}
	for _, edgeID := range node.ContainedEdges {
		edge := model.Edge(edgeID)
		for _, sectionID := range edge.Sections {
			TranslateSection(model, sectionID, offset)
		}
		for _, labelID := range edge.Labels {
			label := model.Label(labelID)
			label.Shape.SetLocation(label.Shape.X+offset.X, label.Shape.Y+offset.Y)
		}
		if junctions, found := graph.TryGet(&edge.Properties, JunctionPoints); found {
			junctions.Offset(offset)
			graph.Set(&edge.Properties, JunctionPoints, junctions)
		}
	}
}

func computeChildAreaDimensions(model *graph.Model, parent graph.NodeID) {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := 0.0, 0.0
	includeShape := func(shape *graph.Shape, properties *graph.PropertyMap) {
		margins := graph.Get(properties, topdownMargins)
		minX = math.Min(minX, shape.X-margins.Left)
		minY = math.Min(minY, shape.Y-margins.Top)
		maxX = math.Max(maxX, shape.X+shape.Width+margins.Right)
		maxY = math.Max(maxY, shape.Y+shape.Height+margins.Bottom)
	}
	node := model.Node(parent)
	for _, labelID := range node.Labels {
		label := model.Label(labelID)
		includeShape(&label.Shape, &label.Properties)
	}
	for _, childID := range node.Children {
		child := model.Node(childID)
		includeShape(&child.Shape, &child.Properties)
	}
	for _, edgeID := range node.ContainedEdges {
		edge := model.Edge(edgeID)
		for _, labelID := range edge.Labels {
			label := model.Label(labelID)
			includeShape(&label.Shape, &label.Properties)
		}
		for _, sectionID := range edge.Sections {
			section := model.Section(sectionID)
			minX = math.Min(minX, math.Min(section.Start.X, section.End.X))
			minY = math.Min(minY, math.Min(section.Start.Y, section.End.Y))
			maxX = math.Max(maxX, math.Max(section.Start.X, section.End.X))
			maxY = math.Max(maxY, math.Max(section.Start.Y, section.End.Y))
			for _, point := range section.BendPoints {
				minX = math.Min(minX, point.X)
				minY = math.Min(minY, point.Y)
				maxX = math.Max(maxX, point.X)
				maxY = math.Max(maxY, point.Y)
			}
		}
	}
	graph.Set(&node.Properties, ChildAreaWidth, maxX-minX)
	graph.Set(&node.Properties, ChildAreaHeight, maxY-minY)
}
