// Copyright (c) 2010, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
)

const randomBarycenterAmount = float32(0.07)

type BarycenterHeuristic struct {
	Arena       *layered.LGraphArena
	Resolver    *ForsterConstraintResolver
	Random      *javacompat.Random
	Distributor *BarycenterPortDistributor
	ModelOrder  *ModelOrderBarycenterHeuristic

	// own holds the heuristic's buffers unless scratch points to shared ones.
	own     heuristicScratch
	scratch *heuristicScratch

	table *portTable // ports and edges, if the minimizer built a table
}

// heuristicScratch is a barycenter heuristic's working storage: layerBuffer
// holds the copy of the layer being minimized; sortBuffer is the merge
// sort's auxiliary array; slots holds the layer's slots (see layerSlots).
type heuristicScratch struct {
	layerBuffer []layered.LNodeID
	sortBuffer  []layered.LNodeID
	slots       []int32
}

func (heuristic *BarycenterHeuristic) buffers() *heuristicScratch {
	if heuristic.scratch != nil {
		return heuristic.scratch
	}
	return &heuristic.own
}

func NewBarycenterHeuristic(
	arena *layered.LGraphArena,
	resolver *ForsterConstraintResolver,
	random *javacompat.Random,
	distributor *BarycenterPortDistributor,
) *BarycenterHeuristic {
	return &BarycenterHeuristic{Arena: arena, Resolver: resolver, Random: random, Distributor: distributor}
}

func (heuristic *BarycenterHeuristic) MinimizeLayer(
	layer *[]layered.LNodeID,
	preordered, randomize, forward bool,
) {
	if randomize {
		heuristic.randomizeBarycenters(*layer)
	} else {
		heuristic.calculateBarycenters(*layer, forward)
		heuristic.fillUnknownBarycenters(*layer, preordered)
	}
	if len(*layer) <= 1 {
		return
	}
	if heuristic.ModelOrder != nil {
		heuristic.ModelOrder.sort(heuristic, *layer)
		if heuristic.ModelOrder.ForceNodeModelOrder {
			return
		}
	} else {
		buffers := heuristic.buffers()
		buffers.sortBuffer = javacompat.GWTStableSortBuffer(*layer, heuristic.compareBarycenters, buffers.sortBuffer)
	}
	heuristic.Resolver.ProcessConstraints(heuristic.Arena, layer)
}

func (heuristic *BarycenterHeuristic) MinimizeCrossings(
	order [][]layered.LNodeID,
	freeLayerIndex int,
	forwardSweep, firstSweep bool,
) bool {
	heuristic.Distributor.ranked, heuristic.Distributor.rankedSet = nil, false
	if !isFirstSweepLayer(len(order), freeLayerIndex, forwardSweep) {
		fixedLayerIndex := freeLayerIndex + 1
		if forwardSweep {
			fixedLayerIndex = freeLayerIndex - 1
		}
		heuristic.Distributor.calculatePortRanksOf(
			heuristic.Arena, order[fixedLayerIndex], portTypeForSweep(forwardSweep),
		)
		heuristic.Distributor.noteHeuristicRanks(order[fixedLayerIndex], portTypeForSweep(forwardSweep))
	}
	preordered := !firstSweep
	if len(order[freeLayerIndex]) > 0 && heuristic.Arena.Node(order[freeLayerIndex][0]).Type == layered.ExternalPortNode {
		preordered = true
	}
	buffers := heuristic.buffers()
	nodes := append(buffers.layerBuffer[:0], order[freeLayerIndex]...)
	if slots, ok := heuristic.layerSlots(nodes); ok {
		heuristic.minimizeLayerFromSlots(&nodes, slots, preordered, false, forwardSweep)
	} else {
		heuristic.MinimizeLayer(&nodes, preordered, false, forwardSweep)
	}
	copy(order[freeLayerIndex], nodes)
	buffers.layerBuffer = nodes
	return false
}

func (heuristic *BarycenterHeuristic) SetFirstLayerOrder(
	order [][]layered.LNodeID,
	forward bool,
) bool {
	index := 0
	if !forward && len(order) > 0 {
		index = len(order) - 1
	}
	buffers := heuristic.buffers()
	nodes := append(buffers.layerBuffer[:0], order[index]...)
	if slots, ok := heuristic.layerSlots(nodes); ok {
		heuristic.minimizeLayerFromSlots(&nodes, slots, false, true, forward)
	} else {
		heuristic.MinimizeLayer(&nodes, false, true, forward)
	}
	copy(order[index], nodes)
	buffers.layerBuffer = nodes
	return false
}

func (heuristic *BarycenterHeuristic) randomizeBarycenters(nodes []layered.LNodeID) {
	for _, nodeID := range nodes {
		value := heuristic.Random.NextDouble()
		state := heuristic.Resolver.state(heuristic.Arena, nodeID)
		state.setBarycenter(value)
		state.SummedWeight = value
		state.Degree = 1
	}
}

func (heuristic *BarycenterHeuristic) fillUnknownBarycenters(
	nodes []layered.LNodeID,
	preordered bool,
) {
	if preordered {
		lastValue := -1.0
		for index, nodeID := range nodes {
			state := heuristic.Resolver.state(heuristic.Arena, nodeID)
			if !state.HasBarycenter {
				nextValue := lastValue + 1
				for _, nextNode := range nodes[index+1:] {
					if next := heuristic.Resolver.state(heuristic.Arena, nextNode); next.HasBarycenter {
						nextValue = next.Barycenter
						break
					}
				}
				value := (lastValue + nextValue) / 2
				state.setBarycenter(value)
				state.SummedWeight = value
				state.Degree = 1
			}
			lastValue = state.Barycenter
		}
		return
	}
	maximum := 0.0
	for _, nodeID := range nodes {
		if state := heuristic.Resolver.state(heuristic.Arena, nodeID); state.HasBarycenter && state.Barycenter > maximum {
			maximum = state.Barycenter
		}
	}
	maximum += 2
	for _, nodeID := range nodes {
		state := heuristic.Resolver.state(heuristic.Arena, nodeID)
		if !state.HasBarycenter {
			value := float64(heuristic.Random.NextFloat())*maximum - 1
			state.setBarycenter(value)
			state.SummedWeight = value
			state.Degree = 1
		}
	}
}

func (heuristic *BarycenterHeuristic) calculateBarycenters(nodes []layered.LNodeID, forward bool) {
	for _, nodeID := range nodes {
		heuristic.Resolver.state(heuristic.Arena, nodeID).Visited = false
	}
	for _, nodeID := range nodes {
		heuristic.calculateBarycenter(nodeID, forward)
	}
}

func (heuristic *BarycenterHeuristic) calculateBarycenter(nodeID layered.LNodeID, forward bool) {
	state := heuristic.Resolver.state(heuristic.Arena, nodeID)
	if state.Visited {
		return
	}
	state.Visited = true
	state.Degree = 0
	state.SummedWeight = 0
	state.clearBarycenter()
	if table := heuristic.table; table != nil {
		// addFixedPort for every edge end: the fixed port's node is
		// end.otherNode, its layer end.otherLayer, its ID end.otherID.
		nodeLayer := layerKey(heuristic.Arena.Node(nodeID).Layer)
		ranks := heuristic.Distributor.PortRanks
		for _, freePort := range heuristic.Arena.Node(nodeID).Ports {
			entry := &table.ports[freePort]
			ends := table.outgoing(entry)
			if forward {
				ends = table.incoming(entry)
			}
			for index := range ends {
				end := &ends[index]
				if end.otherLayer == nodeLayer {
					if end.otherNode == nodeID {
						continue
					}
					heuristic.calculateBarycenter(end.otherNode, forward)
					fixedState := heuristic.Resolver.state(heuristic.Arena, end.otherNode)
					state.Degree += fixedState.Degree
					state.SummedWeight += fixedState.SummedWeight
					continue
				}
				state.SummedWeight += float64(ranks[end.otherID])
				state.Degree++
			}
		}
	} else {
		// addFixedPort for every edge of nodeID's ports towards the fixed
		// layer, its recursion made here: a level of it is this one frame
		// (as two frames at most were in ELK's port).
		for _, freePort := range heuristic.Arena.Node(nodeID).Ports {
			edges := heuristic.Arena.Port(freePort).OutgoingEdges
			if forward {
				edges = heuristic.Arena.Port(freePort).IncomingEdges
			}
			for _, edgeID := range edges {
				var fixedPort layered.LPortID
				if forward {
					fixedPort = *heuristic.Arena.Edge(edgeID).Source
				} else {
					fixedPort = *heuristic.Arena.Edge(edgeID).Target
				}
				fixedNode := *heuristic.Arena.Port(fixedPort).Node
				own := heuristic.Resolver.state(heuristic.Arena, nodeID)
				if !sameLayer(heuristic.Arena.Node(fixedNode).Layer, heuristic.Arena.Node(nodeID).Layer) {
					own.SummedWeight += float64(heuristic.Distributor.PortRanks[heuristic.Arena.Port(fixedPort).ID])
					own.Degree++
					continue
				}
				if fixedNode == nodeID {
					continue
				}
				heuristic.calculateBarycenter(fixedNode, forward)
				fixedState := heuristic.Resolver.state(heuristic.Arena, fixedNode)
				own.Degree += fixedState.Degree
				own.SummedWeight += fixedState.SummedWeight
			}
		}
	}
	for _, associate := range state.barycenterAssociates(heuristic.Arena, nodeID) {
		if !sameLayer(heuristic.Arena.Node(nodeID).Layer, heuristic.Arena.Node(associate).Layer) {
			continue
		}
		heuristic.calculateBarycenter(associate, forward)
		associateState := heuristic.Resolver.state(heuristic.Arena, associate)
		state = heuristic.Resolver.state(heuristic.Arena, nodeID)
		state.Degree += associateState.Degree
		state.SummedWeight += associateState.SummedWeight
	}
	state = heuristic.Resolver.state(heuristic.Arena, nodeID)
	if state.Degree > 0 {
		perturbation := heuristic.Random.NextFloat()*randomBarycenterAmount - randomBarycenterAmount/2
		state.SummedWeight += float64(perturbation)
		state.setBarycenter(state.SummedWeight / float64(state.Degree))
	}
}

func (heuristic *BarycenterHeuristic) compareBarycenters(first, second layered.LNodeID) int {
	firstState := heuristic.Resolver.state(heuristic.Arena, first)
	secondState := heuristic.Resolver.state(heuristic.Arena, second)
	if firstState.HasBarycenter && secondState.HasBarycenter {
		return compareFloat(firstState.Barycenter, secondState.Barycenter)
	}
	if firstState.HasBarycenter {
		return -1
	}
	if secondState.HasBarycenter {
		return 1
	}
	return 0
}
