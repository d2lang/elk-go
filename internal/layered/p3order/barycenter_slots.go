// Copyright (c) 2026 The elk-go Authors.
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

// The layer minimization below is MinimizeLayer for a layer whose nodes all
// belong to the resolver's holder, in a minimization with a port table and
// without model order. Each node's state is the resolver's
// stateBacking[slot] (see sweepIndex), so the states are looked up once per
// layer instead of once per access; everything else is MinimizeLayer's
// sequence of reads, writes and random draws.

// slotSortLimit bounds the layers sorted by insertion on the slots; longer
// layers use the original merge sort.
const slotSortLimit = 32

// layerSlots returns the slots of nodes in the heuristic's scratch, or false
// when the minimization cannot use them (no table, model order, or a node
// that is not the resolver's holder's).
func (heuristic *BarycenterHeuristic) layerSlots(nodes []layered.LNodeID) ([]int32, bool) {
	resolver := heuristic.Resolver
	if heuristic.table == nil || heuristic.ModelOrder != nil || resolver == nil ||
		resolver.index == nil || resolver.owner == 0 {
		return nil, false
	}
	refs := resolver.index.nodes
	buffers := heuristic.buffers()
	slots := buffers.slots[:0]
	for _, nodeID := range nodes {
		if int(nodeID) >= len(refs) {
			return nil, false
		}
		ref := refs[nodeID]
		if ref.owner != resolver.owner {
			return nil, false
		}
		slots = append(slots, ref.slot)
	}
	buffers.slots = slots
	return slots, true
}

// minimizeLayerFromSlots is MinimizeLayer(layer, preordered, randomize,
// forward) with slots[i] the slot of (*layer)[i] (see layerSlots).
func (heuristic *BarycenterHeuristic) minimizeLayerFromSlots(
	layer *[]layered.LNodeID,
	slots []int32,
	preordered, randomize, forward bool,
) {
	nodes := *layer
	states := heuristic.Resolver.stateBacking
	if randomize {
		for _, slot := range slots {
			value := heuristic.Random.NextDouble()
			state := &states[slot]
			state.setBarycenter(value)
			state.SummedWeight = value
			state.Degree = 1
		}
	} else {
		for _, slot := range slots {
			states[slot].Visited = false
		}
		for index, nodeID := range nodes {
			heuristic.calculateBarycenterOf(nodeID, &states[slots[index]], forward)
		}
		heuristic.fillUnknownBarycentersFromSlots(states, slots, preordered)
	}
	if len(nodes) <= 1 {
		return
	}
	if len(nodes) > slotSortLimit {
		buffers := heuristic.buffers()
		buffers.sortBuffer = javacompat.GWTStableSortBuffer(nodes, heuristic.compareBarycenters, buffers.sortBuffer)
		heuristic.Resolver.ProcessConstraints(heuristic.Arena, layer)
		return
	}
	sortBySlotBarycenters(nodes, slots, states)
	heuristic.Resolver.processConstraintsFromSlots(heuristic.Arena, layer, slots)
}

// calculateBarycenterOf is calculateBarycenter(nodeID) with state, nodeID's
// state, given (the resolver's state of nodeID).
func (heuristic *BarycenterHeuristic) calculateBarycenterOf(
	nodeID layered.LNodeID,
	state *BarycenterState,
	forward bool,
) {
	if state.Visited {
		return
	}
	state.Visited = true
	state.Degree = 0
	state.SummedWeight = 0
	state.clearBarycenter()
	arena := heuristic.Arena
	table := heuristic.table
	node := arena.Node(nodeID)
	// addFixedPort for every edge end: the fixed port's node is
	// end.otherNode, its layer end.otherLayer, its ID end.otherID.
	nodeLayer := layerKey(node.Layer)
	ranks := heuristic.Distributor.PortRanks
	for _, freePort := range node.Ports {
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
				fixedState := heuristic.Resolver.state(arena, end.otherNode)
				heuristic.calculateBarycenterOf(end.otherNode, fixedState, forward)
				state.Degree += fixedState.Degree
				state.SummedWeight += fixedState.SummedWeight
				continue
			}
			state.SummedWeight += float64(ranks[end.otherID])
			state.Degree++
		}
	}
	for _, associate := range state.barycenterAssociates(arena, nodeID) {
		if !sameLayer(node.Layer, arena.Node(associate).Layer) {
			continue
		}
		associateState := heuristic.Resolver.state(arena, associate)
		heuristic.calculateBarycenterOf(associate, associateState, forward)
		state.Degree += associateState.Degree
		state.SummedWeight += associateState.SummedWeight
	}
	if state.Degree > 0 {
		perturbation := heuristic.Random.NextFloat()*randomBarycenterAmount - randomBarycenterAmount/2
		state.SummedWeight += float64(perturbation)
		state.setBarycenter(state.SummedWeight / float64(state.Degree))
	}
}

// fillUnknownBarycentersFromSlots is fillUnknownBarycenters on the layer
// whose states are states[slots[i]].
func (heuristic *BarycenterHeuristic) fillUnknownBarycentersFromSlots(
	states []BarycenterState,
	slots []int32,
	preordered bool,
) {
	if preordered {
		lastValue := -1.0
		for index, slot := range slots {
			state := &states[slot]
			if !state.HasBarycenter {
				nextValue := lastValue + 1
				for _, nextSlot := range slots[index+1:] {
					if next := &states[nextSlot]; next.HasBarycenter {
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
	for _, slot := range slots {
		if state := &states[slot]; state.HasBarycenter && state.Barycenter > maximum {
			maximum = state.Barycenter
		}
	}
	maximum += 2
	for _, slot := range slots {
		state := &states[slot]
		if !state.HasBarycenter {
			value := float64(heuristic.Random.NextFloat())*maximum - 1
			state.setBarycenter(value)
			state.SummedWeight = value
			state.Degree = 1
		}
	}
}

// sortBySlotBarycenters stably sorts nodes, and slots along with them, by
// compareBarycenters of their states. compareBarycenters is a strict weak
// order (states with a barycenter first, by value; barycenters are never
// NaN), so every stable sort, in particular the merge sort MinimizeLayer
// uses, gives this result.
func sortBySlotBarycenters(nodes []layered.LNodeID, slots []int32, states []BarycenterState) {
	for index := 1; index < len(nodes); index++ {
		nodeID, slot := nodes[index], slots[index]
		state := &states[slot]
		position := index
		for position > 0 && barycenterStateBefore(state, &states[slots[position-1]]) {
			nodes[position], slots[position] = nodes[position-1], slots[position-1]
			position--
		}
		nodes[position], slots[position] = nodeID, slot
	}
}

// barycenterStateBefore is compareBarycenters(first, second) < 0.
func barycenterStateBefore(first, second *BarycenterState) bool {
	if first.HasBarycenter && second.HasBarycenter {
		return first.Barycenter < second.Barycenter
	}
	return first.HasBarycenter && !second.HasBarycenter
}

// processConstraintsFromSlots is ProcessConstraints(layer) with slots[i] the
// slot of (*layer)[i]: a stage that can add no constraint is recognized
// reading the states by slot (the same successor-constraint reads in the
// same order as stageAddsNoConstraint); a stage that can is run as before,
// and so is everything after it.
func (resolver *ForsterConstraintResolver) processConstraintsFromSlots(
	arena *layered.LGraphArena,
	layer *[]layered.LNodeID,
	slots []int32,
) {
	if resolver.knownSuccessors == len(resolver.stateBacking) && resolver.nonEmptySuccessors == 0 &&
		len(resolver.layoutUnits) == 0 {
		// Every node's successor constraints have been read (so no read is
		// left that materializes a default) and none has any, and there are
		// no layout units: every stage adds no constraint.
		return
	}
	if resolver.constraintsBetweenNonDummies &&
		!resolver.stageAddsNoConstraintFromSlots(arena, *layer, slots, true) {
		resolver.processConstraintStage(arena, layer, true)
		resolver.processConstraintStage(arena, layer, false)
		return
	}
	if !resolver.stageAddsNoConstraintFromSlots(arena, *layer, slots, false) {
		resolver.processConstraintStage(arena, layer, false)
	}
}

// stageAddsNoConstraintFromSlots is stageAddsNoConstraint with the states
// read by slot.
func (resolver *ForsterConstraintResolver) stageAddsNoConstraintFromSlots(
	arena *layered.LGraphArena,
	nodes []layered.LNodeID,
	slots []int32,
	onlyNormal bool,
) bool {
	none := onlyNormal || len(resolver.layoutUnits) == 0
	states := resolver.stateBacking
	for index, nodeID := range nodes {
		if onlyNormal && arena.Node(nodeID).Type != layered.NormalNode {
			continue
		}
		if len(resolver.readSuccessors(&states[slots[index]], arena, nodeID)) > 0 {
			none = false
		}
	}
	return none
}
