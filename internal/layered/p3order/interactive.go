// Copyright (c) 2011, 2019 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"sort"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// InteractiveCrossingMinimizer orders nodes by their previous vertical
// positions and reconstructs previous long-edge positions at each layer.
type InteractiveCrossingMinimizer struct{}

func (*InteractiveCrossingMinimizer) ProcessorConfiguration(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) layered.ProcessorConfiguration {
	var configuration layered.ProcessorConfiguration
	configuration.
		AddBefore(layered.PhaseNodeOrdering, layered.LongEdgeSplitter).
		AddBefore(layered.PhaseNodePlacement, layered.InLayerConstraintProcessor).
		AddAfter(layered.PhaseEdgeRouting, layered.LongEdgeJoiner)
	if basegraph.Get(&arena.Graph(graphID).Properties, layered.GraphProperties)&layered.GraphPropertyNonFreePorts != 0 {
		configuration.AddBefore(layered.PhaseNodeOrdering, layered.PortListSorter)
	}
	return configuration
}

func (*InteractiveCrossingMinimizer) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	graph := arena.Graph(graphID)
	order := make([][]layered.LNodeID, len(graph.Layers))
	for layerIndex, layerID := range graph.Layers {
		arena.Layer(layerID).ID = layerIndex
		order[layerIndex] = append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...)
	}
	distributor := NewBarycenterPortDistributor(NodeRelativePortDistributor, len(order))
	InitializeBarycenterPortDistributor(distributor, arena, order)

	portCount := 0
	for layerIndex, layerID := range graph.Layers {
		layer := arena.Layer(layerID)
		horizontalPosition := 0.0
		nodeCount := 0
		for _, nodeID := range layer.Nodes {
			node := arena.Node(nodeID)
			if node.Position.X > 0 {
				horizontalPosition += node.Position.X + node.Size.X/2
				nodeCount++
			}
			for _, portID := range node.Ports {
				arena.Port(portID).ID = portCount
				portCount++
			}
		}
		if nodeCount > 0 {
			horizontalPosition /= float64(nodeCount)
		}

		positions := make([]float64, len(layer.Nodes))
		for index, nodeID := range layer.Nodes {
			arena.Node(nodeID).ID = index
			positions[index] = interactiveNodePosition(arena, nodeID, horizontalPosition)
			if arena.Node(nodeID).Type == layered.LongEdgeNode {
				basegraph.Set(&arena.Node(nodeID).Properties, layered.OriginalDummyNodePosition, positions[index])
			}
		}
		sort.SliceStable(layer.Nodes, func(first, second int) bool {
			firstID, secondID := layer.Nodes[first], layer.Nodes[second]
			comparison := compareInteractiveFloat(positions[arena.Node(firstID).ID], positions[arena.Node(secondID).ID])
			if comparison == 0 {
				for _, successor := range basegraph.Get(&arena.Node(firstID).Properties, layered.InLayerSuccessorConstraints) {
					if successor == secondID {
						return true
					}
				}
				for _, successor := range basegraph.Get(&arena.Node(secondID).Properties, layered.InLayerSuccessorConstraints) {
					if successor == firstID {
						return false
					}
				}
			}
			return comparison < 0
		})
		distributor.DistributePortsWhileSweeping(arena, order, layerIndex, true)
	}
	return nil
}
func interactiveNodePosition(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	horizontalPosition float64,
) float64 {
	node := arena.Node(nodeID)
	switch node.Type {
	case layered.LongEdgeNode:
		origin, found := basegraph.TryGet(&node.Properties, layered.OriginalElement)
		if found && origin.Kind == layered.OriginLEdge {
			edge := arena.Edge(origin.LEdge)
			points := basegraph.Get(&edge.Properties, layered.OriginalBendPoints).Clone()
			if basegraph.Get(&edge.Properties, layered.Reversed) {
				points = points.Reverse()
			}
			if source, sourceFound := basegraph.TryGet(&node.Properties, layered.LongEdgeSource); sourceFound {
				sourcePoint := interactiveAbsoluteAnchor(arena, source)
				if horizontalPosition <= sourcePoint.X {
					return sourcePoint.Y
				}
				points = append(geometry.Chain{sourcePoint}, points...)
			}
			if target, targetFound := basegraph.TryGet(&node.Properties, layered.LongEdgeTarget); targetFound {
				targetPoint := interactiveAbsoluteAnchor(arena, target)
				if targetPoint.X <= horizontalPosition {
					return targetPoint.Y
				}
				points = append(points, targetPoint)
			}
			if len(points) >= 2 {
				first, second := points[0], points[1]
				index := 2
				for second.X < horizontalPosition && index < len(points) {
					first, second = second, points[index]
					index++
				}
				return first.Y + (horizontalPosition-first.X)/(second.X-first.X)*(second.Y-first.Y)
			}
		}
	case layered.NorthSouthPortNode:
		if len(node.Ports) > 0 {
			origin, found := basegraph.TryGet(&arena.Port(node.Ports[0]).Properties, layered.OriginalElement)
			if found && origin.Kind == layered.OriginLPort {
				port := arena.Port(origin.LPort)
				if port.Node != nil {
					originNode := arena.Node(*port.Node)
					switch port.Side {
					case layered.NorthSide:
						return originNode.Position.Y
					case layered.SouthSide:
						return originNode.Position.Y + originNode.Size.Y
					}
				}
			}
		}
	}
	position := node.Position.Y
	if basegraph.Get(&arena.Graph(arena.NodeGraph(nodeID)).Properties, layered.InteractiveReferencePointOption) == layered.InteractiveReferenceCenter {
		position += node.Size.Y / 2
	}
	return position
}

func interactiveAbsoluteAnchor(arena *layered.LGraphArena, portID layered.LPortID) geometry.Point {
	port := arena.Port(portID)
	point := port.Position.Add(port.Anchor)
	if port.Node != nil {
		point = arena.Node(*port.Node).Position.Add(point)
	}
	return point
}

func compareInteractiveFloat(first, second float64) int {
	if first < second {
		return -1
	}
	if first > second {
		return 1
	}
	return 0
}
