// Copyright (c) 2011, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// PostprocessNorthSouthPorts ports ELK 0.8.1's
// NorthSouthPortPostprocessor. It removes the temporary in-layer dummies,
// reconnects their edges to the original ports, and adds the vertical routing
// segments (or records the dummy y coordinate for spline routing).
func PostprocessNorthSouthPorts(arena *layered.LGraphArena, graphID layered.LGraphID) {
	routing := basegraph.Get(&arena.Graph(graphID).Properties, layered.EdgeRoutingOption)
	for _, layerID := range arena.Graph(graphID).Layers {
		nodeSnapshot := append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...)
		for _, nodeID := range nodeSnapshot {
			node := arena.Node(nodeID)
			if node.Type != layered.NorthSouthPortNode {
				continue
			}

			if routing == layered.EdgeRoutingSplines {
				for _, portID := range node.Ports {
					if len(arena.Port(portID).IncomingEdges) != 0 {
						processSplineNorthSouthInput(arena, portID)
					}
					if len(arena.Port(portID).OutgoingEdges) != 0 {
						processSplineNorthSouthOutput(arena, portID)
					}
				}
			} else if origin, found := basegraph.TryGet(&node.Properties, layered.OriginalElement); found && origin.Kind == layered.OriginLEdge {
				processNorthSouthSelfLoop(arena, nodeID, origin.LEdge)
			} else {
				sameOriginPort := northSouthDummyPortsShareOrigin(arena, nodeID)
				for _, portID := range append([]layered.LPortID(nil), node.Ports...) {
					if len(arena.Port(portID).IncomingEdges) != 0 {
						processNorthSouthInput(arena, portID, sameOriginPort)
					}
					if len(arena.Port(portID).OutgoingEdges) != 0 {
						processNorthSouthOutput(arena, portID, sameOriginPort)
					}
				}
			}
			arena.SetNodeLayer(nodeID, nil)
		}
	}
}

func northSouthDummyPortsShareOrigin(arena *layered.LGraphArena, nodeID layered.LNodeID) bool {
	ports := arena.Node(nodeID).Ports
	if len(ports) < 2 {
		return false
	}
	previous, found := northSouthOriginPort(arena, ports[0])
	if !found {
		return false
	}
	for _, portID := range ports[1:] {
		current, currentFound := northSouthOriginPort(arena, portID)
		if !currentFound || current != previous {
			return false
		}
		previous = current
	}
	return true
}

func northSouthOriginPort(arena *layered.LGraphArena, dummyPortID layered.LPortID) (layered.LPortID, bool) {
	origin, found := basegraph.TryGet(&arena.Port(dummyPortID).Properties, layered.OriginalElement)
	return origin.LPort, found && origin.Kind == layered.OriginLPort
}

func mustNorthSouthOriginPort(arena *layered.LGraphArena, dummyPortID layered.LPortID) layered.LPortID {
	originPort, found := northSouthOriginPort(arena, dummyPortID)
	if !found {
		panic("layered: north/south dummy port has no LPort origin")
	}
	return originPort
}

func processNorthSouthInput(arena *layered.LGraphArena, inputPortID layered.LPortID, addJunction bool) {
	inputPort := arena.Port(inputPortID)
	originPortID := mustNorthSouthOriginPort(arena, inputPortID)
	x := northSouthAbsoluteAnchor(arena, originPortID).X
	y := arena.Node(*inputPort.Node).Position.Y
	for _, edgeID := range append([]layered.LEdgeID(nil), inputPort.IncomingEdges...) {
		arena.SetEdgeTarget(edgeID, &originPortID)
		arena.Edge(edgeID).BendPoints = append(arena.Edge(edgeID).BendPoints, geometry.Point{X: x, Y: y})
		if addJunction {
			appendNorthSouthJunction(arena, edgeID, geometry.Point{X: x, Y: y})
		}
	}
}

func processNorthSouthOutput(arena *layered.LGraphArena, outputPortID layered.LPortID, addJunction bool) {
	outputPort := arena.Port(outputPortID)
	originPortID := mustNorthSouthOriginPort(arena, outputPortID)
	x := northSouthAbsoluteAnchor(arena, originPortID).X
	y := arena.Node(*outputPort.Node).Position.Y
	for _, edgeID := range append([]layered.LEdgeID(nil), outputPort.OutgoingEdges...) {
		arena.SetEdgeSource(edgeID, &originPortID)
		bends := arena.Edge(edgeID).BendPoints
		bends = append(bends, geometry.Point{})
		copy(bends[1:], bends[:len(bends)-1])
		bends[0] = geometry.Point{X: x, Y: y}
		arena.Edge(edgeID).BendPoints = bends
		if addJunction {
			appendNorthSouthJunction(arena, edgeID, geometry.Point{X: x, Y: y})
		}
	}
}

func appendNorthSouthJunction(arena *layered.LGraphArena, edgeID layered.LEdgeID, point geometry.Point) {
	junctions, _ := basegraph.TryGet(&arena.Edge(edgeID).Properties, layered.JunctionPoints)
	junctions = append(junctions, point)
	basegraph.Set(&arena.Edge(edgeID).Properties, layered.JunctionPoints, junctions)
}

func processNorthSouthSelfLoop(
	arena *layered.LGraphArena,
	dummyID layered.LNodeID,
	edgeID layered.LEdgeID,
) {
	westPorts := arena.NodePortsOnSide(dummyID, layered.WestSide)
	eastPorts := arena.NodePortsOnSide(dummyID, layered.EastSide)
	if len(westPorts) == 0 || len(eastPorts) == 0 {
		panic("layered: north/south self-loop dummy is missing west/east ports")
	}
	inputPortID, outputPortID := westPorts[0], eastPorts[0]
	originInput := mustNorthSouthOriginPort(arena, inputPortID)
	originOutput := mustNorthSouthOriginPort(arena, outputPortID)
	arena.SetEdgeSource(edgeID, &originOutput)
	arena.SetEdgeTarget(edgeID, &originInput)
	nodePosition := arena.Node(dummyID).Position
	arena.Edge(edgeID).BendPoints = append(arena.Edge(edgeID).BendPoints,
		geometry.Point{X: northSouthAbsoluteAnchor(arena, originOutput).X, Y: nodePosition.Y},
		geometry.Point{X: northSouthAbsoluteAnchor(arena, originInput).X, Y: nodePosition.Y})
}

func processSplineNorthSouthInput(arena *layered.LGraphArena, inputPortID layered.LPortID) {
	inputPort := arena.Port(inputPortID)
	originPortID := mustNorthSouthOriginPort(arena, inputPortID)
	basegraph.Set(&arena.Port(originPortID).Properties, layered.SplineNorthSouthPortY,
		arena.Node(*inputPort.Node).Position.Y)
	for _, edgeID := range append([]layered.LEdgeID(nil), inputPort.IncomingEdges...) {
		arena.SetEdgeTarget(edgeID, &originPortID)
	}
}

func processSplineNorthSouthOutput(arena *layered.LGraphArena, outputPortID layered.LPortID) {
	outputPort := arena.Port(outputPortID)
	originPortID := mustNorthSouthOriginPort(arena, outputPortID)
	basegraph.Set(&arena.Port(originPortID).Properties, layered.SplineNorthSouthPortY,
		arena.Node(*outputPort.Node).Position.Y)
	for _, edgeID := range append([]layered.LEdgeID(nil), outputPort.OutgoingEdges...) {
		arena.SetEdgeSource(edgeID, &originPortID)
	}
}

func northSouthAbsoluteAnchor(arena *layered.LGraphArena, portID layered.LPortID) geometry.Point {
	port := arena.Port(portID)
	anchor := port.Position.Add(port.Anchor)
	if port.Node != nil {
		anchor = arena.Node(*port.Node).Position.Add(anchor)
	}
	return anchor
}
