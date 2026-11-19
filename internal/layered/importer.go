// Copyright (c) 2015, 2020 Kiel University and others.
// Copyright (c) 2016, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"fmt"

	"github.com/d2lang/elk-go/internal/common/nodespacing"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
)

// ImportMaps preserve object identity between ELK's public graph arena and
// its layered working graph.
type ImportMaps struct {
	Nodes  map[basegraph.NodeID]LNodeID
	Ports  map[basegraph.PortID]LPortID
	Edges  map[basegraph.EdgeID]LEdgeID
	Labels map[basegraph.LabelID]LLabelID
	Graphs map[basegraph.NodeID]LGraphID
}

type ElkGraphImporter struct {
	Model *basegraph.Model
	Maps  ImportMaps
}

func NewElkGraphImporter(model *basegraph.Model) *ElkGraphImporter {
	return &ElkGraphImporter{Model: model, Maps: ImportMaps{
		Nodes: make(map[basegraph.NodeID]LNodeID), Ports: make(map[basegraph.PortID]LPortID),
		Edges: make(map[basegraph.EdgeID]LEdgeID), Labels: make(map[basegraph.LabelID]LLabelID),
		Graphs: make(map[basegraph.NodeID]LGraphID),
	}}
}

func (importer *ElkGraphImporter) ImportGraph(parent basegraph.NodeID) (*LGraphArena, LGraphID, error) {
	if importer.Model == nil {
		return nil, 0, fmt.Errorf("layered: cannot import a nil graph")
	}
	arena := NewLGraphArena()
	topGraph := importer.createGraph(arena, parent)
	importer.Maps.Graphs[parent] = topGraph
	if basegraph.Get(&arena.Graph(topGraph).Properties, core.PartitioningActivate) {
		importer.addGraphProperty(arena, topGraph, GraphPropertyPartitions)
	}
	if err := importer.importExternalPorts(arena, parent, topGraph); err != nil {
		return nil, 0, err
	}

	hierarchical := baseHierarchy(importer.Model.Node(parent)) == HierarchyIncludeChildren
	if err := importer.importNodes(arena, parent, topGraph, hierarchical); err != nil {
		return nil, 0, err
	}
	if err := importer.importEdges(arena, parent, topGraph, hierarchical); err != nil {
		return nil, 0, err
	}
	if err := importer.importInsideSelfLoops(arena, parent, topGraph); err != nil {
		return nil, 0, err
	}
	if baseValue, found := basegraph.TryGet(&arena.Graph(topGraph).Properties, SpacingBaseValue); found {
		ApplySpacingBaseValue(&arena.Graph(topGraph).Properties, baseValue)
	}
	return arena, topGraph, nil
}

// importExternalPorts mirrors ElkGraphImporter's top-level external-port
// transformation. A recursive layout of a compound node represents the
// compound node's own ports as boundary dummies in the graph being laid out.
func (importer *ElkGraphImporter) importExternalPorts(
	arena *LGraphArena,
	parent basegraph.NodeID,
	topGraph LGraphID,
) error {
	parentNode := importer.Model.Node(parent)
	if len(parentNode.Ports) == 0 {
		return nil
	}
	for _, portID := range parentNode.Ports {
		importer.ensureDefinedExternalPortSide(arena, topGraph, portID)
	}

	hasExternalPorts, hasHyperedges := false, false
	insideLabels := stringSetContains(
		basegraph.Get(&parentNode.Properties, nodespacing.PortLabelsPlacement), "INSIDE")
	for _, portID := range parentNode.Ports {
		edgeCount := importer.externalPortEdgeCount(parent, portID)
		if edgeCount > 0 || insideLabels && len(importer.Model.Port(portID).Labels) > 0 {
			hasExternalPorts = true
		}
		if edgeCount > 1 {
			hasHyperedges = true
		}
		if hasExternalPorts && hasHyperedges {
			break
		}
	}
	if !hasExternalPorts {
		return nil
	}
	importer.addGraphProperty(arena, topGraph, GraphPropertyExternalPorts)
	if hasHyperedges {
		importer.addGraphProperty(arena, topGraph, GraphPropertyHyperedges)
	}
	for _, portID := range parentNode.Ports {
		if err := importer.transformExternalPort(arena, parent, portID, topGraph); err != nil {
			return err
		}
	}
	return nil
}

func (importer *ElkGraphImporter) ensureDefinedExternalPortSide(
	arena *LGraphArena,
	graphID LGraphID,
	portID basegraph.PortID,
) {
	port := importer.Model.Port(portID)
	direction := basegraph.Get(&arena.Graph(graphID).Properties, DirectionOption)
	constraints := basegraph.Get(&arena.Graph(graphID).Properties, PortConstraintsOption)
	side := basegraph.Get(&port.Properties, PortSideOption)
	if !constraints.IsSideFixed() {
		side = DirectionPortSide(direction)
		if importer.calculateExternalPortNetFlow(portID) <= 0 {
			side = side.Opposed()
		}
	} else if side == UndefinedSide {
		node := importer.Model.Node(port.Parent)
		side = inferPortSide(
			geometry.Point{X: node.Shape.Width, Y: node.Shape.Height},
			port.Shape.Position(),
			geometry.Point{X: port.Shape.Width, Y: port.Shape.Height},
			direction,
		)
		if side == UndefinedSide {
			side = DirectionPortSide(direction)
		}
	}
	basegraph.Set(&port.Properties, PortSideOption, side)
}

func (importer *ElkGraphImporter) externalPortEdgeCount(
	parent basegraph.NodeID,
	portID basegraph.PortID,
) int {
	port := importer.Model.Port(portID)
	count := 0
	seen := make(map[basegraph.EdgeID]bool)
	consider := func(edgeID basegraph.EdgeID) {
		if seen[edgeID] {
			return
		}
		seen[edgeID] = true
		edge := importer.Model.Edge(edgeID)
		if importer.isInsideSelfLoop(parent, edgeID) {
			count++
			return
		}
		if len(edge.Sources) == 0 || len(edge.Targets) == 0 {
			return
		}
		var opposite basegraph.NodeID
		if shapeListContainsPort(edge.Sources, portID) {
			opposite = importer.Model.ShapeNode(edge.Targets[0])
		} else {
			opposite = importer.Model.ShapeNode(edge.Sources[0])
		}
		if oppositeNode := importer.Model.Node(opposite); oppositeNode.Parent != nil && *oppositeNode.Parent == parent {
			count++
		}
	}
	for _, edgeID := range port.OutgoingEdges {
		consider(edgeID)
	}
	for _, edgeID := range port.IncomingEdges {
		consider(edgeID)
	}
	return count
}

func (importer *ElkGraphImporter) transformExternalPort(
	arena *LGraphArena,
	parent basegraph.NodeID,
	portID basegraph.PortID,
	graphID LGraphID,
) error {
	original := importer.Model.Port(portID)
	parentNode := importer.Model.Node(parent)
	portSize := geometry.Point{X: original.Shape.Width, Y: original.Shape.Height}
	portPosition := original.Shape.Position().Add(portSize.Scale(.5))
	constraints := basegraph.Get(&arena.Graph(graphID).Properties, PortConstraintsOption)
	if constraints == PortConstraintsUndefined {
		constraints = PortConstraintsFree
	}
	side := basegraph.Get(&original.Properties, PortSideOption)
	if side == UndefinedSide {
		return fmt.Errorf("layered: external port %s has no defined side", original.Identifier)
	}
	if !basegraph.Has(&original.Properties, PortBorderOffset) {
		offset := 0.0
		if original.Shape.X != 0 || original.Shape.Y != 0 {
			offset = externalPortBorderOffset(parentNode, original, side)
		}
		basegraph.Set(&original.Properties, PortBorderOffset, offset)
	}

	parentSize := geometry.Point{X: parentNode.Shape.Width, Y: parentNode.Shape.Height}
	dummyID := arena.CreateExternalPortDummy(
		&original.Properties,
		constraints,
		side,
		importer.calculateExternalPortNetFlow(portID),
		&parentSize,
		&portPosition,
		portSize,
		basegraph.Get(&arena.Graph(graphID).Properties, DirectionOption),
		graphID,
	)
	basegraph.Set(&arena.Node(dummyID).Properties, OriginalElement, PortOrigin(portID))
	basegraph.Set(&arena.Node(dummyID).Properties, nodespacing.PortLabelsPlacement, []string{"OUTSIDE"})
	dummyPortID := arena.Node(dummyID).Ports[0]
	arena.Port(dummyPortID).ConnectedToExternalNodes = importer.isConnectedToExternalNodes(portID)

	insideLabels := stringSetContains(
		basegraph.Get(&parentNode.Properties, nodespacing.PortLabelsPlacement), "INSIDE")
	fixedLabels := fixedPortLabelPlacement(&parentNode.Properties)
	for _, labelID := range original.Labels {
		label := importer.Model.Label(labelID)
		if label.Text == "" || basegraph.Get(&label.Properties, core.NoLayout) {
			continue
		}
		llabelID := importer.transformLabel(arena, labelID)
		arena.Port(dummyPortID).Labels = append(arena.Port(dummyPortID).Labels, llabelID)
		if !insideLabels {
			insidePart := 0.0
			if fixedLabels {
				insidePart = portLabelInsidePart(label, original, side)
			}
			switch side {
			case EastSide, WestSide:
				arena.Label(llabelID).Size.X = insidePart
			case NorthSide, SouthSide:
				arena.Label(llabelID).Size.Y = insidePart
			}
		}
	}

	outerProperties := &basegraph.PropertyMap{}
	if parentNode.Parent != nil {
		outerProperties = &importer.Model.Node(*parentNode.Parent).Properties
	}
	basegraph.Set(&arena.Node(dummyID).Properties, SpacingLabelPortHorizontal,
		basegraph.Get(outerProperties, SpacingLabelPortHorizontal))
	basegraph.Set(&arena.Node(dummyID).Properties, SpacingLabelPortVertical,
		basegraph.Get(outerProperties, SpacingLabelPortVertical))
	basegraph.Set(&arena.Node(dummyID).Properties, SpacingLabelLabel,
		basegraph.Get(outerProperties, SpacingLabelLabel))
	arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, dummyID)
	importer.Maps.Ports[portID] = dummyPortID
	return nil
}

func (importer *ElkGraphImporter) calculateExternalPortNetFlow(portID basegraph.PortID) int {
	port := importer.Model.Port(portID)
	parent := port.Parent
	outputVotes, inputVotes := 0, 0
	for _, edgeID := range port.OutgoingEdges {
		edge := importer.Model.Edge(edgeID)
		if len(edge.Targets) == 0 {
			continue
		}
		selfLoop := core.IsSelfLoop(importer.Model, edgeID)
		inside := importer.isInsideSelfLoop(parent, edgeID)
		target := importer.Model.ShapeNode(edge.Targets[0])
		switch {
		case selfLoop && inside:
			inputVotes++
		case selfLoop:
			outputVotes++
		case importer.Model.Node(target).Parent != nil && *importer.Model.Node(target).Parent == parent || target == parent:
			inputVotes++
		default:
			outputVotes++
		}
	}
	for _, edgeID := range port.IncomingEdges {
		edge := importer.Model.Edge(edgeID)
		if len(edge.Sources) == 0 {
			continue
		}
		selfLoop := core.IsSelfLoop(importer.Model, edgeID)
		inside := importer.isInsideSelfLoop(parent, edgeID)
		source := importer.Model.ShapeNode(edge.Sources[0])
		switch {
		case selfLoop && inside:
			outputVotes++
		case selfLoop:
			inputVotes++
		case importer.Model.Node(source).Parent != nil && *importer.Model.Node(source).Parent == parent || source == parent:
			outputVotes++
		default:
			inputVotes++
		}
	}
	return outputVotes - inputVotes
}

func (importer *ElkGraphImporter) isConnectedToExternalNodes(portID basegraph.PortID) bool {
	port := importer.Model.Port(portID)
	parent := port.Parent
	for _, edgeID := range port.OutgoingEdges {
		edge := importer.Model.Edge(edgeID)
		if len(edge.Targets) > 0 && !importer.Model.IsDescendant(importer.Model.ShapeNode(edge.Targets[0]), parent) {
			return true
		}
	}
	for _, edgeID := range port.IncomingEdges {
		edge := importer.Model.Edge(edgeID)
		if len(edge.Sources) > 0 && !importer.Model.IsDescendant(importer.Model.ShapeNode(edge.Sources[0]), parent) {
			return true
		}
	}
	return false
}

func externalPortBorderOffset(node *basegraph.Node, port *basegraph.Port, side PortSide) float64 {
	switch side {
	case NorthSide:
		return -(port.Shape.Y + port.Shape.Height)
	case EastSide:
		return port.Shape.X - node.Shape.Width
	case SouthSide:
		return port.Shape.Y - node.Shape.Height
	case WestSide:
		return -(port.Shape.X + port.Shape.Width)
	default:
		return 0
	}
}

func portLabelInsidePart(label *basegraph.Label, port *basegraph.Port, side PortSide) float64 {
	switch side {
	case NorthSide:
		return max(0, label.Shape.Height+label.Shape.Y-port.Shape.Height)
	case SouthSide:
		return max(0, -label.Shape.Y)
	case EastSide:
		return max(0, -label.Shape.X)
	case WestSide:
		return max(0, label.Shape.Width+label.Shape.X-port.Shape.Width)
	default:
		return 0
	}
}

func shapeListContainsPort(shapes []basegraph.ShapeID, portID basegraph.PortID) bool {
	for _, shape := range shapes {
		if shape.Kind == basegraph.PortShape && shape.Port == portID {
			return true
		}
	}
	return false
}

func stringSetContains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func (importer *ElkGraphImporter) createGraph(arena *LGraphArena, origin basegraph.NodeID) LGraphID {
	graphID := arena.AddGraph()
	graph := arena.Graph(graphID)
	copyTypedProperties(&graph.Properties, &importer.Model.Node(origin).Properties)
	basegraph.Set(&graph.Properties, OriginalElement, NodeOrigin(origin))
	basegraph.Set(&graph.Properties, GraphProperties, GraphProperty(0))
	graph.Padding = basegraph.Get(&graph.Properties, Padding)
	if basegraph.Get(&graph.Properties, DirectionOption) == DirectionUndefined {
		basegraph.Set(&graph.Properties, DirectionOption, DirectionRight)
	}
	return graphID
}

func (importer *ElkGraphImporter) importNodes(
	arena *LGraphArena,
	parent basegraph.NodeID,
	topGraph LGraphID,
	hierarchical bool,
) error {
	if !hierarchical {
		modelOrder := 0
		for _, nodeID := range importer.Model.Node(parent).Children {
			if basegraph.Get(&importer.Model.Node(nodeID).Properties, core.NoLayout) {
				continue
			}
			lnode := importer.transformNode(arena, nodeID, topGraph)
			if needsModelOrder(arena.Graph(topGraph)) &&
				!basegraph.Get(&importer.Model.Node(nodeID).Properties, NoModelOrder) {
				basegraph.Set(&arena.Node(lnode).Properties, ModelOrder, modelOrder)
				modelOrder++
			}
		}
		basegraph.Set(&arena.Graph(topGraph).Properties, MaximumModelOrderNodes, modelOrder)
		return nil
	}

	direction := basegraph.Get(&arena.Graph(topGraph).Properties, DirectionOption)
	queue := append([]basegraph.NodeID(nil), importer.Model.Node(parent).Children...)
	modelOrder := 0
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		node := importer.Model.Node(nodeID)
		if basegraph.Get(&node.Properties, core.NoLayout) {
			continue
		}
		parentGraph := topGraph
		if node.Parent != nil && *node.Parent != parent {
			mappedParent, found := importer.Maps.Nodes[*node.Parent]
			if !found || arena.Node(mappedParent).NestedGraph == nil {
				return fmt.Errorf("layered: parent of node %s has no nested graph", node.Identifier)
			}
			parentGraph = *arena.Node(mappedParent).NestedGraph
		}

		var nested *LGraphID
		if len(node.Children) > 0 && baseHierarchy(node) == HierarchyIncludeChildren {
			nestedID := importer.createGraph(arena, nodeID)
			basegraph.Set(&arena.Graph(nestedID).Properties, DirectionOption, direction)
			importer.Maps.Graphs[nodeID] = nestedID
			nested = &nestedID
			if baseValue, found := basegraph.TryGet(&arena.Graph(nestedID).Properties, SpacingBaseValue); found {
				ApplySpacingBaseValue(&arena.Graph(nestedID).Properties, baseValue)
			}
		}

		lnode := importer.transformNode(arena, nodeID, parentGraph)
		if needsModelOrder(arena.Graph(parentGraph)) &&
			!basegraph.Get(&node.Properties, NoModelOrder) {
			basegraph.Set(&arena.Node(lnode).Properties, ModelOrder, modelOrder)
			modelOrder++
		}
		if nested != nil {
			arena.Node(lnode).NestedGraph = nested
			arena.Graph(*nested).ParentNode = &lnode
			queue = append(queue, node.Children...)
		}
	}
	basegraph.Set(&arena.Graph(topGraph).Properties, MaximumModelOrderNodes, modelOrder)
	return nil
}

func (importer *ElkGraphImporter) transformNode(
	arena *LGraphArena,
	nodeID basegraph.NodeID,
	graphID LGraphID,
) LNodeID {
	original := importer.Model.Node(nodeID)
	lnodeID := arena.AddNode(graphID)
	lnode := arena.Node(lnodeID)
	copyTypedProperties(&lnode.Properties, &original.Properties)
	copyIndividualSpacings(lnode, original)
	basegraph.Set(&lnode.Properties, OriginalElement, NodeOrigin(nodeID))
	lnode.Position = original.Shape.Position()
	lnode.Size = geometry.Point{X: original.Shape.Width, Y: original.Shape.Height}
	lnode.Margin = basegraph.Get(&original.Properties, nodespacing.Margins)
	arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, lnodeID)
	importer.Maps.Nodes[nodeID] = lnodeID

	if len(original.Children) > 0 || basegraph.Get(&original.Properties, core.InsideSelfLoopsActivate) {
		basegraph.Set(&lnode.Properties, CompoundNode, true)
	}
	constraints := basegraph.Get(&lnode.Properties, PortConstraintsOption)
	if constraints == PortConstraintsUndefined {
		constraints = PortConstraintsFree
		basegraph.Set(&lnode.Properties, PortConstraintsOption, constraints)
	} else if constraints != PortConstraintsFree {
		importer.addGraphProperty(arena, graphID, GraphPropertyNonFreePorts)
	}
	direction := basegraph.Get(&arena.Graph(graphID).Properties, DirectionOption)
	assignPortModelOrder := needsModelOrder(arena.Graph(graphID)) &&
		!basegraph.Get(&original.Properties, NoModelOrder)
	for portModelOrder, portID := range original.Ports {
		if basegraph.Get(&importer.Model.Port(portID).Properties, core.NoLayout) {
			continue
		}
		lportID := importer.transformPort(arena, portID, lnodeID, graphID, direction, constraints)
		if assignPortModelOrder {
			basegraph.Set(&arena.Port(lportID).Properties, ModelOrder, portModelOrder)
		}
	}
	for _, labelID := range original.Labels {
		label := importer.Model.Label(labelID)
		if label.Text == "" || basegraph.Get(&label.Properties, core.NoLayout) {
			continue
		}
		lnode.Labels = append(lnode.Labels, importer.transformLabel(arena, labelID))
	}
	if basegraph.Get(&lnode.Properties, core.CommentBox) {
		importer.addGraphProperty(arena, graphID, GraphPropertyComments)
	}
	if basegraph.Get(&lnode.Properties, core.Hypernode) {
		importer.addGraphProperty(arena, graphID, GraphPropertyHypernodes)
		importer.addGraphProperty(arena, graphID, GraphPropertyHyperedges)
		basegraph.Set(&lnode.Properties, PortConstraintsOption, PortConstraintsFree)
	}
	return lnodeID
}

// LayoutConfigurator stores parent-targeted spacing options configured on a
// node in CoreOptions.SPACING_INDIVIDUAL. Direct spacing properties on the
// node are not individual overrides; they participate in normal option
// inheritance and must not be wrapped here.
func copyIndividualSpacings(target *LNode, source *basegraph.Node) {
	var individual IndividualSpacings
	if raw, found := source.Properties.Raw(core.SpacingIndividual.ID); found {
		switch typed := raw.(type) {
		case *core.IndividualSpacings:
			if typed != nil {
				individual.Properties.CopyFrom(&typed.Properties)
			}
		case core.IndividualSpacings:
			individual.Properties.CopyFrom(&typed.Properties)
		case IndividualSpacings:
			individual.Properties.CopyFrom(&typed.Properties)
		}
	}
	if !individual.Properties.Empty() {
		basegraph.Set(&target.Properties, SpacingIndividual, individual)
	}
}

func (importer *ElkGraphImporter) transformPort(
	arena *LGraphArena,
	portID basegraph.PortID,
	nodeID LNodeID,
	graphID LGraphID,
	direction Direction,
	constraints PortConstraints,
) LPortID {
	original := importer.Model.Port(portID)
	lportID := arena.AddPort()
	lport := arena.Port(lportID)
	copyTypedProperties(&lport.Properties, &original.Properties)
	basegraph.Set(&lport.Properties, OriginalElement, PortOrigin(portID))
	lport.Position = original.Shape.Position()
	lport.Size = geometry.Point{X: original.Shape.Width, Y: original.Shape.Height}
	arena.SetPortNode(lportID, &nodeID)
	side := basegraph.Get(&lport.Properties, PortSideOption)
	if side == UndefinedSide {
		if constraints.IsSideFixed() {
			side = inferPortSide(arena.Node(nodeID).Size, lport.Position, lport.Size, direction)
		} else if len(original.IncomingEdges)-len(original.OutgoingEdges) > 0 {
			side = DirectionPortSide(direction)
		} else {
			side = DirectionPortSide(direction).Opposed()
		}
	}
	arena.SetPortSide(lportID, side)
	for _, labelID := range original.Labels {
		label := importer.Model.Label(labelID)
		if label.Text == "" || basegraph.Get(&label.Properties, core.NoLayout) {
			continue
		}
		lport.Labels = append(lport.Labels, importer.transformLabel(arena, labelID))
	}
	if (direction == DirectionRight || direction == DirectionLeft) && (side == NorthSide || side == SouthSide) ||
		(direction == DirectionUp || direction == DirectionDown) && (side == EastSide || side == WestSide) {
		importer.addGraphProperty(arena, graphID, GraphPropertyNorthSouthPorts)
	}
	importer.Maps.Ports[portID] = lportID
	return lportID
}

func (importer *ElkGraphImporter) transformLabel(arena *LGraphArena, labelID basegraph.LabelID) LLabelID {
	original := importer.Model.Label(labelID)
	llabelID := arena.AddLabel(original.Text)
	llabel := arena.Label(llabelID)
	copyTypedProperties(&llabel.Properties, &original.Properties)
	basegraph.Set(&llabel.Properties, OriginalElement, LabelOrigin(labelID))
	llabel.Position = original.Shape.Position()
	llabel.Size = geometry.Point{X: original.Shape.Width, Y: original.Shape.Height}
	importer.Maps.Labels[labelID] = llabelID
	return llabelID
}

func (importer *ElkGraphImporter) importEdges(
	arena *LGraphArena,
	parent basegraph.NodeID,
	topGraph LGraphID,
	hierarchical bool,
) error {
	containers := []basegraph.NodeID{parent}
	if hierarchical {
		// importHierarchicalGraph visits contained-edge lists breadth first,
		// independently of the order in the global edge arena. This is visible
		// through MODEL_ORDER when a later JSON edge is contained by a shallower
		// graph than an earlier one.
		for index := 0; index < len(containers); index++ {
			for _, child := range importer.Model.Node(containers[index]).Children {
				if _, partOfRun := importer.Maps.Graphs[child]; partOfRun {
					containers = append(containers, child)
				}
			}
		}
	}

	modelOrder := 0
	for _, container := range containers {
		for _, edgeID := range append([]basegraph.EdgeID(nil), importer.Model.Node(container).ContainedEdges...) {
			edge := importer.Model.Edge(edgeID)
			if needsModelOrder(arena.Graph(topGraph)) {
				basegraph.Set(&edge.Properties, ModelOrder, modelOrder)
				modelOrder++
			}
			if len(edge.Sources) != 1 || len(edge.Targets) != 1 {
				return fmt.Errorf("layered: edge %s is a hyperedge; hyperedges are unsupported", edge.Identifier)
			}
			if basegraph.Get(&edge.Properties, core.NoLayout) {
				continue
			}
			// Inside self loops are laid out recursively in the source node's
			// own graph, not in the graph that contains the source node.
			if importer.isInsideSelfLoop(importer.Model.ShapeNode(edge.Sources[0]), edgeID) {
				continue
			}
			sourceNode := importer.Model.ShapeNode(edge.Sources[0])
			targetNode := importer.Model.ShapeNode(edge.Targets[0])
			if _, sourceFound := importer.mappedEndpointNode(arena, edge.Sources[0]); !sourceFound {
				continue
			}
			if _, targetFound := importer.mappedEndpointNode(arena, edge.Targets[0]); !targetFound {
				continue
			}
			graphID := importer.edgeGraph(arena, topGraph, parent, container, sourceNode, targetNode)
			ledgeID, err := importer.transformEdge(arena, edgeID, graphID)
			if err != nil {
				return err
			}
			if needsModelOrder(arena.Graph(topGraph)) {
				basegraph.Set(&arena.Edge(ledgeID).Properties, ModelOrder,
					basegraph.Get(&edge.Properties, ModelOrder))
			}
			if sourceNode != targetNode && importer.Model.Node(sourceNode).Parent != importer.Model.Node(targetNode).Parent {
				if origin, found := importer.Maps.Graphs[container]; found {
					basegraph.Set(&arena.Edge(ledgeID).Properties, CoordinateSystemOrigin, origin)
				}
			}
		}
	}
	return nil
}

func (importer *ElkGraphImporter) importInsideSelfLoops(
	arena *LGraphArena,
	parent basegraph.NodeID,
	topGraph LGraphID,
) error {
	parentNode := importer.Model.Node(parent)
	if parentNode.Parent == nil {
		return nil
	}
	for _, edgeID := range importer.Model.Node(*parentNode.Parent).ContainedEdges {
		if !importer.isInsideSelfLoop(parent, edgeID) {
			continue
		}
		if _, alreadyImported := importer.Maps.Edges[edgeID]; alreadyImported {
			continue
		}
		if _, err := importer.transformEdge(arena, edgeID, topGraph); err != nil {
			return err
		}
	}
	return nil
}

func (importer *ElkGraphImporter) isInsideSelfLoop(
	nodeID basegraph.NodeID,
	edgeID basegraph.EdgeID,
) bool {
	edge := importer.Model.Edge(edgeID)
	return basegraph.Get(&importer.Model.Node(nodeID).Properties, core.InsideSelfLoopsActivate) &&
		basegraph.Get(&edge.Properties, core.InsideSelfLoopsEdge) &&
		core.IsSelfLoop(importer.Model, edgeID) &&
		len(edge.Sources) > 0 && importer.Model.ShapeNode(edge.Sources[0]) == nodeID
}

func (importer *ElkGraphImporter) edgeGraph(
	arena *LGraphArena,
	topGraph LGraphID,
	root, containing, source, target basegraph.NodeID,
) LGraphID {
	if importer.Model.IsDescendant(target, source) {
		if graphID, found := importer.Maps.Graphs[source]; found {
			return graphID
		}
	}
	if importer.Model.IsDescendant(source, target) {
		if graphID, found := importer.Maps.Graphs[target]; found {
			return graphID
		}
	}
	if containing == root {
		return topGraph
	}
	if graphID, found := importer.Maps.Graphs[containing]; found {
		return graphID
	}
	if lnode, found := importer.Maps.Nodes[containing]; found {
		return arena.NodeGraph(lnode)
	}
	return topGraph
}

func (importer *ElkGraphImporter) transformEdge(
	arena *LGraphArena,
	edgeID basegraph.EdgeID,
	graphID LGraphID,
) (LEdgeID, error) {
	original := importer.Model.Edge(edgeID)
	if len(original.Sources) != 1 || len(original.Targets) != 1 {
		return 0, fmt.Errorf("layered: edge %s is a hyperedge; hyperedges are unsupported", original.Identifier)
	}
	sourceNodeID := importer.Model.ShapeNode(original.Sources[0])
	targetNodeID := importer.Model.ShapeNode(original.Targets[0])
	sourceNode, sourceFound := importer.mappedEndpointNode(arena, original.Sources[0])
	targetNode, targetFound := importer.mappedEndpointNode(arena, original.Targets[0])
	if !sourceFound || !targetFound {
		return 0, fmt.Errorf("layered: source or target of edge %s could not be transformed", original.Identifier)
	}
	sourcePort, found := importer.mappedPort(original.Sources[0])
	if !found {
		sourcePort = importer.createDefaultPort(arena, graphID, sourceNode, PortTypeOutput)
	}
	targetPort, found := importer.mappedPort(original.Targets[0])
	if !found {
		targetPort = importer.createDefaultPort(arena, arena.NodeGraph(targetNode), targetNode, PortTypeInput)
	}

	ledgeID := arena.AddEdge()
	ledge := arena.Edge(ledgeID)
	copyTypedProperties(&ledge.Properties, &original.Properties)
	basegraph.Set(&ledge.Properties, OriginalElement, EdgeOrigin(edgeID))
	basegraph.Unset(&ledge.Properties, JunctionPoints)
	arena.SetEdgeSource(ledgeID, &sourcePort)
	arena.SetEdgeTarget(ledgeID, &targetPort)
	importer.Maps.Edges[edgeID] = ledgeID

	if sourceNodeID == targetNodeID {
		importer.addGraphProperty(arena, graphID, GraphPropertySelfLoops)
	}
	if len(arena.Port(sourcePort).IncomingEdges) > 1 || len(arena.Port(sourcePort).OutgoingEdges) > 1 ||
		len(arena.Port(targetPort).IncomingEdges) > 1 || len(arena.Port(targetPort).OutgoingEdges) > 1 {
		importer.addGraphProperty(arena, graphID, GraphPropertyHyperedges)
	}
	for _, labelID := range original.Labels {
		label := importer.Model.Label(labelID)
		if label.Text == "" || basegraph.Get(&label.Properties, core.NoLayout) {
			continue
		}
		llabelID := importer.transformLabel(arena, labelID)
		ledge.Labels = append(ledge.Labels, llabelID)
		switch basegraph.Get(&arena.Label(llabelID).Properties, EdgeLabelsPlacement) {
		case EdgeLabelPlacementHead, EdgeLabelPlacementTail:
			importer.addGraphProperty(arena, graphID, GraphPropertyEndLabels)
		case EdgeLabelPlacementCenter:
			importer.addGraphProperty(arena, graphID, GraphPropertyCenterLabels)
		}
	}
	if (basegraph.Get(&arena.Graph(graphID).Properties, CrossingMinimization) == CrossingMinimizationInteractive ||
		basegraph.Get(&arena.Graph(graphID).Properties, NodePlacement) == NodePlacementInteractive) &&
		len(original.Sections) > 0 {
		section := importer.Model.Section(original.Sections[0])
		if len(section.BendPoints) > 0 {
			basegraph.Set(&ledge.Properties, OriginalBendPoints, section.BendPoints.Clone())
		}
	}
	return ledgeID, nil
}

func (importer *ElkGraphImporter) mappedEndpointNode(
	arena *LGraphArena,
	shape basegraph.ShapeID,
) (LNodeID, bool) {
	if shape.Kind == basegraph.PortShape {
		if portID, found := importer.Maps.Ports[shape.Port]; found && arena.Port(portID).Node != nil {
			return *arena.Port(portID).Node, true
		}
	}
	nodeID, found := importer.Maps.Nodes[importer.Model.ShapeNode(shape)]
	return nodeID, found
}

func (importer *ElkGraphImporter) mappedPort(shape basegraph.ShapeID) (LPortID, bool) {
	if shape.Kind != basegraph.PortShape {
		return 0, false
	}
	port, found := importer.Maps.Ports[shape.Port]
	return port, found
}

func (importer *ElkGraphImporter) createDefaultPort(
	arena *LGraphArena,
	graphID LGraphID,
	nodeID LNodeID,
	portType PortType,
) LPortID {
	direction := basegraph.Get(&arena.Graph(graphID).Properties, DirectionOption)
	side := DirectionPortSide(direction)
	if portType == PortTypeInput {
		side = side.Opposed()
	}
	if basegraph.Get(&arena.Graph(graphID).Properties, MergeEdges) &&
		!basegraph.Get(&arena.Node(nodeID).Properties, PortConstraintsOption).IsSideFixed() {
		return arena.ProvideCollectorPort(graphID, nodeID, portType, side)
	}
	portID := arena.AddPort()
	arena.SetPortNode(portID, &nodeID)
	arena.SetPortSide(portID, side)
	return portID
}

func (importer *ElkGraphImporter) addGraphProperty(arena *LGraphArena, graphID LGraphID, property GraphProperty) {
	properties := basegraph.Get(&arena.Graph(graphID).Properties, GraphProperties)
	basegraph.Set(&arena.Graph(graphID).Properties, GraphProperties, properties|property)
}

func needsModelOrder(graph *LGraph) bool {
	cycle := basegraph.Get(&graph.Properties, CycleBreaking)
	layering := basegraph.Get(&graph.Properties, Layering)
	promotion := basegraph.Get(&graph.Properties, NodePromotionStrategyOption)
	return basegraph.Get(&graph.Properties, ConsiderModelOrder) != OrderingNone ||
		cycle == CycleBreakingModelOrder || cycle == CycleBreakingGreedyModelOrder ||
		cycle == CycleBreakingSCCConnectivity || cycle == CycleBreakingSCCNodeType ||
		cycle == CycleBreakingDFSNodeOrder || cycle == CycleBreakingBFSNodeOrder ||
		layering == LayeringBreadthFirstModelOrder || layering == LayeringDepthFirstModelOrder ||
		promotion == NodePromotionModelOrderLeftToRight ||
		promotion == NodePromotionModelOrderRightToLeft ||
		basegraph.Get(&graph.Properties, ForceNodeModelOrder) ||
		basegraph.Get(&graph.Properties, ComponentOrdering) != ComponentOrderingNone ||
		basegraph.Get(&graph.Properties, CrossingCounterNodeInfluence) != 0 ||
		basegraph.Get(&graph.Properties, CrossingCounterPortInfluence) != 0
}

func baseHierarchy(node *basegraph.Node) HierarchyHandling {
	value, _ := node.Properties.Raw(core.HierarchyHandling.ID)
	if hierarchy, found := parseHierarchyHandling(fmt.Sprint(value)); found {
		return hierarchy
	}
	return HierarchySeparateChildren
}

func inferPortSide(nodeSize, position, size geometry.Point, direction Direction) PortSide {
	center := position.Add(size.Scale(0.5))
	distances := []struct {
		side  PortSide
		value float64
	}{
		{NorthSide, center.Y},
		{EastSide, nodeSize.X - center.X},
		{SouthSide, nodeSize.Y - center.Y},
		{WestSide, center.X},
	}
	best := distances[0]
	for _, candidate := range distances[1:] {
		if candidate.value < best.value {
			best = candidate
		}
	}
	if best.value < 0 {
		return DirectionPortSide(direction)
	}
	return best.side
}
