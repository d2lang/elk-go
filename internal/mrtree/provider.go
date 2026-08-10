// Copyright (c) 2013, 2015 Kiel University and others.
// Copyright (c) 2013, 2020 Kiel University and others.
// Copyright (c) 2015, 2020 Kiel University and others.
// Copyright (c) 2017 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package mrtree

import (
	"fmt"
	"math"
	"sort"

	"github.com/d2lang/elk-go/internal/common/nodespacing"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

type Provider struct{}

// Register adds the provider generated from Tree.melk and its exact feature
// declaration to the algorithm registry.
func Register(registry *core.AlgorithmRegistry) error {
	return registry.Register(core.AlgorithmData{
		ID:       AlgorithmID,
		Features: core.FeatureDisconnected,
		Create:   func() core.LayoutProvider { return &Provider{} },
		KnownOptions: []string{
			"org.eclipse.elk.padding",
			"org.eclipse.elk.spacing.nodeNode",
			"org.eclipse.elk.spacing.edgeNode",
			"org.eclipse.elk.aspectRatio",
			"org.eclipse.elk.priority",
			"org.eclipse.elk.separateConnectedComponents",
			"org.eclipse.elk.debugMode",
			"org.eclipse.elk.direction",
			"org.eclipse.elk.interactive",
			"org.eclipse.elk.interactiveLayout",
			"org.eclipse.elk.nodeSize.constraints",
			"org.eclipse.elk.nodeSize.fixedGraphSize",
			"org.eclipse.elk.nodeSize.minimum",
			"org.eclipse.elk.nodeSize.options",
			"org.eclipse.elk.nodeLabels.placement",
			"org.eclipse.elk.omitNodeMicroLayout",
			"org.eclipse.elk.portLabels.placement",
			Weighting.ID,
			SearchOrder.ID,
			core.TopdownLayout.ID,
			core.TopdownScaleFactor.ID,
			core.TopdownHierarchicalNodeWidth.ID,
			core.TopdownHierarchicalNodeAspectRatio.ID,
			core.TopdownNodeType.ID,
			PositionConstraint.ID,
			EdgeRoutingMode.ID,
			TreeLevel.ID,
			Compaction.ID,
			EdgeEndTextureLength.ID,
		},
	})
}

func (*Provider) Layout(model *graph.Model, parent graph.NodeID, monitor core.ProgressMonitor) error {
	if !graph.Get(&model.Node(parent).Properties, OmitNodeMicroLayout) {
		nodespacing.CalculateLabelAndNodeSizes(model, parent, nil)
		nodespacing.NewNodeMarginCalculator().Process(model, parent)
	}
	g := importGraph(model, parent)
	components := splitComponents(g)
	for _, component := range components {
		layoutComponent(component, monitor.Subtask(1/float64(max(1, len(components)))))
	}
	packed := packComponents(components)
	return applyLayout(model, parent, packed)
}

func importGraph(model *graph.Model, parent graph.NodeID) *tGraph {
	parentNode := model.Node(parent)
	g := &tGraph{
		padding:              graph.Get(&parentNode.Properties, Padding),
		spacing:              graph.Get(&parentNode.Properties, SpacingNodeNode),
		spacingEdgeNode:      graph.Get(&parentNode.Properties, SpacingEdgeNode),
		edgeEndTextureLength: graph.Get(&parentNode.Properties, EdgeEndTextureLength),
		aspectRatio:          graph.Get(&parentNode.Properties, AspectRatio),
		separate:             graph.Get(&parentNode.Properties, SeparateConnectedComponents),
		searchOrder:          graph.Get(&parentNode.Properties, SearchOrder),
		weighting:            graph.Get(&parentNode.Properties, Weighting),
		direction:            graph.Get(&parentNode.Properties, Direction),
		edgeRoutingMode:      graph.Get(&parentNode.Properties, EdgeRoutingMode),
		compaction:           graph.Get(&parentNode.Properties, Compaction),
		fixedGraphSize:       graph.Get(&parentNode.Properties, NodeSizeFixedGraphSize),
	}
	if g.direction == "UNDEFINED" {
		g.direction = "DOWN"
	}
	byOrigin := make(map[graph.NodeID]*tNode, len(parentNode.Children))
	for index, nodeID := range parentNode.Children {
		origin := nodeID
		node := model.Node(nodeID)
		tn := &tNode{
			id:       index,
			origin:   &origin,
			position: geometry.Point{X: node.Shape.X + node.Shape.Width/2, Y: node.Shape.Y + node.Shape.Height/2},
			size:     geometry.Point{X: math.Max(node.Shape.Width, 1), Y: math.Max(node.Shape.Height, 1)},
			props:    node.Properties.Clone(),
		}
		g.nodes = append(g.nodes, tn)
		byOrigin[nodeID] = tn
	}

	// ElkGraphUtil.allOutgoingEdges visits each child in model order. Scanning
	// the containing graph's insertion-ordered edge list gives the same order
	// for node- and port-connected edges without depending on Go maps.
	for _, sourceID := range parentNode.Children {
		for _, edgeID := range parentNode.ContainedEdges {
			edge := model.Edge(edgeID)
			if len(edge.Sources) == 0 || len(edge.Targets) == 0 {
				continue
			}
			actualSource := model.ShapeNode(edge.Sources[0])
			actualTarget := model.ShapeNode(edge.Targets[0])
			if actualSource != sourceID || actualSource == actualTarget {
				continue
			}
			source := byOrigin[actualSource]
			target := byOrigin[actualTarget]
			if source == nil || target == nil {
				continue
			}
			origin := edgeID
			te := &tEdge{origin: &origin, source: source, target: target}
			source.outgoing = append(source.outgoing, te)
			target.incoming = append(target.incoming, te)
			g.edges = append(g.edges, te)
		}
	}
	return g
}

func splitComponents(g *tGraph) []*tGraph {
	if !g.separate {
		return []*tGraph{g}
	}
	incidence := make([][]*tEdge, len(g.nodes))
	for _, edge := range g.edges {
		incidence[edge.source.id] = append(incidence[edge.source.id], edge)
		incidence[edge.target.id] = append(incidence[edge.target.id], edge)
	}
	visited := make([]bool, len(g.nodes))
	components := make([]*tGraph, 0)
	var visit func(*tNode, *tGraph)
	visit = func(node *tNode, component *tGraph) {
		if visited[node.id] {
			return
		}
		visited[node.id] = true
		component.nodes = append(component.nodes, node)
		for _, edge := range incidence[node.id] {
			if edge.source != node {
				visit(edge.source, component)
			}
			if edge.target != node {
				visit(edge.target, component)
			}
			if !containsEdge(component.edges, edge) {
				component.edges = append(component.edges, edge)
			}
		}
	}
	for _, node := range g.nodes {
		if visited[node.id] {
			continue
		}
		component := g.settingsClone()
		visit(node, component)
		components = append(components, component)
	}
	if len(components) > 1 {
		for _, component := range components {
			for id, node := range component.nodes {
				node.id = id
			}
		}
	}
	return components
}

func (g *tGraph) settingsClone() *tGraph {
	return &tGraph{
		padding:              g.padding,
		spacing:              g.spacing,
		spacingEdgeNode:      g.spacingEdgeNode,
		edgeEndTextureLength: g.edgeEndTextureLength,
		aspectRatio:          g.aspectRatio,
		separate:             g.separate,
		searchOrder:          g.searchOrder,
		weighting:            g.weighting,
		direction:            g.direction,
		edgeRoutingMode:      g.edgeRoutingMode,
		compaction:           g.compaction,
		fixedGraphSize:       g.fixedGraphSize,
	}
}

func containsEdge(edges []*tEdge, edge *tEdge) bool {
	for _, candidate := range edges {
		if candidate == edge {
			return true
		}
	}
	return false
}

func layoutComponent(g *tGraph, monitor core.ProgressMonitor) {
	monitor.Begin("Tree layout", 1)
	treeify(g)
	root := ensureRoot(g)
	computeFan(root)
	computeTreeLevels(root)
	orderNodes(root, g.weighting)
	computeLevelHeights(root, g.direction)
	computeNeighbors(root)
	placeNodes(g, root)
	applyDirectionAndNodePositions(g)
	computeLevelCoordinates(g)
	compactGraph(g)
	computeGraphBounds(g)
	routeEdges(g)
	monitor.Done()
}

func treeify(g *tGraph) {
	for id, node := range g.nodes {
		node.id = id
	}
	visited := make([]int, len(g.nodes))
	eliminated := make([]*tEdge, 0)
	var dfs func(*tNode)
	dfs = func(node *tNode) {
		visited[node.id] = 1
		for _, edge := range append([]*tEdge(nil), node.outgoing...) {
			target := edge.target
			switch visited[target.id] {
			case 1:
				eliminated = append(eliminated, edge)
			case 2:
				visited[target.id] = 1
			default:
				dfs(target)
			}
		}
	}
	var bfs func(*tNode)
	bfs = func(start *tNode) {
		queue := []*tNode{start}
		for len(queue) > 0 {
			node := queue[0]
			queue = queue[1:]
			visited[node.id] = 1
			for _, edge := range append([]*tEdge(nil), node.outgoing...) {
				target := edge.target
				switch visited[target.id] {
				case 1:
					eliminated = append(eliminated, edge)
				case 2:
					visited[target.id] = 1
				default:
					queue = append(queue, target)
				}
			}
		}
	}
	for _, node := range g.nodes {
		if visited[node.id] != 0 {
			continue
		}
		if g.searchOrder == "BFS" {
			bfs(node)
		} else {
			dfs(node)
		}
		visited[node.id] = 2
	}
	for _, edge := range eliminated {
		edge.source.outgoing = removeEdge(edge.source.outgoing, edge)
		edge.target.incoming = removeEdge(edge.target.incoming, edge)
	}
}

func removeEdge(edges []*tEdge, remove *tEdge) []*tEdge {
	for i, edge := range edges {
		if edge == remove {
			return append(edges[:i], edges[i+1:]...)
		}
	}
	return edges
}

func ensureRoot(g *tGraph) *tNode {
	roots := make([]*tNode, 0)
	for _, node := range g.nodes {
		node.root = false
		if len(node.incoming) == 0 {
			node.root = true
			roots = append(roots, node)
		}
	}
	switch len(roots) {
	case 0:
		root := &tNode{id: 0, root: true, dummy: true}
		g.nodes = append(g.nodes, root)
		return root
	case 1:
		return roots[0]
	default:
		root := &tNode{id: 0, root: true, dummy: true}
		for _, child := range roots {
			child.root = false
			g.addDummyChild(root, child)
		}
		g.nodes = append(g.nodes, root)
		return root
	}
}

// computeFan is the tree-specialized form of FanProcessor's string-prefix
// accounting: fan is the maximum number of descendants on any one deeper
// level and descendants includes the node itself.
func computeFan(node *tNode) []int {
	levels := []int{1}
	for _, child := range node.children() {
		childLevels := computeFan(child)
		for depth, count := range childLevels {
			for len(levels) <= depth+1 {
				levels = append(levels, 0)
			}
			levels[depth+1] += count
		}
	}
	node.descendants = 0
	for _, count := range levels {
		node.descendants += count
	}
	node.fan = 0
	for _, count := range levels[1:] {
		if count > node.fan {
			node.fan = count
		}
	}
	return levels
}

func computeTreeLevels(root *tNode) {
	level := []*tNode{root}
	for depth := 0; len(level) > 0; depth++ {
		next := make([]*tNode, 0)
		for _, node := range level {
			node.treeLevel = depth
			next = append(next, node.children()...)
		}
		level = next
	}
}

func orderNodes(root *tNode, weighting string) {
	if weighting == "MODEL_ORDER" {
		return
	}
	if weighting == "CONSTRAINT" {
		orderNodesByConstraint([]*tNode{root})
		return
	}
	useDescendants := weighting == "DESCENDANTS"
	var orderLevel func([]*tNode)
	orderLevel = func(current []*tNode) {
		if len(current) == 0 {
			return
		}
		weight := func(node *tNode) int {
			if useDescendants {
				return node.descendants
			}
			return node.fan
		}
		sort.SliceStable(current, func(i, j int) bool { return weight(current[i]) < weight(current[j]) })
		firstLeaf := len(current)
		for firstLeaf > 0 && weight(current[firstLeaf-1]) == 0 {
			firstLeaf--
		}
		inners := append([]*tNode(nil), current[:firstLeaf]...)
		leaves := append([]*tNode(nil), current[firstLeaf:]...)
		position := 0
		if len(inners) == 0 {
			for _, leaf := range leaves {
				leaf.positionIndex = position
				position++
			}
			return
		}
		for _, parent := range inners {
			parent.positionIndex = position
			position++
			children := parent.children()
			orderLevel(children)
			sort.SliceStable(children, func(i, j int) bool { return children[i].positionIndex > children[j].positionIndex })
			reordered := make([]*tEdge, 0, len(parent.outgoing))
			for _, child := range children {
				for _, edge := range parent.outgoing {
					if edge.target == child {
						reordered = append(reordered, edge)
					}
				}
			}
			parent.outgoing = reordered

			fillGap := len(parent.outgoing)
			for index := len(leaves) - 1; index >= 0 && fillGap > 0; index-- {
				if weight(leaves[index]) != 0 {
					break
				}
				leaves[index].positionIndex = position
				position++
				fillGap--
				leaves = append(leaves[:index], leaves[index+1:]...)
			}
		}
	}
	orderLevel([]*tNode{root})
}

func orderNodesByConstraint(current []*tNode) {
	if len(current) == 0 {
		return
	}
	undefined := make([]*tNode, 0, len(current))
	inBounds := make([]*tNode, 0, len(current))
	outOfBounds := make([]*tNode, 0, len(current))
	for _, node := range current {
		position := graph.Get(&node.props, PositionConstraint)
		switch {
		case position < 0:
			undefined = append(undefined, node)
		case position < len(current):
			inBounds = append(inBounds, node)
		default:
			outOfBounds = append(outOfBounds, node)
		}
	}

	sortedNodes := make([]*tNode, len(current))
	for index := 0; index < len(inBounds); {
		node := inBounds[index]
		target := graph.Get(&node.props, PositionConstraint)
		// This intentionally uses the shrinking inBounds size. It mirrors the
		// 0.12 Java implementation, including that implementation detail.
		if target >= 0 && target < len(inBounds) && sortedNodes[target] == nil {
			sortedNodes[target] = node
			inBounds = append(inBounds[:index], inBounds[index+1:]...)
			continue
		}
		index++
	}
	for len(inBounds) > 0 {
		node := inBounds[0]
		inBounds = inBounds[1:]
		target := graph.Get(&node.props, PositionConstraint)
		for distance := 0; ; distance++ {
			right := target + distance
			if right >= 0 && right < len(sortedNodes) && sortedNodes[right] == nil {
				sortedNodes[right] = node
				break
			}
			left := target - distance
			if left >= 0 && left < len(sortedNodes) && sortedNodes[left] == nil {
				sortedNodes[left] = node
				break
			}
		}
	}
	sort.SliceStable(outOfBounds, func(i, j int) bool {
		return graph.Get(&outOfBounds[i].props, PositionConstraint) > graph.Get(&outOfBounds[j].props, PositionConstraint)
	})
	for index := len(sortedNodes) - 1; index >= 0 && len(outOfBounds) > 0; index-- {
		if sortedNodes[index] == nil {
			sortedNodes[index] = outOfBounds[0]
			outOfBounds = outOfBounds[1:]
		}
	}
	for index := range sortedNodes {
		if sortedNodes[index] == nil && len(undefined) > 0 {
			sortedNodes[index] = undefined[0]
			undefined = undefined[1:]
		}
		sortedNodes[index].positionIndex = index
	}

	for _, parent := range current {
		if parent.fan == 0 {
			continue
		}
		children := parent.children()
		orderNodesByConstraint(children)
		sort.SliceStable(children, func(i, j int) bool {
			return children[i].positionIndex < children[j].positionIndex
		})
		reordered := make([]*tEdge, 0, len(parent.outgoing))
		for _, child := range children {
			for _, edge := range parent.outgoing {
				if edge.target == child {
					reordered = append(reordered, edge)
				}
			}
		}
		parent.outgoing = reordered
	}
}

func computeLevelHeights(root *tNode, direction string) {
	level := []*tNode{root}
	for len(level) > 0 {
		height := 0.0
		next := make([]*tNode, 0)
		for _, node := range level {
			size := node.size.Y
			if direction == "LEFT" || direction == "RIGHT" {
				size = node.size.X
			}
			if size > height {
				height = size
			}
			next = append(next, node.children()...)
		}
		for _, node := range level {
			node.levelHeight = height
		}
		level = next
	}
}

func computeNeighbors(root *tNode) {
	for _, node := range descendantsIncluding(root) {
		node.leftNeighbor, node.rightNeighbor = nil, nil
		node.leftSibling, node.rightSibling = nil, nil
	}
	level := root.children()
	for len(level) > 0 {
		var left *tNode
		next := make([]*tNode, 0)
		for _, node := range level {
			next = append(next, node.children()...)
			if left != nil {
				left.rightNeighbor = node
				node.leftNeighbor = left
				if left.parent() == node.parent() {
					left.rightSibling = node
					node.leftSibling = left
				}
			}
			left = node
		}
		level = next
	}
}

func descendantsIncluding(root *tNode) []*tNode {
	result := []*tNode{root}
	for index := 0; index < len(result); index++ {
		result = append(result, result[index].children()...)
	}
	return result
}

func placeNodes(g *tGraph, root *tNode) {
	spacing := g.spacing
	var firstWalk func(*tNode, int)
	var leftMost func([]*tNode, int) *tNode
	leftMost = func(level []*tNode, depth int) *tNode {
		if len(level) == 0 {
			return nil
		}
		if depth > 1 {
			next := make([]*tNode, 0)
			for _, node := range level {
				next = append(next, node.children()...)
			}
			return leftMost(next, depth-1)
		}
		if depth < 0 {
			next := make([]*tNode, 0)
			for _, node := range level {
				next = append(next, node.children()...)
			}
			if len(next) > 0 {
				return leftMost(next, depth)
			}
		}
		return level[0]
	}
	meanWidth := func(left, right *tNode) float64 {
		width := 0.0
		if left != nil {
			if g.direction == "LEFT" || g.direction == "RIGHT" {
				width += left.size.Y / 2
			} else {
				width += left.size.X / 2
			}
		}
		if right != nil {
			if g.direction == "LEFT" || g.direction == "RIGHT" {
				width += right.size.Y / 2
			} else {
				width += right.size.X / 2
			}
		}
		return width
	}
	var apportion func(*tNode, int)
	apportion = func(node *tNode, _ int) {
		children := node.children()
		var leftmost *tNode
		if len(children) > 0 {
			leftmost = children[0]
		}
		var neighbor *tNode
		if leftmost != nil {
			neighbor = leftmost.leftNeighbor
		}
		compareDepth := 1
		for leftmost != nil && neighbor != nil {
			leftModSum, rightModSum := 0.0, 0.0
			ancestorLeftmost, ancestorNeighbor := leftmost, neighbor
			for i := 0; i < compareDepth; i++ {
				ancestorLeftmost = ancestorLeftmost.parent()
				ancestorNeighbor = ancestorNeighbor.parent()
				if ancestorLeftmost == nil || ancestorNeighbor == nil {
					return
				}
				rightModSum += ancestorLeftmost.modifier
				leftModSum += ancestorNeighbor.modifier
			}
			moveDistance := neighbor.prelim + leftModSum + spacing + meanWidth(leftmost, neighbor) - leftmost.prelim - rightModSum
			if moveDistance > 0 {
				leftSibling := node
				leftSiblings := 0
				for leftSibling != nil && leftSibling != ancestorNeighbor {
					leftSiblings++
					leftSibling = leftSibling.leftSibling
				}
				if leftSibling == nil {
					return
				}
				portion := moveDistance / float64(leftSiblings)
				for leftSibling = node; leftSibling != ancestorNeighbor; leftSibling = leftSibling.leftSibling {
					leftSibling.prelim += moveDistance
					leftSibling.modifier += moveDistance
					moveDistance -= portion
				}
			}
			compareDepth++
			if leftmost.leaf() {
				leftmost = leftMost(node.children(), compareDepth)
			} else {
				leftmost = leftmost.children()[0]
			}
			if leftmost != nil {
				neighbor = leftmost.leftNeighbor
			} else {
				neighbor = nil
			}
		}
	}
	firstWalk = func(node *tNode, level int) {
		node.modifier = 0
		leftSibling := node.leftSibling
		if node.leaf() {
			if leftSibling != nil {
				node.prelim = leftSibling.prelim + spacing + meanWidth(leftSibling, node)
			} else {
				node.prelim = 0
			}
			return
		}
		children := node.children()
		for _, child := range children {
			firstWalk(child, level+1)
		}
		midPoint := (children[len(children)-1].prelim + children[0].prelim) / 2
		if leftSibling != nil {
			node.prelim = leftSibling.prelim + spacing + meanWidth(leftSibling, node)
			node.modifier = node.prelim - midPoint
			apportion(node, level)
		} else {
			node.prelim = midPoint
		}
	}
	firstWalk(root, 0)
	var secondWalk func(*tNode, float64, float64)
	secondWalk = func(node *tNode, yCoordinate, modifierSum float64) {
		if node == nil {
			return
		}
		x := node.prelim + modifierSum
		y := yCoordinate + node.levelHeight/2
		node.xCoordinate = javaRound(x)
		node.yCoordinate = javaRound(y)
		if !node.leaf() {
			secondWalk(node.children()[0], yCoordinate+node.levelHeight+spacing, modifierSum+node.modifier)
		}
		if node.rightSibling != nil {
			secondWalk(node.rightSibling, yCoordinate, modifierSum)
		}
	}
	secondWalk(root, -root.levelHeight/2, 0)
	for _, node := range g.nodes {
		node.position = geometry.Point{X: float64(node.xCoordinate), Y: float64(node.yCoordinate)}
	}
}

func javaRound(value float64) int { return int(math.Floor(value + .5)) }

func applyDirectionAndNodePositions(g *tGraph) {
	for _, node := range g.nodes {
		x, y := node.xCoordinate, node.yCoordinate
		switch g.direction {
		case "UP":
			y = -y
		case "RIGHT":
			x, y = y, x
		case "LEFT":
			x, y = -y, x
		}
		node.xCoordinate, node.yCoordinate = x, y
		node.position = geometry.Point{
			X: float64(x) - node.size.X/2,
			Y: float64(y) - node.size.Y/2,
		}
	}
}

func computeGraphBounds(g *tGraph) {
	if len(g.nodes) == 0 {
		g.graphMin, g.graphMax = geometry.Point{}, geometry.Point{}
		return
	}
	g.graphMin = geometry.Point{X: math.Inf(1), Y: math.Inf(1)}
	g.graphMax = geometry.Point{X: math.Inf(-1), Y: math.Inf(-1)}
	for _, node := range g.nodes {
		g.graphMin.X = math.Min(g.graphMin.X, node.position.X)
		g.graphMin.Y = math.Min(g.graphMin.Y, node.position.Y)
		g.graphMax.X = math.Max(g.graphMax.X, node.position.X+node.size.X)
		g.graphMax.Y = math.Max(g.graphMax.Y, node.position.Y+node.size.Y)
	}
}

func computeLevelCoordinates(g *tGraph) {
	type interval struct{ min, max float64 }
	levels := make(map[int]interval)
	for _, node := range g.nodes {
		current, found := levels[node.treeLevel]
		if !found {
			current = interval{min: math.Inf(1), max: math.Inf(-1)}
		}
		if g.direction == "LEFT" || g.direction == "RIGHT" {
			current.min = math.Min(current.min, node.position.X)
			current.max = math.Max(current.max, node.position.X+node.size.X)
		} else {
			current.min = math.Min(current.min, node.position.Y)
			current.max = math.Max(current.max, node.position.Y+node.size.Y)
		}
		levels[node.treeLevel] = current
	}
	for _, node := range g.nodes {
		current := levels[node.treeLevel]
		node.levelMin, node.levelMax = current.min, current.max
	}
}

func compactGraph(g *tGraph) {
	if !g.compaction || len(g.nodes) == 0 {
		return
	}
	type interval struct{ min, max float64 }
	levels := make([]interval, 0)
	for _, node := range g.nodes {
		for node.treeLevel >= len(levels) {
			levels = append(levels, interval{min: math.Inf(1), max: math.Inf(-1)})
		}
		current := levels[node.treeLevel]
		if g.direction == "LEFT" || g.direction == "RIGHT" {
			current.min = math.Min(current.min, node.position.X)
			current.max = math.Max(current.max, node.position.X+node.size.X)
		} else {
			current.min = math.Min(current.min, node.position.Y)
			current.max = math.Max(current.max, node.position.Y+node.size.Y)
		}
		levels[node.treeLevel] = current
	}

	computeCompactionConstraints(g, g.spacing/4)
	nodes := append([]*tNode(nil), g.nodes...)
	sort.SliceStable(nodes, func(i, j int) bool {
		return directionProjection(g.direction, nodes[i].position) < directionProjection(g.direction, nodes[j].position)
	})
	for _, node := range nodes {
		if node.root {
			continue
		}
		dependent := lowestDependentNode(node, g.direction)
		parent := lowestParent(node, g)
		newPosition, newPositionSize := 0.0, 0.0
		if dependent != nil {
			switch g.direction {
			case "LEFT":
				newPosition = dependent.position.X - g.spacing - node.size.X
				if parent.position.X-g.spacing-node.size.X < newPosition {
					newPosition = parent.position.X - g.spacing - node.size.X
				}
				newPositionSize = newPosition + node.size.X
			case "RIGHT":
				newPosition = dependent.position.X + dependent.size.X + g.spacing
				if parent.position.X-g.spacing-node.size.X < newPosition {
					newPosition = parent.position.X + parent.size.X + g.spacing
				}
				newPositionSize = newPosition + node.size.X
			case "UP":
				newPosition = dependent.position.Y - g.spacing - node.size.Y
				if parent.position.Y-g.spacing-node.size.Y < newPosition {
					newPosition = parent.position.Y - g.spacing - node.size.Y
				}
				newPositionSize = newPosition + node.size.Y
			default:
				newPosition = dependent.position.Y + dependent.size.Y + g.spacing
				if parent.position.Y+g.spacing > newPosition {
					newPosition = parent.position.Y + parent.size.Y + g.spacing
				}
				newPositionSize = newPosition + node.size.Y
			}
		} else if parent != nil {
			switch g.direction {
			case "LEFT":
				newPosition = parent.position.X - g.spacing - node.size.X
				newPositionSize = newPosition + node.size.X
			case "RIGHT":
				newPosition = parent.position.X + parent.size.X + g.spacing
				newPositionSize = newPosition + node.size.X
			case "UP":
				newPosition = parent.position.Y - g.spacing - node.size.Y
				newPositionSize = newPosition + node.size.Y
			default:
				newPosition = parent.position.Y + parent.size.Y + g.spacing
				newPositionSize = newPosition + node.size.Y
			}
		}

		if g.edgeRoutingMode != "AVOID_OVERLAP" {
			setDirectionalPosition(node, g.direction, newPosition)
			continue
		}
		levelIndex := -1
		for index, level := range levels {
			if level.min <= newPosition && level.max >= newPositionSize {
				levelIndex = index
				break
			}
		}
		if levelIndex < 0 {
			for index := 1; index < len(levels); index++ {
				if g.direction == "LEFT" || g.direction == "UP" {
					if levels[index].min <= newPosition {
						levelIndex = index
						break
					}
				} else if levels[index].min >= newPosition {
					levelIndex = index
					break
				}
			}
		}
		if levelIndex >= 0 {
			setDirectionalPosition(node, g.direction, levels[levelIndex].min)
			if levelIndex > 0 && levelIndex != node.treeLevel {
				node.compactLevelAscension = true
				node.treeLevel = levelIndex
			}
		}
	}
}

func computeCompactionConstraints(g *tGraph, padding float64) {
	for _, node := range g.nodes {
		node.compactConstraints = nil
	}
	type scanPoint struct {
		node  *tNode
		point geometry.Point
		start bool
	}
	points := make([]scanPoint, 0, len(g.nodes)*2)
	for _, node := range g.nodes {
		if node.dummy {
			continue
		}
		points = append(points, scanPoint{node: node, point: geometry.Point{X: node.position.X - padding, Y: node.position.Y - padding}, start: true})
	}
	for _, node := range g.nodes {
		if node.dummy {
			continue
		}
		points = append(points, scanPoint{node: node, point: geometry.Point{X: node.position.X + node.size.X + padding, Y: node.position.Y + node.size.Y + padding}})
	}
	sort.SliceStable(points, func(i, j int) bool {
		return perpendicularProjection(g.direction, points[i].point) < perpendicularProjection(g.direction, points[j].point)
	})
	active := make([]*tNode, 0)
	candidate := make(map[*tNode]*tNode)
	for _, point := range points {
		node := point.node
		if point.start {
			active = compactionSetInsert(active, node, g.direction)
			if left := compactionSetLeft(active, node, g.direction); left != nil {
				candidate[node] = left
			}
			if right := compactionSetRight(active, node, g.direction); right != nil {
				candidate[right] = node
			}
			continue
		}
		if left := compactionSetLeft(active, node, g.direction); left != nil && left == candidate[node] {
			node.compactConstraints = append(node.compactConstraints, left)
		}
		if right := compactionSetRight(active, node, g.direction); right != nil && candidate[right] == node {
			right.compactConstraints = append(right.compactConstraints, node)
		}
		active = compactionSetRemove(active, node, g.direction)
	}
}

func directionProjection(direction string, point geometry.Point) float64 {
	switch direction {
	case "LEFT":
		return -point.X
	case "RIGHT":
		return point.X
	case "UP":
		return -point.Y
	default:
		return point.Y
	}
}

func perpendicularProjection(direction string, point geometry.Point) float64 {
	if direction == "LEFT" || direction == "RIGHT" {
		return point.Y
	}
	return point.X
}

func compactionSetInsert(set []*tNode, node *tNode, direction string) []*tNode {
	value := directionProjection(direction, node.position)
	index := sort.Search(len(set), func(i int) bool {
		return directionProjection(direction, set[i].position) >= value
	})
	if index < len(set) && directionProjection(direction, set[index].position) == value {
		return set
	}
	set = append(set, nil)
	copy(set[index+1:], set[index:])
	set[index] = node
	return set
}

func compactionSetLeft(set []*tNode, node *tNode, direction string) *tNode {
	value := directionProjection(direction, node.position)
	index := sort.Search(len(set), func(i int) bool {
		return directionProjection(direction, set[i].position) >= value
	})
	if index == 0 {
		return nil
	}
	return set[index-1]
}

func compactionSetRight(set []*tNode, node *tNode, direction string) *tNode {
	value := directionProjection(direction, node.position)
	index := sort.Search(len(set), func(i int) bool {
		return directionProjection(direction, set[i].position) >= value
	})
	if index+1 >= len(set) {
		return nil
	}
	return set[index+1]
}

func compactionSetRemove(set []*tNode, node *tNode, direction string) []*tNode {
	value := directionProjection(direction, node.position)
	index := sort.Search(len(set), func(i int) bool {
		return directionProjection(direction, set[i].position) >= value
	})
	if index < len(set) && directionProjection(direction, set[index].position) == value {
		return append(set[:index], set[index+1:]...)
	}
	return set
}

func lowestDependentNode(node *tNode, direction string) *tNode {
	if len(node.compactConstraints) == 0 {
		return nil
	}
	result := node.compactConstraints[0]
	for _, candidate := range node.compactConstraints[1:] {
		switch direction {
		case "LEFT":
			if candidate.position.X < result.position.X {
				result = candidate
			}
		case "RIGHT":
			if candidate.position.X+candidate.size.X > result.position.X+result.size.X {
				result = candidate
			}
		case "UP":
			if candidate.position.Y < result.position.Y {
				result = candidate
			}
		default:
			if candidate.position.Y+candidate.size.Y > result.position.Y+result.size.Y {
				result = candidate
			}
		}
	}
	return result
}

func lowestParent(node *tNode, g *tGraph) *tNode {
	parents := make(map[*tNode]bool)
	for _, edge := range g.edges {
		if edge.target == node {
			parents[edge.source] = true
		}
	}
	var result *tNode
	resultProjection := math.Inf(-1)
	for _, candidate := range g.nodes {
		if !parents[candidate] {
			continue
		}
		center := geometry.Point{X: candidate.position.X + candidate.size.X/2, Y: candidate.position.Y + candidate.size.Y/2}
		projection := directionProjection(g.direction, center)
		if result == nil || projection > resultProjection {
			result, resultProjection = candidate, projection
		}
	}
	return result
}

func setDirectionalPosition(node *tNode, direction string, value float64) {
	if direction == "LEFT" || direction == "RIGHT" {
		node.position.X = value
	} else {
		node.position.Y = value
	}
}

func routeEdges(g *tGraph) {
	for _, edge := range g.edges {
		edge.bendPoints = nil
	}
	switch g.edgeRoutingMode {
	case "NONE":
		return
	case "MIDDLE_TO_MIDDLE":
		for _, edge := range g.edges {
			middleToMiddleEdgeRoute(edge)
		}
	default:
		avoidOverlapStartPoints(g)
		avoidOverlapSpecialEdges(g)
		avoidOverlapEndPoints(g)
		for _, edge := range g.edges {
			if len(edge.bendPoints) < 2 {
				middleToMiddleEdgeRoute(edge)
			}
		}
	}
}

type multiLevelGapEntry struct {
	edge          *tEdge
	first, second int
}

type multiLevelGap struct {
	neighborOne, neighborTwo *tNode
	entries                  []multiLevelGapEntry
	onFirstSide, onLastSide  bool
}

func (gap *multiLevelGap) add(entry multiLevelGapEntry, g *tGraph) {
	gap.entries = append(gap.entries, entry)
	if len(gap.entries) > 1 {
		sort.SliceStable(gap.entries, func(i, j int) bool {
			left := gap.entries[i].edge.target.position
			right := gap.entries[j].edge.target.position
			if g.direction == "LEFT" || g.direction == "RIGHT" {
				return left.Y < right.Y
			}
			return left.X < right.X
		})
	}
	gap.update(g)
}

func (gap *multiLevelGap) update(g *tGraph) {
	count := len(gap.entries)
	for index, entry := range gap.entries {
		interpolation := float64(index+1) / float64(count+1)
		var first, second geometry.Point
		switch {
		case gap.neighborOne == nil && gap.neighborTwo == nil:
			return
		case gap.neighborOne != nil && gap.neighborTwo == nil:
			gap.onLastSide = true
			node := gap.neighborOne
			switch g.direction {
			case "LEFT":
				coordinate := node.position.Y + node.size.Y + g.spacingEdgeNode*float64(index+1)
				first = geometry.Point{X: node.levelMax + g.spacingEdgeNode, Y: coordinate}
				second = geometry.Point{X: node.levelMin - g.spacingEdgeNode, Y: coordinate}
			case "RIGHT":
				coordinate := node.position.Y + node.size.Y + g.spacingEdgeNode*float64(index+1)
				first = geometry.Point{X: node.levelMin - g.spacingEdgeNode, Y: coordinate}
				second = geometry.Point{X: node.levelMax + g.spacingEdgeNode, Y: coordinate}
			case "UP":
				coordinate := node.position.X + node.size.X + g.spacingEdgeNode*float64(index+1)
				first = geometry.Point{X: coordinate, Y: node.levelMax + g.spacingEdgeNode}
				second = geometry.Point{X: coordinate, Y: node.levelMin - g.spacingEdgeNode}
			default:
				coordinate := node.position.X + node.size.X + g.spacingEdgeNode*float64(index+1)
				first = geometry.Point{X: coordinate, Y: node.levelMin - g.spacingEdgeNode}
				second = geometry.Point{X: coordinate, Y: node.levelMax + g.spacingEdgeNode}
			}
		case gap.neighborOne != nil && gap.neighborTwo != nil:
			one, two := gap.neighborOne, gap.neighborTwo
			if g.direction == "LEFT" || g.direction == "RIGHT" {
				coordinate := two.position.Y*interpolation + (one.position.Y+one.size.Y)*(1-interpolation)
				if g.direction == "LEFT" {
					first = geometry.Point{X: one.levelMax + g.spacingEdgeNode, Y: coordinate}
					second = geometry.Point{X: one.levelMin - g.spacingEdgeNode, Y: coordinate}
				} else {
					first = geometry.Point{X: one.levelMin - g.spacingEdgeNode, Y: coordinate}
					second = geometry.Point{X: one.levelMax + g.spacingEdgeNode, Y: coordinate}
				}
			} else {
				coordinate := two.position.X*interpolation + (one.position.X+one.size.X)*(1-interpolation)
				if g.direction == "UP" {
					first = geometry.Point{X: coordinate, Y: one.levelMax + g.spacingEdgeNode}
					second = geometry.Point{X: coordinate, Y: one.levelMin - g.spacingEdgeNode}
				} else {
					first = geometry.Point{X: coordinate, Y: one.levelMin - g.spacingEdgeNode}
					second = geometry.Point{X: coordinate, Y: one.levelMax + g.spacingEdgeNode}
				}
			}
		default:
			gap.onFirstSide = true
			node := gap.neighborTwo
			switch g.direction {
			case "LEFT":
				coordinate := node.position.Y - g.spacingEdgeNode*float64(index+1)
				first = geometry.Point{X: node.levelMax + g.spacingEdgeNode, Y: coordinate}
				second = geometry.Point{X: node.levelMin - g.spacingEdgeNode, Y: coordinate}
			case "RIGHT":
				coordinate := node.position.Y - g.spacingEdgeNode*float64(index+1)
				first = geometry.Point{X: node.levelMin - g.spacingEdgeNode, Y: coordinate}
				second = geometry.Point{X: node.levelMax + g.spacingEdgeNode, Y: coordinate}
			case "UP":
				coordinate := node.position.X - g.spacingEdgeNode*float64(index+1)
				first = geometry.Point{X: coordinate, Y: node.levelMax + g.spacingEdgeNode}
				second = geometry.Point{X: coordinate, Y: node.levelMin - g.spacingEdgeNode}
			default:
				coordinate := node.position.X - g.spacingEdgeNode*float64(index+1)
				first = geometry.Point{X: coordinate, Y: node.levelMin - g.spacingEdgeNode}
				second = geometry.Point{X: coordinate, Y: node.levelMax + g.spacingEdgeNode}
			}
		}
		entry.edge.bendPoints[entry.first] = first
		entry.edge.bendPoints[entry.second] = second
	}
}

func avoidOverlapSpecialEdges(g *tGraph) {
	maxLevel := 0
	for _, node := range g.nodes {
		maxLevel = max(maxLevel, node.treeLevel)
	}
	outsPerLevel := make([]int, maxLevel+1)
	insPerLevel := make([]int, maxLevel+1)
	gaps := make(map[[2]int]*multiLevelGap)
	sideOneEdges, sideTwoEdges := 0, 0

	for _, edge := range g.edges {
		sourceLevel, targetLevel := edge.source.treeLevel, edge.target.treeLevel
		levelDifference := targetLevel - sourceLevel
		switch {
		case levelDifference > 1:
			for currentLevel := sourceLevel + 1; currentLevel < targetLevel; currentLevel++ {
				levelNodes := make([]*tNode, 0)
				for _, node := range g.nodes {
					if node.treeLevel == currentLevel {
						levelNodes = append(levelNodes, node)
					}
				}
				horizontal := g.direction == "LEFT" || g.direction == "RIGHT"
				sort.SliceStable(levelNodes, func(i, j int) bool {
					if horizontal {
						return levelNodes[i].position.Y < levelNodes[j].position.Y
					}
					return levelNodes[i].position.X < levelNodes[j].position.X
				})
				interpolation := float64(currentLevel-sourceLevel) / float64(targetLevel-sourceLevel)
				gapIndex := 0
				for ; gapIndex < len(levelNodes); gapIndex++ {
					candidate := levelNodes[gapIndex].position.X
					desired := edge.source.position.X*(1-interpolation) + edge.target.position.X*interpolation
					if horizontal {
						candidate = levelNodes[gapIndex].position.Y
						desired = edge.source.position.Y*(1-interpolation) + edge.target.position.Y*interpolation
					}
					if candidate > desired {
						break
					}
				}
				if shouldSkipMultiLevelGap(edge, levelNodes, gapIndex, horizontal) {
					continue
				}
				first := len(edge.bendPoints)
				edge.bendPoints = append(edge.bendPoints, geometry.Point{}, geometry.Point{})
				key := [2]int{currentLevel, gapIndex}
				gap := gaps[key]
				if gap == nil {
					gap = &multiLevelGap{}
					if gapIndex > 0 {
						gap.neighborOne = levelNodes[gapIndex-1]
					}
					if gapIndex < len(levelNodes) {
						gap.neighborTwo = levelNodes[gapIndex]
					}
					gaps[key] = gap
				}
				gap.add(multiLevelGapEntry{edge: edge, first: first, second: first + 1}, g)
				if !horizontal {
					if gap.onFirstSide && gap.neighborTwo != nil && gap.neighborTwo.position.X <= g.graphMin.X {
						sideOneEdges++
					}
					if gap.onLastSide && gap.neighborOne != nil && gap.neighborOne.position.X+gap.neighborOne.size.X >= g.graphMax.X {
						sideTwoEdges++
					}
				} else {
					if gap.onFirstSide && gap.neighborTwo != nil && gap.neighborTwo.position.Y <= g.graphMin.Y {
						sideOneEdges++
					}
					if gap.onLastSide && gap.neighborOne != nil && gap.neighborOne.position.Y+gap.neighborOne.size.Y >= g.graphMax.Y {
						sideTwoEdges++
					}
				}
			}
		case levelDifference == 0:
			middleToMiddleEdgeRoute(edge)
		case levelDifference < 0:
			outsPerLevel[sourceLevel]++
			insPerLevel[targetLevel]++
			sideOneEdges, sideTwoEdges = routeCycleEdge(g, edge, sideOneEdges, sideTwoEdges,
				insPerLevel[targetLevel], outsPerLevel[sourceLevel])
		}
	}
}

func shouldSkipMultiLevelGap(edge *tEdge, levelNodes []*tNode, gapIndex int, horizontal bool) bool {
	if len(levelNodes) == 0 {
		return false
	}
	start := edge.source.position
	if len(edge.bendPoints) > 0 {
		start = edge.bendPoints[len(edge.bendPoints)-1]
	}
	lastNode := levelNodes[len(levelNodes)-1]
	last := lastNode.position.Add(lastNode.size)
	firstNode := levelNodes[0]
	first := firstNode.position.Add(firstNode.size)
	if horizontal {
		if gapIndex >= len(levelNodes)-1 && start.Y > last.Y && edge.target.position.Y > last.Y {
			return true
		}
		// first.X is intentional: it matches the 0.12 Java implementation.
		return gapIndex <= 0 && start.Y < first.X && edge.target.position.Y < first.Y
	}
	if gapIndex >= len(levelNodes)-1 && start.X > last.X && edge.target.position.X > last.X {
		return true
	}
	return gapIndex <= 0 && start.X < first.X && edge.target.position.X < first.X
}

func routeCycleEdge(g *tGraph, edge *tEdge, sideOneEdges, sideTwoEdges, incomingCount, outgoingCount int) (int, int) {
	horizontal := g.direction == "LEFT" || g.direction == "RIGHT"
	middle := 0.0
	for _, node := range g.nodes {
		if horizontal {
			middle += node.position.Y + node.size.Y/2
		} else {
			middle += node.position.X + node.size.X/2
		}
	}
	middle /= float64(len(g.nodes))
	sourceMiddle := edge.source.position.X + edge.source.size.X/2
	if horizontal {
		sourceMiddle = edge.source.position.Y + edge.source.size.Y/2
	}
	bendCoordinate := 0.0
	if sourceMiddle > middle {
		sideTwoEdges++
		bendCoordinate = math.Inf(-1)
		for _, node := range g.nodes {
			coordinate := node.position.X + node.size.X + g.spacingEdgeNode*float64(sideTwoEdges)
			if horizontal {
				coordinate = node.position.Y + node.size.Y + g.spacingEdgeNode*float64(sideTwoEdges)
			}
			bendCoordinate = math.Max(bendCoordinate, coordinate)
		}
	} else {
		sideOneEdges++
		bendCoordinate = math.Inf(1)
		for _, node := range g.nodes {
			coordinate := node.position.X - g.spacingEdgeNode*float64(sideOneEdges)
			if horizontal {
				coordinate = node.position.Y - g.spacingEdgeNode*float64(sideOneEdges)
			}
			bendCoordinate = math.Min(bendCoordinate, coordinate)
		}
	}

	source, target := edge.source, edge.target
	switch g.direction {
	case "LEFT":
		edge.bendPoints = append(edge.bendPoints,
			geometry.Point{X: source.levelMin - g.spacingEdgeNode, Y: bendCoordinate},
			geometry.Point{X: target.position.X + target.size.X + g.spacingEdgeNode + g.edgeEndTextureLength, Y: bendCoordinate},
			geometry.Point{X: target.position.X + target.size.X + g.spacingEdgeNode + g.edgeEndTextureLength, Y: target.position.Y + target.size.Y/2},
			geometry.Point{X: target.position.X + target.size.X, Y: target.position.Y + target.size.Y/2})
	case "RIGHT":
		edge.bendPoints = append(edge.bendPoints,
			geometry.Point{X: source.levelMax + g.spacingEdgeNode, Y: source.position.Y + source.size.Y/2},
			geometry.Point{X: source.position.X + source.size.X + g.spacingEdgeNode, Y: bendCoordinate},
			geometry.Point{X: target.position.X - g.spacingEdgeNode - g.edgeEndTextureLength, Y: bendCoordinate},
			geometry.Point{X: target.position.X - g.spacingEdgeNode - g.edgeEndTextureLength, Y: target.position.Y + target.size.Y/2},
			geometry.Point{X: target.position.X, Y: target.position.Y + target.size.Y/2})
	case "UP":
		edge.bendPoints = append(edge.bendPoints,
			geometry.Point{X: bendCoordinate, Y: source.levelMin - g.spacingEdgeNode},
			geometry.Point{X: bendCoordinate, Y: target.position.Y + target.size.Y + g.spacingEdgeNode + g.edgeEndTextureLength},
			geometry.Point{X: target.position.X + target.size.X/2, Y: target.position.Y + target.size.Y + g.spacingEdgeNode + g.edgeEndTextureLength},
			geometry.Point{X: target.position.X + target.size.X/2, Y: target.position.Y + target.size.Y + g.spacingEdgeNode})
	default:
		if len(edge.bendPoints) > 0 {
			edge.bendPoints[len(edge.bendPoints)-1].Y = source.levelMax + g.spacingEdgeNode*float64(outgoingCount)
		}
		edge.bendPoints = append(edge.bendPoints,
			geometry.Point{X: bendCoordinate, Y: source.levelMax + g.spacingEdgeNode*float64(outgoingCount)},
			geometry.Point{X: bendCoordinate, Y: target.position.Y - g.spacingEdgeNode*float64(incomingCount) - g.edgeEndTextureLength})
	}
	return sideOneEdges, sideTwoEdges
}

func middleToMiddleEdgeRoute(edge *tEdge) {
	sourceHalfWidth := float64(edge.source.size.X / 2)
	sourceHalfHeight := float64(edge.source.size.Y / 2)
	source := geometry.Point{
		X: float64(edge.source.position.X + sourceHalfWidth),
		Y: float64(edge.source.position.Y + sourceHalfHeight),
	}
	targetHalfWidth := float64(edge.target.size.X / 2)
	targetHalfHeight := float64(edge.target.size.Y / 2)
	target := geometry.Point{
		X: float64(edge.target.position.X + targetHalfWidth),
		Y: float64(edge.target.position.Y + targetHalfHeight),
	}
	edge.bendPoints = append(geometry.Chain{source}, edge.bendPoints...)
	edge.bendPoints = append(edge.bendPoints, target)
	if len(edge.bendPoints) >= 2 {
		toNodeBorder(&edge.bendPoints[0], edge.bendPoints[1], edge.source.size)
		last := len(edge.bendPoints) - 1
		toNodeBorder(&edge.bendPoints[last], edge.bendPoints[last-1], edge.target.size)
	}
}

func avoidOverlapStartPoints(g *tGraph) {
	for _, node := range g.nodes {
		if node.dummy {
			continue
		}
		outgoing := make([]*tEdge, 0)
		for _, edge := range g.edges {
			if edge.source == node && edge.source.treeLevel != edge.target.treeLevel {
				outgoing = append(outgoing, edge)
			}
		}
		sort.SliceStable(outgoing, func(i, j int) bool {
			left, right := firstRoutePoint(outgoing[i]), firstRoutePoint(outgoing[j])
			if g.direction == "LEFT" || g.direction == "RIGHT" {
				return left.Y < right.Y
			}
			return left.X < right.X
		})
		for index, edge := range outgoing {
			if node.compactLevelAscension && edge.target.treeLevel-edge.source.treeLevel > 0 {
				continue
			}
			interpolation := .5
			if len(outgoing) != 1 {
				interpolation = float64(index+1) / float64(len(outgoing)+1)
			}
			var start, levelEnd geometry.Point
			switch g.direction {
			case "LEFT":
				y := node.position.Y + node.size.Y*interpolation
				start = geometry.Point{X: node.position.X, Y: y}
				levelEnd = geometry.Point{X: math.Min(node.levelMin, node.position.X-g.spacingEdgeNode), Y: y}
			case "RIGHT":
				y := node.position.Y + node.size.Y*interpolation
				start = geometry.Point{X: node.position.X + node.size.X, Y: y}
				levelEnd = geometry.Point{X: node.levelMax + g.spacingEdgeNode, Y: y}
			case "UP":
				x := node.position.X + node.size.X*interpolation
				start = geometry.Point{X: x, Y: node.position.Y}
				levelEnd = geometry.Point{X: x, Y: math.Min(node.position.Y-g.spacingEdgeNode, node.levelMin)}
			default:
				x := node.position.X + node.size.X*interpolation
				start = geometry.Point{X: x, Y: node.position.Y + node.size.Y}
				levelEnd = geometry.Point{X: x, Y: node.levelMax + g.spacingEdgeNode}
			}
			edge.bendPoints = append(geometry.Chain{start, levelEnd}, edge.bendPoints...)
		}
	}
}

func avoidOverlapEndPoints(g *tGraph) {
	for _, node := range g.nodes {
		if node.dummy {
			continue
		}
		incoming := make([]*tEdge, 0)
		for _, edge := range g.edges {
			if edge.target == node && edge.source.treeLevel != edge.target.treeLevel {
				incoming = append(incoming, edge)
			}
		}
		sort.SliceStable(incoming, func(i, j int) bool {
			left, right := lastRoutePoint(incoming[i]), lastRoutePoint(incoming[j])
			if g.direction == "LEFT" || g.direction == "RIGHT" {
				return left.Y < right.Y
			}
			return left.X < right.X
		})
		for index, edge := range incoming {
			interpolation := .5
			if len(incoming) != 1 {
				interpolation = float64(index+1) / float64(len(incoming)+1)
			}
			switch g.direction {
			case "LEFT":
				y := node.position.Y + node.size.Y*interpolation
				if node.position.X+node.size.X+g.edgeEndTextureLength < node.levelMax {
					edge.bendPoints = append(edge.bendPoints, geometry.Point{X: node.levelMax + g.spacingEdgeNode, Y: y})
				} else if steepHorizontalEnd(edge, node, g.edgeEndTextureLength) {
					last := lastRoutePoint(edge)
					nextY := node.position.Y + node.size.Y/2
					bendY := y
					if nextY > last.Y {
						bendY -= g.edgeEndTextureLength / 2
					} else {
						bendY += g.edgeEndTextureLength / 2
					}
					edge.bendPoints = append(edge.bendPoints, geometry.Point{
						X: node.position.X + node.size.X + g.edgeEndTextureLength/5.3,
						Y: bendY,
					})
				}
				edge.bendPoints = append(edge.bendPoints, geometry.Point{X: node.position.X + node.size.X, Y: y})
			case "RIGHT":
				y := node.position.Y + node.size.Y*interpolation
				if node.position.X-g.edgeEndTextureLength > node.levelMin {
					edge.bendPoints = append(edge.bendPoints, geometry.Point{X: node.levelMin - g.spacingEdgeNode, Y: y})
				} else if steepHorizontalEnd(edge, node, g.edgeEndTextureLength) {
					last := lastRoutePoint(edge)
					nextY := node.position.Y + node.size.Y/2
					bendY := y
					if nextY > last.Y {
						bendY -= g.edgeEndTextureLength / 2
					} else {
						bendY += g.edgeEndTextureLength / 2
					}
					edge.bendPoints = append(edge.bendPoints, geometry.Point{
						X: node.position.X - g.edgeEndTextureLength/5.3,
						Y: bendY,
					})
				}
				edge.bendPoints = append(edge.bendPoints, geometry.Point{X: node.position.X, Y: y})
			case "UP":
				x := node.position.X + node.size.X*interpolation
				if node.position.Y+node.size.Y+g.edgeEndTextureLength < node.levelMax {
					edge.bendPoints = append(edge.bendPoints, geometry.Point{X: x, Y: node.levelMax + g.spacingEdgeNode})
				} else if steepVerticalEnd(edge, node, g.edgeEndTextureLength) {
					last := lastRoutePoint(edge)
					nextX := node.position.X + node.size.X/2
					bendX := x
					if nextX > last.X {
						bendX -= g.edgeEndTextureLength / 2
					} else {
						bendX += g.edgeEndTextureLength / 2
					}
					edge.bendPoints = append(edge.bendPoints, geometry.Point{
						X: bendX,
						Y: node.position.Y + node.size.Y + g.edgeEndTextureLength/5.3,
					})
				}
				edge.bendPoints = append(edge.bendPoints, geometry.Point{X: x, Y: node.position.Y + node.size.Y})
			default:
				x := node.position.X + node.size.X*interpolation
				if edge.target.treeLevel-edge.source.treeLevel < 0 {
					last := lastRoutePoint(edge)
					edge.bendPoints = append(edge.bendPoints, geometry.Point{X: x, Y: last.Y})
				} else if node.position.Y-g.edgeEndTextureLength > node.levelMin {
					edge.bendPoints = append(edge.bendPoints, geometry.Point{X: x, Y: node.levelMin - g.spacingEdgeNode})
				} else if steepVerticalEnd(edge, node, g.edgeEndTextureLength) {
					last := lastRoutePoint(edge)
					nextX := node.position.X + node.size.X/2
					bendX := x
					if nextX > last.X {
						bendX -= g.edgeEndTextureLength / 2
					} else {
						bendX += g.edgeEndTextureLength / 2
					}
					edge.bendPoints = append(edge.bendPoints, geometry.Point{
						X: bendX,
						Y: node.position.Y - g.edgeEndTextureLength/5.3,
					})
				}
				edge.bendPoints = append(edge.bendPoints, geometry.Point{X: x, Y: node.position.Y})
			}
		}
	}
}

func steepHorizontalEnd(edge *tEdge, node *tNode, textureLength float64) bool {
	if textureLength <= 0 || len(edge.bendPoints) == 0 {
		return false
	}
	last := lastRoutePoint(edge)
	next := geometry.Point{X: node.position.X + node.size.X/2, Y: node.position.Y + node.size.Y/2}
	return math.Abs(last.Y-next.Y)/(math.Abs(last.X-next.X)/40) > 50
}

func steepVerticalEnd(edge *tEdge, node *tNode, textureLength float64) bool {
	if textureLength <= 0 || len(edge.bendPoints) == 0 {
		return false
	}
	last := lastRoutePoint(edge)
	next := geometry.Point{X: node.position.X + node.size.X/2, Y: node.position.Y + node.size.Y/2}
	return math.Abs(last.X-next.X)/(math.Abs(last.Y-next.Y)/40) > 50
}

func firstRoutePoint(edge *tEdge) geometry.Point {
	if len(edge.bendPoints) > 0 {
		return edge.bendPoints[0]
	}
	return edge.target.position
}

func lastRoutePoint(edge *tEdge) geometry.Point {
	if len(edge.bendPoints) > 0 {
		return edge.bendPoints[len(edge.bendPoints)-1]
	}
	return edge.source.position
}

func packComponents(components []*tGraph) *tGraph {
	if len(components) == 1 {
		normalizeAndApplyPadding(components[0])
		return components[0]
	}
	if len(components) == 0 {
		return &tGraph{}
	}
	type bounds struct{ min, max geometry.Point }
	boxes := make(map[*tGraph]bounds, len(components))
	priorities := make(map[*tGraph]int, len(components))
	for _, component := range components {
		box := bounds{min: geometry.Point{X: float64(math.MaxInt32), Y: float64(math.MaxInt32)}, max: geometry.Point{X: float64(math.MinInt32), Y: float64(math.MinInt32)}}
		for _, node := range component.nodes {
			priorities[component] += graph.Get(&node.props, Priority)
			box.min.X = math.Min(box.min.X, node.position.X)
			box.min.Y = math.Min(box.min.Y, node.position.Y)
			box.max.X = math.Max(box.max.X, node.position.X+node.size.X)
			box.max.Y = math.Max(box.max.Y, node.position.Y+node.size.Y)
		}
		boxes[component] = box
	}
	sort.SliceStable(components, func(i, j int) bool {
		left, right := components[i], components[j]
		if priorities[left] != priorities[right] {
			return priorities[left] > priorities[right]
		}
		leftBox, rightBox := boxes[left], boxes[right]
		leftArea := (leftBox.max.X - leftBox.min.X) * (leftBox.max.Y - leftBox.min.Y)
		rightArea := (rightBox.max.X - rightBox.min.X) * (rightBox.max.Y - rightBox.min.Y)
		return leftArea < rightArea
	})
	result := components[0].settingsClone()
	maxRowWidth, totalArea := 0.0, 0.0
	for _, component := range components {
		box := boxes[component]
		width, height := box.max.X-box.min.X, box.max.Y-box.min.Y
		maxRowWidth = math.Max(maxRowWidth, width)
		totalArea += width * height
	}
	maxRowWidth = math.Max(maxRowWidth, float64(float32(math.Sqrt(totalArea)))*result.aspectRatio)
	x, y, highest := 0.0, 0.0, 0.0
	for _, component := range components {
		box := boxes[component]
		width, height := box.max.X-box.min.X, box.max.Y-box.min.Y
		if x+width > maxRowWidth {
			x = 0
			y += highest + result.spacing
			highest = 0
		}
		offset := geometry.Point{X: x - box.min.X, Y: y - box.min.Y}
		for _, node := range component.nodes {
			node.position = node.position.Add(offset)
			result.nodes = append(result.nodes, node)
		}
		for _, edge := range component.edges {
			edge.bendPoints.Offset(offset)
			if !containsEdge(result.edges, edge) {
				result.edges = append(result.edges, edge)
			}
		}
		highest = math.Max(highest, height)
		x += width + result.spacing
	}
	computeGraphBounds(result)
	normalizeAndApplyPadding(result)
	return result
}

func normalizeAndApplyPadding(g *tGraph) {
	if len(g.nodes) == 0 {
		return
	}
	offset := geometry.Point{
		X: g.padding.Left - g.graphMin.X,
		Y: g.padding.Top - g.graphMin.Y,
	}
	for _, node := range g.nodes {
		node.position = node.position.Add(offset)
	}
	seen := make(map[*tEdge]bool, len(g.edges))
	for _, edge := range g.edges {
		if seen[edge] {
			continue
		}
		seen[edge] = true
		edge.bendPoints.Offset(offset)
	}
	g.graphMin = g.graphMin.Add(offset)
	g.graphMax = g.graphMax.Add(offset)
}

func applyLayout(model *graph.Model, parent graph.NodeID, g *tGraph) error {
	minX, minY := float64(math.MaxInt32), float64(math.MaxInt32)
	maxX, maxY := float64(math.MinInt32), float64(math.MinInt32)
	for _, node := range g.nodes {
		minX = math.Min(minX, node.position.X-node.size.X/2)
		minY = math.Min(minY, node.position.Y-node.size.Y/2)
		maxX = math.Max(maxX, node.position.X+node.size.X/2)
		maxY = math.Max(maxY, node.position.Y+node.size.Y/2)
	}
	for _, node := range g.nodes {
		if node.origin == nil {
			continue
		}
		origin := model.Node(*node.origin)
		origin.Shape.SetLocation(node.position.X, node.position.Y)
		origin.Properties.CopyFrom(&node.props)
	}
	for _, edge := range g.edges {
		if edge.origin == nil || edge.source.origin == nil || edge.target.origin == nil {
			continue
		}
		if len(edge.bendPoints) < 2 {
			return fmt.Errorf("mrtree: the vector chain must contain at least a source and a target point")
		}
		origin := model.Edge(*edge.origin)
		var sectionID graph.SectionID
		if len(origin.Sections) == 0 {
			sectionID = model.AddSection(*edge.origin)
		} else {
			sectionID = origin.Sections[0]
		}
		section := model.Section(sectionID)
		section.Start = edge.bendPoints[0]
		section.End = edge.bendPoints[len(edge.bendPoints)-1]
		section.StartPresent, section.EndPresent = true, true
		if g.edgeRoutingMode == "MIDDLE_TO_MIDDLE" {
			// elkjs 0.12's JSON bridge exposes both vectors in this mode as
			// bend points in addition to using them as the section endpoints.
			section.BendPoints = append(section.BendPoints[:0], edge.bendPoints...)
		} else {
			section.BendPoints = append(section.BendPoints[:0], edge.bendPoints[1:len(edge.bendPoints)-1]...)
		}
		section.BendPointsPresent = len(section.BendPoints) > 0
	}
	root := model.Node(parent)
	width := maxX - minX + g.padding.Left + g.padding.Right
	height := maxY - minY + g.padding.Top + g.padding.Bottom
	if !g.fixedGraphSize {
		root.Shape.SetDimensions(width, height)
	}
	graph.Set(&root.Properties, core.ChildAreaWidth, width-g.padding.Left-g.padding.Right)
	graph.Set(&root.Properties, core.ChildAreaHeight, height-g.padding.Top-g.padding.Bottom)
	return nil
}

func toNodeBorder(center *geometry.Point, next geometry.Point, size geometry.Point) {
	halfWidth, halfHeight := float64(size.X/2), float64(size.Y/2)
	deltaX := float64(next.X - center.X)
	deltaY := float64(next.Y - center.Y)
	absX, absY := math.Abs(deltaX), math.Abs(deltaY)
	xScale, yScale := 1.0, 1.0
	if absX > halfWidth {
		xScale = float64(halfWidth / absX)
	}
	if absY > halfHeight {
		yScale = float64(halfHeight / absY)
	}
	scale := math.Min(xScale, yScale)
	xOffset := float64(scale * deltaX)
	yOffset := float64(scale * deltaY)
	center.X = float64(center.X + xOffset)
	center.Y = float64(center.Y + yOffset)
}
