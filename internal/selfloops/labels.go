// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package selfloops

import (
	"math"
	"slices"

	"github.com/d2lang/elk-go/internal/geometry"
)

// On each side, remember the span of previously placed labels along the face
// and how far an arm can leave the node before reaching that span.
type labelBand struct {
	used              bool
	near, first, last float64
}

func (b *labelBand) add(near, first, last float64) {
	if near < 0 {
		return
	}
	if !b.used {
		b.used, b.near, b.first, b.last = true, near, first, last
		return
	}
	b.near, b.first, b.last = math.Min(b.near, near), math.Min(b.first, first), math.Max(b.last, last)
}

func (p *Plan) includeLabel(bands *[4]labelBand, box geometry.Rectangle) {
	if box.Width <= 0 || box.Height <= 0 {
		return
	}
	bands[North].add(-box.MaxY(), box.X, box.MaxX())
	bands[East].add(box.X-p.width, box.Y, box.MaxY())
	bands[South].add(box.Y-p.height, box.X, box.MaxX())
	bands[West].add(-box.MaxX(), box.Y, box.MaxY())
}

// A more distant lane can still cross an inner label on the way out or
// back. Turn outward along the face before the first label, then go around
// the ends of the label band. Symmetric widening keeps the route midpoint
// on the far segment for an inline label.
func (p *Plan) clearInnerLabels(path geometry.Chain, side Side, band labelBand) geometry.Chain {
	if !band.used {
		return path
	}
	tangent := func(v geometry.Point) float64 {
		if side == North || side == South {
			return v.X
		}
		return v.Y
	}
	point := func(t, outward float64) geometry.Point {
		switch side {
		case North:
			return geometry.Point{X: t, Y: -outward}
		case East:
			return geometry.Point{X: p.width + outward, Y: t}
		case South:
			return geometry.Point{X: t, Y: p.height + outward}
		default:
			return geometry.Point{X: -outward, Y: t}
		}
	}
	start, end := path[0], path[len(path)-1]
	from, to := tangent(start), tangent(end)
	left, right := band.first-p.laneGap, band.last+p.laneGap
	if (from <= left || from >= right) && (to <= left || to >= right) {
		return path
	}
	grow := math.Max(math.Min(from, to)-left, right-math.Max(from, to))
	if grow <= 0 {
		return path
	}
	delta := grow
	if from > to {
		delta = -grow
	}
	lead := math.Min(band.near/2, p.laneGap)
	outer := path[1]
	var reach float64
	switch side {
	case North:
		reach = -outer.Y
	case East:
		reach = outer.X - p.width
	case South:
		reach = outer.Y - p.height
	case West:
		reach = -outer.X
	}
	return slices.Compact(geometry.Chain{
		start, point(from, lead), point(from-delta, lead), point(from-delta, reach),
		point(to+delta, reach), point(to+delta, lead), point(to, lead), end,
	})
}
