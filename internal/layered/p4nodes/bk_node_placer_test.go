// Copyright (c) 2015 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestBrandesKoepfDiamondRightDown(t *testing.T) {
	arena, graphID, nodes := buildPlacementDiamond()
	basegraph.Set(&arena.Graph(graphID).Properties, layered.FavorStraightEdges, true)
	PlaceBrandesKoepf(arena, graphID)
	want := []float64{2, 2, 35, 2}
	for index, nodeID := range nodes {
		if got := arena.Node(nodeID).Position.Y; got != want[index] {
			t.Errorf("node %d y = %v, want %v", index, got, want[index])
		}
	}
}

func TestBrandesKoepfDiamondBalanced(t *testing.T) {
	arena, graphID, nodes := buildPlacementDiamond()
	basegraph.Set(&arena.Graph(graphID).Properties, layered.FavorStraightEdges, false)
	PlaceBrandesKoepf(arena, graphID)
	want := []float64{8.5, -8, 25, 8.5}
	for index, nodeID := range nodes {
		if got := arena.Node(nodeID).Position.Y; got != want[index] {
			t.Errorf("node %d y = %v, want %v", index, got, want[index])
		}
	}
}

func TestBrandesKoepfDiamondFixedLeftUp(t *testing.T) {
	arena, graphID, nodes := buildPlacementDiamond()
	basegraph.Set(&arena.Graph(graphID).Properties, layered.BKFixedAlignment, layered.FixedAlignmentLeftUp)
	PlaceBrandesKoepf(arena, graphID)
	want := []float64{0, -33, 0, 0}
	for index, nodeID := range nodes {
		if got := arena.Node(nodeID).Position.Y; got != want[index] {
			t.Errorf("node %d y = %v, want %v", index, got, want[index])
		}
	}
}

func TestBrandesKoepfHierarchyConfiguration(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	basegraph.Set(
		&arena.Graph(graphID).Properties,
		layered.GraphProperties,
		layered.GraphPropertyExternalPorts,
	)
	configuration := (BrandesKoepfPlacer{}).ProcessorConfiguration(arena, graphID)
	got := configuration.Slot(int(layered.PhaseEdgeRouting))
	if len(got) != 1 || got[0] != layered.HierarchicalPortPositionProcessor {
		t.Fatalf("before edge routing = %v, want HIERARCHICAL_PORT_POSITION_PROCESSOR", got)
	}
}

func buildPlacementDiamond() (*layered.LGraphArena, layered.LGraphID, []layered.LNodeID) {
	arena, graphID, layers := newPlacementGraph(3)
	basegraph.Set(&arena.Graph(graphID).Properties, layered.SpacingNodeNode, 20.0)
	a := addPlacementNode(arena, graphID, layers[0], 30)
	b := addPlacementNode(arena, graphID, layers[1], 10)
	c := addPlacementNode(arena, graphID, layers[1], 10)
	d := addPlacementNode(arena, graphID, layers[2], 30)
	arena.Node(b).Margin.Top = 2
	arena.Node(b).Margin.Bottom = 3
	connectPlacementNodes(arena, a, b)
	connectPlacementNodes(arena, a, c)
	connectPlacementNodes(arena, b, d)
	connectPlacementNodes(arena, c, d)
	return arena, graphID, []layered.LNodeID{a, b, c, d}
}
