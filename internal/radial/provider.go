// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2017, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package radial

import (
	"fmt"
	"math"
	"sort"

	"github.com/d2lang/elk-go/internal/basicoptions"
	"github.com/d2lang/elk-go/internal/common/nodespacing"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
)

type Provider struct{}

func Register(registry *core.AlgorithmRegistry) error {
	return registry.Register(core.AlgorithmData{
		ID:     AlgorithmID,
		Create: func() core.LayoutProvider { return &Provider{} },
		KnownOptions: []string{
			Position.ID,
			SpacingNodeNode.ID,
			"org.eclipse.elk.nodeSize.constraints",
			basicoptions.NodeSizeFixedGraphSize.ID,
			"org.eclipse.elk.nodeSize.minimum",
			"org.eclipse.elk.nodeSize.options",
			"org.eclipse.elk.nodeLabels.placement",
			OmitNodeMicroLayout.ID,
			"org.eclipse.elk.portLabels.placement",
			CompactionStepSize.ID,
			Compactor.ID,
			Rotate.ID,
			RotationTargetAngle.ID,
			RotationComputeAdditionalWedgeSpace.ID,
			RotationOutgoingEdgeAngles.ID,
			OptimizationCriteria.ID,
			OrderID.ID,
			Radius.ID,
			Sorter.ID,
			WedgeCriteria.ID,
			CenterOnRoot.ID,
		},
	})
}

func (*Provider) Layout(model *graph.Model, parent graph.NodeID, monitor core.ProgressMonitor) error {
	parentNode := model.Node(parent)
	if !graph.Get(&parentNode.Properties, OmitNodeMicroLayout) {
		nodespacing.CalculateLabelAndNodeSizes(model, parent, nil)
		nodespacing.NewNodeMarginCalculator().Process(model, parent)
	}
	root := findRoot(model, parent)
	if root == nil {
		return fmt.Errorf("the given graph is not a tree")
	}
	radius := graph.Get(&parentNode.Properties, Radius)
	if radius == 0 {
		radius = largestNode(model, parent)
	}
	graph.Set(&parentNode.Properties, Radius, radius)
	monitor.Begin("Radial layout", 5)
	positionGraph(model, *root, radius, graph.Get(&parentNode.Properties, Sorter), graph.Get(&parentNode.Properties, WedgeCriteria), graph.Get(&parentNode.Properties, OptimizationCriteria))
	monitor.Worked(1)
	removeOverlaps(model, *root, graph.Get(&parentNode.Properties, SpacingNodeNode), graph.Get(&parentNode.Properties, Sorter))
	monitor.Worked(1)
	switch graph.Get(&parentNode.Properties, Compactor) {
	case "RADIAL_COMPACTION":
		radialCompact(model, *root, graph.Get(&parentNode.Properties, SpacingNodeNode), graph.Get(&parentNode.Properties, CompactionStepSize), graph.Get(&parentNode.Properties, Sorter))
	case "WEDGE_COMPACTION":
		wedgeCompact(model, *root, graph.Get(&parentNode.Properties, SpacingNodeNode), graph.Get(&parentNode.Properties, CompactionStepSize), graph.Get(&parentNode.Properties, Sorter))
	}
	monitor.Worked(1)
	if graph.Get(&parentNode.Properties, Rotate) {
		rotateGraph(model, parent, *root)
	}
	calculateGraphSize(model, parent, *root)
	monitor.Worked(1)
	routeEdges(model, *root)
	if graph.Get(&parentNode.Properties, RotationOutgoingEdgeAngles) {
		calculateOutgoingEdgeAngles(model, *root)
	}
	monitor.Worked(1)
	monitor.Done()
	return nil
}

func findRoot(model *graph.Model, parent graph.NodeID) *graph.NodeID {
	for _, child := range model.Node(parent).Children {
		if len(allIncoming(model, child)) == 0 {
			root := child
			return &root
		}
	}
	return nil
}

func treeParent(model *graph.Model, node graph.NodeID) *graph.NodeID {
	edges := allIncoming(model, node)
	if len(edges) == 0 || len(model.Edge(edges[0]).Sources) == 0 {
		return nil
	}
	parent := model.ShapeNode(model.Edge(edges[0]).Sources[0])
	return &parent
}

func allIncoming(model *graph.Model, node graph.NodeID) []graph.EdgeID {
	result := append([]graph.EdgeID(nil), model.Node(node).IncomingEdges...)
	for _, port := range model.Node(node).Ports {
		result = append(result, model.Port(port).IncomingEdges...)
	}
	return result
}

// successors retains RadialUtil.getSuccessors's unusual port behavior: an
// edge whose first source is a port is ignored entirely.
func successors(model *graph.Model, node graph.NodeID) []graph.NodeID {
	children := make(map[graph.NodeID]struct{}, len(model.Node(node).Children))
	for _, child := range model.Node(node).Children {
		children[child] = struct{}{}
	}
	result := make([]graph.NodeID, 0)
	for _, edgeID := range model.Node(node).OutgoingEdges {
		edge := model.Edge(edgeID)
		if len(edge.Sources) == 0 || edge.Sources[0].Kind == graph.PortShape || len(edge.Targets) == 0 {
			continue
		}
		target := model.ShapeNode(edge.Targets[0])
		if _, nested := children[target]; !nested {
			result = append(result, target)
		}
	}
	return result
}

func largestNode(model *graph.Model, parent graph.NodeID) float64 {
	largest := 0.0
	for _, child := range model.Node(parent).Children {
		node := model.Node(child)
		largest = math.Max(largest, radialLength(node.Shape.Width, node.Shape.Height))
		largest = math.Max(largest, largestNode(model, child))
	}
	return largest
}

func leafCount(model *graph.Model, node graph.NodeID) int {
	children := successors(model, node)
	if len(children) == 0 {
		return 1
	}
	result := 0
	for _, child := range children {
		result += leafCount(model, child)
	}
	return result
}

func wedgeSpace(model *graph.Model, node graph.NodeID, strategy string) float64 {
	if strategy == "LEAF_NUMBER" {
		return float64(leafCount(model, node))
	}
	shape := model.Node(node).Shape
	space := math.Sqrt(shape.Width*shape.Width + shape.Height*shape.Height)
	childrenSpace := 0.0
	for _, child := range successors(model, node) {
		childrenSpace += wedgeSpace(model, child, strategy)
	}
	return math.Max(childrenSpace, space)
}

func positionGraph(model *graph.Model, root graph.NodeID, radius float64, sorter, wedge, optimizer string) {
	offset := 0.0
	if optimizer != "NONE" {
		best := math.MaxFloat64
		for degree := 0; degree < 360; degree++ {
			candidate := float64(degree) * math.Pi / 180
			positionNodes(model, root, root, radius, sorter, wedge, 0, 0, 2*math.Pi, candidate)
			value := evaluate(model, root, optimizer)
			if value < best {
				best, offset = value, candidate
			}
		}
	}
	positionNodes(model, root, root, radius, sorter, wedge, 0, 0, 2*math.Pi, offset)
}

func positionNodes(model *graph.Model, root, node graph.NodeID, radius float64, sorter, wedge string, currentRadius, minAlpha, maxAlpha, offset float64) {
	alphaPoint := (minAlpha+maxAlpha)/2 + offset
	centerX := currentRadius * javacompat.JSMathCos(alphaPoint)
	centerY := currentRadius * javacompat.JSMathSin(alphaPoint)
	shape := &model.Node(node).Shape
	shape.SetLocation(centerX-shape.Width/2, centerY-shape.Height/2)
	totalSpace := wedgeSpace(model, node, wedge)
	// EadesRadial 0.8.1 computes tau with
	// currentRadius/currentRadius + radius. For every useful radius this is
	// NaN or outside acos's domain, so the observable branch uses the whole
	// available wedge.
	step := (maxAlpha - minAlpha) / totalSpace
	alpha := minAlpha
	children := successors(model, node)
	sortNodes(model, root, children, sorter)
	for _, child := range children {
		childSpace := wedgeSpace(model, child, wedge)
		positionNodes(model, root, child, radius, sorter, wedge, currentRadius+radius, alpha, alpha+step*childSpace, offset)
		alpha += step * childSpace
	}
}

func sortNodes(model *graph.Model, root graph.NodeID, nodes []graph.NodeID, strategy string) {
	switch strategy {
	case "ID":
		sort.SliceStable(nodes, func(i, j int) bool {
			return graph.Get(&model.Node(nodes[i]).Properties, OrderID) < graph.Get(&model.Node(nodes[j]).Properties, OrderID)
		})
	case "POLAR_COORDINATE":
		initializePolarSorter(model, root)
		sort.SliceStable(nodes, func(i, j int) bool {
			return graph.Get(&model.Node(nodes[i]).Properties, OrderID) < graph.Get(&model.Node(nodes[j]).Properties, OrderID)
		})
	}
}

func initializePolarSorter(model *graph.Model, root graph.NodeID) {
	first := successors(model, root)
	polarSort(model, first, 0)
	setPolarIDs(model, first, 0)
}

func setPolarIDs(model *graph.Model, nodes []graph.NodeID, offset int) int {
	id, nextLayerID := offset, 0
	for _, node := range nodes {
		graph.Set(&model.Node(node).Properties, OrderID, id)
		id++
		children := successors(model, node)
		shape := model.Node(node).Shape
		arc := math.Atan2(shape.Y+shape.Height/2, shape.X+shape.Width/2)
		if arc < 0 {
			arc += 2 * math.Pi
		}
		switch {
		case arc < math.Pi/4 || arc > 7*math.Pi/4:
			polarSort(model, children, math.Pi)
		case arc <= 7*math.Pi/4 && arc > 5*math.Pi/4:
			polarSort(model, children, 3*math.Pi/2)
		case arc <= 5*math.Pi/4 && arc > 3*math.Pi/4:
			polarSort(model, children, 0)
		case arc <= 3*math.Pi/4:
			polarSort(model, children, math.Pi/2)
		}
		nextLayerID = setPolarIDs(model, children, nextLayerID)
	}
	return id
}

func polarSort(model *graph.Model, nodes []graph.NodeID, radialOffset float64) {
	sort.SliceStable(nodes, func(i, j int) bool {
		left, right := polarArc(graph.Get(&model.Node(nodes[i]).Properties, Position), radialOffset), polarArc(graph.Get(&model.Node(nodes[j]).Properties, Position), radialOffset)
		if math.Abs(left-right) <= 1e-10 {
			return false
		}
		return left < right
	})
}

func polarArc(position geometry.Point, offset float64) float64 {
	arc := math.Atan2(position.Y, position.X)
	if arc < 0 {
		arc += 2 * math.Pi
	}
	arc += offset
	if arc > 2*math.Pi {
		arc -= 2 * math.Pi
	}
	return arc
}

func evaluate(model *graph.Model, root graph.NodeID, strategy string) float64 {
	switch strategy {
	case "EDGE_LENGTH":
		return evaluateEdgeLength(model, root)
	case "EDGE_LENGTH_BY_POSITION":
		return evaluateEdgeLengthByPosition(model, root)
	case "CROSSING_MINIMIZATION_BY_POSITION":
		return evaluateCrossings(model, root)
	default:
		return 0
	}
}

func evaluateEdgeLength(model *graph.Model, root graph.NodeID) float64 {
	result := 0.0
	rootShape := model.Node(root).Shape
	for _, child := range successors(model, root) {
		childShape := model.Node(child).Shape
		source := geometry.Point{X: rootShape.X + rootShape.Width/2, Y: rootShape.Y + rootShape.Height/2}
		target := geometry.Point{X: childShape.X + childShape.Width/2, Y: childShape.Y + childShape.Height/2}
		source, target = clippedEndpoints(source, target, geometry.Point{X: rootShape.Width, Y: rootShape.Height}, geometry.Point{X: childShape.Width, Y: childShape.Height})
		vectorX := target.X - source.X
		vectorY := target.Y - source.Y
		// EdgeLengthOptimization uses Math.sqrt(x*x + y*y), rather than
		// Math.hypot. The rounding difference is observable when rotationally
		// equivalent offsets tie and ELK retains the first minimum.
		result += math.Sqrt(vectorX*vectorX + vectorY*vectorY)
	}
	return result
}

func evaluateEdgeLengthByPosition(model *graph.Model, root graph.NodeID) float64 {
	result := 0.0
	rootShape := model.Node(root).Shape
	for _, child := range successors(model, root) {
		childShape := model.Node(child).Shape
		target := geometry.Point{X: childShape.X + childShape.Width/2, Y: childShape.Y + childShape.Height/2}
		position := graph.Get(&model.Node(child).Properties, Position)
		source := geometry.Point{X: rootShape.X + position.X + rootShape.Width/2, Y: rootShape.Y + position.Y + rootShape.Height}
		result += radialLength(target.X-source.X, target.Y-source.Y)
	}
	return result
}

func evaluateCrossings(model *graph.Model, root graph.NodeID) float64 {
	nodes := successors(model, root)
	crossings := 0
	for first := range nodes {
		for second := first + 1; second < len(nodes); second++ {
			if crossing(model, root, nodes[first], nodes[second]) {
				crossings++
			}
		}
	}
	return float64(crossings)
}

func crossing(model *graph.Model, root, first, second graph.NodeID) bool {
	rootShape := model.Node(root).Shape
	// CrossingMinimizationPosition intentionally uses root.x/width for both
	// axes in ELK 0.8.1.
	rootX := rootShape.X + rootShape.Width/2
	rootY := rootShape.X + rootShape.Width/2
	line := func(node graph.NodeID) (geometry.Point, geometry.Point, float64, float64) {
		shape := model.Node(node).Shape
		center := geometry.Point{X: shape.X + shape.Width/2, Y: shape.Y + shape.Height/2}
		position := graph.Get(&model.Node(node).Properties, Position)
		position.X += rootX
		position.Y += rootY
		graph.Set(&model.Node(node).Properties, Position, position)
		slope := (center.Y - position.Y) / (center.X - position.X)
		intercept := center.Y - slope*center.X
		return center, position, slope, intercept
	}
	center1, position1, slope1, intercept1 := line(first)
	center2, position2, slope2, intercept2 := line(second)
	xCut := (intercept1 - intercept2) / (slope2 - slope1)
	if position1.X < xCut && center1.X < xCut || xCut < position1.X && xCut < center1.X {
		return false
	}
	if position2.X < xCut && center2.X < xCut || xCut < position2.X && xCut < center2.X {
		return false
	}
	return true
}

type radialMover struct {
	model   *graph.Model
	root    graph.NodeID
	spacing float64
	step    int
}

func (m radialMover) contractLayer(nodes []graph.NodeID, contracting bool) {
	root := m.model.Node(m.root).Shape
	rootCenter := geometry.Point{X: root.X + root.Width/2, Y: root.Y + root.Height/2}
	for _, nodeID := range nodes {
		node := &m.model.Node(nodeID).Shape
		center := geometry.Point{X: node.X + node.Width/2, Y: node.Y + node.Height/2}
		delta := center.Sub(rootCenter)
		length := radialLength(delta.X, delta.Y)
		delta = delta.Scale(float64(m.step) / length)
		if contracting {
			center = center.Sub(delta)
		} else {
			center = center.Add(delta)
		}
		node.SetLocation(center.X-node.Width/2, center.Y-node.Height/2)
	}
}

func (m radialMover) moveNode(nodeID graph.NodeID, distance float64) {
	root := m.model.Node(m.root).Shape
	node := &m.model.Node(nodeID).Shape
	rootCenter := geometry.Point{X: root.X + root.Width/2, Y: root.Y + root.Height/2}
	nodeCenter := geometry.Point{X: node.X + node.Width/2, Y: node.Y + node.Height/2}
	delta := nodeCenter.Sub(rootCenter)
	length := radialLength(delta.X, delta.Y)
	node.SetLocation(node.X+delta.X/length*distance, node.Y+delta.Y/length*distance)
}

func (m radialMover) overlap(firstID, secondID graph.NodeID) bool {
	first, second := m.model.Node(firstID).Shape, m.model.Node(secondID).Shape
	x1, x2 := first.X-m.spacing/2, second.X-m.spacing/2
	y1, y2 := first.Y-m.spacing/2, second.Y-m.spacing/2
	w1, w2 := first.Width+m.spacing, second.Width+m.spacing
	h1, h2 := first.Height+m.spacing, second.Height+m.spacing
	return x1 < x2+w2 && x2 < x1 && y1 < y2+h2 && y2 < y1 ||
		x2 < x1+w1 && x1 < x2 && y2 < y1+h1 && y1 < y2 ||
		x1 < x2+w2 && x2 < x1 && y1 < y2 && y2 < y1+h1 ||
		x2 < x1+w1 && x1 < x2 && y1 < y2+h2 && y2 < y1
}

func (m radialMover) overlapLayer(nodes []graph.NodeID) bool {
	if len(nodes) < 2 {
		return false
	}
	for index := range nodes {
		if m.overlap(nodes[index], nodes[(index+1)%len(nodes)]) {
			return true
		}
	}
	return false
}

func removeOverlaps(model *graph.Model, root graph.NodeID, spacing float64, sorter string) {
	mover := radialMover{model: model, root: root, spacing: spacing, step: 1}
	var extend func([]graph.NodeID)
	extend = func(nodes []graph.NodeID) {
		if len(nodes) == 0 {
			return
		}
		old := make([]geometry.Point, len(nodes))
		for index, node := range nodes {
			old[index] = model.Node(node).Shape.Position()
		}
		for mover.overlapLayer(nodes) {
			mover.contractLayer(nodes, false)
		}
		firstPosition := model.Node(nodes[0]).Shape.Position()
		distance := radialLength(firstPosition.X-old[0].X, firstPosition.Y-old[0].Y)
		next := uniqueNextLevel(model, nodes)
		if len(next) > 0 {
			for _, node := range next {
				mover.moveNode(node, distance)
			}
		}
		sortNodes(model, root, next, sorter)
		extend(next)
	}
	extend(successors(model, root))
}

func uniqueNextLevel(model *graph.Model, nodes []graph.NodeID) []graph.NodeID {
	seen := make(map[graph.NodeID]struct{})
	result := make([]graph.NodeID, 0)
	for _, node := range nodes {
		for _, child := range successors(model, node) {
			if _, found := seen[child]; found {
				continue
			}
			seen[child] = struct{}{}
			result = append(result, child)
		}
	}
	return result
}

func radialCompact(model *graph.Model, root graph.NodeID, spacing float64, step int, sorter string) {
	mover := radialMover{model: model, root: root, spacing: spacing, step: step}
	lastRadius := 0.0
	var radius func(graph.NodeID) float64
	radius = func(node graph.NodeID) float64 {
		shape, rootShape := model.Node(node).Shape, model.Node(root).Shape
		return radialLength(shape.X-rootShape.X, shape.Y-rootShape.Y)
	}
	var compact func([]graph.NodeID)
	compact = func(nodes []graph.NodeID) {
		if len(nodes) == 0 {
			return
		}
		sortNodes(model, root, nodes, sorter)
		overlapping := func() bool {
			if mover.overlapLayer(nodes) {
				return true
			}
			for _, node := range nodes {
				parent := treeParent(model, node)
				if parent != nil && mover.overlap(node, *parent) {
					return true
				}
				if radius(node)-spacing <= lastRadius {
					return true
				}
			}
			return false
		}
		contracted := false
		for !overlapping() {
			mover.contractLayer(nodes, true)
			contracted = true
		}
		if contracted {
			mover.contractLayer(nodes, false)
		}
		next := make([]graph.NodeID, 0)
		for _, node := range nodes {
			next = append(next, successors(model, node)...)
		}
		lastRadius = radius(nodes[0])
		compact(next)
	}
	compact(successors(model, root))
}

func wedgeCompact(model *graph.Model, root graph.NodeID, spacing float64, step int, sorter string) {
	mover := radialMover{model: model, root: root, spacing: spacing, step: step}
	first := successors(model, root)
	sortNodes(model, root, first, sorter)
	leftContours := make(map[graph.NodeID][]graph.NodeID)
	rightContours := make(map[graph.NodeID][]graph.NodeID)
	for _, parent := range first {
		leftContours[parent] = append(leftContours[parent], parent)
		rightContours[parent] = append(rightContours[parent], parent)
		level := successors(model, parent)
		for len(level) > 0 {
			sortNodes(model, root, level, sorter)
			leftContours[parent] = append(leftContours[parent], level[0])
			rightContours[parent] = append(rightContours[parent], level[len(level)-1])
			next := make([]graph.NodeID, 0)
			for _, node := range level {
				next = append(next, successors(model, node)...)
			}
			level = next
		}
	}
	contourOverlap := func(parent, node graph.NodeID, left bool) bool {
		contour := rightContours[parent]
		if left {
			contour = leftContours[parent]
		}
		for _, other := range contour {
			if mover.overlap(node, other) {
				return true
			}
		}
		return false
	}
	var compactWedge func([]graph.NodeID, graph.NodeID, graph.NodeID)
	compactWedge = func(nodes []graph.NodeID, leftParent, rightParent graph.NodeID) {
		if len(nodes) == 0 {
			return
		}
		sortNodes(model, root, nodes, sorter)
		predecessors := make([]graph.NodeID, 0)
		for _, node := range nodes {
			if parent := treeParent(model, node); parent != nil && !containsNode(predecessors, *parent) {
				predecessors = append(predecessors, *parent)
			}
		}
		overlapping := func() bool {
			if contourOverlap(leftParent, nodes[0], false) || contourOverlap(rightParent, nodes[len(nodes)-1], true) || mover.overlapLayer(nodes) {
				return true
			}
			for _, node := range nodes {
				for _, predecessor := range predecessors {
					if mover.overlap(node, predecessor) {
						return true
					}
				}
			}
			return false
		}
		contracted := false
		for !overlapping() {
			mover.contractLayer(nodes, true)
			contracted = true
		}
		if contracted {
			mover.contractLayer(nodes, false)
		}
		next := make([]graph.NodeID, 0)
		for _, node := range nodes {
			next = append(next, successors(model, node)...)
		}
		compactWedge(next, leftParent, rightParent)
	}
	for pass := 0; pass < 2; pass++ {
		for index, node := range first {
			left := first[(index+len(first)-1)%len(first)]
			right := first[(index+1)%len(first)]
			compactWedge([]graph.NodeID{node}, left, right)
		}
	}
}

func containsNode(nodes []graph.NodeID, node graph.NodeID) bool {
	for _, candidate := range nodes {
		if candidate == node {
			return true
		}
	}
	return false
}

func rotateGraph(model *graph.Model, parent, root graph.NodeID) {
	parentNode := model.Node(parent)
	targetAngle := graph.Get(&parentNode.Properties, RotationTargetAngle)
	if graph.Get(&parentNode.Properties, RotationComputeAdditionalWedgeSpace) {
		outgoing := model.Node(root).OutgoingEdges
		if len(outgoing) > 0 {
			firstEdge, lastEdge := model.Edge(outgoing[0]), model.Edge(outgoing[len(outgoing)-1])
			if len(firstEdge.Targets) > 0 && len(lastEdge.Targets) > 0 {
				firstNode := model.Node(model.ShapeNode(firstEdge.Targets[0]))
				lastNode := model.Node(model.ShapeNode(lastEdge.Targets[0]))
				firstVector := geometry.Point{X: firstNode.Shape.X + firstNode.Shape.Width/2, Y: firstNode.Shape.Y + firstNode.Shape.Height/2}
				lastVector := geometry.Point{X: lastNode.Shape.X + lastNode.Shape.Width/2, Y: lastNode.Shape.Y + lastNode.Shape.Height/2}
				alpha := targetAngle
				if alpha <= 0 {
					alpha += 2 * math.Pi
				}
				wedgeAngle := math.Acos((lastVector.X*firstVector.X + lastVector.Y*firstVector.Y) /
					(math.Hypot(lastVector.X, lastVector.Y) * math.Hypot(firstVector.X, firstVector.Y)))
				if wedgeAngle <= 0 {
					wedgeAngle += 2 * math.Pi
				}
				alignmentAngle := math.Atan2(lastVector.Y, lastVector.X)
				if alignmentAngle <= 0 {
					alignmentAngle += 2 * math.Pi
				}
				targetAngle = math.Pi - (alignmentAngle - alpha + wedgeAngle/2)
			}
		}
	}
	sine, cosine := javacompat.JSMathSin(targetAngle), javacompat.JSMathCos(targetAngle)
	for _, child := range parentNode.Children {
		node := model.Node(child)
		centerX := node.Shape.X + node.Shape.Width/2
		centerY := node.Shape.Y + node.Shape.Height/2
		xCosine := float64(centerX * cosine)
		ySine := float64(centerY * sine)
		xSine := float64(centerX * sine)
		yCosine := float64(centerY * cosine)
		rotatedX := float64(xCosine - ySine)
		rotatedY := float64(xSine + yCosine)
		node.Shape.SetLocation(rotatedX-node.Shape.Width/2, rotatedY-node.Shape.Height/2)
	}
}

func radialLength(x, y float64) float64 {
	// KVector.length() evaluates each square and their sum as separate
	// JavaScript binary64 operations. Keep Go from fusing either product.
	xSquared := float64(x * x)
	ySquared := float64(y * y)
	return math.Sqrt(float64(xSquared + ySquared))
}

func calculateGraphSize(model *graph.Model, parent, root graph.NodeID) {
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := math.SmallestNonzeroFloat64, math.SmallestNonzeroFloat64
	for _, child := range model.Node(parent).Children {
		node := model.Node(child)
		margins := graph.Get(&node.Properties, nodespacing.Margins)
		minX = math.Min(minX, node.Shape.X-margins.Left)
		minY = math.Min(minY, node.Shape.Y-margins.Top)
		maxX = math.Max(maxX, node.Shape.X+node.Shape.Width+margins.Right)
		maxY = math.Max(maxY, node.Shape.Y+node.Shape.Height+margins.Bottom)
	}
	padding := graph.Get(&model.Node(parent).Properties, Padding)
	offset := geometry.Point{X: minX - padding.Left, Y: minY - padding.Top}
	width := maxX - minX + padding.Left + padding.Right
	height := maxY - minY + padding.Top + padding.Bottom
	parentNode := model.Node(parent)
	if graph.Get(&parentNode.Properties, CenterOnRoot) {
		rootNode := model.Node(root)
		rootMargins := graph.Get(&rootNode.Properties, nodespacing.Margins)
		rootX := rootNode.Shape.X + rootNode.Shape.Width/2 + (rootMargins.Left+rootMargins.Right)/2 - offset.X
		rootY := rootNode.Shape.Y + rootNode.Shape.Height/2 + (rootMargins.Top+rootMargins.Bottom)/2 - offset.Y
		dx, dy := width-rootX, height-rootY
		if dx < width/2 {
			additional := dx - rootX
			width += additional
			offset.X -= additional
		} else {
			width += rootX - dx
		}
		if dy < height/2 {
			additional := dy - rootY
			height += additional
			offset.Y -= additional
		} else {
			height += rootY - dy
		}
	}
	for _, child := range model.Node(parent).Children {
		node := model.Node(child)
		node.Shape.SetLocation(node.Shape.X-offset.X, node.Shape.Y-offset.Y)
	}
	if !graph.Get(&parentNode.Properties, basicoptions.NodeSizeFixedGraphSize) {
		parentNode.Shape.SetDimensions(width, height)
	}
	graph.Set(&parentNode.Properties, core.ChildAreaWidth, width-padding.Left-padding.Right)
	graph.Set(&parentNode.Properties, core.ChildAreaHeight, height-padding.Top-padding.Bottom)
}

func calculateOutgoingEdgeAngles(model *graph.Model, root graph.NodeID) {
	for _, edgeID := range model.Node(root).OutgoingEdges {
		edge := model.Edge(edgeID)
		if len(edge.Sections) == 0 || len(edge.Targets) == 0 {
			continue
		}
		section := model.Section(edge.Sections[0])
		angle := math.Atan2(section.End.Y-section.Start.Y, section.End.X-section.Start.X)
		target := model.ShapeNode(edge.Targets[0])
		graph.Set(&model.Node(target).Properties, RotationTargetAngle, angle)
	}
}

// routeEdges gives every tree edge below node a straight section, depth
// first. It is ELK's recursion made iterative, so that deep trees need no
// deeper call stack (each level of the recursion also held a section
// composite on its frame, which grew with the property map): frames holds
// the pending calls. A call walks its node's outgoing edges in order (the
// list does not change while edges are routed), routing an edge and then
// making the call for its target, which completes before the next edge is
// looked at. (A cycle below the root never gets here: wedgeSpace recurses
// over it without end first, in the original too.)
func routeEdges(model *graph.Model, node graph.NodeID) {
	type frame struct {
		node graph.NodeID
		next int
	}
	frames := []frame{{node: node}}
	for len(frames) > 0 {
		top := &frames[len(frames)-1]
		node := top.node
		outgoing := model.Node(node).OutgoingEdges
		if top.next >= len(outgoing) {
			frames = frames[:len(frames)-1]
			continue
		}
		edgeID := outgoing[top.next]
		top.next++
		edge := model.Edge(edgeID)
		if len(edge.Sources) == 0 || edge.Sources[0].Kind == graph.PortShape || len(edge.Targets) == 0 {
			continue
		}
		target := model.ShapeNode(edge.Targets[0])
		if edge.ContainingNode == nil || model.Node(node).Parent == nil || *edge.ContainingNode != *model.Node(node).Parent {
			continue
		}
		sourceShape, targetShape := model.Node(node).Shape, model.Node(target).Shape
		source := geometry.Point{X: sourceShape.X + sourceShape.Width/2, Y: sourceShape.Y + sourceShape.Height/2}
		end := geometry.Point{X: targetShape.X + targetShape.Width/2, Y: targetShape.Y + targetShape.Height/2}
		source, end = clippedEndpoints(source, end, geometry.Point{X: sourceShape.Width, Y: sourceShape.Height}, geometry.Point{X: targetShape.Width, Y: targetShape.Height})
		var sectionID graph.SectionID
		if len(edge.Sections) == 0 {
			sectionID = model.AddSection(edgeID)
		} else {
			sectionID = edge.Sections[0]
		}
		section := model.Section(sectionID)
		section.Start, section.End = source, end
		section.StartPresent, section.EndPresent = true, true
		section.BendPoints = nil
		section.BendPointsPresent = false
		frames = append(frames, frame{node: target}) // the call routeEdges(model, target)
	}
}

func clippedEndpoints(source, target geometry.Point, sourceSize, targetSize geometry.Point) (geometry.Point, geometry.Point) {
	vector := target.Sub(source)
	sourceClip := clipVector(vector, sourceSize)
	vector = vector.Sub(sourceClip)
	source = target.Sub(vector)
	targetClip := clipVector(vector, targetSize)
	vector = vector.Sub(targetClip)
	target = source.Add(vector)
	return source, target
}

func clipVector(vector, size geometry.Point) geometry.Point {
	xScale, yScale := 1.0, 1.0
	if math.Abs(vector.X) > size.X/2 {
		xScale = size.X / 2 / math.Abs(vector.X)
	}
	if math.Abs(vector.Y) > size.Y/2 {
		yScale = size.Y / 2 / math.Abs(vector.Y)
	}
	return vector.Scale(math.Min(xScale, yScale))
}
