// Copyright (c) 2010, 2015 Kiel University and others.
// Copyright (c) 2022 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package p1cycles implements ELK Layered phase 1.
package p1cycles

import (
	"fmt"
	"math"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
)

type GreedyCycleBreaker struct {
	ModelOrderTies bool
}

func NewGreedyCycleBreaker() *GreedyCycleBreaker { return &GreedyCycleBreaker{} }

func NewGreedyModelOrderCycleBreaker() *GreedyCycleBreaker {
	return &GreedyCycleBreaker{ModelOrderTies: true}
}

func (*GreedyCycleBreaker) ProcessorConfiguration(
	_ *layered.LGraphArena,
	_ layered.LGraphID,
) layered.ProcessorConfiguration {
	return layered.GreedyCycleConfiguration()
}

// greedyScratch is the working storage of GreedyCycleBreaker.Process, kept
// in the arena (layered.ScratchGreedyCycles); every part is reset or
// overwritten before it is read.
type greedyScratch struct {
	degrees []int
	sources []layered.LNodeID
	sinks   []layered.LNodeID
	maximum []layered.LNodeID
	ports   []layered.LPortID
	edges   []layered.LEdgeID
}

func (breaker *GreedyCycleBreaker) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	if portless(arena, arena.Graph(graphID).LayerlessNodes) {
		// Nodes without ports have no edges: the general path numbers the
		// nodes, finds every node to be a sink, reverses nothing and reads
		// or writes no property (and draws nothing); only the numbering
		// remains.
		for index, nodeID := range arena.Graph(graphID).LayerlessNodes {
			arena.Node(nodeID).ID = index
		}
		return nil
	}
	scratch, _ := arena.Scratch(layered.ScratchGreedyCycles).(*greedyScratch)
	if scratch == nil {
		scratch = &greedyScratch{}
		arena.SetScratch(layered.ScratchGreedyCycles, scratch)
	}
	nodes := arena.Graph(graphID).LayerlessNodes
	nodeCount := len(nodes)
	// Three zeroed, disjoint arrays from one block (only indexed).
	degrees := scratch.degrees
	if cap(degrees) < 3*nodeCount {
		degrees = make([]int, 3*nodeCount)
	} else {
		degrees = degrees[:3*nodeCount]
		clear(degrees)
	}
	scratch.degrees = degrees
	indegree := degrees[:nodeCount:nodeCount]
	outdegree := degrees[nodeCount : 2*nodeCount : 2*nodeCount]
	mark := degrees[2*nodeCount:]
	// The source and sink queues are walked by head index (the order of
	// taking the first element each time).
	sources, sourceHead := scratch.sources[:0], 0
	sinks, sinkHead := scratch.sinks[:0], 0

	for index, nodeID := range nodes {
		arena.Node(nodeID).ID = index
		for _, portID := range arena.Node(nodeID).Ports {
			port := arena.Port(portID)
			for _, edgeID := range port.IncomingEdges {
				if arena.EdgeSourceNode(edgeID) == nodeID {
					continue
				}
				priority := basegraph.Get(&arena.Edge(edgeID).Properties, layered.PriorityDirection)
				if priority > 0 {
					indegree[index] += priority + 1
				} else {
					indegree[index]++
				}
			}
			for _, edgeID := range port.OutgoingEdges {
				if arena.EdgeTargetNode(edgeID) == nodeID {
					continue
				}
				priority := basegraph.Get(&arena.Edge(edgeID).Properties, layered.PriorityDirection)
				if priority > 0 {
					outdegree[index] += priority + 1
				} else {
					outdegree[index]++
				}
			}
		}
		if outdegree[index] == 0 {
			sinks = append(sinks, nodeID)
		} else if indegree[index] == 0 {
			sources = append(sources, nodeID)
		}
	}

	updateNeighbors := func(nodeID layered.LNodeID) {
		for _, portID := range arena.Node(nodeID).Ports {
			// The loop changes no edge list, so the lists are walked in place.
			for _, edges := range arena.PortEdgeLists(portID) {
				for _, edgeID := range edges {
					connectedPort := arena.EdgeOtherPort(edgeID, portID)
					endpoint := *arena.Port(connectedPort).Node
					if endpoint == nodeID {
						continue
					}
					priority := basegraph.Get(&arena.Edge(edgeID).Properties, layered.PriorityDirection)
					if priority < 0 {
						priority = 0
					}
					index := arena.Node(endpoint).ID
					if mark[index] != 0 {
						continue
					}
					if arena.Edge(edgeID).Target != nil && *arena.Edge(edgeID).Target == connectedPort {
						indegree[index] -= priority + 1
						if indegree[index] <= 0 && outdegree[index] > 0 {
							sources = append(sources, endpoint)
						}
					} else {
						outdegree[index] -= priority + 1
						if outdegree[index] <= 0 && indegree[index] > 0 {
							sinks = append(sinks, endpoint)
						}
					}
				}
			}
		}
	}

	unprocessed := nodeCount
	nextRight, nextLeft := -1, 1
	for unprocessed > 0 {
		for sinkHead < len(sinks) {
			nodeID := sinks[sinkHead]
			sinkHead++
			index := arena.Node(nodeID).ID
			if mark[index] != 0 {
				continue
			}
			mark[index] = nextRight
			nextRight--
			updateNeighbors(nodeID)
			unprocessed--
		}
		for sourceHead < len(sources) {
			nodeID := sources[sourceHead]
			sourceHead++
			index := arena.Node(nodeID).ID
			if mark[index] != 0 {
				continue
			}
			mark[index] = nextLeft
			nextLeft++
			updateNeighbors(nodeID)
			unprocessed--
		}
		if unprocessed == 0 {
			break
		}

		maximumOutflow := math.MinInt
		maximumNodes := scratch.maximum[:0]
		for _, nodeID := range nodes {
			index := arena.Node(nodeID).ID
			if mark[index] != 0 {
				continue
			}
			outflow := outdegree[index] - indegree[index]
			if outflow >= maximumOutflow {
				if outflow > maximumOutflow {
					maximumNodes = maximumNodes[:0]
					maximumOutflow = outflow
				}
				maximumNodes = append(maximumNodes, nodeID)
			}
		}
		scratch.maximum = maximumNodes
		if len(maximumNodes) == 0 {
			scratch.sources, scratch.sinks = sources[:0], sinks[:0]
			return fmt.Errorf("layered p1: no unprocessed maximum-outflow node")
		}
		chosen := breaker.chooseMaximumNode(arena, graphID, maximumNodes)
		mark[arena.Node(chosen).ID] = nextLeft
		nextLeft++
		updateNeighbors(chosen)
		unprocessed--
	}

	shiftBase := nodeCount + 1
	for index := range mark {
		if mark[index] < 0 {
			mark[index] += shiftBase
		}
	}
	// Reversing edges changes the lists, so each loop walks a snapshot; the
	// snapshots live in two buffers (the loops are strictly nested and each
	// buffer is refilled only when its loop starts).
	portsSnapshot, edgesSnapshot := scratch.ports[:0], scratch.edges[:0]
	for _, nodeID := range nodes {
		portsSnapshot = append(portsSnapshot[:0], arena.Node(nodeID).Ports...)
		for _, portID := range portsSnapshot {
			edgesSnapshot = append(edgesSnapshot[:0], arena.Port(portID).OutgoingEdges...)
			for _, edgeID := range edgesSnapshot {
				target := arena.EdgeTargetNode(edgeID)
				if mark[arena.Node(nodeID).ID] > mark[arena.Node(target).ID] {
					arena.ReverseEdge(graphID, edgeID, true)
					basegraph.Set(&arena.Graph(graphID).Properties, layered.Cyclic, true)
				}
			}
		}
	}
	scratch.sources, scratch.sinks = sources[:0], sinks[:0]
	scratch.ports, scratch.edges = portsSnapshot[:0], edgesSnapshot[:0]
	return nil
}

func (breaker *GreedyCycleBreaker) chooseMaximumNode(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	nodes []layered.LNodeID,
) layered.LNodeID {
	if breaker.ModelOrderTies {
		minimumOrder := math.MaxInt
		var chosen layered.LNodeID
		found := false
		for _, nodeID := range nodes {
			order, present := basegraph.TryGet(&arena.Node(nodeID).Properties, layered.ModelOrder)
			if present && order < minimumOrder {
				minimumOrder, chosen, found = order, nodeID, true
			}
		}
		if found {
			return chosen
		}
	}
	random, found := basegraph.TryGet(&arena.Graph(graphID).Properties, layered.Random)
	if !found || random == nil {
		random = javacompatRandom(arena, graphID)
	}
	return nodes[random.NextIntN(int32(len(nodes)))]
}

func javacompatRandom(arena *layered.LGraphArena, graphID layered.LGraphID) *javacompat.Random {
	layered.ConfigureGraphProperties(arena, graphID)
	return basegraph.Get(&arena.Graph(graphID).Properties, layered.Random)
}

// portless reports whether none of nodes has a port.
func portless(arena *layered.LGraphArena, nodes []layered.LNodeID) bool {
	for _, nodeID := range nodes {
		if len(arena.Node(nodeID).Ports) != 0 {
			return false
		}
	}
	return true
}
