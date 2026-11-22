// Copyright (c) 2010, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package p2layers implements ELK Layered phase 2.
package p2layers

import (
	"math"

	"github.com/d2lang/elk-go/internal/common/networksimplex"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

const iterationLimitFactor = 4

type NetworkSimplexLayerer struct{}

func (*NetworkSimplexLayerer) ProcessorConfiguration(
	_ *layered.LGraphArena,
	_ layered.LGraphID,
) layered.ProcessorConfiguration {
	return layered.NetworkSimplexLayeringConfiguration()
}

// layeringScratch is NetworkSimplexLayerer.Process's working storage, kept in
// the arena (layered.ScratchNetworkSimplexLayering); every part is reset or
// overwritten before it is read.
type layeringScratch struct {
	visited         []bool
	all             []layered.LNodeID
	components      [][]layered.LNodeID
	componentOf     []int32
	componentLayers []int
	outgoing        []layered.LEdgeID
	previous        []int
	visits          []visitFrame
}

func (*NetworkSimplexLayerer) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	graph := arena.Graph(graphID)
	nodes := graph.LayerlessNodes
	if len(nodes) == 0 {
		return nil
	}
	if len(nodes) == 1 && len(arena.Node(nodes[0]).Ports) == 0 {
		// One node without ports (so without edges): what the general path
		// does to it. connectedComponentsInto numbers it 0 and makes it the only
		// component; network simplex on one node without edges gives layer 0
		// (normalize) and no previous counts are used; the first layer is
		// created if the graph has none and the node is put into it.
		// Thoroughness has a non-materializing default, so its read has no
		// effect; the rest of the general path only touches the arena's
		// scratch storage.
		nodeID := nodes[0]
		arena.Node(nodeID).ID = 0
		for len(graph.Layers) == 0 {
			arena.AddLayerToGraph(graphID)
		}
		layerID := graph.Layers[0]
		arena.SetNodeLayer(nodeID, &layerID)
		graph.LayerlessNodes = graph.LayerlessNodes[:0]
		return nil
	}
	scratch, _ := arena.Scratch(layered.ScratchNetworkSimplexLayering).(*layeringScratch)
	if scratch == nil {
		scratch = &layeringScratch{}
		arena.SetScratch(layered.ScratchNetworkSimplexLayering, scratch)
	}
	components := connectedComponentsInto(arena, nodes, scratch)
	thoroughness := basegraph.Get(&graph.Properties, layered.Thoroughness) * iterationLimitFactor
	var previousLayeringNodeCounts []int
	var componentLayers []int

	// connectedComponentsInto numbered the nodes by their position in nodes
	// (node.ID). Membership in the current component is a per-position
	// component number (a node is a member when it is at its position in
	// nodes and carries the number), and a node's position in its component
	// is kept per position too. One edge buffer serves all components: Layer
	// consumes each outgoing list before asking for the next.
	// (Should a component hold a node that is not in nodes, the sets are
	// kept in a map instead, as before.)
	componentOf := scratch.componentOf
	if cap(componentOf) < 2*len(nodes) {
		componentOf = make([]int32, 2*len(nodes))
	} else {
		componentOf = componentOf[:2*len(nodes)]
		clear(componentOf)
	}
	scratch.componentOf = componentOf
	positionIn := componentOf[len(nodes):]
	componentOf = componentOf[:len(nodes)]
	for index := range componentOf {
		componentOf[index] = -1
	}
	member := func(nodeID layered.LNodeID) int {
		index := arena.Node(nodeID).ID
		if index >= 0 && index < len(nodes) && nodes[index] == nodeID {
			return index
		}
		return -1
	}
	dense := true
	for _, component := range components {
		for _, nodeID := range component {
			if member(nodeID) < 0 {
				dense = false
			}
		}
	}
	var componentSet map[layered.LNodeID]struct{}
	if !dense {
		componentSet = make(map[layered.LNodeID]struct{})
	}
	outgoingEdges := scratch.outgoing[:0]
	componentLayers = scratch.componentLayers[:0]
	for componentNumber, component := range components {
		if dense {
			for position, nodeID := range component {
				index := member(nodeID)
				componentOf[index] = int32(componentNumber)
				positionIn[index] = int32(position)
			}
		} else {
			for nodeID := range componentSet {
				delete(componentSet, nodeID)
			}
			for _, nodeID := range component {
				componentSet[nodeID] = struct{}{}
			}
		}
		current := int32(componentNumber)
		inComponent := func(nodeID layered.LNodeID) int {
			if index := member(nodeID); index >= 0 && componentOf[index] == current {
				return index
			}
			return -1
		}
		inside := func(nodeID layered.LNodeID) bool {
			if !dense {
				_, found := componentSet[nodeID]
				return found
			}
			return inComponent(nodeID) >= 0
		}
		outgoing := func(nodeID layered.LNodeID) []layered.LEdgeID {
			edges := outgoingEdges[:0]
			// arena.NodeOutgoingEdges order, without the copy.
			for _, portID := range arena.Node(nodeID).Ports {
				for _, edgeID := range arena.Port(portID).OutgoingEdges {
					if arena.EdgeIsSelfLoop(edgeID) {
						continue
					}
					if inside(arena.EdgeTargetNode(edgeID)) {
						edges = append(edges, edgeID)
					}
				}
			}
			outgoingEdges = edges
			return edges
		}
		iterationLimit := thoroughness * int(math.Sqrt(float64(len(component))))
		// Layers arrive in component order (Layer's map, read back in that
		// order, without the map).
		componentLayers = componentLayers[:0]
		var index func(layered.LNodeID) int
		if dense {
			index = func(nodeID layered.LNodeID) int {
				if member := inComponent(nodeID); member >= 0 {
					return int(positionIn[member])
				}
				return -1
			}
		}
		networksimplex.LayerInto(networksimplex.Adapter[layered.LNodeID, layered.LEdgeID]{
			Nodes:    component,
			Outgoing: outgoing,
			Source:   arena.EdgeSourceNode,
			Target:   arena.EdgeTargetNode,
			Weight: func(edgeID layered.LEdgeID) float64 {
				priority := basegraph.Get(&arena.Edge(edgeID).Properties, layered.PriorityShortness)
				if priority < 1 {
					priority = 1
				}
				return float64(priority)
			},
			MinLength: func(layered.LEdgeID) int { return 1 },
			Index:     index,
		}, networksimplex.Options{
			Balance:                    true,
			PreviousLayeringNodeCounts: previousLayeringNodeCounts,
			IterationLimit:             iterationLimit,
		}, func(_ layered.LNodeID, layer int) { componentLayers = append(componentLayers, layer) })

		for index, nodeID := range component {
			layerIndex := componentLayers[index]
			for len(graph.Layers) <= layerIndex {
				arena.AddLayerToGraph(graphID)
			}
			layerID := graph.Layers[layerIndex]
			arena.SetNodeLayer(nodeID, &layerID)
		}
		if len(components) > 1 {
			// LayerInto copies the counts it is given, so one buffer serves
			// every component.
			counts := scratch.previous
			if cap(counts) < len(graph.Layers) {
				counts = make([]int, len(graph.Layers))
			}
			counts = counts[:len(graph.Layers)]
			scratch.previous = counts
			previousLayeringNodeCounts = counts
			for index, layerID := range graph.Layers {
				previousLayeringNodeCounts[index] = len(arena.Layer(layerID).Nodes)
			}
		}
	}
	graph.LayerlessNodes = graph.LayerlessNodes[:0]
	scratch.outgoing, scratch.componentLayers = outgoingEdges[:0], componentLayers[:0]
	return nil
}

// connectedComponentsInto numbers nodes by their position (node.ID) and
// returns their connected components, a component larger than the first one
// so far going first and any other last, cut from one array in scratch (each
// capacity-capped; they are only read).
func connectedComponentsInto(
	arena *layered.LGraphArena,
	nodes []layered.LNodeID,
	scratch *layeringScratch,
) [][]layered.LNodeID {
	for index, nodeID := range nodes {
		arena.Node(nodeID).ID = index
	}
	visited := scratch.visited
	if cap(visited) < len(nodes) {
		visited = make([]bool, len(nodes))
	} else {
		visited = visited[:len(nodes)]
		clear(visited)
	}
	scratch.visited = visited
	all := scratch.all[:0]
	components := scratch.components[:0]
	for _, nodeID := range nodes {
		if visited[arena.Node(nodeID).ID] {
			continue
		}
		start := len(all)
		all, scratch.visits = visitComponent(arena, visited, all, nodeID, scratch.visits)
		component := all[start:len(all):len(all)]
		if len(components) == 0 || len(components[0]) < len(component) {
			components = append(components, nil)
			copy(components[1:], components[:len(components)-1])
			components[0] = component
		} else {
			components = append(components, component)
		}
	}
	scratch.all, scratch.components = all, components
	return components
}

// visitFrame is one node of visitComponent's walk: the node, its port being
// walked (index into its Ports), the port's edge list being walked (0:
// incoming, 1: outgoing) and the next edge's index in that list.
type visitFrame struct {
	node layered.LNodeID
	port int32
	list int32
	edge int32
}

// visitComponent appends nodeID and the not yet visited nodes reachable from
// it, depth first, to component (ELK's DFS visiting order). It is ELK's
// recursive visit made iterative, so that long chains need no deeper call
// stack: frames holds one frame per pending call (returned for reuse). Every
// read and write happens in the recursion's order: a node's ID, its visited
// mark, its addition to component; then for each of its ports in order, the
// port's incoming and then outgoing edges in order, each edge's other end
// and that end's visited mark, a visit of an unvisited end completing before
// the next edge is looked at. The lists are read again at each step instead
// of being kept: visiting changes no port or edge list.
func visitComponent(
	arena *layered.LGraphArena,
	visited []bool,
	component []layered.LNodeID,
	nodeID layered.LNodeID,
	frames []visitFrame,
) ([]layered.LNodeID, []visitFrame) {
	index := arena.Node(nodeID).ID
	if visited[index] {
		return component, frames
	}
	visited[index] = true
	component = append(component, nodeID)
	frames = append(frames[:0], visitFrame{node: nodeID})
	for len(frames) > 0 {
		frame := &frames[len(frames)-1]
		ports := arena.Node(frame.node).Ports
		if int(frame.port) >= len(ports) {
			frames = frames[:len(frames)-1]
			continue
		}
		edges := arena.PortEdgeLists(ports[frame.port])[frame.list]
		if int(frame.edge) >= len(edges) {
			if frame.list == 0 {
				frame.list = 1
			} else {
				frame.port, frame.list = frame.port+1, 0
			}
			frame.edge = 0
			continue
		}
		edgeID := edges[frame.edge]
		frame.edge++
		opposite := arena.EdgeOtherNode(edgeID, frame.node)
		if visited[arena.Node(opposite).ID] {
			continue
		}
		// The recursive visit of opposite.
		index := arena.Node(opposite).ID
		if visited[index] {
			continue
		}
		visited[index] = true
		component = append(component, opposite)
		frames = append(frames, visitFrame{node: opposite})
	}
	return component, frames[:0]
}
