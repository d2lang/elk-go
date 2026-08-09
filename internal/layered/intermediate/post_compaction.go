// Copyright (c) 2016 Kiel University and others.
// Copyright (c) 2017 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"fmt"
	"math"
	"sort"

	"github.com/d2lang/elk-go/internal/common/networksimplex"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

const (
	compactionFuzzyTolerance = 0.0001
	compactionSpacingEpsilon = 0.5
	compactionSmallEpsilon   = 0.01
)

// CompactHorizontally is the arena-backed port of ELK 0.8.1's
// HorizontalGraphCompactor and its one-dimensional constraint graph.
func CompactHorizontally(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	strategy := basegraph.Get(&arena.Graph(graphID).Properties, layered.PostCompactionStrategy)
	if strategy == layered.GraphCompactionNone {
		return nil
	}
	transformer := newLayeredCompactionTransformer(arena, graphID)
	graph, err := transformer.transform()
	if err != nil {
		return err
	}
	compactor := newOneDimensionalCompactor(graph, transformer)
	constraints := basegraph.Get(&arena.Graph(graphID).Properties, layered.PostCompactionConstraints)
	compactor.quadraticConstraints = constraints == layered.ConstraintCalculationQuadratic

	switch strategy {
	case layered.GraphCompactionLeft:
		compactor.compact(false)
	case layered.GraphCompactionRight:
		compactor.changeDirection(compactRight)
		compactor.compact(false)
	case layered.GraphCompactionLeftRightConstraintLocking:
		compactor.compact(false)
		compactor.changeDirection(compactRight)
		compactor.lock = func(node *compactNode, _ compactDirection) bool {
			return node.group.outDegreeReal == 0
		}
		compactor.compact(false)
	case layered.GraphCompactionLeftRightConnectionLocking:
		compactor.compact(false)
		compactor.changeDirection(compactRight)
		compactor.lock = func(node *compactNode, direction compactDirection) bool {
			locks := transformer.locks[node]
			return direction == compactLeft && locks.left || direction == compactRight && locks.right
		}
		compactor.compact(false)
	case layered.GraphCompactionEdgeLength:
		compactor.compact(true)
	default:
		return fmt.Errorf("layered: unsupported post-compaction strategy %d", strategy)
	}
	compactor.changeDirection(compactLeft)
	transformer.applyLayout()
	return nil
}

type compactDirection uint8

const (
	compactUndefined compactDirection = iota
	compactLeft
	compactRight
)

type compactLock struct{ left, right bool }

type compactGraph struct {
	nodes      []*compactNode
	groups     []*compactGroup
	predefined [][2]*compactNode
}

type compactNode struct {
	id          int
	node        *layered.LNodeID
	segment     *verticalSegment
	group       *compactGroup
	groupOffset geometry.Point
	hitbox      geometry.Rectangle
	before      geometry.Rectangle
	constraints []*compactNode
	start       float64
}

type compactGroup struct {
	id            int
	nodes         []*compactNode
	master        *compactNode
	reference     *compactNode
	start         float64
	outDegree     int
	outDegreeReal int
}

func (graph *compactGraph) addNode(node *compactNode) *compactNode {
	graph.nodes = append(graph.nodes, node)
	return node
}

func (graph *compactGraph) addGroup(nodes ...*compactNode) *compactGroup {
	group := &compactGroup{nodes: append([]*compactNode(nil), nodes...)}
	for _, node := range nodes {
		if node.group != nil {
			panic("layered: compaction node belongs to multiple groups")
		}
		node.group = group
	}
	if len(nodes) > 0 {
		group.reference = nodes[0]
	}
	graph.groups = append(graph.groups, group)
	return group
}

func (group *compactGroup) add(node *compactNode) {
	if node.group != nil {
		panic("layered: compaction node belongs to multiple groups")
	}
	group.nodes = append(group.nodes, node)
	node.group = group
	if group.reference == nil {
		group.reference = node
	}
}

type verticalSegment struct {
	potentialParents      []*compactNode
	edges                 []layered.LEdgeID
	sourcePorts           []layered.LPortID
	targetPorts           []layered.LPortID
	affectedBends         []*geometry.Point
	affectedControls      []*geometry.Point
	affectedBoundingBoxes []*geometry.Rectangle
	affectedJunctions     []*geometry.Point
	hitbox                geometry.Rectangle
	ignoreUp              bool
	ignoreDown            bool
	constraints           []*verticalSegment
	port                  *layered.LPortID
	joined                []*verticalSegment
}

func newVerticalSegment(
	arena *layered.LGraphArena,
	first, second *geometry.Point,
	parent *compactNode,
	edgeID layered.LEdgeID,
) *verticalSegment {
	segment := &verticalSegment{
		edges:         []layered.LEdgeID{edgeID},
		affectedBends: []*geometry.Point{first, second},
		hitbox: geometry.Rectangle{
			X:      math.Min(first.X, second.X),
			Y:      math.Min(first.Y, second.Y),
			Width:  math.Abs(first.X - second.X),
			Height: math.Abs(first.Y - second.Y),
		},
	}
	if parent != nil {
		segment.potentialParents = append(segment.potentialParents, parent)
	}
	if junctions, found := basegraph.TryGet(&arena.Edge(edgeID).Properties, layered.JunctionPoints); found {
		for index := range junctions {
			if fuzzyEqual(junctions[index].X, first.X) {
				segment.affectedJunctions = append(segment.affectedJunctions, &junctions[index])
			}
		}
	}
	return segment
}

func (segment *verticalSegment) intersects(other *verticalSegment) bool {
	return fuzzyEqual(segment.hitbox.X, other.hitbox.X) &&
		!(fuzzyLess(segment.hitbox.MaxY(), other.hitbox.Y) ||
			fuzzyLess(other.hitbox.MaxY(), segment.hitbox.Y))
}

func (segment *verticalSegment) join(other *verticalSegment) {
	segment.edges = append(segment.edges, other.edges...)
	segment.sourcePorts = append(segment.sourcePorts, other.sourcePorts...)
	segment.targetPorts = append(segment.targetPorts, other.targetPorts...)
	segment.affectedBends = append(segment.affectedBends, other.affectedBends...)
	segment.affectedControls = append(segment.affectedControls, other.affectedControls...)
	segment.affectedBoundingBoxes = append(
		segment.affectedBoundingBoxes, other.affectedBoundingBoxes...)
	segment.affectedJunctions = append(segment.affectedJunctions, other.affectedJunctions...)
	segment.constraints = append(segment.constraints, other.constraints...)
	segment.potentialParents = append(segment.potentialParents, other.potentialParents...)
	segment.hitbox = segment.hitbox.Union(other.hitbox)
	segment.ignoreUp = segment.ignoreUp || other.ignoreUp
	segment.ignoreDown = segment.ignoreDown || other.ignoreDown
	if segment.port == nil {
		segment.port = other.port
	}
	segment.joined = append(segment.joined, other.joined...)
	segment.joined = append(segment.joined, other)
}

type commentOffset struct {
	comment layered.LNodeID
	other   layered.LNodeID
	offset  geometry.Point
}

type layeredCompactionTransformer struct {
	arena   *layered.LGraphArena
	graphID layered.LGraphID
	graph   *compactGraph

	nodes        map[layered.LNodeID]*compactNode
	segmentNodes map[*verticalSegment]*compactNode
	locks        map[*compactNode]compactLock
	comments     []commentOffset
	edgeRouting  layered.EdgeRouting
}

func newLayeredCompactionTransformer(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) *layeredCompactionTransformer {
	return &layeredCompactionTransformer{
		arena:        arena,
		graphID:      graphID,
		graph:        &compactGraph{},
		nodes:        make(map[layered.LNodeID]*compactNode),
		segmentNodes: make(map[*verticalSegment]*compactNode),
		locks:        make(map[*compactNode]compactLock),
		edgeRouting:  basegraph.Get(&arena.Graph(graphID).Properties, layered.EdgeRoutingOption),
	}
}

func (transformer *layeredCompactionTransformer) transform() (*compactGraph, error) {
	transformer.transformNodes()
	var segments []*verticalSegment
	switch transformer.edgeRouting {
	case layered.EdgeRoutingOrthogonal:
		segments = transformer.collectOrthogonalSegments()
	case layered.EdgeRoutingSplines:
		segments = transformer.collectSplineSegments()
	default:
		return nil, fmt.Errorf("layered: post-compaction does not support edge routing %d", transformer.edgeRouting)
	}
	transformer.mergeSegments(segments)
	for segment, node := range transformer.segmentNodes {
		for _, constrained := range segment.constraints {
			if other := transformer.segmentNodes[constrained]; other != nil {
				transformer.graph.predefined = append(transformer.graph.predefined, [2]*compactNode{node, other})
			}
		}
	}
	return transformer.graph, nil
}

func (transformer *layeredCompactionTransformer) transformNodes() {
	graph := transformer.arena.Graph(transformer.graphID)
	for layerIndex, layerID := range graph.Layers {
		transformer.arena.Layer(layerID).ID = layerIndex
		for _, nodeID := range transformer.arena.Layer(layerID).Nodes {
			node := transformer.arena.Node(nodeID)
			// Comment nodes are represented by their owner's margin and are
			// restored by relative offset after compaction.
			if basegraph.Get(&node.Properties, core.CommentBox) {
				edges := transformer.arena.NodeConnectedEdges(nodeID)
				if len(edges) > 0 {
					other := transformer.arena.EdgeSourceNode(edges[0])
					if other == nodeID {
						other = transformer.arena.EdgeTargetNode(edges[0])
					}
					transformer.comments = append(transformer.comments, commentOffset{
						comment: nodeID,
						other:   other,
						offset:  node.Position.Sub(transformer.arena.Node(other).Position),
					})
					continue
				}
			}
			copyID := nodeID
			compact := transformer.graph.addNode(&compactNode{
				node: &copyID,
				hitbox: geometry.Rectangle{
					X:      node.Position.X - node.Margin.Left,
					Y:      node.Position.Y - node.Margin.Top,
					Width:  node.Size.X + node.Margin.Left + node.Margin.Right,
					Height: node.Size.Y + node.Margin.Top + node.Margin.Bottom,
				},
			})
			group := transformer.graph.addGroup(compact)
			group.master = compact
			locks := compactLock{}
			difference := len(transformer.arena.NodeIncomingEdges(nodeID)) - len(transformer.arena.NodeOutgoingEdges(nodeID))
			if difference < 0 {
				locks.left = true
			} else if difference > 0 {
				locks.right = true
			}
			if node.Type == layered.ExternalPortNode {
				locks = compactLock{}
			}
			transformer.nodes[nodeID] = compact
			transformer.locks[compact] = locks
		}
	}
}

func (transformer *layeredCompactionTransformer) collectOrthogonalSegments() []*verticalSegment {
	segments := make([]*verticalSegment, 0)
	for _, layerID := range transformer.arena.Graph(transformer.graphID).Layers {
		for _, nodeID := range transformer.arena.Layer(layerID).Nodes {
			parent := transformer.nodes[nodeID]
			for _, edgeID := range transformer.arena.NodeOutgoingEdges(nodeID) {
				edge := transformer.arena.Edge(edgeID)
				if len(edge.BendPoints) == 0 || parent == nil {
					continue
				}
				bend1 := &edge.BendPoints[0]
				firstVertical := true
				var lastSegment *verticalSegment
				sourcePort := transformer.arena.Port(*edge.Source)
				switch sourcePort.Side {
				case layered.NorthSide:
					boundary := &geometry.Point{X: bend1.X, Y: parent.hitbox.Y}
					segment := newVerticalSegment(transformer.arena, bend1, boundary, parent, edgeID)
					segment.ignoreDown = true
					port := *edge.Source
					segment.port = &port
					segments = append(segments, segment)
				case layered.SouthSide:
					boundary := &geometry.Point{X: bend1.X, Y: parent.hitbox.MaxY()}
					segment := newVerticalSegment(transformer.arena, bend1, boundary, parent, edgeID)
					segment.ignoreUp = true
					port := *edge.Source
					segment.port = &port
					segments = append(segments, segment)
				}

				for bendIndex := 1; bendIndex < len(edge.BendPoints); bendIndex++ {
					bend2 := &edge.BendPoints[bendIndex]
					if !fuzzyEqual(bend1.Y, bend2.Y) {
						lastSegment = newVerticalSegment(transformer.arena, bend1, bend2, nil, edgeID)
						segments = append(segments, lastSegment)
						if firstVertical {
							firstVertical = false
							if bend2.Y < parent.hitbox.Y {
								lastSegment.ignoreDown = true
							} else if bend2.Y > parent.hitbox.MaxY() {
								lastSegment.ignoreUp = true
							} else {
								lastSegment.ignoreUp, lastSegment.ignoreDown = true, true
							}
						}
					}
					// ELK only advances bend1 when another iterator element
					// remains; the endpoint used below is consequently the
					// penultimate bend.
					if bendIndex+1 < len(edge.BendPoints) {
						bend1 = bend2
					}
				}
				if lastSegment != nil {
					target := transformer.nodes[transformer.arena.EdgeTargetNode(edgeID)]
					if target != nil {
						if bend1.Y < target.hitbox.Y {
							lastSegment.ignoreDown = true
						} else if bend1.Y > target.hitbox.MaxY() {
							lastSegment.ignoreUp = true
						} else {
							lastSegment.ignoreUp, lastSegment.ignoreDown = true, true
						}
					}
				}
			}

			for _, edgeID := range transformer.arena.NodeIncomingEdges(nodeID) {
				edge := transformer.arena.Edge(edgeID)
				if len(edge.BendPoints) == 0 || parent == nil {
					continue
				}
				bend := &edge.BendPoints[len(edge.BendPoints)-1]
				targetPort := transformer.arena.Port(*edge.Target)
				switch targetPort.Side {
				case layered.NorthSide:
					boundary := &geometry.Point{X: bend.X, Y: parent.hitbox.Y}
					segment := newVerticalSegment(transformer.arena, bend, boundary, parent, edgeID)
					segment.ignoreDown = true
					port := *edge.Target
					segment.port = &port
					segments = append(segments, segment)
				case layered.SouthSide:
					boundary := &geometry.Point{X: bend.X, Y: parent.hitbox.MaxY()}
					segment := newVerticalSegment(transformer.arena, bend, boundary, parent, edgeID)
					segment.ignoreUp = true
					port := *edge.Target
					segment.port = &port
					segments = append(segments, segment)
				}
			}
		}
	}
	return segments
}

func (transformer *layeredCompactionTransformer) collectSplineSegments() []*verticalSegment {
	segments := make([]*verticalSegment, 0)
	for _, layerID := range transformer.arena.Graph(transformer.graphID).Layers {
		for _, nodeID := range transformer.arena.Layer(layerID).Nodes {
			for _, edgeID := range transformer.arena.NodeOutgoingEdges(nodeID) {
				if transformer.arena.EdgeIsSelfLoop(edgeID) {
					continue
				}
				edge := transformer.arena.Edge(edgeID)
				controlSegments, found := basegraph.TryGet(&edge.Properties, layered.SplineControlPointSegments)
				if !found {
					continue
				}
				compactionSegments, hasCompactionSegments := basegraph.TryGet(
					&edge.Properties, layered.SplineCompactionSegments)
				var previous *verticalSegment
				if hasCompactionSegments && len(compactionSegments) == len(controlSegments) {
					for segmentIndex := range compactionSegments {
						compactionSegment := &compactionSegments[segmentIndex]
						if compactionSegment.Straight {
							continue
						}
						controls := controlSegments[segmentIndex]
						refs := make([]*geometry.Point, 0, len(controls))
						for pointIndex := range controls {
							refs = append(refs, &controls[pointIndex])
						}
						segment := &verticalSegment{
							edges:                 []layered.LEdgeID{compactionSegment.Edge},
							sourcePorts:           []layered.LPortID{compactionSegment.SourcePort},
							targetPorts:           []layered.LPortID{compactionSegment.TargetPort},
							affectedControls:      refs,
							affectedBoundingBoxes: []*geometry.Rectangle{&compactionSegment.BoundingBox},
							hitbox:                compactionSegment.BoundingBox,
						}
						segments = append(segments, segment)
						if previous != nil {
							previous.constraints = append(previous.constraints, segment)
						}
						previous = segment
					}
					continue
				}

				// Compatibility for graphs constructed directly against the
				// arena without phase-5 SplineSegment metadata.
				for segmentIndex := range controlSegments {
					controls := controlSegments[segmentIndex]
					if len(controls) <= 1 {
						continue
					}
					left, top := math.Inf(1), math.Inf(1)
					right, bottom := math.Inf(-1), math.Inf(-1)
					refs := make([]*geometry.Point, 0, len(controls))
					for pointIndex := range controls {
						point := &controls[pointIndex]
						refs = append(refs, point)
						left, top = math.Min(left, point.X), math.Min(top, point.Y)
						right, bottom = math.Max(right, point.X), math.Max(bottom, point.Y)
					}
					segment := &verticalSegment{
						edges:            []layered.LEdgeID{edgeID},
						affectedControls: refs,
						hitbox: geometry.Rectangle{
							X: left, Y: top, Width: right - left, Height: bottom - top,
						},
					}
					segments = append(segments, segment)
					if previous != nil {
						previous.constraints = append(previous.constraints, segment)
					}
					previous = segment
				}
			}
		}
	}
	return segments
}

func (transformer *layeredCompactionTransformer) mergeSegments(segments []*verticalSegment) {
	if len(segments) == 0 {
		return
	}
	sort.SliceStable(segments, func(first, second int) bool {
		comparison := fuzzyCompare(segments[first].hitbox.X, segments[second].hitbox.X)
		if comparison == 0 {
			return segments[first].hitbox.Y < segments[second].hitbox.Y
		}
		return comparison < 0
	})
	survivor := segments[0]
	for _, next := range segments[1:] {
		if survivor.intersects(next) {
			survivor.join(next)
			continue
		}
		transformer.segmentToNode(survivor)
		survivor = next
	}
	transformer.segmentToNode(survivor)
}

func (transformer *layeredCompactionTransformer) segmentToNode(segment *verticalSegment) {
	node := transformer.graph.addNode(&compactNode{segment: segment, hitbox: segment.hitbox})
	if len(segment.potentialParents) > 0 {
		segment.potentialParents[0].group.add(node)
	}
	locks := compactLock{}
	sources := make(map[layered.LPortID]struct{})
	targets := make(map[layered.LPortID]struct{})
	for _, portID := range segment.sourcePorts {
		sources[portID] = struct{}{}
	}
	for _, portID := range segment.targetPorts {
		targets[portID] = struct{}{}
	}
	for _, edgeID := range segment.edges {
		edge := transformer.arena.Edge(edgeID)
		if edge.Source != nil {
			sources[*edge.Source] = struct{}{}
		}
		if edge.Target != nil {
			targets[*edge.Target] = struct{}{}
		}
	}
	if len(sources) < len(targets) {
		locks.left = true
	} else if len(sources) > len(targets) {
		locks.right = true
	}
	transformer.locks[node] = locks
	for _, joined := range segment.joined {
		transformer.segmentNodes[joined] = node
	}
	transformer.segmentNodes[segment] = node
}

func absoluteCompactionPortAnchor(arena *layered.LGraphArena, portID layered.LPortID) geometry.Point {
	port := arena.Port(portID)
	return arena.Node(*port.Node).Position.Add(port.Position).Add(port.Anchor)
}

type oneDimensionalCompactor struct {
	graph                *compactGraph
	transformer          *layeredCompactionTransformer
	direction            compactDirection
	quadraticConstraints bool
	lock                 func(*compactNode, compactDirection) bool
}

func newOneDimensionalCompactor(
	graph *compactGraph,
	transformer *layeredCompactionTransformer,
) *oneDimensionalCompactor {
	compactor := &oneDimensionalCompactor{graph: graph, transformer: transformer}
	compactor.calculateGroupOffsets()
	for _, node := range graph.nodes {
		if node.group == nil {
			graph.addGroup(node)
		}
		node.before = node.hitbox
	}
	return compactor
}

func (compactor *oneDimensionalCompactor) compact(networkSimplex bool) {
	if compactor.direction == compactUndefined {
		compactor.changeDirection(compactLeft)
	}
	for _, group := range compactor.graph.groups {
		group.outDegree = group.outDegreeReal
	}
	for _, node := range compactor.graph.nodes {
		node.start = math.Inf(-1)
	}
	if networkSimplex {
		compactor.networkSimplexCompact()
	} else {
		compactor.longestPathCompact()
	}
}

func (compactor *oneDimensionalCompactor) changeDirection(direction compactDirection) {
	if direction == compactor.direction {
		return
	}
	old := compactor.direction
	compactor.direction = direction
	switch {
	case old == compactUndefined && direction == compactLeft:
		compactor.calculateConstraints()
	case old == compactUndefined && direction == compactRight:
		compactor.mirrorHitboxes()
		compactor.calculateConstraints()
	case old == compactLeft && direction == compactRight,
		old == compactRight && direction == compactLeft:
		compactor.mirrorHitboxes()
		compactor.reverseConstraints()
	default:
		panic("layered: unsupported one-dimensional compaction direction transition")
	}
}

func (compactor *oneDimensionalCompactor) calculateGroupOffsets() {
	for _, group := range compactor.graph.groups {
		group.reference = nil
		for _, node := range group.nodes {
			node.groupOffset = geometry.Point{}
			if group.reference == nil || node.hitbox.X < group.reference.hitbox.X {
				group.reference = node
			}
		}
		if group.reference == nil {
			continue
		}
		for _, node := range group.nodes {
			node.groupOffset.X = node.hitbox.X - group.reference.hitbox.X
			node.groupOffset.Y = node.hitbox.Y - group.reference.hitbox.Y
		}
	}
}

func (compactor *oneDimensionalCompactor) mirrorHitboxes() {
	for _, node := range compactor.graph.nodes {
		node.hitbox.X = -node.hitbox.X - node.hitbox.Width
	}
	compactor.calculateGroupOffsets()
}

func (compactor *oneDimensionalCompactor) calculateConstraints() {
	for _, node := range compactor.graph.nodes {
		node.constraints = node.constraints[:0]
	}
	for _, constraint := range compactor.graph.predefined {
		if compactor.direction == compactLeft {
			constraint[0].constraints = append(constraint[0].constraints, constraint[1])
		} else {
			constraint[1].constraints = append(constraint[1].constraints, constraint[0])
		}
	}
	if compactor.quadraticConstraints {
		compactor.calculateQuadraticConstraints()
	} else {
		compactor.calculateEdgeAwareConstraints()
	}
	compactor.calculateConstraintsForGroups()
}

func (compactor *oneDimensionalCompactor) calculateConstraintsForGroups() {
	for _, group := range compactor.graph.groups {
		group.outDegree, group.outDegreeReal = 0, 0
	}
	for _, group := range compactor.graph.groups {
		for _, node := range group.nodes {
			for _, constrained := range node.constraints {
				if constrained.group != group {
					constrained.group.outDegree++
					constrained.group.outDegreeReal++
				}
			}
		}
	}
}

func (compactor *oneDimensionalCompactor) reverseConstraints() {
	incoming := make(map[*compactNode][]*compactNode, len(compactor.graph.nodes))
	for _, node := range compactor.graph.nodes {
		node.start = math.Inf(-1)
		for _, constrained := range node.constraints {
			incoming[constrained] = append(incoming[constrained], node)
		}
	}
	for _, node := range compactor.graph.nodes {
		node.constraints = incoming[node]
	}
	compactor.calculateConstraintsForGroups()
}

func (compactor *oneDimensionalCompactor) isLocked(node *compactNode) bool {
	return compactor.lock != nil && compactor.lock(node, compactor.direction)
}

func (compactor *oneDimensionalCompactor) groupIsLocked(group *compactGroup) bool {
	for _, node := range group.nodes {
		if compactor.isLocked(node) {
			return true
		}
	}
	return false
}

func (compactor *oneDimensionalCompactor) longestPathCompact() {
	minimum := math.Inf(1)
	for _, node := range compactor.graph.nodes {
		minimum = math.Min(minimum, node.group.reference.hitbox.X+node.groupOffset.X)
	}
	queue := make([]*compactGroup, 0)
	for _, group := range compactor.graph.groups {
		group.start = minimum
		if group.outDegree == 0 {
			queue = append(queue, group)
		}
	}
	for len(queue) > 0 {
		group := queue[0]
		queue = queue[1:]
		for _, node := range group.nodes {
			suggested := group.start + node.groupOffset.X
			if !compactor.groupIsLocked(group) || node.hitbox.X < suggested {
				node.start = suggested
			} else {
				node.start = node.hitbox.X
			}
		}
		for _, node := range group.nodes {
			for _, constrained := range node.constraints {
				spacing := compactor.horizontalSpacing(node, constrained)
				constrained.group.start = math.Max(constrained.group.start,
					node.start+node.hitbox.Width+spacing-constrained.groupOffset.X)
				if compactor.isLocked(constrained) {
					constrained.group.start = math.Max(constrained.group.start,
						constrained.hitbox.X-constrained.groupOffset.X)
				}
				constrained.group.outDegree--
				if constrained.group.outDegree == 0 {
					queue = append(queue, constrained.group)
				}
			}
		}
	}
	for _, node := range compactor.graph.nodes {
		node.hitbox.X = node.start
	}
}

func (compactor *oneDimensionalCompactor) horizontalSpacing(first, second *compactNode) float64 {
	if verticalSegmentsShareEdge(first, second) {
		return 0
	}
	firstType, firstExternal := compactor.compactionNodeType(first)
	secondType, secondExternal := compactor.compactionNodeType(second)
	if firstExternal || secondExternal {
		return 0
	}
	return layered.HorizontalSpacingByType(
		compactor.transformer.arena, compactor.transformer.graphID, firstType, secondType)
}

func (compactor *oneDimensionalCompactor) verticalSpacing(first, second *compactNode) float64 {
	if verticalSegmentsShareEdge(first, second) {
		return 1
	}
	firstType, _ := compactor.compactionNodeType(first)
	secondType, _ := compactor.compactionNodeType(second)
	return layered.VerticalSpacingByType(
		compactor.transformer.arena, compactor.transformer.graphID, firstType, secondType)
}

func (compactor *oneDimensionalCompactor) compactionNodeType(node *compactNode) (layered.NodeType, bool) {
	if node.node == nil {
		return layered.LongEdgeNode, false
	}
	typeValue := compactor.transformer.arena.Node(*node.node).Type
	return typeValue, typeValue == layered.ExternalPortNode
}

func verticalSegmentsShareEdge(first, second *compactNode) bool {
	if first.segment == nil || second.segment == nil {
		return false
	}
	edges := make(map[layered.LEdgeID]struct{}, len(first.segment.edges))
	for _, edgeID := range first.segment.edges {
		edges[edgeID] = struct{}{}
	}
	for _, edgeID := range second.segment.edges {
		if _, found := edges[edgeID]; found {
			return true
		}
	}
	return false
}

func (compactor *oneDimensionalCompactor) calculateQuadraticConstraints() {
	// ELK's quadratic implementation clears the predefined constraints that
	// the surrounding compactor installed immediately beforehand.
	for _, node := range compactor.graph.nodes {
		node.constraints = node.constraints[:0]
	}
	for _, first := range compactor.graph.nodes {
		for _, second := range compactor.graph.nodes {
			if first == second || first.group == second.group {
				continue
			}
			spacing := compactor.verticalSpacing(first, second)
			if (second.hitbox.X > first.hitbox.X ||
				(second.hitbox.X == first.hitbox.X && first.hitbox.Width < second.hitbox.Width)) &&
				fuzzyGreater(second.hitbox.MaxY()+spacing, first.hitbox.Y) &&
				fuzzyLess(second.hitbox.Y, first.hitbox.MaxY()+spacing) {
				first.constraints = append(first.constraints, second)
			}
		}
	}
}

func (compactor *oneDimensionalCompactor) calculateEdgeAwareConstraints() {
	switch compactor.transformer.edgeRouting {
	case layered.EdgeRoutingOrthogonal:
		compactor.calculateOrthogonalConstraints()
	case layered.EdgeRoutingSplines:
		compactor.calculateSplineConstraints()
	default:
		panic("layered: edge-aware constraints require orthogonal or spline routing")
	}
}

func (compactor *oneDimensionalCompactor) calculateSplineConstraints() {
	compactor.scanline(func(node *compactNode) bool { return node.segment != nil })
	minimum := compactor.minimumExpansionSpacing()
	for _, node := range compactor.graph.nodes {
		if node.node != nil {
			compactor.alterHitbox(node, minimum, 1)
		}
	}
	compactor.scanline(func(*compactNode) bool { return true })
	for _, node := range compactor.graph.nodes {
		if node.node != nil {
			compactor.alterHitbox(node, minimum, -1)
		}
	}
}

func (compactor *oneDimensionalCompactor) calculateOrthogonalConstraints() {
	edgeSpacing := basegraph.Get(
		&compactor.transformer.arena.Graph(compactor.transformer.graphID).Properties,
		layered.SpacingEdgeEdge,
	)
	segmentSpacing := math.Max(0, edgeSpacing/2-compactionSpacingEpsilon)
	for _, node := range compactor.graph.nodes {
		if node.segment != nil {
			compactor.alterHitbox(node, segmentSpacing, 1)
		}
	}
	compactor.scanline(func(node *compactNode) bool { return node.segment != nil })
	for _, node := range compactor.graph.nodes {
		if node.segment != nil {
			compactor.alterHitbox(node, segmentSpacing, -1)
		}
	}

	for _, node := range compactor.graph.nodes {
		if node.node == nil {
			continue
		}
		spacing := layered.IndividualOrDefault(
			compactor.transformer.arena, *node.node, &layered.SpacingEdgeEdge)
		compactor.alterHitbox(node, math.Max(0, spacing/2-compactionSpacingEpsilon), 1)
	}
	compactor.scanline(func(node *compactNode) bool { return node.node != nil })
	for _, node := range compactor.graph.nodes {
		if node.node == nil {
			continue
		}
		spacing := layered.IndividualOrDefault(
			compactor.transformer.arena, *node.node, &layered.SpacingEdgeEdge)
		compactor.alterHitbox(node, math.Max(0, spacing/2-compactionSpacingEpsilon), -1)
	}

	minimum := compactor.minimumExpansionSpacing()
	for _, group := range compactor.graph.groups {
		compactor.alterGroupedHitbox(group, minimum, 1)
	}
	compactor.scanline(func(*compactNode) bool { return true })
	for _, group := range compactor.graph.groups {
		compactor.alterGroupedHitbox(group, minimum, -1)
	}
}

func (compactor *oneDimensionalCompactor) minimumExpansionSpacing() float64 {
	edgeSpacing := basegraph.Get(
		&compactor.transformer.arena.Graph(compactor.transformer.graphID).Properties,
		layered.SpacingEdgeEdge,
	)
	minimum := math.Inf(1)
	for _, node := range compactor.graph.nodes {
		if node.node != nil && compactor.transformer.arena.Node(*node.node).Type == layered.ExternalPortNode {
			continue
		}
		if node.segment != nil {
			minimum = math.Min(minimum, math.Max(0, edgeSpacing/2-compactionSpacingEpsilon))
		} else if node.node != nil {
			spacing := layered.IndividualOrDefault(
				compactor.transformer.arena, *node.node, &layered.SpacingNodeNode)
			minimum = math.Min(minimum, math.Max(0, spacing/2-compactionSpacingEpsilon))
		}
	}
	if math.IsInf(minimum, 1) {
		return 0
	}
	return minimum
}

func (compactor *oneDimensionalCompactor) alterGroupedHitbox(
	group *compactGroup,
	spacing, factor float64,
) {
	master := group.master
	if master == nil && len(group.nodes) > 0 {
		master = group.nodes[0]
	}
	if master == nil {
		return
	}
	compactor.alterHitbox(master, spacing, factor)
	if len(group.nodes) == 1 {
		return
	}
	delta := spacing * factor
	for _, node := range group.nodes {
		if node == master || node.segment == nil {
			continue
		}
		if node.segment.ignoreUp {
			node.hitbox.Y += delta + compactionSmallEpsilon
			node.hitbox.Height -= delta + compactionSmallEpsilon
		} else if node.segment.ignoreDown {
			node.hitbox.Height -= delta + compactionSmallEpsilon
		}
	}
}

func (compactor *oneDimensionalCompactor) alterHitbox(
	node *compactNode,
	spacing, factor float64,
) {
	delta := spacing * factor
	if node.segment != nil {
		if !node.segment.ignoreUp {
			node.hitbox.Y -= delta + compactionSmallEpsilon
			node.hitbox.Height += delta + compactionSmallEpsilon
		} else if !node.segment.ignoreDown {
			node.hitbox.Height += delta + compactionSmallEpsilon
		}
	} else if node.node != nil {
		node.hitbox.Y -= delta
		node.hitbox.Height += 2 * delta
	}
}

type compactTimestamp struct {
	node  *compactNode
	low   bool
	order int
}

func (timestamp compactTimestamp) coordinate() float64 {
	if timestamp.low {
		return timestamp.node.hitbox.Y
	}
	return timestamp.node.hitbox.MaxY()
}

func (compactor *oneDimensionalCompactor) scanline(include func(*compactNode) bool) {
	events := make([]compactTimestamp, 0, len(compactor.graph.nodes)*2)
	order := 0
	for _, node := range compactor.graph.nodes {
		if include(node) {
			events = append(events,
				compactTimestamp{node: node, low: true, order: order},
				compactTimestamp{node: node, low: false, order: order + 1})
			order += 2
		}
	}
	sort.SliceStable(events, func(first, second int) bool {
		firstCoordinate, secondCoordinate := events[first].coordinate(), events[second].coordinate()
		if firstCoordinate != secondCoordinate {
			return firstCoordinate < secondCoordinate
		}
		if events[first].low != events[second].low {
			// High (deletion) timestamps precede low (insertion) timestamps.
			return !events[first].low
		}
		return events[first].order < events[second].order
	})

	active := make([]*compactNode, 0)
	candidate := make(map[*compactNode]*compactNode)
	center := func(node *compactNode) float64 { return node.hitbox.X + node.hitbox.Width/2 }
	indexOf := func(target *compactNode) int {
		for index, node := range active {
			if node == target {
				return index
			}
		}
		return -1
	}
	for _, event := range events {
		if event.low {
			position := sort.Search(len(active), func(index int) bool {
				return center(active[index]) >= center(event.node)
			})
			if position < len(active) && center(active[position]) == center(event.node) {
				panic(fmt.Sprintf("layered: invalid hitboxes for scanline constraint calculation: center %v, first %+v, second %+v",
					center(event.node), active[position].hitbox, event.node.hitbox))
			}
			active = append(active, nil)
			copy(active[position+1:], active[position:])
			active[position] = event.node
			if position > 0 {
				candidate[event.node] = active[position-1]
			}
			if position+1 < len(active) {
				candidate[active[position+1]] = event.node
			}
			continue
		}

		position := indexOf(event.node)
		if position < 0 {
			continue
		}
		if position > 0 {
			left := active[position-1]
			if candidate[event.node] == left && left.group != event.node.group {
				left.constraints = append(left.constraints, event.node)
			}
		}
		if position+1 < len(active) {
			right := active[position+1]
			if candidate[right] == event.node && right.group != event.node.group {
				event.node.constraints = append(event.node.constraints, right)
			}
		}
		copy(active[position:], active[position+1:])
		active[len(active)-1] = nil
		active = active[:len(active)-1]
	}
}

func (compactor *oneDimensionalCompactor) networkSimplexCompact() {
	network := networksimplex.NewGraph()
	groupNodes := make([]networksimplex.NodeID, len(compactor.graph.groups))
	for index, group := range compactor.graph.groups {
		group.id = index
		nodeID := network.AddNode()
		network.Node(nodeID).ID = index
		network.Node(nodeID).Origin = group
		groupNodes[index] = nodeID
	}

	for _, node := range compactor.graph.nodes {
		for _, constrained := range node.constraints {
			if node.group == constrained.group {
				continue
			}
			spacing := compactor.horizontalSpacing(node, constrained)
			delta := int(math.Ceil(node.groupOffset.X + node.hitbox.Width + spacing - constrained.groupOffset.X))
			delta = max(0, delta)
			if !verticalSegmentsShareEdge(node, constrained) {
				weight := 1.0
				if node.segment != nil && constrained.node != nil ||
					constrained.segment != nil && node.node != nil {
					weight = 2
				}
				network.AddEdge(groupNodes[node.group.id], groupNodes[constrained.group.id], weight, delta)
				continue
			}

			helper := network.AddNode()
			offsetDelta := int(math.Ceil(constrained.groupOffset.X - node.groupOffset.X))
			adjust := float64(offsetDelta) - (constrained.groupOffset.X - node.groupOffset.X)
			alter := node
			port := node.segment.port
			if port == nil {
				port = constrained.segment.port
				adjust = -adjust
				alter = constrained
			}
			if port != nil {
				alter.groupOffset.X -= adjust
				compactor.transformer.arena.Port(*port).Position.X -= adjust
			}
			network.AddEdge(helper, groupNodes[node.group.id], 1, max(0, offsetDelta))
			network.AddEdge(helper, groupNodes[constrained.group.id], 1, max(0, -offsetDelta))
		}
	}

	edgeSegments := make(map[layered.LEdgeID][]*compactNode)
	for _, node := range compactor.graph.nodes {
		if node.segment == nil {
			continue
		}
		seen := make(map[layered.LEdgeID]struct{})
		for _, edgeID := range node.segment.edges {
			if _, found := seen[edgeID]; found {
				continue
			}
			seen[edgeID] = struct{}{}
			edgeSegments[edgeID] = append(edgeSegments[edgeID], node)
		}
	}
	for _, node := range compactor.graph.nodes {
		if node.node == nil {
			continue
		}
		for _, edgeID := range compactor.transformer.arena.NodeOutgoingEdges(*node.node) {
			if compactor.transformer.arena.EdgeIsSelfLoop(edgeID) {
				continue
			}
			edge := compactor.transformer.arena.Edge(edgeID)
			sourcePort, targetPort := compactor.transformer.arena.Port(*edge.Source), compactor.transformer.arena.Port(*edge.Target)
			if isNorthSouth(sourcePort.Side) && isNorthSouth(targetPort.Side) {
				continue
			}
			target := compactor.transformer.nodes[compactor.transformer.arena.EdgeTargetNode(edgeID)]
			if target == nil {
				continue
			}
			network.AddEdge(groupNodes[node.group.id], groupNodes[target.group.id], 100, 0)

			if sourcePort.Side == layered.WestSide && len(sourcePort.OutgoingEdges) > 0 {
				for _, segment := range edgeSegments[edgeID] {
					if segment.hitbox.X < node.hitbox.X && segment.group != node.group {
						network.AddEdge(groupNodes[segment.group.id], groupNodes[node.group.id], 100, 1)
					}
				}
			}
			if targetPort.Side == layered.EastSide && len(targetPort.IncomingEdges) > 0 {
				for _, segment := range edgeSegments[edgeID] {
					if segment.hitbox.X > node.hitbox.X && segment.group != node.group {
						network.AddEdge(groupNodes[node.group.id], groupNodes[segment.group.id], 100, 1)
					}
				}
			}
		}
	}

	sources := make([]networksimplex.NodeID, 0)
	for _, nodeID := range network.Nodes {
		if len(network.Node(nodeID).IncomingEdges) == 0 {
			sources = append(sources, nodeID)
		}
	}
	if len(sources) > 1 {
		artificial := network.AddNode()
		for _, source := range sources {
			network.AddEdge(artificial, source, 0, 1)
		}
	}
	networksimplex.ForGraph(network).Execute()
	for _, node := range compactor.graph.nodes {
		node.hitbox.X = float64(network.Node(groupNodes[node.group.id]).Layer) + node.groupOffset.X
	}
}

func isNorthSouth(side layered.PortSide) bool {
	return side == layered.NorthSide || side == layered.SouthSide
}

func (transformer *layeredCompactionTransformer) applyLayout() {
	for _, node := range transformer.graph.nodes {
		if node.node != nil {
			original := transformer.arena.Node(*node.node)
			original.Position.X = node.hitbox.X + original.Margin.Left
		}
	}
	for _, comment := range transformer.comments {
		transformer.arena.Node(comment.comment).Position =
			transformer.arena.Node(comment.other).Position.Add(comment.offset)
	}
	for _, node := range transformer.graph.nodes {
		if node.segment == nil {
			continue
		}
		delta := node.hitbox.X - node.before.X
		for _, point := range node.segment.affectedBends {
			point.X += delta
		}
		for _, point := range node.segment.affectedControls {
			point.X += delta
		}
		for _, boundingBox := range node.segment.affectedBoundingBoxes {
			boundingBox.X += delta
		}
		for _, point := range node.segment.affectedJunctions {
			point.X += delta
		}
	}

	if transformer.edgeRouting == layered.EdgeRoutingSplines {
		for nodeID, compact := range transformer.nodes {
			delta := compact.hitbox.X - compact.before.X
			for _, edgeID := range transformer.arena.NodeOutgoingEdges(nodeID) {
				if transformer.arena.EdgeIsSelfLoop(edgeID) {
					for pointIndex := range transformer.arena.Edge(edgeID).BendPoints {
						transformer.arena.Edge(edgeID).BendPoints[pointIndex].X += delta
					}
				}
			}
		}
		transformer.adjustStraightSplineControls()
	}

	topLeft := geometry.Point{X: math.Inf(1), Y: math.Inf(1)}
	bottomRight := geometry.Point{X: math.Inf(-1), Y: math.Inf(-1)}
	for _, node := range transformer.graph.nodes {
		topLeft.X, topLeft.Y = math.Min(topLeft.X, node.hitbox.X), math.Min(topLeft.Y, node.hitbox.Y)
		bottomRight.X, bottomRight.Y = math.Max(bottomRight.X, node.hitbox.MaxX()), math.Max(bottomRight.Y, node.hitbox.MaxY())
	}
	if len(transformer.graph.nodes) == 0 {
		topLeft, bottomRight = geometry.Point{}, geometry.Point{}
	}
	graph := transformer.arena.Graph(transformer.graphID)
	graph.Offset = geometry.Point{X: -topLeft.X, Y: -topLeft.Y}
	graph.Size = bottomRight.Sub(topLeft)

	for _, node := range transformer.graph.nodes {
		if node.node == nil {
			continue
		}
		original := transformer.arena.Node(*node.node)
		if original.Type != layered.ExternalPortNode {
			continue
		}
		switch basegraph.Get(&original.Properties, layered.ExternalPortSide) {
		case layered.WestSide:
			original.Position.X = topLeft.X
		case layered.EastSide:
			original.Position.X = bottomRight.X - (original.Size.X + original.Margin.Right)
		case layered.NorthSide:
			original.Position.Y = topLeft.Y
		case layered.SouthSide:
			original.Position.Y = bottomRight.Y - (original.Size.Y + original.Margin.Bottom)
		}
	}
}

func (transformer *layeredCompactionTransformer) adjustStraightSplineControls() {
	for _, layerID := range transformer.arena.Graph(transformer.graphID).Layers {
		for _, nodeID := range transformer.arena.Layer(layerID).Nodes {
			for _, edgeID := range transformer.arena.NodeOutgoingEdges(nodeID) {
				edge := transformer.arena.Edge(edgeID)
				segments, found := basegraph.TryGet(&edge.Properties, layered.SplineControlPointSegments)
				if !found || len(segments) == 0 {
					continue
				}
				compactionSegments, hasCompactionSegments := basegraph.TryGet(
					&edge.Properties, layered.SplineCompactionSegments)
				if hasCompactionSegments && len(compactionSegments) == len(segments) {
					transformer.adjustStraightSplineSegmentBoxes(
						edgeID, segments, compactionSegments)
					continue
				}

				// Compatibility for directly assembled test arenas that only
				// provide the old control-point representation.
				for start := 0; start < len(segments); {
					if len(segments[start]) != 1 {
						start++
						continue
					}
					end := start
					for end+1 < len(segments) && len(segments[end+1]) == 1 {
						end++
					}
					left := absoluteCompactionPortAnchor(transformer.arena, *edge.Source).X
					if start > 0 && len(segments[start-1]) > 0 {
						left = segments[start-1][len(segments[start-1])-1].X
					}
					right := absoluteCompactionPortAnchor(transformer.arena, *edge.Target).X
					if end+1 < len(segments) && len(segments[end+1]) > 0 {
						right = segments[end+1][0].X
					}
					chunk := (right - left) / float64(max(2, end-start+2))
					for index := start; index <= end; index++ {
						segments[index][0].X = left + chunk*float64(index-start+1)
					}
					start = end + 1
				}
			}
		}
	}
}

// adjustStraightSplineSegmentBoxes is the source-shaped implementation of
// LGraphToCGraphTransformer.adjustSplineControlPoints(). Non-straight segment
// boxes were moved as compaction nodes; the straight boxes between them are
// redistributed equidistantly afterwards.
func (transformer *layeredCompactionTransformer) adjustStraightSplineSegmentBoxes(
	edgeID layered.LEdgeID,
	controls []geometry.Chain,
	segments []layered.SplineCompactionSegment,
) {
	if len(segments) == 0 {
		return
	}
	if len(segments) == 1 {
		transformer.adjustSplineControlsBetween(
			edgeID, controls, segments, 0, 0, 1, 0)
		return
	}

	lastSegment := 0
	for index := 1; index < len(segments); {
		if lastSegment == 0 || !segments[lastSegment].Straight {
			next := index
			for next < len(segments)-1 && segments[next].Straight {
				next++
			}
			transformer.adjustSplineControlsBetween(
				edgeID, controls, segments, lastSegment, next, index, next)
			index = next + 1
			lastSegment = next
		}
	}
}

func (transformer *layeredCompactionTransformer) adjustSplineControlsBetween(
	edgeID layered.LEdgeID,
	controls []geometry.Chain,
	segments []layered.SplineCompactionSegment,
	leftSegment, rightSegment, leftIndex, rightIndex int,
) {
	startX := segments[leftSegment].BoundingBox.MaxX()
	firstIndex := leftIndex
	if leftSegment == 0 && segments[leftSegment].Straight {
		source := transformer.nodes[transformer.arena.EdgeSourceNode(edgeID)]
		if source != nil {
			startX = source.hitbox.MaxX()
		}
		firstIndex--
	}

	endX := segments[rightSegment].BoundingBox.X
	lastIndex := rightIndex
	if rightSegment == len(segments)-1 && segments[rightSegment].Straight {
		target := transformer.nodes[transformer.arena.EdgeTargetNode(edgeID)]
		if target != nil {
			endX = target.hitbox.X
		}
		lastIndex++
	}

	chunk := (endX - startX) / float64(max(2, lastIndex-firstIndex))
	newPosition := startX + chunk
	for index := firstIndex; index < lastIndex; index++ {
		oldCenter := segments[index].BoundingBox.X +
			segments[index].BoundingBox.Width/2
		segments[index].BoundingBox.X = newPosition -
			segments[index].BoundingBox.Width/2
		delta := newPosition - oldCenter
		for pointIndex := range controls[index] {
			controls[index][pointIndex].X += delta
		}
		newPosition += chunk
	}
}

func fuzzyCompare(first, second float64) int {
	if math.Abs(first-second) <= compactionFuzzyTolerance || first == second {
		return 0
	}
	if first < second {
		return -1
	}
	return 1
}

func fuzzyEqual(first, second float64) bool   { return fuzzyCompare(first, second) == 0 }
func fuzzyLess(first, second float64) bool    { return fuzzyCompare(first, second) < 0 }
func fuzzyGreater(first, second float64) bool { return fuzzyCompare(first, second) > 0 }
