// Copyright (c) 2016 Kiel University and others.
// Copyright (c) 2016, 2017 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package networksimplex implements ELK's deterministic network-simplex
// layering primitive. The implementation is a direct port of ELK v0.8.1's
// org.eclipse.elk.alg.common.networksimplex package.
package networksimplex

import "fmt"

// NodeID and EdgeID are stable arena handles. Arena order is Java object
// creation order; Nodes separately preserves NGraph's mutable list order.
type NodeID uint32
type EdgeID uint32

// Node is the Go counterpart of NNode.
type Node struct {
	ID     int
	Origin any
	Layer  int

	IncomingEdges []EdgeID
	OutgoingEdges []EdgeID

	internalID       int
	treeNode         bool
	unknownCutvalues []EdgeID
}

// ConnectedEdges returns incoming edges followed by outgoing edges, matching
// NNode.getConnectedEdges' iteration order.
func (node *Node) ConnectedEdges() []EdgeID {
	edges := make([]EdgeID, 0, len(node.IncomingEdges)+len(node.OutgoingEdges))
	edges = append(edges, node.IncomingEdges...)
	edges = append(edges, node.OutgoingEdges...)
	return edges
}

// Edge is the Go counterpart of NEdge.
type Edge struct {
	ID     int
	Origin any
	Source NodeID
	Target NodeID
	Weight float64
	Delta  int

	internalID int
	treeEdge   bool
}

// Other returns the opposite endpoint of edge.
func (edge *Edge) Other(node NodeID) NodeID {
	switch node {
	case edge.Source:
		return edge.Target
	case edge.Target:
		return edge.Source
	default:
		panic(fmt.Sprintf("node %d is not part of edge %d", node, edge.ID))
	}
}

// Graph stores network-simplex nodes and edges in deterministic arenas.
type Graph struct {
	// Nodes is the mutable list ELK removes from and appends to while pruning
	// subtrees. It intentionally differs from the immutable node arena.
	Nodes []NodeID
	nodes []Node
	edges []Edge
}

func NewGraph() *Graph { return &Graph{} }

func (graph *Graph) Node(id NodeID) *Node { return &graph.nodes[int(id)] }
func (graph *Graph) Edge(id EdgeID) *Edge { return &graph.edges[int(id)] }
func (graph *Graph) EdgeCount() int       { return len(graph.edges) }

// AddNode creates and appends an NNode.
func (graph *Graph) AddNode() NodeID {
	id := NodeID(len(graph.nodes))
	graph.nodes = append(graph.nodes, Node{})
	graph.Nodes = append(graph.Nodes, id)
	return id
}

// AddEdge creates an NEdge and registers it with both endpoints.
func (graph *Graph) AddEdge(source, target NodeID, weight float64, delta int) EdgeID {
	if source == target {
		panic(fmt.Sprintf("network simplex does not support self-loops on node %d", source))
	}
	id := EdgeID(len(graph.edges))
	graph.edges = append(graph.edges, Edge{
		Source: source,
		Target: target,
		Weight: weight,
		Delta:  delta,
	})
	graph.Node(source).OutgoingEdges = append(graph.Node(source).OutgoingEdges, id)
	graph.Node(target).IncomingEdges = append(graph.Node(target).IncomingEdges, id)
	return id
}

// ReverseEdge reverses edge and updates endpoint incidence lists in the same
// order as NGraph.reverseEdge.
func (graph *Graph) ReverseEdge(id EdgeID) {
	edge := graph.Edge(id)
	edge.Source, edge.Target = edge.Target, edge.Source
	source, target := edge.Source, edge.Target
	graph.Node(target).OutgoingEdges = removeFirst(graph.Node(target).OutgoingEdges, id)
	graph.Node(target).IncomingEdges = append(graph.Node(target).IncomingEdges, id)
	graph.Node(source).IncomingEdges = removeFirst(graph.Node(source).IncomingEdges, id)
	graph.Node(source).OutgoingEdges = append(graph.Node(source).OutgoingEdges, id)
}

// MakeConnected adds one artificial root for a disconnected graph and joins
// each connected component to it with zero-weight, zero-delta edges. The
// second return value reports whether a root was created.
func (graph *Graph) MakeConnected() (NodeID, bool) {
	for index, node := range append([]NodeID(nil), graph.Nodes...) {
		graph.Node(node).internalID = index
	}
	representatives := graph.connectedComponentRepresentatives()
	if len(representatives) <= 1 {
		return 0, false
	}
	root := graph.AddNode()
	for _, node := range representatives {
		graph.AddEdge(root, node, 0, 0)
	}
	return root, true
}

func (graph *Graph) connectedComponentRepresentatives() []NodeID {
	representatives := make([]NodeID, 0)
	marked := make([]bool, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if !marked[graph.Node(node).internalID] {
			representatives = append(representatives, node)
			graph.markComponent(node, marked)
		}
	}
	return representatives
}

func (graph *Graph) markComponent(node NodeID, marked []bool) {
	index := graph.Node(node).internalID
	if marked[index] {
		return
	}
	marked[index] = true
	for _, edge := range graph.Node(node).ConnectedEdges() {
		graph.markComponent(graph.Edge(edge).Other(node), marked)
	}
}

// IsAcyclic creates the same topological numbering as NGraph.isAcyclic and
// reports whether every edge points forward in it.
func (graph *Graph) IsAcyclic() bool {
	for index, node := range append([]NodeID(nil), graph.Nodes...) {
		graph.Node(node).internalID = index
	}
	incident := make([]int, len(graph.Nodes))
	layers := make([]int, len(graph.Nodes))
	for _, node := range graph.Nodes {
		incident[graph.Node(node).internalID] += len(graph.Node(node).IncomingEdges)
	}
	queue := make([]NodeID, 0)
	for _, node := range graph.Nodes {
		if len(graph.Node(node).IncomingEdges) == 0 {
			queue = append(queue, node)
		}
	}
	if len(queue) == 0 && len(graph.Nodes) != 0 {
		return false
	}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		for _, edgeID := range graph.Node(node).OutgoingEdges {
			target := graph.Edge(edgeID).Target
			targetIndex := graph.Node(target).internalID
			nodeIndex := graph.Node(node).internalID
			layers[targetIndex] = max(layers[targetIndex], layers[nodeIndex]+1)
			incident[targetIndex]--
			if incident[targetIndex] == 0 {
				queue = append(queue, target)
			}
		}
	}
	for _, node := range graph.Nodes {
		for _, edgeID := range graph.Node(node).OutgoingEdges {
			edge := graph.Edge(edgeID)
			if layers[graph.Node(edge.Target).internalID] <= layers[graph.Node(edge.Source).internalID] {
				return false
			}
		}
	}
	return true
}

func removeFirst[T comparable](values []T, value T) []T {
	for index, candidate := range values {
		if candidate == value {
			copy(values[index:], values[index+1:])
			var zero T
			values[len(values)-1] = zero
			return values[:len(values)-1]
		}
	}
	return values
}
