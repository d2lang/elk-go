// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package networksimplex

// Adapter lets a caller retain its own node and edge arena IDs while using
// the common network-simplex implementation. Nodes and outgoing edges must be
// supplied in their Java iteration order.
type Adapter[N comparable, E comparable] struct {
	Nodes     []N
	Outgoing  func(N) []E
	Source    func(E) N
	Target    func(E) N
	Weight    func(E) float64
	MinLength func(E) int
}

// Options configures the callback-based Layer entry point.
type Options struct {
	Balance                    bool
	PreviousLayeringNodeCounts []int
	IterationLimit             int
}

// Layer maps a caller-owned acyclic graph into ELK's common graph, executes
// network simplex, and returns layers keyed by the original node IDs.
func Layer[N comparable, E comparable](adapter Adapter[N, E], options Options) map[N]int {
	graph := NewGraph()
	nodeIDs := make(map[N]NodeID, len(adapter.Nodes))
	for _, original := range adapter.Nodes {
		id := graph.AddNode()
		graph.Node(id).Origin = original
		nodeIDs[original] = id
	}
	for _, originalNode := range adapter.Nodes {
		for _, originalEdge := range adapter.Outgoing(originalNode) {
			edge := graph.AddEdge(
				nodeIDs[adapter.Source(originalEdge)],
				nodeIDs[adapter.Target(originalEdge)],
				adapter.Weight(originalEdge),
				adapter.MinLength(originalEdge),
			)
			graph.Edge(edge).Origin = originalEdge
		}
	}
	simplex := ForGraph(graph).WithBalancing(options.Balance)
	if options.PreviousLayeringNodeCounts != nil {
		simplex.WithPreviousLayering(options.PreviousLayeringNodeCounts)
	}
	if options.IterationLimit != 0 {
		simplex.WithIterationLimit(options.IterationLimit)
	}
	simplex.Execute()
	result := make(map[N]int, len(adapter.Nodes))
	for _, original := range adapter.Nodes {
		result[original] = graph.Node(nodeIDs[original]).Layer
	}
	return result
}
