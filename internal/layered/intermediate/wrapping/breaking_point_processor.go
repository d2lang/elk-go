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
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/layered/intermediate"
)

// ProcessBreakingPoints folds the chunks marked by InsertBreakingPoints onto
// the initial layers and optionally applies ELK's two dummy-chain shortening
// passes.
func ProcessBreakingPoints(arena *layered.LGraphArena, graphID layered.LGraphID) {
	performWrapping(arena, graphID)
	if !basegraph.Get(&arena.Graph(graphID).Properties, layered.WrappingMultiEdgeImproveWrappedEdges) {
		return
	}
	for _, layerID := range arena.Graph(graphID).Layers {
		for index, nodeID := range arena.Layer(layerID).Nodes {
			arena.Node(nodeID).ID = index
		}
	}
	improveMultiCutIndexEdges(arena, graphID)
	improveUnnecessarilyLongEdges(arena, graphID, true)
	improveUnnecessarilyLongEdges(arena, graphID, false)
}

func performWrapping(arena *layered.LGraphArena, graphID layered.LGraphID) {
	graph := arena.Graph(graphID)
	empty := arena.AddLayer(graphID)
	graph.Layers = insertLayer(graph.Layers, 0, empty)
	reverse := false
	index := 1
	position := 1
	for position < len(graph.Layers) {
		layerID := graph.Layers[position]
		position++
		newLayer := graph.Layers[index]
		nodesToMove := append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...)
		offset := len(nodesToMove)
		for _, nodeID := range nodesToMove {
			arena.SetNodeLayer(nodeID, &newLayer)
		}
		if reverse {
			for nodeIndex := len(nodesToMove) - 1; nodeIndex >= 0; nodeIndex-- {
				nodeID := nodesToMove[nodeIndex]
				incoming := append([]layered.LEdgeID(nil), arena.NodeIncomingEdges(nodeID)...)
				for _, edgeID := range incoming {
					arena.ReverseEdge(graphID, edgeID, true)
					basegraph.Set(&graph.Properties, layered.Cyclic, true)
					dummyEdges := InsertDummies(arena, graphID, edgeID, offset)
					info := basegraph.Get(&arena.Node(nodeID).Properties, breakingPointInfo)
					startInLayerEdge := dummyEdges[len(dummyEdges)-1]
					startInLayerDummy := arena.EdgeSourceNode(startInLayerEdge)
					endInLayerDummy := arena.EdgeTargetNode(edgeID)
					info.StartInLayerDummy = &startInLayerDummy
					info.StartInLayerEdge = &startInLayerEdge
					info.EndInLayerDummy = &endInLayerDummy
					endInLayerEdge := edgeID
					info.EndInLayerEdge = &endInLayerEdge
				}
			}
			reverse = false
		} else if len(nodesToMove) > 0 && arena.Node(nodesToMove[0]).Type == layered.BreakingPointNode {
			reverse = true
			index = -1
		}
		index++
	}
	retained := graph.Layers[:0]
	for _, layerID := range graph.Layers {
		if len(arena.Layer(layerID).Nodes) > 0 {
			retained = append(retained, layerID)
		}
	}
	graph.Layers = retained
}

func improveMultiCutIndexEdges(arena *layered.LGraphArena, graphID layered.LGraphID) {
	for _, layerID := range arena.Graph(graphID).Layers {
		for _, nodeID := range append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...) {
			if !IsBreakingPointStart(arena, nodeID) {
				continue
			}
			info := basegraph.Get(&arena.Node(nodeID).Properties, breakingPointInfo)
			if info.Prev != nil || info.Next == nil {
				continue
			}
			current := info
			next := info.Next
			for next != nil {
				dropDummies(arena, next.Start, *next.StartInLayerDummy, false, true)
				updateIndexesAfter(arena, current.End)
				updateIndexesAfter(arena, next.Start)
				updateIndexesAfter(arena, *next.StartInLayerDummy)
				updateIndexesAfter(arena, *next.EndInLayerDummy)

				target := *arena.Edge(*current.EndInLayerEdge).Target
				arena.SetEdgeTarget(*next.EndInLayerEdge, &target)
				arena.SetEdgeTarget(*current.EndInLayerEdge, nil)
				arena.SetNodeLayer(current.End, nil)
				arena.SetNodeLayer(next.Start, nil)
				arena.SetNodeLayer(*next.StartInLayerDummy, nil)
				arena.SetNodeLayer(*next.EndInLayerDummy, nil)

				newInfo := &BPInfo{
					Start: current.Start, End: next.End,
					NodeStartEdge:     current.NodeStartEdge,
					StartEndEdge:      next.StartEndEdge,
					OriginalEdge:      next.OriginalEdge,
					StartInLayerDummy: current.StartInLayerDummy,
					StartInLayerEdge:  current.StartInLayerEdge,
					EndInLayerDummy:   current.EndInLayerDummy,
					EndInLayerEdge:    next.EndInLayerEdge,
					Prev:              current.Prev,
					Next:              next.Next,
				}
				basegraph.Set(&arena.Node(current.Start).Properties, breakingPointInfo, newInfo)
				basegraph.Set(&arena.Node(next.End).Properties, breakingPointInfo, newInfo)
				next = next.Next
				current = newInfo
			}
		}
	}
}

func improveUnnecessarilyLongEdges(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	forwards bool,
) {
	changed := false
	for {
		changed = false
		layers := arena.Graph(graphID).Layers
		for layerOffset := 0; layerOffset < len(layers); layerOffset++ {
			layerIndex := layerOffset
			if forwards {
				layerIndex = len(layers) - 1 - layerOffset
			}
			nodes := append([]layered.LNodeID(nil), arena.Layer(layers[layerIndex]).Nodes...)
			// ELK 0.8.1 calls Lists.reverse(nodes) here without retaining the
			// returned view, so the copied list remains in forward order.
			for _, nodeID := range nodes {
				matches := IsBreakingPointStart(arena, nodeID)
				if forwards {
					matches = IsBreakingPointEnd(arena, nodeID)
				}
				if !matches {
					continue
				}
				info := basegraph.Get(&arena.Node(nodeID).Properties, breakingPointInfo)
				dummy := *info.StartInLayerDummy
				if forwards {
					dummy = *info.EndInLayerDummy
				}
				// Source assignment is intentional: ELK only retains the result
				// of the last matching node in this traversal.
				changed = dropDummies(arena, nodeID, dummy, forwards, false)
			}
		}
		if !changed {
			return
		}
	}
}

func dropDummies(
	arena *layered.LGraphArena,
	breakingPoint layered.LNodeID,
	inLayerDummy layered.LNodeID,
	forwards, force bool,
) bool {
	first, firstFound := nextLongEdgeDummy(arena, breakingPoint, forwards)
	second, secondFound := nextLongEdgeDummy(arena, inLayerDummy, forwards)
	changed := false
	for firstFound && secondFound {
		if !force && !adjacentOrSeparatedByBreakingPoints(arena, first, second, forwards) {
			break
		}
		nextFirst, nextFirstFound := nextLongEdgeDummy(arena, first, forwards)
		nextSecond, nextSecondFound := nextLongEdgeDummy(arena, second, forwards)
		updateIndexesAfter(arena, inLayerDummy)
		updateIndexesAfter(arena, breakingPoint)
		newLayer := *arena.Node(first).Layer
		firstIndex, secondIndex := arena.Node(first).ID, arena.Node(second).ID
		intermediate.JoinLongEdgeAt(arena, first, false)
		intermediate.JoinLongEdgeAt(arena, second, false)
		if forwards {
			arena.SetNodeLayerAt(inLayerDummy, &newLayer, secondIndex)
			arena.Node(inLayerDummy).ID = secondIndex
			arena.SetNodeLayerAt(breakingPoint, &newLayer, firstIndex+1)
			arena.Node(breakingPoint).ID = firstIndex
		} else {
			arena.SetNodeLayerAt(breakingPoint, &newLayer, firstIndex)
			arena.Node(breakingPoint).ID = firstIndex
			arena.SetNodeLayerAt(inLayerDummy, &newLayer, secondIndex+1)
			arena.Node(inLayerDummy).ID = secondIndex
		}
		arena.SetNodeLayer(first, nil)
		arena.SetNodeLayer(second, nil)
		first, firstFound = nextFirst, nextFirstFound
		second, secondFound = nextSecond, nextSecondFound
		changed = true
	}
	return changed
}

func adjacentOrSeparatedByBreakingPoints(
	arena *layered.LGraphArena,
	first, second layered.LNodeID,
	forwards bool,
) bool {
	start, end := first, second
	if forwards {
		start, end = second, first
	}
	layer := arena.Layer(*arena.Node(first).Layer)
	for index := arena.Node(start).ID + 1; index < arena.Node(end).ID; index++ {
		nodeID := layer.Nodes[index]
		if arena.Node(nodeID).Type != layered.BreakingPointNode && !isInLayerDummy(arena, nodeID) {
			return false
		}
	}
	return true
}

func nextLongEdgeDummy(
	arena *layered.LGraphArena,
	start layered.LNodeID,
	forwards bool,
) (layered.LNodeID, bool) {
	edges := arena.NodeIncomingEdges(start)
	if forwards {
		edges = arena.NodeOutgoingEdges(start)
	}
	for _, edgeID := range edges {
		other := arena.EdgeOtherNode(edgeID, start)
		if arena.Node(other).Type == layered.LongEdgeNode &&
			*arena.Node(other).Layer != *arena.Node(start).Layer {
			return other, true
		}
	}
	return 0, false
}

func isInLayerDummy(arena *layered.LGraphArena, nodeID layered.LNodeID) bool {
	if arena.Node(nodeID).Type != layered.LongEdgeNode {
		return false
	}
	for _, edgeID := range arena.NodeConnectedEdges(nodeID) {
		if !arena.EdgeIsSelfLoop(edgeID) {
			other := arena.EdgeOtherNode(edgeID, nodeID)
			if arena.Node(other).Layer != nil && arena.Node(nodeID).Layer != nil &&
				*arena.Node(other).Layer == *arena.Node(nodeID).Layer {
				return true
			}
		}
	}
	return false
}

func updateIndexesAfter(arena *layered.LGraphArena, nodeID layered.LNodeID) {
	layer := arena.Layer(*arena.Node(nodeID).Layer)
	for index := arena.Node(nodeID).ID + 1; index < len(layer.Nodes); index++ {
		arena.Node(layer.Nodes[index]).ID--
	}
}
