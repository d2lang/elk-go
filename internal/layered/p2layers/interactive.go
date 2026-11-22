// Copyright (c) 2011, 2020 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p2layers

import "github.com/d2lang/elk-go/internal/layered"

type interactiveLayerSpan struct {
	start float64
	end   float64
	nodes []layered.LNodeID
}

// InteractiveLayerer derives initial layers from overlapping horizontal node
// spans, then moves targets right until every non-loop edge advances a layer.
type InteractiveLayerer struct{}

func (*InteractiveLayerer) ProcessorConfiguration(
	_ *layered.LGraphArena,
	_ layered.LGraphID,
) layered.ProcessorConfiguration {
	var configuration layered.ProcessorConfiguration
	configuration.
		AddBefore(layered.PhaseCycleBreaking, layered.InteractiveExternalPortPositioner).
		AddBefore(layered.PhaseLayering, layered.LayerConstraintPreprocessor).
		AddBefore(layered.PhaseNodeOrdering, layered.LayerConstraintPostprocessor)
	return configuration
}

func (*InteractiveLayerer) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	graph := arena.Graph(graphID)
	spans := make([]interactiveLayerSpan, 0)
	for _, nodeID := range graph.LayerlessNodes {
		node := arena.Node(nodeID)
		minimumX := node.Position.X
		maximumX := node.Position.X + node.Size.X
		if maximumX < minimumX+1 {
			maximumX = minimumX + 1
		}

		insertAt := len(spans)
		found := -1
		for index := 0; index < len(spans); {
			span := &spans[index]
			if span.start >= maximumX {
				insertAt = index
				break
			}
			if span.end > minimumX {
				if found < 0 {
					span.nodes = append(span.nodes, nodeID)
					if minimumX < span.start {
						span.start = minimumX
					}
					if maximumX > span.end {
						span.end = maximumX
					}
					found = index
					index++
					continue
				}
				spans[found].nodes = append(spans[found].nodes, span.nodes...)
				if span.end > spans[found].end {
					spans[found].end = span.end
				}
				spans = append(spans[:index], spans[index+1:]...)
				continue
			}
			index++
		}
		if found < 0 {
			span := interactiveLayerSpan{start: minimumX, end: maximumX, nodes: []layered.LNodeID{nodeID}}
			spans = append(spans, interactiveLayerSpan{})
			copy(spans[insertAt+1:], spans[insertAt:])
			spans[insertAt] = span
		}
	}

	for index, span := range spans {
		layerID := arena.AddLayer(graphID)
		arena.Layer(layerID).ID = index
		graph.Layers = append(graph.Layers, layerID)
		for _, nodeID := range span.nodes {
			arena.SetNodeLayer(nodeID, &layerID)
			arena.Node(nodeID).ID = 0
		}
	}

	// ELK's recursive checkNode(node), made iterative so that long paths need
	// no deeper call stack: frames holds one frame per pending call. Every
	// read and write happens in the recursion's order: a call marks its node
	// and reads its layer's index, then walks its outgoing edges (the list
	// taken when the call starts); a target not in a later layer is moved to
	// the next layer (added if missing) and checked, that call completing
	// before the next edge is looked at.
	type checkFrame struct {
		node       layered.LNodeID
		layerIndex int
		edges      []layered.LEdgeID
		next       int
	}
	// nextLayer is the layer after layerIndex, added when missing.
	nextLayer := func(layerIndex int) layered.LayerID {
		newIndex := layerIndex + 1
		if newIndex == len(graph.Layers) {
			newLayer := arena.AddLayer(graphID)
			arena.Layer(newLayer).ID = newIndex
			graph.Layers = append(graph.Layers, newLayer)
			return newLayer
		}
		return graph.Layers[newIndex]
	}
	// checkRecursive is ELK's checkNode, for calls beyond maxIterativeDepth.
	var checkRecursive func(layered.LNodeID)
	checkRecursive = func(nodeID layered.LNodeID) {
		node := arena.Node(nodeID)
		node.ID = 1
		layerIndex := arena.Layer(*node.Layer).ID
		for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
			target := arena.EdgeTargetNode(edgeID)
			if target == nodeID {
				continue
			}
			if arena.Layer(*arena.Node(target).Layer).ID <= layerIndex {
				newLayer := nextLayer(layerIndex)
				arena.SetNodeLayer(target, &newLayer)
				checkRecursive(target)
			}
		}
	}
	var frames []checkFrame
	call := func(nodeID layered.LNodeID) {
		node := arena.Node(nodeID)
		node.ID = 1
		layerIndex := arena.Layer(*node.Layer).ID
		frames = append(frames, checkFrame{node: nodeID, layerIndex: layerIndex, edges: arena.NodeOutgoingEdges(nodeID)})
	}
	checkNode := func(start layered.LNodeID) {
		frames = frames[:0]
		call(start)
		for len(frames) > 0 {
			frame := &frames[len(frames)-1]
			if frame.next == len(frame.edges) {
				frames = frames[:len(frames)-1]
				continue
			}
			nodeID, layerIndex := frame.node, frame.layerIndex
			target := arena.EdgeTargetNode(frame.edges[frame.next])
			frame.next++
			if target == nodeID {
				continue
			}
			if arena.Layer(*arena.Node(target).Layer).ID <= layerIndex {
				newLayer := nextLayer(layerIndex)
				arena.SetNodeLayer(target, &newLayer)
				if len(frames) >= maxIterativeDepth {
					checkRecursive(target)
				} else {
					call(target)
				}
			}
		}
	}
	for _, nodeID := range graph.LayerlessNodes {
		if arena.Node(nodeID).ID == 0 {
			checkNode(nodeID)
		}
	}

	kept := graph.Layers[:0]
	for _, layerID := range graph.Layers {
		if len(arena.Layer(layerID).Nodes) > 0 {
			kept = append(kept, layerID)
		}
	}
	graph.Layers = kept
	graph.LayerlessNodes = graph.LayerlessNodes[:0]
	return nil
}
