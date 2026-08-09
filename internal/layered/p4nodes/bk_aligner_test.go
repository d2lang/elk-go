// Copyright (c) 2015 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"testing"

	"github.com/d2lang/elk-go/internal/layered"
)

func TestBKAlignerBuildsAndSizesStraightBlock(t *testing.T) {
	arena, graphID, layers := newPlacementGraph(2)
	first := addPlacementNode(arena, graphID, layers[0], 10)
	second := addPlacementNode(arena, graphID, layers[1], 20)
	connectPlacementNodes(arena, first, second)
	info := BuildNeighborhoodInformation(arena, graphID)
	layout := NewBKAlignedLayout(graphID, info.NodeCount, VerticalDown, HorizontalRight)
	aligner := NewBKAligner(arena, graphID, info)
	aligner.VerticalAlignment(layout, nil)
	if requireNode(layout.Root[arena.Node(second).ID], "root") != first {
		t.Fatalf("second root = %v, want %v", requireNode(layout.Root[arena.Node(second).ID], "root"), first)
	}
	aligner.InsideBlockShift(layout)
	if got := requireFloat(layout.BlockSize[arena.Node(first).ID], "block size"); got != 20 {
		t.Fatalf("block size = %v, want 20", got)
	}
	if got := requireFloat(layout.InnerShift[arena.Node(first).ID], "shift"); got != 0 {
		t.Fatalf("root inner shift = %v, want 0", got)
	}
}

func TestBKAlignerHonorsMarkedConflict(t *testing.T) {
	arena, graphID, layers := newPlacementGraph(2)
	first := addPlacementNode(arena, graphID, layers[0], 10)
	second := addPlacementNode(arena, graphID, layers[1], 10)
	edgeID := connectPlacementNodes(arena, first, second)
	info := BuildNeighborhoodInformation(arena, graphID)
	layout := NewBKAlignedLayout(graphID, 2, VerticalDown, HorizontalRight)
	NewBKAligner(arena, graphID, info).VerticalAlignment(layout, map[layered.LEdgeID]bool{edgeID: true})
	if got := requireNode(layout.Root[arena.Node(second).ID], "root"); got != second {
		t.Fatalf("marked edge aligned block to %v", got)
	}
}
