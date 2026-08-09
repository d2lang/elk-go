// Copyright (c) 2011, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"math"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// ProcessHypernodes moves simple hypernodes onto the first orthogonal join
// point of their outgoing hyperedge, matching HypernodesProcessor.
func ProcessHypernodes(arena *layered.LGraphArena, graphID layered.LGraphID) {
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...) {
			node := arena.Node(nodeID)
			if !basegraph.Get(&node.Properties, core.Hypernode) || len(node.Ports) > 2 {
				continue
			}
			top, right, bottom, left := 0, 0, 0, 0
			for _, portID := range node.Ports {
				switch arena.Port(portID).Side {
				case layered.NorthSide:
					top++
				case layered.EastSide:
					right++
				case layered.SouthSide:
					bottom++
				case layered.WestSide:
					left++
				}
			}
			if top == 0 && bottom == 0 {
				moveHypernode(arena, graphID, nodeID, left <= right)
			}
		}
	}
}

func moveHypernode(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	nodeID layered.LNodeID,
	right bool,
) {
	const javaIntegerMaxValue = float64(1<<31 - 1)
	bendEdges := make([]layered.LEdgeID, 0)
	bendX := javaIntegerMaxValue
	diffX, diffY := javaIntegerMaxValue, javaIntegerMaxValue
	if right {
		bendX = arena.Graph(graphID).Size.X
		for _, portID := range append([]layered.LPortID(nil), arena.Node(nodeID).Ports...) {
			for _, edgeID := range append([]layered.LEdgeID(nil), arena.Port(portID).OutgoingEdges...) {
				points := arena.Edge(edgeID).BendPoints
				if len(points) == 0 {
					continue
				}
				first := points[0]
				if first.X < bendX {
					diffX = bendX - first.X
					diffY = javaIntegerMaxValue
					bendEdges = bendEdges[:0]
					bendX = first.X
				}
				if first.X <= bendX {
					bendEdges = append(bendEdges, edgeID)
					if len(points) > 1 {
						diffY = math.Min(diffY, math.Abs(points[1].Y-first.Y))
					}
				}
			}
		}
	} else {
		// ELK 0.8.1 initializes bendX to Integer.MAX_VALUE before this
		// greater-than search. Preserve that observable source behavior.
		for _, portID := range append([]layered.LPortID(nil), arena.Node(nodeID).Ports...) {
			for _, edgeID := range append([]layered.LEdgeID(nil), arena.Port(portID).IncomingEdges...) {
				points := arena.Edge(edgeID).BendPoints
				if len(points) == 0 {
					continue
				}
				last := points[len(points)-1]
				if last.X > bendX {
					diffX = last.X - bendX
					diffY = javaIntegerMaxValue
					bendEdges = bendEdges[:0]
					bendX = last.X
				}
				if last.X >= bendX {
					bendEdges = append(bendEdges, edgeID)
					if len(points) > 1 {
						diffY = math.Min(diffY,
							math.Abs(points[len(points)-2].Y-last.Y))
					}
				}
			}
		}
	}

	nodeSize := arena.Node(nodeID).Size
	if len(bendEdges) == 0 || diffX <= nodeSize.X/2 || diffY <= nodeSize.Y/2 {
		return
	}
	northPort := arena.AddPort()
	arena.SetPortNode(northPort, &nodeID)
	arena.SetPortSide(northPort, layered.NorthSide)
	arena.Port(northPort).Position.X = nodeSize.X / 2
	southPort := arena.AddPort()
	arena.SetPortNode(southPort, &nodeID)
	arena.SetPortSide(southPort, layered.SouthSide)
	arena.Port(southPort).Position = geometry.Point{X: nodeSize.X / 2, Y: nodeSize.Y}

	for _, edgeID := range bendEdges {
		points := arena.Edge(edgeID).BendPoints
		var eliminated, second geometry.Point
		if right {
			eliminated = points[0]
			arena.Edge(edgeID).BendPoints = points[1:]
			if len(arena.Edge(edgeID).BendPoints) == 0 {
				second = internalPortAbsoluteAnchor(arena, *arena.Edge(edgeID).Target)
			} else {
				second = arena.Edge(edgeID).BendPoints[0]
			}
			if second.Y >= eliminated.Y {
				arena.SetEdgeSource(edgeID, &southPort)
			} else {
				arena.SetEdgeSource(edgeID, &northPort)
			}
		} else {
			eliminated = points[len(points)-1]
			arena.Edge(edgeID).BendPoints = points[:len(points)-1]
			if len(arena.Edge(edgeID).BendPoints) == 0 {
				second = internalPortAbsoluteAnchor(arena, *arena.Edge(edgeID).Source)
			} else {
				second = arena.Edge(edgeID).BendPoints[len(arena.Edge(edgeID).BendPoints)-1]
			}
			if second.Y >= eliminated.Y {
				arena.SetEdgeTarget(edgeID, &southPort)
			} else {
				arena.SetEdgeTarget(edgeID, &northPort)
			}
		}
		if junctions, found := basegraph.TryGet(&arena.Edge(edgeID).Properties, layered.JunctionPoints); found {
			basegraph.Set(&arena.Edge(edgeID).Properties, layered.JunctionPoints,
				removeFirstPoint(junctions, eliminated))
		}
	}
	arena.Node(nodeID).Position.X = bendX - nodeSize.X/2
}

func internalPortAbsoluteAnchor(
	arena *layered.LGraphArena,
	portID layered.LPortID,
) geometry.Point {
	port := arena.Port(portID)
	position := port.Position.Add(port.Anchor)
	if port.Node != nil {
		position = position.Add(arena.Node(*port.Node).Position)
	}
	return position
}

func removeFirstPoint(points geometry.Chain, target geometry.Point) geometry.Chain {
	for index, point := range points {
		if point == target {
			return append(points[:index], points[index+1:]...)
		}
	}
	return points
}

// ProcessConstraints stores final layer and position identifiers on normal
// nodes for interactive-layout and generated-constraint output.
func ProcessConstraints(arena *layered.LGraphArena, graphID layered.LGraphID) {
	for layerIndex, layerID := range arena.Graph(graphID).Layers {
		positionIndex := 0
		for _, nodeID := range arena.Layer(layerID).Nodes {
			if arena.Node(nodeID).Type != layered.NormalNode {
				continue
			}
			basegraph.Set(&arena.Node(nodeID).Properties, layered.LayerIDOption, layerIndex)
			basegraph.Set(&arena.Node(nodeID).Properties, layered.PositionIDOption, positionIndex)
			positionIndex++
		}
	}
}
