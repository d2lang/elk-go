// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package selfloops

import (
	"math"

	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

// IntersectsBox checks the actual loop routes and label bounds against a
// node-local obstacle. Unoccupied space inside the plan's margins is allowed.
func (p *Plan) IntersectsBox(model *graph.Model, box geometry.Rectangle) bool {
	for _, l := range p.loops {
		label := geometry.Rectangle{X: l.center.X - l.labelWidth/2, Y: l.center.Y - l.labelHeight/2, Width: l.labelWidth, Height: l.labelHeight}
		if l.labelWidth > 0 && l.labelHeight > 0 && box.Intersects(label) {
			return true
		}
		for i := 1; i < len(l.path); i++ {
			if entersBox(l.path[i-1], l.path[i], box) {
				return true
			}
		}
	}
	// Supplied routes are stored in parent coordinates.
	box = box.Move(p.existing.owner)
	for _, id := range p.existing.edges {
		edge := model.Edge(id)
		for _, sectionID := range edge.Sections {
			section := model.Section(sectionID)
			prev, havePrev := section.Start, section.StartPresent
			for _, point := range section.BendPoints {
				if havePrev && entersBox(prev, point, box) {
					return true
				}
				prev, havePrev = point, true
			}
			if havePrev && section.EndPresent && entersBox(prev, section.End, box) {
				return true
			}
		}
		for _, labelID := range edge.Labels {
			label := &model.Label(labelID).Shape
			if label.Presence&(graph.HasX|graph.HasY) != 0 && label.Width > 0 && label.Height > 0 && box.Intersects(label.Rectangle()) {
				return true
			}
		}
	}
	return false
}

// Clip the segment's parameter interval against the open box. Border contact
// is allowed; imported sections may contain diagonal segments.
func entersBox(start, end geometry.Point, box geometry.Rectangle) bool {
	enter, exit := 0., 1.
	axis := func(a, b, lo, hi float64) bool {
		if a == b {
			return a > lo && a < hi
		}
		t0, t1 := (lo-a)/(b-a), (hi-a)/(b-a)
		enter = math.Max(enter, math.Min(t0, t1))
		exit = math.Min(exit, math.Max(t0, t1))
		return enter < exit
	}
	return axis(start.X, end.X, box.X, box.MaxX()) && axis(start.Y, end.Y, box.Y, box.MaxY())
}
