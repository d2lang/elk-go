// Copyright (c) 2016 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"testing"

	"github.com/d2lang/elk-go/internal/layered"
)

func TestLayerSweepTypeDeciderAlwaysProcessesRootBottomUp(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = []layered.LayerID{layerID}
	nodeID := addCountingNode(arena, graphID, layerID, layered.EastSide)
	order := [][]layered.LNodeID{{nodeID}}
	decider := NewLayerSweepTypeDecider(1)
	decider.InitAtLayerLevel(arena, 0, order)
	decider.InitAtNodeLevel(arena, 0, 0, order)
	if !decider.UseBottomUp(arena, graphID, nil, false, order) {
		t.Fatal("root graph must be a bottom-up sweep root")
	}
}
