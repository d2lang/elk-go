// Copyright (c) 2016 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package stress

import (
	"math"

	"github.com/d2lang/elk-go/internal/force"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
)

// Majorization implements the localized stress-majorization procedure from
// Gansner, Koren, and North exactly as used by ELK 0.8.1.
type Majorization struct {
	model *graph.Model
	graph *force.FGraph

	apsp [][]float64
	w    [][]float64

	desiredEdgeLength float64
	dimension         string
	epsilon           float64
	iterationLimit    int
	connectedEdges    [][]*force.FEdge
}

func (majorization *Majorization) Initialize(model *graph.Model, fgraph *force.FGraph) {
	if len(fgraph.Nodes) <= 1 {
		return
	}
	majorization.model = model
	majorization.graph = fgraph
	parent := model.Node(fgraph.Origin)
	majorization.dimension = graph.Get(&parent.Properties, Dimension)
	majorization.iterationLimit = graph.Get(&parent.Properties, IterationLimit)
	majorization.epsilon = graph.Get(&parent.Properties, Epsilon)
	majorization.desiredEdgeLength = graph.Get(&parent.Properties, DesiredEdgeLength)

	majorization.connectedEdges = make([][]*force.FEdge, len(fgraph.Nodes))
	for _, edge := range fgraph.Edges {
		majorization.connectedEdges[edge.Source.ID] = append(majorization.connectedEdges[edge.Source.ID], edge)
		majorization.connectedEdges[edge.Target.ID] = append(majorization.connectedEdges[edge.Target.ID], edge)
	}

	n := len(fgraph.Nodes)
	majorization.apsp = make([][]float64, n)
	for _, source := range fgraph.Nodes {
		majorization.apsp[source.ID] = make([]float64, n)
		majorization.dijkstra(source, majorization.apsp[source.ID])
	}
	majorization.w = make([][]float64, n)
	for i := 0; i < n; i++ {
		majorization.w[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			distance := majorization.apsp[i][j]
			majorization.w[i][j] = 1 / (distance * distance)
		}
	}
}

func (majorization *Majorization) Execute() {
	if majorization.graph == nil || len(majorization.graph.Nodes) <= 1 {
		return
	}
	count := 0
	previousStress := majorization.computeStress()
	currentStress := math.Inf(1)
	for {
		if count > 0 {
			previousStress = currentStress
		}
		for _, node := range majorization.graph.Nodes {
			if graph.Get(&majorization.model.Node(node.Origin).Properties, Fixed) {
				continue
			}
			node.Position = majorization.computeNewPosition(node)
		}
		currentStress = majorization.computeStress()
		done := previousStress == 0 ||
			(previousStress-currentStress)/previousStress < majorization.epsilon ||
			count >= majorization.iterationLimit
		count++
		if done {
			return
		}
	}
}

func (majorization *Majorization) dijkstra(source *force.FNode, distances []float64) {
	marked := make([]bool, len(majorization.graph.Nodes))
	for _, node := range majorization.graph.Nodes {
		if node.ID == source.ID {
			distances[node.ID] = 0
		} else {
			distances[node.ID] = float64(math.MaxInt32)
		}
	}
	queue := javacompat.NewPriorityQueueCapacity(len(majorization.graph.Nodes), func(a, b *force.FNode) int {
		return javacompat.DoubleCompare(distances[a.ID], distances[b.ID])
	})
	for _, node := range majorization.graph.Nodes {
		queue.Add(node)
	}
	for !queue.IsEmpty() {
		node, _ := queue.Poll()
		marked[node.ID] = true
		for _, edge := range majorization.connectedEdges[node.ID] {
			other := edge.Source
			if other == node {
				other = edge.Target
			}
			if marked[other.ID] {
				continue
			}
			edgeLength := majorization.desiredEdgeLength
			if explicit, found := graph.TryGet(&majorization.model.Edge(edge.Origin).Properties, DesiredEdgeLength); found {
				edgeLength = explicit
			}
			candidate := distances[node.ID] + edgeLength
			if candidate < distances[other.ID] {
				distances[other.ID] = candidate
				queue.RemoveFirst(func(candidate *force.FNode) bool { return candidate == other })
				queue.Add(other)
			}
		}
	}
}

func (majorization *Majorization) computeStress() float64 {
	stress := 0.0
	for i, first := range majorization.graph.Nodes {
		for j := i + 1; j < len(majorization.graph.Nodes); j++ {
			second := majorization.graph.Nodes[j]
			displacement := stressDistance(first.Position, second.Position) - majorization.apsp[first.ID][second.ID]
			// These same-type conversions intentionally force the intermediate
			// rounding performed by GWT's generated JavaScript. Without them Go
			// may fuse the products with the following addition.
			weightedDisplacement := float64(majorization.w[first.ID][second.ID] * displacement)
			term := float64(weightedDisplacement * displacement)
			stress += term
		}
	}
	return stress
}

func (majorization *Majorization) computeNewPosition(node *force.FNode) geometry.Point {
	weightSum, xDisplacement, yDisplacement := 0.0, 0.0, 0.0
	for _, other := range majorization.graph.Nodes {
		if node == other {
			continue
		}
		weight := majorization.w[node.ID][other.ID]
		weightSum += weight
		euclideanDistance := stressDistance(node.Position, other.Position)
		if euclideanDistance > 0 && majorization.dimension != DimensionY {
			xDifference := node.Position.X - other.Position.X
			xScaledDifference := float64(majorization.apsp[node.ID][other.ID] * xDifference)
			xTarget := other.Position.X + xScaledDifference/euclideanDistance
			xContribution := float64(weight * xTarget)
			xDisplacement += xContribution
		}
		if euclideanDistance > 0 && majorization.dimension != DimensionX {
			yDifference := node.Position.Y - other.Position.Y
			yScaledDifference := float64(majorization.apsp[node.ID][other.ID] * yDifference)
			yTarget := other.Position.Y + yScaledDifference/euclideanDistance
			yContribution := float64(weight * yTarget)
			yDisplacement += yContribution
		}
	}
	switch majorization.dimension {
	case DimensionX:
		return geometry.Point{X: xDisplacement / weightSum, Y: node.Position.Y}
	case DimensionY:
		return geometry.Point{X: node.Position.X, Y: yDisplacement / weightSum}
	default:
		return geometry.Point{X: xDisplacement / weightSum, Y: yDisplacement / weightSum}
	}
}

func stressDistance(first, second geometry.Point) float64 {
	dx := first.X - second.X
	dy := first.Y - second.Y
	// Keep the products separately rounded before adding, as KVector.distance
	// does in the generated ELK.js worker.
	dxSquared := float64(dx * dx)
	dySquared := float64(dy * dy)
	return math.Sqrt(dxSquared + dySquared)
}
