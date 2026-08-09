// Copyright (c) 2016 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
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

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

type externalCompactionDirection uint8

const (
	externalDirectionUndefined externalCompactionDirection = iota
	externalDirectionLeft
	externalDirectionRight
	externalDirectionUp
	externalDirectionDown
)

func (direction externalCompactionDirection) horizontal() bool {
	return direction == externalDirectionLeft || direction == externalDirectionRight
}

type externalCompactionExtension struct {
	direction        externalCompactionDirection
	side             layered.PortSide
	edges            []layered.LEdgeID
	group            *componentCompactionGroup
	node             *componentCompactionNode
	placeholder      *componentCompactionNode
	placeholderGroup *componentCompactionGroup
}

type externalCompactedComponent struct {
	graphID     layered.LGraphID
	oldPosition geometry.Point
	offsetNode  *componentCompactionNode
}

type externalComponentsCompactor struct {
	arena             *layered.LGraphArena
	componentIDs      []layered.LGraphID
	spacing           float64
	graph             *componentCompactionGraph
	components        []externalCompactedComponent
	extensions        []*externalCompactionExtension
	horizontal        []*externalCompactionExtension
	vertical          []*externalCompactionExtension
	graphTopLeft      geometry.Point
	graphBottomRight  geometry.Point
	compactionTopLeft geometry.Point
	compactor         *componentOneDimensionalCompactor
}

func compactExternalComponentGroups(
	arena *layered.LGraphArena,
	componentIDs []layered.LGraphID,
	targetID layered.LGraphID,
	spacing float64,
) error {
	for _, componentID := range componentIDs {
		component := arena.Graph(componentID)
		offsetGraph(arena, componentID, component.Offset)
	}
	size, offset, err := compactConnectedComponentsWithExternal(
		arena, componentIDs, spacing)
	if err != nil {
		return err
	}
	for _, componentID := range componentIDs {
		arena.Graph(componentID).Offset = offset
	}
	arena.Graph(targetID).Size = size
	return nil
}

// compactConnectedComponentsWithExternal is ComponentsCompactor plus
// ComponentsToCGraphTransformer and OneDimensionalComponentsCompaction for
// ComponentGroupGraphPlacer's external-port path. Component coordinates are
// moved by their individual compaction offsets. The returned common offset is
// installed as every component graph's offset by the caller and is therefore
// applied exactly once by AbstractGraphPlacer.moveGraph.
func compactConnectedComponentsWithExternal(
	arena *layered.LGraphArena,
	componentIDs []layered.LGraphID,
	spacing float64,
) (geometry.Point, geometry.Point, error) {
	compaction := &externalComponentsCompactor{
		arena:            arena,
		componentIDs:     append([]layered.LGraphID(nil), componentIDs...),
		spacing:          spacing,
		graph:            &componentCompactionGraph{},
		graphTopLeft:     geometry.Point{X: positiveInfinity, Y: positiveInfinity},
		graphBottomRight: geometry.Point{X: negativeInfinity, Y: negativeInfinity},
	}
	if err := compaction.build(); err != nil {
		return geometry.Point{}, geometry.Point{}, err
	}
	compaction.compact()
	return compaction.apply()
}

func (compaction *externalComponentsCompactor) build() error {
	for _, graphID := range compaction.componentIDs {
		for _, nodeID := range compaction.arena.Graph(graphID).LayerlessNodes {
			node := compaction.arena.Node(nodeID)
			compaction.graphTopLeft.X = math.Min(
				compaction.graphTopLeft.X, node.Position.X-node.Margin.Left)
			compaction.graphTopLeft.Y = math.Min(
				compaction.graphTopLeft.Y, node.Position.Y-node.Margin.Top)
			compaction.graphBottomRight.X = math.Max(
				compaction.graphBottomRight.X,
				node.Position.X+node.Size.X+node.Margin.Right)
			compaction.graphBottomRight.Y = math.Max(
				compaction.graphBottomRight.Y,
				node.Position.Y+node.Size.Y+node.Margin.Bottom)
		}
	}

	for _, graphID := range compaction.componentIDs {
		if err := compaction.transformGraph(graphID); err != nil {
			return err
		}
	}

	// ELK adds all extension representors before constructing the one-
	// dimensional compactor so their offsets within the component group are
	// initialized, then removes them until the relevant directional pass.
	for _, extension := range compaction.horizontal {
		compaction.addExtension(extension)
	}
	for _, extension := range compaction.vertical {
		compaction.addExtension(extension)
	}
	compaction.compactor = newComponentOneDimensionalCompactor(
		compaction.graph, compaction.spacing)

	compaction.compactionTopLeft = geometry.Point{
		X: positiveInfinity, Y: positiveInfinity,
	}
	bottomRight := geometry.Point{X: negativeInfinity, Y: negativeInfinity}
	for _, node := range compaction.graph.nodes {
		compaction.compactionTopLeft.X = math.Min(
			compaction.compactionTopLeft.X, node.hitbox.X)
		compaction.compactionTopLeft.Y = math.Min(
			compaction.compactionTopLeft.Y, node.hitbox.Y)
		bottomRight.X = math.Max(bottomRight.X, node.hitbox.MaxX())
		bottomRight.Y = math.Max(bottomRight.Y, node.hitbox.MaxY())
	}

	for _, extension := range compaction.horizontal {
		compaction.removeExtension(extension)
	}
	for _, extension := range compaction.vertical {
		compaction.removeExtension(extension)
	}
	return nil
}

func (compaction *externalComponentsCompactor) transformGraph(
	graphID layered.LGraphID,
) error {
	graph := compaction.arena.Graph(graphID)
	containsRegularNode := false
	for _, nodeID := range graph.LayerlessNodes {
		containsRegularNode = containsRegularNode ||
			compaction.arena.Node(nodeID).Type == layered.NormalNode
	}
	if !containsRegularNode {
		if err := compaction.createDummyNode(graphID); err != nil {
			return err
		}
	}

	hull := newComponentCompactionHull()
	for _, nodeID := range graph.LayerlessNodes {
		node := compaction.arena.Node(nodeID)
		if node.Type == layered.ExternalPortNode {
			continue
		}
		hull.addNode(node)
		for _, edgeID := range compaction.arena.NodeOutgoingEdges(nodeID) {
			if compaction.externalEdge(edgeID) {
				continue
			}
			for _, point := range compaction.arena.Edge(edgeID).BendPoints {
				hull.addPoint(point)
			}
		}
	}

	type outerSegments struct {
		minimum [5]float64
		maximum [5]float64
		extent  [5]float64
		edges   [5][]layered.LEdgeID
	}
	outer := outerSegments{}
	for index := range outer.minimum {
		outer.minimum[index] = positiveInfinity
		outer.maximum[index] = negativeInfinity
		outer.extent[index] = negativeInfinity
	}
	for _, nodeID := range graph.LayerlessNodes {
		for _, edgeID := range compaction.arena.NodeOutgoingEdges(nodeID) {
			if !compaction.externalEdge(edgeID) {
				continue
			}
			side, inner, outside, err := compaction.externalEdgeSegments(edgeID)
			if err != nil {
				return err
			}
			thickness := math.Max(basegraph.Get(
				&compaction.arena.Edge(edgeID).Properties,
				layered.EdgeThickness), 1)
			for _, segment := range inner {
				hull.addRectangle(segmentRectangle(
					segment[0], segment[1], thickness))
			}
			outsideRectangle := segmentRectangle(
				outside[0], outside[1], thickness)
			if side == layered.WestSide || side == layered.EastSide {
				outer.minimum[side] = math.Min(
					outer.minimum[side], outsideRectangle.Y)
				outer.maximum[side] = math.Max(
					outer.maximum[side], outsideRectangle.MaxY())
			} else {
				outer.minimum[side] = math.Min(
					outer.minimum[side], outsideRectangle.X)
				outer.maximum[side] = math.Max(
					outer.maximum[side], outsideRectangle.MaxX())
			}
			externalNode := compaction.externalNode(edgeID)
			margins := compaction.arena.Node(externalNode).Margin
			var extent float64
			switch side {
			case layered.WestSide:
				extent = margins.Right
			case layered.EastSide:
				extent = margins.Left
			case layered.NorthSide:
				extent = margins.Bottom
			case layered.SouthSide:
				extent = margins.Top
			}
			outer.extent[side] = math.Max(outer.extent[side], extent)
			outer.edges[side] = appendUniqueEdge(outer.edges[side], edgeID)
		}
	}

	connections := basegraph.Get(&graph.Properties,
		layered.ExternalPortConnections)
	graphExtensions := make([]*externalCompactionExtension, 0, 4)
	for side := layered.NorthSide; side <= layered.WestSide; side++ {
		if connections&(1<<side) == 0 {
			continue
		}
		if len(outer.edges[side]) == 0 {
			return fmt.Errorf(
				"components: external side %s has no external edge", side)
		}
		minimum, maximum := outer.minimum[side], outer.maximum[side]
		extent := outer.extent[side]
		var representor, placeholder geometry.Rectangle
		switch side {
		case layered.WestSide:
			representor = geometry.Rectangle{
				X: compaction.graphTopLeft.X, Y: minimum,
				Width:  hull.topLeft.X - compaction.graphTopLeft.X,
				Height: maximum - minimum,
			}
			placeholder = geometry.Rectangle{
				X: compaction.graphTopLeft.X, Y: minimum,
				Width: extent, Height: maximum - minimum,
			}
			hull.addPoint(representor.TopRight())
			hull.addPoint(representor.BottomRight())
		case layered.EastSide:
			representor = geometry.Rectangle{
				X: hull.bottomRight.X, Y: minimum,
				Width:  compaction.graphBottomRight.X - hull.bottomRight.X,
				Height: maximum - minimum,
			}
			placeholder = geometry.Rectangle{
				X: compaction.graphBottomRight.X - extent, Y: minimum,
				Width: extent, Height: maximum - minimum,
			}
			hull.addPoint(representor.Position())
			hull.addPoint(representor.BottomLeft())
		case layered.NorthSide:
			representor = geometry.Rectangle{
				X: minimum, Y: compaction.graphTopLeft.Y,
				Width:  maximum - minimum,
				Height: hull.topLeft.Y - compaction.graphTopLeft.Y,
			}
			placeholder = geometry.Rectangle{
				X: minimum, Y: compaction.graphTopLeft.Y,
				Width: maximum - minimum, Height: extent,
			}
			hull.addPoint(representor.BottomLeft())
			hull.addPoint(representor.BottomRight())
		case layered.SouthSide:
			representor = geometry.Rectangle{
				X: minimum, Y: hull.bottomRight.Y,
				Width:  maximum - minimum,
				Height: compaction.graphBottomRight.Y - hull.bottomRight.Y,
			}
			placeholder = geometry.Rectangle{
				X: minimum, Y: compaction.graphBottomRight.Y - extent,
				Width: maximum - minimum, Height: extent,
			}
			hull.addPoint(representor.Position())
			hull.addPoint(representor.TopRight())
		}
		placeholderSpacing := 1.0
		extension := &externalCompactionExtension{
			direction: externalDirectionForSide(side),
			side:      side,
			edges:     append([]layered.LEdgeID(nil), outer.edges[side]...),
			node: &componentCompactionNode{
				hitbox: representor, reposition: true,
				lock: componentLocks(connections),
			},
			placeholder: &componentCompactionNode{
				hitbox: placeholder, spacing: &placeholderSpacing,
				reposition: true, lock: componentLocks(connections),
			},
		}
		extension.placeholderGroup = &componentCompactionGroup{
			nodes:     []*componentCompactionNode{extension.placeholder},
			reference: extension.placeholder, reposition: true,
		}
		extension.placeholder.group = extension.placeholderGroup
		graphExtensions = append(graphExtensions, extension)
	}

	rectangles := splitRectilinearHull(hull.points)
	if len(rectangles) == 0 {
		return fmt.Errorf("components: component %d has no rectilinear hull", graphID)
	}
	groupNodes := make([]*componentCompactionNode, 0, len(rectangles))
	locks := componentLocks(connections)
	for _, rectangle := range rectangles {
		node := &componentCompactionNode{
			hitbox: rectangle, reposition: true, lock: locks,
		}
		compaction.graph.nodes = append(compaction.graph.nodes, node)
		groupNodes = append(groupNodes, node)
	}
	group := compaction.graph.addGroup(groupNodes...)
	for _, extension := range graphExtensions {
		extension.group = group
		compaction.extensions = append(compaction.extensions, extension)
		if extension.direction.horizontal() {
			compaction.horizontal = append(compaction.horizontal, extension)
		} else {
			compaction.vertical = append(compaction.vertical, extension)
		}
	}
	compaction.components = append(compaction.components,
		externalCompactedComponent{
			graphID: graphID, oldPosition: rectangles[0].Position(),
			offsetNode: groupNodes[0],
		})
	return nil
}

func (compaction *externalComponentsCompactor) createDummyNode(
	graphID layered.LGraphID,
) error {
	graph := compaction.arena.Graph(graphID)
	if len(graph.LayerlessNodes) != 1 {
		return fmt.Errorf(
			"components: component %d without regular nodes has %d external nodes",
			graphID, len(graph.LayerlessNodes))
	}
	externalID := graph.LayerlessNodes[0]
	external := compaction.arena.Node(externalID)
	if external.Type != layered.ExternalPortNode || len(external.Ports) == 0 {
		return fmt.Errorf(
			"components: component %d has no usable external-port dummy", graphID)
	}
	dummyID := compaction.arena.AddNode(graphID)
	dummy := compaction.arena.Node(dummyID)
	dummy.Size = geometry.Point{
		X: math.Max(1, external.Size.X),
		Y: math.Max(1, external.Size.Y),
	}
	dummy.Position = external.Position
	side := basegraph.Get(&external.Properties, layered.ExternalPortSide)
	switch side {
	case layered.WestSide:
		dummy.Position.X += 2
	case layered.NorthSide:
		dummy.Position.Y += 2
	case layered.EastSide:
		dummy.Position.X -= 2
	case layered.SouthSide:
		dummy.Position.Y -= 2
	}
	graph.LayerlessNodes = append(graph.LayerlessNodes, dummyID)
	dummyPortID := compaction.arena.AddPort()
	compaction.arena.SetPortNode(dummyPortID, &dummyID)
	externalPortID := external.Ports[0]
	dummyPort := compaction.arena.Port(dummyPortID)
	dummyPort.Position = compaction.arena.Port(externalPortID).Position
	dummyPort.Anchor = compaction.arena.Port(externalPortID).Anchor
	dummyEdge := compaction.arena.AddEdge()
	compaction.arena.SetEdgeSource(dummyEdge, &externalPortID)
	compaction.arena.SetEdgeTarget(dummyEdge, &dummyPortID)
	return nil
}

func (compaction *externalComponentsCompactor) compact() {
	compaction.compactor.lockingStrategy = func(
		node *componentCompactionNode,
		_ componentCompactDirection,
	) bool {
		return node.group.outDegree != 0
	}
	run := 0
	for {
		delta := compaction.compactRun()
		run++
		if !((run < 2 || delta > connectedComponentsEpsilon) &&
			run < connectedComponentsMaxIterations) {
			break
		}
	}
	compaction.compactor.lockingStrategy = func(
		node *componentCompactionNode,
		direction componentCompactDirection,
	) bool {
		return node.lock[direction] ||
			(node.group.outDegree != 0 && node.lock[direction])
	}
	compaction.compactRun()
}

func (compaction *externalComponentsCompactor) compactRun() float64 {
	for _, group := range compaction.graph.groups {
		group.delta = 0
		group.deltaNormalized = 0
	}

	compaction.addPlaceholders(true)
	for _, extension := range compaction.vertical {
		compaction.addExtension(extension)
	}
	compaction.compactor.calculateGroupOffsets()
	compaction.compactor.forceConstraintsRecalculation()
	compaction.compactor.changeDirection(componentDirectionLeft)
	compaction.compactor.compact()
	compaction.compactor.changeDirection(componentDirectionRight)
	compaction.compactor.applyLockingStrategy()
	compaction.compactor.compact()
	compaction.compactor.changeDirection(componentDirectionLeft)
	compaction.compactor.applyLockingStrategy()
	compaction.compactor.compact()
	compaction.compactor.changeDirection(componentDirectionLeft)
	for _, extension := range compaction.vertical {
		compaction.removeExtension(extension)
	}
	compaction.removePlaceholders(true)
	compaction.updateExternalExtensions(true)
	compaction.updatePlaceholders(false)

	compaction.addPlaceholders(false)
	for _, extension := range compaction.horizontal {
		compaction.addExtension(extension)
	}
	compaction.compactor.calculateGroupOffsets()
	delta := 0.0
	for _, group := range compaction.graph.groups {
		delta += math.Abs(group.deltaNormalized)
		group.delta = 0
		group.deltaNormalized = 0
	}
	compaction.compactor.changeDirection(componentDirectionUp)
	compaction.compactor.forceConstraintsRecalculation()
	compaction.compactor.compact()
	compaction.compactor.changeDirection(componentDirectionDown)
	compaction.compactor.applyLockingStrategy()
	compaction.compactor.compact()
	compaction.compactor.changeDirection(componentDirectionUp)
	compaction.compactor.applyLockingStrategy()
	compaction.compactor.compact()
	compaction.compactor.changeDirection(componentDirectionLeft)
	for _, extension := range compaction.horizontal {
		compaction.removeExtension(extension)
	}
	compaction.removePlaceholders(false)
	compaction.updateExternalExtensions(false)
	compaction.updatePlaceholders(true)
	compaction.compactor.forceConstraintsRecalculation()
	for _, group := range compaction.graph.groups {
		delta += math.Abs(group.deltaNormalized)
	}
	return delta
}

func (compaction *externalComponentsCompactor) addExtension(
	extension *externalCompactionExtension,
) {
	compaction.graph.nodes = append(compaction.graph.nodes, extension.node)
	extension.group.nodes = append(extension.group.nodes, extension.node)
	extension.node.group = extension.group
}

func (compaction *externalComponentsCompactor) removeExtension(
	extension *externalCompactionExtension,
) {
	compaction.graph.nodes = removeCompactionNode(
		compaction.graph.nodes, extension.node)
	extension.group.nodes = removeCompactionNode(
		extension.group.nodes, extension.node)
}

func (compaction *externalComponentsCompactor) addPlaceholders(horizontal bool) {
	for _, extension := range compaction.extensions {
		if extension.direction.horizontal() != horizontal {
			continue
		}
		compaction.graph.nodes = append(
			compaction.graph.nodes, extension.placeholder)
		compaction.graph.groups = append(
			compaction.graph.groups, extension.placeholderGroup)
	}
}

func (compaction *externalComponentsCompactor) removePlaceholders(horizontal bool) {
	for _, extension := range compaction.extensions {
		if extension.direction.horizontal() != horizontal {
			continue
		}
		compaction.graph.nodes = removeCompactionNode(
			compaction.graph.nodes, extension.placeholder)
		compaction.graph.groups = removeCompactionGroup(
			compaction.graph.groups, extension.placeholderGroup)
	}
}

func (compaction *externalComponentsCompactor) updatePlaceholders(horizontal bool) {
	for _, extension := range compaction.extensions {
		if extension.direction.horizontal() != horizontal {
			continue
		}
		delta := extension.group.deltaNormalized
		if extension.direction.horizontal() {
			extension.placeholder.hitbox.Y += delta
		} else {
			extension.placeholder.hitbox.X += delta
		}
	}
}

func (compaction *externalComponentsCompactor) updateExternalExtensions(
	horizontal bool,
) {
	for _, extension := range compaction.extensions {
		if extension.direction.horizontal() != horizontal {
			continue
		}
		delta := extension.group.deltaNormalized
		switch extension.direction {
		case externalDirectionLeft:
			extension.node.hitbox.X = compaction.compactionTopLeft.X
			extension.node.hitbox.Width = math.Max(
				1, extension.node.hitbox.Width+delta)
		case externalDirectionRight:
			extension.node.hitbox.X += delta
			extension.node.hitbox.Width = math.Max(
				1, extension.node.hitbox.Width-delta)
		case externalDirectionUp:
			extension.node.hitbox.Y = compaction.compactionTopLeft.Y
			extension.node.hitbox.Height = math.Max(
				1, extension.node.hitbox.Height+delta)
		case externalDirectionDown:
			extension.node.hitbox.Y += delta
			extension.node.hitbox.Height = math.Max(
				1, extension.node.hitbox.Height-delta)
		}
	}
}

func (compaction *externalComponentsCompactor) apply() (
	geometry.Point,
	geometry.Point,
	error,
) {
	topLeft := geometry.Point{X: positiveInfinity, Y: positiveInfinity}
	bottomRight := geometry.Point{X: negativeInfinity, Y: negativeInfinity}
	for _, node := range compaction.graph.nodes {
		topLeft.X = math.Min(topLeft.X, node.hitbox.X)
		topLeft.Y = math.Min(topLeft.Y, node.hitbox.Y)
		bottomRight.X = math.Max(bottomRight.X, node.hitbox.MaxX())
		bottomRight.Y = math.Max(bottomRight.Y, node.hitbox.MaxY())
	}
	for _, extension := range compaction.extensions {
		node := extension.placeholder
		topLeft.X = math.Min(topLeft.X, node.hitbox.X)
		topLeft.Y = math.Min(topLeft.Y, node.hitbox.Y)
		bottomRight.X = math.Max(bottomRight.X, node.hitbox.MaxX())
		bottomRight.Y = math.Max(bottomRight.Y, node.hitbox.MaxY())
	}
	globalOffset := geometry.Point{X: -topLeft.X, Y: -topLeft.Y}
	graphSize := bottomRight.Sub(topLeft)
	for _, component := range compaction.components {
		newPosition := component.offsetNode.hitbox.Position()
		individualOffset := newPosition.Sub(component.oldPosition).Add(globalOffset)
		offsetGraph(compaction.arena, component.graphID, individualOffset)
		for _, nodeID := range compaction.arena.Graph(component.graphID).LayerlessNodes {
			node := compaction.arena.Node(nodeID)
			if node.Type != layered.ExternalPortNode {
				continue
			}
			node.Position = compaction.externalPortPosition(
				node.Position,
				basegraph.Get(&node.Properties, layered.ExternalPortSide))
		}
	}

	yetAnotherOffset := geometry.Point{}
	seenEdges := make(map[layered.LEdgeID]bool)
	for _, extension := range compaction.extensions {
		for _, edgeID := range extension.edges {
			if seenEdges[edgeID] {
				continue
			}
			seenEdges[edgeID] = true
			edge := compaction.arena.Edge(edgeID)
			if edge.Source == nil || edge.Target == nil {
				continue
			}
			points := make(geometry.Chain, 0, len(edge.BendPoints)+2)
			points = append(points,
				componentPortAbsoluteAnchor(compaction.arena, *edge.Source))
			points = append(points, edge.BendPoints...)
			points = append(points,
				componentPortAbsoluteAnchor(compaction.arena, *edge.Target))
			for index := 1; index < len(points); index++ {
				last, current := points[index-1], points[index]
				if math.Abs(last.X-current.X) <= connectedComponentsEpsilon {
					yetAnotherOffset.X = math.Min(yetAnotherOffset.X, last.X)
					graphSize.X = math.Max(graphSize.X, last.X)
				} else if math.Abs(last.Y-current.Y) <= connectedComponentsEpsilon {
					yetAnotherOffset.Y = math.Min(yetAnotherOffset.Y, last.Y)
					graphSize.Y = math.Max(graphSize.Y, last.Y)
				}
			}
		}
	}
	yetAnotherOffset.X = -yetAnotherOffset.X
	yetAnotherOffset.Y = -yetAnotherOffset.Y
	graphSize = graphSize.Add(yetAnotherOffset)
	return graphSize, yetAnotherOffset, nil
}

func (compaction *externalComponentsCompactor) externalEdge(
	edgeID layered.LEdgeID,
) bool {
	edge := compaction.arena.Edge(edgeID)
	return edge.Source != nil && edge.Target != nil &&
		(compaction.arena.Node(compaction.arena.EdgeSourceNode(edgeID)).Type ==
			layered.ExternalPortNode ||
			compaction.arena.Node(compaction.arena.EdgeTargetNode(edgeID)).Type ==
				layered.ExternalPortNode)
}

func (compaction *externalComponentsCompactor) externalNode(
	edgeID layered.LEdgeID,
) layered.LNodeID {
	source := compaction.arena.EdgeSourceNode(edgeID)
	if compaction.arena.Node(source).Type == layered.ExternalPortNode {
		return source
	}
	return compaction.arena.EdgeTargetNode(edgeID)
}

func (compaction *externalComponentsCompactor) externalEdgeSegments(
	edgeID layered.LEdgeID,
) (layered.PortSide, [][2]geometry.Point, [2]geometry.Point, error) {
	edge := compaction.arena.Edge(edgeID)
	if edge.Source == nil || edge.Target == nil {
		return layered.UndefinedSide, nil, [2]geometry.Point{},
			fmt.Errorf("components: external edge %d has an unset endpoint", edgeID)
	}
	externalPort := *edge.Source
	outerFirst := compaction.arena.Node(
		compaction.arena.EdgeSourceNode(edgeID)).Type == layered.ExternalPortNode
	if !outerFirst {
		externalPort = *edge.Target
	}
	externalNode := *compaction.arena.Port(externalPort).Node
	side := basegraph.Get(
		&compaction.arena.Node(externalNode).Properties,
		layered.ExternalPortSide)
	first := componentPortAbsoluteAnchor(compaction.arena, *edge.Source)
	last := componentPortAbsoluteAnchor(compaction.arena, *edge.Target)
	if outerFirst {
		first = compaction.externalPortPosition(first, side)
		last = componentPortPositionOnMargin(compaction.arena, *edge.Target)
	} else {
		first = componentPortPositionOnMargin(compaction.arena, *edge.Source)
		last = compaction.externalPortPosition(last, side)
	}
	points := make(geometry.Chain, 0, len(edge.BendPoints)+2)
	points = append(points, first)
	points = append(points, edge.BendPoints...)
	points = append(points, last)
	inner := make([][2]geometry.Point, 0, len(points)-2)
	var outside [2]geometry.Point
	for index := 0; index+1 < len(points); index++ {
		segment := [2]geometry.Point{points[index], points[index+1]}
		if outerFirst && index == 0 || !outerFirst && index == len(points)-2 {
			outside = segment
		} else {
			inner = append(inner, segment)
		}
	}
	return side, inner, outside, nil
}

func (compaction *externalComponentsCompactor) externalPortPosition(
	position geometry.Point,
	side layered.PortSide,
) geometry.Point {
	switch side {
	case layered.NorthSide:
		position.Y = math.Min(compaction.graphTopLeft.Y, position.Y)
	case layered.EastSide:
		position.X = math.Max(compaction.graphBottomRight.X, position.X)
	case layered.SouthSide:
		position.Y = math.Max(compaction.graphBottomRight.Y, position.Y)
	case layered.WestSide:
		position.X = math.Min(position.X, compaction.graphTopLeft.X)
	}
	return position
}

type componentCompactionHull struct {
	points      []hullPoint
	topLeft     geometry.Point
	bottomRight geometry.Point
}

func newComponentCompactionHull() *componentCompactionHull {
	return &componentCompactionHull{
		topLeft:     geometry.Point{X: positiveInfinity, Y: positiveInfinity},
		bottomRight: geometry.Point{X: negativeInfinity, Y: negativeInfinity},
	}
}

func (hull *componentCompactionHull) addPoint(point geometry.Point) {
	hull.points = append(hull.points, newHullPoint(point.X, point.Y))
	hull.topLeft.X = math.Min(hull.topLeft.X, point.X)
	hull.topLeft.Y = math.Min(hull.topLeft.Y, point.Y)
	hull.bottomRight.X = math.Max(hull.bottomRight.X, point.X)
	hull.bottomRight.Y = math.Max(hull.bottomRight.Y, point.Y)
}

func (hull *componentCompactionHull) addRectangle(rectangle geometry.Rectangle) {
	hull.addPoint(rectangle.Position())
	hull.addPoint(rectangle.TopRight())
	hull.addPoint(rectangle.BottomLeft())
	hull.addPoint(rectangle.BottomRight())
}

func (hull *componentCompactionHull) addNode(node *layered.LNode) {
	hull.addRectangle(geometry.Rectangle{
		X:      node.Position.X - node.Margin.Left,
		Y:      node.Position.Y - node.Margin.Top,
		Width:  node.Size.X + node.Margin.Left + node.Margin.Right,
		Height: node.Size.Y + node.Margin.Top + node.Margin.Bottom,
	})
}

func componentPortAbsoluteAnchor(
	arena *layered.LGraphArena,
	portID layered.LPortID,
) geometry.Point {
	port := arena.Port(portID)
	return arena.Node(*port.Node).Position.Add(port.Position).Add(port.Anchor)
}

func componentPortPositionOnMargin(
	arena *layered.LGraphArena,
	portID layered.LPortID,
) geometry.Point {
	port := arena.Port(portID)
	position := componentPortAbsoluteAnchor(arena, portID)
	margins := arena.Node(*port.Node).Margin
	switch port.Side {
	case layered.NorthSide:
		position.Y -= margins.Top
	case layered.EastSide:
		position.X += margins.Right
	case layered.SouthSide:
		position.Y += margins.Bottom
	case layered.WestSide:
		position.X -= margins.Left
	}
	return position
}

func segmentRectangle(
	first, second geometry.Point,
	extent float64,
) geometry.Rectangle {
	return geometry.Rectangle{
		X:      math.Min(first.X, second.X) - extent/2,
		Y:      math.Min(first.Y, second.Y) - extent/2,
		Width:  math.Abs(first.X-second.X) + extent,
		Height: math.Abs(first.Y-second.Y) + extent,
	}
}

func externalDirectionForSide(side layered.PortSide) externalCompactionDirection {
	switch side {
	case layered.NorthSide:
		return externalDirectionUp
	case layered.EastSide:
		return externalDirectionRight
	case layered.SouthSide:
		return externalDirectionDown
	case layered.WestSide:
		return externalDirectionLeft
	default:
		return externalDirectionUndefined
	}
}

func componentLocks(sides uint8) [5]bool {
	all := [5]bool{false, true, true, true, true}
	switch sides {
	case sidesNone:
		return all
	case sidesNorth:
		return [5]bool{false, true, true, true, false}
	case sidesEast:
		return [5]bool{false, false, true, true, true}
	case sidesSouth:
		return [5]bool{false, true, true, false, true}
	case sidesWest:
		return [5]bool{false, true, false, true, true}
	case sidesNorthEast:
		return [5]bool{false, false, true, true, false}
	case sidesEastSouth:
		return [5]bool{false, false, true, false, true}
	case sidesSouthWest:
		return [5]bool{false, true, false, false, true}
	case sidesNorthWest:
		return [5]bool{false, true, false, true, false}
	default:
		return all
	}
}

func appendUniqueEdge(
	edges []layered.LEdgeID,
	edge layered.LEdgeID,
) []layered.LEdgeID {
	for _, existing := range edges {
		if existing == edge {
			return edges
		}
	}
	return append(edges, edge)
}

func removeCompactionNode(
	nodes []*componentCompactionNode,
	wanted *componentCompactionNode,
) []*componentCompactionNode {
	for index, node := range nodes {
		if node == wanted {
			copy(nodes[index:], nodes[index+1:])
			return nodes[:len(nodes)-1]
		}
	}
	return nodes
}

func removeCompactionGroup(
	groups []*componentCompactionGroup,
	wanted *componentCompactionGroup,
) []*componentCompactionGroup {
	for index, group := range groups {
		if group == wanted {
			copy(groups[index:], groups[index+1:])
			return groups[:len(groups)-1]
		}
	}
	return groups
}
