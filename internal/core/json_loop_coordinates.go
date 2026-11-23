// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

// NormalizeLoopCoordinates expresses supplied routes and labels in the same
// frame as their imported owner's position. Providers preserve that relative
// geometry when they place the node. Other shapes and edges retain ELK's
// ordinary input semantics.
func (codec *JSONCodec) NormalizeLoopCoordinates(model *graph.Model, loops []graph.EdgeID) error {
	if len(loops) == 0 {
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
	for _, id := range loops {
		edge := model.Edge(id)
		owner := model.ShapeNode(edge.Sources[0])
		// Undo the edge's output mode and align the resulting container-local
		// geometry to the owner's original position (itself possibly ROOT).
		delta := nodeOutputOffset(model, context, int(owner)).Sub(codec.edgeOutputOffset(model, context, id))
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
