// Copyright (c) 2011, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package force

import (
	"math"
	"sort"

	"github.com/d2lang/elk-go/internal/geometry"
)

// Split mirrors ComponentsProcessor.split, including its source-shaped DFS
// edge append behavior on cyclic components.
func Split(fgraph *FGraph) []*FGraph {
	if !fgraph.SeparateConnectedComponents {
		return []*FGraph{fgraph}
	}
	visited := make([]bool, len(fgraph.Nodes))
	incidence := make([][]*FEdge, len(fgraph.Nodes))
	for _, edge := range fgraph.Edges {
		incidence[edge.Source.ID] = append(incidence[edge.Source.ID], edge)
		incidence[edge.Target.ID] = append(incidence[edge.Target.ID], edge)
	}
	components := make([]*FGraph, 0)
	var dfs func(*FNode, *FNode, *FGraph) *FGraph
	dfs = func(node, last *FNode, component *FGraph) *FGraph {
		if visited[node.ID] {
			return nil
		}
		visited[node.ID] = true
		if component == nil {
			component = copyGraphProperties(fgraph)
		}
		component.Nodes = append(component.Nodes, node)
		for _, edge := range incidence[node.ID] {
			if edge.Target == last || edge.Source == last {
				continue
			}
			if edge.Source != node {
				dfs(edge.Source, node, component)
			}
			if edge.Target != node {
				dfs(edge.Target, node, component)
			}
			component.Edges = append(component.Edges, edge)
			component.Labels = append(component.Labels, edge.Labels...)
		}
		return component
	}
	for _, node := range fgraph.Nodes {
		if component := dfs(node, nil, nil); component != nil {
			components = append(components, component)
		}
	}
	if len(components) > 1 {
		for _, component := range components {
			for id, node := range component.Nodes {
				node.ID = id
			}
		}
	}
	return components
}

func copyGraphProperties(source *FGraph) *FGraph {
	return &FGraph{
		Origin:                      source.Origin,
		Model:                       source.Model,
		Iterations:                  source.Iterations,
		Temperature:                 source.Temperature,
		Repulsion:                   source.Repulsion,
		SpacingNodeNode:             source.SpacingNodeNode,
		AspectRatio:                 source.AspectRatio,
		SeparateConnectedComponents: source.SeparateConnectedComponents,
		Interactive:                 source.Interactive,
		Padding:                     source.Padding,
		Random:                      source.Random,
		Priority:                    source.Priority,
		BBUpLeft:                    source.BBUpLeft,
		BBLowRight:                  source.BBLowRight,
	}
}

// Recombine mirrors ComponentsProcessor.recombine.
func Recombine(components []*FGraph) *FGraph {
	if len(components) == 1 {
		return components[0]
	}
	if len(components) == 0 {
		return &FGraph{}
	}
	for _, component := range components {
		priority := 0
		minX, minY := float64(math.MaxInt32), float64(math.MaxInt32)
		maxX, maxY := float64(math.MinInt32), float64(math.MinInt32)
		for _, node := range component.Nodes {
			priority += node.Priority
			topLeft, bottomRight := node.OccupiedBounds()
			minX = math.Min(minX, topLeft.X)
			minY = math.Min(minY, topLeft.Y)
			maxX = math.Max(maxX, bottomRight.X)
			maxY = math.Max(maxY, bottomRight.Y)
		}
		component.Priority = priority
		component.BBUpLeft = geometry.Point{X: minX, Y: minY}
		component.BBLowRight = geometry.Point{X: maxX, Y: maxY}
	}
	sort.SliceStable(components, func(i, j int) bool {
		if components[i].Priority != components[j].Priority {
			return components[i].Priority > components[j].Priority
		}
		size1 := components[i].BBLowRight.Sub(components[i].BBUpLeft)
		size2 := components[j].BBLowRight.Sub(components[j].BBUpLeft)
		return size1.X*size1.Y < size2.X*size2.Y
	})

	result := copyGraphProperties(components[0])
	maxRowWidth, totalArea := 0.0, 0.0
	for _, component := range components {
		size := component.BBLowRight.Sub(component.BBUpLeft)
		maxRowWidth = math.Max(maxRowWidth, size.X)
		totalArea += size.X * size.Y
	}
	// ELK casts sqrt(totalArea) to float before multiplying by aspect ratio.
	maxRowWidth = math.Max(maxRowWidth, float64(float32(math.Sqrt(totalArea)))*result.AspectRatio)
	spacing := result.SpacingNodeNode
	x, y, highestBox := 0.0, 0.0, 0.0
	for _, component := range components {
		size := component.BBLowRight.Sub(component.BBUpLeft)
		if x+size.X > maxRowWidth {
			x = 0
			y += highestBox + spacing
			highestBox = 0
		}
		moveGraph(result, component, x, y)
		highestBox = math.Max(highestBox, size.Y)
		x += size.X + spacing
	}
	return result
}

func moveGraph(destination, source *FGraph, offsetX, offsetY float64) {
	offset := geometry.Point{X: offsetX, Y: offsetY}.Sub(source.BBUpLeft)
	for _, node := range source.Nodes {
		node.Position = node.Position.Add(offset)
		destination.Nodes = append(destination.Nodes, node)
	}
	for _, edge := range source.Edges {
		for _, bendpoint := range edge.Bendpoints {
			bendpoint.Position = bendpoint.Position.Add(offset)
		}
		destination.Edges = append(destination.Edges, edge)
	}
	for _, label := range source.Labels {
		label.Position = label.Position.Add(offset)
		destination.Labels = append(destination.Labels, label)
	}
}
