// Copyright (c) 2014, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// SwitchDecider compares the crossing count of two adjacent nodes in their
// current and switched orders.
type SwitchDecider struct {
	freeLayer                        []layered.LNodeID
	leftInLayerCounter               *CrossingsCounter
	rightInLayerCounter              *CrossingsCounter
	northSouthCounter                *NorthSouthEdgeNeighbouringNodeCrossingsCounter
	crossingMatrixFiller             *CrossingMatrixFiller
	parentCrossCounter               *CrossingsCounter
	countCrossingsCausedByPortSwitch bool
}

func NewSwitchDecider(
	arena *layered.LGraphArena,
	freeLayerIndex int,
	order [][]layered.LNodeID,
	filler *CrossingMatrixFiller,
	portPositions []int,
	holder *GraphInfoHolder,
	oneSided bool,
) *SwitchDecider {
	if freeLayerIndex >= len(order) {
		panic("p3order: greedy SwitchDecider free layer is not in graph")
	}
	freeLayer := order[freeLayerIndex]
	decider := &SwitchDecider{
		freeLayer:            freeLayer,
		leftInLayerCounter:   NewCrossingsCounter(portPositions),
		rightInLayerCounter:  NewCrossingsCounter(portPositions),
		crossingMatrixFiller: filler,
	}
	decider.leftInLayerCounter.InitPortPositionsForInLayerCrossings(arena, freeLayer, layered.WestSide)
	decider.rightInLayerCounter.InitPortPositionsForInLayerCrossings(arena, freeLayer, layered.EastSide)
	decider.northSouthCounter = NewNorthSouthEdgeNeighbouringNodeCrossingsCounter(arena, freeLayer)
	decider.countCrossingsCausedByPortSwitch = !oneSided && holder.HasParent && !holder.DontSweepInto() &&
		arena.Node(freeLayer[0]).Type == layered.ExternalPortNode
	if decider.countCrossingsCausedByPortSwitch {
		decider.initParentCrossingsCounter(arena, freeLayerIndex, len(order), holder)
	}
	return decider
}

func (decider *SwitchDecider) initParentCrossingsCounter(
	arena *layered.LGraphArena,
	freeLayerIndex, length int,
	holder *GraphInfoHolder,
) {
	parentData := holder.ParentGraphData
	parentOrder := parentData.CurrentNodeOrder
	decider.parentCrossCounter = NewCrossingsCounter(parentData.PortPositions)
	parentLayerPosition := arena.Layer(*arena.Node(*holder.Parent).Layer).ID
	var leftLayer, rightLayer []layered.LNodeID
	if parentLayerPosition > 0 {
		leftLayer = parentOrder[parentLayerPosition-1]
	}
	middleLayer := parentOrder[parentLayerPosition]
	if parentLayerPosition < len(parentOrder)-1 {
		rightLayer = parentOrder[parentLayerPosition+1]
	}
	if freeLayerIndex == length-1 {
		decider.parentCrossCounter.InitForCountingBetween(arena, middleLayer, rightLayer)
	} else {
		decider.parentCrossCounter.InitForCountingBetween(arena, leftLayer, middleLayer)
	}
}

func (decider *SwitchDecider) NotifyOfSwitch(
	arena *layered.LGraphArena,
	upper, lower layered.LNodeID,
) {
	decider.leftInLayerCounter.SwitchNodes(arena, upper, lower, layered.WestSide)
	decider.rightInLayerCounter.SwitchNodes(arena, upper, lower, layered.EastSide)
	if decider.countCrossingsCausedByPortSwitch {
		decider.parentCrossCounter.SwitchPorts(
			arena, externalDummyOriginPort(arena, upper), externalDummyOriginPort(arena, lower),
		)
	}
}

func (decider *SwitchDecider) DoesSwitchReduceCrossings(
	arena *layered.LGraphArena,
	upperIndex, lowerIndex int,
) bool {
	if decider.constraintsPreventSwitch(arena, upperIndex, lowerIndex) {
		return false
	}
	upper, lower := decider.freeLayer[upperIndex], decider.freeLayer[lowerIndex]
	leftUpperLower, leftLowerUpper := decider.leftInLayerCounter.
		CountInLayerCrossingsBetweenNodesInBothOrders(arena, upper, lower, layered.WestSide)
	rightUpperLower, rightLowerUpper := decider.rightInLayerCounter.
		CountInLayerCrossingsBetweenNodesInBothOrders(arena, upper, lower, layered.EastSide)
	decider.northSouthCounter.CountCrossings(arena, upper, lower)
	upperLower := decider.crossingMatrixFiller.GetCrossingMatrixEntry(arena, upper, lower) +
		leftUpperLower + rightUpperLower + decider.northSouthCounter.UpperLowerCrossings
	lowerUpper := decider.crossingMatrixFiller.GetCrossingMatrixEntry(arena, lower, upper) +
		leftLowerUpper + rightLowerUpper + decider.northSouthCounter.LowerUpperCrossings
	if decider.countCrossingsCausedByPortSwitch {
		parentUpperLower, parentLowerUpper := decider.parentCrossCounter.CountCrossingsBetweenPortsInBothOrders(
			arena, externalDummyOriginPort(arena, upper), externalDummyOriginPort(arena, lower),
		)
		upperLower += parentUpperLower
		lowerUpper += parentLowerUpper
	}
	return upperLower > lowerUpper
}

func (decider *SwitchDecider) constraintsPreventSwitch(
	arena *layered.LGraphArena,
	upperIndex, lowerIndex int,
) bool {
	upper, lower := decider.freeLayer[upperIndex], decider.freeLayer[lowerIndex]
	return haveSuccessorConstraint(arena, upper, lower) ||
		haveLayoutUnitConstraint(arena, upper, lower) ||
		areNormalAndNorthSouthPortDummy(arena, upper, lower)
}

func haveSuccessorConstraint(
	arena *layered.LGraphArena,
	upper, lower layered.LNodeID,
) bool {
	for _, successor := range basegraph.Get(&arena.Node(upper).Properties, layered.InLayerSuccessorConstraints) {
		if successor == lower {
			return true
		}
	}
	return false
}

func haveLayoutUnitConstraint(
	arena *layered.LGraphArena,
	upper, lower layered.LNodeID,
) bool {
	neitherLongEdge := arena.Node(upper).Type != layered.LongEdgeNode &&
		arena.Node(lower).Type != layered.LongEdgeNode
	upperUnit, upperFound := basegraph.TryGet(&arena.Node(upper).Properties, layered.InLayerLayoutUnit)
	lowerUnit, lowerFound := basegraph.TryGet(&arena.Node(lower).Properties, layered.InLayerLayoutUnit)
	differentUnits := upperFound != lowerFound || upperUnit != lowerUnit
	nodesHaveUnits := upperFound && upperUnit != upper || lowerFound && lowerUnit != lower
	upperNorthern := hasEdgesOnSide(arena, upper, layered.NorthSide)
	lowerSouthern := hasEdgesOnSide(arena, lower, layered.SouthSide)
	nodesHaveUnits = nodesHaveUnits || hasEdgesOnSide(arena, upper, layered.SouthSide) ||
		hasEdgesOnSide(arena, lower, layered.NorthSide)
	return neitherLongEdge && (nodesHaveUnits && differentUnits || upperNorthern || lowerSouthern)
}

func hasEdgesOnSide(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	side layered.PortSide,
) bool {
	for _, portID := range arena.NodePortsOnSide(nodeID, side) {
		if basegraph.Has(&arena.Port(portID).Properties, layered.PortDummy) ||
			len(arena.PortConnectedEdges(portID)) > 0 {
			return true
		}
	}
	return false
}

func areNormalAndNorthSouthPortDummy(
	arena *layered.LGraphArena,
	upper, lower layered.LNodeID,
) bool {
	return arena.Node(upper).Type == layered.NorthSouthPortNode && arena.Node(lower).Type == layered.NormalNode ||
		arena.Node(lower).Type == layered.NorthSouthPortNode && arena.Node(upper).Type == layered.NormalNode
}

func externalDummyOriginPort(arena *layered.LGraphArena, nodeID layered.LNodeID) layered.LPortID {
	origin, found := basegraph.TryGet(&arena.Node(nodeID).Properties, layered.OriginalElement)
	if !found || origin.Kind != layered.OriginLPort {
		panic("p3order: external port dummy has no LPort origin")
	}
	return origin.LPort
}
