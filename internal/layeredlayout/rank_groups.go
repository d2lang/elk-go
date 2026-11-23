// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package layeredlayout

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/d2lang/elk-go/internal/common/networksimplex"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// Rank equalities live in the root coordinate system. The normal per-parent
// rank graphs cannot express an equality involving two different parents.
// Keep ELK's measured shapes, ports, self-loops and unaffected sublayouts;
// rank the quotient graph of the remaining blocks in one coordinate system.
type rankLayout struct {
	m                 *graph.Model
	vertical, reverse bool
	expanded          []bool         // containers on a path from the root to a rank member
	group             []graph.NodeID // representative of each member
	owner             []graph.NodeID // topmost intact block containing a node
	blocks            []graph.NodeID
	rank              []int
	loopBounds        map[graph.NodeID]geometry.Rectangle
	old, box          []geometry.Rectangle // world coordinates
	crossRankEdge     []bool
	flatOrder         [][][2]graph.NodeID // edges projected to children of their LCA
	gap, layerGap     float64
}

func newRankLayout(m *graph.Model, raw string) (*rankLayout, error) {
	var groups [][]string
	if err := json.Unmarshal([]byte(raw), &groups); err != nil {
		return nil, fmt.Errorf("d2.rankGroups expects an array of node ID arrays: %w", err)
	}
	n := len(m.Nodes)
	p := &rankLayout{m: m, expanded: make([]bool, n), group: make([]graph.NodeID, n),
		crossRankEdge: make([]bool, n), owner: make([]graph.NodeID, n), rank: make([]int, n), old: make([]geometry.Rectangle, n), box: make([]geometry.Rectangle, n), flatOrder: make([][][2]graph.NodeID, n)}
	ids := make(map[string]graph.NodeID, n)
	for i := range m.Nodes {
		id := graph.NodeID(i)
		ids[m.Node(id).Identifier] = id
		p.group[id] = id
	}
	active := false
	for i, names := range groups {
		var first graph.NodeID
		for j, name := range names {
			id, ok := ids[name]
			if !ok {
				return nil, fmt.Errorf("d2.rankGroups[%d]: unknown node %q", i, name)
			}
			if id == m.Root || len(m.Node(id).Children) > 0 {
				return nil, fmt.Errorf("d2.rankGroups[%d]: %q must be a leaf node", i, name)
			}
			if j == 0 {
				first = id
				continue
			}
			a, b := p.rep(first), p.rep(id)
			if a != b {
				p.group[max(a, b)] = min(a, b)
				active = true
			}
		}
	}
	if !active {
		return nil, nil
	}
	counts := make([]int, n)
	for i := range m.Nodes {
		id := graph.NodeID(i)
		p.group[id] = p.rep(id)
		counts[p.group[id]]++
	}
	for i := range m.Nodes {
		id := graph.NodeID(i)
		if counts[p.group[id]] < 2 {
			continue
		}
		for parent := m.Node(id).Parent; parent != nil && !p.expanded[*parent]; parent = m.Node(*parent).Parent {
			p.expanded[*parent] = true
		}
	}
	root := m.Node(m.Root)
	dir := graph.Get(&root.Properties, core.Direction)
	p.vertical = dir == "UP" || dir == "DOWN"
	p.reverse = dir == "LEFT" || dir == "UP"
	p.gap = math.Max(4, graph.Get(&root.Properties, core.SpacingNodeNode))
	p.layerGap = math.Max(4, graph.Get(&root.Properties, layered.SpacingNodeNodeBetweenLayers))
	return p, nil
}

func (p *rankLayout) rep(id graph.NodeID) graph.NodeID {
	for p.group[id] != id {
		p.group[id] = p.group[p.group[id]]
		id = p.group[id]
	}
	return id
}

// axis returns a rectangle in (flow, transverse) coordinates. The same
// involution converts it back to (x,y); reversal is applied to rank centers.
func (p *rankLayout) axis(r geometry.Rectangle) geometry.Rectangle {
	if p.vertical {
		return geometry.Rectangle{X: r.Y, Y: r.X, Width: r.Height, Height: r.Width}
	}
	return r
}

func (p *rankLayout) layout(monitor core.ProgressMonitor) error {
	m := p.m
	order := append([]graph.NodeID{m.Root}, m.Descendants(m.Root)...)
	for _, id := range order {
		node := m.Node(id)
		p.old[id] = node.Shape.Rectangle()
		if par := node.Parent; par != nil {
			p.old[id] = p.old[id].Move(p.old[*par].Position())
			if p.expanded[*par] {
				p.owner[id] = id
				if !p.expanded[id] {
					p.blocks = append(p.blocks, id)
				}
			} else {
				p.owner[id] = p.owner[*par]
			}
		}
	}
	p.measureLoopBounds()
	if err := p.assignRanks(); err != nil {
		return err
	}
	p.pack(order)
	p.transfer(order)
	return p.route(monitor)
}

// Include a preserved loop's bends and labels in the footprint of its block.
// This leaves room for the loop whichever side ELK chose for its ports.
func (p *rankLayout) measureLoopBounds() {
	p.loopBounds = make(map[graph.NodeID]geometry.Rectangle)
	for i := range p.m.Edges {
		e := &p.m.Edges[i]
		if len(e.Sources) != 1 || len(e.Targets) != 1 {
			continue
		}
		id := p.m.ShapeNode(e.Sources[0])
		if id != p.m.ShapeNode(e.Targets[0]) {
			continue
		}
		block := p.owner[id]
		if p.expanded[id] {
			block = id
		}
		bounds, ok := p.loopBounds[block]
		if !ok {
			bounds = p.old[block]
		}
		parent := p.m.Root
		if e.ContainingNode != nil {
			parent = *e.ContainingNode
		}
		d := p.old[parent].Position()
		for _, sid := range e.Sections {
			s := p.m.Section(sid)
			for _, pt := range s.BendPoints {
				v := pt.Add(d)
				bounds = bounds.Union(geometry.Rectangle{X: v.X, Y: v.Y})
			}
		}
		for _, lid := range e.Labels {
			bounds = bounds.Union(p.m.Label(lid).Shape.Rectangle().Move(d))
		}
		p.loopBounds[block] = bounds
	}
}

// footprint expands a block rectangle in (flow, transverse) coordinates.
func (p *rankLayout) footprint(id graph.NodeID, r geometry.Rectangle) geometry.Rectangle {
	if bounds, ok := p.loopBounds[id]; ok {
		old, ink := p.axis(p.old[id]), p.axis(bounds)
		r.X += ink.X - old.X
		r.Y += ink.Y - old.Y
		r.Width += ink.Width - old.Width
		r.Height += ink.Height - old.Height
		return r
	}
	return r
}

type rankArc struct {
	from, to graph.NodeID
	length   int
	hard     bool // containment and extrema must survive cycle breaking
}

// Each expanded container used as an endpoint gets separate entry and exit
// variables. Bound every contained block between them: taking the extremal
// child from the ordinary layout fails as soon as an equality moves it.
func (p *rankLayout) rankGraph() ([]graph.NodeID, []rankArc, []graph.NodeID) {
	m := p.m
	group := append([]graph.NodeID(nil), p.group...)
	vertices := append([]graph.NodeID(nil), p.blocks...)
	var arcs []rankArc
	ends := make(map[graph.NodeID][2]graph.NodeID)
	endpoint := func(id graph.NodeID, exit bool) graph.NodeID {
		if !p.expanded[id] {
			return p.owner[id]
		}
		v, ok := ends[id]
		if !ok {
			v = [2]graph.NodeID{graph.NodeID(len(group)), graph.NodeID(len(group) + 1)}
			group = append(group, v[0], v[1])
			vertices = append(vertices, v[0], v[1])
			ends[id] = v
		}
		if exit {
			return v[1]
		}
		return v[0]
	}
	for _, e := range m.Edges {
		if len(e.Sources) != 1 || len(e.Targets) != 1 {
			continue
		}
		src, dst := m.ShapeNode(e.Sources[0]), m.ShapeNode(e.Targets[0])
		if src == dst {
			continue
		}
		a := endpoint(src, !p.expanded[src] || !m.IsDescendant(dst, src))
		b := endpoint(dst, p.expanded[dst] && m.IsDescendant(src, dst))
		if a == b {
			continue
		}
		if group[a] == group[b] {
			lca, _ := m.LowestCommonAncestor(a, b)
			for *m.Node(a).Parent != lca {
				a = *m.Node(a).Parent
			}
			for *m.Node(b).Parent != lca {
				b = *m.Node(b).Parent
			}
			p.flatOrder[lca] = append(p.flatOrder[lca], [2]graph.NodeID{a, b})
			continue
		}
		p.crossRankEdge[src], p.crossRankEdge[dst] = true, true
		arcs = append(arcs, rankArc{group[a], group[b], 1, false})
	}
	for _, id := range p.blocks {
		for parent := m.Node(id).Parent; parent != nil; parent = m.Node(*parent).Parent {
			if v, ok := ends[*parent]; ok {
				arcs = append(arcs, rankArc{v[0], group[id], 0, true}, rankArc{group[id], v[1], 0, true})
			}
		}
	}
	return vertices, arcs, group
}

func (p *rankLayout) assignRanks() error {
	vertices, arcs, group := p.rankGraph()
	// FIRST/LAST constrain the connected component in which ELK would have
	// laid out the node. First merge equal extrema, then bound the rank of
	// every real block in that component.
	component := append([]graph.NodeID(nil), group...)
	find := func(id graph.NodeID) graph.NodeID {
		for component[id] != id {
			component[id] = component[component[id]]
			id = component[id]
		}
		return id
	}
	for _, a := range arcs {
		x, y := find(a.from), find(a.to)
		component[y] = x
	}
	bounds := map[graph.NodeID]string{}
	for i := range p.m.Nodes {
		id := graph.NodeID(i)
		raw, _ := p.m.Node(id).Properties.Raw(layered.LayerConstraintOption.ID)
		name, _ := raw.(string)
		if name == "" || name == "NONE" {
			continue
		}
		if p.expanded[id] {
			return fmt.Errorf("d2.rankGroups: %s layer constraint on container %q containing rank members is unsupported", name, p.m.Node(id).Identifier)
		}
		if p.owner[id] != id {
			continue
		}
		rep := group[id]
		old := bounds[rep]
		if old != "" && old != name {
			return fmt.Errorf("d2.rankGroups: conflicting layer constraints %s and %s in the group containing %q", old, name, p.m.Node(id).Identifier)
		}
		bounds[rep] = name
	}
	type extreme struct {
		component graph.NodeID
		kind      string
	}
	pinned := map[extreme]graph.NodeID{}
	rep := append([]graph.NodeID(nil), group...)
	for _, id := range p.blocks {
		r := group[id]
		if name := bounds[r]; name != "" {
			key := extreme{find(id), name}
			if other, ok := pinned[key]; ok {
				rep[r] = other
			} else {
				pinned[key] = r
			}
		}
	}
	added := map[graph.NodeID]bool{}
	for _, id := range p.blocks {
		r := rep[group[id]]
		if added[r] {
			continue
		}
		added[r] = true
		for _, name := range []string{"FIRST_SEPARATE", "FIRST", "LAST", "LAST_SEPARATE"} {
			bound, ok := pinned[extreme{find(id), name}]
			if !ok || bound == r {
				continue
			}
			src, dst := bound, r
			if boundOrder(name) > boundOrder(bounds[r]) {
				src, dst = dst, src
			}
			delta := 0
			if name == "FIRST_SEPARATE" || name == "LAST_SEPARATE" {
				delta = 1
			}
			arcs = append(arcs, rankArc{src, dst, delta, true})
		}
	}
	nodes := make([]graph.NodeID, 0, len(vertices))
	seen := make(map[graph.NodeID]bool, len(vertices))
	for _, id := range vertices {
		r := rep[group[id]]
		if !seen[r] {
			seen[r] = true
			nodes = append(nodes, r)
		}
	}
	out := make([][]rankArc, len(group))
	totalIn := make([]int, len(group))
	hardIn := make([]int, len(group))
	for i := range arcs {
		a := &arcs[i]
		a.from, a.to = rep[a.from], rep[a.to]
		if a.from == a.to {
			continue
		}
		// Respect requested extrema when choosing the feedback edges, even
		// in a component that otherwise contains a cycle.
		if !a.hard && boundOrder(bounds[a.from]) > boundOrder(bounds[a.to]) {
			a.from, a.to = a.to, a.from
		}
		out[a.from] = append(out[a.from], *a)
		totalIn[a.to]++
		if a.hard {
			hardIn[a.to]++
		}
	}
	// Kahn order preserves every edge of a DAG. At a cycle, choose a vertex
	// with no remaining hard predecessor and reverse its incoming flow arcs.
	// Structural containment is never reversed, even when a group creates
	// a cycle through a compound edge.
	queue := make([]graph.NodeID, 0, len(nodes))
	for _, id := range nodes {
		if totalIn[id] == 0 {
			queue = append(queue, id)
		}
	}
	position := make([]int, len(group))
	for i := range position {
		position[i] = -1
	}
	for i := 0; i < len(nodes); i++ {
		if i == len(queue) {
			best := len(arcs) + 1
			var next graph.NodeID
			for _, id := range nodes {
				if position[id] < 0 && hardIn[id] == 0 && totalIn[id] < best {
					next, best = id, totalIn[id]
				}
			}
			if best > len(arcs) {
				return fmt.Errorf("d2.rankGroups: layer extrema conflict with containment constraints")
			}
			queue = append(queue, next)
		}
		id := queue[i]
		position[id] = i
		for _, a := range out[id] {
			totalIn[a.to]--
			if a.hard {
				hardIn[a.to]--
			}
			if totalIn[a.to] == 0 && position[a.to] < 0 {
				queue = append(queue, a.to)
			}
		}
	}
	net := networksimplex.NewGraph()
	super := net.AddNode()
	index := make(map[graph.NodeID]networksimplex.NodeID, len(nodes))
	for _, id := range nodes {
		index[id] = net.AddNode()
	}
	for _, a := range arcs {
		if a.from == a.to {
			continue
		}
		if position[a.from] > position[a.to] {
			a.from, a.to = a.to, a.from
		}
		weight := 1.
		if a.hard {
			weight = 0
		}
		net.AddEdge(index[a.from], index[a.to], weight, a.length)
	}
	for _, id := range nodes {
		n := index[id]
		if len(net.Node(n).IncomingEdges) == 0 {
			net.AddEdge(super, n, 0, 1)
		}
	}
	networksimplex.ForGraph(net).WithIterationLimit(4 * len(nodes)).Execute()
	for _, id := range p.blocks {
		p.rank[id] = net.Node(index[rep[group[id]]]).Layer - 1
	}
	return nil
}

func boundOrder(kind string) int {
	switch kind {
	case "FIRST_SEPARATE":
		return -2
	case "FIRST":
		return -1
	case "LAST":
		return 1
	case "LAST_SEPARATE":
		return 2
	}
	return 0
}

// Put the two endpoints of flat edges next to each other when their branch
// order is free. ELK's ordinary crossing order supplies all tie breakers.
func (p *rankLayout) children(parent graph.NodeID) []graph.NodeID {
	ids := append([]graph.NodeID(nil), p.m.Node(parent).Children...)
	sort.SliceStable(ids, func(i, j int) bool { return p.axis(p.old[ids[i]]).Center().Y < p.axis(p.old[ids[j]]).Center().Y })
	if len(p.flatOrder[parent]) == 0 {
		return ids
	}
	adj := map[graph.NodeID][]graph.NodeID{}
	for _, pair := range p.flatOrder[parent] {
		a, b := pair[0], pair[1]
		adj[a] = append(adj[a], b)
		adj[b] = append(adj[b], a)
	}
	seen := map[graph.NodeID]bool{}
	result := make([]graph.NodeID, 0, len(ids))
	var add func(graph.NodeID)
	add = func(id graph.NodeID) {
		if seen[id] {
			return
		}
		seen[id] = true
		result = append(result, id)
		for _, b := range adj[id] {
			add(b)
		}
	}
	// A two-node flat component can be packed in either direction. Put
	// its only connection to another rank on the shared baseline. This
	// keeps the entry and exit of a state with a reciprocal peer straight.
	only := func(links []graph.NodeID, other graph.NodeID) bool {
		for _, child := range links {
			if child != other {
				return false
			}
		}
		return true
	}
	start := func(id graph.NodeID) {
		if seen[id] {
			return
		}
		links := adj[id]
		if len(links) != 0 && !p.crossRankEdge[id] {
			peer := links[0]
			if p.crossRankEdge[peer] && only(links, peer) && only(adj[peer], id) {
				id = peer
			}
		}
		add(id)
	}
	// Starting at the end of a flat path avoids folding its second half
	// behind the first. Isolated siblings retain their crossing order.
	for _, id := range ids {
		if len(adj[id]) < 2 {
			start(id)
		}
	}
	for _, id := range ids {
		start(id)
	}
	return result
}

func (p *rankLayout) overlapInFlow(a, b geometry.Rectangle) bool {
	return a.X < b.MaxX()+p.layerGap && b.X < a.MaxX()+p.layerGap
}

// Keep consecutive blocks on the same native centerline together while their
// new flow bands are disjoint. Narrowing boxes in one branch can otherwise
// introduce a bend into a straight connection in a parallel branch.
func (p *rankLayout) alignedSiblings(ids []graph.NodeID) []graph.NodeID {
	first := ids[0]
	if p.expanded[first] {
		return ids[:1]
	}
	center := p.axis(p.old[first]).Center().Y
	for i := 1; i < len(ids); i++ {
		id := ids[i]
		if p.expanded[id] || math.Abs(p.axis(p.old[id]).Center().Y-center) > 1e-7 {
			return ids[:i]
		}
		foot := p.footprint(id, p.box[id])
		for _, prior := range ids[:i] {
			if p.overlapInFlow(foot, p.footprint(prior, p.box[prior])) {
				return ids[:i]
			}
		}
	}
	return ids
}

func (p *rankLayout) pack(order []graph.NodeID) {
	m := p.m
	last := 0
	for _, id := range p.blocks {
		last = max(last, p.rank[id])
	}
	half := make([]float64, last+1)
	gap := p.gap
	for _, e := range m.Edges {
		for _, id := range e.Labels {
			r := p.axis(m.Label(id).Shape.Rectangle())
			gap = math.Max(gap, r.Height+12)
			p.layerGap = math.Max(p.layerGap, r.Width+12)
		}
	}
	type extent struct {
		first, last   int
		before, after float64
	}
	extents := make([]extent, len(m.Nodes))
	for i := len(order) - 1; i > 0; i-- {
		id := order[i]
		if !p.expanded[*m.Node(id).Parent] {
			continue
		}
		var e extent
		if !p.expanded[id] {
			r := p.axis(p.old[id])
			ink := p.footprint(id, r)
			e = extent{p.rank[id], p.rank[id], r.Center().X - ink.X, ink.MaxX() - r.Center().X}
			if p.reverse {
				e.before, e.after = e.after, e.before
			}
		} else {
			e.first, e.last = len(half), -1
			for _, child := range m.Node(id).Children {
				k := extents[child]
				if k.first < e.first {
					e.first, e.before = k.first, k.before
				} else if k.first == e.first {
					e.before = max(e.before, k.before)
				}
				if k.last > e.last {
					e.last, e.after = k.last, k.after
				} else if k.last == e.last {
					e.after = max(e.after, k.after)
				}
			}
			pad := graph.Get(&m.Node(id).Properties, core.Padding)
			left, right := pad.Left, pad.Right
			if p.vertical {
				left, right = pad.Top, pad.Bottom
			}
			if p.reverse {
				left, right = right, left
			}
			e.before += left
			e.after += right
		}
		extents[id] = e
		half[e.first] = max(half[e.first], e.before)
		half[e.last] = max(half[e.last], e.after)
	}
	centers := make([]float64, len(half))
	for k := 1; k < len(half); k++ {
		centers[k] = centers[k-1] + half[k-1] + p.layerGap + half[k]
	}
	for _, id := range p.blocks {
		r := p.axis(p.old[id])
		c := centers[p.rank[id]]
		if p.reverse {
			c = -c
		}
		r.X = c - r.Width/2
		r.Y = -r.Height / 2
		p.box[id] = r
	}
	// Start each block at a common transverse center. Siblings on disjoint
	// ranks can keep their straight connections even when their sizes differ.
	// Only move a block when it would overlap an earlier sibling's footprint.
	for i := len(order) - 1; i >= 0; i-- {
		id := order[i]
		if !p.expanded[id] {
			continue
		}
		ids := p.children(id)
		var bounds geometry.Rectangle
		for j := 0; j < len(ids); {
			aligned := p.alignedSiblings(ids[j:])
			shift := 0.
			for _, child := range aligned {
				foot := p.footprint(child, p.box[child])
				for _, prev := range ids[:j] {
					s := p.footprint(prev, p.box[prev])
					if p.overlapInFlow(foot, s) {
						shift = math.Max(shift, s.MaxY()+gap-foot.Y)
					}
				}
			}
			for i, child := range aligned {
				r := p.box[child]
				r.Y += shift
				p.box[child] = r
				foot := p.footprint(child, r)
				if j+i == 0 {
					bounds = foot
				} else {
					bounds = bounds.Union(foot)
				}
			}
			j += len(aligned)
		}
		pad := graph.Get(&m.Node(id).Properties, core.Padding)
		before, after, low, high := pad.Left, pad.Right, pad.Top, pad.Bottom
		if p.vertical {
			before, after, low, high = pad.Top, pad.Bottom, pad.Left, pad.Right
		}
		minimum := graph.Get(&m.Node(id).Properties, core.NodeSizeMinimum)
		minFlow, minCross := minimum.X, minimum.Y
		if p.vertical {
			minFlow, minCross = minimum.Y, minimum.X
		}
		flow := math.Max(bounds.Width+before+after, minFlow)
		cross := math.Max(bounds.Height+low+high, minCross)
		// Carry the children's shared transverse reference into the next
		// packing level. Store their offsets relative to the resized parent
		// so an entire subtree can move together if it encounters a sibling.
		crossStart := bounds.Y - low
		for _, child := range ids {
			r := p.box[child]
			r.Y -= crossStart
			p.box[child] = r
		}
		p.box[id] = geometry.Rectangle{X: bounds.X - before, Y: crossStart, Width: flow, Height: cross}
	}
	start := p.box[m.Root].X
	p.box[m.Root].Y = 0
	for _, id := range order {
		if id != m.Root && !p.expanded[*m.Node(id).Parent] {
			continue
		}
		r := p.box[id]
		r.X -= start
		if par := m.Node(id).Parent; par != nil {
			r.Y += p.axis(p.box[*par]).Y
		}
		p.box[id] = p.axis(r)
	}
}

func (p *rankLayout) transfer(order []graph.NodeID) {
	m := p.m
	for _, id := range order {
		n := m.Node(id)
		if par := n.Parent; par != nil && !p.expanded[*par] {
			p.box[id] = p.old[id].Move(p.box[*par].Position().Sub(p.old[*par].Position()))
			continue
		}
		r := p.box[id]
		if par := n.Parent; par != nil {
			r = r.Move(p.box[*par].Position().Negate())
		}
		n.Shape.SetLocation(r.X, r.Y)
		if !p.expanded[id] {
			continue
		}
		// Preserve the attachment of parent decorations to the nearer side
		// as enclosing boxes grow or shrink.
		shift := func(s *graph.Shape) {
			if s.X+s.Width/2 > n.Shape.Width/2 {
				s.X += r.Width - n.Shape.Width
			}
			if s.Y+s.Height/2 > n.Shape.Height/2 {
				s.Y += r.Height - n.Shape.Height
			}
		}
		for _, pid := range n.Ports {
			shift(&m.Port(pid).Shape)
		}
		for _, lid := range n.Labels {
			shift(&m.Label(lid).Shape)
		}
		n.Shape.SetDimensions(r.Width, r.Height)
	}
}
