// Copyright (c) 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import "github.com/d2lang/elk-go/internal/layered"

type CrossingCountSide uint8

const (
	CrossingsWest CrossingCountSide = iota
	CrossingsEast
)

// CrossingMatrixFiller lazily caches both orders of each node pair. Node.ID
// is the immutable identity assigned by GraphInfoHolder initialization; it is
// deliberately not changed when the current order swaps nodes.
type CrossingMatrixFiller struct {
	filled   [][]bool
	matrix   [][]int
	counter  *betweenLayerEdgeTwoNodeCrossingsCounter
	side     CrossingCountSide
	oneSided bool
}

func NewCrossingMatrixFiller(
	arena *layered.LGraphArena,
	crossMinType CrossMinType,
	order [][]layered.LNodeID,
	freeLayerIndex int,
	side CrossingCountSide,
) *CrossingMatrixFiller {
	length := len(order[freeLayerIndex])
	filled := make([][]bool, length)
	matrix := make([][]int, length)
	for index := 0; index < length; index++ {
		filled[index] = make([]bool, length)
		matrix[index] = make([]int, length)
	}
	return &CrossingMatrixFiller{
		filled:   filled,
		matrix:   matrix,
		counter:  newBetweenLayerEdgeTwoNodeCrossingsCounter(arena, order, freeLayerIndex),
		side:     side,
		oneSided: crossMinType == CrossMinOneSidedGreedySwitch,
	}
}

func (filler *CrossingMatrixFiller) GetCrossingMatrixEntry(
	arena *layered.LGraphArena,
	upper, lower layered.LNodeID,
) int {
	upperID, lowerID := arena.Node(upper).ID, arena.Node(lower).ID
	if !filler.filled[upperID][lowerID] {
		if filler.oneSided {
			if filler.side == CrossingsEast {
				filler.counter.countEasternEdgeCrossings(arena, upper, lower)
			} else {
				filler.counter.countWesternEdgeCrossings(arena, upper, lower)
			}
		} else {
			filler.counter.countBothSideCrossings(arena, upper, lower)
		}
		filler.matrix[upperID][lowerID] = filler.counter.upperLowerCrossings
		filler.matrix[lowerID][upperID] = filler.counter.lowerUpperCrossings
		filler.filled[upperID][lowerID] = true
		filler.filled[lowerID][upperID] = true
	}
	return filler.matrix[upperID][lowerID]
}
