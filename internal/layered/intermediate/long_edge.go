// Copyright (c) 2010, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"math"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// SplitLongEdges makes the layering proper by inserting one dummy into each
// intervening layer of every long edge.
func SplitLongEdges(arena *layered.LGraphArena, graphID layered.LGraphID) {
	layers := arena.Graph(graphID).Layers
	if len(layers) <= 2 {
		return
	}
	for layerIndex := 0; layerIndex+1 < len(layers); layerIndex++ {
		layerID, nextLayerID := layers[layerIndex], layers[layerIndex+1]
		// Java's enhanced loops observe the layer's node count dynamically. A
		// dummy is added to nextLayer, never to layerID, so a direct range here
		// has the same behavior.
		for _, nodeID := range arena.Layer(layerID).Nodes {
			for _, portID := range arena.Node(nodeID).Ports {
				for _, edgeID := range append([]layered.LEdgeID(nil), arena.Port(portID).OutgoingEdges...) {
					targetNode := arena.EdgeTargetNode(edgeID)
					targetLayer := arena.Node(targetNode).Layer
					if targetLayer == nil || *targetLayer == layerID || *targetLayer == nextLayerID {
						continue
					}
					if layerIndex+2 >= len(layers) {
						panic("layered: long edge points beyond the final layer")
					}
					dummyID := createLongEdgeDummy(arena, graphID, nextLayerID, edgeID)
					SplitEdgeAtDummy(arena, edgeID, dummyID)
				}
			}
		}
	}
}

func createLongEdgeDummy(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	layerID layered.LayerID,
	edgeID layered.LEdgeID,
) layered.LNodeID {
	dummyID := arena.AddNode(graphID)
	dummy := arena.Node(dummyID)
	dummy.Type = layered.LongEdgeNode
	basegraph.Set(&dummy.Properties, layered.OriginalElement, layered.LEdgeOrigin(edgeID))
	basegraph.Set(&dummy.Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedPosition)
	arena.SetNodeLayer(dummyID, &layerID)
	return dummyID
}

// SplitEdgeAtDummy reroutes edge into dummy and returns the new outgoing edge.
func SplitEdgeAtDummy(
	arena *layered.LGraphArena,
	edgeID layered.LEdgeID,
	dummyID layered.LNodeID,
) layered.LEdgeID {
	edge := arena.Edge(edgeID)
	if edge.Target == nil {
		panic("layered: cannot split an edge without a target")
	}
	oldTarget := *edge.Target
	thickness := basegraph.Get(&edge.Properties, layered.EdgeThickness)
	if thickness < 0 {
		thickness = 0
		basegraph.Set(&edge.Properties, layered.EdgeThickness, thickness)
	}
	dummy := arena.Node(dummyID)
	dummy.Size.Y = thickness
	portY := math.Floor(thickness / 2)

	input := arena.AddPort()
	arena.SetPortSide(input, layered.WestSide)
	arena.SetPortNode(input, &dummyID)
	arena.Port(input).Position.Y = portY
	output := arena.AddPort()
	arena.SetPortSide(output, layered.EastSide)
	arena.SetPortNode(output, &dummyID)
	arena.Port(output).Position.Y = portY

	arena.SetEdgeTarget(edgeID, &input)
	dummyEdgeID := arena.AddEdge()
	dummyEdge := arena.Edge(dummyEdgeID)
	dummyEdge.Properties.CopyFrom(&edge.Properties)
	basegraph.Unset(&dummyEdge.Properties, layered.JunctionPoints)
	arena.SetEdgeSource(dummyEdgeID, &output)
	arena.SetEdgeTarget(dummyEdgeID, &oldTarget)

	setLongEdgeDummyProperties(arena, dummyID, edgeID, dummyEdgeID)
	moveHeadLabels(arena, edgeID, dummyEdgeID)
	return dummyEdgeID
}

func setLongEdgeDummyProperties(
	arena *layered.LGraphArena,
	dummyID layered.LNodeID,
	inEdgeID, outEdgeID layered.LEdgeID,
) {
	dummy := arena.Node(dummyID)
	inSourceNode := arena.EdgeSourceNode(inEdgeID)
	outTargetNode := arena.EdgeTargetNode(outEdgeID)
	if arena.Node(inSourceNode).Type == layered.LongEdgeNode {
		basegraph.Set(&dummy.Properties, layered.LongEdgeSource,
			basegraph.Get(&arena.Node(inSourceNode).Properties, layered.LongEdgeSource))
		basegraph.Set(&dummy.Properties, layered.LongEdgeTarget,
			basegraph.Get(&arena.Node(inSourceNode).Properties, layered.LongEdgeTarget))
		basegraph.Set(&dummy.Properties, layered.LongEdgeHasLabelDummies,
			basegraph.Get(&arena.Node(inSourceNode).Properties, layered.LongEdgeHasLabelDummies))
		return
	}
	if arena.Node(inSourceNode).Type == layered.LabelNode {
		basegraph.Set(&dummy.Properties, layered.LongEdgeSource,
			basegraph.Get(&arena.Node(inSourceNode).Properties, layered.LongEdgeSource))
		basegraph.Set(&dummy.Properties, layered.LongEdgeTarget,
			basegraph.Get(&arena.Node(inSourceNode).Properties, layered.LongEdgeTarget))
		basegraph.Set(&dummy.Properties, layered.LongEdgeHasLabelDummies, true)
		return
	}
	if arena.Node(outTargetNode).Type == layered.LabelNode {
		basegraph.Set(&dummy.Properties, layered.LongEdgeSource,
			basegraph.Get(&arena.Node(outTargetNode).Properties, layered.LongEdgeSource))
		basegraph.Set(&dummy.Properties, layered.LongEdgeTarget,
			basegraph.Get(&arena.Node(outTargetNode).Properties, layered.LongEdgeTarget))
		basegraph.Set(&dummy.Properties, layered.LongEdgeHasLabelDummies, true)
		return
	}
	basegraph.Set(&dummy.Properties, layered.LongEdgeSource, *arena.Edge(inEdgeID).Source)
	basegraph.Set(&dummy.Properties, layered.LongEdgeTarget, *arena.Edge(outEdgeID).Target)
}

func moveHeadLabels(arena *layered.LGraphArena, oldEdgeID, newEdgeID layered.LEdgeID) {
	oldEdge, newEdge := arena.Edge(oldEdgeID), arena.Edge(newEdgeID)
	retained := oldEdge.Labels[:0]
	for _, labelID := range oldEdge.Labels {
		label := arena.Label(labelID)
		if basegraph.Get(&label.Properties, layered.EdgeLabelsPlacement) != layered.EdgeLabelPlacementHead {
			retained = append(retained, labelID)
			continue
		}
		newEdge.Labels = append(newEdge.Labels, labelID)
		if !basegraph.Has(&label.Properties, layered.EndLabelEdge) {
			basegraph.Set(&label.Properties, layered.EndLabelEdge, oldEdgeID)
		}
	}
	oldEdge.Labels = retained
}

// JoinLongEdges removes long-edge dummies after routing. The dummy's Layer
// pointer intentionally remains set, matching ELK's postcondition.
func JoinLongEdges(arena *layered.LGraphArena, graphID layered.LGraphID) {
	addUnnecessary := basegraph.Get(&arena.Graph(graphID).Properties, layered.UnnecessaryBendpoints)
	for _, layerID := range arena.Graph(graphID).Layers {
		nodes := arena.Layer(layerID).Nodes
		retained := nodes[:0]
		for _, nodeID := range nodes {
			if arena.Node(nodeID).Type == layered.LongEdgeNode {
				JoinLongEdgeAt(arena, nodeID, addUnnecessary)
				continue
			}
			retained = append(retained, nodeID)
		}
		arena.Layer(layerID).Nodes = retained
	}
}

// JoinLongEdgeAt joins matching west-incoming and east-outgoing edges.
func JoinLongEdgeAt(arena *layered.LGraphArena, dummyID layered.LNodeID, addUnnecessary bool) {
	west := arena.NodePortsOnSide(dummyID, layered.WestSide)
	east := arena.NodePortsOnSide(dummyID, layered.EastSide)
	if len(west) == 0 || len(east) == 0 {
		panic("layered: long-edge dummy has no west/east ports")
	}
	inputEdges := arena.Port(west[0]).IncomingEdges
	outputEdges := arena.Port(east[0]).OutgoingEdges
	if len(outputEdges) < len(inputEdges) {
		panic("layered: long-edge dummy edge lists are inconsistent")
	}
	firstPort := arena.Port(arena.Node(dummyID).Ports[0])
	unnecessary := arena.Node(dummyID).Position.Add(firstPort.Position).Add(firstPort.Anchor)

	for len(arena.Port(west[0]).IncomingEdges) > 0 {
		survivingID := arena.Port(west[0]).IncomingEdges[0]
		droppedID := arena.Port(east[0]).OutgoingEdges[0]
		dropped := arena.Edge(droppedID)
		target := *dropped.Target
		targetIndex := indexEdge(arena.Port(target).IncomingEdges, droppedID)
		arena.SetEdgeTargetAt(survivingID, &target, targetIndex)
		arena.SetEdgeSource(droppedID, nil)
		arena.SetEdgeTarget(droppedID, nil)

		surviving := arena.Edge(survivingID)
		if addUnnecessary {
			surviving.BendPoints = append(surviving.BendPoints, unnecessary)
		}
		surviving.BendPoints = append(surviving.BendPoints, dropped.BendPoints...)
		if droppedSplineSegments, found := basegraph.TryGet(&dropped.Properties, layered.SplineControlPointSegments); found {
			survivingSplineSegments, _ := basegraph.TryGet(&surviving.Properties, layered.SplineControlPointSegments)
			survivingSplineSegments = append(survivingSplineSegments, droppedSplineSegments...)
			basegraph.Set(&surviving.Properties, layered.SplineControlPointSegments, survivingSplineSegments)
		}
		if droppedCompactionSegments, found := basegraph.TryGet(
			&dropped.Properties, layered.SplineCompactionSegments,
		); found {
			survivingCompactionSegments, _ := basegraph.TryGet(
				&surviving.Properties, layered.SplineCompactionSegments)
			survivingCompactionSegments = append(
				survivingCompactionSegments, droppedCompactionSegments...)
			basegraph.Set(&surviving.Properties, layered.SplineCompactionSegments,
				survivingCompactionSegments)
		}
		surviving.Labels = append(surviving.Labels, dropped.Labels...)
		if droppedJunctions, found := basegraph.TryGet(&dropped.Properties, layered.JunctionPoints); found && droppedJunctions != nil {
			survivingJunctions, _ := basegraph.TryGet(&surviving.Properties, layered.JunctionPoints)
			survivingJunctions = append(survivingJunctions, droppedJunctions...)
			basegraph.Set(&surviving.Properties, layered.JunctionPoints, survivingJunctions)
		}
	}
}

func indexEdge(edges []layered.LEdgeID, wanted layered.LEdgeID) int {
	for index, edgeID := range edges {
		if edgeID == wanted {
			return index
		}
	}
	panic("layered: edge is missing from adjacency list")
}
