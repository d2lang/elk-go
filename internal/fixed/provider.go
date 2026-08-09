// Copyright (c) 2010, 2019 Kiel University and others.
// Copyright (c) 2015, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package fixed implements org.eclipse.elk.fixed from ELK 0.8.1.
package fixed

import (
	"fmt"
	"math"

	"github.com/d2lang/elk-go/internal/basicoptions"
	"github.com/d2lang/elk-go/internal/common/elkutil"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

type Provider struct{}

func Register(registry *core.AlgorithmRegistry) error {
	return registry.Register(core.AlgorithmData{
		ID:     "org.eclipse.elk.fixed",
		Create: func() core.LayoutProvider { return &Provider{} },
		KnownOptions: []string{
			basicoptions.FixedPadding.ID,
			basicoptions.Position.ID,
			basicoptions.BendPoints.ID,
			core.NodeSizeConstraints.ID,
			core.NodeSizeMinimum.ID,
			basicoptions.NodeSizeFixedGraphSize.ID,
		},
	})
}

func (*Provider) Layout(model *graph.Model, parent graph.NodeID, _ core.ProgressMonitor) error {
	parentNode := model.Node(parent)
	edgeRouting := graph.Get(&parentNode.Properties, basicoptions.EdgeRouting)
	maxX, maxY := 0.0, 0.0

	for _, nodeID := range parentNode.Children {
		node := model.Node(nodeID)
		if position, found := graph.TryGet(&node.Properties, basicoptions.Position); found {
			node.Shape.SetLocation(position.X, position.Y)
			if contains(graph.Get(&node.Properties, core.NodeSizeConstraints), "MINIMUM_SIZE") {
				minimum := graph.Get(&node.Properties, core.NodeSizeMinimum)
				if minimum.X > 0 && minimum.Y > 0 {
					elkutil.ResizeNode(model, nodeID, minimum.X, minimum.Y, true, true)
				}
			}
		}
		maxX = math.Max(maxX, node.Shape.X+node.Shape.Width)
		maxY = math.Max(maxY, node.Shape.Y+node.Shape.Height)

		for _, labelID := range node.Labels {
			label := model.Label(labelID)
			applyPosition(&label.Shape, &label.Properties)
			maxX = math.Max(maxX, node.Shape.X+label.Shape.X+label.Shape.Width)
			maxY = math.Max(maxY, node.Shape.Y+label.Shape.Y+label.Shape.Height)
		}

		for _, portID := range node.Ports {
			port := model.Port(portID)
			applyPosition(&port.Shape, &port.Properties)
			portX := node.Shape.X + port.Shape.X
			portY := node.Shape.Y + port.Shape.Y
			maxX = math.Max(maxX, portX+port.Shape.Width)
			maxY = math.Max(maxY, portY+port.Shape.Height)
			for _, labelID := range port.Labels {
				label := model.Label(labelID)
				applyPosition(&label.Shape, &label.Properties)
				maxX = math.Max(maxX, portX+label.Shape.X+label.Shape.Width)
				maxY = math.Max(maxY, portY+label.Shape.Y+label.Shape.Height)
			}
		}

		for _, edgeID := range elkutil.AllOutgoingEdges(model, nodeID) {
			maximum, err := processEdge(model, edgeID)
			if err != nil {
				return err
			}
			maxX = math.Max(maxX, maximum.X)
			maxY = math.Max(maxY, maximum.Y)
		}
		for _, edgeID := range elkutil.AllIncomingEdges(model, nodeID) {
			edge := model.Edge(edgeID)
			if len(edge.Sources) == 0 {
				continue
			}
			source := model.ShapeNode(edge.Sources[0])
			sourceParent := model.Node(source).Parent
			if sourceParent == nil || *sourceParent != parent {
				maximum, err := processEdge(model, edgeID)
				if err != nil {
					return err
				}
				maxX = math.Max(maxX, maximum.X)
				maxY = math.Max(maxY, maximum.Y)
			}
		}
	}

	if edgeRouting == "ORTHOGONAL" {
		for _, nodeID := range parentNode.Children {
			for _, edgeID := range elkutil.AllOutgoingEdges(model, nodeID) {
				points, err := elkutil.DetermineJunctionPoints(model, edgeID)
				if err != nil {
					return err
				}
				if len(points) == 0 {
					graph.Unset(&model.Edge(edgeID).Properties, core.JunctionPoints)
				} else {
					graph.Set(&model.Edge(edgeID).Properties, core.JunctionPoints, points)
				}
			}
		}
	}

	if !graph.Get(&parentNode.Properties, basicoptions.NodeSizeFixedGraphSize) {
		padding := graph.Get(&parentNode.Properties, basicoptions.FixedPadding)
		elkutil.ResizeNode(model, parent, maxX+padding.Left+padding.Right, maxY+padding.Top+padding.Bottom, true, true)
	}
	return nil
}

func applyPosition(shape *graph.Shape, properties *graph.PropertyMap) {
	if position, found := graph.TryGet(properties, basicoptions.Position); found {
		shape.SetLocation(position.X, position.Y)
	}
}

func processEdge(model *graph.Model, edgeID graph.EdgeID) (geometry.Point, error) {
	edge := model.Edge(edgeID)
	if len(edge.Sources) == 0 || len(edge.Targets) == 0 {
		return geometry.Point{}, nil
	}
	sourceParent := model.Node(model.ShapeNode(edge.Sources[0])).Parent
	targetParent := model.Node(model.ShapeNode(edge.Targets[0])).Parent
	sameHierarchy := sourceParent != nil && targetParent != nil && *sourceParent == *targetParent
	maximum := geometry.Point{}

	if bendPoints, found := graph.TryGet(&edge.Properties, basicoptions.BendPoints); found && len(bendPoints) >= 2 {
		if len(edge.Sections) == 0 {
			model.AddSection(edgeID)
		} else if len(edge.Sections) > 1 {
			// The 0.8.1 source attempts ListIterator.remove before next,
			// which raises IllegalStateException for this invalid state.
			return geometry.Point{}, fmt.Errorf("fixed edge %q has multiple sections", edge.Identifier)
		}
		elkutil.ApplyVectorChain(model, edge.Sections[0], bendPoints)
	}
	if sameHierarchy {
		for _, sectionID := range edge.Sections {
			for _, point := range model.Section(sectionID).BendPoints {
				maximum.X = math.Max(maximum.X, point.X)
				maximum.Y = math.Max(maximum.Y, point.Y)
			}
		}
	}
	for _, labelID := range edge.Labels {
		label := model.Label(labelID)
		applyPosition(&label.Shape, &label.Properties)
		if sameHierarchy {
			maximum.X = math.Max(maximum.X, label.Shape.X+label.Shape.Width)
			maximum.Y = math.Max(maximum.Y, label.Shape.Y+label.Shape.Height)
		}
	}
	return maximum, nil
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
