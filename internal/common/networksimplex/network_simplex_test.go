// Copyright (c) 2015, 2020 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package networksimplex

import (
	"reflect"
	"testing"
)

func nodeLayers(graph *Graph, nodes []NodeID) []int {
	result := make([]int, len(nodes))
	for index, node := range nodes {
		result[index] = graph.Node(node).Layer
	}
	return result
}

func TestDiamondWithChain(t *testing.T) {
	graph := NewGraph()
	nodes := make([]NodeID, 5)
	for index := range nodes {
		nodes[index] = graph.AddNode()
	}
	graph.AddEdge(nodes[0], nodes[1], 1, 1)
	graph.AddEdge(nodes[0], nodes[2], 1, 1)
	graph.AddEdge(nodes[1], nodes[3], 1, 1)
	graph.AddEdge(nodes[2], nodes[3], 1, 1)
	graph.AddEdge(nodes[3], nodes[4], 1, 1)
	ForGraph(graph).WithBalancing(true).Execute()
	if got, want := nodeLayers(graph, nodes), []int{0, 1, 1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("layers = %v, want %v", got, want)
	}
}

func TestMinimalSlackTreeShift(t *testing.T) {
	graph := NewGraph()
	a, b, c, d, x := graph.AddNode(), graph.AddNode(), graph.AddNode(), graph.AddNode(), graph.AddNode()
	graph.AddEdge(a, b, 1, 1)
	graph.AddEdge(b, c, 1, 1)
	graph.AddEdge(c, d, 1, 1)
	graph.AddEdge(x, d, 1, 1)
	ForGraph(graph).Execute()
	if got, want := nodeLayers(graph, []NodeID{a, b, c, d, x}), []int{0, 1, 2, 3, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("layers = %v, want %v", got, want)
	}
}

func TestExchangeAndBalancing(t *testing.T) {
	build := func() (*Graph, []NodeID) {
		graph := NewGraph()
		n := []NodeID{graph.AddNode(), graph.AddNode(), graph.AddNode(), graph.AddNode(), graph.AddNode()}
		graph.AddEdge(n[0], n[1], 1, 1)
		graph.AddEdge(n[0], n[4], 1, 1)
		graph.AddEdge(n[1], n[2], 1, 1)
		graph.AddEdge(n[2], n[3], 1, 1)
		graph.AddEdge(n[4], n[3], 10, 1)
		return graph, n
	}
	graph, nodes := build()
	ForGraph(graph).Execute()
	if got, want := nodeLayers(graph, nodes), []int{0, 1, 2, 3, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unbalanced layers = %v, want %v", got, want)
	}
	graph, nodes = build()
	ForGraph(graph).WithBalancing(true).Execute()
	if got, want := nodeLayers(graph, nodes), []int{0, 1, 2, 3, 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("balanced layers = %v, want %v", got, want)
	}
}

func TestSubtreeRemovalOnLongChain(t *testing.T) {
	graph := NewGraph()
	nodes := make([]NodeID, 45)
	for index := range nodes {
		nodes[index] = graph.AddNode()
		if index > 0 {
			graph.AddEdge(nodes[index-1], nodes[index], 1, 1)
		}
	}
	ForGraph(graph).WithBalancing(true).Execute()
	for index, layer := range nodeLayers(graph, nodes) {
		if layer != index {
			t.Fatalf("node %d layer = %d, want %d", index, layer, index)
		}
	}
}

func TestGraphUtilities(t *testing.T) {
	graph := NewGraph()
	a, b, c := graph.AddNode(), graph.AddNode(), graph.AddNode()
	edge := graph.AddEdge(a, b, 1, 1)
	if !graph.IsAcyclic() {
		t.Fatal("acyclic graph reported cyclic")
	}
	root, created := graph.MakeConnected()
	if !created || len(graph.Node(root).OutgoingEdges) != 2 || graph.Edge(graph.Node(root).OutgoingEdges[0]).Target != a || graph.Edge(graph.Node(root).OutgoingEdges[1]).Target != c {
		t.Fatalf("unexpected artificial root: created=%v root=%+v", created, graph.Node(root))
	}
	graph.ReverseEdge(edge)
	if graph.Edge(edge).Source != b || graph.Edge(edge).Target != a {
		t.Fatalf("reversed edge = %+v", graph.Edge(edge))
	}
	cyclic := NewGraph()
	x, y := cyclic.AddNode(), cyclic.AddNode()
	cyclic.AddEdge(x, y, 1, 1)
	cyclic.AddEdge(y, x, 1, 1)
	if cyclic.IsAcyclic() {
		t.Fatal("cycle reported acyclic")
	}
}

func TestCallbackAdapter(t *testing.T) {
	type edge struct{ source, target int }
	edges := []edge{{0, 1}, {0, 2}, {1, 3}, {2, 3}}
	outgoing := [][]int{{0, 1}, {2}, {3}, nil}
	layers := Layer(Adapter[int, int]{
		Nodes:     []int{0, 1, 2, 3},
		Outgoing:  func(node int) []int { return outgoing[node] },
		Source:    func(edgeID int) int { return edges[edgeID].source },
		Target:    func(edgeID int) int { return edges[edgeID].target },
		Weight:    func(int) float64 { return 1 },
		MinLength: func(int) int { return 1 },
	}, Options{Balance: true})
	for node, want := range map[int]int{0: 0, 1: 1, 2: 1, 3: 2} {
		if got := layers[node]; got != want {
			t.Fatalf("node %d layer = %d, want %d", node, got, want)
		}
	}
}
