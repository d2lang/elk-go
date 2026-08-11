// Copyright (c) 2012, 2017 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestSelectSmartLabelSidesPlacesSingleTopDummyAbove(t *testing.T) {
	arena, graphID, dummies := labelDummyChain(t, 2, 0)
	dummyID := dummies[0]
	labelLayer := arena.Node(dummyID).Layer
	arena.SetNodeLayer(dummies[1], labelLayer)
	origin := arena.NodeIncomingEdges(dummyID)[0]
	basegraph.Set(&arena.Node(dummyID).Properties, layered.OriginalElement, layered.LEdgeOrigin(origin))
	basegraph.Set(&arena.Graph(graphID).Properties, layered.EdgeLabelSideSelectionOption, layered.EdgeLabelSideSmartDown)

	SelectLabelSides(arena, graphID)

	if got := basegraph.Get(&arena.Node(dummyID).Properties, layered.InternalLabelSide); got != layered.LabelSideAbove {
		t.Fatalf("label side = %v, want ABOVE", got)
	}
}

func TestSelectLabelSidesUsesInlineAndMovesPorts(t *testing.T) {
	arena, graphID, dummies := labelDummyChain(t, 1, 0)
	dummyID := dummies[0]
	origin := arena.NodeIncomingEdges(dummyID)[0]
	basegraph.Set(&arena.Node(dummyID).Properties, layered.OriginalElement, layered.LEdgeOrigin(origin))
	basegraph.Set(&arena.Graph(graphID).Properties, layered.SpacingEdgeLabel, 2.0)
	arena.Node(dummyID).Size.Y = 12
	labels := basegraph.Get(&arena.Node(dummyID).Properties, layered.RepresentedLabels)
	basegraph.Set(&arena.Label(labels[0]).Properties, layered.EdgeLabelsInline, true)
	basegraph.Set(&arena.Graph(graphID).Properties, layered.EdgeLabelSideSelectionOption, layered.EdgeLabelSideAlwaysDown)

	SelectLabelSides(arena, graphID)

	if got := basegraph.Get(&arena.Node(dummyID).Properties, layered.InternalLabelSide); got != layered.LabelSideInline {
		t.Fatalf("label side = %v, want INLINE", got)
	}
	if got := arena.Node(dummyID).Size.Y; got != 9 {
		t.Fatalf("inline dummy height = %v, want 9", got)
	}
	for _, portID := range arena.Node(dummyID).Ports {
		if got := arena.Port(portID).Position.Y; got != 4.5 {
			t.Fatalf("inline port y = %v, want 4.5", got)
		}
	}
}
