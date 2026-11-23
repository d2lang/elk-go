// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package selfloops

import (
	"math"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

// Retain supplied section topology and route geometry, including sections
// produced by a previous Layout call. Their owner can move during placement.
type existingRoutes struct {
	labelBands [4]labelBand
	edges      []graph.EdgeID
	owner      geometry.Point
	min, max   geometry.Point
}

func (p *Plan) retainExisting(model *graph.Model, edgeID graph.EdgeID, owner geometry.Point) {
	r := &p.existing
	r.owner = owner
	r.edges = append(r.edges, edgeID)
	include := func(point geometry.Point) {
		local := point.Sub(owner)
		r.min.X, r.min.Y = math.Min(r.min.X, local.X), math.Min(r.min.Y, local.Y)
		r.max.X, r.max.Y = math.Max(r.max.X, local.X), math.Max(r.max.Y, local.Y)
	}
	edge := model.Edge(edgeID)
	for _, id := range edge.Sections {
		section := model.Section(id)
		if section.StartPresent {
			include(section.Start)
		}
		for _, point := range section.BendPoints {
			include(point)
		}
		if section.EndPresent {
			include(section.End)
		}
	}
	for _, id := range edge.Labels {
		label := model.Label(id).Shape
		if label.Presence&(graph.HasX|graph.HasY) != 0 {
			include(label.Position())
			include(label.Rectangle().BottomRight())
			p.includeLabel(&r.labelBands, label.Rectangle().Move(geometry.Point{X: -owner.X, Y: -owner.Y}))
		}
	}
	if junctions, ok := graph.TryGet(&edge.Properties, core.JunctionPoints); ok {
		for _, point := range junctions {
			include(point)
		}
	}
}

// TranslateExisting keeps supplied routes attached when a provider changes a
// node position, including during the intermediate Force layout used by Stress.
func (p *Plan) TranslateExisting(model *graph.Model, x, y float64) {
	r := &p.existing
	to := geometry.Point{X: x, Y: y}
	if len(r.edges) == 0 || to == r.owner {
		return
	}
	delta := to.Sub(r.owner)
	for _, id := range r.edges {
		edge := model.Edge(id)
		for _, section := range edge.Sections {
			core.TranslateSection(model, section, delta)
		}
		for _, labelID := range edge.Labels {
			label := &model.Label(labelID).Shape
			if label.Presence&(graph.HasX|graph.HasY) != 0 {
				label.SetLocation(label.X+delta.X, label.Y+delta.Y)
			}
		}
		if junctions, ok := graph.TryGet(&edge.Properties, core.JunctionPoints); ok {
			graph.Set(&edge.Properties, core.JunctionPoints, junctions.Offset(delta))
		}
	}
	r.owner = to
}
