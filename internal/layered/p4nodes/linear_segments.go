// Copyright (c) 2010, 2015 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"math"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// LinearSegmentsNodePlacer is the ELK 0.8.1 Sander-style placement phase. A
// segment consists of one regular node or a chain of long-edge / north-south
// port dummies. The dependency order is placed first, then balanced by the
// alternating pendulum and rubber iterations from the Java implementation.
type LinearSegmentsNodePlacer struct {
	segments       []*linearSegment
	inputPriority  map[layered.LNodeID]int
	outputPriority map[layered.LNodeID]int
}

type linearSegment struct {
	nodes            []layered.LNodeID
	id               int
	indexInLastLayer int
	lastLayer        int
	deflection       float64
	weight           int
	reference        *linearSegment
	nodeType         layered.NodeType
}

func (segment *linearSegment) region() *linearSegment {
	result := segment
	for result.reference != nil {
		result = result.reference
	}
	return result
}

func (segment *linearSegment) split(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	newID int,
) *linearSegment {
	index := 0
	for index < len(segment.nodes) && segment.nodes[index] != nodeID {
		index++
	}
	result := &linearSegment{
		nodes:            append([]layered.LNodeID(nil), segment.nodes[index:]...),
		id:               newID,
		indexInLastLayer: -1,
		lastLayer:        -1,
	}
	segment.nodes = segment.nodes[:index]
	for _, moved := range result.nodes {
		arena.Node(moved).ID = newID
	}
	return result
}

func (*LinearSegmentsNodePlacer) ProcessorConfiguration(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) layered.ProcessorConfiguration {
	var configuration layered.ProcessorConfiguration
	if basegraph.Get(&arena.Graph(graphID).Properties, layered.GraphProperties)&layered.GraphPropertyExternalPorts != 0 {
		configuration.AddBefore(layered.PhaseEdgeRouting, layered.HierarchicalPortPositionProcessor)
	}
	return configuration
}

func (placer *LinearSegmentsNodePlacer) Process(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) error {
	placer.sortLinearSegments(arena, graphID)
	placer.createUnbalancedPlacement(arena, graphID)
	placer.balancePlacement(arena, graphID)
	placer.postProcess(arena, graphID)
	placer.segments = nil
	placer.inputPriority = nil
	placer.outputPriority = nil
	return nil
}

func (placer *LinearSegmentsNodePlacer) sortLinearSegments(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) {
	placer.inputPriority = make(map[layered.LNodeID]int)
	placer.outputPriority = make(map[layered.LNodeID]int)
	segmentList := make([]*linearSegment, 0)
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			node := arena.Node(nodeID)
			node.ID = -1
			incoming, outgoing := math.MinInt, math.MinInt
			for _, portID := range node.Ports {
				port := arena.Port(portID)
				for _, edgeID := range port.IncomingEdges {
					incoming = max(incoming, basegraph.Get(&arena.Edge(edgeID).Properties, layered.PriorityStraightness))
				}
				for _, edgeID := range port.OutgoingEdges {
					outgoing = max(outgoing, basegraph.Get(&arena.Edge(edgeID).Properties, layered.PriorityStraightness))
				}
			}
			placer.inputPriority[nodeID] = incoming
			placer.outputPriority[nodeID] = outgoing
		}
	}

	nextID := 0
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			if arena.Node(nodeID).ID < 0 {
				segment := &linearSegment{id: nextID, indexInLastLayer: -1, lastLayer: -1}
				nextID++
				placer.fillSegment(arena, nodeID, segment)
				segmentList = append(segmentList, segment)
			}
		}
	}

	outgoing := make([][]*linearSegment, len(segmentList))
	incomingCount := make([]int, len(segmentList))
	placer.createDependencyGraphEdges(arena, graphID, &segmentList, &outgoing, &incomingCount)

	noIncoming := make([]*linearSegment, 0)
	for index, segment := range segmentList {
		if incomingCount[index] == 0 {
			noIncoming = append(noIncoming, segment)
		}
	}
	newRanks := make([]int, len(segmentList))
	nextRank := 0
	for len(noIncoming) > 0 {
		segment := noIncoming[0]
		noIncoming = noIncoming[1:]
		newRanks[segment.id] = nextRank
		nextRank++
		for len(outgoing[segment.id]) > 0 {
			target := outgoing[segment.id][0]
			outgoing[segment.id] = outgoing[segment.id][1:]
			incomingCount[target.id]--
			if incomingCount[target.id] == 0 {
				noIncoming = append(noIncoming, target)
			}
		}
	}

	placer.segments = make([]*linearSegment, len(segmentList))
	for oldID, segment := range segmentList {
		rank := newRanks[oldID]
		placer.segments[rank] = segment
		segment.id = rank
		for _, nodeID := range segment.nodes {
			arena.Node(nodeID).ID = rank
		}
	}
}

func (placer *LinearSegmentsNodePlacer) createDependencyGraphEdges(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	segmentList *[]*linearSegment,
	outgoing *[][]*linearSegment,
	incomingCount *[]int,
) {
	nextID := len(*segmentList)
	layerIndex := 0
	for _, layerID := range arena.Graph(graphID).Layers {
		nodes := arena.Layer(layerID).Nodes
		if len(nodes) == 0 {
			continue
		}
		var previous *layered.LNodeID
		for index, currentNode := range nodes {
			currentSegment := (*segmentList)[arena.Node(currentNode).ID]
			if currentSegment.indexInLastLayer >= 0 {
				var cycleSegment *linearSegment
				for _, cycleNode := range nodes[index+1:] {
					candidate := (*segmentList)[arena.Node(cycleNode).ID]
					if candidate.lastLayer == currentSegment.lastLayer &&
						candidate.indexInLastLayer < currentSegment.indexInLastLayer {
						cycleSegment = candidate
						break
					}
				}
				if cycleSegment != nil {
					if previous != nil {
						oldID := arena.Node(currentNode).ID
						(*incomingCount)[oldID]--
						previousID := arena.Node(*previous).ID
						(*outgoing)[previousID] = removeLinearSegment((*outgoing)[previousID], currentSegment)
					}
					currentSegment = currentSegment.split(arena, currentNode, nextID)
					nextID++
					*segmentList = append(*segmentList, currentSegment)
					*outgoing = append(*outgoing, nil)
					if previous != nil {
						previousID := arena.Node(*previous).ID
						(*outgoing)[previousID] = append((*outgoing)[previousID], currentSegment)
						*incomingCount = append(*incomingCount, 1)
					} else {
						*incomingCount = append(*incomingCount, 0)
					}
				}
			}

			if index+1 < len(nodes) {
				nextNode := nodes[index+1]
				nextSegment := (*segmentList)[arena.Node(nextNode).ID]
				currentID := arena.Node(currentNode).ID
				nextSegmentID := arena.Node(nextNode).ID
				(*outgoing)[currentID] = append((*outgoing)[currentID], nextSegment)
				(*incomingCount)[nextSegmentID]++
			}
			currentSegment.lastLayer = layerIndex
			currentSegment.indexInLastLayer = index
			copyID := currentNode
			previous = &copyID
		}
		layerIndex++
	}
}

func removeLinearSegment(segments []*linearSegment, target *linearSegment) []*linearSegment {
	for index, segment := range segments {
		if segment == target {
			return append(segments[:index], segments[index+1:]...)
		}
	}
	return segments
}

func (placer *LinearSegmentsNodePlacer) fillSegment(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	segment *linearSegment,
) bool {
	node := arena.Node(nodeID)
	if node.ID >= 0 {
		return false
	}
	node.ID = segment.id
	segment.nodes = append(segment.nodes, nodeID)
	segment.nodeType = node.Type
	if node.Type != layered.LongEdgeNode && node.Type != layered.NorthSouthPortNode {
		return true
	}
	for _, portID := range node.Ports {
		for _, edgeID := range arena.Port(portID).OutgoingEdges {
			targetNodeID := arena.EdgeTargetNode(edgeID)
			targetNode := arena.Node(targetNodeID)
			if node.Layer != nil && targetNode.Layer != nil && *node.Layer == *targetNode.Layer {
				continue
			}
			if targetNode.Type == layered.LongEdgeNode || targetNode.Type == layered.NorthSouthPortNode {
				if placer.fillSegment(arena, targetNodeID, segment) {
					return true
				}
			}
		}
	}
	return true
}

func (placer *LinearSegmentsNodePlacer) createUnbalancedPlacement(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) {
	graph := arena.Graph(graphID)
	nodeCount := make([]int, len(graph.Layers))
	recentNode := make([]*layered.LNodeID, len(graph.Layers))
	for _, segment := range placer.segments {
		uppermost := 0.0
		for _, nodeID := range segment.nodes {
			node := arena.Node(nodeID)
			layerIndex := arena.LayerIndex(*node.Layer)
			nodeCount[layerIndex]++
			spacing := basegraph.Get(&graph.Properties, layered.SpacingEdgeEdge)
			if recentNode[layerIndex] != nil {
				spacing = layered.VerticalSpacing(arena, *recentNode[layerIndex], nodeID)
			}
			uppermost = math.Max(uppermost, arena.Layer(*node.Layer).Size.Y+spacing)
		}
		for _, nodeID := range segment.nodes {
			node := arena.Node(nodeID)
			node.Position.Y = uppermost + node.Margin.Top
			layer := arena.Layer(*node.Layer)
			layer.Size.Y = uppermost + node.Margin.Top + node.Size.Y + node.Margin.Bottom
			copyID := nodeID
			recentNode[arena.LayerIndex(*node.Layer)] = &copyID
		}
	}
}

type linearSegmentsMode uint8

const (
	linearSegmentsForwardPendulum linearSegmentsMode = iota
	linearSegmentsBackwardPendulum
	linearSegmentsRubber
)

func (placer *LinearSegmentsNodePlacer) balancePlacement(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) {
	graph := arena.Graph(graphID)
	dampening := basegraph.Get(&graph.Properties, layered.LinearSegmentsDeflectionDampening)
	thoroughness := basegraph.Get(&graph.Properties, layered.Thoroughness)
	pendulumIterations := 4
	finalIterations := 3
	threshold := 20.0 / float64(thoroughness)
	ready := false
	mode := linearSegmentsForwardPendulum
	lastTotalDeflection := float64(math.MaxInt)
	for !(ready && finalIterations <= 0) {
		incoming := mode != linearSegmentsBackwardPendulum
		outgoing := mode != linearSegmentsForwardPendulum
		totalDeflection := 0.0
		for _, segment := range placer.segments {
			segment.reference = nil
			placer.calculateDeflection(arena, segment, incoming, outgoing, dampening)
			totalDeflection += math.Abs(segment.deflection)
		}
		for placer.mergeRegions(arena, graphID) {
		}
		for _, segment := range placer.segments {
			deflection := segment.region().deflection
			if deflection != 0 {
				for _, nodeID := range segment.nodes {
					arena.Node(nodeID).Position.Y += deflection
				}
			}
		}
		if mode == linearSegmentsForwardPendulum || mode == linearSegmentsBackwardPendulum {
			pendulumIterations--
			if pendulumIterations <= 0 && (totalDeflection < lastTotalDeflection || -pendulumIterations > thoroughness) {
				mode = linearSegmentsRubber
				lastTotalDeflection = float64(math.MaxInt)
			} else if mode == linearSegmentsForwardPendulum {
				mode = linearSegmentsBackwardPendulum
				lastTotalDeflection = totalDeflection
			} else {
				mode = linearSegmentsForwardPendulum
				lastTotalDeflection = totalDeflection
			}
		} else {
			ready = totalDeflection >= lastTotalDeflection || lastTotalDeflection-totalDeflection < threshold
			lastTotalDeflection = totalDeflection
			if ready {
				finalIterations--
			}
		}
	}
}

func (placer *LinearSegmentsNodePlacer) calculateDeflection(
	arena *layered.LGraphArena,
	segment *linearSegment,
	incoming, outgoing bool,
	dampening float64,
) {
	segmentDeflection := 0.0
	nodeWeightSum := 0
	for _, nodeID := range segment.nodes {
		node := arena.Node(nodeID)
		nodeDeflection := 0.0
		edgeWeightSum := 0
		inputPriority, outputPriority := math.MinInt, math.MinInt
		if incoming {
			inputPriority = placer.inputPriority[nodeID]
		}
		if outgoing {
			outputPriority = placer.outputPriority[nodeID]
		}
		minimumPriority := max(inputPriority, outputPriority)
		for _, portID := range node.Ports {
			port := arena.Port(portID)
			portPosition := node.Position.Y + port.Position.Y + port.Anchor.Y
			if outgoing {
				for _, edgeID := range port.OutgoingEdges {
					otherPortID := *arena.Edge(edgeID).Target
					otherNodeID := *arena.Port(otherPortID).Node
					if segment == placer.segments[arena.Node(otherNodeID).ID] {
						continue
					}
					otherPriority := max(placer.inputPriority[otherNodeID], placer.outputPriority[otherNodeID])
					priority := basegraph.Get(&arena.Edge(edgeID).Properties, layered.PriorityStraightness)
					if priority >= minimumPriority && priority >= otherPriority {
						otherPort := arena.Port(otherPortID)
						nodeDeflection += arena.Node(otherNodeID).Position.Y + otherPort.Position.Y + otherPort.Anchor.Y - portPosition
						edgeWeightSum++
					}
				}
			}
			if incoming {
				for _, edgeID := range port.IncomingEdges {
					otherPortID := *arena.Edge(edgeID).Source
					otherNodeID := *arena.Port(otherPortID).Node
					if segment == placer.segments[arena.Node(otherNodeID).ID] {
						continue
					}
					otherPriority := max(placer.inputPriority[otherNodeID], placer.outputPriority[otherNodeID])
					priority := basegraph.Get(&arena.Edge(edgeID).Properties, layered.PriorityStraightness)
					if priority >= minimumPriority && priority >= otherPriority {
						otherPort := arena.Port(otherPortID)
						nodeDeflection += arena.Node(otherNodeID).Position.Y + otherPort.Position.Y + otherPort.Anchor.Y - portPosition
						edgeWeightSum++
					}
				}
			}
		}
		if edgeWeightSum > 0 {
			segmentDeflection += nodeDeflection / float64(edgeWeightSum)
			nodeWeightSum++
		}
	}
	if nodeWeightSum > 0 {
		segment.deflection = dampening * segmentDeflection / float64(nodeWeightSum)
		segment.weight = nodeWeightSum
	} else {
		segment.deflection = 0
		segment.weight = 0
	}
}

func (placer *LinearSegmentsNodePlacer) mergeRegions(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) bool {
	changed := false
	threshold := 0.0001 * basegraph.Get(&arena.Graph(graphID).Properties, layered.SpacingNodeNode)
	for _, layerID := range arena.Graph(graphID).Layers {
		nodes := arena.Layer(layerID).Nodes
		if len(nodes) == 0 {
			continue
		}
		node1 := nodes[0]
		region1 := placer.segments[arena.Node(node1).ID].region()
		for _, node2 := range nodes[1:] {
			region2 := placer.segments[arena.Node(node2).ID].region()
			if region1 != region2 {
				first, second := arena.Node(node1), arena.Node(node2)
				spacing := layered.VerticalSpacing(arena, node1, node2)
				firstExtent := first.Position.Y + first.Size.Y + first.Margin.Bottom + region1.deflection + spacing
				secondExtent := second.Position.Y - second.Margin.Top + region2.deflection
				if firstExtent > secondExtent+threshold {
					weight := region1.weight + region2.weight
					region2.deflection = (float64(region2.weight)*region2.deflection + float64(region1.weight)*region1.deflection) / float64(weight)
					region2.weight = weight
					region1.reference = region2
					changed = true
				}
			}
			node1 = node2
			region1 = region2
		}
	}
	return changed
}

func (placer *LinearSegmentsNodePlacer) postProcess(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) {
	_ = graphID
	for _, segment := range placer.segments {
		roomAbove, roomBelow := float64(math.MaxInt), float64(math.MaxInt)
		for _, nodeID := range segment.nodes {
			node := arena.Node(nodeID)
			layerNodes := arena.Layer(*node.Layer).Nodes
			index := 0
			for layerNodes[index] != nodeID {
				index++
			}
			var currentAbove float64
			if index > 0 {
				neighborID := layerNodes[index-1]
				neighbor := arena.Node(neighborID)
				currentAbove = node.Position.Y - node.Margin.Top -
					(neighbor.Position.Y + neighbor.Size.Y + neighbor.Margin.Bottom + layered.VerticalSpacing(arena, nodeID, neighborID))
			} else {
				currentAbove = node.Position.Y - node.Margin.Top
			}
			roomAbove = math.Min(roomAbove, currentAbove)
			var currentBelow float64
			if index+1 < len(layerNodes) {
				neighborID := layerNodes[index+1]
				neighbor := arena.Node(neighborID)
				currentBelow = neighbor.Position.Y - neighbor.Margin.Top -
					(node.Position.Y + node.Size.Y + node.Margin.Bottom + layered.VerticalSpacing(arena, nodeID, neighborID))
			} else {
				currentBelow = 2 * node.Position.Y
			}
			roomBelow = math.Min(roomBelow, currentBelow)
		}

		minimumDisplacement := float64(math.MaxInt)
		found := false
		firstNode := arena.Node(segment.nodes[0])
		for _, portID := range firstNode.Ports {
			port := arena.Port(portID)
			position := firstNode.Position.Y + port.Position.Y + port.Anchor.Y
			for _, edgeID := range port.IncomingEdges {
				sourcePort := arena.Port(*arena.Edge(edgeID).Source)
				sourceNode := arena.Node(*sourcePort.Node)
				displacement := sourceNode.Position.Y + sourcePort.Position.Y + sourcePort.Anchor.Y - position
				room := roomBelow
				if displacement < 0 {
					room = roomAbove
				}
				if math.Abs(displacement) < math.Abs(minimumDisplacement) && math.Abs(displacement) < room {
					minimumDisplacement, found = displacement, true
				}
			}
		}
		lastNode := arena.Node(segment.nodes[len(segment.nodes)-1])
		for _, portID := range lastNode.Ports {
			port := arena.Port(portID)
			position := lastNode.Position.Y + port.Position.Y + port.Anchor.Y
			for _, edgeID := range port.OutgoingEdges {
				targetPort := arena.Port(*arena.Edge(edgeID).Target)
				targetNode := arena.Node(*targetPort.Node)
				displacement := targetNode.Position.Y + targetPort.Position.Y + targetPort.Anchor.Y - position
				room := roomBelow
				if displacement < 0 {
					room = roomAbove
				}
				if math.Abs(displacement) < math.Abs(minimumDisplacement) && math.Abs(displacement) < room {
					minimumDisplacement, found = displacement, true
				}
			}
		}
		if found && minimumDisplacement != 0 {
			for _, nodeID := range segment.nodes {
				arena.Node(nodeID).Position.Y += minimumDisplacement
			}
		}
	}
}
