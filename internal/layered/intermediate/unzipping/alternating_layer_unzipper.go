// Copyright (c) 2024 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package unzipping ports ELK Layered's post-crossing-minimization layer
// splitting processors.
package unzipping

import (
	"math"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/layered/intermediate"
)

type layerInsertion struct {
	layer layered.LayerID
	index int
}

// AlternateLayers ports ELK 0.12.0's AlternatingLayerUnzipper. It divides a
// layer into sublayers while preserving its decided node/edge order, inserting
// long-edge dummies and temporary placeholders where the split requires them.
func AlternateLayers(arena *layered.LGraphArena, graphID layered.LGraphID) {
	graph := arena.Graph(graphID)
	originalLayers := append([]layered.LayerID(nil), graph.Layers...)
	insertionOffset := 1
	newLayers := make([]layerInsertion, 0)
	for layerIndex, layerID := range originalLayers {
		layerSplit := layerSplitProperty(arena, layerID)
		resetOnLongEdges := resetOnLongEdgesProperty(arena, layerID)
		minimizeEdgeLength := minimizeEdgeLengthProperty(arena, layerID)
		if minimizeEdgeLength && shouldKeepLayerUnsplit(arena, graphID, layerID) {
			continue
		}
		if len(arena.Layer(layerID).Nodes) < layerSplit {
			continue
		}

		subLayers := make([]layered.LayerID, 1, layerSplit)
		subLayers[0] = layerID
		for splitIndex := 0; splitIndex < layerSplit-1; splitIndex++ {
			newLayerID := arena.AddLayer(graphID)
			newLayers = append(newLayers, layerInsertion{
				layer: newLayerID,
				index: layerIndex + splitIndex + insertionOffset,
			})
			subLayers = append(subLayers, newLayerID)
		}
		insertionOffset += layerSplit - 1

		nodesInLayer := len(arena.Layer(layerID).Nodes)
		for visited, nodeIndex, targetLayer := 0, 0, 0; visited < nodesInLayer; visited, nodeIndex, targetLayer = visited+1, nodeIndex+1, targetLayer+1 {
			nodeID := arena.Layer(layerID).Nodes[nodeIndex]
			if arena.Node(nodeID).Type != layered.NonShiftingPlaceholderNode {
				nodeIndex += shiftNode(arena, graphID, subLayers, targetLayer%layerSplit, nodeIndex)
			} else {
				visited--
				targetLayer--
			}
			if resetOnLongEdges && arena.Node(nodeID).Type == layered.LongEdgeNode {
				targetLayer = -1
			}
		}
	}

	for _, insertion := range newLayers {
		layers := arena.Graph(graphID).Layers
		layers = append(layers, 0)
		copy(layers[insertion.index+1:], layers[insertion.index:])
		layers[insertion.index] = insertion.layer
		arena.Graph(graphID).Layers = layers
	}
	for _, layerID := range arena.Graph(graphID).Layers {
		nodes := arena.Layer(layerID).Nodes
		retained := nodes[:0]
		for _, nodeID := range nodes {
			typeOfNode := arena.Node(nodeID).Type
			if typeOfNode != layered.PlaceholderNode && typeOfNode != layered.NonShiftingPlaceholderNode {
				retained = append(retained, nodeID)
			}
		}
		arena.Layer(layerID).Nodes = retained
	}
}

func layerSplitProperty(arena *layered.LGraphArena, layerID layered.LayerID) int {
	value := math.MaxInt
	propertySet := false
	for _, nodeID := range arena.Layer(layerID).Nodes {
		if nodeValue, found := basegraph.TryGet(
			&arena.Node(nodeID).Properties, layered.LayerUnzippingLayerSplit,
		); found {
			propertySet = true
			value = min(value, nodeValue)
		}
	}
	if !propertySet {
		return 2
	}
	return value
}

func resetOnLongEdgesProperty(arena *layered.LGraphArena, layerID layered.LayerID) bool {
	for _, nodeID := range arena.Layer(layerID).Nodes {
		if value, found := basegraph.TryGet(
			&arena.Node(nodeID).Properties, layered.LayerUnzippingResetOnLongEdges,
		); found && !value {
			return false
		}
	}
	return true
}

func minimizeEdgeLengthProperty(arena *layered.LGraphArena, layerID layered.LayerID) bool {
	for _, nodeID := range arena.Layer(layerID).Nodes {
		if value, found := basegraph.TryGet(
			&arena.Node(nodeID).Properties, layered.LayerUnzippingMinimizeEdgeLength,
		); found && value {
			return true
		}
	}
	return false
}

func shouldKeepLayerUnsplit(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	layerID layered.LayerID,
) bool {
	maxWidth := 0.0
	averageHeight := 0.0
	nodes := arena.Layer(layerID).Nodes
	for _, nodeID := range nodes {
		node := arena.Node(nodeID)
		maxWidth = math.Max(maxWidth, node.Size.X)
		averageHeight += node.Size.Y
	}
	averageHeight /= float64(len(nodes))
	graph := arena.Graph(graphID)
	maxWidth += math.Max(
		2*basegraph.Get(&graph.Properties, layered.SpacingEdgeNodeBetweenLayers),
		math.Max(
			float64(len(nodes))*basegraph.Get(&graph.Properties, layered.SpacingEdgeEdgeBetweenLayers),
			basegraph.Get(&graph.Properties, layered.SpacingNodeNodeBetweenLayers),
		),
	)
	averageHeight += math.Max(
		basegraph.Get(&graph.Properties, layered.SpacingNodeNode),
		basegraph.Get(&graph.Properties, layered.SpacingEdgeNode),
	)
	return maxWidth/averageHeight >= float64(len(nodes))/4
}

func shiftNode(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	subLayers []layered.LayerID,
	targetLayer int,
	nodeIndex int,
) int {
	nodeID := arena.Layer(subLayers[0]).Nodes[nodeIndex]
	if targetLayer > 0 {
		arena.SetNodeLayer(nodeID, &subLayers[targetLayer])
	}

	edgeCount := 0
	incoming := arena.NodeIncomingEdges(nodeID)
	for left, right := 0, len(incoming)-1; left < right; left, right = left+1, right-1 {
		incoming[left], incoming[right] = incoming[right], incoming[left]
	}
	for _, incomingEdge := range incoming {
		nextEdge := incomingEdge
		for layerIndex := 0; layerIndex < targetLayer; layerIndex++ {
			dummyID := createDummyNode(arena, graphID, nextEdge)
			insertNode(arena, dummyID, subLayers[layerIndex], nodeIndex+edgeCount)
			nextEdge = intermediate.SplitEdgeAtDummy(arena, nextEdge, dummyID)
		}
		if targetLayer > 0 {
			edgeCount++
		}
	}
	if len(incoming) == 0 {
		for layerIndex := 0; layerIndex < targetLayer; layerIndex++ {
			placeholder := arena.AddNode(graphID)
			arena.Node(placeholder).Type = layered.PlaceholderNode
			insertNode(arena, placeholder, subLayers[layerIndex], nodeIndex+edgeCount)
		}
		if targetLayer > 0 {
			edgeCount++
		}
	}

	extraEdge := false
	for _, outgoingEdge := range append([]layered.LEdgeID(nil), arena.NodeOutgoingEdges(nodeID)...) {
		nextEdge := outgoingEdge
		for layerIndex := targetLayer + 1; layerIndex < len(subLayers); layerIndex++ {
			dummyID := createDummyNode(arena, graphID, nextEdge)
			arena.SetNodeLayer(dummyID, &subLayers[layerIndex])
			nextEdge = intermediate.SplitEdgeAtDummy(arena, nextEdge, dummyID)
		}
		if extraEdge {
			for layerIndex := 0; layerIndex <= targetLayer; layerIndex++ {
				placeholder := arena.AddNode(graphID)
				arena.Node(placeholder).Type = layered.NonShiftingPlaceholderNode
				insertNode(arena, placeholder, subLayers[layerIndex], nodeIndex+1)
			}
			edgeCount++
		}
		extraEdge = true
	}
	if edgeCount > 0 {
		return edgeCount - 1
	}
	return 0
}

func insertNode(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	layerID layered.LayerID,
	index int,
) {
	if index > len(arena.Layer(layerID).Nodes) {
		arena.SetNodeLayer(nodeID, &layerID)
	} else {
		arena.SetNodeLayerAt(nodeID, &layerID, index)
	}
}

func createDummyNode(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	edgeID layered.LEdgeID,
) layered.LNodeID {
	dummyID := arena.AddNode(graphID)
	dummy := arena.Node(dummyID)
	dummy.Type = layered.LongEdgeNode
	basegraph.Set(&dummy.Properties, layered.OriginalElement, layered.LEdgeOrigin(edgeID))
	basegraph.Set(&dummy.Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedPosition)
	return dummyID
}
