// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

// NormalizeSuppliedCoordinates restores the input frames required to lay out
// a previously routed graph. Native loops retain their supplied geometry;
// cross-container edges are routed anew using their nodes, ports and titles.
func (codec *JSONCodec) NormalizeSuppliedCoordinates(model *graph.Model, loops []graph.EdgeID, algorithms *AlgorithmRegistry) error {
	compound := hasSuppliedCompoundEdge(model, algorithms)
	if len(loops) == 0 && !compound {
		return nil
	}
	context, err := codec.nodeCoordinates(model, false)
	if err != nil {
		return err
	}
	// nodeCoordinates gives absolute positions for a laid-out (parent-local)
	// model. Here the input positions may instead be ROOT coordinates. Walk
	// parent-first so mixed, inherited modes yield the actual input origins.
	for i := range model.Nodes {
		node := &model.Nodes[i]
		origin := node.Shape.Position()
		if parent := node.Parent; parent != nil && context.nodeShapeMode[*parent] != "ROOT" {
			origin = origin.Add(context.nodeGlobal[*parent])
		}
		context.nodeGlobal[i] = origin
	}
	if compound {
		normalizeCompoundShapes(model, context)
	}
	for _, id := range loops {
		edge := model.Edge(id)
		owner := model.ShapeNode(edge.Sources[0])
		// Undo the edge's output mode and align the resulting container-local
		// geometry to the owner's original position (itself possibly ROOT).
		delta := codec.edgeOutputOffset(model, context, id).Negate()
		if !compound {
			delta = delta.Add(nodeOutputOffset(model, context, int(owner)))
		}
		if input := codec.edgeJSON[id].JunctionPoints; len(input) != 0 {
			points := make(geometry.Chain, len(input))
			for j, p := range input {
				points[j] = fromJSONPoint(p)
			}
			graph.Set(&edge.Properties, JunctionPoints, points)
		}
		if delta == (geometry.Point{}) {
			continue
		}
		for _, section := range edge.Sections {
			TranslateSection(model, section, delta)
		}
		for _, labelID := range edge.Labels {
			label := &model.Label(labelID).Shape
			if label.Presence&graph.HasX != 0 {
				label.X += delta.X
			}
			if label.Presence&graph.HasY != 0 {
				label.Y += delta.Y
			}
		}
		if points, ok := graph.TryGet(&edge.Properties, JunctionPoints); ok {
			points.Offset(delta)
		}
	}
	return nil
}

// Preserve a missing axis. A freshly added shape or port can omit a position.
func subtractOrigin(shape *graph.Shape, origin geometry.Point) {
	if shape.Presence&graph.HasX != 0 {
		shape.X -= origin.X
	}
	if shape.Presence&graph.HasY != 0 {
		shape.Y -= origin.Y
	}
}

func hasSuppliedCompoundEdge(model *graph.Model, algorithms *AlgorithmRegistry) bool {
	if !graph.Get(&model.Node(model.Root).Properties, RouteCrossContainerEdges) {
		return false
	}
	for i := range model.Edges {
		edge := &model.Edges[i]
		if len(edge.Sections) == 0 {
			continue
		}
		owner, ok := compoundOwner(model, edge)
		if !ok {
			continue
		}
		algorithm := algorithms.BySuffix(graph.Get(&model.Node(owner).Properties, Algorithm))
		if algorithm != nil && supportsCompoundProjection(algorithm.ID) {
			return true
		}
	}
	return false
}

func normalizeCompoundShapes(model *graph.Model, context *coordinateContext) {
	// TransferLayout exports children, ports and node labels in their
	// parent's shapeCoords mode. Restore parent-local geometry so fixed
	// ports and obstacles do not drift on each successive layout.
	for i := range model.Nodes {
		node := &model.Nodes[i]
		subtractOrigin(&node.Shape, nodeOutputOffset(model, context, i))
		for _, id := range node.Ports {
			port := model.Port(id)
			if context.nodeShapeMode[i] == "ROOT" {
				subtractOrigin(&port.Shape, context.nodeGlobal[i])
			}
			mode := resolveCoordinateMode(coordinateMode(&port.Properties, JSONShapeCoords), context.nodeShapeMode[i], "PARENT")
			if mode == "ROOT" {
				origin := context.nodeGlobal[i].Add(port.Shape.Position())
				for _, label := range port.Labels {
					subtractOrigin(&model.Label(label).Shape, origin)
				}
			}
		}
		if context.nodeShapeMode[i] == "ROOT" {
			for _, id := range node.Labels {
				subtractOrigin(&model.Label(id).Shape, context.nodeGlobal[i])
			}
		}
	}
}
