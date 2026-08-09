// Copyright (c) 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import "github.com/d2lang/elk-go/internal/layered"

// crossingMinimizationHeuristic is the source ICrossingMinimizationHeuristic
// contract shared by barycenter and greedy switch.
type crossingMinimizationHeuristic interface {
	MinimizeCrossings(order [][]layered.LNodeID, freeLayerIndex int, forwardSweep, firstSweep bool) bool
	SetFirstLayerOrder(order [][]layered.LNodeID, forward bool) bool
}

type sweepPortDistributor interface {
	DistributePortsWhileSweeping(
		arena *layered.LGraphArena,
		order [][]layered.LNodeID,
		currentIndex int,
		forward bool,
	) bool
}

// GreedySwitchHeuristic switches adjacent nodes whenever doing so strictly
// reduces the selected one- or two-sided crossing count.
type GreedySwitchHeuristic struct {
	Arena         *layered.LGraphArena
	Type          CrossMinType
	GraphData     *GraphInfoHolder
	PortPositions []int
	nPorts        int
}

func NewGreedySwitchHeuristic(
	arena *layered.LGraphArena,
	crossMinType CrossMinType,
	holder *GraphInfoHolder,
) *GreedySwitchHeuristic {
	return &GreedySwitchHeuristic{Arena: arena, Type: crossMinType, GraphData: holder}
}

func (heuristic *GreedySwitchHeuristic) MinimizeCrossings(
	order [][]layered.LNodeID,
	freeLayerIndex int,
	forwardSweep, _ bool,
) bool {
	decider := heuristic.setUp(order, freeLayerIndex, forwardSweep)
	improved := false
	for {
		changed := heuristic.sweepDownwardInLayer(order, decider, freeLayerIndex)
		improved = improved || changed
		if !changed {
			return improved
		}
	}
}

func (heuristic *GreedySwitchHeuristic) SetFirstLayerOrder(
	order [][]layered.LNodeID,
	forward bool,
) bool {
	index := 0
	if !forward {
		index = len(order) - 1
	}
	decider := heuristic.setUp(order, index, forward)
	return heuristic.sweepDownwardInLayer(order, decider, index)
}

func (heuristic *GreedySwitchHeuristic) setUp(
	order [][]layered.LNodeID,
	freeLayerIndex int,
	forward bool,
) *SwitchDecider {
	side := CrossingsEast
	if forward {
		side = CrossingsWest
	}
	filler := NewCrossingMatrixFiller(heuristic.Arena, heuristic.Type, order, freeLayerIndex, side)
	return NewSwitchDecider(
		heuristic.Arena,
		freeLayerIndex,
		order,
		filler,
		heuristic.PortPositions,
		heuristic.GraphData,
		heuristic.Type == CrossMinOneSidedGreedySwitch,
	)
}

func (heuristic *GreedySwitchHeuristic) sweepDownwardInLayer(
	order [][]layered.LNodeID,
	decider *SwitchDecider,
	layerIndex int,
) bool {
	changed := false
	for upper := 0; upper+1 < len(order[layerIndex]); upper++ {
		lower := upper + 1
		if !decider.DoesSwitchReduceCrossings(heuristic.Arena, upper, lower) {
			continue
		}
		decider.NotifyOfSwitch(heuristic.Arena, order[layerIndex][upper], order[layerIndex][lower])
		order[layerIndex][upper], order[layerIndex][lower] = order[layerIndex][lower], order[layerIndex][upper]
		changed = true
	}
	return changed
}

func (heuristic *GreedySwitchHeuristic) InitAtLayerLevel(
	arena *layered.LGraphArena,
	layerIndex int,
	order [][]layered.LNodeID,
) {
	layerID := *arena.Node(order[layerIndex][0]).Layer
	arena.Layer(layerID).ID = layerIndex
}

func (heuristic *GreedySwitchHeuristic) InitAtPortLevel() { heuristic.nPorts++ }

func (heuristic *GreedySwitchHeuristic) InitAfterTraversal() {
	heuristic.PortPositions = make([]int, heuristic.nPorts)
}
