// Copyright (c) 2018, 2019 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package routing

import (
	"fmt"
	"sort"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/layered/intermediate/loops"
)

// PlaceLabels determines each aggregate self-loop label's side, alignment,
// and first coordinate. JSON ELK graphs cannot supply Java's ILabelManager,
// so the optional management branch is intentionally inactive, as in elkjs.
func PlaceLabels(arena *layered.LGraphArena, holder *loops.Holder) error {
	var northSequenced, southSequenced []*loops.HyperLoop
	ordering := basegraph.Get(&arena.Node(holder.LNode).Properties, layered.SelfLoopOrdering)

	for _, loop := range holder.HyperLoops {
		if loop.Labels == nil {
			continue
		}
		switch loop.Type {
		case loops.OneSide:
			side := firstOccupiedSide(loop)
			if ordering == layered.SelfLoopOrderingSequenced && side == layered.NorthSide {
				northSequenced = append(northSequenced, loop)
			} else if ordering == layered.SelfLoopOrderingSequenced && side == layered.SouthSide {
				southSequenced = append(southSequenced, loop)
			} else {
				assignOneSidedSimple(arena, loop, side)
			}
		case loops.TwoSidesCorner:
			assignTwoSidesCorner(arena, loop)
		case loops.TwoSidesOpposing, loops.ThreeSides:
			assignOpposingAndThree(loop)
		case loops.FourSides:
			assignFourSides(arena, loop)
		default:
			return fmt.Errorf("unsupported self-loop label placement type")
		}
	}
	if len(northSequenced) != 0 {
		assignOneSidedSequenced(arena, northSequenced, layered.NorthSide)
	}
	if len(southSequenced) != 0 {
		assignOneSidedSequenced(arena, southSequenced, layered.SouthSide)
	}
	for _, loop := range holder.HyperLoops {
		if loop.Labels != nil {
			computeLabelCoordinates(arena, loop)
		}
	}
	return nil
}

func assignOneSidedSimple(arena *layered.LGraphArena, loop *loops.HyperLoop, side layered.PortSide) {
	switch side {
	case layered.EastSide, layered.WestSide:
		topmost := loop.LeftmostPort
		if arena.Port(loop.RightmostPort.LPort).Position.Y < arena.Port(topmost.LPort).Position.Y {
			topmost = loop.RightmostPort
		}
		assignLabel(loop, side, loops.LabelTop, topmost)
	case layered.NorthSide, layered.SouthSide:
		assignLabel(loop, side, loops.LabelCenter, nil)
	}
}

func assignOneSidedSequenced(
	arena *layered.LGraphArena,
	loopList []*loops.HyperLoop,
	side layered.PortSide,
) {
	for index, portID := range arena.Node(loopList[0].Holder.LNode).Ports {
		arena.Port(portID).ID = index
	}
	sort.SliceStable(loopList, func(first, second int) bool {
		firstID := arena.Port(loopList[first].LeftmostPort.LPort).ID
		secondID := arena.Port(loopList[second].LeftmostPort.LPort).ID
		if side == layered.NorthSide {
			return firstID < secondID
		}
		return firstID > secondID
	})
	left, right := 0, len(loopList)-1
	for left < right {
		leftLoop, rightLoop := loopList[left], loopList[right]
		leftReference, rightReference := leftLoop.RightmostPort, rightLoop.LeftmostPort
		if side == layered.SouthSide {
			leftReference, rightReference = leftLoop.LeftmostPort, rightLoop.RightmostPort
		}
		assignLabel(leftLoop, side, loops.LabelRight, leftReference)
		assignLabel(rightLoop, side, loops.LabelLeft, rightReference)
		left++
		right--
	}
	if left == right {
		assignLabel(loopList[left], side, loops.LabelCenter, nil)
	}
}

func assignTwoSidesCorner(arena *layered.LGraphArena, loop *loops.HyperLoop) {
	leftSide := arena.Port(loop.LeftmostPort.LPort).Side
	rightSide := arena.Port(loop.RightmostPort.LPort).Side
	switch {
	case leftSide == layered.NorthSide:
		assignLabel(loop, layered.NorthSide, loops.LabelLeft, loop.LeftmostPort)
	case rightSide == layered.NorthSide:
		assignLabel(loop, layered.NorthSide, loops.LabelRight, loop.RightmostPort)
	case leftSide == layered.SouthSide:
		assignLabel(loop, layered.SouthSide, loops.LabelRight, loop.LeftmostPort)
	case rightSide == layered.SouthSide:
		assignLabel(loop, layered.SouthSide, loops.LabelLeft, loop.RightmostPort)
	}
}

func assignOpposingAndThree(loop *loops.HyperLoop) {
	sides := loop.OccupiedSides
	switch {
	case !sides[layered.NorthSide]:
		assignLabel(loop, layered.SouthSide, loops.LabelCenter, nil)
	case !sides[layered.SouthSide]:
		assignLabel(loop, layered.NorthSide, loops.LabelCenter, nil)
	case !sides[layered.WestSide]:
		assignLabel(loop, layered.NorthSide, loops.LabelLeft, loop.LeftmostPort)
	case !sides[layered.EastSide]:
		assignLabel(loop, layered.NorthSide, loops.LabelRight, loop.RightmostPort)
	}
}

func assignFourSides(arena *layered.LGraphArena, loop *loops.HyperLoop) {
	leftSide := arena.Port(loop.LeftmostPort.LPort).Side
	// ELK 0.8.1 reads leftmost twice here. Keep the typo for elkjs parity.
	rightSide := arena.Port(loop.LeftmostPort.LPort).Side
	if leftSide == layered.NorthSide || rightSide == layered.NorthSide {
		assignLabel(loop, layered.SouthSide, loops.LabelCenter, nil)
	} else {
		assignLabel(loop, layered.NorthSide, loops.LabelCenter, nil)
	}
}

func assignLabel(
	loop *loops.HyperLoop,
	side layered.PortSide,
	alignment loops.LabelAlignment,
	reference *loops.Port,
) {
	loop.Labels.Side = side
	loop.Labels.Alignment = alignment
	loop.Labels.AlignmentReference = reference
}

func computeLabelCoordinates(arena *layered.LGraphArena, loop *loops.HyperLoop) {
	labels := loop.Labels
	switch labels.Alignment {
	case loops.LabelCenter:
		labels.Position.X = (arena.Node(loop.Holder.LNode).Size.X - labels.Size.X) / 2
	case loops.LabelLeft:
		port := arena.Port(labels.AlignmentReference.LPort)
		labels.Position.X = port.Position.X + port.Anchor.X
	case loops.LabelRight:
		port := arena.Port(labels.AlignmentReference.LPort)
		labels.Position.X = port.Position.X + port.Anchor.X - labels.Size.X
	case loops.LabelTop:
		port := arena.Port(labels.AlignmentReference.LPort)
		labels.Position.Y = port.Position.Y + port.Anchor.Y
	}
}

func firstOccupiedSide(loop *loops.HyperLoop) layered.PortSide {
	for side := layered.NorthSide; side <= layered.WestSide; side++ {
		if loop.OccupiedSides[side] {
			return side
		}
	}
	return layered.UndefinedSide
}
