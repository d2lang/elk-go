// Copyright (c) 2016 Kiel University and others.
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

// ProcessHighDegreeNodes is the arena-backed port of ELK 0.8.1's
// HighDegreeNodeLayeringProcessor. It isolates bounded-height trees adjacent
// to high-degree nodes in newly introduced layers.
func ProcessHighDegreeNodes(arena *layered.LGraphArena, graphID layered.LGraphID) {
	processor := highDegreeNodeProcessor{
		arena:               arena,
		graphID:             graphID,
		degreeThreshold:     basegraph.Get(&arena.Graph(graphID).Properties, layered.HighDegreeNodeThreshold),
		treeHeightThreshold: basegraph.Get(&arena.Graph(graphID).Properties, layered.HighDegreeNodeTreeHeight),
	}
	if processor.treeHeightThreshold == 0 {
		processor.treeHeightThreshold = math.MaxInt
	}
	processor.process()
}

type highDegreeNodeProcessor struct {
	arena               *layered.LGraphArena
	graphID             layered.LGraphID
	degreeThreshold     int
	treeHeightThreshold int
}

type highDegreeNodeInformation struct {
	incomingMaxHeight int
	incomingRoots     []layered.LNodeID
	outgoingMaxHeight int
	outgoingRoots     []layered.LNodeID
}

type treeDirection uint8

const (
	treeIncoming treeDirection = iota
	treeOutgoing
)

func (processor *highDegreeNodeProcessor) process() {
	// Java's ListIterator skips the layers it introduces. Retaining the
	// original layer identities gives the same traversal while slice inserts
	// move their indices underneath us.
	originalLayers := append([]layered.LayerID(nil), processor.arena.Graph(processor.graphID).Layers...)
	for _, layerID := range originalLayers {
		graph := processor.arena.Graph(processor.graphID)
		layerIndex := indexOfLayer(graph.Layers, layerID)
		if layerIndex < 0 {
			continue
		}

		type highDegreeEntry struct {
			node layered.LNodeID
			info highDegreeNodeInformation
		}
		entries := make([]highDegreeEntry, 0)
		incomingMax, outgoingMax := -1, -1
		for _, nodeID := range append([]layered.LNodeID(nil), processor.arena.Layer(layerID).Nodes...) {
			if !processor.isHighDegreeNode(nodeID) {
				continue
			}
			info := processor.calculateInformation(nodeID)
			incomingMax = max(incomingMax, info.incomingMaxHeight)
			outgoingMax = max(outgoingMax, info.outgoingMaxHeight)
			entries = append(entries, highDegreeEntry{node: nodeID, info: info})
		}

		before := make([]layered.LayerID, 0, max(0, incomingMax))
		for range incomingMax {
			newLayer := processor.insertLayer(layerIndex)
			// Each repeated prepend is nearer to the current layer than the
			// previous one; Java consequently adds it at index zero.
			before = append([]layered.LayerID{newLayer}, before...)
			layerIndex++
		}
		for _, entry := range entries {
			for _, root := range entry.info.incomingRoots {
				processor.moveTree(root, treeIncoming, before)
			}
		}

		after := make([]layered.LayerID, 0, max(0, outgoingMax))
		for range outgoingMax {
			newLayer := processor.insertLayer(layerIndex + 1 + len(after))
			after = append(after, newLayer)
		}
		for _, entry := range entries {
			for _, root := range entry.info.outgoingRoots {
				processor.moveTree(root, treeOutgoing, after)
			}
		}
	}

	graph := processor.arena.Graph(processor.graphID)
	kept := graph.Layers[:0]
	for _, layerID := range graph.Layers {
		if len(processor.arena.Layer(layerID).Nodes) != 0 {
			kept = append(kept, layerID)
		}
	}
	graph.Layers = kept
}

func (processor *highDegreeNodeProcessor) insertLayer(index int) layered.LayerID {
	graph := processor.arena.Graph(processor.graphID)
	layerID := processor.arena.AddLayer(processor.graphID)
	graph = processor.arena.Graph(processor.graphID) // AddLayer may grow the arena.
	graph.Layers = append(graph.Layers, 0)
	copy(graph.Layers[index+1:], graph.Layers[index:])
	graph.Layers[index] = layerID
	return layerID
}

func (processor *highDegreeNodeProcessor) isHighDegreeNode(nodeID layered.LNodeID) bool {
	return len(processor.arena.NodeConnectedEdges(nodeID)) >= processor.degreeThreshold
}

func (processor *highDegreeNodeProcessor) calculateInformation(nodeID layered.LNodeID) highDegreeNodeInformation {
	info := highDegreeNodeInformation{incomingMaxHeight: -1, outgoingMaxHeight: -1}
	for _, edgeID := range processor.arena.NodeIncomingEdges(nodeID) {
		if processor.arena.EdgeIsSelfLoop(edgeID) {
			continue
		}
		source := processor.arena.EdgeSourceNode(edgeID)
		if !processor.hasSingleConnection(source, treeOutgoing) {
			continue
		}
		height := processor.treeHeight(source, treeOutgoing, treeIncoming)
		if height < 0 {
			continue
		}
		info.incomingMaxHeight = max(info.incomingMaxHeight, height)
		info.incomingRoots = append(info.incomingRoots, source)
	}
	for _, edgeID := range processor.arena.NodeOutgoingEdges(nodeID) {
		if processor.arena.EdgeIsSelfLoop(edgeID) {
			continue
		}
		target := processor.arena.EdgeTargetNode(edgeID)
		if !processor.hasSingleConnection(target, treeIncoming) {
			continue
		}
		height := processor.treeHeight(target, treeIncoming, treeOutgoing)
		if height < 0 {
			continue
		}
		info.outgoingMaxHeight = max(info.outgoingMaxHeight, height)
		info.outgoingRoots = append(info.outgoingRoots, target)
	}
	return info
}

func (processor *highDegreeNodeProcessor) edges(nodeID layered.LNodeID, direction treeDirection) []layered.LEdgeID {
	if direction == treeIncoming {
		return processor.arena.NodeIncomingEdges(nodeID)
	}
	return processor.arena.NodeOutgoingEdges(nodeID)
}

func (processor *highDegreeNodeProcessor) hasSingleConnection(
	nodeID layered.LNodeID,
	direction treeDirection,
) bool {
	var connection *layered.LNodeID
	for _, edgeID := range processor.edges(nodeID, direction) {
		other := processor.arena.EdgeOtherNode(edgeID, nodeID)
		if connection == nil {
			value := other
			connection = &value
		} else if *connection != other {
			return false
		}
	}
	return true
}

func (processor *highDegreeNodeProcessor) treeHeight(
	root layered.LNodeID,
	ancestorEdges treeDirection,
	descendantEdges treeDirection,
) int {
	if processor.isHighDegreeNode(root) || !processor.hasSingleConnection(root, ancestorEdges) {
		return -1
	}
	descendants := processor.edges(root, descendantEdges)
	if len(descendants) == 0 {
		return 1
	}
	currentHeight := 0
	for _, edgeID := range descendants {
		other := processor.arena.EdgeOtherNode(edgeID, root)
		height := processor.treeHeight(other, ancestorEdges, descendantEdges)
		if height < 0 {
			return -1
		}
		currentHeight = max(currentHeight, height)
		if currentHeight > processor.treeHeightThreshold-1 {
			return -1
		}
	}
	return currentHeight + 1
}

func (processor *highDegreeNodeProcessor) moveTree(
	root layered.LNodeID,
	direction treeDirection,
	layers []layered.LayerID,
) {
	if len(layers) == 0 {
		panic("layered: high-degree tree exceeds its calculated height")
	}
	processor.arena.SetNodeLayer(root, &layers[0])
	for _, edgeID := range processor.edges(root, direction) {
		processor.moveTree(processor.arena.EdgeOtherNode(edgeID, root), direction, layers[1:])
	}
}

func indexOfLayer(layers []layered.LayerID, target layered.LayerID) int {
	for index, layerID := range layers {
		if layerID == target {
			return index
		}
	}
	return -1
}
