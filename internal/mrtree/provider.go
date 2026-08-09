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
			"org.eclipse.elk.aspectRatio",
			"org.eclipse.elk.priority",
			"org.eclipse.elk.separateConnectedComponents",
			"org.eclipse.elk.debugMode",
			"org.eclipse.elk.nodeSize.constraints",
			"org.eclipse.elk.nodeSize.minimum",
			"org.eclipse.elk.nodeSize.options",
			"org.eclipse.elk.nodeLabels.placement",
			"org.eclipse.elk.omitNodeMicroLayout",
			"org.eclipse.elk.portLabels.placement",
			Weighting.ID,
			SearchOrder.ID,
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
	applyLayout(model, parent, packed)
	return nil
}

func importGraph(model *graph.Model, parent graph.NodeID) *tGraph {
	parentNode := model.Node(parent)
	g := &tGraph{
		padding:     graph.Get(&parentNode.Properties, Padding),
		spacing:     graph.Get(&parentNode.Properties, SpacingNodeNode),
		aspectRatio: graph.Get(&parentNode.Properties, AspectRatio),
		separate:    graph.Get(&parentNode.Properties, SeparateConnectedComponents),
		searchOrder: graph.Get(&parentNode.Properties, SearchOrder),
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
		component := &tGraph{padding: g.padding, spacing: g.spacing, aspectRatio: g.aspectRatio, separate: g.separate, searchOrder: g.searchOrder}
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
	orderNodes(root)
	computeLevelHeights(root)
	computeNeighbors(root)
	placeNodes(g, root)
	for _, node := range g.nodes {
		for _, edge := range node.outgoing {
			edge.bendPoints = nil
		}
	}
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

func orderNodes(root *tNode) {
	var orderLevel func([]*tNode)
	orderLevel = func(current []*tNode) {
		if len(current) == 0 {
			return
		}
		sort.SliceStable(current, func(i, j int) bool { return current[i].fan < current[j].fan })
		firstLeaf := len(current)
		for firstLeaf > 0 && current[firstLeaf-1].fan == 0 {
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
				if leaves[index].fan != 0 {
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

func computeLevelHeights(root *tNode) {
	level := []*tNode{root}
	for len(level) > 0 {
		height := 0.0
		next := make([]*tNode, 0)
		for _, node := range level {
			if node.size.Y > height {
				height = node.size.Y
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
			width += left.size.X / 2
		}
		if right != nil {
			width += right.size.X / 2
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

func packComponents(components []*tGraph) *tGraph {
	if len(components) == 1 {
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
	result := &tGraph{padding: components[0].padding, spacing: components[0].spacing, aspectRatio: components[0].aspectRatio, separate: components[0].separate, searchOrder: components[0].searchOrder}
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
	return result
}

func applyLayout(model *graph.Model, parent graph.NodeID, g *tGraph) {
	minX, minY := float64(math.MaxInt32), float64(math.MaxInt32)
	maxX, maxY := float64(math.MinInt32), float64(math.MinInt32)
	for _, node := range g.nodes {
		minX = math.Min(minX, node.position.X-node.size.X/2)
		minY = math.Min(minY, node.position.Y-node.size.Y/2)
		maxX = math.Max(maxX, node.position.X+node.size.X/2)
		maxY = math.Max(maxY, node.position.Y+node.size.Y/2)
	}
	offset := geometry.Point{X: g.padding.Left - minX, Y: g.padding.Top - minY}
	for _, node := range g.nodes {
		if node.origin == nil {
			continue
		}
		node.position = node.position.Add(offset)
		origin := model.Node(*node.origin)
		origin.Shape.SetLocation(node.position.X-origin.Shape.Width/2, node.position.Y-origin.Shape.Height/2)
	}
	for _, edge := range g.edges {
		if edge.origin == nil || edge.source.origin == nil || edge.target.origin == nil {
			continue
		}
		source, target := edge.source.position, edge.target.position
		toNodeBorder(&source, target, edge.source.size)
		toNodeBorder(&target, source, edge.target.size)
		origin := model.Edge(*edge.origin)
		var sectionID graph.SectionID
		if len(origin.Sections) == 0 {
			sectionID = model.AddSection(*edge.origin)
		} else {
			sectionID = origin.Sections[0]
		}
		section := model.Section(sectionID)
		section.Start, section.End = source, target
		section.StartPresent, section.EndPresent = true, true
		// The elkjs 0.8.2 JSON bridge exposes Mr. Tree's two-point vector
		// chain in bendPoints as well as startPoint/endPoint.
		section.BendPoints = geometry.Chain{source, target}
		section.BendPointsPresent = true
	}
	root := model.Node(parent)
	root.Shape.SetDimensions(maxX-minX+g.padding.Left+g.padding.Right, maxY-minY+g.padding.Top+g.padding.Bottom)
}

func toNodeBorder(center *geometry.Point, next geometry.Point, size geometry.Point) {
	halfWidth, halfHeight := size.X/2, size.Y/2
	absX, absY := math.Abs(next.X-center.X), math.Abs(next.Y-center.Y)
	xScale, yScale := 1.0, 1.0
	if absX > halfWidth {
		xScale = halfWidth / absX
	}
	if absY > halfHeight {
		yScale = halfHeight / absY
	}
	scale := math.Min(xScale, yScale)
	center.X += scale * (next.X - center.X)
	center.Y += scale * (next.Y - center.Y)
}
