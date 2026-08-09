// Copyright (c) 2015 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"math"
	"testing"

	"github.com/d2lang/elk-go/internal/layered"
)

func TestBKAlignedLayoutSizeDeltaAndShift(t *testing.T) {
	arena, graphID, layers := newPlacementGraph(2)
	first := addPlacementNode(arena, graphID, layers[0], 10)
	second := addPlacementNode(arena, graphID, layers[1], 20)
	edgeID := connectPlacementNodes(arena, first, second)
	info := BuildNeighborhoodInformation(arena, graphID)
	layout := NewBKAlignedLayout(graphID, info.NodeCount, VerticalDown, HorizontalLeft)
	for _, nodeID := range []layered.LNodeID{first, second} {
		index := arena.Node(nodeID).ID
		layout.Root[index] = nodePointer(nodeID)
		layout.Align[index] = nodePointer(nodeID)
		layout.InnerShift[index] = floatPointer(0)
		layout.BlockSize[index] = floatPointer(arena.Node(nodeID).Size.Y)
	}
	layout.Y[arena.Node(first).ID] = floatPointer(5)
	layout.Y[arena.Node(second).ID] = floatPointer(30)
	if got := layout.LayoutSize(arena); got != 45 {
		t.Fatalf("layout size = %v, want 45", got)
	}
	source, target := *arena.Edge(edgeID).Source, *arena.Edge(edgeID).Target
	if got := layout.CalculateDelta(arena, source, target); got != 25 {
		t.Fatalf("delta = %v, want 25", got)
	}
	layout.ShiftBlock(arena, first, 3)
	if got := requireFloat(layout.Y[arena.Node(first).ID], "y"); got != 8 {
		t.Fatalf("shifted y = %v, want 8", got)
	}
	if layout.String() != "LEFTDOWN" {
		t.Fatalf("String = %q", layout.String())
	}
}

func TestBKAlignedLayoutSpacingChecks(t *testing.T) {
	arena, graphID, layers := newPlacementGraph(1)
	upper := addPlacementNode(arena, graphID, layers[0], 10)
	lower := addPlacementNode(arena, graphID, layers[0], 10)
	info := BuildNeighborhoodInformation(arena, graphID)
	layout := NewBKAlignedLayout(graphID, 2, VerticalDown, HorizontalLeft)
	for _, nodeID := range []layered.LNodeID{upper, lower} {
		index := arena.Node(nodeID).ID
		layout.Root[index] = nodePointer(nodeID)
		layout.Align[index] = nodePointer(nodeID)
		layout.InnerShift[index] = floatPointer(0)
		layout.BlockSize[index] = floatPointer(10)
	}
	layout.Y[arena.Node(upper).ID] = floatPointer(0)
	layout.Y[arena.Node(lower).ID] = floatPointer(40)
	if got := layout.CheckSpaceAbove(arena, lower, math.Inf(1), info); got != 10 {
		t.Fatalf("space above = %v, want 10", got)
	}
}
