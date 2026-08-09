// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"testing"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestCommentProcessorsExtractReserveAndRestoreSingleComment(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	realNode := arena.AddNode(graphID)
	comment := arena.AddNode(graphID)
	arena.Node(realNode).Size = geometry.Point{X: 100, Y: 50}
	arena.Node(comment).Size = geometry.Point{X: 40, Y: 20}
	basegraph.Set(&arena.Node(comment).Properties, core.CommentBox, true)
	arena.Graph(graphID).LayerlessNodes = []layered.LNodeID{realNode, comment}

	commentPort := arena.AddPort()
	arena.SetPortNode(commentPort, &comment)
	realPort := arena.AddPort()
	arena.SetPortNode(realPort, &realNode)
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &commentPort)
	arena.SetEdgeTarget(edgeID, &realPort)

	if err := PreprocessComments(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if got := arena.Graph(graphID).LayerlessNodes; len(got) != 1 || got[0] != realNode {
		t.Fatalf("layerless nodes after extraction = %v", got)
	}
	if arena.Edge(edgeID).Target != nil || arena.Port(realPort).Node != nil {
		t.Fatal("comment's normal-node endpoint was not detached")
	}
	if top, found := basegraph.TryGet(&arena.Node(realNode).Properties, layered.TopComments); !found || len(top) != 1 || top[0] != comment {
		t.Fatalf("top comments = %v, found=%v", top, found)
	}

	layerID := arena.AddLayerToGraph(graphID)
	arena.SetNodeLayer(realNode, &layerID)
	CalculateCommentNodeMargins(arena, graphID)
	if got := arena.Node(realNode).Margin; got.Top != 30 {
		t.Fatalf("comment margin = %+v, want top 30", got)
	}
	arena.Node(realNode).Position = geometry.Point{X: 12, Y: 42}
	if err := PostprocessComments(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if got := arena.Node(comment).Position; got != (geometry.Point{X: 42, Y: 12}) {
		t.Fatalf("comment position = %+v, want (42,12)", got)
	}
	if arena.Node(comment).Layer == nil || *arena.Node(comment).Layer != layerID ||
		arena.Edge(edgeID).Target == nil || *arena.Edge(edgeID).Target != realPort ||
		arena.Port(realPort).Node == nil || *arena.Port(realPort).Node != realNode {
		t.Fatal("comment node or edge was not restored")
	}
	if got := arena.Port(commentPort).Position; got != (geometry.Point{X: 20, Y: 20}) {
		t.Fatalf("comment port position = %+v", got)
	}
	if got := arena.Port(realPort).Position; got != (geometry.Point{X: 50, Y: 0}) {
		t.Fatalf("node port position = %+v", got)
	}
}
