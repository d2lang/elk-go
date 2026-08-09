// Copyright (c) 2016 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package components

import (
	"fmt"
	"math"
	"sort"

	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/layered"
)

const (
	connectedComponentsMaxIterations = 10
	connectedComponentsEpsilon       = 0.0001
)

var (
	positiveInfinity = math.Inf(1)
	negativeInfinity = math.Inf(-1)
)

// componentCompactDirection is ELK's Direction restricted to the values used
// by OneDimensionalComponentsCompaction.
type componentCompactDirection uint8

const (
	componentDirectionUndefined componentCompactDirection = iota
	componentDirectionLeft
	componentDirectionRight
	componentDirectionUp
	componentDirectionDown
)

func (direction componentCompactDirection) horizontal() bool {
	return direction == componentDirectionLeft || direction == componentDirectionRight
}

type componentCompactionGraph struct {
	nodes  []*componentCompactionNode
	groups []*componentCompactionGroup
}

type componentCompactionNode struct {
	id          int
	hitbox      geometry.Rectangle
	group       *componentCompactionGroup
	groupOffset geometry.Point
	constraints []*componentCompactionNode
	spacing     *float64
	start       float64
	reposition  bool
	lock        [5]bool
}

type componentCompactionGroup struct {
	nodes           []*componentCompactionNode
	reference       *componentCompactionNode
	start           float64
	delta           float64
	deltaNormalized float64
	outDegree       int
	reposition      bool
}

func (graph *componentCompactionGraph) addGroup(
	nodes ...*componentCompactionNode,
) *componentCompactionGroup {
	group := &componentCompactionGroup{
		nodes:      append([]*componentCompactionNode(nil), nodes...),
		reposition: true,
	}
	for _, node := range nodes {
		if node.group != nil {
			panic("components: compaction node belongs to another group")
		}
		node.group = group
		if group.reference == nil {
			group.reference = node
		}
	}
	graph.groups = append(graph.groups, group)
	return group
}

type compactedComponent struct {
	graphID     layered.LGraphID
	oldPosition geometry.Point
	offsetNode  *componentCompactionNode
}

// compactConnectedComponents is the no-external-extension path through ELK
// 0.8.1's ComponentsCompactor, ComponentsToCGraphTransformer, and
// OneDimensionalComponentsCompaction. Component placement with external ports
// is selected earlier by componentsProcessor and never reaches SimpleRow.
func compactConnectedComponents(
	arena *layered.LGraphArena,
	componentIDs []layered.LGraphID,
	spacing float64,
) (geometry.Point, error) {
	constraintGraph := &componentCompactionGraph{}
	components := make([]compactedComponent, 0, len(componentIDs))

	for _, graphID := range componentIDs {
		points := componentHullPoints(arena, graphID)
		rectangles := splitRectilinearHull(points)
		if len(rectangles) == 0 {
			return geometry.Point{}, fmt.Errorf(
				"components: component %d has no rectilinear hull", graphID)
		}

		groupNodes := make([]*componentCompactionNode, 0, len(rectangles))
		for _, rectangle := range rectangles {
			node := &componentCompactionNode{
				hitbox:     rectangle,
				reposition: true,
				// Components without external extensions are locked in all
				// directions by ComponentsToCGraphTransformer.setLock().
				lock: [5]bool{false, true, true, true, true},
			}
			constraintGraph.nodes = append(constraintGraph.nodes, node)
			groupNodes = append(groupNodes, node)
		}
		constraintGraph.addGroup(groupNodes...)
		components = append(components, compactedComponent{
			graphID:     graphID,
			oldPosition: rectangles[0].Position(),
			offsetNode:  groupNodes[0],
		})
	}

	compactor := newComponentOneDimensionalCompactor(constraintGraph, spacing)
	compactor.compactComponents()
	compactor.changeDirection(componentDirectionLeft)

	topLeft := geometry.Point{X: positiveInfinity, Y: positiveInfinity}
	bottomRight := geometry.Point{X: negativeInfinity, Y: negativeInfinity}
	for _, node := range constraintGraph.nodes {
		topLeft.X = math.Min(topLeft.X, node.hitbox.X)
		topLeft.Y = math.Min(topLeft.Y, node.hitbox.Y)
		bottomRight.X = math.Max(bottomRight.X, node.hitbox.MaxX())
		bottomRight.Y = math.Max(bottomRight.Y, node.hitbox.MaxY())
	}
	globalOffset := geometry.Point{X: -topLeft.X, Y: -topLeft.Y}
	for _, component := range components {
		newPosition := component.offsetNode.hitbox.Position()
		componentOffset := newPosition.Sub(component.oldPosition).Add(globalOffset)
		offsetGraph(arena, component.graphID, componentOffset)
	}

	return bottomRight.Sub(topLeft), nil
}

// componentHullPoints is ComponentsCompactor.componentHullPoints(). Node
// margins already include labels, ports, and their labels at this phase.
func componentHullPoints(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) []hullPoint {
	points := make([]hullPoint, 0)
	for _, nodeID := range arena.Graph(graphID).LayerlessNodes {
		node := arena.Node(nodeID)
		if node.Type == layered.ExternalPortNode {
			continue
		}
		left := node.Position.X - node.Margin.Left
		top := node.Position.Y - node.Margin.Top
		right := node.Position.X + node.Size.X + node.Margin.Right
		bottom := node.Position.Y + node.Size.Y + node.Margin.Bottom
		points = append(points,
			newHullPoint(left, top),
			newHullPoint(left, bottom),
			newHullPoint(right, top),
			newHullPoint(right, bottom),
		)

		for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
			if arena.Node(arena.EdgeSourceNode(edgeID)).Type == layered.ExternalPortNode ||
				arena.Node(arena.EdgeTargetNode(edgeID)).Type == layered.ExternalPortNode {
				continue
			}
			for _, bendPoint := range arena.Edge(edgeID).BendPoints {
				points = append(points, newHullPoint(bendPoint.X, bendPoint.Y))
			}
		}
	}
	return points
}

type componentOneDimensionalCompactor struct {
	graph           *componentCompactionGraph
	direction       componentCompactDirection
	spacing         float64
	lockingStrategy func(*componentCompactionNode, componentCompactDirection) bool
}

func newComponentOneDimensionalCompactor(
	graph *componentCompactionGraph,
	spacing float64,
) *componentOneDimensionalCompactor {
	compactor := &componentOneDimensionalCompactor{
		graph:   graph,
		spacing: spacing,
	}
	compactor.lockingStrategy = func(
		node *componentCompactionNode,
		_ componentCompactDirection,
	) bool {
		return node.group.outDegree != 0
	}
	compactor.calculateGroupOffsets()
	return compactor
}

func (compactor *componentOneDimensionalCompactor) compactComponents() {
	// OneDimensionalComponentsCompaction repeats alternating horizontal and
	// vertical passes until the normalized movement converges.
	run := 0
	for {
		delta := compactor.compactRun()
		run++
		if !((run < 2 || delta > connectedComponentsEpsilon) &&
			run < connectedComponentsMaxIterations) {
			break
		}
	}

	// The final pass aligns the drawing's top-left. With no external
	// extensions every directional lock is true.
	compactor.lockingStrategy = func(
		node *componentCompactionNode,
		direction componentCompactDirection,
	) bool {
		return node.lock[direction] ||
			(node.group.outDegree != 0 && node.lock[direction])
	}
	compactor.compactRun()
}

// compactRun is OneDimensionalComponentsCompaction.compact(int), with the
// extension and placeholder collections empty in SimpleRow's supported path.
func (compactor *componentOneDimensionalCompactor) compactRun() float64 {
	for _, group := range compactor.graph.groups {
		group.delta = 0
		group.deltaNormalized = 0
	}

	compactor.calculateGroupOffsets()
	compactor.forceConstraintsRecalculation()
	compactor.changeDirection(componentDirectionLeft)
	compactor.compact()
	compactor.changeDirection(componentDirectionRight)
	compactor.applyLockingStrategy()
	compactor.compact()
	compactor.changeDirection(componentDirectionLeft)
	compactor.applyLockingStrategy()
	compactor.compact()
	compactor.changeDirection(componentDirectionLeft)

	compactor.calculateGroupOffsets()
	delta := 0.0
	for _, group := range compactor.graph.groups {
		delta += math.Abs(group.deltaNormalized)
		group.delta = 0
		group.deltaNormalized = 0
	}

	compactor.changeDirection(componentDirectionUp)
	compactor.forceConstraintsRecalculation()
	compactor.compact()
	compactor.changeDirection(componentDirectionDown)
	compactor.applyLockingStrategy()
	compactor.compact()
	compactor.changeDirection(componentDirectionUp)
	compactor.applyLockingStrategy()
	compactor.compact()
	compactor.changeDirection(componentDirectionLeft)
	compactor.forceConstraintsRecalculation()

	for _, group := range compactor.graph.groups {
		delta += math.Abs(group.deltaNormalized)
	}
	return delta
}

func (compactor *componentOneDimensionalCompactor) compact() {
	if compactor.direction == componentDirectionUndefined {
		compactor.changeDirection(componentDirectionLeft)
	}
	for _, group := range compactor.graph.groups {
		group.outDegree = 0
	}
	for _, node := range compactor.graph.nodes {
		node.start = negativeInfinity
		for _, constrained := range node.constraints {
			constrained.group.outDegree++
		}
	}
	compactor.longestPathCompact()
	for _, node := range compactor.graph.nodes {
		node.reposition = true
	}
}

func (compactor *componentOneDimensionalCompactor) changeDirection(
	direction componentCompactDirection,
) {
	if direction == compactor.direction {
		return
	}
	oldDirection := compactor.direction
	compactor.direction = direction

	switch oldDirection {
	case componentDirectionUndefined:
		switch direction {
		case componentDirectionLeft:
			compactor.calculateConstraints()
		case componentDirectionRight:
			compactor.mirrorHitboxes()
			compactor.calculateConstraints()
		case componentDirectionUp:
			compactor.transposeHitboxes()
			compactor.calculateConstraints()
		case componentDirectionDown:
			compactor.transposeHitboxes()
			compactor.mirrorHitboxes()
			compactor.calculateConstraints()
		}
	case componentDirectionLeft:
		switch direction {
		case componentDirectionRight:
			compactor.mirrorHitboxes()
			compactor.reverseConstraints()
		case componentDirectionUp:
			compactor.transposeHitboxes()
			compactor.calculateConstraints()
		case componentDirectionDown:
			compactor.transposeHitboxes()
			compactor.mirrorHitboxes()
			compactor.calculateConstraints()
		}
	case componentDirectionRight:
		switch direction {
		case componentDirectionLeft:
			compactor.mirrorHitboxes()
			compactor.reverseConstraints()
		case componentDirectionUp:
			compactor.mirrorHitboxes()
			compactor.transposeHitboxes()
			compactor.calculateConstraints()
		case componentDirectionDown:
			compactor.mirrorHitboxes()
			compactor.transposeHitboxes()
			compactor.mirrorHitboxes()
			compactor.calculateConstraints()
		}
	case componentDirectionUp:
		switch direction {
		case componentDirectionLeft:
			compactor.transposeHitboxes()
			compactor.calculateConstraints()
		case componentDirectionRight:
			compactor.transposeHitboxes()
			compactor.mirrorHitboxes()
			compactor.calculateConstraints()
		case componentDirectionDown:
			compactor.mirrorHitboxes()
			compactor.reverseConstraints()
		}
	case componentDirectionDown:
		switch direction {
		case componentDirectionLeft:
			compactor.mirrorHitboxes()
			compactor.transposeHitboxes()
			compactor.calculateConstraints()
		case componentDirectionRight:
			compactor.mirrorHitboxes()
			compactor.transposeHitboxes()
			compactor.mirrorHitboxes()
			compactor.calculateConstraints()
		case componentDirectionUp:
			compactor.mirrorHitboxes()
			compactor.reverseConstraints()
		}
	}
}

func (compactor *componentOneDimensionalCompactor) applyLockingStrategy() {
	for _, group := range compactor.graph.groups {
		group.reposition = true
	}
	for _, node := range compactor.graph.nodes {
		node.reposition = compactor.lockingStrategy(node, compactor.direction)
		node.group.reposition = node.group.reposition &&
			compactor.lockingStrategy(node, compactor.direction)
	}
}

func (compactor *componentOneDimensionalCompactor) calculateGroupOffsets() {
	for _, group := range compactor.graph.groups {
		group.reference = nil
		for _, node := range group.nodes {
			node.groupOffset = geometry.Point{}
			if group.reference == nil || node.hitbox.X < group.reference.hitbox.X {
				group.reference = node
			}
		}
		for _, node := range group.nodes {
			node.groupOffset.X = node.hitbox.X - group.reference.hitbox.X
			node.groupOffset.Y = node.hitbox.Y - group.reference.hitbox.Y
		}
	}
}

func (compactor *componentOneDimensionalCompactor) mirrorHitboxes() {
	for _, node := range compactor.graph.nodes {
		node.hitbox.X = -node.hitbox.X - node.hitbox.Width
	}
	compactor.calculateGroupOffsets()
}

func (compactor *componentOneDimensionalCompactor) transposeHitboxes() {
	for _, node := range compactor.graph.nodes {
		node.hitbox.X, node.hitbox.Y = node.hitbox.Y, node.hitbox.X
		node.hitbox.Width, node.hitbox.Height = node.hitbox.Height, node.hitbox.Width
		node.groupOffset.X, node.groupOffset.Y =
			node.groupOffset.Y, node.groupOffset.X
	}
	compactor.calculateGroupOffsets()
}

func (compactor *componentOneDimensionalCompactor) forceConstraintsRecalculation() {
	compactor.calculateConstraints()
}

func (compactor *componentOneDimensionalCompactor) calculateConstraints() {
	for _, node := range compactor.graph.nodes {
		node.constraints = node.constraints[:0]
	}
	compactor.calculateScanlineConstraints()
	compactor.calculateConstraintsForGroups()
}

func (compactor *componentOneDimensionalCompactor) calculateConstraintsForGroups() {
	for _, group := range compactor.graph.groups {
		group.outDegree = 0
	}
	for _, group := range compactor.graph.groups {
		for _, node := range group.nodes {
			for _, constrained := range node.constraints {
				if constrained.group != group {
					constrained.group.outDegree++
				}
			}
		}
	}
}

func (compactor *componentOneDimensionalCompactor) reverseConstraints() {
	incoming := make(map[*componentCompactionNode][]*componentCompactionNode,
		len(compactor.graph.nodes))
	for _, node := range compactor.graph.nodes {
		node.start = negativeInfinity
		for _, constrained := range node.constraints {
			incoming[constrained] = append(incoming[constrained], node)
		}
	}
	for _, node := range compactor.graph.nodes {
		node.constraints = incoming[node]
	}
	compactor.calculateConstraintsForGroups()
}

func (compactor *componentOneDimensionalCompactor) longestPathCompact() {
	minimumStart := positiveInfinity
	for _, node := range compactor.graph.nodes {
		minimumStart = math.Min(minimumStart,
			node.group.reference.hitbox.X+node.groupOffset.X)
	}

	queue := make([]*componentCompactionGroup, 0, len(compactor.graph.groups))
	for _, group := range compactor.graph.groups {
		group.start = minimumStart
		if group.outDegree == 0 {
			queue = append(queue, group)
		}
	}

	for len(queue) > 0 {
		group := queue[0]
		queue = queue[1:]
		difference := group.reference.hitbox.X

		for _, node := range group.nodes {
			suggested := group.start + node.groupOffset.X
			if group.reposition || node.hitbox.X < suggested {
				node.start = suggested
			} else {
				node.start = node.hitbox.X
			}
		}
		difference -= group.reference.start
		group.delta += difference
		if compactor.direction == componentDirectionRight ||
			compactor.direction == componentDirectionDown {
			group.deltaNormalized += difference
		} else {
			group.deltaNormalized -= difference
		}

		for _, node := range group.nodes {
			for _, constrained := range node.constraints {
				spacing := compactor.spacing
				if node.spacing != nil && *node.spacing < spacing {
					spacing = *node.spacing
				}
				if constrained.spacing != nil && *constrained.spacing < spacing {
					spacing = *constrained.spacing
				}
				constrained.group.start = math.Max(constrained.group.start,
					node.start+node.hitbox.Width+spacing-
						constrained.groupOffset.X)
				if !constrained.reposition {
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

type componentCompactionTimestamp struct {
	node *componentCompactionNode
	low  bool
}

func (timestamp componentCompactionTimestamp) coordinate() float64 {
	if timestamp.low {
		return timestamp.node.hitbox.Y
	}
	return timestamp.node.hitbox.Y + timestamp.node.hitbox.Height
}

type componentIntervalSet []*componentCompactionNode

func componentIntervalCenter(node *componentCompactionNode) float64 {
	return node.hitbox.X + node.hitbox.Width/2
}

func (intervals componentIntervalSet) firstAtOrAfter(center float64) int {
	return sort.Search(len(intervals), func(index int) bool {
		return componentIntervalCenter(intervals[index]) >= center
	})
}

func (intervals componentIntervalSet) lower(node *componentCompactionNode) *componentCompactionNode {
	index := intervals.firstAtOrAfter(componentIntervalCenter(node))
	if index == 0 {
		return nil
	}
	return intervals[index-1]
}

func (intervals componentIntervalSet) higher(node *componentCompactionNode) *componentCompactionNode {
	center := componentIntervalCenter(node)
	index := sort.Search(len(intervals), func(index int) bool {
		return componentIntervalCenter(intervals[index]) > center
	})
	if index == len(intervals) {
		return nil
	}
	return intervals[index]
}

func (intervals *componentIntervalSet) add(node *componentCompactionNode) {
	center := componentIntervalCenter(node)
	index := intervals.firstAtOrAfter(center)
	// TreeSet's center-only comparator treats equal centers as the same key.
	if index < len(*intervals) && componentIntervalCenter((*intervals)[index]) == center {
		return
	}
	*intervals = append(*intervals, nil)
	copy((*intervals)[index+1:], (*intervals)[index:])
	(*intervals)[index] = node
}

func (intervals *componentIntervalSet) remove(node *componentCompactionNode) {
	center := componentIntervalCenter(node)
	index := intervals.firstAtOrAfter(center)
	if index == len(*intervals) || componentIntervalCenter((*intervals)[index]) != center {
		return
	}
	copy((*intervals)[index:], (*intervals)[index+1:])
	*intervals = (*intervals)[:len(*intervals)-1]
}

// calculateScanlineConstraints is ELK 0.8.1's
// ScanlineConstraintCalculator with spacingFun(n) == 0.
func (compactor *componentOneDimensionalCompactor) calculateScanlineConstraints() {
	timestamps := make([]componentCompactionTimestamp, 0, len(compactor.graph.nodes)*2)
	for index, node := range compactor.graph.nodes {
		node.id = index
		timestamps = append(timestamps,
			componentCompactionTimestamp{node: node, low: true},
			componentCompactionTimestamp{node: node, low: false},
		)
	}
	sort.SliceStable(timestamps, func(first, second int) bool {
		firstCoordinate := timestamps[first].coordinate()
		secondCoordinate := timestamps[second].coordinate()
		if firstCoordinate != secondCoordinate {
			return firstCoordinate < secondCoordinate
		}
		// At equal positions the high (deletion) timestamp precedes low.
		return !timestamps[first].low && timestamps[second].low
	})

	intervals := componentIntervalSet{}
	candidates := make([]*componentCompactionNode, len(compactor.graph.nodes))
	for _, timestamp := range timestamps {
		node := timestamp.node
		if timestamp.low {
			intervals.add(node)
			candidates[node.id] = intervals.lower(node)
			if right := intervals.higher(node); right != nil {
				candidates[right.id] = node
			}
			continue
		}

		left := intervals.lower(node)
		if left != nil && left == candidates[node.id] && left.group != node.group {
			left.constraints = append(left.constraints, node)
		}
		right := intervals.higher(node)
		if right != nil && candidates[right.id] == node && right.group != node.group {
			node.constraints = append(node.constraints, right)
		}
		intervals.remove(node)
	}
}
