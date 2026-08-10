// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"reflect"
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestPrepareNodePlacementRepeatFixesSizeAndRedistributesPorts(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = []layered.LayerID{layerID}
	nodeID := arena.AddNode(graphID)
	arena.SetNodeLayer(nodeID, &layerID)
	node := arena.Node(nodeID)
	node.Size.X, node.Size.Y = 50, 32
	node.Position.Y = 99
	basegraph.Set(&node.Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedPosition)
	basegraph.Set(&node.Properties, layered.OriginalPortConstraints, layered.PortConstraintsFixedOrder)
	basegraph.Set(&node.Properties, layered.NetworkSimplexNodeFlexibility, layered.NodeFlexibilitySize)
	basegraph.Set(&node.Properties, layered.NodeSizeConstraints, []string{"PORTS"})
	basegraph.Set(
		&arena.Graph(graphID).Properties,
		layered.NetworkSimplexNodeFlexibilityDefault,
		layered.NodeFlexibilitySize,
	)

	for _, y := range []float64{0, 14, 28} {
		portID := arena.AddPort()
		arena.SetPortNode(portID, &nodeID)
		arena.SetPortSide(portID, layered.EastSide)
		port := arena.Port(portID)
		port.Size.X, port.Size.Y = 4, 4
		port.Position.Y = y
	}

	PrepareNodePlacementRepeat(arena, graphID)

	if node.Position.Y != 0 {
		t.Fatalf("node y = %v, want 0", node.Position.Y)
	}
	if _, present := basegraph.TryGet(&node.Properties, layered.NetworkSimplexNodeFlexibility); present {
		t.Fatal("node flexibility remains configured")
	}
	if _, present := basegraph.TryGet(&node.Properties, layered.NodeSizeConstraints); present {
		t.Fatal("node size constraints remain configured")
	}
	if got := basegraph.Get(
		&arena.Graph(graphID).Properties,
		layered.NetworkSimplexNodeFlexibilityDefault,
	); got != layered.NodeFlexibilityNone {
		t.Fatalf("default node flexibility = %v, want NONE", got)
	}
	if got := basegraph.Get(&node.Properties, layered.PortConstraintsOption); got != layered.PortConstraintsFixedPosition {
		t.Fatalf("port constraints = %v, want FIXED_POS after sizing", got)
	}
	gotPositions := make([]float64, len(node.Ports))
	for index, portID := range node.Ports {
		gotPositions[index] = arena.Port(portID).Position.Y
	}
	if want := []float64{5, 14, 23}; !reflect.DeepEqual(gotPositions, want) {
		t.Fatalf("port y positions = %v, want %v", gotPositions, want)
	}
	if node.Margin.Right != 4 {
		t.Fatalf("right margin = %v, want 4", node.Margin.Right)
	}
}
