// Copyright (c) 2012, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"cmp"
	"math"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// This file is PlaceBrandesKoepf over dense per-node arrays. Brandes-Koepf
// identifies the nodes of the graph it places by the scratch IDs that
// buildNeighborhoodInformation assigns: layer by layer, and within a layer in
// node order. So the node at position p of layer L has scratch ID
// start[L]+p, its neighbors in the layer are the adjacent scratch IDs, and
// every per-node value BK reads (size, margins, type, layer, position, the
// neighbor lists and the incident edges with their other ends) can be
// gathered once into arrays indexed by scratch ID. Root, Align and Sink hold
// scratch IDs instead of node IDs: for the nodes of the graph, node <->
// scratch ID is a bijection and arena.Node(x).ID is x's scratch ID during
// the whole placement (nothing reassigns IDs), so every comparison and every
// array index is the same.
//
// The dense path is taken only when every node of the graph is listed once,
// in the layer its Layer field names, and every neighbor and every other end
// of an edge incident to a node of the graph is itself a node of the graph
// (the checks in build); otherwise PlaceBrandesKoepf runs the original code,
// which then behaves exactly as before for such graphs.
//
// Every floating-point expression is the original's, term by term in the
// same order; loops visit nodes, layers, edges and layouts in the original
// order; the threshold strategy's queue and stack and the compactor's
// classes are processed in the same order. Values the original keeps as
// optional entries and only ever reads after setting them are plain values
// here (see the notes at each array); Y keeps its set flag because
// placeBlock tests it, and Shift keeps its optional entries because a class
// that is never reached in placeClasses leaves an unset shift, which the
// original reports by panicking when the final loop reads it.

// bkEdgeEnd is one edge incident to a node, in NodeConnectedEdges order (per
// port: incoming edges, then outgoing edges).
type bkEdgeEnd struct {
	edge     layered.LEdgeID
	other    int32 // EdgeOtherNode(edge, node) as a scratch ID
	incoming bool
	inLayer  bool // EdgeIsInLayer(edge)
}

type bkNeighbor struct {
	node int32 // scratch ID
	edge layered.LEdgeID
}

// bkDense is the per-node data of one placement, indexed by scratch ID.
type bkDense struct {
	n      int
	layers int
	node   []layered.LNodeID
	layer  []int32
	start  []int32 // first scratch ID of each layer; start[layers] == n

	sizeY, marginTop, marginBottom []float64
	longEdge                       []bool
	nodeType                       []layered.NodeType
	// individual: the node has an individual spacing (SpacingIndividual).
	individual []bool
	// spacing caches SpacingCache.Vertical per pair of node types for nodes
	// without individual spacings: it is then the graph's value of the
	// pair's spacing property, which nothing changes during placement.
	spacing      [6][6]float64
	spacingKnown [6][6]bool

	left, right []([]bkNeighbor)
	ends        [][]bkEdgeEnd

	neighborBacking []bkNeighbor
	endBacking      []bkEdgeEnd

	layouts [5]bkDenseLayout
	order   []*bkDenseLayout
	ints    []int32
	floats  []float64
	bools   []bool
	shifts  []optFloat

	roots []int32
	seen  []bool
	used  int

	// threshold strategy state (ThresholdStrategy's, by scratch ID)
	simple           bool
	blockFinished    []bool
	postprocessQueue []bkPost
	postprocessStack []bkPost
}

// bkDenseLayout is BKAlignedLayout by scratch ID. Root, Align, InnerShift
// (verticalAlignment), BlockSize (InsideBlockShift, for every root) and Sink
// (HorizontalCompaction) are set for every node before they are read, so
// they are plain values; Y is reset to unset by HorizontalCompaction and
// tested by placeBlock, so it has a set flag.
type bkDenseLayout struct {
	root, align, sink     []int32
	blockSize, innerShift []float64
	y                     []float64
	ySet                  []bool
	shift                 []optFloat
	su, od                []bool
	v                     VerticalDirection
	h                     HorizontalDirection
}

type bkPost struct {
	free        int32
	isRoot      bool
	hasEdges    bool
	edge        layered.LEdgeID
	edgeInLayer bool
	hasEdge     bool
}

// bkDenseDisabled forces the original implementation (for tests).
var bkDenseDisabled bool

func (d *bkDense) scratch(arena *layered.LGraphArena, nodeID layered.LNodeID) (int32, bool) {
	s := arena.Node(nodeID).ID
	if s < 0 || s >= d.n || d.node[s] != nodeID {
		return 0, false
	}
	return int32(s), true
}

// build gathers the graph's node data after buildNeighborhoodInformation
// has assigned the scratch IDs, and reports whether the dense path applies.
// It only reads the arena.
func (d *bkDense) build(arena *layered.LGraphArena, graphID layered.LGraphID, info *NeighborhoodInformation) bool {
	layers := arena.Graph(graphID).Layers
	n := info.NodeCount
	d.n, d.layers = n, len(layers)
	d.node = resizeNodeIDs(d.node, n)
	d.layer = resizeInt32s(d.layer, n)
	d.start = resizeInt32s(d.start, len(layers)+1)
	d.sizeY = resizeFloats(d.sizeY, n)
	d.marginTop = resizeFloats(d.marginTop, n)
	d.marginBottom = resizeFloats(d.marginBottom, n)
	d.longEdge = resizeBools(d.longEdge, n)
	d.nodeType = resizeNodeTypes(d.nodeType, n)
	d.individual = resizeBools(d.individual, n)
	d.spacingKnown = [6][6]bool{}
	s := 0
	for layerIndex, layerID := range layers {
		d.start[layerIndex] = int32(s)
		// verticalSpacing keeps one value per pair of node types, the graph's:
		// a node's spacings come from the graph its layer names.
		if arena.Layer(layerID).Graph != graphID {
			return false
		}
		for _, nodeID := range arena.Layer(layerID).Nodes {
			node := arena.Node(nodeID)
			// The original indexes every per-node array by the node's scratch
			// ID and finds a node's layer (its size and its neighbors in it)
			// through node.Layer. Arrays by position are the same only if
			// every node is listed once (its scratch ID is then its position;
			// a node listed twice keeps its last position's ID) and node.Layer
			// is the layer listing it. Both can fail: ELK's hierarchical sweep
			// can write an external-port dummy into a second layer list.
			if node.ID != s || node.Layer == nil || *node.Layer != layerID {
				return false
			}
			d.node[s] = nodeID
			d.layer[s] = int32(layerIndex)
			d.sizeY[s] = node.Size.Y
			d.marginTop[s] = node.Margin.Top
			d.marginBottom[s] = node.Margin.Bottom
			d.longEdge[s] = node.Type == layered.LongEdgeNode
			d.nodeType[s] = node.Type
			_, d.individual[s] = basegraph.TryGet(&node.Properties, layered.SpacingIndividual)
			s++
		}
	}
	d.start[len(layers)] = int32(s)
	if s != n {
		return false
	}

	// Neighbor lists by scratch ID; every neighbor must be a node of the graph.
	if cap(d.left) < n {
		d.left = make([][]bkNeighbor, n)
		d.right = make([][]bkNeighbor, n)
	} else {
		d.left, d.right = d.left[:n], d.right[:n]
	}
	backing := d.neighborBacking[:0]
	for scratchID := 0; scratchID < n; scratchID++ {
		for side := 0; side < 2; side++ {
			list := info.LeftNeighbors[scratchID]
			if side == 1 {
				list = info.RightNeighbors[scratchID]
			}
			start := len(backing)
			for _, neighbor := range list {
				other, ok := d.scratch(arena, neighbor.Node)
				if !ok {
					d.neighborBacking = backing
					return false
				}
				backing = append(backing, bkNeighbor{node: other, edge: neighbor.Edge})
			}
			if side == 0 {
				d.left[scratchID] = backing[start:len(backing):len(backing)]
			} else {
				d.right[scratchID] = backing[start:len(backing):len(backing)]
			}
		}
	}
	d.neighborBacking = backing

	// Incident edges in NodeConnectedEdges order with their other ends.
	if cap(d.ends) < n {
		d.ends = make([][]bkEdgeEnd, n)
	} else {
		d.ends = d.ends[:n]
	}
	ends := d.endBacking[:0]
	for scratchID := 0; scratchID < n; scratchID++ {
		nodeID := d.node[scratchID]
		start := len(ends)
		for _, portID := range arena.Node(nodeID).Ports {
			port := arena.Port(portID)
			for direction, edges := range [2][]layered.LEdgeID{port.IncomingEdges, port.OutgoingEdges} {
				for _, edgeID := range edges {
					edge := arena.Edge(edgeID)
					if edge.Source == nil || edge.Target == nil {
						d.endBacking = ends
						return false
					}
					sourcePort, targetPort := arena.Port(*edge.Source), arena.Port(*edge.Target)
					if sourcePort.Node == nil || targetPort.Node == nil {
						d.endBacking = ends
						return false
					}
					source, sourceOK := d.scratch(arena, *sourcePort.Node)
					target, targetOK := d.scratch(arena, *targetPort.Node)
					if !sourceOK || !targetOK {
						d.endBacking = ends
						return false
					}
					incoming := direction == 0
					// EdgeOtherNode: the source unless it is the node itself,
					// in which case the target (also the node for a self-loop).
					other := source
					if source == int32(scratchID) {
						other = target
					}
					if incoming && target != int32(scratchID) || !incoming && source != int32(scratchID) {
						d.endBacking = ends
						return false
					}
					ends = append(ends, bkEdgeEnd{
						edge:     edgeID,
						other:    other,
						incoming: incoming,
						inLayer:  source != target && d.layer[source] == d.layer[target],
					})
				}
			}
		}
		d.ends[scratchID] = ends[start:len(ends):len(ends)]
	}
	d.endBacking = ends
	return true
}

func (d *bkDense) position(s int32) int32 { return s - d.start[d.layer[s]] }

func (d *bkDense) layerSize(layer int) int { return int(d.start[layer+1] - d.start[layer]) }

// place is PlaceBrandesKoepf after buildNeighborhoodInformation.
func (d *bkDense) place(arena *layered.LGraphArena, graphID layered.LGraphID, workspace *bkWorkspace) {
	alignment := basegraph.Get(&arena.Graph(graphID).Properties, layered.BKFixedAlignment)
	favorStraightEdges := basegraph.Get(&arena.Graph(graphID).Properties, layered.FavorStraightEdges)
	produceBalanced := alignment == layered.FixedAlignmentNone && !favorStraightEdges ||
		alignment == layered.FixedAlignmentBalanced
	markedEdges := &workspace.marked
	markedEdges.clear()
	d.markConflicts(markedEdges)
	layouts := d.createDirectionalLayouts(alignment)
	for _, layout := range layouts {
		d.verticalAlignment(layout, markedEdges)
		d.insideBlockShift(arena, layout)
	}
	compactor := &workspace.compact
	compactor.reset(arena, graphID)
	d.simple = compactor.threshold.simple
	for _, layout := range layouts {
		d.horizontalCompaction(arena, graphID, compactor, layout)
	}
	var chosen *bkDenseLayout
	if produceBalanced {
		balanced := d.fillBalancedLayout(layouts, d.newLayout(VerticalBalanced, HorizontalBalanced))
		if d.checkOrderConstraint(balanced) {
			chosen = balanced
		}
	}
	if chosen == nil {
		for _, layout := range layouts {
			if !d.checkOrderConstraint(layout) {
				continue
			}
			if chosen == nil || d.layoutSize(chosen) > d.layoutSize(layout) {
				chosen = layout
			}
		}
	}
	if chosen == nil {
		chosen = layouts[0]
	}
	for s := 0; s < d.n; s++ {
		arena.Node(d.node[s]).Position.Y = chosen.y[s] + chosen.innerShift[s]
	}
}

// createDirectionalLayouts is createDirectionalLayoutsIn over dense layouts.
func (d *bkDense) createDirectionalLayouts(alignment layered.FixedAlignment) []*bkDenseLayout {
	directions := [4][2]uint8{
		{uint8(VerticalDown), uint8(HorizontalRight)}, {uint8(VerticalUp), uint8(HorizontalRight)},
		{uint8(VerticalDown), uint8(HorizontalLeft)}, {uint8(VerticalUp), uint8(HorizontalLeft)},
	}
	count := 4
	switch alignment {
	case layered.FixedAlignmentLeftDown:
		directions[0], count = [2]uint8{uint8(VerticalDown), uint8(HorizontalLeft)}, 1
	case layered.FixedAlignmentLeftUp:
		directions[0], count = [2]uint8{uint8(VerticalUp), uint8(HorizontalLeft)}, 1
	case layered.FixedAlignmentRightDown:
		directions[0], count = [2]uint8{uint8(VerticalDown), uint8(HorizontalRight)}, 1
	case layered.FixedAlignmentRightUp:
		directions[0], count = [2]uint8{uint8(VerticalUp), uint8(HorizontalRight)}, 1
	}
	d.reserve(count + 1)
	layouts := d.order[:0]
	for index := 0; index < count; index++ {
		layouts = append(layouts, d.newLayout(VerticalDirection(directions[index][0]), HorizontalDirection(directions[index][1])))
	}
	d.order = layouts
	return layouts
}

// reserve prepares storage for count layouts (contents are written before
// they are read, see bkDenseLayout).
func (d *bkDense) reserve(count int) {
	n := d.n
	d.ints = resizeInt32s(d.ints, 3*n*count)
	d.floats = resizeFloats(d.floats, 3*n*count)
	d.bools = resizeBools(d.bools, 3*n*count)
	d.shifts = resizeOptFloats(d.shifts, n*count)
	for index := range d.layouts {
		d.layouts[index] = bkDenseLayout{}
	}
	d.used = 0
}

func (d *bkDense) newLayout(vertical VerticalDirection, horizontal HorizontalDirection) *bkDenseLayout {
	n, slot := d.n, d.used
	d.used++
	cut := func(index int) (int, int) { return (3*slot + index) * n, (3*slot + index + 1) * n }
	layout := &d.layouts[slot]
	r0, r1 := cut(0)
	a0, a1 := cut(1)
	s0, s1 := cut(2)
	b0, b1 := cut(0)
	i0, i1 := cut(1)
	y0, y1 := cut(2)
	u0, u1 := cut(0)
	o0, o1 := cut(1)
	f0, f1 := cut(2)
	*layout = bkDenseLayout{
		root: d.ints[r0:r1:r1], align: d.ints[a0:a1:a1], sink: d.ints[s0:s1:s1],
		blockSize: d.floats[b0:b1:b1], innerShift: d.floats[i0:i1:i1], y: d.floats[y0:y1:y1],
		su: d.bools[u0:u1:u1], od: d.bools[o0:o1:o1], ySet: d.bools[f0:f1:f1],
		shift: d.shifts[slot*n : (slot+1)*n : (slot+1)*n],
		v:     vertical, h: horizontal,
	}
	// NewBKAlignedLayout: SU all false, OD all true, every optional entry unset.
	for index := range layout.su {
		layout.su[index] = false
		layout.od[index] = true
		layout.ySet[index] = false
	}
	for index := range layout.shift {
		layout.shift[index] = optFloat{}
	}
	return layout
}

func (d *bkDense) markConflicts(markedEdges *edgeMarks) {
	if d.layers < minimumLayersForConflicts {
		return
	}
	for layerIndex := 1; layerIndex < d.layers-1; layerIndex++ {
		next := layerIndex + 1
		nextSize := d.layerSize(next)
		k0, lowerStart := int32(0), 0
		for lowerEnd := 0; lowerEnd < nextSize; lowerEnd++ {
			s := d.start[next] + int32(lowerEnd)
			inner := d.incidentToInnerSegment(s, int32(next), int32(layerIndex))
			if lowerEnd != nextSize-1 && !inner {
				continue
			}
			k1 := int32(d.layerSize(layerIndex) - 1)
			if inner {
				neighbors := d.left[s]
				k1 = d.position(neighbors[0].node)
			}
			for lowerStart <= lowerEnd {
				current := d.start[next] + int32(lowerStart)
				if !d.incidentToInnerSegment(current, int32(next), int32(layerIndex)) {
					for _, neighbor := range d.left[current] {
						position := d.position(neighbor.node)
						if position < k0 || position > k1 {
							markedEdges.mark(neighbor.edge)
						}
					}
				}
				lowerStart++
			}
			k0 = k1
		}
	}
}

func (d *bkDense) incidentToInnerSegment(s, layer1, layer2 int32) bool {
	if !d.longEdge[s] {
		return false
	}
	for _, end := range d.ends[s] {
		if !end.incoming {
			continue
		}
		source := end.other
		if d.longEdge[source] && d.layer[source] == layer2 && d.layer[s] == layer1 {
			return true
		}
	}
	return false
}

func (d *bkDense) verticalAlignment(layout *bkDenseLayout, markedEdges *edgeMarks) {
	for s := 0; s < d.n; s++ {
		layout.root[s] = int32(s)
		layout.align[s] = int32(s)
		layout.innerShift[s] = 0
	}
	for layerStep := 0; layerStep < d.layers; layerStep++ {
		layerIndex := layerStep
		if layout.h == HorizontalLeft {
			layerIndex = d.layers - 1 - layerStep
		}
		r := -1
		if layout.v == VerticalUp {
			r = math.MaxInt
		}
		size := d.layerSize(layerIndex)
		for nodeStep := 0; nodeStep < size; nodeStep++ {
			position := nodeStep
			if layout.v == VerticalUp {
				position = size - 1 - nodeStep
			}
			s := d.start[layerIndex] + int32(position)
			neighbors := d.left[s]
			if layout.h == HorizontalLeft {
				neighbors = d.right[s]
			}
			if len(neighbors) == 0 {
				continue
			}
			count := float64(len(neighbors))
			low := int(math.Floor((count+1)/2)) - 1
			high := int(math.Ceil((count+1)/2)) - 1
			if layout.v == VerticalUp {
				for median := high; median >= low; median-- {
					r = d.tryAlign(layout, markedEdges, s, neighbors[median], r, true)
				}
			} else {
				for median := low; median <= high; median++ {
					r = d.tryAlign(layout, markedEdges, s, neighbors[median], r, false)
				}
			}
		}
	}
}

func (d *bkDense) tryAlign(layout *bkDenseLayout, markedEdges *edgeMarks, s int32, neighbor bkNeighbor, r int, up bool) int {
	if layout.align[s] != s {
		return r
	}
	neighborPosition := int(d.position(neighbor.node))
	positionAllowed := r < neighborPosition
	if up {
		positionAllowed = r > neighborPosition
	}
	if markedEdges.has(neighbor.edge) || !positionAllowed {
		return r
	}
	layout.align[neighbor.node] = s
	root := layout.root[neighbor.node]
	layout.root[s] = root
	layout.align[s] = root
	layout.od[root] = layout.od[root] && d.longEdge[s]
	return neighborPosition
}

func (d *bkDense) insideBlockShift(arena *layered.LGraphArena, layout *bkDenseLayout) {
	n := d.n
	roots := d.roots[:0]
	if cap(d.seen) < n {
		d.seen = make([]bool, n)
	} else {
		d.seen = d.seen[:n]
		clear(d.seen)
	}
	for s := 0; s < n; s++ {
		root := layout.root[s]
		if !d.seen[root] {
			d.seen[root] = true
			roots = append(roots, root)
		}
	}
	d.roots = roots
	for _, root := range roots {
		spaceAbove := d.marginTop[root]
		spaceBelow := d.sizeY[root] + d.marginBottom[root]
		layout.innerShift[root] = 0
		current := root
		for {
			next := layout.align[current]
			if next == root {
				break
			}
			edgeID, found := d.edgeBetween(current, next)
			if !found {
				panic("p4nodes: aligned nodes are not connected")
			}
			edge := arena.Edge(edgeID)
			source, target := arena.Port(*edge.Source), arena.Port(*edge.Target)
			portPositionDifference := source.Position.Y + source.Anchor.Y - target.Position.Y - target.Anchor.Y
			if layout.h == HorizontalLeft {
				portPositionDifference = target.Position.Y + target.Anchor.Y - source.Position.Y - source.Anchor.Y
			}
			nextInnerShift := layout.innerShift[current] + portPositionDifference
			layout.innerShift[next] = nextInnerShift
			spaceAbove = maxFloat(spaceAbove, d.marginTop[next]-nextInnerShift)
			spaceBelow = maxFloat(spaceBelow, nextInnerShift+d.sizeY[next]+d.marginBottom[next])
			current = next
		}
		current = root
		for {
			layout.innerShift[current] = layout.innerShift[current] + spaceAbove
			current = layout.align[current]
			if current == root {
				break
			}
		}
		layout.blockSize[root] = spaceAbove + spaceBelow
	}
}

// edgeBetween is edgeBetween: the first connected edge of first whose other
// end is second.
func (d *bkDense) edgeBetween(first, second int32) (layered.LEdgeID, bool) {
	for _, end := range d.ends[first] {
		if end.other == second {
			return end.edge, true
		}
	}
	return 0, false
}

func (d *bkDense) horizontalCompaction(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	compactor *BKCompactor,
	layout *bkDenseLayout,
) {
	n := d.n
	shift := math.Inf(1)
	if layout.v == VerticalUp {
		shift = math.Inf(-1)
	}
	for s := 0; s < n; s++ {
		layout.sink[s] = int32(s)
		layout.shift[s] = someFloat(shift)
	}
	compactor.clearClasses()
	compactor.classNodes = compactor.classNodes[:0]
	if !compactor.spacings.BoundTo(arena, graphID) {
		compactor.spacings.Reset(arena, graphID)
	}
	leftward := layout.h == HorizontalLeft
	d.initThreshold(n)
	for s := 0; s < n; s++ {
		layout.ySet[s] = false
	}
	for layerStep := 0; layerStep < d.layers; layerStep++ {
		layerIndex := layerStep
		if leftward {
			layerIndex = d.layers - 1 - layerStep
		}
		size := d.layerSize(layerIndex)
		for nodeStep := 0; nodeStep < size; nodeStep++ {
			position := nodeStep
			if layout.v == VerticalUp {
				position = size - 1 - nodeStep
			}
			s := d.start[layerIndex] + int32(position)
			if layout.root[s] == s {
				d.placeBlock(arena, graphID, compactor, layout, s)
			}
		}
	}
	d.placeClasses(compactor, layout)
	for layerStep := 0; layerStep < d.layers; layerStep++ {
		layerIndex := layerStep
		if leftward {
			layerIndex = d.layers - 1 - layerStep
		}
		for s := d.start[layerIndex]; s < d.start[layerIndex+1]; s++ {
			root := layout.root[s]
			layout.y[s] = layout.y[root]
			layout.ySet[s] = true
			if root == s {
				sink := layout.sink[s]
				sinkShift := requireFloat(layout.shift[sink], "shift")
				if layout.v == VerticalUp && !math.IsInf(sinkShift, -1) ||
					layout.v == VerticalDown && !math.IsInf(sinkShift, 1) {
					layout.y[s] = layout.y[s] + sinkShift
				}
			}
		}
	}
	d.postProcess(arena, layout)
}

func (d *bkDense) placeBlock(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	compactor *BKCompactor,
	layout *bkDenseLayout,
	root int32,
) {
	if layout.ySet[root] {
		return
	}
	initialAssignment := true
	layout.y[root] = 0
	layout.ySet[root] = true
	current := root
	threshold := math.Inf(1)
	if layout.v == VerticalDown {
		threshold = math.Inf(-1)
	}
	for {
		layerIndex := d.layer[current]
		currentPosition := current - d.start[layerIndex]
		layerSize := d.start[layerIndex+1] - d.start[layerIndex]
		hasNeighbor := layout.v == VerticalDown && currentPosition > 0 ||
			layout.v == VerticalUp && currentPosition < layerSize-1
		if hasNeighbor {
			neighbor := current - 1
			if layout.v == VerticalUp {
				neighbor = current + 1
			}
			neighborRoot := layout.root[neighbor]
			d.placeBlock(arena, graphID, compactor, layout, neighborRoot)
			threshold = d.calculateThreshold(arena, layout, threshold, root, current)
			if layout.sink[root] == root {
				layout.sink[root] = layout.sink[neighborRoot]
			}
			rootSink := layout.sink[root]
			neighborSink := layout.sink[neighborRoot]
			if rootSink == neighborSink {
				spacing := d.verticalSpacing(compactor, current, neighbor)
				currentPositionValue := layout.y[root]
				var newPosition float64
				if layout.v == VerticalUp {
					newPosition = layout.y[neighborRoot] +
						layout.innerShift[neighbor] -
						d.marginTop[neighbor] - spacing - d.marginBottom[current] -
						d.sizeY[current] - layout.innerShift[current]
					value := minFloat(newPosition, threshold)
					if !initialAssignment {
						value = minFloat(currentPositionValue, value)
					}
					layout.y[root] = value
				} else {
					newPosition = layout.y[neighborRoot] +
						layout.innerShift[neighbor] + d.sizeY[neighbor] +
						d.marginBottom[neighbor] + spacing + d.marginTop[current] -
						layout.innerShift[current]
					value := maxFloat(newPosition, threshold)
					if !initialAssignment {
						value = maxFloat(currentPositionValue, value)
					}
					layout.y[root] = value
				}
				initialAssignment = false
			} else {
				spacing := basegraph.Get(&arena.Graph(graphID).Properties, layered.SpacingNodeNode)
				sinkNode := compactor.classNode(arena, d.node[rootSink])
				neighborClass := compactor.classNode(arena, d.node[neighborSink])
				var requiredSpace float64
				if layout.v == VerticalUp {
					requiredSpace = layout.y[root] + layout.innerShift[current] +
						d.sizeY[current] + d.marginBottom[current] + spacing -
						(layout.y[neighborRoot] + layout.innerShift[neighbor] - d.marginTop[neighbor])
				} else {
					requiredSpace = layout.y[root] + layout.innerShift[current] -
						d.marginTop[current] - layout.y[neighborRoot] -
						layout.innerShift[neighbor] - d.sizeY[neighbor] -
						d.marginBottom[neighbor] - spacing
				}
				compactor.addClassEdge(sinkNode, neighborClass, requiredSpace)
			}
		} else {
			threshold = d.calculateThreshold(arena, layout, threshold, root, current)
		}
		current = layout.align[current]
		if current == root {
			break
		}
	}
	d.blockFinished[root] = true
}

// verticalSpacing is compactor.spacings.Vertical for two nodes of the graph.
func (d *bkDense) verticalSpacing(compactor *BKCompactor, first, second int32) float64 {
	firstType, secondType := d.nodeType[first], d.nodeType[second]
	if d.individual[first] || d.individual[second] || firstType >= 6 || secondType >= 6 {
		return compactor.spacings.Vertical(d.node[first], d.node[second])
	}
	if d.spacingKnown[firstType][secondType] {
		return d.spacing[firstType][secondType]
	}
	value := compactor.spacings.Vertical(d.node[first], d.node[second])
	d.spacing[firstType][secondType], d.spacingKnown[firstType][secondType] = value, true
	return value
}

// placeClasses is BKCompactor.placeClasses writing the dense layout's shifts.
// The compactor lists the sinks' scratch IDs in sinkScratch, in sinkOrder's
// order (classNode appends to both).
func (d *bkDense) placeClasses(compactor *BKCompactor, layout *bkDenseLayout) {
	queue := compactor.queue[:0]
	for _, scratchID := range compactor.sinkScratch {
		index := int(compactor.sinkClass[scratchID]) - 1
		if compactor.classNodes[index].indegree == 0 {
			queue = append(queue, index)
		}
	}
	for head := 0; head < len(queue); head++ {
		index := queue[head]
		if !compactor.classNodes[index].classShift.set {
			compactor.classNodes[index].classShift = someFloat(0)
		}
		shift := compactor.classNodes[index].classShift.value
		for _, edge := range compactor.classNodes[index].outgoing {
			target := &compactor.classNodes[edge.target]
			candidate := shift + edge.separation
			if !target.classShift.set {
				target.classShift = someFloat(candidate)
			} else if layout.v == VerticalDown {
				target.classShift = someFloat(minFloat(target.classShift.value, candidate))
			} else {
				target.classShift = someFloat(maxFloat(target.classShift.value, candidate))
			}
			target.indegree--
			if target.indegree == 0 {
				queue = append(queue, edge.target)
			}
		}
	}
	compactor.queue = queue[:0]
	for _, scratchID := range compactor.sinkScratch {
		class := compactor.classNodes[int(compactor.sinkClass[scratchID])-1]
		layout.shift[scratchID] = class.classShift
	}
}

// initThreshold is ThresholdStrategy.initFor(n).
func (d *bkDense) initThreshold(n int) {
	if cap(d.blockFinished) < n {
		d.blockFinished = make([]bool, n)
	} else {
		d.blockFinished = d.blockFinished[:n]
		clear(d.blockFinished)
	}
	d.postprocessQueue = d.postprocessQueue[:0]
	d.postprocessStack = d.postprocessStack[:0]
}

// calculateThreshold is ThresholdStrategy.CalculateThreshold.
func (d *bkDense) calculateThreshold(
	arena *layered.LGraphArena,
	layout *bkDenseLayout,
	oldThreshold float64,
	blockRoot, currentNode int32,
) float64 {
	if !d.simple {
		if layout.v == VerticalUp {
			return math.Inf(1)
		}
		return math.Inf(-1)
	}
	isRoot := blockRoot == currentNode
	isLast := layout.align[currentNode] == blockRoot
	if !isRoot && !isLast {
		return oldThreshold
	}
	threshold := oldThreshold
	if isRoot {
		threshold = d.bound(arena, layout, blockRoot, true)
	}
	if math.IsInf(threshold, 0) && isLast {
		threshold = d.bound(arena, layout, currentNode, false)
	}
	return threshold
}

// pickEdge is ThresholdStrategy.pickEdge.
func (d *bkDense) pickEdge(layout *bkDenseLayout, processable *bkPost) {
	var incoming bool
	if processable.isRoot {
		incoming = layout.h == HorizontalRight
	} else {
		incoming = layout.h == HorizontalLeft
	}
	hasEdges := false
	for _, end := range d.ends[processable.free] {
		if end.incoming != incoming {
			continue
		}
		root := layout.root[processable.free]
		if !layout.od[root] && end.inLayer {
			continue
		}
		if layout.su[root] {
			continue
		}
		hasEdges = true
		otherRoot := layout.root[end.other]
		if d.blockFinished[otherRoot] {
			processable.hasEdges = true
			processable.edge, processable.edgeInLayer, processable.hasEdge = end.edge, end.inLayer, true
			return
		}
	}
	processable.hasEdges = hasEdges
	processable.edge, processable.edgeInLayer, processable.hasEdge = 0, false, false
}

// nodeScratch is the scratch ID of a port's node (a node of the graph: the
// port belongs to an edge incident to one, checked by build).
func (d *bkDense) portScratch(arena *layered.LGraphArena, portID layered.LPortID) int32 {
	return int32(arena.Node(*arena.Port(portID).Node).ID)
}

// bound is ThresholdStrategy.bound.
func (d *bkDense) bound(arena *layered.LGraphArena, layout *bkDenseLayout, blockNode int32, isRoot bool) float64 {
	invalid := math.Inf(-1)
	if layout.v == VerticalUp {
		invalid = math.Inf(1)
	}
	processable := bkPost{free: blockNode, isRoot: isRoot}
	d.pickEdge(layout, &processable)
	if !processable.hasEdge {
		if processable.hasEdges {
			d.postprocessQueue = append(d.postprocessQueue, processable)
		}
		return invalid
	}
	edge := arena.Edge(processable.edge)
	left, right := *edge.Source, *edge.Target
	rootPort, otherPort := left, right
	if isRoot {
		if layout.h == HorizontalRight {
			rootPort, otherPort = right, left
		}
	} else if layout.h == HorizontalLeft {
		rootPort, otherPort = right, left
	}
	otherNode := d.portScratch(arena, otherPort)
	rootPortNode := d.portScratch(arena, rootPort)
	otherRoot := layout.root[otherNode]
	threshold := layout.y[otherRoot] +
		layout.innerShift[otherNode] +
		arena.Port(otherPort).Position.Y + arena.Port(otherPort).Anchor.Y -
		layout.innerShift[rootPortNode] -
		arena.Port(rootPort).Position.Y - arena.Port(rootPort).Anchor.Y
	leftRoot := layout.root[d.portScratch(arena, left)]
	rightRoot := layout.root[d.portScratch(arena, right)]
	layout.su[leftRoot] = true
	layout.su[rightRoot] = true
	return threshold
}

// postProcess is ThresholdStrategy.PostProcess.
func (d *bkDense) postProcess(arena *layered.LGraphArena, layout *bkDenseLayout) {
	if !d.simple {
		return
	}
	for head := 0; head < len(d.postprocessQueue); head++ {
		processable := d.postprocessQueue[head]
		d.pickEdge(layout, &processable)
		if !processable.hasEdge {
			continue
		}
		root := layout.root[processable.free]
		if !layout.od[root] && processable.edgeInLayer {
			continue
		}
		if !d.processPostprocessable(arena, layout, processable) {
			d.postprocessStack = append(d.postprocessStack, processable)
		}
	}
	d.postprocessQueue = d.postprocessQueue[:0]
	for len(d.postprocessStack) > 0 {
		last := len(d.postprocessStack) - 1
		processable := d.postprocessStack[last]
		d.postprocessStack = d.postprocessStack[:last]
		d.processPostprocessable(arena, layout, processable)
	}
}

func (d *bkDense) processPostprocessable(arena *layered.LGraphArena, layout *bkDenseLayout, processable bkPost) bool {
	edgeID := processable.edge
	edge := arena.Edge(edgeID)
	fix, block := *edge.Target, *edge.Source
	if d.portScratch(arena, *edge.Source) != processable.free {
		fix, block = *edge.Source, *edge.Target
	}
	delta := d.calculateDelta(arena, layout, fix, block)
	blockNode := d.portScratch(arena, block)
	if delta > 0 && delta < straighteningThreshold {
		available := d.checkSpaceAbove(arena, layout, blockNode, delta)
		d.shiftBlock(layout, blockNode, -available)
		return available > 0
	}
	if delta < 0 && -delta < straighteningThreshold {
		available := d.checkSpaceBelow(arena, layout, blockNode, -delta)
		d.shiftBlock(layout, blockNode, available)
		return available > 0
	}
	return false
}

// calculateDelta is BKAlignedLayout.CalculateDelta.
func (d *bkDense) calculateDelta(arena *layered.LGraphArena, layout *bkDenseLayout, sourceID, targetID layered.LPortID) float64 {
	sourceNode, targetNode := d.portScratch(arena, sourceID), d.portScratch(arena, targetID)
	source := arena.Port(sourceID)
	target := arena.Port(targetID)
	sourcePosition := layout.y[sourceNode] +
		layout.innerShift[sourceNode] +
		source.Position.Y + source.Anchor.Y
	targetPosition := layout.y[targetNode] +
		layout.innerShift[targetNode] +
		target.Position.Y + target.Anchor.Y
	return targetPosition - sourcePosition
}

func (d *bkDense) shiftBlock(layout *bkDenseLayout, root int32, delta float64) {
	current := root
	for {
		layout.y[current] = layout.y[current] + delta
		current = layout.align[current]
		if current == root {
			return
		}
	}
}

func (d *bkDense) checkSpaceAbove(arena *layered.LGraphArena, layout *bkDenseLayout, blockRoot int32, delta float64) float64 {
	available := delta
	current := blockRoot
	for {
		current = layout.align[current]
		minimum := d.minY(layout, current)
		if d.position(current) > 0 {
			neighbor := current - 1
			available = minFloat(available, minimum-(d.maxY(layout, neighbor)+layered.VerticalSpacing(arena, d.node[current], d.node[neighbor])))
		}
		if current == blockRoot {
			return available
		}
	}
}

func (d *bkDense) checkSpaceBelow(arena *layered.LGraphArena, layout *bkDenseLayout, blockRoot int32, delta float64) float64 {
	available := delta
	current := blockRoot
	for {
		current = layout.align[current]
		maximum := d.maxY(layout, current)
		if layerIndex := d.layer[current]; current < d.start[layerIndex+1]-1 {
			neighbor := current + 1
			available = minFloat(available, d.minY(layout, neighbor)-(maximum+layered.VerticalSpacing(arena, d.node[current], d.node[neighbor])))
		}
		if current == blockRoot {
			return available
		}
	}
}

func (d *bkDense) minY(layout *bkDenseLayout, s int32) float64 {
	root := layout.root[s]
	return layout.y[root] + layout.innerShift[s] - d.marginTop[s]
}

func (d *bkDense) maxY(layout *bkDenseLayout, s int32) float64 {
	root := layout.root[s]
	return layout.y[root] + layout.innerShift[s] + d.sizeY[s] + d.marginBottom[s]
}

// layoutSize is BKAlignedLayout.LayoutSize.
func (d *bkDense) layoutSize(layout *bkDenseLayout) float64 {
	minimum, maximum := math.Inf(1), math.Inf(-1)
	for s := 0; s < d.n; s++ {
		yMin := layout.y[s]
		root := layout.root[s]
		yMax := yMin + layout.blockSize[root]
		minimum = minFloat(minimum, yMin)
		maximum = maxFloat(maximum, yMax)
	}
	return maximum - minimum
}

func (d *bkDense) checkOrderConstraint(layout *bkDenseLayout) bool {
	for layerIndex := 0; layerIndex < d.layers; layerIndex++ {
		position := math.Inf(-1)
		for s := d.start[layerIndex]; s < d.start[layerIndex+1]; s++ {
			top := layout.y[s] + layout.innerShift[s] - d.marginTop[s]
			bottom := layout.y[s] + layout.innerShift[s] +
				d.sizeY[s] + d.marginBottom[s]
			if top <= position || bottom <= position {
				return false
			}
			position = bottom
		}
	}
	return true
}

// fillBalancedLayout is fillBalancedLayout; the per-node sort of the
// layouts' positions is sort.Float64s' algorithm for slices of at most 12
// elements (insertion sort with cmp.Less), done in place.
func (d *bkDense) fillBalancedLayout(layouts []*bkDenseLayout, balanced *bkDenseLayout) *bkDenseLayout {
	var widthArray, minimumArray, maximumArray, shiftArray, calculatedArray [4]float64
	if len(layouts) > len(widthArray) {
		panic("p4nodes: more than four directional layouts")
	}
	width := widthArray[:len(layouts)]
	minimum := minimumArray[:len(layouts)]
	maximum := maximumArray[:len(layouts)]
	for index := range minimum {
		minimum[index] = float64(math.MaxInt32) // Java initializes min to Integer.MAX_VALUE.
		maximum[index] = float64(math.MinInt32)
	}
	minimumWidthLayout := 0
	for layoutIndex, layout := range layouts {
		width[layoutIndex] = d.layoutSize(layout)
		if width[minimumWidthLayout] > width[layoutIndex] {
			minimumWidthLayout = layoutIndex
		}
		for s := 0; s < d.n; s++ {
			position := layout.y[s] + layout.innerShift[s]
			minimum[layoutIndex] = minFloat(minimum[layoutIndex], position)
			maximum[layoutIndex] = maxFloat(maximum[layoutIndex], position+d.sizeY[s])
		}
	}
	shift := shiftArray[:len(layouts)]
	for index, layout := range layouts {
		if layout.v == VerticalDown {
			shift[index] = minimum[minimumWidthLayout] - minimum[index]
		} else {
			shift[index] = maximum[minimumWidthLayout] - maximum[index]
		}
	}
	calculated := calculatedArray[:len(layouts)]
	for s := 0; s < d.n; s++ {
		for layoutIndex, layout := range layouts {
			calculated[layoutIndex] = layout.y[s] +
				layout.innerShift[s] + shift[layoutIndex]
		}
		for i := 1; i < len(calculated); i++ {
			for j := i; j > 0 && cmp.Less(calculated[j], calculated[j-1]); j-- {
				calculated[j], calculated[j-1] = calculated[j-1], calculated[j]
			}
		}
		balanced.y[s] = (calculated[1] + calculated[2]) / 2
		balanced.innerShift[s] = 0
	}
	return balanced
}

func resizeInt32s(values []int32, n int) []int32 {
	if cap(values) < n {
		return make([]int32, n)
	}
	return values[:n]
}

func resizeFloats(values []float64, n int) []float64 {
	if cap(values) < n {
		return make([]float64, n)
	}
	return values[:n]
}

func resizeBools(values []bool, n int) []bool {
	if cap(values) < n {
		return make([]bool, n)
	}
	return values[:n]
}

func resizeOptFloats(values []optFloat, n int) []optFloat {
	if cap(values) < n {
		return make([]optFloat, n)
	}
	return values[:n]
}

func resizeNodeTypes(values []layered.NodeType, n int) []layered.NodeType {
	if cap(values) < n {
		return make([]layered.NodeType, n)
	}
	return values[:n]
}

func resizeNodeIDs(values []layered.LNodeID, n int) []layered.LNodeID {
	if cap(values) < n {
		return make([]layered.LNodeID, n)
	}
	return values[:n]
}

// maxFloat is math.Max: +Inf if either argument is +Inf (even with a NaN),
// otherwise the builtin max (NaN if either is NaN, +0 over -0), which the
// compiler inlines. Only a NaN result's payload can differ from math.Max's
// (math.Max returns math.NaN() on some platforms), and no NaN payload
// reaches any result: arithmetic and comparisons treat all NaNs alike.
func maxFloat(x, y float64) float64 {
	if math.IsInf(x, 1) || math.IsInf(y, 1) {
		return math.Inf(1)
	}
	return max(x, y)
}

// minFloat is math.Min, as maxFloat is math.Max (-Inf wins over NaN; -0
// over +0).
func minFloat(x, y float64) float64 {
	if math.IsInf(x, -1) || math.IsInf(y, -1) {
		return math.Inf(-1)
	}
	return min(x, y)
}
