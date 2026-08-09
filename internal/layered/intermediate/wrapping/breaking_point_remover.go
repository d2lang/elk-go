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
	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// RemoveBreakingPoints reassembles each split edge after routing and removes
// the paired breaking-point markers.
func RemoveBreakingPoints(arena *layered.LGraphArena, graphID layered.LGraphID) {
	routing := basegraph.Get(&arena.Graph(graphID).Properties, layered.EdgeRoutingOption)
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...) {
			if !IsBreakingPointEnd(arena, nodeID) {
				continue
			}
			info := basegraph.Get(&arena.Node(nodeID).Properties, breakingPointInfo)
			if info.Next == nil {
				removeBreakingPoint(arena, info, routing)
			}
		}
	}
}

func removeBreakingPoint(
	arena *layered.LGraphArena,
	info *BPInfo,
	routing layered.EdgeRouting,
) {
	original := arena.Edge(info.OriginalEdge)
	newBends := make(geometry.Chain, 0,
		len(arena.Edge(info.NodeStartEdge).BendPoints)+
			len(arena.Edge(info.StartEndEdge).BendPoints)+len(original.BendPoints)+2)
	switch routing {
	case layered.EdgeRoutingSplines:
		first, _ := basegraph.TryGet(&arena.Edge(info.NodeStartEdge).Properties, layered.SplineControlPointSegments)
		middle, _ := basegraph.TryGet(&arena.Edge(info.StartEndEdge).Properties, layered.SplineControlPointSegments)
		third, _ := basegraph.TryGet(&original.Properties, layered.SplineControlPointSegments)
		segments := make([]geometry.Chain, 0, len(first)+len(middle)+len(third))
		for _, segment := range first {
			segments = append(segments, segment)
		}
		for index := len(middle) - 1; index >= 0; index-- {
			segments = append(segments, middle[index].Reverse())
		}
		for _, segment := range third {
			segments = append(segments, segment)
		}
		basegraph.Set(&original.Properties, layered.SplineControlPointSegments, segments)
		basegraph.Unset(&arena.Edge(info.NodeStartEdge).Properties, layered.SplineControlPointSegments)
		basegraph.Unset(&arena.Edge(info.StartEndEdge).Properties, layered.SplineControlPointSegments)

		firstCompaction, _ := basegraph.TryGet(
			&arena.Edge(info.NodeStartEdge).Properties, layered.SplineCompactionSegments)
		middleCompaction, _ := basegraph.TryGet(
			&arena.Edge(info.StartEndEdge).Properties, layered.SplineCompactionSegments)
		thirdCompaction, _ := basegraph.TryGet(
			&original.Properties, layered.SplineCompactionSegments)
		compactionSegments := make([]layered.SplineCompactionSegment, 0,
			len(firstCompaction)+len(middleCompaction)+len(thirdCompaction))
		compactionSegments = append(compactionSegments, firstCompaction...)
		for index := len(middleCompaction) - 1; index >= 0; index-- {
			compactionSegments = append(compactionSegments, middleCompaction[index])
		}
		compactionSegments = append(compactionSegments, thirdCompaction...)
		basegraph.Set(&original.Properties, layered.SplineCompactionSegments, compactionSegments)
		basegraph.Unset(&arena.Edge(info.NodeStartEdge).Properties, layered.SplineCompactionSegments)
		basegraph.Unset(&arena.Edge(info.StartEndEdge).Properties, layered.SplineCompactionSegments)
	case layered.EdgeRoutingPolyline:
		newBends = append(newBends, arena.Edge(info.NodeStartEdge).BendPoints...)
		newBends = append(newBends, arena.Node(info.Start).Position)
		newBends = append(newBends, arena.Edge(info.StartEndEdge).BendPoints.Reverse()...)
		newBends = append(newBends, arena.Node(info.End).Position)
		newBends = append(newBends, original.BendPoints...)
	default:
		newBends = append(newBends, arena.Edge(info.NodeStartEdge).BendPoints...)
		newBends = append(newBends, arena.Edge(info.StartEndEdge).BendPoints.Reverse()...)
		newBends = append(newBends, original.BendPoints...)
	}
	original.BendPoints = newBends
	source := *arena.Edge(info.NodeStartEdge).Source
	arena.SetEdgeSource(info.OriginalEdge, &source)

	firstJunctions, firstFound := basegraph.TryGet(&arena.Edge(info.NodeStartEdge).Properties, layered.JunctionPoints)
	middleJunctions, middleFound := basegraph.TryGet(&arena.Edge(info.StartEndEdge).Properties, layered.JunctionPoints)
	thirdJunctions, thirdFound := basegraph.TryGet(&original.Properties, layered.JunctionPoints)
	if firstFound || middleFound || thirdFound {
		junctions := thirdJunctions.Clone()
		junctions = append(junctions, middleJunctions...)
		junctions = append(junctions, firstJunctions...)
		basegraph.Set(&original.Properties, layered.JunctionPoints, junctions)
	}

	arena.SetEdgeSource(info.StartEndEdge, nil)
	arena.SetEdgeTarget(info.StartEndEdge, nil)
	arena.SetEdgeSource(info.NodeStartEdge, nil)
	arena.SetEdgeTarget(info.NodeStartEdge, nil)
	arena.SetNodeLayer(info.End, nil)
	arena.SetNodeLayer(info.Start, nil)
	if info.Prev != nil {
		removeBreakingPoint(arena, info.Prev, routing)
	}
}
