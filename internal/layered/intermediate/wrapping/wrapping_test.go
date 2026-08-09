// Copyright (c) 2016 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package wrapping

import (
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestSingleEdgeManualCutWrapsPath(t *testing.T) {
	arena, graphID, nodes := pathGraph(6)
	graph := arena.Graph(graphID)
	basegraph.Set(&graph.Properties, layered.WrappingCuttingStrategy, layered.CuttingManual)
	basegraph.Set(&graph.Properties, layered.WrappingCuttingCuts, []int{3})
	basegraph.Set(&graph.Properties, layered.WrappingValidifyStrategy, layered.ValidifyGreedy)

	WrapSingleEdgeGraph(arena, graphID)
	if len(graph.Layers) != 3 {
		t.Fatalf("layers = %d, want 3", len(graph.Layers))
	}
	for layerIndex, want := range [][]layered.LNodeID{
		{nodes[0], nodes[3]},
		{nodes[1], nodes[4]},
		{nodes[2], nodes[5]},
	} {
		var normals []layered.LNodeID
		for _, nodeID := range arena.Layer(graph.Layers[layerIndex]).Nodes {
			if arena.Node(nodeID).Type == layered.NormalNode {
				normals = append(normals, nodeID)
			}
		}
		if len(normals) != 2 || normals[0] != want[0] || normals[1] != want[1] {
			t.Fatalf("layer %d normal nodes = %v, want %v", layerIndex, normals, want)
		}
	}
}

func TestMultiEdgeInserterCreatesPairedBreakingPoints(t *testing.T) {
	arena, graphID, _ := pathGraph(8)
	graph := arena.Graph(graphID)
	basegraph.Set(&graph.Properties, layered.WrappingCuttingStrategy, layered.CuttingARD)
	basegraph.Set(&graph.Properties, layered.WrappingMultiEdgeImproveCuts, false)

	InsertBreakingPoints(arena, graphID)
	starts, ends := 0, 0
	for _, layerID := range graph.Layers {
		for _, nodeID := range arena.Layer(layerID).Nodes {
			if IsBreakingPointStart(arena, nodeID) {
				starts++
			}
			if IsBreakingPointEnd(arena, nodeID) {
				ends++
			}
		}
	}
	if starts == 0 || starts != ends {
		t.Fatalf("breaking points = %d starts, %d ends", starts, ends)
	}
}

func TestBreakingPointRemoverKeepsCompactionSegmentsAligned(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	nodes := make([]layered.LNodeID, 4)
	for index := range nodes {
		nodes[index] = arena.AddNode(graphID)
		arena.Graph(graphID).LayerlessNodes = append(
			arena.Graph(graphID).LayerlessNodes, nodes[index])
	}
	newEdge := func(sourceNode, targetNode layered.LNodeID) layered.LEdgeID {
		source := arena.AddPort()
		arena.SetPortNode(source, &sourceNode)
		target := arena.AddPort()
		arena.SetPortNode(target, &targetNode)
		edge := arena.AddEdge()
		arena.SetEdgeSource(edge, &source)
		arena.SetEdgeTarget(edge, &target)
		return edge
	}
	nodeStart := newEdge(nodes[0], nodes[1])
	startEnd := newEdge(nodes[1], nodes[2])
	original := newEdge(nodes[2], nodes[3])
	info := &BPInfo{
		Start: nodes[1], End: nodes[2],
		NodeStartEdge: nodeStart, StartEndEdge: startEnd,
		OriginalEdge: original,
	}

	basegraph.Set(&arena.Edge(nodeStart).Properties,
		layered.SplineControlPointSegments,
		[]geometry.Chain{{{X: 1}}})
	basegraph.Set(&arena.Edge(startEnd).Properties,
		layered.SplineControlPointSegments,
		[]geometry.Chain{{{X: 2}}, {{X: 3}}})
	basegraph.Set(&arena.Edge(original).Properties,
		layered.SplineControlPointSegments,
		[]geometry.Chain{{{X: 4}}})
	basegraph.Set(&arena.Edge(nodeStart).Properties,
		layered.SplineCompactionSegments,
		[]layered.SplineCompactionSegment{{Rank: 1}})
	basegraph.Set(&arena.Edge(startEnd).Properties,
		layered.SplineCompactionSegments,
		[]layered.SplineCompactionSegment{{Rank: 2}, {Rank: 3}})
	basegraph.Set(&arena.Edge(original).Properties,
		layered.SplineCompactionSegments,
		[]layered.SplineCompactionSegment{{Rank: 4}})

	removeBreakingPoint(arena, info, layered.EdgeRoutingSplines)
	segments := basegraph.Get(&arena.Edge(original).Properties,
		layered.SplineCompactionSegments)
	if len(segments) != 4 {
		t.Fatalf("compaction segments = %d, want 4", len(segments))
	}
	for index, want := range []int{1, 3, 2, 4} {
		if segments[index].Rank != want {
			t.Fatalf("compaction segment %d rank = %d, want %d",
				index, segments[index].Rank, want)
		}
	}
	for _, edgeID := range []layered.LEdgeID{nodeStart, startEnd} {
		if basegraph.Has(&arena.Edge(edgeID).Properties,
			layered.SplineCompactionSegments) {
			t.Fatalf("split edge %d retained compaction metadata", edgeID)
		}
	}
}

func pathGraph(count int) (*layered.LGraphArena, layered.LGraphID, []layered.LNodeID) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	graph := arena.Graph(graphID)
	basegraph.Set(&graph.Properties, layered.DirectionOption, layered.DirectionRight)
	basegraph.Set(&graph.Properties, layered.AspectRatio, 1.6)
	nodes := make([]layered.LNodeID, count)
	for index := 0; index < count; index++ {
		layerID := arena.AddLayerToGraph(graphID)
		nodeID := arena.AddNode(graphID)
		nodes[index] = nodeID
		arena.Node(nodeID).Size.X = 30
		arena.Node(nodeID).Size.Y = 20
		arena.SetNodeLayer(nodeID, &layerID)
		if index == 0 {
			continue
		}
		source := arena.AddPort()
		arena.SetPortNode(source, &nodes[index-1])
		arena.SetPortSide(source, layered.EastSide)
		target := arena.AddPort()
		arena.SetPortNode(target, &nodeID)
		arena.SetPortSide(target, layered.WestSide)
		edgeID := arena.AddEdge()
		arena.SetEdgeSource(edgeID, &source)
		arena.SetEdgeTarget(edgeID, &target)
	}
	return arena, graphID, nodes
}
