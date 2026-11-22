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

// connectedLists returns the incoming and outgoing edge lists; iterating them
// in order visits ConnectedEdges' edges without copying them.
func (node *Node) connectedLists() [2][]EdgeID {
	return [2][]EdgeID{node.IncomingEdges, node.OutgoingEdges}
}

func (node *Node) connectedCount() int { return len(node.IncomingEdges) + len(node.OutgoingEdges) }

// firstConnected is ConnectedEdges()[0]; the node must have an edge.
func (node *Node) firstConnected() EdgeID {
	if len(node.IncomingEdges) > 0 {
		return node.IncomingEdges[0]
	}
	return node.OutgoingEdges[0]
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

// AddNode creates and appends an NNode. In a reset graph the new node reuses
// the (emptied) edge lists left in its slot by reset; it starts empty either
// way.
func (graph *Graph) AddNode() NodeID {
	id := NodeID(len(graph.nodes))
	var node Node
	if n := len(graph.nodes); n < cap(graph.nodes) {
		slot := &graph.nodes[:n+1][n]
		node.IncomingEdges = slot.IncomingEdges[:0]
		node.OutgoingEdges = slot.OutgoingEdges[:0]
		node.unknownCutvalues = slot.unknownCutvalues[:0]
	}
	graph.nodes = append(graph.nodes, node)
	graph.Nodes = append(graph.Nodes, id)
	return id
}

// reset empties graph for reuse, keeping its storage. Each node slot keeps
// only its emptied edge lists (which no other node shares); origins are
// dropped so nothing they reference is kept alive.
func (graph *Graph) reset() {
	for index := range graph.nodes {
		node := &graph.nodes[index]
		*node = Node{
			IncomingEdges:    keptEdgeList(node.IncomingEdges),
			OutgoingEdges:    keptEdgeList(node.OutgoingEdges),
			unknownCutvalues: keptEdgeList(node.unknownCutvalues),
		}
	}
	for index := range graph.edges {
		graph.edges[index] = Edge{}
	}
	graph.Nodes, graph.nodes, graph.edges = graph.Nodes[:0], graph.nodes[:0], graph.edges[:0]
}

// maxKeptEdgeList bounds the capacity of an edge list a reset node slot
// keeps for its next node (a larger list is dropped), so that the storage of
// a reused graph stays proportional to its node slots.
const maxKeptEdgeList = 32

func keptEdgeList(edges []EdgeID) []EdgeID {
	if cap(edges) > maxKeptEdgeList {
		return nil
	}
	return edges[:0]
}

// retainedBytes bounds the bytes of storage a reset graph keeps on a 64-bit
// platform (node slots with their kept edge lists, edge slots, the node list).
func (graph *Graph) retainedBytes() int {
	return cap(graph.Nodes)*4 + cap(graph.nodes)*(120+3*maxKeptEdgeList*4) + cap(graph.edges)*64
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

// markComponent marks node and every node connected to it. It is ELK's
// recursive markComponent made iterative, so that long chains need no deeper
// call stack: every read and write happens in the recursion's order (a call
// reads its node's index and mark, marks it, then makes a call for the other
// end of each of its incoming and then outgoing edges in order, each call
// completing before the next edge is looked at); the edge lists are read
// again at each step instead of being kept, as marking changes no list.
func (graph *Graph) markComponent(node NodeID, marked []bool) {
	index := graph.Node(node).internalID
	if marked[index] {
		return
	}
	marked[index] = true
	type frame struct {
		node NodeID
		list int32
		edge int32
	}
	frames := []frame{{node: node}}
	for len(frames) > 0 {
		top := &frames[len(frames)-1]
		edges := graph.Node(top.node).connectedLists()[top.list]
		if int(top.edge) >= len(edges) {
			if top.list == 0 {
				top.list, top.edge = 1, 0
			} else {
				frames = frames[:len(frames)-1]
			}
			continue
		}
		edge := edges[top.edge]
		top.edge++
		// The call markComponent(other end, marked).
		other := graph.Edge(edge).Other(top.node)
		index := graph.Node(other).internalID
		if marked[index] {
			continue
		}
		marked[index] = true
		frames = append(frames, frame{node: other})
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
