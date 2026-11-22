// Copyright (c) 2010, 2020 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p2layers

import "github.com/d2lang/elk-go/internal/layered"

// LongestPathLayerer assigns each node according to its longest path to a
// sink. Layers are prepended exactly as in ELK 0.8.1.
type LongestPathLayerer struct{}

// maxIterativeDepth bounds the explicit stacks of the walks made iterative
// here (LongestPathLayerer's visit, InteractiveLayerer's checkNode): a call
// deeper than this is made by ELK's recursion itself, on the goroutine stack.
// So a long path needs maxIterativeDepth fewer stack frames than in the
// original, and a walk that never ends (a cycle left in the graph: the
// original recurses until the goroutine stack overflows, a fatal error) ends
// the same way after holding at most maxIterativeDepth frames, instead of
// growing an explicit stack until memory runs out. (A variable for tests.)
var maxIterativeDepth = 1 << 16

func (*LongestPathLayerer) ProcessorConfiguration(
	_ *layered.LGraphArena,
	_ layered.LGraphID,
) layered.ProcessorConfiguration {
	return layered.NetworkSimplexLayeringConfiguration()
}

func (*LongestPathLayerer) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	graph := arena.Graph(graphID)
	nodes := graph.LayerlessNodes
	heights := make([]int, len(nodes))
	for index, nodeID := range nodes {
		arena.Node(nodeID).ID = index
		heights[index] = -1
	}

	// ELK's recursive visit(node), made iterative so that long paths need no
	// deeper call stack: frames holds one frame per pending call. Every read
	// and write happens in the recursion's order: a call reads its node's
	// height and returns it if known; otherwise it walks the node's outgoing
	// edges (the list taken when the call starts), a call made for an edge's
	// target completing before the next edge is looked at, and then adds the
	// layers it needs, puts the node into its layer and records its height.
	type visitFrame struct {
		node    layered.LNodeID
		index   int
		edges   []layered.LEdgeID
		next    int
		maximum int
	}
	// place puts a node whose height is maximum into its layer, adding the
	// layers it needs, and records its height.
	place := func(nodeID layered.LNodeID, index, maximum int) {
		for len(graph.Layers) < maximum {
			newLayer := arena.AddLayer(graphID)
			graph.Layers = append(graph.Layers, 0)
			copy(graph.Layers[1:], graph.Layers[:len(graph.Layers)-1])
			graph.Layers[0] = newLayer
		}
		layerID := graph.Layers[len(graph.Layers)-maximum]
		arena.SetNodeLayer(nodeID, &layerID)
		heights[index] = maximum
	}
	// visitRecursive is ELK's visit, for calls beyond maxIterativeDepth.
	var visitRecursive func(layered.LNodeID) int
	visitRecursive = func(nodeID layered.LNodeID) int {
		index := arena.Node(nodeID).ID
		if heights[index] >= 0 {
			return heights[index]
		}
		maximum := 1
		for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
			target := arena.EdgeTargetNode(edgeID)
			if target == nodeID {
				continue
			}
			height := visitRecursive(target) + 1
			if height > maximum {
				maximum = height
			}
		}
		place(nodeID, index, maximum)
		return maximum
	}
	var frames []visitFrame
	visit := func(start layered.LNodeID) {
		index := arena.Node(start).ID
		if heights[index] >= 0 {
			return
		}
		frames = append(frames[:0], visitFrame{node: start, index: index, edges: arena.NodeOutgoingEdges(start), maximum: 1})
		for len(frames) > 0 {
			frame := &frames[len(frames)-1]
			if frame.next < len(frame.edges) {
				target := arena.EdgeTargetNode(frame.edges[frame.next])
				frame.next++
				if target == frame.node {
					continue
				}
				// The call visit(target).
				targetIndex := arena.Node(target).ID
				if heights[targetIndex] < 0 {
					if len(frames) >= maxIterativeDepth {
						if height := visitRecursive(target) + 1; height > frame.maximum {
							frame.maximum = height
						}
						continue
					}
					frames = append(frames, visitFrame{node: target, index: targetIndex,
						edges: arena.NodeOutgoingEdges(target), maximum: 1})
					continue
				}
				if height := heights[targetIndex] + 1; height > frame.maximum {
					frame.maximum = height
				}
				continue
			}
			maximum := frame.maximum
			place(frame.node, frame.index, maximum)
			frames = frames[:len(frames)-1]
			if len(frames) > 0 {
				parent := &frames[len(frames)-1]
				if height := maximum + 1; height > parent.maximum {
					parent.maximum = height
				}
			}
		}
	}

	for _, nodeID := range nodes {
		visit(nodeID)
	}
	graph.LayerlessNodes = graph.LayerlessNodes[:0]
	return nil
}
