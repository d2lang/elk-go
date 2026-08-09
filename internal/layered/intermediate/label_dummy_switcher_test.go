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

func TestSwitchLabelDummyToMedianLayer(t *testing.T) {
	arena, graphID, dummies := labelDummyChain(t, 4, 0)
	labelDummy := dummies[0]
	if arena.LayerIndex(*arena.Node(labelDummy).Layer) != 0 {
		t.Fatal("test setup did not put label dummy first")
	}

	SwitchLabelDummies(arena, graphID)

	if got := arena.LayerIndex(*arena.Node(labelDummy).Layer); got != 1 {
		t.Fatalf("median label layer = %d, want 1", got)
	}
	if !basegraph.Get(&arena.Node(dummies[0]).Properties, layered.LongEdgeBeforeLabelDummy) &&
		!basegraph.Get(&arena.Node(dummies[1]).Properties, layered.LongEdgeBeforeLabelDummy) {
		// Depending on which arena ID now occupies the predecessor layer, at
		// least one long-edge dummy must be marked.
		t.Fatal("no predecessor dummy was marked")
	}
}

func TestSwitchLabelDummyHeadLayerSetsAlignment(t *testing.T) {
	arena, graphID, dummies := labelDummyChain(t, 3, 0)
	labelDummy := dummies[0]
	basegraph.Set(&arena.Graph(graphID).Properties, layered.CenterEdgeLabelPlacement, layered.CenterLabelHeadLayer)

	SwitchLabelDummies(arena, graphID)

	if got := arena.LayerIndex(*arena.Node(labelDummy).Layer); got != 2 {
		t.Fatalf("head label layer = %d, want 2", got)
	}
	if got := basegraph.Get(&arena.Node(labelDummy).Properties, layered.AlignmentOption); got != layered.AlignmentRight {
		t.Fatalf("head alignment = %v, want RIGHT", got)
	}
}

// labelDummyChain creates source -> dummy... -> target. labelAt is the dummy
// index that represents labels; every other dummy is a long-edge dummy.
func labelDummyChain(t *testing.T, dummyCount, labelAt int) (*layered.LGraphArena, layered.LGraphID, []layered.LNodeID) {
	t.Helper()
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layers := make([]layered.LayerID, dummyCount)
	for index := range layers {
		layers[index] = arena.AddLayerToGraph(graphID)
	}
	source, target := arena.AddNode(graphID), arena.AddNode(graphID)
	previousNode := source
	dummies := make([]layered.LNodeID, dummyCount)
	var previousPort *layered.LPortID
	for index := 0; index <= dummyCount; index++ {
		currentNode := target
		if index < dummyCount {
			currentNode = arena.AddNode(graphID)
			dummies[index] = currentNode
			arena.Node(currentNode).Type = layered.LongEdgeNode
			if index == labelAt {
				arena.Node(currentNode).Type = layered.LabelNode
				labelID := arena.AddLabel("center")
				basegraph.Set(&arena.Node(currentNode).Properties, layered.RepresentedLabels, []layered.LLabelID{labelID})
			}
			arena.SetNodeLayer(currentNode, &layers[index])
		}
		out := arena.AddPort()
		arena.SetPortNode(out, &previousNode)
		in := arena.AddPort()
		arena.SetPortNode(in, &currentNode)
		edgeID := arena.AddEdge()
		arena.SetEdgeSource(edgeID, &out)
		arena.SetEdgeTarget(edgeID, &in)
		_ = previousPort
		previousPort = &in
		previousNode = currentNode
	}
	return arena, graphID, dummies
}
