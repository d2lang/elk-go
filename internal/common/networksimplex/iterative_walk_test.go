// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package networksimplex

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"testing"
)

// The recursive tree walks as they were before they were made iterative
// (their frames made long chains overflow a 1 MiB stack); the iterative ones must do exactly what these do.

func recursiveTightTreeDFS(simplex *NetworkSimplex, node NodeID) int {
	nodeCount := 1
	simplex.graph.Node(node).treeNode = true
	for _, edges := range simplex.graph.Node(node).connectedLists() {
		for _, edgeID := range edges {
			internalID := simplex.graph.Edge(edgeID).internalID
			if simplex.edgeVisited[internalID] {
				continue
			}
			simplex.edgeVisited[internalID] = true
			opposite := simplex.graph.Edge(edgeID).Other(node)
			if simplex.graph.Edge(edgeID).treeEdge {
				nodeCount += recursiveTightTreeDFS(simplex, opposite)
				continue
			}
			edge := simplex.graph.Edge(edgeID)
			tight := edge.Delta == simplex.graph.Node(edge.Target).Layer-simplex.graph.Node(edge.Source).Layer
			if !simplex.graph.Node(opposite).treeNode && tight {
				simplex.graph.Edge(edgeID).treeEdge = true
				simplex.treeEdges = append(simplex.treeEdges, edgeID)
				nodeCount += recursiveTightTreeDFS(simplex, opposite)
			}
		}
	}
	return nodeCount
}

func recursivePostorderTraversal(simplex *NetworkSimplex, node NodeID) int {
	lowest := math.MaxInt
	for _, edges := range simplex.graph.Node(node).connectedLists() {
		for _, edgeID := range edges {
			internalID := simplex.graph.Edge(edgeID).internalID
			if simplex.graph.Edge(edgeID).treeEdge && !simplex.edgeVisited[internalID] {
				simplex.edgeVisited[internalID] = true
				lowest = min(lowest, recursivePostorderTraversal(simplex, simplex.graph.Edge(edgeID).Other(node)))
			}
		}
	}
	nodeIndex := simplex.graph.Node(node).internalID
	simplex.postOrderID[nodeIndex] = simplex.postOrder
	simplex.lowestPostOrderID[nodeIndex] = min(lowest, simplex.postOrder)
	simplex.postOrder++
	return simplex.lowestPostOrderID[nodeIndex]
}

func recursiveMarkComponent(graph *Graph, node NodeID, marked []bool) {
	index := graph.Node(node).internalID
	if marked[index] {
		return
	}
	marked[index] = true
	for _, edges := range graph.Node(node).connectedLists() {
		for _, edge := range edges {
			recursiveMarkComponent(graph, graph.Edge(edge).Other(node), marked)
		}
	}
}

// randomWalkGraph is a random graph (parallel edges and both directions
// between a pair included; no self-loops) with random layers, deltas and
// tree-edge flags, initialized as the walks find it.
func randomWalkGraph(random *rand.Rand) *NetworkSimplex {
	graph := NewGraph()
	nodes := 1 + random.Intn(12)
	for index := 0; index < nodes; index++ {
		graph.Node(graph.AddNode()).Layer = random.Intn(4)
	}
	if nodes > 1 {
		for edges := random.Intn(3 * nodes); edges > 0; edges-- {
			source, target := NodeID(random.Intn(nodes)), NodeID(random.Intn(nodes))
			if source == target {
				continue
			}
			graph.AddEdge(source, target, float64(random.Intn(3)), random.Intn(3))
		}
	}
	simplex := ForGraph(graph)
	simplex.initialize()
	for index := range graph.edges {
		graph.edges[index].treeEdge = random.Intn(3) == 0
	}
	for index := range graph.nodes {
		graph.nodes[index].treeNode = random.Intn(4) == 0
	}
	for index := range simplex.edgeVisited {
		simplex.edgeVisited[index] = random.Intn(5) == 0
	}
	simplex.postOrder = random.Intn(3)
	return simplex
}

// cloneSimplex copies simplex and its graph deeply.
func cloneSimplex(simplex *NetworkSimplex) *NetworkSimplex {
	graph := &Graph{
		Nodes: append([]NodeID(nil), simplex.graph.Nodes...),
		nodes: append([]Node(nil), simplex.graph.nodes...),
		edges: append([]Edge(nil), simplex.graph.edges...),
	}
	for index := range graph.nodes {
		node := &graph.nodes[index]
		node.IncomingEdges = append([]EdgeID(nil), node.IncomingEdges...)
		node.OutgoingEdges = append([]EdgeID(nil), node.OutgoingEdges...)
		node.unknownCutvalues = append([]EdgeID(nil), node.unknownCutvalues...)
	}
	clone := *simplex
	clone.graph = graph
	clone.edges = append([]EdgeID(nil), simplex.edges...)
	clone.treeEdges = append([]EdgeID(nil), simplex.treeEdges...)
	clone.edgeVisited = append([]bool(nil), simplex.edgeVisited...)
	clone.postOrderID = append([]int(nil), simplex.postOrderID...)
	clone.lowestPostOrderID = append([]int(nil), simplex.lowestPostOrderID...)
	clone.walk = nil
	return &clone
}

// walkState is everything the walks can change.
func walkState(simplex *NetworkSimplex) string {
	var treeNodes, treeEdges []bool
	for _, node := range simplex.graph.nodes {
		treeNodes = append(treeNodes, node.treeNode)
	}
	for _, edge := range simplex.graph.edges {
		treeEdges = append(treeEdges, edge.treeEdge)
	}
	return fmt.Sprint(treeNodes, treeEdges, simplex.treeEdges, simplex.edgeVisited,
		simplex.postOrderID, simplex.lowestPostOrderID, simplex.postOrder)
}

func callRecovering(call func() int) (result int, panicked any) {
	defer func() { panicked = recover() }()
	return call(), nil
}

// TestIterativeWalksMatchRecursion compares the iterative
// tightTreeDFS, postorderTraversal and markComponent with the recursive
// originals on random graphs from every node: results, every flag, list and
// number they write (in particular the order of treeEdges), and panics.
func TestIterativeWalksMatchRecursion(t *testing.T) {
	random := rand.New(rand.NewSource(1))
	for round := 0; round < 3000; round++ {
		base := randomWalkGraph(random)
		for start := range base.graph.Nodes {
			node := base.graph.Nodes[start]
			walks := []struct {
				name                 string
				recursive, iterative func(*NetworkSimplex) int
			}{
				{"tightTreeDFS",
					func(simplex *NetworkSimplex) int { return recursiveTightTreeDFS(simplex, node) },
					func(simplex *NetworkSimplex) int { return simplex.tightTreeDFS(node) }},
				{"postorderTraversal",
					func(simplex *NetworkSimplex) int { return recursivePostorderTraversal(simplex, node) },
					func(simplex *NetworkSimplex) int { return simplex.postorderTraversal(node) }},
			}
			for _, walk := range walks {
				wantSimplex, gotSimplex := cloneSimplex(base), cloneSimplex(base)
				wantResult, wantPanic := callRecovering(func() int { return walk.recursive(wantSimplex) })
				gotResult, gotPanic := callRecovering(func() int { return walk.iterative(gotSimplex) })
				if wantResult != gotResult || fmt.Sprint(wantPanic) != fmt.Sprint(gotPanic) ||
					walkState(wantSimplex) != walkState(gotSimplex) {
					t.Fatalf("round %d, %s from %d: recursive %d %v %s, iterative %d %v %s", round, walk.name, node,
						wantResult, wantPanic, walkState(wantSimplex), gotResult, gotPanic, walkState(gotSimplex))
				}
			}
			for index, nodeID := range base.graph.Nodes {
				base.graph.Node(nodeID).internalID = index
			}
			want := make([]bool, len(base.graph.Nodes))
			got := make([]bool, len(base.graph.Nodes))
			for index := range want {
				want[index] = random.Intn(4) == 0
			}
			copy(got, want)
			recursiveMarkComponent(base.graph, node, want)
			base.graph.markComponent(node, got)
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("round %d, markComponent from %d: recursive %v, iterative %v", round, node, want, got)
			}
		}
	}
}
