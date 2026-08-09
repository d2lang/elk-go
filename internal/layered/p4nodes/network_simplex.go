// Copyright (c) 2016, 2017 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"errors"
	"math"

	"github.com/d2lang/elk-go/internal/common/networksimplex"
	"github.com/d2lang/elk-go/internal/common/nodespacing"
	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
)

const (
	networkSimplexEdgeWeightBase       = 4.0
	networkSimplexSmallEdgeWeight      = 0.1
	networkSimplexLongEdgeVsPathFactor = 2.0
	networkSimplexNodeSizeWeightStatic = 10000.0
	networkSimplexNodeSizeWeightFlex   = 1.0
	networkSimplexEpsilon              = 0.00001
)

// NetworkSimplexPlacer is the arena-backed port of ELK 0.8.1's
// NetworkSimplexPlacer. It converts vertical placement into an auxiliary
// layering problem and solves that problem with ELK's common network simplex.
type NetworkSimplexPlacer struct{}

func (*NetworkSimplexPlacer) ProcessorConfiguration(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) layered.ProcessorConfiguration {
	var configuration layered.ProcessorConfiguration
	if basegraph.Get(&arena.Graph(graphID).Properties, layered.GraphProperties)&
		layered.GraphPropertyExternalPorts != 0 {
		configuration.AddBefore(layered.PhaseEdgeRouting, layered.HierarchicalPortPositionProcessor)
	}
	return configuration
}

func (*NetworkSimplexPlacer) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	placer := networkSimplexPlacement{
		arena:              arena,
		graphID:            graphID,
		portMap:            make(map[layered.LPortID]networksimplex.NodeID),
		flexibleWhereSpace: make(map[networksimplex.EdgeID]struct{}),
	}
	placer.prepare()
	if err := placer.buildInitialAuxiliaryGraph(); err != nil {
		return err
	}
	placer.insertNorthSouthAuxiliaryEdges()
	if err := placer.insertInLayerAuxiliaryEdges(); err != nil {
		return err
	}
	if basegraph.Get(&arena.Graph(graphID).Properties, layered.FavorStraightEdges) {
		placer.preferStraightEdges()
	}
	placer.network.MakeConnected()
	iterationLimit := basegraph.Get(&arena.Graph(graphID).Properties, layered.Thoroughness) *
		len(placer.network.Nodes)
	networksimplex.ForGraph(placer.network).
		WithIterationLimit(iterationLimit).
		WithBalancing(false).
		Execute()
	if len(placer.flexibleWhereSpace) != 0 {
		placer.insertFlexibleWhereSpaceAuxiliaryEdges()
		for edgeID := range placer.flexibleWhereSpace {
			placer.network.Edge(edgeID).Weight = networkSimplexNodeSizeWeightFlex
		}
		networksimplex.ForGraph(placer.network).
			WithIterationLimit(iterationLimit).
			WithBalancing(false).
			Execute()
	}
	if basegraph.Get(&arena.Graph(graphID).Properties, layered.FavorStraightEdges) {
		placer.postProcessTwoPaths()
	}
	placer.applyPositions()
	return nil
}

type networkSimplexPlacement struct {
	arena   *layered.LGraphArena
	graphID layered.LGraphID
	network *networksimplex.Graph

	nodeReps []networkSimplexNodeRep
	edgeReps []networkSimplexEdgeRep
	portMap  map[layered.LPortID]networksimplex.NodeID

	nodeState []int
	twoPaths  [][]layered.LEdgeID
	crossing  []bool

	flexibleWhereSpace map[networksimplex.EdgeID]struct{}
}

type networkSimplexNodeRep struct {
	origin     layered.LNodeID
	head       networksimplex.NodeID
	tail       networksimplex.NodeID
	isFlexible bool
}

type networkSimplexEdgeRep struct {
	origin layered.LEdgeID
	left   networksimplex.EdgeID
	right  networksimplex.EdgeID
}

func (rep networkSimplexEdgeRep) notStraightBy(graph *networksimplex.Graph) int {
	left, right := graph.Edge(rep.left), graph.Edge(rep.right)
	return (graph.Node(left.Target).Layer - left.Delta) -
		(graph.Node(right.Target).Layer - right.Delta)
}

func (rep networkSimplexEdgeRep) isStraight(graph *networksimplex.Graph) bool {
	return rep.notStraightBy(graph) == 0
}

func (placer *networkSimplexPlacement) prepare() {
	placer.network = networksimplex.NewGraph()
	nodeCount, edgeCount := 0, 0
	for _, layerID := range placer.arena.Graph(placer.graphID).Layers {
		for _, nodeID := range placer.arena.Layer(layerID).Nodes {
			node := placer.arena.Node(nodeID)
			node.ID = nodeCount
			nodeCount++
			for _, edgeID := range placer.arena.NodeOutgoingEdges(nodeID) {
				placer.arena.Edge(edgeID).ID = edgeCount
				edgeCount++
			}

			anchorMustBeInteger := placer.isFlexibleNode(nodeID)
			for _, portID := range node.Ports {
				port := placer.arena.Port(portID)
				if anchorMustBeInteger && port.Anchor.Y != math.Floor(port.Anchor.Y) {
					offset := port.Anchor.Y - float64(javacompat.MathRoundDouble(port.Anchor.Y))
					port.Anchor.Y -= offset
				}
				positionAndAnchor := port.Position.Y + port.Anchor.Y
				if positionAndAnchor != math.Floor(positionAndAnchor) {
					offset := positionAndAnchor - float64(javacompat.MathRoundDouble(positionAndAnchor))
					port.Position.Y -= offset
				}
			}
		}
	}
	placer.nodeReps = make([]networkSimplexNodeRep, nodeCount)
	placer.edgeReps = make([]networkSimplexEdgeRep, edgeCount)
}

func (placer *networkSimplexPlacement) buildInitialAuxiliaryGraph() error {
	for _, layerID := range placer.arena.Graph(placer.graphID).Layers {
		placer.transformLayer(layerID)
	}
	for _, layerID := range placer.arena.Graph(placer.graphID).Layers {
		for _, nodeID := range placer.arena.Layer(layerID).Nodes {
			for _, edgeID := range placer.arena.NodeOutgoingEdges(nodeID) {
				if placer.isHandledEdge(edgeID) {
					if err := placer.transformEdge(edgeID); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (placer *networkSimplexPlacement) transformLayer(layerID layered.LayerID) {
	var previous *networkSimplexNodeRep
	for _, nodeID := range placer.arena.Layer(layerID).Nodes {
		var rep networkSimplexNodeRep
		if placer.isFlexibleNode(nodeID) {
			rep = placer.transformFixedOrderNode(nodeID)
		} else {
			rep = placer.transformFixedPositionNode(nodeID)
		}
		placer.nodeReps[placer.arena.Node(nodeID).ID] = rep
		if previous != nil {
			previousNode := placer.arena.Node(previous.origin)
			node := placer.arena.Node(nodeID)
			spacing := previousNode.Margin.Bottom +
				layered.VerticalSpacing(placer.arena, previous.origin, nodeID) +
				node.Margin.Top
			if !previous.isFlexible {
				spacing += previousNode.Size.Y
			}
			placer.network.AddEdge(previous.tail, rep.head, 0, int(math.Ceil(spacing)))
		}
		copyRep := rep
		previous = &copyRep
	}
}

func (placer *networkSimplexPlacement) transformFixedPositionNode(
	nodeID layered.LNodeID,
) networkSimplexNodeRep {
	nodeRep := placer.network.AddNode()
	placer.network.Node(nodeRep).Origin = nodeID
	for _, portID := range placer.arena.Node(nodeID).Ports {
		side := placer.arena.Port(portID).Side
		if side == layered.EastSide || side == layered.WestSide {
			placer.portMap[portID] = nodeRep
		}
	}
	return networkSimplexNodeRep{origin: nodeID, head: nodeRep, tail: nodeRep}
}

func (placer *networkSimplexPlacement) transformFixedOrderNode(
	nodeID layered.LNodeID,
) networkSimplexNodeRep {
	head := placer.network.AddNode()
	tail := placer.network.AddNode()
	placer.network.Node(head).Origin = nodeID
	placer.network.Node(tail).Origin = nodeID
	rep := networkSimplexNodeRep{origin: nodeID, head: head, tail: tail, isFlexible: true}

	flexibility := placer.nodeFlexibility(nodeID)
	weight := networkSimplexNodeSizeWeightStatic
	if flexibility.IsFlexibleSize() {
		weight = networkSimplexNodeSizeWeightFlex
	}
	sizeEdge := placer.network.AddEdge(
		head, tail, weight, int(math.Ceil(placer.arena.Node(nodeID).Size.Y)),
	)
	if flexibility == layered.NodeFlexibilitySizeWhereSpacePermits {
		placer.flexibleWhereSpace[sizeEdge] = struct{}{}
	}

	west := append([]layered.LPortID(nil), placer.arena.NodePortSideView(nodeID, layered.WestSide)...)
	for left, right := 0, len(west)-1; left < right; left, right = left+1, right-1 {
		west[left], west[right] = west[right], west[left]
	}
	placer.transformPorts(west, rep)
	placer.transformPorts(placer.arena.NodePortSideView(nodeID, layered.EastSide), rep)
	return rep
}

func (placer *networkSimplexPlacement) transformPorts(
	ports []layered.LPortID,
	rep networkSimplexNodeRep,
) {
	if len(ports) == 0 {
		return
	}
	portSpacing := layered.IndividualOrDefault(
		placer.arena, rep.origin, &layered.SpacingPortPort,
	)
	portSurrounding := placer.portSurrounding(rep.origin)
	lastNode := rep.head
	var lastPort *layered.LPortID
	for _, portID := range ports {
		spacing := portSurrounding.Top
		if lastPort != nil {
			spacing = portSpacing + placer.arena.Port(*lastPort).Size.Y
		}
		portNode := placer.network.AddNode()
		placer.network.Node(portNode).Origin = portID
		placer.portMap[portID] = portNode
		placer.network.AddEdge(lastNode, portNode, 0, int(math.Ceil(spacing)))
		copyPort := portID
		lastPort = &copyPort
		lastNode = portNode
	}
	placer.network.AddEdge(
		lastNode,
		rep.tail,
		0,
		int(math.Ceil(portSurrounding.Bottom+placer.arena.Port(*lastPort).Size.Y)),
	)
}

func (placer *networkSimplexPlacement) transformEdge(edgeID layered.LEdgeID) error {
	edge := placer.arena.Edge(edgeID)
	dummy := placer.network.AddNode()
	sourcePort, targetPort := *edge.Source, *edge.Target
	sourceNode, sourceFound := placer.portMap[sourcePort]
	targetNode, targetFound := placer.portMap[targetPort]
	if !sourceFound || !targetFound {
		return errors.New("NEdge must have a source and target NNode specified")
	}
	sourceRep := placer.nodeReps[placer.arena.Node(placer.arena.EdgeSourceNode(edgeID)).ID]
	targetRep := placer.nodeReps[placer.arena.Node(placer.arena.EdgeTargetNode(edgeID)).ID]
	sourceOffset := placer.arena.Port(sourcePort).Anchor.Y
	targetOffset := placer.arena.Port(targetPort).Anchor.Y
	if !sourceRep.isFlexible {
		sourceOffset += placer.arena.Port(sourcePort).Position.Y
	}
	if !targetRep.isFlexible {
		targetOffset += placer.arena.Port(targetPort).Position.Y
	}
	targetDelta := int(math.Max(0, sourceOffset-targetOffset))
	sourceDelta := int(math.Max(0, targetOffset-sourceOffset))
	weight := placer.edgeWeight(edgeID)
	left := placer.network.AddEdge(dummy, sourceNode, weight, sourceDelta)
	right := placer.network.AddEdge(dummy, targetNode, weight, targetDelta)
	placer.network.Edge(left).Origin = edgeID
	placer.network.Edge(right).Origin = edgeID
	placer.edgeReps[edge.ID] = networkSimplexEdgeRep{origin: edgeID, left: left, right: right}
	return nil
}

func (placer *networkSimplexPlacement) insertInLayerAuxiliaryEdges() error {
	for _, layerID := range placer.arena.Graph(placer.graphID).Layers {
		for _, nodeID := range placer.arena.Layer(layerID).Nodes {
			if placer.arena.Node(nodeID).Type != layered.NormalNode {
				continue
			}
			for _, edgeID := range placer.arena.NodeConnectedEdges(nodeID) {
				if !placer.arena.EdgeIsInLayer(edgeID) {
					continue
				}
				source := placer.arena.EdgeSourceNode(edgeID)
				target := placer.arena.EdgeTargetNode(edgeID)
				sourceIsDummy := placer.arena.Node(source).Type != layered.NormalNode
				portID, dummyID := *placer.arena.Edge(edgeID).Source, target
				if sourceIsDummy {
					portID, dummyID = *placer.arena.Edge(edgeID).Target, source
				}
				portRep, found := placer.portMap[portID]
				if !found {
					return errors.New("NEdge must have a source and target NNode specified")
				}
				dummyRep := placer.nodeReps[placer.arena.Node(dummyID).ID].head
				if placer.arena.NodeIndexInLayer(*placer.arena.Port(portID).Node) <
					placer.arena.NodeIndexInLayer(dummyID) {
					placer.network.AddEdge(portRep, dummyRep, networkSimplexEdgeWeightBase, 0)
				} else {
					placer.network.AddEdge(dummyRep, portRep, networkSimplexEdgeWeightBase, 0)
				}
			}
		}
	}
	return nil
}

func (placer *networkSimplexPlacement) insertNorthSouthAuxiliaryEdges() {
	for _, layerID := range placer.arena.Graph(placer.graphID).Layers {
		for _, nodeID := range placer.arena.Layer(layerID).Nodes {
			for _, portID := range placer.arena.NodePortSideView(nodeID, layered.SouthSide) {
				if dummy, found := basegraph.TryGet(&placer.arena.Port(portID).Properties, layered.PortDummy); found {
					placer.network.AddEdge(
						placer.nodeReps[placer.arena.Node(nodeID).ID].tail,
						placer.nodeReps[placer.arena.Node(dummy).ID].head,
						networkSimplexSmallEdgeWeight,
						0,
					)
				}
			}
			for _, portID := range placer.arena.NodePortSideView(nodeID, layered.NorthSide) {
				if dummy, found := basegraph.TryGet(&placer.arena.Port(portID).Properties, layered.PortDummy); found {
					placer.network.AddEdge(
						placer.nodeReps[placer.arena.Node(dummy).ID].tail,
						placer.nodeReps[placer.arena.Node(nodeID).ID].head,
						networkSimplexSmallEdgeWeight,
						0,
					)
				}
			}
		}
	}
}

func (placer *networkSimplexPlacement) insertFlexibleWhereSpaceAuxiliaryEdges() {
	minimumLayer, maximumLayer := math.MaxInt, math.MinInt
	for _, nodeID := range placer.network.Nodes {
		layer := placer.network.Node(nodeID).Layer
		minimumLayer = min(minimumLayer, layer)
		maximumLayer = max(maximumLayer, layer)
	}
	usedLayers := maximumLayer - minimumLayer
	globalSource := placer.network.AddNode()
	globalSink := placer.network.AddNode()
	placer.network.AddEdge(
		globalSource, globalSink, networkSimplexNodeSizeWeightStatic*2, usedLayers,
	)
	for _, rep := range placer.nodeReps {
		node := placer.arena.Node(rep.origin)
		if node.Type != layered.NormalNode || len(node.Ports) <= 1 {
			continue
		}
		placer.network.AddEdge(globalSource, rep.tail, 0,
			placer.network.Node(rep.tail).Layer-minimumLayer)
		placer.network.AddEdge(rep.head, globalSink, 0,
			usedLayers-placer.network.Node(rep.head).Layer)
	}
}

func (placer *networkSimplexPlacement) applyPositions() {
	for _, layerID := range placer.arena.Graph(placer.graphID).Layers {
		for _, nodeID := range placer.arena.Layer(layerID).Nodes {
			rep := placer.nodeReps[placer.arena.Node(nodeID).ID]
			minimumY := float64(placer.network.Node(rep.head).Layer)
			maximumY := float64(placer.network.Node(rep.tail).Layer)
			node := placer.arena.Node(nodeID)
			node.Position.Y = minimumY
			sizeDelta := maximumY - minimumY - node.Size.Y
			flexibility := placer.nodeFlexibility(nodeID)
			if rep.isFlexible && flexibility.IsFlexibleSizeWhereSpacePermits() {
				node.Size.Y += sizeDelta
			}
			if rep.isFlexible && flexibility.IsFlexiblePorts() {
				for _, portID := range node.Ports {
					port := placer.arena.Port(portID)
					if port.Side == layered.EastSide || port.Side == layered.WestSide {
						port.Position.Y = float64(placer.network.Node(placer.portMap[portID]).Layer) - minimumY
					}
				}
				placer.adjustLabelsAndSouthPorts(nodeID, sizeDelta, flexibility)
			}
		}
	}
}

func (placer *networkSimplexPlacement) edgeWeight(edgeID layered.LEdgeID) float64 {
	priority := basegraph.Get(&placer.arena.Edge(edgeID).Properties, layered.PriorityStraightness)
	if priority < 1 {
		priority = 1
	}
	return float64(priority) * placer.edgeTypeWeight(
		placer.arena.Node(placer.arena.EdgeSourceNode(edgeID)).Type,
		placer.arena.Node(placer.arena.EdgeTargetNode(edgeID)).Type,
	)
}

func (*networkSimplexPlacement) edgeTypeWeight(first, second layered.NodeType) float64 {
	if first == layered.NormalNode && second == layered.NormalNode {
		return networkSimplexEdgeWeightBase
	}
	if first == layered.NormalNode || second == layered.NormalNode {
		return 2 * networkSimplexEdgeWeightBase
	}
	return 8 * networkSimplexEdgeWeightBase
}

func (placer *networkSimplexPlacement) isHandledEdge(edgeID layered.LEdgeID) bool {
	return !placer.arena.EdgeIsSelfLoop(edgeID) && !placer.arena.EdgeIsInLayer(edgeID)
}

func (placer *networkSimplexPlacement) nodeFlexibility(nodeID layered.LNodeID) layered.NodeFlexibility {
	if flexibility, found := basegraph.TryGet(
		&placer.arena.Node(nodeID).Properties, layered.NetworkSimplexNodeFlexibility,
	); found {
		return flexibility
	}
	return basegraph.Get(
		&placer.arena.Graph(placer.arena.NodeGraph(nodeID)).Properties,
		layered.NetworkSimplexNodeFlexibilityDefault,
	)
}

func (placer *networkSimplexPlacement) isFlexibleNode(nodeID layered.LNodeID) bool {
	node := placer.arena.Node(nodeID)
	if node.Type != layered.NormalNode || len(node.Ports) <= 1 {
		return false
	}
	constraints := basegraph.Get(&node.Properties, layered.PortConstraintsOption)
	// The Go LabelAndNodeSizeProcessor records the original constraint before
	// materializing FIXED_POS. ELK's adapter fixes coordinates without replacing
	// the property, so consult the saved value to preserve the Java decision.
	if original, found := basegraph.TryGet(&node.Properties, layered.OriginalPortConstraints); found {
		constraints = original
	}
	if constraints.IsPositionFixed() {
		return false
	}
	flexibility := placer.nodeFlexibility(nodeID)
	if flexibility == layered.NodeFlexibilityNone {
		return false
	}
	if !flexibility.IsFlexibleSizeWhereSpacePermits() {
		portSpacing := layered.IndividualOrDefault(placer.arena, nodeID, &layered.SpacingPortPort)
		surrounding := placer.portSurrounding(nodeID)
		for _, side := range []layered.PortSide{layered.WestSide, layered.EastSide} {
			ports := placer.arena.NodePortSideView(nodeID, side)
			required := surrounding.Top + surrounding.Bottom + float64(len(ports)-1)*portSpacing
			if required > node.Size.Y {
				return false
			}
		}
	}
	return true
}

func (placer *networkSimplexPlacement) portSurrounding(nodeID layered.LNodeID) geometry.Spacing {
	if individual, found := basegraph.TryGet(
		&placer.arena.Node(nodeID).Properties, layered.SpacingIndividual,
	); found {
		if value, explicit := basegraph.TryGet(
			&individual.Properties, layered.SpacingPortsSurrounding,
		); explicit {
			return value
		}
	}
	return basegraph.Get(
		&placer.arena.Graph(placer.arena.NodeGraph(nodeID)).Properties,
		layered.SpacingPortsSurrounding,
	)
}

func (placer *networkSimplexPlacement) adjustLabelsAndSouthPorts(
	nodeID layered.LNodeID,
	sizeDelta float64,
	flexibility layered.NodeFlexibility,
) {
	node := placer.arena.Node(nodeID)
	placement := basegraph.Get(&node.Properties, nodespacing.NodeLabelsPlacement)
	for _, labelID := range node.Labels {
		if containsString(placement, "V_BOTTOM") {
			placer.arena.Label(labelID).Position.Y += sizeDelta
		} else if containsString(placement, "V_CENTER") {
			placer.arena.Label(labelID).Position.Y += sizeDelta / 2
		}
	}
	if flexibility.IsFlexibleSizeWhereSpacePermits() {
		for _, portID := range placer.arena.NodePortSideView(nodeID, layered.SouthSide) {
			placer.arena.Port(portID).Position.Y += sizeDelta
		}
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

const (
	networkSimplexVisited  = -1
	networkSimplexOther    = 0
	networkSimplexJunction = 2
)

func (placer *networkSimplexPlacement) preferStraightEdges() {
	placer.nodeState = make([]int, len(placer.nodeReps))
	placer.twoPaths = nil
	for _, layerID := range placer.arena.Graph(placer.graphID).Layers {
		for _, nodeID := range placer.arena.Layer(layerID).Nodes {
			placer.nodeState[placer.arena.Node(nodeID).ID] = placer.nodeStraighteningState(nodeID)
		}
	}
	placer.markEdgeCrossings()
	for _, path := range placer.identifyPaths() {
		if len(path) <= 1 {
			continue
		}
		if len(path) == 2 {
			path = placer.orderTwoPath(path)
			center := placer.arena.EdgeTargetNode(path[0])
			if !placer.isFlexibleNode(center) {
				placer.twoPaths = append(placer.twoPaths, path)
			}
			continue
		}
		if placer.pathContainsLongEdgeDummy(path) ||
			placer.pathContainsFlexibleResizableNode(path) {
			continue
		}
		for index, edgeID := range path {
			rep := &placer.edgeReps[placer.arena.Edge(edgeID).ID]
			weight := placer.edgeTypeWeight(layered.LongEdgeNode, layered.LongEdgeNode)
			if index == 0 || index == len(path)-1 {
				weight = placer.edgeTypeWeight(layered.NormalNode, layered.LongEdgeNode)
			}
			weight *= networkSimplexLongEdgeVsPathFactor
			left := placer.network.Edge(rep.left)
			left.Weight = math.Max(left.Weight, left.Weight+(weight-left.Weight))
			right := placer.network.Edge(rep.right)
			right.Weight = math.Max(right.Weight, right.Weight+(weight-right.Weight))
		}
	}
}

func (placer *networkSimplexPlacement) postProcessTwoPaths() {
	queue := append([][]layered.LEdgeID(nil), placer.twoPaths...)
	stack := make([][]layered.LEdgeID, 0)
	for len(queue) != 0 {
		path := queue[0]
		queue = queue[1:]
		if placer.improveTwoPath(path, true) {
			stack = append(stack, path)
		}
	}
	for len(stack) != 0 {
		last := len(stack) - 1
		placer.improveTwoPath(stack[last], false)
		stack = stack[:last]
	}
}

func (placer *networkSimplexPlacement) improveTwoPath(
	path []layered.LEdgeID,
	probe bool,
) bool {
	leftEdge := placer.edgeReps[placer.arena.Edge(path[0]).ID]
	rightEdge := placer.edgeReps[placer.arena.Edge(path[1]).ID]
	if leftEdge.isStraight(placer.network) && rightEdge.isStraight(placer.network) {
		return false
	}
	centerOrigin := placer.network.Node(placer.network.Edge(leftEdge.right).Target).Origin
	centerNode, isNode := centerOrigin.(layered.LNodeID)
	if !isNode {
		return false
	}
	rep := &placer.nodeReps[placer.arena.Node(centerNode).ID]
	layerID := *placer.arena.Node(centerNode).Layer
	nodes := placer.arena.Layer(layerID).Nodes
	nodeIndex := placer.arena.NodeIndexInLayer(centerNode)
	aboveDistance := math.Inf(1)
	if nodeIndex > 0 {
		above := nodes[nodeIndex-1]
		aboveRep := placer.nodeReps[placer.arena.Node(above).ID]
		spacing := math.Ceil(layered.VerticalSpacing(placer.arena, above, centerNode))
		aboveDistance = float64(placer.network.Node(rep.head).Layer) -
			placer.arena.Node(centerNode).Margin.Top -
			(float64(placer.network.Node(aboveRep.head).Layer) +
				placer.arena.Node(above).Size.Y + placer.arena.Node(above).Margin.Bottom) -
			spacing
	}
	belowDistance := math.Inf(1)
	if nodeIndex < len(nodes)-1 {
		below := nodes[nodeIndex+1]
		belowRep := placer.nodeReps[placer.arena.Node(below).ID]
		spacing := math.Ceil(layered.VerticalSpacing(placer.arena, below, centerNode))
		belowDistance = float64(placer.network.Node(belowRep.head).Layer) -
			placer.arena.Node(below).Margin.Top -
			(float64(placer.network.Node(rep.head).Layer) +
				placer.arena.Node(centerNode).Size.Y + placer.arena.Node(centerNode).Margin.Bottom) -
			spacing
	}
	if probe && fuzzyEqual(aboveDistance, belowDistance, networkSimplexEpsilon) {
		return true
	}

	a := placer.networkEdgeLength(leftEdge.left)
	b := -placer.networkEdgeLength(leftEdge.right)
	c := -placer.networkEdgeLength(rightEdge.left)
	d := placer.networkEdgeLength(rightEdge.right)
	leftNotStraight := leftEdge.notStraightBy(placer.network)
	rightNotStraight := rightEdge.notStraightBy(placer.network)
	caseD := leftNotStraight > 0 && rightNotStraight < 0
	caseC := leftNotStraight < 0 && rightNotStraight > 0
	leftLeft := placer.network.Edge(leftEdge.left)
	leftRight := placer.network.Edge(leftEdge.right)
	rightLeft := placer.network.Edge(rightEdge.left)
	rightRight := placer.network.Edge(rightEdge.right)
	caseB := placer.network.Node(leftLeft.Target).Layer+leftRight.Delta <
		placer.network.Node(rightRight.Target).Layer+rightLeft.Delta
	caseA := placer.network.Node(leftLeft.Target).Layer+leftRight.Delta >
		placer.network.Node(rightRight.Target).Layer+rightLeft.Delta
	move := 0
	if !caseD && !caseC {
		if caseA {
			if aboveDistance+float64(c) > 0 {
				move = c
			} else if belowDistance-float64(a) > 0 {
				move = a
			}
		} else if caseB {
			if aboveDistance+float64(b) > 0 {
				move = b
			} else if belowDistance-float64(d) > 0 {
				move = d
			}
		}
	}
	placer.network.Node(rep.head).Layer += move
	if rep.isFlexible {
		placer.network.Node(rep.tail).Layer += move
	}
	return false
}

func (placer *networkSimplexPlacement) networkEdgeLength(edgeID networksimplex.EdgeID) int {
	edge := placer.network.Edge(edgeID)
	difference := placer.network.Node(edge.Source).Layer - placer.network.Node(edge.Target).Layer
	if difference < 0 {
		difference = -difference
	}
	return difference - edge.Delta
}

func (placer *networkSimplexPlacement) identifyPaths() [][]layered.LEdgeID {
	paths := make([][]layered.LEdgeID, 0)
	for _, layerID := range placer.arena.Graph(placer.graphID).Layers {
		for _, junction := range placer.arena.Layer(layerID).Nodes {
			if placer.nodeState[placer.arena.Node(junction).ID] != networkSimplexJunction {
				continue
			}
			for _, edgeID := range placer.arena.NodeConnectedEdges(junction) {
				if !placer.isHandledEdge(edgeID) {
					continue
				}
				path := placer.followPath(edgeID, junction, nil)
				if len(path) > 1 {
					paths = append(paths, path)
				}
			}
		}
	}
	return paths
}

func (placer *networkSimplexPlacement) followPath(
	edgeID layered.LEdgeID,
	current layered.LNodeID,
	path []layered.LEdgeID,
) []layered.LEdgeID {
	other := placer.arena.EdgeOtherNode(edgeID, current)
	path = append(path, edgeID)
	state := placer.nodeState[placer.arena.Node(other).ID]
	if state == networkSimplexVisited || state == networkSimplexJunction ||
		placer.crossing[placer.arena.Edge(edgeID).ID] {
		return path
	}
	placer.nodeState[placer.arena.Node(other).ID] = networkSimplexVisited
	for _, incident := range placer.arena.NodeConnectedEdges(other) {
		if incident == edgeID || !placer.isHandledEdge(incident) {
			continue
		}
		return placer.followPath(incident, other, path)
	}
	return path
}

func (placer *networkSimplexPlacement) nodeStraighteningState(nodeID layered.LNodeID) int {
	incoming, outgoing := 0, 0
	for _, portID := range placer.arena.Node(nodeID).Ports {
		for _, edgeID := range placer.arena.Port(portID).IncomingEdges {
			if !placer.arena.EdgeIsSelfLoop(edgeID) {
				incoming++
			}
		}
		for _, edgeID := range placer.arena.Port(portID).OutgoingEdges {
			if !placer.arena.EdgeIsSelfLoop(edgeID) {
				outgoing++
			}
		}
		if incoming > 1 || outgoing > 1 {
			return networkSimplexJunction
		}
	}
	if incoming+outgoing == 1 {
		return networkSimplexJunction
	}
	return networkSimplexOther
}

func (placer *networkSimplexPlacement) markEdgeCrossings() {
	placer.crossing = make([]bool, len(placer.edgeReps))
	layers := placer.arena.Graph(placer.graphID).Layers
	for index := 0; index+1 < len(layers); index++ {
		placer.markCrossingsBetween(layers[index], layers[index+1])
	}
}

func (placer *networkSimplexPlacement) markCrossingsBetween(
	leftLayer, rightLayer layered.LayerID,
) {
	openEdges := make([]layered.LEdgeID, 0)
	for _, nodeID := range placer.arena.Layer(leftLayer).Nodes {
		for _, portID := range placer.arena.NodePortSideView(nodeID, layered.EastSide) {
			for _, edgeID := range placer.arena.Port(portID).OutgoingEdges {
				if placer.arena.EdgeIsInLayer(edgeID) || placer.arena.EdgeIsSelfLoop(edgeID) ||
					placer.arena.Node(placer.arena.EdgeTargetNode(edgeID)).Layer == nil ||
					*placer.arena.Node(placer.arena.EdgeTargetNode(edgeID)).Layer != rightLayer {
					continue
				}
				openEdges = append(openEdges, edgeID)
			}
		}
	}
	rightNodes := placer.arena.Layer(rightLayer).Nodes
	for nodeIndex := len(rightNodes) - 1; nodeIndex >= 0; nodeIndex-- {
		for _, portID := range placer.arena.NodePortSideView(rightNodes[nodeIndex], layered.WestSide) {
			for _, edgeID := range placer.arena.Port(portID).IncomingEdges {
				if placer.arena.EdgeIsInLayer(edgeID) || placer.arena.EdgeIsSelfLoop(edgeID) ||
					placer.arena.Node(placer.arena.EdgeSourceNode(edgeID)).Layer == nil ||
					*placer.arena.Node(placer.arena.EdgeSourceNode(edgeID)).Layer != leftLayer {
					continue
				}
				if len(openEdges) == 0 {
					continue
				}
				index := len(openEdges) - 1
				last := openEdges[index]
				for last != edgeID && index > 0 {
					placer.crossing[placer.arena.Edge(last).ID] = true
					placer.crossing[placer.arena.Edge(edgeID).ID] = true
					index--
					last = openEdges[index]
				}
				// This deliberately retains ELK's ListIterator.hasPrevious guard:
				// an edge at index zero is not removed.
				if index > 0 {
					copy(openEdges[index:], openEdges[index+1:])
					openEdges = openEdges[:len(openEdges)-1]
				}
			}
		}
	}
}

func (placer *networkSimplexPlacement) pathContainsLongEdgeDummy(path []layered.LEdgeID) bool {
	if len(path) == 0 {
		return false
	}
	if placer.arena.Node(placer.arena.EdgeSourceNode(path[0])).Type == layered.LongEdgeNode {
		return true
	}
	for _, edgeID := range path {
		if placer.arena.Node(placer.arena.EdgeTargetNode(edgeID)).Type == layered.LongEdgeNode {
			return true
		}
	}
	return false
}

func (placer *networkSimplexPlacement) pathContainsFlexibleResizableNode(
	path []layered.LEdgeID,
) bool {
	if len(path) == 0 {
		return false
	}
	if placer.nodeFlexibility(placer.arena.EdgeSourceNode(path[0])).IsFlexibleSizeWhereSpacePermits() {
		return true
	}
	for _, edgeID := range path {
		if placer.nodeFlexibility(placer.arena.EdgeTargetNode(edgeID)).IsFlexibleSizeWhereSpacePermits() {
			return true
		}
	}
	return false
}

func (placer *networkSimplexPlacement) orderTwoPath(path []layered.LEdgeID) []layered.LEdgeID {
	if len(path) != 2 {
		panic("network simplex: only a two-path may be ordered")
	}
	if placer.arena.EdgeTargetNode(path[0]) != placer.arena.EdgeSourceNode(path[1]) {
		path[0], path[1] = path[1], path[0]
	}
	return path
}

func fuzzyEqual(first, second, epsilon float64) bool {
	return first == second || math.Abs(first-second) <= epsilon
}
