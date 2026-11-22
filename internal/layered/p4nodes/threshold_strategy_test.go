// Copyright (c) 2015 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"math"
	"testing"
)

func TestNullThresholdStrategyUsesDirectionInfinity(t *testing.T) {
	arena, graphID, layers := newPlacementGraph(1)
	nodeID := addPlacementNode(arena, graphID, layers[0], 10)
	info := BuildNeighborhoodInformation(arena, graphID)
	layout := NewBKAlignedLayout(graphID, info.NodeCount, VerticalDown, HorizontalLeft)
	layout.Align[arena.Node(nodeID).ID] = someNode(nodeID)
	strategy := NewThresholdStrategy(false)
	if got := strategy.CalculateThreshold(arena, layout, 42, nodeID, nodeID); !math.IsInf(got, -1) {
		t.Fatalf("down threshold = %v", got)
	}
	layout.VDirection = VerticalUp
	if got := strategy.CalculateThreshold(arena, layout, 42, nodeID, nodeID); !math.IsInf(got, 1) {
		t.Fatalf("up threshold = %v", got)
	}
}
