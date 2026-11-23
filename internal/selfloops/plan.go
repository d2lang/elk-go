// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

// Package selfloops plans exterior self-loops in node-local coordinates. Each
// provider uses the occupied margins during placement, then translates the
// planned routes by the final position of the node's visible box.
package selfloops

import (
	"math"
	"slices"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

// RouteOption is the opt-in extension on the root graph.
const RouteOption = "elkgo.routeSelfLoops"

var (
	enabled     = graph.PropertyWithDefault(RouteOption, func() bool { return false })
	loopSpacing = graph.PropertyWithDefault("org.eclipse.elk.spacing.nodeSelfLoop", func() float64 { return 10 })
	laneSpacing = graph.PropertyWithDefault("org.eclipse.elk.spacing.edgeEdge", func() float64 { return 10 })
)

type Side uint8

const (
	North Side = iota
	East
	South
	West
)

type endpoint struct {
	port  bool
	side  Side
	point geometry.Point
}

type loop struct {
	id                      graph.EdgeID
	src, dst                endpoint
	labelWidth, labelHeight float64
	path                    geometry.Chain
	center                  geometry.Point
}

type Plan struct {
	Left, Right, Top, Bottom float64
	width, height            float64
	spacing, laneGap         float64
	loops                    []loop
	existing                 existingRoutes
}

func Collect(model *graph.Model, parent graph.NodeID) map[graph.NodeID]*Plan {
	if !graph.Get(&model.Node(model.Root).Properties, enabled) {
		return nil
	}
	var result map[graph.NodeID]*Plan
	container := model.Node(parent)
	for _, id := range container.ContainedEdges {
		edge := model.Edge(id)
		owner, ok := exteriorOwner(model, edge, parent)
		if !ok {
			continue
		}
		node := model.Node(owner)
		if result == nil {
			result = make(map[graph.NodeID]*Plan)
		}
		p := result[owner]
		if p == nil {
			p = &Plan{width: node.Shape.Width, height: node.Shape.Height,
				spacing: math.Max(12, graph.Get(&container.Properties, loopSpacing)),
				laneGap: math.Max(4, graph.Get(&container.Properties, laneSpacing))}
			result[owner] = p
		}
		if len(edge.Sections) != 0 {
			p.retainExisting(model, id, node.Shape.Position())
			continue
		}
		l := loop{id: id, src: portEndpoint(model, edge.Sources[0], node.Shape), dst: portEndpoint(model, edge.Targets[0], node.Shape)}
		for i, labelID := range edge.Labels {
			size := model.Label(labelID).Shape
			l.labelWidth = math.Max(l.labelWidth, size.Width)
			if i > 0 {
				l.labelHeight += 4
			}
			l.labelHeight += size.Height
		}
		p.loops = append(p.loops, l)
	}
	for _, p := range result {
		p.Arrange(nil)
	}
	return result
}

func exteriorOwner(model *graph.Model, edge *graph.Edge, parent graph.NodeID) (graph.NodeID, bool) {
	if len(edge.Sources) != 1 || len(edge.Targets) != 1 || graph.Get(&edge.Properties, core.NoLayout) {
		return 0, false
	}
	owner := model.ShapeNode(edge.Sources[0])
	node := model.Node(owner)
	if owner != model.ShapeNode(edge.Targets[0]) || node.Parent == nil || *node.Parent != parent {
		return 0, false
	}
	return owner, !(graph.Get(&edge.Properties, core.InsideSelfLoopsEdge) && graph.Get(&node.Properties, core.InsideSelfLoopsActivate))
}

// SuppliedExteriorLoops selects only the JSON geometry these providers retain
// during layout. Other ELK providers keep their standard import semantics.
func SuppliedExteriorLoops(model *graph.Model, algorithms *core.AlgorithmRegistry) []graph.EdgeID {
	if !graph.Get(&model.Node(model.Root).Properties, enabled) {
		return nil
	}
	var result []graph.EdgeID
	for i := range model.Edges {
		edge := &model.Edges[i]
		if len(edge.Sections) == 0 || edge.ContainingNode == nil {
			continue
		}
		parent := *edge.ContainingNode
		if _, ok := exteriorOwner(model, edge, parent); !ok {
			continue
		}
		provider := algorithms.BySuffix(graph.Get(&model.Node(parent).Properties, core.Algorithm))
		if provider == nil {
			continue
		}
		switch provider.ID {
		case "org.eclipse.elk.force", "org.eclipse.elk.stress", "org.eclipse.elk.mrtree":
			if layoutEnabled(model, parent) {
				result = append(result, graph.EdgeID(i))
			}
		}
	}
	return result
}

func layoutEnabled(model *graph.Model, parent graph.NodeID) bool {
	for node := model.Node(parent); ; node = model.Node(*node.Parent) {
		if graph.Get(&node.Properties, core.NoLayout) {
			return false
		}
		if node.Parent == nil {
			return true
		}
	}
}

func portEndpoint(model *graph.Model, ref graph.ShapeID, box graph.Shape) endpoint {
	if ref.Kind != graph.PortShape {
		return endpoint{}
	}
	port := model.Port(ref.Port)
	anchor := geometry.Point{X: port.Shape.Width / 2, Y: port.Shape.Height / 2}
	if override, ok := graph.TryGet(&port.Properties, core.PortAnchor); ok {
		anchor = override
	}
	point := port.Shape.Position().Add(anchor)
	distances := [4]float64{math.Abs(point.Y), math.Abs(point.X - box.Width), math.Abs(point.Y - box.Height), math.Abs(point.X)}
	side := North
	switch graph.Get(&port.Properties, core.PortSide) {
	case "NORTH":
		side = North
	case "EAST":
		side = East
	case "SOUTH":
		side = South
	case "WEST":
		side = West
	default:
		for s := East; s <= West; s++ {
			if distances[s] < distances[side] {
				side = s
			}
		}
	}
	return endpoint{port: true, side: side, point: point}
}

// Arrange chooses free endpoints in the provider's preferred side order.
// Explicit port attachments take precedence. Replanning never accumulates
// margins, so a provider can refine its choice using its initial positions.
func (p *Plan) Arrange(preferred []Side) {
	if len(preferred) == 0 {
		preferred = []Side{West, East, North, South}
	}
	p.Left, p.Right, p.Top, p.Bottom = 0, 0, 0, 0
	p.include(p.existing.min)
	p.include(p.existing.max)
	// Allocate separate border anchors as well as separate lanes. Larger
	// detours use the outermost pair of anchors on a shared side.
	var counts, used [4]int
	for i, l := range p.loops {
		a, b := l.sides(preferred[i%len(preferred)])
		counts[a]++
		if a != b {
			counts[b]++
		}
	}
	bands := p.existing.labelBands
	occupied := [4]float64{p.Top, p.Right, p.Bottom, p.Left}
	for i := range p.loops {
		l := &p.loops[i]
		startSide, endSide := l.sides(preferred[i%len(preferred)])
		start, end := l.src.point, l.dst.point
		if !l.src.port {
			start = p.anchor(startSide, float64(counts[startSide]-used[startSide])/float64(2*counts[startSide]+1))
		}
		if !l.dst.port {
			end = p.anchor(endSide, float64(counts[endSide]+1+used[endSide])/float64(2*counts[endSide]+1))
		}
		used[startSide]++
		if endSide != startSide {
			used[endSide]++
		}
		halfLabel := l.labelWidth / 2
		if startSide == North || startSide == South {
			halfLabel = l.labelHeight / 2
		}
		if startSide != endSide {
			halfLabel = math.Max(l.labelHeight, l.labelWidth) / 2
		}
		reach := math.Max(p.spacing, math.Max(occupied[startSide], occupied[endSide])+p.laneGap+halfLabel)
		l.path = p.route(start, startSide, end, endSide, reach)
		if startSide == endSide {
			l.path = p.clearInnerLabels(l.path, startSide, bands[startSide])
		}
		l.center = l.path.PointAt(l.path.Length() / 2)
		for _, v := range l.path {
			p.include(v)
		}
		box := geometry.Rectangle{X: l.center.X - l.labelWidth/2, Y: l.center.Y - l.labelHeight/2, Width: l.labelWidth, Height: l.labelHeight}
		p.include(box.Position())
		p.include(box.BottomRight())
		p.includeLabel(&bands, box)
		occupied = [4]float64{p.Top, p.Right, p.Bottom, p.Left}
	}
}

func (l *loop) sides(preferred Side) (Side, Side) {
	if l.src.port && l.dst.port {
		return l.src.side, l.dst.side
	}
	if l.src.port {
		return l.src.side, l.src.side
	}
	if l.dst.port {
		return l.dst.side, l.dst.side
	}
	return preferred, preferred
}

func (p *Plan) anchor(side Side, fraction float64) geometry.Point {
	switch side {
	case North:
		return geometry.Point{X: p.width * fraction}
	case South:
		return geometry.Point{X: p.width * fraction, Y: p.height}
	case West:
		return geometry.Point{Y: p.height * fraction}
	default:
		return geometry.Point{X: p.width, Y: p.height * fraction}
	}
}

func (p *Plan) include(v geometry.Point) {
	p.Left = math.Max(p.Left, -v.X)
	p.Top = math.Max(p.Top, -v.Y)
	p.Right = math.Max(p.Right, v.X-p.width)
	p.Bottom = math.Max(p.Bottom, v.Y-p.height)
}

func (p *Plan) route(start geometry.Point, source Side, end geometry.Point, target Side, reach float64) geometry.Chain {
	// The four clockwise corners of a rectangle outside the visible box.
	left, right := math.Min(0, math.Min(start.X, end.X))-reach, math.Max(p.width, math.Max(start.X, end.X))+reach
	top, bottom := math.Min(0, math.Min(start.Y, end.Y))-reach, math.Max(p.height, math.Max(start.Y, end.Y))+reach
	corners := [4]geometry.Point{{X: right, Y: top}, {X: right, Y: bottom}, {X: left, Y: bottom}, {X: left, Y: top}}
	project := func(v geometry.Point, s Side) geometry.Point {
		switch s {
		case North:
			v.Y = top
		case East:
			v.X = right
		case South:
			v.Y = bottom
		case West:
			v.X = left
		}
		return v
	}
	a, b := project(start, source), project(end, target)
	path := geometry.Chain{start, a}
	if source == target && start == end {
		// Two ends on the identical port still need a bend and distinct
		// incoming/outgoing directions. Return to the port along its face.
		step := geometry.Point{X: reach / 2}
		if source == West || source == East {
			step = geometry.Point{Y: reach / 2}
		}
		path = append(path, a.Add(step), end.Add(step))
	} else if source != target {
		cw, ccw := geometry.Chain{a}, geometry.Chain{a}
		for s := source; s != target; s = (s + 1) % 4 {
			cw = append(cw, corners[s])
		}
		for s := source; s != target; s = (s + 3) % 4 {
			ccw = append(ccw, corners[(s+3)%4])
		}
		cw, ccw = append(cw, b), append(ccw, b)
		if ccw.Length() < cw.Length() {
			cw = ccw
		}
		path = append(path, cw[1:]...)
	} else {
		path = append(path, b)
	}
	return slices.Compact(append(path, end))
}

// Write emits coordinates relative to the containing graph using the final
// top-left of the visible node. Planning and component packing are complete.
func (p *Plan) Write(model *graph.Model, x, y float64) {
	p.TranslateExisting(model, x, y)
	offset := geometry.Point{X: x, Y: y}
	for _, l := range p.loops {
		edge := model.Edge(l.id)
		section := model.Section(model.FirstSection(l.id, true))
		section.Start, section.End = l.path[0].Add(offset), l.path[len(l.path)-1].Add(offset)
		section.StartPresent, section.EndPresent = true, true
		section.IncomingShape, section.OutgoingShape = &edge.Sources[0], &edge.Targets[0]
		for _, bend := range l.path[1 : len(l.path)-1] {
			section.BendPoints = append(section.BendPoints, bend.Add(offset))
		}
		section.BendPointsPresent = len(section.BendPoints) > 0
		labelY := y + l.center.Y - l.labelHeight/2
		for _, labelID := range edge.Labels {
			label := model.Label(labelID)
			label.Shape.SetLocation(x+l.center.X-label.Shape.Width/2, labelY)
			labelY += label.Shape.Height + 4
		}
	}
}
