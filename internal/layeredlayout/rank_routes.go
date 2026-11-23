// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package layeredlayout

import (
	"container/heap"
	"fmt"
	"math"
	"slices"
	"sort"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

type attachment struct{ point, normal geometry.Point }

func (p *rankLayout) attachment(id graph.ShapeID, other graph.NodeID, flat bool, lane float64) attachment {
	m := p.m
	box := p.box[m.ShapeNode(id)]
	point := box.Center()
	v := p.box[other].Center().Sub(point)
	xSide := p.vertical == flat
	if id.Kind == graph.PortShape {
		port := m.Port(id.Port)
		point = port.Shape.Rectangle().Center().Add(box.Position())
		// An explicitly positioned port belongs to its side, even when
		// the side faces away from its same-rank neighbor.
		side, _ := port.Properties.Raw("org.eclipse.elk.port.side")
		switch side {
		case "NORTH":
			return attachment{point, geometry.NewPoint(0, -1)}
		case "SOUTH":
			return attachment{point, geometry.NewPoint(0, 1)}
		case "EAST":
			return attachment{point, geometry.NewPoint(1, 0)}
		case "WEST":
			return attachment{point, geometry.NewPoint(-1, 0)}
		}
		v = point.Sub(box.Center())
		xSide = math.Min(math.Abs(point.X-box.X), math.Abs(point.X-box.MaxX())) <= math.Min(math.Abs(point.Y-box.Y), math.Abs(point.Y-box.MaxY()))
	}
	if xSide {
		d := math.Copysign(1, v.X)
		if id.Kind == graph.NodeShape {
			point.X += d * box.Width / 2
			point.Y += lane * box.Height
		}
		return attachment{point, geometry.NewPoint(d, 0)}
	}
	d := math.Copysign(1, v.Y)
	if id.Kind == graph.NodeShape {
		point.Y += d * box.Height / 2
		point.X += lane * box.Width
	}
	return attachment{point, geometry.NewPoint(0, d)}
}

// Give distinct free edges their own border attachment when an earlier route
// has already used the preferred stub. Fixed ports keep their declared point.
func (p *rankLayout) freeAttachment(id graph.ShapeID, other graph.NodeID, flat bool, lane float64, used []trackSegment) attachment {
	preferred := p.attachment(id, other, flat, lane)
	if id.Kind != graph.NodeShape || !segmentOnTrack(preferred.point, preferred.point.Add(preferred.normal.Scale(5)), used) {
		return preferred
	}
	toward := p.box[other].Center().Sub(preferred.point)
	across := toward.X
	if preferred.normal.X != 0 {
		across = toward.Y
	}
	sign := math.Copysign(1, across)
	for _, fraction := range []float64{.18, -.18, .32, -.32, .10, -.10, .40, -.40} {
		v := p.attachment(id, other, flat, sign*fraction)
		if !segmentOnTrack(v.point, v.point.Add(v.normal.Scale(5)), used) {
			return v
		}
	}
	return preferred
}

func (p *rankLayout) obstacles(a, b graph.NodeID) []geometry.Rectangle {
	// Ancestors of either endpoint are permeable; the outermost rectangle
	// of every other subtree is an obstacle, including an intact container.
	open := map[graph.NodeID]bool{}
	for _, id := range []graph.NodeID{a, b} {
		for par := p.m.Node(id).Parent; par != nil; par = p.m.Node(*par).Parent {
			open[*par] = true
		}
	}
	var boxes []geometry.Rectangle
	var visit func(graph.NodeID)
	visit = func(id graph.NodeID) {
		if !open[id] {
			boxes = append(boxes, p.box[id])
			return
		}
		for _, child := range p.m.Node(id).Children {
			visit(child)
		}
	}
	visit(p.m.Root)
	return boxes
}

// Transform a native route in its original container coordinates to its new
// world position. A loop on an expanded compound must follow both moved
// borders; points beyond a border retain their original routing clearance.
func (p *rankLayout) movedPoint(id, parent graph.NodeID, pt geometry.Point) geometry.Point {
	old, box := p.old[id], p.box[id]
	pt = pt.Add(p.old[parent].Position()).Sub(old.Position())
	scale := func(v, before, after float64) float64 {
		if v <= 0 {
			return v
		}
		if v >= before {
			return after + v - before
		}
		return v * after / before
	}
	return box.Position().Add(geometry.NewPoint(scale(pt.X, old.Width, box.Width), scale(pt.Y, old.Height, box.Height)))
}

func (p *rankLayout) route(monitor core.ProgressMonitor) error {
	m := p.m
	pair := func(e *graph.Edge) [2]graph.NodeID {
		a, b := m.ShapeNode(e.Sources[0]), m.ShapeNode(e.Targets[0])
		return [2]graph.NodeID{min(a, b), max(a, b)}
	}
	counts := map[[2]graph.NodeID]int{}
	seen := map[[2]graph.NodeID]int{}
	for i := range m.Edges {
		e := &m.Edges[i]
		if len(e.Sources) == 1 && len(e.Targets) == 1 {
			counts[pair(e)]++
		}
	}
	var labels []geometry.Rectangle
	// Loop labels move with their native routes and already occupy a
	// reserved footprint. Make them obstacles independent of edge order.
	for _, e := range m.Edges {
		if len(e.Sources) != 1 || len(e.Targets) != 1 {
			continue
		}
		a, b := m.ShapeNode(e.Sources[0]), m.ShapeNode(e.Targets[0])
		if a != b && (p.expanded[a] || p.expanded[b] || p.owner[a] != p.owner[b]) {
			continue
		}
		parent := m.Root
		if e.ContainingNode != nil {
			parent = *e.ContainingNode
		}
		for _, id := range e.Labels {
			r := m.Label(id).Shape.Rectangle()
			at := p.movedPoint(a, parent, r.Position())
			r.X, r.Y = at.X, at.Y
			labels = append(labels, r)
		}
	}
	var used []trackSegment
	type routedLabel struct {
		id        graph.LabelID
		path      []geometry.Point
		offset    geometry.Point
		obstacles []geometry.Rectangle
	}
	var pending []routedLabel
	bounds := p.box[m.Root]
	for i := range m.Edges {
		if monitor.Canceled() {
			return fmt.Errorf("d2.rankGroups: layout canceled")
		}
		e := &m.Edges[i]
		if len(e.Sources) != 1 || len(e.Targets) != 1 {
			return fmt.Errorf("d2.rankGroups: edge %q requires one source and one target", e.Identifier)
		}
		a, b := m.ShapeNode(e.Sources[0]), m.ShapeNode(e.Targets[0])
		parent := m.Root
		if e.ContainingNode != nil {
			parent = *e.ContainingNode
		}
		offset := p.box[parent].Position()
		if a == b || (!p.expanded[a] && !p.expanded[b] && p.owner[a] == p.owner[b]) {
			// ELK has already routed this loop or intact subtree. Its
			// container may lie outside the moved block.
			move := func(pt geometry.Point) geometry.Point { return p.movedPoint(a, parent, pt).Sub(offset) }
			for _, id := range e.Sections {
				s := m.Section(id)
				s.Start = move(s.Start)
				s.End = move(s.End)
				for j := range s.BendPoints {
					s.BendPoints[j] = move(s.BendPoints[j])
				}
			}
			for _, id := range e.Labels {
				l := m.Label(id)
				pt := move(l.Shape.Position())
				l.Shape.SetLocation(pt.X, pt.Y)
				bounds = bounds.Union(l.Shape.Rectangle().Move(offset))
			}
		} else {
			key := pair(e)
			n := counts[key]
			k := seen[key]
			seen[key]++
			lane := float64(2*k+1-n) / float64(n) * 0.35
			flat := math.Abs(p.axis(p.box[a]).Center().X-p.axis(p.box[b]).Center().X) < 1e-7
			start, end := p.freeAttachment(e.Sources[0], b, flat, lane, used), p.freeAttachment(e.Targets[0], a, flat, lane, used)
			obstacles := p.obstacles(a, b)
			// The initial port stubs force a legal exit/entry direction.
			// Shorten them when neighbors are tightly spaced.
			distance := math.Min(5, start.point.Distance(end.point)/4)
			s, t := start.point.Add(start.normal.Scale(distance)), end.point.Add(end.normal.Scale(distance))
			if flat && e.Sources[0].Kind == graph.NodeShape && e.Targets[0].Kind == graph.NodeShape {
				// Composing parent-relative offsets can introduce a fraction of
				// an ulp of difference between otherwise collinear ports.
				if math.Abs(s.X-t.X) < 1e-7 {
					t.X = s.X
					end.point.X = s.X
				}
				if math.Abs(s.Y-t.Y) < 1e-7 {
					t.Y = s.Y
					end.point.Y = s.Y
				}
			}
			horizontal := start.normal.X != 0 && end.normal.X != 0
			path := orthogonalRoute(s, t, horizontal, obstacles, used, monitor)
			freePorts := e.Sources[0].Kind == graph.NodeShape && e.Targets[0].Kind == graph.NodeShape
			conflict := false
			if flat && freePorts && len(path) > 0 {
				initial := append([]geometry.Point{start.point}, path...)
				conflict = overlapsTrack(append(initial, end.point), used)
			}
			if flat && freePorts && (k > 0 || conflict) {
				space := 10.
				for _, id := range e.Labels {
					space = max(space, p.axis(m.Label(id).Shape.Rectangle()).Width+8)
				}
				if detour := p.flatTrack(a, b, k, space, obstacles, used); detour != nil {
					start.point, end.point = detour[0], detour[3]
					path = detour[1:3]
				}
			}
			if path == nil && len(used) > 0 && !monitor.Canceled() {
				// Coincident fixed ports can require a shared stub.
				path = orthogonalRoute(s, t, horizontal, obstacles, nil, monitor)
			}
			if path == nil {
				if monitor.Canceled() {
					return fmt.Errorf("d2.rankGroups: layout canceled")
				}
				return fmt.Errorf("d2.rankGroups: no free orthogonal route for edge %q", e.Identifier)
			}
			points := append([]geometry.Point{start.point}, path...)
			points = compactRoute(append(points, end.point))
			for j := 1; j < len(points); j++ {
				used = append(used, trackSegment{points[j-1], points[j]})
			}
			if len(points) == 1 {
				points = append(points, points[0])
			}
			var sid graph.SectionID
			if len(e.Sections) > 0 {
				sid = e.Sections[0]
				e.Sections = e.Sections[:1]
			} else {
				sid = m.AddSection(graph.EdgeID(i))
				section := m.Section(sid)
				section.Identifier = fmt.Sprintf("%s_s0", e.Identifier)
				section.IdentifierPresent = true
				section.IncomingShape = &e.Sources[0]
				section.OutgoingShape = &e.Targets[0]
			}
			section := m.Section(sid)
			section.Start, section.End = points[0].Sub(offset), points[len(points)-1].Sub(offset)
			section.StartPresent, section.EndPresent = true, true
			section.BendPoints = section.BendPoints[:0]
			for _, pt := range points[1 : len(points)-1] {
				section.BendPoints = append(section.BendPoints, pt.Sub(offset))
			}
			section.BendPointsPresent = len(section.BendPoints) > 0
			for _, id := range e.Labels {
				pending = append(pending, routedLabel{id, points, offset, obstacles})
			}
			e.Properties.Delete(core.JunctionPoints.ID)
		}
		for _, sid := range e.Sections {
			s := m.Section(sid)
			for _, pt := range s.BendPoints {
				v := pt.Add(offset)
				bounds = bounds.Union(geometry.Rectangle{X: v.X, Y: v.Y})
			}
		}
	}
	// Place edge labels after every track has been chosen. A label crossing a
	// different edge erases that edge behind its text in the final drawing.
	for _, seg := range used {
		labels = append(labels, geometry.Rectangle{X: min(seg.a.X, seg.b.X) - 1, Y: min(seg.a.Y, seg.b.Y) - 1,
			Width: math.Abs(seg.a.X-seg.b.X) + 2, Height: math.Abs(seg.a.Y-seg.b.Y) + 2})
	}
	for _, item := range pending {
		l := m.Label(item.id)
		r := placeRankLabel(item.path, l.Shape.Width, l.Shape.Height, item.obstacles, labels)
		labels = append(labels, r)
		l.Shape.SetLocation(r.X-item.offset.X, r.Y-item.offset.Y)
		bounds = bounds.Union(r)
	}
	// Exterior routing tracks and labels are part of the drawing. Move the
	// root coordinate origin when they extend to the left or above it.
	d := bounds.Position().Negate()
	for _, child := range m.Node(m.Root).Children {
		s := &m.Node(child).Shape
		s.X += d.X
		s.Y += d.Y
	}
	for i := range m.Edges {
		e := &m.Edges[i]
		if e.ContainingNode != nil && *e.ContainingNode != m.Root {
			continue
		}
		for _, sid := range e.Sections {
			s := m.Section(sid)
			s.Start = s.Start.Add(d)
			s.End = s.End.Add(d)
			for j := range s.BendPoints {
				s.BendPoints[j] = s.BendPoints[j].Add(d)
			}
		}
		for _, id := range e.Labels {
			l := m.Label(id)
			l.Shape.X += d.X
			l.Shape.Y += d.Y
		}
	}
	m.Node(m.Root).Shape.SetDimensions(bounds.Width, bounds.Height)
	return nil
}

type trackSegment struct{ a, b geometry.Point }

func overlapsTrack(path []geometry.Point, used []trackSegment) bool {
	for i := 1; i < len(path); i++ {
		if segmentOnTrack(path[i-1], path[i], used) {
			return true
		}
	}
	return false
}

func segmentOnTrack(a, b geometry.Point, used []trackSegment) bool {
	const eps = 1e-7
	for _, s := range used {
		if math.Abs(a.Y-b.Y) < eps && math.Abs(a.Y-s.a.Y) < eps && math.Abs(a.Y-s.b.Y) < eps &&
			min(max(a.X, b.X), max(s.a.X, s.b.X))-max(min(a.X, b.X), min(s.a.X, s.b.X)) > eps {
			return true
		}
		if math.Abs(a.X-b.X) < eps && math.Abs(a.X-s.a.X) < eps && math.Abs(a.X-s.b.X) < eps &&
			min(max(a.Y, b.Y), max(s.a.Y, s.b.Y))-max(min(a.Y, b.Y), min(s.a.Y, s.b.Y)) > eps {
			return true
		}
	}
	return false
}

// Route a flat link around its rank only when its ordinary corridor is
// already occupied. The same reservation handles parallel links and links
// between distinct pairs which span the same intermediate nodes.
func (p *rankLayout) flatTrack(a, b graph.NodeID, ordinal int, spacing float64, obstacles []geometry.Rectangle, used []trackSegment) []geometry.Point {
	r1, r2 := p.axis(p.box[a]), p.axis(p.box[b])
	center := r1.Center().X
	low, high := min(r1.X, r2.X), max(r1.MaxX(), r2.MaxX())
	for _, box := range obstacles {
		r := p.axis(box)
		if r.X <= center && center <= r.MaxX() && r.Y < max(r1.MaxY(), r2.MaxY()) && min(r1.Y, r2.Y) < r.MaxY() {
			low = min(low, r.X)
			high = max(high, r.MaxX())
		}
	}
	xy := func(flow, cross float64) geometry.Point {
		if p.vertical {
			return geometry.NewPoint(cross, flow)
		}
		return geometry.NewPoint(flow, cross)
	}
	fractions := []float64{0, -.2, .2, -.35, .35, -.1, .1, -.28, .28}
	if p.crossRankEdge[a] || p.crossRankEdge[b] {
		// Leave the center port available for the connection to the adjacent
		// rank. A returning flat edge can reach both faces off-center.
		fractions[0], fractions[1] = fractions[1], fractions[0]
	}
	// Every rejected occupied lane has a previously reserved segment. For
	// visually crowded ranks also stagger attachments along the border.
	for k := 0; k <= len(used)+1; k++ {
		for side := 0; side < 2; side++ {
			top := (side+max(ordinal-1, 0))%2 == 0
			start, end, track := r1.X, r2.X, low-spacing*float64(k+1)
			if !top {
				start, end, track = r1.MaxX(), r2.MaxX(), high+spacing*float64(k+1)
			}
			for _, fraction := range fractions {
				y1, y2 := r1.Center().Y+fraction*r1.Height, r2.Center().Y+fraction*r2.Height
				path := []geometry.Point{xy(start, y1), xy(track, y1), xy(track, y2), xy(end, y2)}
				if !overlapsTrack(path, used) && clearSegment(path[0], path[1], obstacles) && clearSegment(path[1], path[2], obstacles) && clearSegment(path[2], path[3], obstacles) {
					return path
				}
			}
		}
	}
	return nil
}

func compactRoute(points []geometry.Point) []geometry.Point {
	out := points[:0]
	for _, pt := range points {
		n := len(out)
		if n > 0 && pt == out[n-1] {
			continue
		}
		if n > 1 {
			a, b := out[n-2], out[n-1]
			if (a.X == b.X && b.X == pt.X || a.Y == b.Y && b.Y == pt.Y) && b.Sub(a).Dot(pt.Sub(b)) >= 0 {
				out[n-1] = pt
				continue
			}
		}
		out = append(out, pt)
	}
	return out
}

func clearSegment(a, b geometry.Point, obstacles []geometry.Rectangle) bool {
	for _, r := range obstacles {
		if a.X == b.X && r.X < a.X && a.X < r.MaxX() && min(a.Y, b.Y) < r.MaxY() && max(a.Y, b.Y) > r.Y {
			return false
		}
		if a.Y == b.Y && r.Y < a.Y && a.Y < r.MaxY() && min(a.X, b.X) < r.MaxX() && max(a.X, b.X) > r.X {
			return false
		}
	}
	return true
}

func orthogonalRoute(a, b geometry.Point, horizontal bool, obs []geometry.Rectangle, used []trackSegment, monitor core.ProgressMonitor) []geometry.Point {
	clearRoute := func(a, b geometry.Point) bool {
		return clearSegment(a, b, obs) && !segmentOnTrack(a, b, used)
	}
	if (a.X == b.X || a.Y == b.Y) && clearRoute(a, b) {
		return []geometry.Point{a, b}
	}
	// Prefer a centered dogleg. Flat edges between immediate neighbors
	// take the straight segment above, including across containment levels.
	paths := [][]geometry.Point{
		{a, {X: a.X, Y: (a.Y + b.Y) / 2}, {X: b.X, Y: (a.Y + b.Y) / 2}, b},
		{a, {X: (a.X + b.X) / 2, Y: a.Y}, {X: (a.X + b.X) / 2, Y: b.Y}, b},
		{a, {X: a.X, Y: b.Y}, b}, {a, {X: b.X, Y: a.Y}, b},
	}
	if horizontal {
		paths[0], paths[1] = paths[1], paths[0]
	}
	for _, path := range paths {
		clear := true
		for i := 1; i < len(path); i++ {
			if !clearRoute(path[i-1], path[i]) {
				clear = false
				break
			}
		}
		if clear {
			return path
		}
	}
	// Search the Manhattan visibility grid lazily. A rectilinear path can
	// turn at a line immediately outside an obstacle; endpoints supply the
	// other coordinate lines. A* visits only the needed intersections.
	xs := []float64{a.X, b.X}
	ys := []float64{a.Y, b.Y}
	for _, r := range obs {
		xs = append(xs, r.X-2, r.MaxX()+2)
		ys = append(ys, r.Y-2, r.MaxY()+2)
	}
	for _, seg := range used {
		if seg.a.X == seg.b.X {
			xs = append(xs, seg.a.X-6, seg.a.X+6)
		} else {
			ys = append(ys, seg.a.Y-6, seg.a.Y+6)
		}
	}
	sort.Float64s(xs)
	xs = slices.Compact(xs)
	sort.Float64s(ys)
	ys = slices.Compact(ys)
	type cell = trackCell
	pos := func(c cell) geometry.Point { return geometry.NewPoint(xs[c.x], ys[c.y]) }
	start := cell{sort.SearchFloat64s(xs, a.X), sort.SearchFloat64s(ys, a.Y), 0}
	queue := &trackQueue{{cell: start}}
	dist := map[cell]float64{start: 0}
	prev := map[cell]cell{}
	for visited := 0; queue.Len() > 0; visited++ {
		if visited%256 == 0 && monitor.Canceled() {
			return nil
		}
		v := heap.Pop(queue).(trackStep)
		c := v.cell
		if v.cost != dist[c] {
			continue
		}
		pt := pos(c)
		if pt == b {
			path := []geometry.Point{pt}
			for c != start {
				c = prev[c]
				path = append(path, pos(c))
			}
			slices.Reverse(path)
			return path
		}
		for _, k := range []cell{{c.x - 1, c.y, 1}, {c.x + 1, c.y, 1}, {c.x, c.y - 1, 2}, {c.x, c.y + 1, 2}} {
			if k.x < 0 || k.x >= len(xs) || k.y < 0 || k.y >= len(ys) {
				continue
			}
			q := pos(k)
			if !clearRoute(pt, q) {
				continue
			}
			cost := v.cost + math.Abs(pt.X-q.X) + math.Abs(pt.Y-q.Y)
			if c.dir != 0 && k.dir != c.dir {
				cost += 8
			}
			if old, ok := dist[k]; ok && old <= cost {
				continue
			}
			dist[k] = cost
			prev[k] = c
			heap.Push(queue, trackStep{k, cost, cost + math.Abs(q.X-b.X) + math.Abs(q.Y-b.Y)})
		}
	}
	return nil
}

type trackCell struct{ x, y, dir int }
type trackStep struct {
	cell           trackCell
	cost, estimate float64
}
type trackQueue []trackStep

func (q trackQueue) Len() int           { return len(q) }
func (q trackQueue) Less(i, j int) bool { return q[i].estimate < q[j].estimate }
func (q trackQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *trackQueue) Push(v any)        { *q = append(*q, v.(trackStep)) }
func (q *trackQueue) Pop() any          { a := *q; v := a[len(a)-1]; *q = a[:len(a)-1]; return v }

func placeRankLabel(path []geometry.Point, w, h float64, obstacles, labels []geometry.Rectangle) geometry.Rectangle {
	best := geometry.Rectangle{}
	bestScore := math.Inf(1)
	for i := 1; i < len(path); i++ {
		a, b := path[i-1], path[i]
		mid := a.Add(b).Scale(.5)
		for _, frac := range []float64{0, .25, -.25} {
			p := mid.Add(b.Sub(a).Scale(frac))
			for _, side := range []float64{-1, 1} {
				r := geometry.Rectangle{X: p.X - w/2, Y: p.Y - h/2, Width: w, Height: h}
				if a.Y == b.Y {
					r.Y += side * (h/2 + 2)
				} else {
					r.X += side * (w/2 + 2)
				}
				score := -a.Distance(b) + math.Abs(frac)*4
				for _, list := range [][]geometry.Rectangle{obstacles, labels} {
					for _, o := range list {
						if r.Intersects(o) {
							score += 1e9 + (min(r.MaxX(), o.MaxX())-max(r.X, o.X))*(min(r.MaxY(), o.MaxY())-max(r.Y, o.Y))
						}
					}
				}
				if score < bestScore {
					best, bestScore = r, score
				}
			}
		}
	}
	// A crowded fixed-port fan can leave no free midpoint. Walk the
	// occupied intervals on either axis to the closest free label position.
	occupied := append(append([]geometry.Rectangle(nil), obstacles...), labels...)
	if !slices.ContainsFunc(occupied, best.Intersects) {
		return best
	}
	origin := best
	cost := math.Inf(1)
	for _, o := range occupied {
		for _, q := range []geometry.Point{{X: o.X - w - 2, Y: origin.Y}, {X: o.MaxX() + 2, Y: origin.Y}, {X: origin.X, Y: o.Y - h - 2}, {X: origin.X, Y: o.MaxY() + 2}} {
			r := geometry.Rectangle{X: q.X, Y: q.Y, Width: w, Height: h}
			clear := true
			for _, other := range occupied {
				if r.Intersects(other) {
					clear = false
					break
				}
			}
			if d := q.Distance(origin.Position()); clear && d < cost {
				best, cost = r, d
			}
		}
	}
	return best
}
