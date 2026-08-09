// Copyright (c) 2016 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package networksimplex

import "math"

const (
	removeSubtreesThreshold = 40
	fuzzySmallerThanZero    = -1e-10
)

// NetworkSimplex determines an optimal weighted layering of an acyclic graph.
type NetworkSimplex struct {
	graph                      *Graph
	previousLayeringNodeCounts []int
	balance                    bool
	iterationLimit             int

	edges             []EdgeID
	treeEdges         []EdgeID
	sources           []NodeID
	edgeVisited       []bool
	postOrder         int
	postOrderID       []int
	lowestPostOrderID []int
	cutvalue          []float64
	subtreeStack      []subtreeEntry
}

type subtreeEntry struct {
	node NodeID
	edge EdgeID
}

func ForGraph(graph *Graph) *NetworkSimplex {
	return &NetworkSimplex{graph: graph, iterationLimit: math.MaxInt}
}

func (simplex *NetworkSimplex) WithBalancing(balance bool) *NetworkSimplex {
	simplex.balance = balance
	return simplex
}

func (simplex *NetworkSimplex) WithPreviousLayering(nodeCounts []int) *NetworkSimplex {
	simplex.previousLayeringNodeCounts = append([]int(nil), nodeCounts...)
	return simplex
}

func (simplex *NetworkSimplex) WithIterationLimit(limit int) *NetworkSimplex {
	simplex.iterationLimit = limit
	return simplex
}

// Execute computes the layering and stores each result in Node.Layer.
func (simplex *NetworkSimplex) Execute() {
	if len(simplex.graph.Nodes) == 0 {
		return
	}
	for _, node := range append([]NodeID(nil), simplex.graph.Nodes...) {
		simplex.graph.Node(node).Layer = 0
	}
	removeSubtrees := len(simplex.graph.Nodes) >= removeSubtreesThreshold
	if removeSubtrees {
		simplex.removeSubtrees()
	}
	simplex.initialize()
	simplex.feasibleTree()
	leave, found := simplex.leaveEdge()
	for iterations := 0; found && iterations < simplex.iterationLimit; iterations++ {
		enter, ok := simplex.enterEdge(leave)
		if !ok {
			panic("no entering edge found")
		}
		simplex.exchange(leave, enter)
		leave, found = simplex.leaveEdge()
	}
	if removeSubtrees {
		simplex.reattachSubtrees()
	}
	filling := simplex.normalize()
	if simplex.balance {
		simplex.balanceLayers(filling)
	}
}

func (simplex *NetworkSimplex) initialize() {
	nodeCount := len(simplex.graph.Nodes)
	for _, node := range append([]NodeID(nil), simplex.graph.Nodes...) {
		simplex.graph.Node(node).treeNode = false
	}
	simplex.postOrderID = make([]int, nodeCount)
	simplex.lowestPostOrderID = make([]int, nodeCount)
	simplex.sources = simplex.sources[:0]

	edges := make([]EdgeID, 0)
	for index, node := range append([]NodeID(nil), simplex.graph.Nodes...) {
		simplex.graph.Node(node).internalID = index
		if len(simplex.graph.Node(node).IncomingEdges) == 0 {
			simplex.sources = append(simplex.sources, node)
		}
		edges = append(edges, simplex.graph.Node(node).OutgoingEdges...)
	}
	for index, edge := range edges {
		simplex.graph.Edge(edge).internalID = index
		simplex.graph.Edge(edge).treeEdge = false
	}
	simplex.cutvalue = make([]float64, len(edges))
	simplex.edgeVisited = make([]bool, len(edges))
	simplex.edges = edges
	simplex.treeEdges = make([]EdgeID, 0, len(edges))
	simplex.postOrder = 1
}

func (simplex *NetworkSimplex) removeSubtrees() {
	simplex.subtreeStack = simplex.subtreeStack[:0]
	queue := make([]NodeID, 0)
	for _, node := range simplex.graph.Nodes {
		if len(simplex.graph.Node(node).ConnectedEdges()) == 1 {
			queue = append(queue, node)
		}
	}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		connected := simplex.graph.Node(node).ConnectedEdges()
		if len(connected) == 0 {
			continue
		}
		edge := connected[0]
		isOutgoing := len(simplex.graph.Node(node).OutgoingEdges) != 0
		other := simplex.graph.Edge(edge).Other(node)
		if isOutgoing {
			simplex.graph.Node(other).IncomingEdges = removeFirst(simplex.graph.Node(other).IncomingEdges, edge)
		} else {
			simplex.graph.Node(other).OutgoingEdges = removeFirst(simplex.graph.Node(other).OutgoingEdges, edge)
		}
		if len(simplex.graph.Node(other).ConnectedEdges()) == 1 {
			queue = append(queue, other)
		}
		simplex.subtreeStack = append(simplex.subtreeStack, subtreeEntry{node: node, edge: edge})
		simplex.graph.Nodes = removeFirst(simplex.graph.Nodes, node)
	}
}

func (simplex *NetworkSimplex) reattachSubtrees() {
	for len(simplex.subtreeStack) > 0 {
		last := len(simplex.subtreeStack) - 1
		entry := simplex.subtreeStack[last]
		simplex.subtreeStack = simplex.subtreeStack[:last]
		placed := simplex.graph.Edge(entry.edge).Other(entry.node)
		if simplex.graph.Edge(entry.edge).Target == entry.node {
			simplex.graph.Node(placed).OutgoingEdges = append(simplex.graph.Node(placed).OutgoingEdges, entry.edge)
			simplex.graph.Node(entry.node).Layer = simplex.graph.Node(placed).Layer + simplex.graph.Edge(entry.edge).Delta
		} else {
			simplex.graph.Node(placed).IncomingEdges = append(simplex.graph.Node(placed).IncomingEdges, entry.edge)
			simplex.graph.Node(entry.node).Layer = simplex.graph.Node(placed).Layer - simplex.graph.Edge(entry.edge).Delta
		}
		simplex.graph.Nodes = append(simplex.graph.Nodes, entry.node)
	}
}

func (simplex *NetworkSimplex) feasibleTree() {
	simplex.topologicalLayering()
	if len(simplex.edges) == 0 {
		return
	}
	clear(simplex.edgeVisited)
	for simplex.tightTreeDFS(simplex.graph.Nodes[0]) < len(simplex.graph.Nodes) {
		edgeID, found := simplex.minimalSlack()
		if !found {
			panic("no minimal-slack edge found")
		}
		edge := simplex.graph.Edge(edgeID)
		slack := simplex.graph.Node(edge.Target).Layer - simplex.graph.Node(edge.Source).Layer - edge.Delta
		if simplex.graph.Node(edge.Target).treeNode {
			slack = -slack
		}
		for _, node := range append([]NodeID(nil), simplex.graph.Nodes...) {
			if simplex.graph.Node(node).treeNode {
				simplex.graph.Node(node).Layer += slack
			}
		}
		clear(simplex.edgeVisited)
	}
	clear(simplex.edgeVisited)
	simplex.postorderTraversal(simplex.graph.Nodes[0])
	simplex.calculateCutvalues()
}

func (simplex *NetworkSimplex) topologicalLayering() {
	incident := make([]int, len(simplex.graph.Nodes))
	for _, node := range simplex.graph.Nodes {
		incident[simplex.graph.Node(node).internalID] += len(simplex.graph.Node(node).IncomingEdges)
	}
	queue := append([]NodeID(nil), simplex.sources...)
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		for _, edgeID := range append([]EdgeID(nil), simplex.graph.Node(node).OutgoingEdges...) {
			edge := simplex.graph.Edge(edgeID)
			target := edge.Target
			newLayer := simplex.graph.Node(node).Layer + edge.Delta
			simplex.graph.Node(target).Layer = max(simplex.graph.Node(target).Layer, newLayer)
			targetIndex := simplex.graph.Node(target).internalID
			incident[targetIndex]--
			if incident[targetIndex] == 0 {
				queue = append(queue, target)
			}
		}
	}
}

func (simplex *NetworkSimplex) minimalSpan(node NodeID) (int, int) {
	minOutgoing, minIncoming := math.MaxInt, math.MaxInt
	for _, edgeID := range simplex.graph.Node(node).ConnectedEdges() {
		edge := simplex.graph.Edge(edgeID)
		span := simplex.graph.Node(edge.Target).Layer - simplex.graph.Node(edge.Source).Layer
		if edge.Target == node && span < minIncoming {
			minIncoming = span
		} else if span < minOutgoing {
			minOutgoing = span
		}
	}
	if minIncoming == math.MaxInt {
		minIncoming = -1
	}
	if minOutgoing == math.MaxInt {
		minOutgoing = -1
	}
	return minIncoming, minOutgoing
}

func (simplex *NetworkSimplex) tightTreeDFS(node NodeID) int {
	nodeCount := 1
	simplex.graph.Node(node).treeNode = true
	for _, edgeID := range simplex.graph.Node(node).ConnectedEdges() {
		internalID := simplex.graph.Edge(edgeID).internalID
		if simplex.edgeVisited[internalID] {
			continue
		}
		simplex.edgeVisited[internalID] = true
		opposite := simplex.graph.Edge(edgeID).Other(node)
		if simplex.graph.Edge(edgeID).treeEdge {
			nodeCount += simplex.tightTreeDFS(opposite)
			continue
		}
		edge := simplex.graph.Edge(edgeID)
		tight := edge.Delta == simplex.graph.Node(edge.Target).Layer-simplex.graph.Node(edge.Source).Layer
		if !simplex.graph.Node(opposite).treeNode && tight {
			simplex.graph.Edge(edgeID).treeEdge = true
			simplex.treeEdges = append(simplex.treeEdges, edgeID)
			nodeCount += simplex.tightTreeDFS(opposite)
		}
	}
	return nodeCount
}

func (simplex *NetworkSimplex) minimalSlack() (EdgeID, bool) {
	minimum := math.MaxInt
	var result EdgeID
	found := false
	for _, edgeID := range simplex.edges {
		edge := simplex.graph.Edge(edgeID)
		if simplex.graph.Node(edge.Source).treeNode == simplex.graph.Node(edge.Target).treeNode {
			continue
		}
		slack := simplex.graph.Node(edge.Target).Layer - simplex.graph.Node(edge.Source).Layer - edge.Delta
		if slack < minimum {
			minimum, result, found = slack, edgeID, true
		}
	}
	return result, found
}

func (simplex *NetworkSimplex) postorderTraversal(node NodeID) int {
	lowest := math.MaxInt
	for _, edgeID := range simplex.graph.Node(node).ConnectedEdges() {
		internalID := simplex.graph.Edge(edgeID).internalID
		if simplex.graph.Edge(edgeID).treeEdge && !simplex.edgeVisited[internalID] {
			simplex.edgeVisited[internalID] = true
			lowest = min(lowest, simplex.postorderTraversal(simplex.graph.Edge(edgeID).Other(node)))
		}
	}
	nodeIndex := simplex.graph.Node(node).internalID
	simplex.postOrderID[nodeIndex] = simplex.postOrder
	simplex.lowestPostOrderID[nodeIndex] = min(lowest, simplex.postOrder)
	simplex.postOrder++
	return simplex.lowestPostOrderID[nodeIndex]
}

func (simplex *NetworkSimplex) isInHead(node NodeID, edgeID EdgeID) bool {
	edge := simplex.graph.Edge(edgeID)
	source := simplex.graph.Node(edge.Source).internalID
	target := simplex.graph.Node(edge.Target).internalID
	nodeIndex := simplex.graph.Node(node).internalID
	if simplex.lowestPostOrderID[source] <= simplex.postOrderID[nodeIndex] &&
		simplex.postOrderID[nodeIndex] <= simplex.postOrderID[source] &&
		simplex.lowestPostOrderID[target] <= simplex.postOrderID[nodeIndex] &&
		simplex.postOrderID[nodeIndex] <= simplex.postOrderID[target] {
		return simplex.postOrderID[source] >= simplex.postOrderID[target]
	}
	return simplex.postOrderID[source] < simplex.postOrderID[target]
}

func (simplex *NetworkSimplex) calculateCutvalues() {
	leaves := make([]NodeID, 0)
	for _, node := range append([]NodeID(nil), simplex.graph.Nodes...) {
		simplex.graph.Node(node).unknownCutvalues = simplex.graph.Node(node).unknownCutvalues[:0]
		for _, edgeID := range simplex.graph.Node(node).ConnectedEdges() {
			if simplex.graph.Edge(edgeID).treeEdge {
				simplex.graph.Node(node).unknownCutvalues = append(simplex.graph.Node(node).unknownCutvalues, edgeID)
			}
		}
		if len(simplex.graph.Node(node).unknownCutvalues) == 1 {
			leaves = append(leaves, node)
		}
	}
	for _, leaf := range leaves {
		node := leaf
		for len(simplex.graph.Node(node).unknownCutvalues) == 1 {
			toDetermine := simplex.graph.Node(node).unknownCutvalues[0]
			determined := simplex.graph.Edge(toDetermine)
			index := determined.internalID
			simplex.cutvalue[index] = determined.Weight
			source, target := determined.Source, determined.Target
			for _, edgeID := range simplex.graph.Node(node).ConnectedEdges() {
				if edgeID == toDetermine {
					continue
				}
				edge := simplex.graph.Edge(edgeID)
				if edge.treeEdge {
					if source == edge.Source || target == edge.Target {
						simplex.cutvalue[index] -= simplex.cutvalue[edge.internalID] - edge.Weight
					} else {
						simplex.cutvalue[index] += simplex.cutvalue[edge.internalID] - edge.Weight
					}
				} else if node == source {
					if edge.Source == node {
						simplex.cutvalue[index] += edge.Weight
					} else {
						simplex.cutvalue[index] -= edge.Weight
					}
				} else if edge.Source == node {
					simplex.cutvalue[index] -= edge.Weight
				} else {
					simplex.cutvalue[index] += edge.Weight
				}
			}
			simplex.graph.Node(source).unknownCutvalues = removeFirst(simplex.graph.Node(source).unknownCutvalues, toDetermine)
			simplex.graph.Node(target).unknownCutvalues = removeFirst(simplex.graph.Node(target).unknownCutvalues, toDetermine)
			if source == node {
				node = target
			} else {
				node = source
			}
		}
	}
}

func (simplex *NetworkSimplex) leaveEdge() (EdgeID, bool) {
	for _, edgeID := range simplex.treeEdges {
		edge := simplex.graph.Edge(edgeID)
		if edge.treeEdge && simplex.cutvalue[edge.internalID] < fuzzySmallerThanZero {
			return edgeID, true
		}
	}
	return 0, false
}

func (simplex *NetworkSimplex) enterEdge(leave EdgeID) (EdgeID, bool) {
	if !simplex.graph.Edge(leave).treeEdge {
		panic("input edge is not a tree edge")
	}
	minimum := math.MaxInt
	var result EdgeID
	found := false
	for _, edgeID := range simplex.edges {
		edge := simplex.graph.Edge(edgeID)
		if simplex.isInHead(edge.Source, leave) && !simplex.isInHead(edge.Target, leave) {
			slack := simplex.graph.Node(edge.Target).Layer - simplex.graph.Node(edge.Source).Layer - edge.Delta
			if slack < minimum {
				minimum, result, found = slack, edgeID, true
			}
		}
	}
	return result, found
}

func (simplex *NetworkSimplex) exchange(leave, enter EdgeID) {
	if !simplex.graph.Edge(leave).treeEdge {
		panic("leave edge is not a tree edge")
	}
	if simplex.graph.Edge(enter).treeEdge {
		panic("enter edge is already a tree edge")
	}
	simplex.graph.Edge(leave).treeEdge = false
	simplex.treeEdges = removeFirst(simplex.treeEdges, leave)
	simplex.graph.Edge(enter).treeEdge = true
	simplex.treeEdges = append(simplex.treeEdges, enter)
	edge := simplex.graph.Edge(enter)
	delta := simplex.graph.Node(edge.Target).Layer - simplex.graph.Node(edge.Source).Layer - edge.Delta
	if !simplex.isInHead(edge.Target, leave) {
		delta = -delta
	}
	for _, node := range append([]NodeID(nil), simplex.graph.Nodes...) {
		if !simplex.isInHead(node, leave) {
			simplex.graph.Node(node).Layer += delta
		}
	}
	simplex.postOrder = 1
	clear(simplex.edgeVisited)
	simplex.postorderTraversal(simplex.graph.Nodes[0])
	simplex.calculateCutvalues()
}

func (simplex *NetworkSimplex) normalize() []int {
	highest, lowest := math.MinInt, math.MaxInt
	for _, node := range simplex.graph.Nodes {
		lowest = min(lowest, simplex.graph.Node(node).Layer)
		highest = max(highest, simplex.graph.Node(node).Layer)
	}
	filling := make([]int, highest-lowest+1)
	for _, node := range append([]NodeID(nil), simplex.graph.Nodes...) {
		simplex.graph.Node(node).Layer -= lowest
		filling[simplex.graph.Node(node).Layer]++
	}
	for layer, count := range simplex.previousLayeringNodeCounts {
		if layer == len(filling) {
			break
		}
		filling[layer] += count
	}
	return filling
}

func (simplex *NetworkSimplex) balanceLayers(filling []int) {
	for _, node := range append([]NodeID(nil), simplex.graph.Nodes...) {
		n := simplex.graph.Node(node)
		if len(n.IncomingEdges) != len(n.OutgoingEdges) {
			continue
		}
		newLayer := n.Layer
		minimumIncoming, minimumOutgoing := simplex.minimalSpan(node)
		for layer := n.Layer - minimumIncoming + 1; layer < n.Layer+minimumOutgoing; layer++ {
			if filling[layer] < filling[newLayer] {
				newLayer = layer
			}
		}
		if filling[newLayer] < filling[n.Layer] {
			filling[n.Layer]--
			filling[newLayer]++
			simplex.graph.Node(node).Layer = newLayer
		}
	}
}
