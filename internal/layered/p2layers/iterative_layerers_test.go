// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package p2layers

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/d2lang/elk-go/internal/layered"
)

// The longest-path and interactive layerers as they were before their
// recursions were made iterative (their frames grew with the arena helpers
// inlined into them, so long paths overflowed a 1 MiB stack sooner than in
// the original); the iterative ones must do exactly what these do.

func recursiveLongestPath(arena *layered.LGraphArena, graphID layered.LGraphID) {
	graph := arena.Graph(graphID)
	nodes := graph.LayerlessNodes
	heights := make([]int, len(nodes))
	for index, nodeID := range nodes {
		arena.Node(nodeID).ID = index
		heights[index] = -1
	}
	var visit func(layered.LNodeID) int
	visit = func(nodeID layered.LNodeID) int {
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
			height := visit(target) + 1
			if height > maximum {
				maximum = height
			}
		}
		for len(graph.Layers) < maximum {
			newLayer := arena.AddLayer(graphID)
			graph.Layers = append(graph.Layers, 0)
			copy(graph.Layers[1:], graph.Layers[:len(graph.Layers)-1])
			graph.Layers[0] = newLayer
		}
		layerID := graph.Layers[len(graph.Layers)-maximum]
		arena.SetNodeLayer(nodeID, &layerID)
		heights[index] = maximum
		return maximum
	}
	for _, nodeID := range nodes {
		visit(nodeID)
	}
	graph.LayerlessNodes = graph.LayerlessNodes[:0]
}

// recursiveInteractiveChecks is the second half of the interactive
// layerer (after the initial layers): ELK's recursive checkNode.
func recursiveInteractiveChecks(arena *layered.LGraphArena, graphID layered.LGraphID) {
	graph := arena.Graph(graphID)
	var checkNode func(layered.LNodeID)
	checkNode = func(nodeID layered.LNodeID) {
		node := arena.Node(nodeID)
		node.ID = 1
		layerIndex := arena.Layer(*node.Layer).ID
		for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
			target := arena.EdgeTargetNode(edgeID)
			if target == nodeID {
				continue
			}
			targetLayer := arena.Layer(*arena.Node(target).Layer)
			if targetLayer.ID <= layerIndex {
				newIndex := layerIndex + 1
				var newLayer layered.LayerID
				if newIndex == len(graph.Layers) {
					newLayer = arena.AddLayer(graphID)
					arena.Layer(newLayer).ID = newIndex
					graph.Layers = append(graph.Layers, newLayer)
				} else {
					newLayer = graph.Layers[newIndex]
				}
				arena.SetNodeLayer(target, &newLayer)
				checkNode(target)
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
}

// randomDAG builds, from seed, a graph of random nodes (random x spans)
// with ports and edges from earlier to later nodes (parallel edges and
// self-loops included).
func randomDAG(seed int64) (*layered.LGraphArena, layered.LGraphID) {
	random := rand.New(rand.NewSource(seed))
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	count := 1 + random.Intn(14)
	var ports [][]layered.LPortID
	for index := 0; index < count; index++ {
		nodeID := arena.AddNode(graphID)
		arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, nodeID)
		arena.Node(nodeID).Position.X = float64(random.Intn(60))
		arena.Node(nodeID).Size.X = float64(random.Intn(20))
		var nodePorts []layered.LPortID
		for portCount := 1 + random.Intn(3); portCount > 0; portCount-- {
			portID := arena.AddPort()
			arena.SetPortNode(portID, &nodeID)
			nodePorts = append(nodePorts, portID)
		}
		ports = append(ports, nodePorts)
	}
	for edges := random.Intn(3 * count); edges > 0; edges-- {
		from := random.Intn(count)
		to := from + random.Intn(count-from)
		source := ports[from][random.Intn(len(ports[from]))]
		target := ports[to][random.Intn(len(ports[to]))]
		edgeID := arena.AddEdge()
		arena.SetEdgeSource(edgeID, &source)
		arena.SetEdgeTarget(edgeID, &target)
	}
	return arena, graphID
}

// describeLayering is everything a layerer writes: the graph's layers with their
// IDs and nodes in order, every node's layer and ID, and the arena's layers.
func describeLayering(arena *layered.LGraphArena, graphID layered.LGraphID) string {
	out := fmt.Sprint(len(arena.Layers), arena.Graph(graphID).LayerlessNodes, "|")
	for _, layerID := range arena.Graph(graphID).Layers {
		out += fmt.Sprint(layerID, arena.Layer(layerID).ID, arena.Layer(layerID).Nodes, ";")
	}
	for index := range arena.Nodes {
		node := &arena.Nodes[index]
		layer := -1
		if node.Layer != nil {
			layer = int(*node.Layer)
		}
		out += fmt.Sprint(" ", node.ID, ":", layer)
	}
	return out
}

// TestIterativeLayerersMatchRecursion compares the iterative
// longest-path layerer and interactive layer checks with the recursive
// originals on random DAGs.
func TestIterativeLayerersMatchRecursion(t *testing.T) {
	for seed := int64(1); seed <= 3000; seed++ {
		want, wantGraph := randomDAG(seed)
		got, gotGraph := randomDAG(seed)
		recursiveLongestPath(want, wantGraph)
		if err := (&LongestPathLayerer{}).Process(got, gotGraph); err != nil {
			t.Fatal(err)
		}
		if describeLayering(want, wantGraph) != describeLayering(got, gotGraph) {
			t.Fatalf("longest path, seed %d:\nrecursive %s\niterative %s", seed, describeLayering(want, wantGraph), describeLayering(got, gotGraph))
		}

		// The interactive layerer: its first half (initial layers from the x
		// spans) is unchanged, so the reference runs it too and then the
		// recursive checks.
		want, wantGraph = randomDAG(seed)
		got, gotGraph = randomDAG(seed)
		if err := (&InteractiveLayerer{}).Process(got, gotGraph); err != nil {
			t.Fatal(err)
		}
		interactiveInitialLayers(want, wantGraph)
		recursiveInteractiveChecks(want, wantGraph)
		if describeLayering(want, wantGraph) != describeLayering(got, gotGraph) {
			t.Fatalf("interactive, seed %d:\nrecursive %s\niterative %s", seed, describeLayering(want, wantGraph), describeLayering(got, gotGraph))
		}
	}
}

// interactiveInitialLayers is the interactive layerer's first half: the
// layers made from overlapping x spans.
func interactiveInitialLayers(arena *layered.LGraphArena, graphID layered.LGraphID) {
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
}
