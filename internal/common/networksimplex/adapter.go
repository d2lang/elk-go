// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package networksimplex

import "github.com/d2lang/elk-go/internal/reuse"

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

	// Index, when set, returns a node's position in Nodes, or -1 for a node
	// that is not in Nodes (Nodes must not repeat a node). LayerInto then
	// finds nodes by position instead of through a map, with the same
	// answers (a node not in Nodes maps to node 0, the map's zero value).
	Index func(N) int
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
	result := make(map[N]int, len(adapter.Nodes))
	LayerInto(adapter, options, func(original N, layer int) { result[original] = layer })
	return result
}

// layerWorkspace is the graph, simplex and node map one LayerInto call
// uses; it is reused by later calls.
type layerWorkspace struct {
	graph   Graph
	simplex NetworkSimplex
	nodeIDs any // map[N]NodeID of the last instantiation used
	byIndex []NodeID
}

var layerWorkspaces reuse.Pool[layerWorkspace]

// maxSlotLayerNodes bounds the node slots of a workspace that goes to
// layerWorkspaces' slot (it may stay alive indefinitely), and
// maxSlotLayerBytes its storage (see retainedBytes); a larger one goes to the
// sync.Pool only under the gc toolchain, which garbage collection empties,
// and is dropped under TinyGo (see reuse.Pool and reuse.OversizedToPool).
const (
	maxSlotLayerNodes = 1 << 12
	maxSlotLayerBytes = 1 << 20
)

// retainedBytes bounds the bytes of storage workspace keeps on a 64-bit
// platform: the reset graph, the simplex's buffers, the node map (of at most
// maxSlotLayerNodes entries: a kept workspace never had more nodes) and the
// node index. Element sizes are checked by a test.
func (workspace *layerWorkspace) retainedBytes() int {
	simplex := &workspace.simplex
	bytes := workspace.graph.retainedBytes() + cap(workspace.byIndex)*4 +
		(cap(simplex.previousLayeringNodeCounts)+cap(simplex.postOrderID)+cap(simplex.lowestPostOrderID)+
			cap(simplex.cutvalue)+cap(simplex.incident)+cap(simplex.filling))*8 +
		(cap(simplex.edges)+cap(simplex.treeEdges)+cap(simplex.sources)+cap(simplex.queue)+cap(simplex.leaves))*4 +
		cap(simplex.edgeVisited) + cap(simplex.subtreeStack)*8 + cap(simplex.walk)*24
	if workspace.nodeIDs != nil {
		bytes += maxSlotLayerNodes * 48
	}
	return bytes
}

// LayerInto is Layer reporting each node's layer to assign, in adapter.Nodes
// order, instead of building a map.
func LayerInto[N comparable, E comparable](adapter Adapter[N, E], options Options, assign func(N, int)) {
	workspace := layerWorkspaces.Get()
	if workspace == nil {
		workspace = &layerWorkspace{}
	}
	layerIn(workspace, adapter, options, assign)
	// The workspace goes back to the pool only from a call that returned:
	// one that panics (network simplex panics on some inputs, as ELK's does)
	// may leave it half-updated, so it is dropped with the call instead.
	workspace.graph.reset()
	workspace.simplex.resetFor(nil)
	layerWorkspaces.Put(workspace, cap(workspace.graph.nodes) <= maxSlotLayerNodes && workspace.retainedBytes() <= maxSlotLayerBytes)
}

// layerIn is LayerInto with its workspace acquired.
func layerIn[N comparable, E comparable](
	workspace *layerWorkspace,
	adapter Adapter[N, E],
	options Options,
	assign func(N, int),
) {
	graph := &workspace.graph
	var nodeIDs map[N]NodeID
	byIndex := workspace.byIndex[:0]
	if adapter.Index == nil {
		var reused bool
		nodeIDs, reused = workspace.nodeIDs.(map[N]NodeID)
		if reused {
			for original := range nodeIDs {
				delete(nodeIDs, original)
			}
		} else {
			nodeIDs = make(map[N]NodeID, len(adapter.Nodes))
			workspace.nodeIDs = nodeIDs
		}
	}
	for _, original := range adapter.Nodes {
		id := graph.AddNode()
		graph.Node(id).Origin = original
		if nodeIDs != nil {
			nodeIDs[original] = id
		} else {
			byIndex = append(byIndex, id)
		}
	}
	workspace.byIndex = byIndex
	lookup := func(original N) NodeID {
		if nodeIDs != nil {
			return nodeIDs[original]
		}
		if index := adapter.Index(original); index >= 0 && index < len(byIndex) {
			return byIndex[index]
		}
		return 0
	}
	for _, originalNode := range adapter.Nodes {
		for _, originalEdge := range adapter.Outgoing(originalNode) {
			edge := graph.AddEdge(
				lookup(adapter.Source(originalEdge)),
				lookup(adapter.Target(originalEdge)),
				adapter.Weight(originalEdge),
				adapter.MinLength(originalEdge),
			)
			graph.Edge(edge).Origin = originalEdge
		}
	}
	simplex := &workspace.simplex
	simplex.resetFor(graph)
	simplex.WithBalancing(options.Balance)
	if options.PreviousLayeringNodeCounts != nil {
		simplex.WithPreviousLayering(options.PreviousLayeringNodeCounts)
	}
	if options.IterationLimit != 0 {
		simplex.WithIterationLimit(options.IterationLimit)
	}
	simplex.Execute()
	for _, original := range adapter.Nodes {
		assign(original, graph.Node(lookup(original)).Layer)
	}
}
