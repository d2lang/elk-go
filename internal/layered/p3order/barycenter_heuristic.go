// Copyright (c) 2010, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
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
		javacompat.GWTStableSort(*layer, heuristic.compareBarycenters)
	}
	heuristic.Resolver.ProcessConstraints(heuristic.Arena, layer)
}

func (heuristic *BarycenterHeuristic) MinimizeCrossings(
	order [][]layered.LNodeID,
	freeLayerIndex int,
	forwardSweep, firstSweep bool,
) bool {
	if !isFirstSweepLayer(len(order), freeLayerIndex, forwardSweep) {
		fixedLayerIndex := freeLayerIndex + 1
		if forwardSweep {
			fixedLayerIndex = freeLayerIndex - 1
		}
		heuristic.Distributor.CalculatePortRanks(
			heuristic.Arena, order[fixedLayerIndex], portTypeForSweep(forwardSweep),
		)
	}
	preordered := !firstSweep
	if len(order[freeLayerIndex]) > 0 && heuristic.Arena.Node(order[freeLayerIndex][0]).Type == layered.ExternalPortNode {
		preordered = true
	}
	nodes := append([]layered.LNodeID(nil), order[freeLayerIndex]...)
	heuristic.MinimizeLayer(&nodes, preordered, false, forwardSweep)
	copy(order[freeLayerIndex], nodes)
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
	nodes := append([]layered.LNodeID(nil), order[index]...)
	heuristic.MinimizeLayer(&nodes, false, true, forward)
	copy(order[index], nodes)
	return false
}

func (heuristic *BarycenterHeuristic) randomizeBarycenters(nodes []layered.LNodeID) {
	for _, nodeID := range nodes {
		value := heuristic.Random.NextDouble()
		state := heuristic.Resolver.state(heuristic.Arena, nodeID)
		state.Barycenter = floatPtr(value)
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
			if state.Barycenter == nil {
				nextValue := lastValue + 1
				for _, nextNode := range nodes[index+1:] {
					if barycenter := heuristic.Resolver.state(heuristic.Arena, nextNode).Barycenter; barycenter != nil {
						nextValue = *barycenter
						break
					}
				}
				value := (lastValue + nextValue) / 2
				state.Barycenter = floatPtr(value)
				state.SummedWeight = value
				state.Degree = 1
			}
			lastValue = *state.Barycenter
		}
		return
	}
	maximum := 0.0
	for _, nodeID := range nodes {
		if barycenter := heuristic.Resolver.state(heuristic.Arena, nodeID).Barycenter; barycenter != nil && *barycenter > maximum {
			maximum = *barycenter
		}
	}
	maximum += 2
	for _, nodeID := range nodes {
		state := heuristic.Resolver.state(heuristic.Arena, nodeID)
		if state.Barycenter == nil {
			value := float64(heuristic.Random.NextFloat())*maximum - 1
			state.Barycenter = floatPtr(value)
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
	state.Barycenter = nil
	for _, freePort := range heuristic.Arena.Node(nodeID).Ports {
		if forward {
			for _, edgeID := range heuristic.Arena.Port(freePort).IncomingEdges {
				heuristic.addFixedPort(nodeID, *heuristic.Arena.Edge(edgeID).Source, forward)
			}
		} else {
			for _, edgeID := range heuristic.Arena.Port(freePort).OutgoingEdges {
				heuristic.addFixedPort(nodeID, *heuristic.Arena.Edge(edgeID).Target, forward)
			}
		}
	}
	for _, associate := range basegraph.Get(&heuristic.Arena.Node(nodeID).Properties, layered.BarycenterAssociates) {
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
		state.Barycenter = floatPtr(state.SummedWeight / float64(state.Degree))
	}
}

func (heuristic *BarycenterHeuristic) addFixedPort(
	nodeID layered.LNodeID,
	fixedPort layered.LPortID,
	forward bool,
) {
	fixedNode := *heuristic.Arena.Port(fixedPort).Node
	state := heuristic.Resolver.state(heuristic.Arena, nodeID)
	if sameLayer(heuristic.Arena.Node(fixedNode).Layer, heuristic.Arena.Node(nodeID).Layer) {
		if fixedNode == nodeID {
			return
		}
		heuristic.calculateBarycenter(fixedNode, forward)
		fixedState := heuristic.Resolver.state(heuristic.Arena, fixedNode)
		state.Degree += fixedState.Degree
		state.SummedWeight += fixedState.SummedWeight
		return
	}
	state.SummedWeight += float64(heuristic.Distributor.PortRanks[heuristic.Arena.Port(fixedPort).ID])
	state.Degree++
}

func (heuristic *BarycenterHeuristic) compareBarycenters(first, second layered.LNodeID) int {
	firstBarycenter := heuristic.Resolver.state(heuristic.Arena, first).Barycenter
	secondBarycenter := heuristic.Resolver.state(heuristic.Arena, second).Barycenter
	if firstBarycenter != nil && secondBarycenter != nil {
		return compareFloat(*firstBarycenter, *secondBarycenter)
	}
	if firstBarycenter != nil {
		return -1
	}
	if secondBarycenter != nil {
		return 1
	}
	return 0
}
