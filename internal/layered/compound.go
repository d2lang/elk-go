// Copyright (c) 2013, 2015 Kiel University and others.
// Copyright (c) 2013, 2020 Kiel University and others.
// Copyright (c) 2014, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"fmt"
	"math"
	"sort"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
)

// CrossHierarchyEdge is one per-hierarchy-level replacement segment of an
// original edge. The declaration mirrors CompoundGraphPreprocessor's private
// CrossHierarchyEdge value.
type CrossHierarchyEdge struct {
	Edge     LEdgeID
	Graph    LGraphID
	PortType PortType
}

// CrossHierarchyEntry retains Java LinkedHashMap insertion order.
type CrossHierarchyEntry struct {
	Original LEdgeID
	Segments []CrossHierarchyEdge
}

// CrossHierarchyMap is the exact non-cloneable value stored on the top graph.
// Entries, rather than a Go map, are authoritative because insertion order is
// observable while labels and dummy edges are restored.
type CrossHierarchyMap struct {
	Entries []CrossHierarchyEntry
	index   map[LEdgeID]int
}

func (crossings *CrossHierarchyMap) add(original LEdgeID, segment CrossHierarchyEdge) {
	if crossings.index == nil {
		crossings.index = make(map[LEdgeID]int)
	}
	index, found := crossings.index[original]
	if !found {
		index = len(crossings.Entries)
		crossings.index[original] = index
		crossings.Entries = append(crossings.Entries, CrossHierarchyEntry{Original: original})
	}
	crossings.Entries[index].Segments = append(crossings.Entries[index].Segments, segment)
}

// CompoundGraphPreprocessor splits every cross-hierarchy edge into one edge
// per hierarchy level, connected through external-port dummy nodes.
type CompoundGraphPreprocessor struct{}

func (*CompoundGraphPreprocessor) Process(arena *LGraphArena, topGraph LGraphID) error {
	if arena == nil || topGraph.Index() >= len(arena.Graphs) {
		return fmt.Errorf("layered: invalid graph passed to compound preprocessor")
	}
	crossings := &CrossHierarchyMap{}
	originalEdgeCount := len(arena.Edges)
	for index := 0; index < originalEdgeCount; index++ {
		edgeID := LEdgeID(index)
		edge := arena.Edge(edgeID)
		if edge.Source == nil || edge.Target == nil {
			continue
		}
		sourceGraph := arena.NodeGraph(arena.EdgeSourceNode(edgeID))
		targetGraph := arena.NodeGraph(arena.EdgeTargetNode(edgeID))
		if sourceGraph == targetGraph {
			continue
		}
		if err := splitCrossHierarchyEdge(arena, topGraph, edgeID, crossings); err != nil {
			return err
		}
	}
	// Once one port of a compound node crosses its hierarchy boundary, ELK
	// represents every port of that node inside the child graph. The additional
	// unconnected dummies are observable: phase 3 orders them together and the
	// hierarchical node resizer transfers all of their positions back.
	ensureCompoundPortDummies(arena, topGraph)
	// The Java implementation creates hierarchy dummies during a recursive,
	// post-order walk. Their insertion order becomes the initial order used by
	// crossing minimization, so recover that source-shaped order after the
	// per-edge splitting above.
	reorderCompoundPortDummies(arena, topGraph)
	basegraph.Set(&arena.Graph(topGraph).Properties, CrossHierarchyEdges, crossings)
	return nil
}

type compoundPortDummyOrder struct {
	node       LNodeID
	connected  bool
	path       []int
	portIndex  int
	edgeOrder  int
	stableRank int
}

// reorderCompoundPortDummies reproduces transformHierarchyEdges' observable
// post-order insertion order. Descendant boundary ports precede ports on the
// compound node itself; siblings retain node, port, and edge model order.
func reorderCompoundPortDummies(arena *LGraphArena, graphID LGraphID) {
	for _, nodeID := range append([]LNodeID(nil), arena.Graph(graphID).LayerlessNodes...) {
		if nested := arena.Node(nodeID).NestedGraph; nested != nil {
			reorderCompoundPortDummies(arena, *nested)
		}
	}

	nodes := append([]LNodeID(nil), arena.Graph(graphID).LayerlessNodes...)
	normal := make([]LNodeID, 0, len(nodes))
	dummies := make([]compoundPortDummyOrder, 0)
	for stableRank, nodeID := range nodes {
		if arena.Node(nodeID).Type != ExternalPortNode {
			normal = append(normal, nodeID)
			continue
		}
		order := compoundPortDummyOrder{
			node: nodeID, portIndex: math.MaxInt, edgeOrder: math.MaxInt,
			stableRank: stableRank,
		}
		if insidePort, edgeID, found := compoundDummyInsideEndpoint(arena, nodeID); found {
			order.connected = true
			order.path, order.portIndex = compoundInsideTraversalKey(arena, graphID, insidePort)
			if modelOrder, found := basegraph.TryGet(&arena.Edge(edgeID).Properties, ModelOrder); found {
				order.edgeOrder = modelOrder
			} else {
				order.edgeOrder = edgeID.Index()
			}
		} else if origin, found := basegraph.TryGet(&arena.Node(nodeID).Properties, OriginalElement); found &&
			origin.Kind == OriginLPort && arena.Port(origin.LPort).Node != nil {
			order.portIndex = nodePortIndex(arena, *arena.Port(origin.LPort).Node, origin.LPort)
		}
		dummies = append(dummies, order)
	}
	if len(dummies) < 2 {
		return
	}
	sort.SliceStable(dummies, func(first, second int) bool {
		left, right := dummies[first], dummies[second]
		if left.connected != right.connected {
			return left.connected
		}
		if comparison := compareCompoundTraversalPaths(left.path, right.path); comparison != 0 {
			return comparison < 0
		}
		if left.portIndex != right.portIndex {
			return left.portIndex < right.portIndex
		}
		if left.edgeOrder != right.edgeOrder {
			return left.edgeOrder < right.edgeOrder
		}
		return left.stableRank < right.stableRank
	})
	ordered := make([]LNodeID, 0, len(nodes))
	ordered = append(ordered, normal...)
	for _, order := range dummies {
		ordered = append(ordered, order.node)
	}
	arena.Graph(graphID).LayerlessNodes = ordered
}

// compoundDummyInsideEndpoint returns the endpoint of the dummy's segment
// that lies inside its graph. External input dummies have an outgoing segment;
// external output dummies have an incoming segment.
func compoundDummyInsideEndpoint(
	arena *LGraphArena,
	dummyID LNodeID,
) (LPortID, LEdgeID, bool) {
	for _, portID := range arena.Node(dummyID).Ports {
		for _, edgeID := range arena.Port(portID).OutgoingEdges {
			edge := arena.Edge(edgeID)
			if edge.Target != nil {
				return *edge.Target, edgeID, true
			}
		}
		for _, edgeID := range arena.Port(portID).IncomingEdges {
			edge := arena.Edge(edgeID)
			if edge.Source != nil {
				return *edge.Source, edgeID, true
			}
		}
	}
	return 0, 0, false
}

func compoundInsideTraversalKey(
	arena *LGraphArena,
	graphID LGraphID,
	portID LPortID,
) ([]int, int) {
	port := arena.Port(portID)
	if port.Node == nil {
		return nil, math.MaxInt
	}
	nodeID := *port.Node
	path := []int{graphNodeIndex(arena, graphID, nodeID)}
	node := arena.Node(nodeID)
	if node.NestedGraph != nil {
		if dummyID, found := basegraph.TryGet(&port.Properties, PortDummy); found {
			if nextPort, _, connected := compoundDummyInsideEndpoint(arena, dummyID); connected {
				nestedPath, terminalPort := compoundInsideTraversalKey(arena, *node.NestedGraph, nextPort)
				return append(path, nestedPath...), terminalPort
			}
		}
	}
	return path, nodePortIndex(arena, nodeID, portID)
}

func graphNodeIndex(arena *LGraphArena, graphID LGraphID, wanted LNodeID) int {
	index := 0
	for _, nodeID := range arena.Graph(graphID).LayerlessNodes {
		if arena.Node(nodeID).Type == ExternalPortNode {
			continue
		}
		if nodeID == wanted {
			return index
		}
		index++
	}
	return math.MaxInt
}

func nodePortIndex(arena *LGraphArena, nodeID LNodeID, wanted LPortID) int {
	for index, portID := range arena.Node(nodeID).Ports {
		if portID == wanted {
			return index
		}
	}
	return math.MaxInt
}

func compareCompoundTraversalPaths(first, second []int) int {
	limit := len(first)
	if len(second) < limit {
		limit = len(second)
	}
	for index := 0; index < limit; index++ {
		if first[index] < second[index] {
			return -1
		}
		if first[index] > second[index] {
			return 1
		}
	}
	// transformHierarchyEdges recurses before processing ports directly on a
	// compound child, so the descendant path sorts before its prefix.
	if len(first) > len(second) {
		return -1
	}
	if len(first) < len(second) {
		return 1
	}
	return 0
}

func ensureCompoundPortDummies(arena *LGraphArena, graphID LGraphID) {
	// CompoundGraphPreprocessor.transformHierarchyEdges is post-order. Copy the
	// node list because the recursive call may append external-port dummies to a
	// nested graph's layerless nodes.
	nodes := append([]LNodeID(nil), arena.Graph(graphID).LayerlessNodes...)
	for _, parentNode := range nodes {
		nested := arena.Node(parentNode).NestedGraph
		if nested == nil {
			continue
		}
		ensureCompoundPortDummies(arena, *nested)
		properties := basegraph.Get(&arena.Graph(*nested).Properties, GraphProperties)
		if properties&GraphPropertyExternalPorts == 0 {
			continue
		}

		constraints := basegraph.Get(&arena.Node(parentNode).Properties, PortConstraintsOption)
		ports := append([]LPortID(nil), arena.Node(parentNode).Ports...)
		for _, portID := range ports {
			if _, found := basegraph.TryGet(&arena.Port(portID).Properties, PortDummy); found {
				continue
			}
			createUnconnectedExternalPortDummy(arena, *nested, parentNode, portID, constraints)
		}
	}
}

func createUnconnectedExternalPortDummy(
	arena *LGraphArena,
	childGraph LGraphID,
	parentNode LNodeID,
	portID LPortID,
	constraints PortConstraints,
) LNodeID {
	port := arena.Port(portID)
	properties := port.Properties.Clone()
	labels := append([]LLabelID(nil), port.Labels...)
	position := geometry.Point{}
	netFlow := len(port.OutgoingEdges) - len(port.IncomingEdges)
	direction := basegraph.Get(&arena.Graph(childGraph).Properties, DirectionOption)
	dummy := arena.CreateExternalPortDummy(
		&properties,
		constraints,
		port.Side,
		netFlow,
		nil,
		&position,
		port.Size,
		direction,
		childGraph,
	)
	basegraph.Set(&arena.Node(dummy).Properties, OriginalElement, LPortOrigin(portID))
	arena.Graph(childGraph).LayerlessNodes = append(arena.Graph(childGraph).LayerlessNodes, dummy)

	// Dummy port labels reserve the same space as the compound node's ports.
	// They deliberately carry only dimensions; they are not exported labels.
	dummyPort := arena.Node(dummy).Ports[0]
	for _, labelID := range labels {
		dummyLabel := arena.AddLabel("")
		arena.Label(dummyLabel).Size = arena.Label(labelID).Size
		arena.Port(dummyPort).Labels = append(arena.Port(dummyPort).Labels, dummyLabel)
	}

	port = arena.Port(portID)
	basegraph.Set(&port.Properties, PortDummy, dummy)
	basegraph.Set(&port.Properties, InsideConnections, true)
	side := basegraph.Get(&arena.Node(dummy).Properties, ExternalPortSide)
	arena.SetPortSide(portID, side)
	basegraph.Set(&port.Properties, PortSideOption, side)
	basegraph.Set(&arena.Node(parentNode).Properties, PortConstraintsOption, PortConstraintsFixedSide)
	parentGraph := arena.NodeGraph(parentNode)
	parentProperties := basegraph.Get(&arena.Graph(parentGraph).Properties, GraphProperties)
	basegraph.Set(&arena.Graph(parentGraph).Properties, GraphProperties,
		parentProperties|GraphPropertyNonFreePorts)
	return dummy
}

func splitCrossHierarchyEdge(
	arena *LGraphArena,
	topGraph LGraphID,
	original LEdgeID,
	crossings *CrossHierarchyMap,
) error {
	edge := arena.Edge(original)
	originalSource, originalTarget := *edge.Source, *edge.Target
	sourceGraph := arena.NodeGraph(*arena.Port(originalSource).Node)
	targetGraph := arena.NodeGraph(*arena.Port(originalTarget).Node)
	lca, found := lowestCommonGraph(arena, sourceGraph, targetGraph, topGraph)
	if !found {
		return fmt.Errorf("layered: edge %d endpoints are outside graph %d's hierarchy", original, topGraph)
	}

	currentSource := originalSource
	currentGraph := sourceGraph
	sourceSegments := make([]CrossHierarchyEdge, 0)
	for currentGraph != lca {
		parentNode, parentGraph, ok := hierarchyParent(arena, currentGraph)
		if !ok {
			return fmt.Errorf("layered: graph %d has no parent on edge %d's source path", currentGraph, original)
		}
		boundary := boundaryPort(arena, currentGraph, parentGraph, parentNode,
			PortTypeOutput, original, originalTarget)
		dummy := externalDummyForBoundary(arena, currentGraph, parentNode, boundary,
			PortTypeOutput, original)
		dummyPort := arena.Node(dummy).Ports[0]
		segment := createHierarchyDummyEdge(arena, original, currentSource, dummyPort)
		sourceSegments = append(sourceSegments, CrossHierarchyEdge{
			Edge: segment, Graph: currentGraph, PortType: PortTypeOutput,
		})
		currentSource, currentGraph = boundary, parentGraph
	}

	currentTarget := originalTarget
	currentGraph = targetGraph
	targetSegmentsDeepFirst := make([]CrossHierarchyEdge, 0)
	for currentGraph != lca {
		parentNode, parentGraph, ok := hierarchyParent(arena, currentGraph)
		if !ok {
			return fmt.Errorf("layered: graph %d has no parent on edge %d's target path", currentGraph, original)
		}
		boundary := boundaryPort(arena, currentGraph, parentGraph, parentNode,
			PortTypeInput, original, originalSource)
		dummy := externalDummyForBoundary(arena, currentGraph, parentNode, boundary,
			PortTypeInput, original)
		dummyPort := arena.Node(dummy).Ports[0]
		segment := createHierarchyDummyEdge(arena, original, dummyPort, currentTarget)
		targetSegmentsDeepFirst = append(targetSegmentsDeepFirst, CrossHierarchyEdge{
			Edge: segment, Graph: currentGraph, PortType: PortTypeInput,
		})
		currentTarget, currentGraph = boundary, parentGraph
	}

	segments := append([]CrossHierarchyEdge(nil), sourceSegments...)
	if currentSource != currentTarget {
		centralType := PortTypeOutput
		if len(sourceSegments) == 0 {
			centralType = PortTypeInput
		}
		central := createHierarchyDummyEdge(arena, original, currentSource, currentTarget)
		segments = append(segments, CrossHierarchyEdge{Edge: central, Graph: lca, PortType: centralType})
	}
	for index := len(targetSegmentsDeepFirst) - 1; index >= 0; index-- {
		segments = append(segments, targetSegmentsDeepFirst[index])
	}
	if len(segments) == 0 {
		return fmt.Errorf("layered: cross-hierarchy edge %d produced no segments", original)
	}
	for _, segment := range segments {
		crossings.add(original, segment)
	}
	moveCrossHierarchyLabels(arena, topGraph, original, segments)
	arena.SetEdgeSource(original, nil)
	arena.SetEdgeTarget(original, nil)
	return nil
}

func hierarchyParent(arena *LGraphArena, graphID LGraphID) (LNodeID, LGraphID, bool) {
	parent := arena.Graph(graphID).ParentNode
	if parent == nil {
		return 0, 0, false
	}
	return *parent, arena.NodeGraph(*parent), true
}

func lowestCommonGraph(
	arena *LGraphArena,
	first, second, top LGraphID,
) (LGraphID, bool) {
	ancestors := make(map[LGraphID]bool)
	current := first
	for {
		ancestors[current] = true
		if current == top {
			break
		}
		_, parent, ok := hierarchyParent(arena, current)
		if !ok {
			return 0, false
		}
		current = parent
	}
	current = second
	for {
		if ancestors[current] {
			return current, true
		}
		if current == top {
			return 0, false
		}
		_, parent, ok := hierarchyParent(arena, current)
		if !ok {
			return 0, false
		}
		current = parent
	}
}

// boundaryPort returns an existing port when the edge ends at the compound
// node itself. Otherwise it introduces the external port exposed by that
// compound node to its parent graph.
func boundaryPort(
	arena *LGraphArena,
	childGraph, parentGraph LGraphID,
	parentNode LNodeID,
	portType PortType,
	original LEdgeID,
	opposite LPortID,
) LPortID {
	if arena.Port(opposite).Node != nil && *arena.Port(opposite).Node == parentNode {
		return opposite
	}
	portID := arena.AddPort()
	arena.SetPortNode(portID, &parentNode)
	direction := basegraph.Get(&arena.Graph(parentGraph).Properties, DirectionOption)
	side := DirectionPortSide(direction)
	if portType == PortTypeInput {
		side = side.Opposed()
	}
	arena.SetPortSide(portID, side)
	basegraph.Set(&arena.Port(portID).Properties, PortSideOption, side)
	basegraph.Set(&arena.Port(portID).Properties, PortBorderOffset,
		basegraph.Get(&arena.Graph(childGraph).Properties, SpacingEdgeEdge)/2)
	basegraph.Set(&arena.Port(portID).Properties, OriginalElement, LEdgeOrigin(original))
	return portID
}

func externalDummyForBoundary(
	arena *LGraphArena,
	childGraph LGraphID,
	parentNode LNodeID,
	boundary LPortID,
	portType PortType,
	original LEdgeID,
) LNodeID {
	port := arena.Port(boundary)
	// CreateExternalPortDummy adds the dummy's own port. Keep its property
	// holder detached from arena.Ports so a backing-slice reallocation cannot
	// leave the helper (or the writes below) pointing at stale storage.
	properties := port.Properties.Clone()
	constraints := basegraph.Get(&arena.Node(parentNode).Properties, PortConstraintsOption)
	if constraints == PortConstraintsUndefined {
		constraints = PortConstraintsFree
	}
	netFlow := 1
	if portType == PortTypeInput {
		netFlow = -1
	}
	// A segment that ends directly at the compound parent reuses the original
	// endpoint port. ELK derives that port's side from all of its connections,
	// rather than from the hierarchy segment's INPUT/OUTPUT classification.
	// The distinction matters for an edge from an ancestor to a non-direct
	// descendant: the segment points into deeper hierarchy, but the ancestor
	// port's net flow still makes it an output port.
	edge := arena.Edge(original)
	outsidePort := edge.Target
	if portType == PortTypeInput {
		outsidePort = edge.Source
	}
	if outsidePort != nil && arena.Port(*outsidePort).Node != nil &&
		*arena.Port(*outsidePort).Node == parentNode {
		netFlow = compoundPortNetFlow(arena, *outsidePort)
	}
	position := port.Position
	nodeSize := arena.Node(parentNode).Size
	direction := basegraph.Get(&arena.Graph(childGraph).Properties, DirectionOption)
	dummy := arena.CreateExternalPortDummy(&properties, constraints, port.Side,
		netFlow, &nodeSize, &position, port.Size, direction, childGraph)
	basegraph.Set(&arena.Node(dummy).Properties, OriginalElement, LPortOrigin(boundary))
	arena.Graph(childGraph).LayerlessNodes = append(arena.Graph(childGraph).LayerlessNodes, dummy)
	port = arena.Port(boundary)
	basegraph.Set(&port.Properties, PortDummy, dummy)
	basegraph.Set(&port.Properties, InsideConnections, true)
	side := basegraph.Get(&arena.Node(dummy).Properties, ExternalPortSide)
	arena.SetPortSide(boundary, side)
	basegraph.Set(&port.Properties, PortSideOption, side)
	basegraph.Set(&arena.Node(parentNode).Properties, PortConstraintsOption, PortConstraintsFixedSide)

	childProperties := basegraph.Get(&arena.Graph(childGraph).Properties, GraphProperties)
	basegraph.Set(&arena.Graph(childGraph).Properties, GraphProperties,
		childProperties|GraphPropertyExternalPorts)
	if basegraph.Get(&arena.Graph(childGraph).Properties, PortConstraintsOption).IsSideFixed() {
		basegraph.Set(&arena.Graph(childGraph).Properties, PortConstraintsOption, PortConstraintsFixedSide)
	} else {
		basegraph.Set(&arena.Graph(childGraph).Properties, PortConstraintsOption, PortConstraintsFree)
	}
	parentProperties := basegraph.Get(&arena.Graph(arena.NodeGraph(parentNode)).Properties, GraphProperties)
	basegraph.Set(&arena.Graph(arena.NodeGraph(parentNode)).Properties, GraphProperties,
		parentProperties|GraphPropertyNonFreePorts)
	return dummy
}

// compoundPortNetFlow mirrors CompoundGraphPreprocessor.calculateNetFlow.
// Positive values make a port an output of its compound parent; negative
// values make it an input.
func compoundPortNetFlow(arena *LGraphArena, portID LPortID) int {
	port := arena.Port(portID)
	if port.Node == nil {
		return 0
	}
	nodeID := *port.Node
	insideSelfLoops := basegraph.Get(&arena.Node(nodeID).Properties, core.InsideSelfLoopsActivate)
	outputVotes, inputVotes := 0, 0

	for _, edgeID := range port.OutgoingEdges {
		edge := arena.Edge(edgeID)
		if edge.Source == nil || edge.Target == nil {
			continue
		}
		selfLoop := arena.EdgeIsSelfLoop(edgeID)
		inside := selfLoop && insideSelfLoops &&
			basegraph.Get(&edge.Properties, core.InsideSelfLoopsEdge)
		targetNode := arena.EdgeTargetNode(edgeID)
		targetGraph := arena.Graph(arena.NodeGraph(targetNode))
		switch {
		case selfLoop && inside:
			inputVotes++
		case selfLoop:
			outputVotes++
		case targetGraph.ParentNode != nil && *targetGraph.ParentNode == nodeID:
			inputVotes++
		default:
			outputVotes++
		}
	}

	for _, edgeID := range port.IncomingEdges {
		edge := arena.Edge(edgeID)
		if edge.Source == nil || edge.Target == nil {
			continue
		}
		selfLoop := arena.EdgeIsSelfLoop(edgeID)
		inside := selfLoop && insideSelfLoops &&
			basegraph.Get(&edge.Properties, core.InsideSelfLoopsEdge)
		sourceNode := arena.EdgeSourceNode(edgeID)
		sourceGraph := arena.Graph(arena.NodeGraph(sourceNode))
		switch {
		case selfLoop && inside:
			outputVotes++
		case selfLoop:
			inputVotes++
		case sourceGraph.ParentNode != nil && *sourceGraph.ParentNode == nodeID:
			outputVotes++
		default:
			inputVotes++
		}
	}

	return outputVotes - inputVotes
}

func createHierarchyDummyEdge(
	arena *LGraphArena,
	original LEdgeID,
	source, target LPortID,
) LEdgeID {
	dummy := arena.AddEdge()
	arena.Edge(dummy).Properties.CopyFrom(&arena.Edge(original).Properties)
	basegraph.Unset(&arena.Edge(dummy).Properties, JunctionPoints)
	arena.SetEdgeSource(dummy, &source)
	arena.SetEdgeTarget(dummy, &target)
	return dummy
}

func moveCrossHierarchyLabels(
	arena *LGraphArena,
	topGraph LGraphID,
	original LEdgeID,
	segments []CrossHierarchyEdge,
) {
	labels := append([]LLabelID(nil), arena.Edge(original).Labels...)
	if len(labels) == 0 {
		return
	}
	sorted := append([]CrossHierarchyEdge(nil), segments...)
	sortHierarchySegments(arena, sorted, topGraph)
	remaining := make([]LLabelID, 0)
	for _, labelID := range labels {
		target := -1
		switch basegraph.Get(&arena.Label(labelID).Properties, EdgeLabelsPlacement) {
		case EdgeLabelPlacementTail:
			target = 0
		case EdgeLabelPlacementHead:
			target = len(sorted) - 1
		case EdgeLabelPlacementCenter:
			target = shallowestHierarchySegment(sorted)
		}
		if target < 0 {
			remaining = append(remaining, labelID)
			continue
		}
		segment := sorted[target]
		arena.Edge(segment.Edge).Labels = append(arena.Edge(segment.Edge).Labels, labelID)
		properties := basegraph.Get(&arena.Graph(segment.Graph).Properties, GraphProperties)
		basegraph.Set(&arena.Graph(segment.Graph).Properties, GraphProperties,
			properties|GraphPropertyEndLabels|GraphPropertyCenterLabels)
		basegraph.Set(&arena.Label(labelID).Properties, OriginalLabelEdge, original)
	}
	arena.Edge(original).Labels = remaining
}

func shallowestHierarchySegment(segments []CrossHierarchyEdge) int {
	result := -1
	for index, segment := range segments {
		if segment.PortType == PortTypeInput {
			if index == 0 {
				return 0
			}
			return index - 1
		}
		if index == len(segments)-1 {
			result = index
		}
	}
	return result
}

func sortHierarchySegments(arena *LGraphArena, segments []CrossHierarchyEdge, top LGraphID) {
	sort.SliceStable(segments, func(first, second int) bool {
		left, right := segments[first], segments[second]
		if left.PortType == PortTypeOutput && right.PortType == PortTypeInput {
			return true
		}
		if left.PortType == PortTypeInput && right.PortType == PortTypeOutput {
			return false
		}
		leftLevel := hierarchyLevel(arena, left.Graph, top)
		rightLevel := hierarchyLevel(arena, right.Graph, top)
		if left.PortType == PortTypeOutput {
			return leftLevel > rightLevel
		}
		return leftLevel < rightLevel
	})
}

func hierarchyLevel(arena *LGraphArena, graphID, top LGraphID) int {
	level := 0
	for graphID != top {
		_, parent, ok := hierarchyParent(arena, graphID)
		if !ok {
			return int(^uint(0) >> 1)
		}
		graphID = parent
		level++
	}
	return level
}

// CompoundGraphPostprocessor joins the laid-out hierarchy segments back into
// their original edge while translating bend points, junction points, labels,
// and the target anchor into a single reference coordinate system.
type CompoundGraphPostprocessor struct{}

func (*CompoundGraphPostprocessor) Process(arena *LGraphArena, topGraph LGraphID) error {
	if arena == nil || topGraph.Index() >= len(arena.Graphs) {
		return fmt.Errorf("layered: invalid graph passed to compound postprocessor")
	}
	crossings, found := basegraph.TryGet(&arena.Graph(topGraph).Properties, CrossHierarchyEdges)
	if !found || crossings == nil {
		return nil
	}
	addUnnecessary := basegraph.Get(&arena.Graph(topGraph).Properties, UnnecessaryBendpoints)
	dummyEdges := make([]LEdgeID, 0)
	for _, entry := range crossings.Entries {
		if len(entry.Segments) == 0 {
			continue
		}
		segments := append([]CrossHierarchyEdge(nil), entry.Segments...)
		sortHierarchySegments(arena, segments, topGraph)
		sourcePort := segmentActualSource(arena, segments[0])
		targetPort := segmentActualTarget(arena, segments[len(segments)-1])
		sourceNode, targetNode := *arena.Port(sourcePort).Node, *arena.Port(targetPort).Node
		referenceGraph := arena.NodeGraph(sourceNode)
		if arena.IsDescendant(targetNode, sourceNode) && arena.Node(sourceNode).NestedGraph != nil {
			referenceGraph = *arena.Node(sourceNode).NestedGraph
		}

		junctions := prepareJunctionPoints(arena, entry.Original, segments)
		arena.Edge(entry.Original).BendPoints = nil
		var lastPoint *geometry.Point
		for _, segment := range segments {
			offset := graphCoordinateOffset(arena, segment.Graph, referenceGraph)
			ledge := arena.Edge(segment.Edge)
			bends := make(geometry.Chain, len(ledge.BendPoints))
			for index, point := range ledge.BendPoints {
				bends[index] = point.Add(offset)
			}
			sourcePoint := portAbsoluteAnchor(arena, *ledge.Source).Add(offset)
			targetPoint := portAbsoluteAnchor(arena, *ledge.Target).Add(offset)
			if lastPoint != nil {
				nextPoint := targetPoint
				if len(bends) > 0 {
					nextPoint = bends[0]
				}
				xDifferent := math.Abs(lastPoint.X-nextPoint.X) > .00001
				yDifferent := math.Abs(lastPoint.Y-nextPoint.Y) > .00001
				if (!addUnnecessary && xDifferent && yDifferent) ||
					(addUnnecessary && (xDifferent || yDifferent)) {
					arena.Edge(entry.Original).BendPoints = append(
						arena.Edge(entry.Original).BendPoints, sourcePoint)
				}
			}
			arena.Edge(entry.Original).BendPoints = append(
				arena.Edge(entry.Original).BendPoints, bends...)
			if len(bends) == 0 {
				point := sourcePoint
				lastPoint = &point
			} else {
				point := bends[len(bends)-1]
				lastPoint = &point
			}
			if segmentJunctions, ok := basegraph.TryGet(&ledge.Properties, JunctionPoints); ok && junctions != nil {
				for _, point := range segmentJunctions {
					*junctions = append(*junctions, point.Add(offset))
				}
			}
			if segmentActualTarget(arena, segment) == targetPort {
				targetOffset := offset
				targetGraph := arena.NodeGraph(*arena.Port(targetPort).Node)
				if targetGraph != segment.Graph {
					targetOffset = graphCoordinateOffset(arena, targetGraph, referenceGraph)
				}
				basegraph.Set(&arena.Edge(entry.Original).Properties, TargetOffset, targetOffset)
			}
			copyHierarchyLabelsBack(arena, segment.Edge, entry.Original, referenceGraph)
			dummyEdges = append(dummyEdges, segment.Edge)
		}
		if junctions != nil {
			basegraph.Set(&arena.Edge(entry.Original).Properties, JunctionPoints, *junctions)
		}
		arena.SetEdgeSource(entry.Original, &sourcePort)
		arena.SetEdgeTarget(entry.Original, &targetPort)
	}
	seen := make(map[LEdgeID]bool)
	for _, edgeID := range dummyEdges {
		if seen[edgeID] {
			continue
		}
		seen[edgeID] = true
		arena.SetEdgeSource(edgeID, nil)
		arena.SetEdgeTarget(edgeID, nil)
	}
	return nil
}

func segmentActualSource(arena *LGraphArena, segment CrossHierarchyEdge) LPortID {
	return segmentActualPort(arena, *arena.Edge(segment.Edge).Source)
}

func segmentActualTarget(arena *LGraphArena, segment CrossHierarchyEdge) LPortID {
	return segmentActualPort(arena, *arena.Edge(segment.Edge).Target)
}

func segmentActualPort(arena *LGraphArena, portID LPortID) LPortID {
	port := arena.Port(portID)
	if port.Node == nil || arena.Node(*port.Node).Type != ExternalPortNode {
		return portID
	}
	if origin, found := basegraph.TryGet(&arena.Node(*port.Node).Properties, OriginalElement); found && origin.Kind == OriginLPort {
		return origin.LPort
	}
	return portID
}

func prepareJunctionPoints(
	arena *LGraphArena,
	original LEdgeID,
	segments []CrossHierarchyEdge,
) *geometry.Chain {
	any := false
	for _, segment := range segments {
		if points, found := basegraph.TryGet(&arena.Edge(segment.Edge).Properties, JunctionPoints); found && len(points) > 0 {
			any = true
			break
		}
	}
	if !any {
		basegraph.Unset(&arena.Edge(original).Properties, JunctionPoints)
		return nil
	}
	points := geometry.Chain{}
	return &points
}

func copyHierarchyLabelsBack(
	arena *LGraphArena,
	segment, original LEdgeID,
	referenceGraph LGraphID,
) {
	labels := append([]LLabelID(nil), arena.Edge(segment).Labels...)
	remaining := make([]LLabelID, 0, len(labels))
	for _, labelID := range labels {
		labelEdge, found := basegraph.TryGet(&arena.Label(labelID).Properties, OriginalLabelEdge)
		if !found || labelEdge != original {
			remaining = append(remaining, labelID)
			continue
		}
		sourceGraph := arena.NodeGraph(arena.EdgeSourceNode(segment))
		arena.Label(labelID).Position = changeGraphCoordinates(
			arena, arena.Label(labelID).Position, sourceGraph, referenceGraph)
		arena.Edge(original).Labels = append(arena.Edge(original).Labels, labelID)
	}
	arena.Edge(segment).Labels = remaining
}

func portAbsoluteAnchor(arena *LGraphArena, portID LPortID) geometry.Point {
	port := arena.Port(portID)
	if port.Node == nil {
		return port.Position.Add(port.Anchor)
	}
	return arena.Node(*port.Node).Position.Add(port.Position).Add(port.Anchor)
}

func graphCoordinateOffset(arena *LGraphArena, from, to LGraphID) geometry.Point {
	return changeGraphCoordinates(arena, geometry.Point{}, from, to)
}

func changeGraphCoordinates(
	arena *LGraphArena,
	point geometry.Point,
	from, to LGraphID,
) geometry.Point {
	// LGraphUtil.changeCoordSystem returns before doing any arithmetic when
	// both coordinate systems are identical. Apart from avoiding needless
	// work, that preserves the input's exact floating-point representation.
	if from == to {
		return point
	}
	// LGraphUtil.changeCoordSystem mutates the supplied vector while it walks
	// the old hierarchy, then walks the new hierarchy and subtracts each term.
	// Reconstructing two absolute origins and subtracting them is algebraically
	// equivalent but changes the final ULP for fractional graph offsets, so
	// preserve the Java/GWT evaluation order here.
	result := point
	for graphID := from; ; {
		graph := arena.Graph(graphID)
		result = result.Add(graph.Offset)
		if graph.ParentNode == nil {
			break
		}
		result = result.Add(geometry.Point{X: graph.Padding.Left, Y: graph.Padding.Top})
		parent := *graph.ParentNode
		result = result.Add(arena.Node(parent).Position)
		graphID = arena.NodeGraph(parent)
	}
	for graphID := to; ; {
		graph := arena.Graph(graphID)
		result = result.Sub(graph.Offset)
		if graph.ParentNode == nil {
			break
		}
		result = result.Sub(geometry.Point{X: graph.Padding.Left, Y: graph.Padding.Top})
		parent := *graph.ParentNode
		result = result.Sub(arena.Node(parent).Position)
		graphID = arena.NodeGraph(parent)
	}
	return result
}

func graphContentOrigin(arena *LGraphArena, graphID LGraphID) geometry.Point {
	graph := arena.Graph(graphID)
	local := graph.Offset.Add(geometry.Point{X: graph.Padding.Left, Y: graph.Padding.Top})
	if graph.ParentNode == nil {
		return local
	}
	parent := *graph.ParentNode
	return graphContentOrigin(arena, arena.NodeGraph(parent)).Add(
		arena.Node(parent).Position).Add(local)
}
