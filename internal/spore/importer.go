// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package spore

import (
	"fmt"
	"math"

	"github.com/d2lang/elk-go/internal/common/elkutil"
	"github.com/d2lang/elk-go/internal/common/nodespacing"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
)

type importer struct {
	model   *graph.Model
	parent  graph.NodeID
	graph   *sporeGraph
	spacing float64
}

func newImporter(model *graph.Model, parent graph.NodeID) *importer {
	return &importer{model: model, parent: parent}
}

func (i *importer) importGraph() (*sporeGraph, error) {
	parentNode := i.model.Node(i.parent)
	nodespacing.NewNodeMarginCalculator().Process(i.model, i.parent)

	i.spacing = graph.Get(&parentNode.Properties, SpacingNodeNode)
	g := &sporeGraph{
		treeConstruction: graph.Get(&parentNode.Properties, TreeConstruction),
		compaction:       graph.Get(&parentNode.Properties, Compaction),
		orthogonal:       graph.Get(&parentNode.Properties, Orthogonal),
	}
	i.graph = g

	jitter := javacompat.NewRandom(1)
	for _, nodeID := range parentNode.Children {
		elkNode := i.model.Node(nodeID)
		halfWidth, halfHeight := elkNode.Shape.Width/2, elkNode.Shape.Height/2
		point := geometry.Point{X: elkNode.Shape.X + halfWidth, Y: elkNode.Shape.Y + halfHeight}
		for i.containsPoint(point) {
			point.X += (jitter.NextDouble() - 0.5) * 0.001
			point.Y += (jitter.NextDouble() - 0.5) * 0.001
		}
		original := &vertex{point: point}
		margin := graph.Get(&elkNode.Properties, nodespacing.Margins)
		rect := geometry.Rectangle{
			X:      point.X - halfWidth - i.spacing/2 - margin.Left,
			Y:      point.Y - halfHeight - i.spacing/2 - margin.Top,
			Width:  elkNode.Shape.Width + i.spacing + margin.Horizontal(),
			Height: elkNode.Shape.Height + i.spacing + margin.Vertical(),
		}
		g.vertices = append(g.vertices, newNode(original, rect, nodeID))
	}
	if len(g.vertices) == 0 {
		return g, nil
	}

	switch graph.Get(&parentNode.Properties, RootSelection) {
	case RootFixed:
		preferred, present := graph.TryGet(&parentNode.Properties, PreferredRoot)
		if !present {
			g.preferredRoot = g.vertices[0]
		} else {
			for _, candidate := range g.vertices {
				if i.model.Node(candidate.origin).IdentifierPresent && i.model.Node(candidate.origin).Identifier == preferred {
					g.preferredRoot = candidate
				}
			}
		}
	case RootCenter:
		center := geometry.Point{
			X: parentNode.Shape.X + parentNode.Shape.Width/2,
			Y: parentNode.Shape.Y + parentNode.Shape.Height/2,
		}
		closest := math.Inf(1)
		for _, candidate := range g.vertices {
			distance := candidate.original.point.Distance(center)
			if distance < closest {
				closest = distance
				g.preferredRoot = candidate
			}
		}
	default:
		return nil, fmt.Errorf("no SPOrE root selection implementation for %q", graph.Get(&parentNode.Properties, RootSelection))
	}

	switch graph.Get(&parentNode.Properties, SpanningTreeCost) {
	case CostCenterDistance:
		g.cost = func(edge *tEdge) float64 { return edge.u.point.Distance(edge.v.point) }
	case CostCircleUnderlap:
		g.cost = func(edge *tEdge) float64 {
			first, second := i.nodeFor(edge.u), i.nodeFor(edge.v)
			return edge.u.point.Distance(edge.v.point) -
				edge.u.point.Distance(first.rect.Position()) -
				edge.v.point.Distance(second.rect.Position())
		}
	case CostRectangleUnderlap:
		g.cost = func(edge *tEdge) float64 { return i.nodeFor(edge.u).underlap(i.nodeFor(edge.v)) }
	case CostInvertedOverlap:
		g.cost = func(edge *tEdge) float64 {
			first, second := i.nodeFor(edge.u), i.nodeFor(edge.v)
			distance := shortestDistance(first.rect, second.rect)
			if distance >= 0 {
				return distance
			}
			centerDistance := second.rect.Center().Sub(first.rect.Center()).Length()
			return -(overlap(first.rect, second.rect) - 1) * centerDistance
		}
	case CostMinimumRootDistance:
		g.cost = func(edge *tEdge) float64 {
			return math.Min(
				edge.u.point.Distance(g.preferredRoot.vertex),
				edge.v.point.Distance(g.preferredRoot.vertex),
			)
		}
	default:
		return nil, fmt.Errorf("no SPOrE spanning-tree cost implementation for %q", graph.Get(&parentNode.Properties, SpanningTreeCost))
	}
	return g, nil
}

func (i *importer) containsPoint(point geometry.Point) bool {
	for _, candidate := range i.graph.vertices {
		if candidate.original.point == point {
			return true
		}
	}
	return false
}

func (i *importer) nodeFor(wanted *vertex) *node {
	for _, candidate := range i.graph.vertices {
		if candidate.original == wanted {
			return candidate
		}
	}
	return nil
}

func (i *importer) updateGraph() {
	i.graph.edges = nil
	i.graph.tree = nil
	for _, candidate := range i.graph.vertices {
		candidate.original = &vertex{point: candidate.rect.Center()}
	}
}

func (i *importer) applyPositions() {
	if len(i.graph.vertices) == 0 {
		return
	}
	minimumX, minimumY := math.Inf(1), math.Inf(1)
	maximumX, maximumY := math.Inf(-1), math.Inf(-1)
	for _, candidate := range i.graph.vertices {
		elkNode := i.model.Node(candidate.origin)
		elkNode.Shape.SetLocation(candidate.rect.X, candidate.rect.Y)
		minimumX = math.Min(minimumX, elkNode.Shape.X)
		minimumY = math.Min(minimumY, elkNode.Shape.Y)
		maximumX = math.Max(maximumX, elkNode.Shape.X+elkNode.Shape.Width)
		maximumY = math.Max(maximumY, elkNode.Shape.Y+elkNode.Shape.Height)
	}

	padding := graph.Get(&i.model.Node(i.parent).Properties, Padding)
	elkutil.ResizeNode(
		i.model,
		i.parent,
		maximumX-minimumX+padding.Horizontal(),
		maximumY-minimumY+padding.Vertical(),
		true,
		true,
	)
	elkutil.Translate(i.model, i.parent, -minimumX+padding.Left, -minimumY+padding.Top)
	i.routeEdges()
}

func (i *importer) routeEdges() {
	for _, edgeID := range i.model.Node(i.parent).ContainedEdges {
		edge := i.model.Edge(edgeID)
		if len(edge.Sources) == 0 || len(edge.Targets) == 0 {
			continue
		}
		if len(edge.Sections) == 0 {
			i.model.AddSection(edgeID)
		}
		section := i.model.Section(edge.Sections[0])
		source := i.model.Node(i.model.ShapeNode(edge.Sources[0]))
		target := i.model.Node(i.model.ShapeNode(edge.Targets[0]))
		start := source.Shape.Rectangle().Center()
		end := target.Shape.Rectangle().Center()
		start = start.Add(clipVector(end.Sub(start), source.Shape.Width, source.Shape.Height))
		end = end.Add(clipVector(start.Sub(end), target.Shape.Width, target.Shape.Height))
		section.Start, section.End = start, end
		section.StartPresent, section.EndPresent = true, true
	}
}

func runUnderlying(model *graph.Model, parent graph.NodeID, monitor core.ProgressMonitor, algorithms *core.AlgorithmRegistry) error {
	properties := &model.Node(parent).Properties
	requested, present := graph.TryGet(properties, UnderlyingLayoutAlgorithm)
	if !present || algorithms == nil {
		return nil
	}
	algorithm := algorithms.BySuffix(requested)
	if algorithm == nil {
		return nil
	}
	provider := algorithm.Create()
	if provider == nil {
		return fmt.Errorf("underlying layout algorithm %q returned a nil provider", requested)
	}
	return provider.Layout(model, parent, monitor.Subtask(1))
}
