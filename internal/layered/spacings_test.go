// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
)

func TestSpacingUsesMaximumIndividualOverride(t *testing.T) {
	arena := NewLGraphArena()
	graphID := arena.AddGraph()
	first := arena.AddNode(graphID)
	second := arena.AddNode(graphID)
	basegraph.Set(&arena.Graph(graphID).Properties, SpacingNodeNode, 20.0)
	var individual IndividualSpacings
	basegraph.Set(&individual.Properties, SpacingNodeNode, 37.0)
	basegraph.Set(&arena.Node(first).Properties, SpacingIndividual, individual)
	if got := VerticalSpacing(arena, first, second); got != 37 {
		t.Fatalf("vertical spacing = %v, want 37", got)
	}
}

func TestApplySpacingBaseValuePreservesExplicitValue(t *testing.T) {
	var properties basegraph.PropertyMap
	basegraph.Set(&properties, SpacingEdgeNode, 123.0)
	ApplySpacingBaseValue(&properties, 40)
	if got := basegraph.Get(&properties, SpacingEdgeNode); got != 123 {
		t.Fatalf("explicit edge-node spacing = %v", got)
	}
	if got := basegraph.Get(&properties, SpacingNodeNodeBetweenLayers); got != 40 {
		t.Fatalf("scaled between-layer spacing = %v, want 40", got)
	}
}
