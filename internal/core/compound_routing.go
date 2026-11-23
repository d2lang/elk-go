// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"cmp"
	"fmt"
	"math"
	"slices"

	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
)

var compoundEdgeNodeSpacing = graph.PropertyWithDefault("org.eclipse.elk.spacing.edgeNode", func() float64 { return 16 })

// Allocate attachment slots in input order. Opposite directions share the
// same group; port identity is part of the key so each SQL row stays fixed.
func routeCompoundEdges(model *graph.Model, edges []compoundEdge) error {
	type pair [2]graph.ShapeID
	type group struct {
		count, next int
		labels      []geometry.Rectangle
	}
	groups := make(map[pair]*group)
	order := make([]*group, 0, len(edges))
	for _, c := range edges {
		key := pair{c.source[0], c.target[0]}
		if model.ShapeNode(key[0]) > model.ShapeNode(key[1]) {
			key[0], key[1] = key[1], key[0]
		}
		g := groups[key]
		if g == nil {
			g = &group{}
			groups[key] = g
		}
		g.count++
		order = append(order, g)
	}
	for i, c := range edges {
		g := order[i]
		fraction := (float64(g.next) + 0.5) / float64(g.count)
		g.next++
		if err := routeCompoundEdge(model, c.id, fraction, g.labels); err != nil {
			return err
		}
		if edge := model.Edge(c.id); g.count > 1 && len(edge.Labels) > 0 {
			chain := model.SectionChain(edge.Sections[0])
			center := chain.PointAt(chain.Length() / 2)
			for _, id := range edge.Labels {
				box := model.Label(id).Shape.Rectangle()
				g.labels = append(g.labels, box)
				// Inline consumers can recenter labels on the route.
				if box.Center() != center {
					box.X, box.Y = center.X-box.Width/2, center.Y-box.Height/2
					g.labels = append(g.labels, box)
				}
			}
		}
	}
	return nil
}

// Route in the edge container's coordinate system. Only the two branches
// leading to its endpoints are permeable. Any sibling subtree is a single
// obstacle, regardless of its depth.
func routeCompoundEdge(model *graph.Model, edgeID graph.EdgeID, fraction float64, parallelLabels []geometry.Rectangle) error {
	edge := model.Edge(edgeID)
	source, target := edge.Sources[0], edge.Targets[0]
	container := *edge.ContainingNode
	src, dst := model.ShapeNode(source), model.ShapeNode(target)
	branches := make(map[graph.NodeID]bool)
	for _, endpoint := range []graph.NodeID{src, dst} {
		for node := endpoint; node != container; {
			parent := model.Node(node).Parent
			if parent == nil {
				return fmt.Errorf("edge %q lies outside its container", edge.Identifier)
			}
			node = *parent
			branches[node] = true
		}
	}
	var srcBox, dstBox geometry.Rectangle
	var obstacles []geometry.Rectangle
	type visit struct {
		id graph.NodeID
		at geometry.Point
	}
	stack := []visit{{id: container}}
	for len(stack) > 0 {
		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		node := model.Node(item.id)
		box := geometry.Rectangle{X: item.at.X, Y: item.at.Y, Width: node.Shape.Width, Height: node.Shape.Height}
		if item.id == src {
			srcBox = box
		}
		if item.id == dst {
			dstBox = box
		}
		if !branches[item.id] && item.id != src && item.id != dst {
			obstacles = append(obstacles, box)
		}
		for _, id := range node.Labels {
			label := model.Label(id).Shape
			if label.Width <= 0 || label.Height <= 0 {
				continue
			}
			obstacles = append(obstacles, label.Rectangle().Move(item.at))
			// A container with a measured but unpositioned title can be
			// drawn at top center by its consumer (as in D2). Protect that
			// space too. The extra inset keeps even a border stroke clear.
			if branches[item.id] && label.X == 0 && label.Y == 0 {
				obstacles = append(obstacles, geometry.Rectangle{X: box.Center().X - label.Width/2, Y: box.Y, Width: label.Width, Height: label.Height + 5})
			}
		}
		if branches[item.id] {
			for _, child := range node.Children {
				stack = append(stack, visit{child, item.at.Add(model.Node(child).Shape.Position())})
			}
		}
	}

	bounds := model.Node(container).Shape.Rectangle()
	bounds.X, bounds.Y = 0, 0
	var path geometry.Chain
	var blocked []geometry.Rectangle
	// Prefer a little clearance from boxes. Tight or fixed layouts may
	// have only a narrow corridor between two obstacles.
	// A sufficiently long normal approach gives rounded bends and arrowheads
	// room before the port. If the corridor is narrow, shorten the lead
	// before giving up clearance from neighboring obstacles.
	preferredLead := math.Max(5, graph.Get(&model.Node(container).Properties, compoundEdgeNodeSpacing))
	for _, attempt := range [...]struct{ lead, clearance float64 }{{preferredLead, 5}, {5, 5}, {0, 0}} {
		clearance := attempt.clearance
		starts := compoundAnchors(model, source, srcBox, dstBox, branches[src], attempt.lead, fraction)
		ends := compoundAnchors(model, target, dstBox, srcBox, branches[dst], attempt.lead, fraction)
		// Keep strokes clear even when a candidate grazes an obstacle.
		// Endpoint boxes themselves must remain accessible from their border.
		blocked = make([]geometry.Rectangle, 0, len(obstacles)+len(parallelLabels)+2)
		for _, box := range obstacles {
			blocked = append(blocked, compoundExpandBox(box, clearance, clearance))
		}
		if !branches[src] {
			blocked = append(blocked, srcBox)
		}
		if !branches[dst] {
			blocked = append(blocked, dstBox)
		}
		// Prefer a lane around labels already placed on parallel edges.
		// A fixed port can leave no choice at the endpoint.
		if clearance > 0 && len(parallelLabels) > 0 {
			w, h := 0.0, 0.0
			for _, id := range edge.Labels {
				label := model.Label(id).Shape
				w = math.Max(w, label.Width)
				h = math.Max(h, label.Height)
			}
			for _, box := range parallelLabels {
				blocked = append(blocked, compoundExpandBox(box, w/2+clearance, h/2+clearance))
			}
		}
		path = compoundPath(starts, ends, blocked, bounds, clearance)
		if len(path) >= 2 {
			// An explicit port anchor may sit inside its owner's box.
			// The obstacle search starts at the accessible face. Add back
			// the terminal segment from the caller's actual port position.
			if source.Kind == graph.PortShape && starts[0].actual != path[0] {
				path = append(geometry.Chain{starts[0].actual}, path...)
			}
			if target.Kind == graph.PortShape && ends[0].actual != path[len(path)-1] {
				path = append(path, ends[0].actual)
			}
			path = compactCompoundPath(path)
			break
		}
	}
	if len(path) < 2 {
		return fmt.Errorf("cannot route cross-container edge %q between its endpoints without crossing a shape or label", edge.Identifier)
	}
	section := model.Section(model.FirstSection(edgeID, true))
	section.Start, section.End = path[0], path[len(path)-1]
	section.StartPresent, section.EndPresent = true, true
	section.BendPoints = path[1 : len(path)-1]
	section.BendPointsPresent = len(section.BendPoints) > 0
	section.IncomingShape, section.OutgoingShape = &source, &target

	for _, id := range edge.Labels {
		label := &model.Label(id).Shape
		point := compoundLabelPosition(path, label.Width, label.Height, blocked)
		label.SetLocation(point.X, point.Y)
		blocked = append(blocked, label.Rectangle())
	}
	return nil
}

func compoundExpandBox(box geometry.Rectangle, horizontal, vertical float64) geometry.Rectangle {
	return geometry.Rectangle{X: box.X - horizontal, Y: box.Y - vertical, Width: box.Width + 2*horizontal, Height: box.Height + 2*vertical}
}

// Keep a label centered on the path when possible. In a group of parallel
// connections, a different point on the same route can clear an earlier label.
func compoundLabelPosition(path geometry.Chain, width, height float64, obstacles []geometry.Rectangle) geometry.Point {
	length := path.Length()
	middle := path.PointAt(length / 2)
	position := func(center geometry.Point) geometry.Point {
		return geometry.Point{X: center.X - width/2, Y: center.Y - height/2}
	}
	for _, fraction := range [...]float64{.5, .4, .6, .3, .7, .2, .8, .1, .9} {
		p := position(path.PointAt(length * fraction))
		box := geometry.Rectangle{X: p.X, Y: p.Y, Width: width, Height: height}
		fits := true
		for _, r := range obstacles {
			if box.Intersects(r) {
				fits = false
				break
			}
		}
		if fits {
			return p
		}
	}
	return position(middle)
}

type compoundAnchor struct{ actual, point, lead geometry.Point }

func compoundAnchors(model *graph.Model, shape graph.ShapeID, box, other geometry.Rectangle, inside bool, lead, fraction float64) []compoundAnchor {
	center := geometry.Point{X: box.X + fraction*box.Width, Y: box.Y + fraction*box.Height}
	anchors := []geometry.Point{{X: box.X, Y: center.Y}, {X: box.MaxX(), Y: center.Y}, {X: center.X, Y: box.Y}, {X: center.X, Y: box.MaxY()}}
	normals := [...]geometry.Point{{X: -1}, {X: 1}, {Y: -1}, {Y: 1}}
	only := -1
	if shape.Kind == graph.PortShape {
		port := model.Port(shape.Port)
		offset := geometry.Point{X: port.Shape.Width / 2, Y: port.Shape.Height / 2}
		if anchor, ok := graph.TryGet(&port.Properties, PortAnchor); ok {
			offset = anchor
		}
		p := box.Position().Add(port.Shape.Position()).Add(offset)
		switch graph.Get(&port.Properties, PortSide) {
		case "WEST":
			only = 0
		case "EAST":
			only = 1
			if port.Shape.X == 0 && port.Shape.Width == 0 {
				p.X = box.MaxX() + offset.X
			}
		case "NORTH":
			only = 2
		case "SOUTH":
			only = 3
			if port.Shape.Y == 0 && port.Shape.Height == 0 {
				p.Y = box.MaxY() + offset.Y
			}
		default:
			dists := [...]float64{math.Abs(p.X - box.X), math.Abs(p.X - box.MaxX()), math.Abs(p.Y - box.Y), math.Abs(p.Y - box.MaxY())}
			only = slices.Index(dists[:], slices.Min(dists[:]))
		}
		anchors[only] = p
	}
	// For a fixed port, the provider can leave the anchor slightly inset
	// (or outside, for an edge pointing into its own container).
	faces := [...]float64{box.X, box.MaxX(), box.Y, box.MaxY()}
	if inside {
		lead = -lead
	}
	result := make([]compoundAnchor, 0, len(anchors))
	for i, p := range anchors {
		if only >= 0 && i != only {
			continue
		}
		at := p
		if only >= 0 {
			direction := normals[i]
			if inside {
				direction = direction.Negate()
			}
			if direction.X != 0 && (p.X-faces[i])*direction.X < 0 {
				at.X = faces[i]
			}
			if direction.Y != 0 && (p.Y-faces[i])*direction.Y < 0 {
				at.Y = faces[i]
			}
		}
		// If the two faces look into the same narrow gap, their leads
		// meet halfway. Crossing the leads would force a detour just
		// to turn around and enter the target from the correct side.
		length := lead
		if !inside {
			gaps := [...]float64{at.X - other.MaxX(), other.X - at.X, at.Y - other.MaxY(), other.Y - at.Y}
			horizontal := normals[i].X != 0
			faceToFace := horizontal && box.Y < other.MaxY() && other.Y < box.MaxY() || !horizontal && box.X < other.MaxX() && other.X < box.MaxX()
			if faceToFace && gaps[i] > 0 {
				length = math.Min(length, gaps[i]/2)
			}
		}
		result = append(result, compoundAnchor{actual: p, point: at, lead: at.Add(normals[i].Scale(length))})
	}
	return result
}

func compoundSegmentHits(a, b geometry.Point, obstacle geometry.Rectangle) bool {
	return obstacle.Intersects(geometry.Rectangle{
		X: math.Min(a.X, b.X), Y: math.Min(a.Y, b.Y),
		Width: math.Abs(a.X - b.X), Height: math.Abs(a.Y - b.Y),
	})
}

func compoundPath(starts, ends []compoundAnchor, obstacles []geometry.Rectangle, bounds geometry.Rectangle, clearance float64) geometry.Chain {
	// Build the coordinate grid lazily. On an open diagram the only grid
	// lines needed are the endpoint coordinates. Add obstacle boundaries
	// only when a candidate path actually meets them.
	var active []geometry.Rectangle
	seen := make([]bool, len(obstacles))
	for {
		path := compoundGrid(starts, ends, active, bounds, clearance)
		if len(path) < 2 {
			return nil
		}
		added := false
		for i, obstacle := range obstacles {
			if seen[i] || obstacle.Width <= 0 || obstacle.Height <= 0 {
				continue
			}
			for j := 1; j < len(path); j++ {
				if compoundSegmentHits(path[j-1], path[j], obstacle) {
					active = append(active, obstacle)
					seen[i], added = true, true
					break
				}
			}
		}
		if !added {
			return path
		}
	}
}

// Search the horizontal and vertical lines through the ports and obstacle
// margins. The incoming direction is part of the Dijkstra state so bends cost
// more and neither terminal segment can double back. Store cells on demand.
func compoundGrid(starts, ends []compoundAnchor, obstacles []geometry.Rectangle, bounds geometry.Rectangle, clearance float64) geometry.Chain {
	inBounds := func(p geometry.Point) bool {
		return p.X >= bounds.X && p.X <= bounds.MaxX() && p.Y >= bounds.Y && p.Y <= bounds.MaxY()
	}
	clear := func(a, b geometry.Point) bool {
		for _, r := range obstacles {
			if compoundSegmentHits(a, b, r) {
				return false
			}
		}
		return true
	}
	var xs, ys []float64
	add := func(p geometry.Point) { xs = append(xs, p.X); ys = append(ys, p.Y) }
	for _, anchors := range [][]compoundAnchor{starts, ends} {
		for _, a := range anchors {
			if inBounds(a.lead) {
				add(a.lead)
			}
		}
	}
	add(bounds.Position())
	add(bounds.BottomRight())
	for _, r := range obstacles {
		xs = append(xs, math.Max(bounds.X, r.X-clearance), math.Min(bounds.MaxX(), r.MaxX()+clearance))
		ys = append(ys, math.Max(bounds.Y, r.Y-clearance), math.Min(bounds.MaxY(), r.MaxY()+clearance))
	}
	slices.Sort(xs)
	xs = slices.Compact(xs)
	slices.Sort(ys)
	ys = slices.Compact(ys)
	type cell struct{ x, y int }
	at := func(p geometry.Point) cell {
		x, _ := slices.BinarySearch(xs, p.X)
		y, _ := slices.BinarySearch(ys, p.Y)
		return cell{x, y}
	}
	point := func(c cell) geometry.Point { return geometry.Point{X: xs[c.x], Y: ys[c.y]} }
	direction := func(a, b geometry.Point) cell { return cell{cmp.Compare(b.X, a.X), cmp.Compare(b.Y, a.Y)} }
	type state struct {
		cell
		heading cell
	}
	type entry struct {
		state
		cost float64
	}
	q := javacompat.NewPriorityQueue(func(a, b entry) int { return cmp.Compare(a.cost, b.cost) })
	distance := make(map[state]float64)
	prev := make(map[state]state)
	startAt := make(map[state]geometry.Point)
	endAt := make(map[cell]compoundAnchor)
	for _, end := range ends {
		if inBounds(end.lead) && clear(end.point, end.lead) {
			endAt[at(end.lead)] = end
		}
	}
	if len(endAt) == 0 {
		return nil
	}
	for _, start := range starts {
		if !inBounds(start.lead) || !clear(start.point, start.lead) {
			continue
		}
		s := state{at(start.lead), direction(start.point, start.lead)}
		distance[s] = 0
		startAt[s] = start.point
		q.Add(entry{s, 0})
	}

	for !q.IsEmpty() {
		item, _ := q.Poll()
		if item.cost != distance[item.state] {
			continue
		}
		p := point(item.cell)
		// Approaching a port from the back produces a reversed final segment.
		reverse := cell{-item.heading.x, -item.heading.y}
		if end, ok := endAt[item.cell]; ok && (reverse == (cell{}) || direction(end.lead, end.point) != reverse) {
			path := geometry.Chain{end.point, p}
			current := item.state
			for {
				parent, ok := prev[current]
				if !ok {
					break
				}
				current = parent
				path = append(path, point(current.cell))
			}
			path = append(path, startAt[current])
			slices.Reverse(path)
			path = compactCompoundPath(path)
			if len(path) >= 2 {
				return path
			}
		}
		for _, move := range [...]cell{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			if move == reverse {
				continue
			}
			next := cell{item.x + move.x, item.y + move.y}
			if next.x < 0 || next.x >= len(xs) || next.y < 0 || next.y >= len(ys) {
				continue
			}
			np := point(next)
			if !clear(p, np) {
				continue
			}
			s := state{next, move}
			cost := item.cost + math.Abs(p.X-np.X) + math.Abs(p.Y-np.Y)
			if item.heading != (cell{}) && move != item.heading {
				cost += 5
			}
			if d, ok := distance[s]; ok && d <= cost {
				continue
			}
			distance[s] = cost
			prev[s] = item.state
			q.Add(entry{s, cost})
		}
	}
	return nil
}

func compactCompoundPath(points geometry.Chain) geometry.Chain {
	// Radial placement can leave two aligned rows a few ULPs apart. A
	// subpixel jog between them otherwise becomes a fake rounded corner.
	// Remove only intermediate points; preserve the exact port anchors.
	const epsilon = 1e-9
	onAxis := func(a, b, c float64) bool { return math.Abs(a-b) <= epsilon && math.Abs(b-c) <= epsilon }
	between := func(a, b, c float64) bool { return b >= math.Min(a, c)-epsilon && b <= math.Max(a, c)+epsilon }
	out := points[:0]
	for _, p := range points {
		for len(out) >= 2 {
			n := len(out)
			a, b := out[n-2], out[n-1]
			vertical := onAxis(a.X, b.X, p.X) && between(a.Y, b.Y, p.Y)
			horizontal := onAxis(a.Y, b.Y, p.Y) && between(a.X, b.X, p.X)
			if !vertical && !horizontal {
				break
			}
			out = out[:n-1]
		}
		if len(out) == 0 || out[len(out)-1] != p {
			out = append(out, p)
		}
	}
	return out
}
