// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package disco ports org.eclipse.elk.alg.disco from ELK 0.8.1.
package disco

import (
	"fmt"
	"math"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/disco/polyomino"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

type Provider struct {
	algorithms *core.AlgorithmRegistry
}

func Register(registry *core.AlgorithmRegistry) error {
	return registry.Register(core.AlgorithmData{
		ID:       "org.eclipse.elk.disco",
		Features: core.FeatureDisconnected | core.FeatureMultiEdges | core.FeatureEdgeLabels | core.FeaturePorts,
		Create:   func() core.LayoutProvider { return &Provider{algorithms: registry} },
		KnownOptions: []string{
			SpacingComponent.ID, EdgeThickness.ID, AspectRatio.ID, Padding.ID,
			PolyLowSort.ID, PolyHighSort.ID, PolyTraversal.ID, PolyFill.ID,
			Strategy.ID, ComponentLayoutAlgorithm.ID, DebugGraph.ID, DebugPolyominoes.ID,
		},
	})
}

type rectangle struct{ x, y, width, height float64 }

func (value rectangle) right() float64  { return value.x + value.width }
func (value rectangle) bottom() float64 { return value.y + value.height }
func intersects(a, b rectangle) bool {
	return a.x <= b.right() && a.right() >= b.x && a.y <= b.bottom() && a.bottom() >= b.y
}

type component struct {
	nodes                     []graph.NodeID
	edges                     []graph.EdgeID
	shapes                    []rectangle
	minX, minY, width, height float64
	offset                    geometry.Point
}

func (provider *Provider) Layout(model *graph.Model, parent graph.NodeID, monitor core.ProgressMonitor) error {
	node := model.Node(parent)
	if requested, present := graph.TryGet(&node.Properties, ComponentLayoutAlgorithm); present && provider.algorithms != nil {
		if algorithm := provider.algorithms.BySuffix(requested); algorithm != nil {
			componentProvider := algorithm.Create()
			if componentProvider == nil {
				return fmt.Errorf("component layout algorithm %q returned a nil provider", requested)
			}
			if err := componentProvider.Layout(model, parent, monitor.Subtask(1)); err != nil {
				return err
			}
		}
	}
	spacing := graph.Get(&node.Properties, SpacingComponent)
	components := splitComponents(model, parent)
	for _, edgeID := range node.ContainedEdges {
		edge := model.Edge(edgeID)
		if len(edge.Sections) == 0 {
			sectionID := model.AddSection(edgeID)
			section := model.Section(sectionID)
			section.StartPresent, section.EndPresent = true, true
		}
	}
	for _, comp := range components {
		importShapes(model, comp, spacing)
	}
	cell := computeCellSize(components)
	aspect := 1.0
	if explicit, found := graph.TryGet(&node.Properties, AspectRatio); found {
		aspect = explicit
	}
	cellX, cellY := cell, cell
	if aspect > 1 {
		cellX *= aspect
	} else {
		cellY /= aspect
	}
	polys := make([]*polyomino.Poly, 0, len(components))
	for index, comp := range components {
		polys = append(polys, makePoly(comp, cellX, cellY, index))
	}
	grid, polys := polyomino.Pack(polys, polyomino.Options{
		AspectRatio: aspect, Fill: graph.Get(&node.Properties, PolyFill),
		// Preserve ELK 0.8.1's metadata-plumbing behavior. DisCo copies
		// fill into Polyominoes, but the three strategy options never reach
		// that holder and consequently remain at their declared defaults.
		LowLevelSort: "BY_SIZE_AND_SHAPE", HighLevelSort: "NUM_OF_EXTERNAL_SIDES_THAN_NUM_OF_EXTENSIONS_LAST",
		Traversal: "QUADRANTS_LINE_BY_LINE",
	})
	cropX, cropY, cropWidth, cropHeight := grid.FilledBounds()
	padding := graph.Get(&node.Properties, Padding)
	oldWidth, oldHeight := node.Shape.Width, node.Shape.Height
	newWidth := float64(cropWidth)*cellX + padding.Horizontal()
	newHeight := float64(cropHeight)*cellY + padding.Vertical()
	node.Shape.SetDimensions(newWidth, newHeight)
	xFactor, yFactor := newWidth/oldWidth, newHeight/oldHeight
	for _, labelID := range node.Labels {
		label := model.Label(labelID)
		label.Shape.SetLocation(label.Shape.X*xFactor, label.Shape.Y*yFactor)
	}
	for _, portID := range node.Ports {
		port := model.Port(portID)
		x, y := port.Shape.X, port.Shape.Y
		if x > 0 {
			x *= xFactor
		}
		if y > 0 {
			y *= yFactor
		}
		port.Shape.SetLocation(x, y)
	}
	for _, poly := range polys {
		comp := components[poly.Data]
		polyOffset := geometry.Point{X: (comp.width - float64(poly.Width)*cellX) * -.5, Y: (comp.height - float64(poly.Height)*cellY) * -.5}
		position := geometry.Point{X: float64(poly.X-cropX)*cellX + polyOffset.X, Y: float64(poly.Y-cropY)*cellY + polyOffset.Y}
		comp.offset = geometry.Point{X: position.X - comp.minX, Y: position.Y - comp.minY}
		applyOffset(model, comp)
	}
	return nil
}

func splitComponents(model *graph.Model, parent graph.NodeID) []*component {
	children := model.Node(parent).Children
	index := make(map[graph.NodeID]int, len(children))
	for i, id := range children {
		index[id] = i
	}
	adjacency := make([][]graph.NodeID, len(children))
	edgesForNode := make([][]graph.EdgeID, len(children))
	for _, edgeID := range model.Node(parent).ContainedEdges {
		edge := model.Edge(edgeID)
		endpoints := make([]graph.NodeID, 0, len(edge.Sources)+len(edge.Targets))
		for _, shape := range append(append([]graph.ShapeID(nil), edge.Sources...), edge.Targets...) {
			id := model.ShapeNode(shape)
			if _, ok := index[id]; ok {
				endpoints = append(endpoints, id)
				edgesForNode[index[id]] = appendUniqueEdge(edgesForNode[index[id]], edgeID)
			}
		}
		for _, a := range endpoints {
			for _, b := range endpoints {
				if a != b {
					adjacency[index[a]] = appendUniqueNode(adjacency[index[a]], b)
				}
			}
		}
	}
	visited := make(map[graph.NodeID]bool, len(children))
	result := make([]*component, 0)
	for _, start := range children {
		if visited[start] {
			continue
		}
		comp := &component{}
		stack := []graph.NodeID{start}
		for len(stack) > 0 {
			current := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if visited[current] {
				continue
			}
			visited[current] = true
			comp.nodes = append(comp.nodes, current)
			for _, edge := range edgesForNode[index[current]] {
				comp.edges = appendUniqueEdge(comp.edges, edge)
			}
			neighbors := adjacency[index[current]]
			for i := len(neighbors) - 1; i >= 0; i-- {
				if !visited[neighbors[i]] {
					stack = append(stack, neighbors[i])
				}
			}
		}
		result = append(result, comp)
	}
	return result
}
func appendUniqueNode(values []graph.NodeID, value graph.NodeID) []graph.NodeID {
	for _, v := range values {
		if v == value {
			return values
		}
	}
	return append(values, value)
}
func appendUniqueEdge(values []graph.EdgeID, value graph.EdgeID) []graph.EdgeID {
	for _, v := range values {
		if v == value {
			return values
		}
	}
	return append(values, value)
}

func importShapes(model *graph.Model, comp *component, spacing float64) {
	half := spacing / 2
	for _, id := range comp.nodes {
		node := model.Node(id)
		comp.shapes = append(comp.shapes, rectangle{node.Shape.X - half, node.Shape.Y - half, node.Shape.Width + spacing, node.Shape.Height + spacing})
		for _, labelID := range node.Labels {
			label := model.Label(labelID)
			comp.shapes = append(comp.shapes, rectangle{node.Shape.X + label.Shape.X - half, node.Shape.Y + label.Shape.Y - half, label.Shape.Width + spacing, label.Shape.Height + spacing})
		}
		for _, portID := range node.Ports {
			port := model.Port(portID)
			comp.shapes = append(comp.shapes, rectangle{node.Shape.X + port.Shape.X - half, node.Shape.Y + port.Shape.Y - half, port.Shape.Width + spacing, port.Shape.Height + spacing})
			for _, labelID := range port.Labels {
				label := model.Label(labelID)
				comp.shapes = append(comp.shapes, rectangle{node.Shape.X + port.Shape.X + label.Shape.X - half, node.Shape.Y + port.Shape.Y + label.Shape.Y - half, label.Shape.Width + spacing, label.Shape.Height + spacing})
			}
		}
	}
	for _, edgeID := range comp.edges {
		edge := model.Edge(edgeID)
		thickness := graph.Get(&edge.Properties, EdgeThickness) + spacing
		for _, sectionID := range edge.Sections {
			section := model.Section(sectionID)
			points := append([]geometry.Point{section.Start}, section.BendPoints...)
			points = append(points, section.End)
			for i := 0; i+1 < len(points); i++ {
				first, second := points[i], points[i+1]
				angle := math.Atan2(second.X-first.X, second.Y-first.Y)
				radius := thickness / 2
				offsetX := radius * math.Sin(angle+math.Pi/2)
				offsetY := radius * math.Cos(angle+math.Pi/2)
				x0 := math.Min(math.Min(first.X+offsetX, first.X-offsetX), math.Min(second.X+offsetX, second.X-offsetX))
				x1 := math.Max(math.Max(first.X+offsetX, first.X-offsetX), math.Max(second.X+offsetX, second.X-offsetX))
				y0 := math.Min(math.Min(first.Y+offsetY, first.Y-offsetY), math.Min(second.Y+offsetY, second.Y-offsetY))
				y1 := math.Max(math.Max(first.Y+offsetY, first.Y-offsetY), math.Max(second.Y+offsetY, second.Y-offsetY))
				comp.shapes = append(comp.shapes, rectangle{x0, y0, x1 - x0, y1 - y0})
			}
		}
		for _, labelID := range edge.Labels {
			label := model.Label(labelID)
			comp.shapes = append(comp.shapes, rectangle{label.Shape.X - half, label.Shape.Y - half, label.Shape.Width + spacing, label.Shape.Height + spacing})
		}
	}
	comp.minX, comp.minY = math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, shape := range comp.shapes {
		comp.minX = math.Min(comp.minX, shape.x)
		comp.minY = math.Min(comp.minY, shape.y)
		maxX = math.Max(maxX, shape.right())
		maxY = math.Max(maxY, shape.bottom())
	}
	comp.width, comp.height = maxX-comp.minX, maxY-comp.minY
}

func computeCellSize(components []*component) float64 {
	sum, product := 0.0, 0.0
	for _, comp := range components {
		sum += comp.width + comp.height
		product += comp.width * comp.height
	}
	n := float64(len(components))
	numerator := math.Sqrt(4*100*n*product-4*product+sum*sum) + sum
	denominator := 2 * (100*n - 1)
	if denominator == 0 {
		return numerator
	}
	return numerator / denominator
}

func makePoly(comp *component, cellX, cellY float64, index int) *polyomino.Poly {
	width, height := int(math.Ceil(comp.width/cellX)), int(math.Ceil(comp.height/cellY))
	poly := polyomino.NewPoly(width, height)
	poly.Data = index
	offsetX, offsetY := (comp.width-float64(width)*cellX)*-.5, (comp.height-float64(height)*cellY)*-.5
	baseX, baseY := comp.minX-offsetX, comp.minY-offsetY
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			cell := rectangle{baseX + float64(x)*cellX, baseY + float64(y)*cellY, cellX, cellY}
			for _, shape := range comp.shapes {
				if intersects(shape, cell) {
					poly.Set(x, y, polyomino.Blocked)
					break
				}
			}
		}
	}
	return poly
}

func applyOffset(model *graph.Model, comp *component) {
	for _, id := range comp.nodes {
		node := model.Node(id)
		node.Shape.SetLocation(node.Shape.X+comp.offset.X, node.Shape.Y+comp.offset.Y)
	}
	for _, edgeID := range comp.edges {
		edge := model.Edge(edgeID)
		for _, sectionID := range edge.Sections {
			core.TranslateSection(model, sectionID, comp.offset)
		}
		for _, labelID := range edge.Labels {
			label := model.Label(labelID)
			label.Shape.SetLocation(label.Shape.X+comp.offset.X, label.Shape.Y+comp.offset.Y)
		}
	}
}
