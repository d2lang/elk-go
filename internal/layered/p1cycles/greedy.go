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

func (breaker *GreedyCycleBreaker) Process(arena *layered.LGraphArena, graphID layered.LGraphID) error {
	nodes := arena.Graph(graphID).LayerlessNodes
	nodeCount := len(nodes)
	indegree := make([]int, nodeCount)
	outdegree := make([]int, nodeCount)
	mark := make([]int, nodeCount)
	sources := make([]layered.LNodeID, 0)
	sinks := make([]layered.LNodeID, 0)

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
			for _, edgeID := range arena.PortConnectedEdges(portID) {
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

	unprocessed := nodeCount
	nextRight, nextLeft := -1, 1
	for unprocessed > 0 {
		for len(sinks) > 0 {
			nodeID := sinks[0]
			sinks = sinks[1:]
			index := arena.Node(nodeID).ID
			if mark[index] != 0 {
				continue
			}
			mark[index] = nextRight
			nextRight--
			updateNeighbors(nodeID)
			unprocessed--
		}
		for len(sources) > 0 {
			nodeID := sources[0]
			sources = sources[1:]
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
		maximumNodes := make([]layered.LNodeID, 0)
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
		if len(maximumNodes) == 0 {
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
	for _, nodeID := range nodes {
		for _, portID := range append([]layered.LPortID(nil), arena.Node(nodeID).Ports...) {
			for _, edgeID := range append([]layered.LEdgeID(nil), arena.Port(portID).OutgoingEdges...) {
				target := arena.EdgeTargetNode(edgeID)
				if mark[arena.Node(nodeID).ID] > mark[arena.Node(target).ID] {
					arena.ReverseEdge(graphID, edgeID, true)
					basegraph.Set(&arena.Graph(graphID).Properties, layered.Cyclic, true)
				}
			}
		}
	}
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
