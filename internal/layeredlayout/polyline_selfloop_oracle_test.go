// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package layeredlayout_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	elk "github.com/d2lang/elk-go"
	"github.com/d2lang/elk-go/internal/oracle"
)

// TestLayeredPolylineSelfLoopFrozen pins the exact ELK 0.8.1 polyline
// self-loop result without requiring Node.js. In particular, the four corner
// cuts distinguish PolylineSelfLoopRouter from its orthogonal skeleton.
func TestLayeredPolylineSelfLoopFrozen(t *testing.T) {
	var graph elk.Graph
	if err := json.Unmarshal(polylineSelfLoopFixture, &graph); err != nil {
		t.Fatal(err)
	}
	if err := elk.Layout(&graph); err != nil {
		t.Fatal(err)
	}
	if len(graph.Children) != 1 {
		t.Fatalf("root children = %d, want 1", len(graph.Children))
	}
	assertNear(t, "root width", elk.Number(graph.Width), 74)
	assertNear(t, "root height", elk.Number(graph.Height), 64)
	assertNear(t, "node x", elk.Number(graph.Children[0].X), 12)
	assertNear(t, "node y", elk.Number(graph.Children[0].Y), 22)
	if len(graph.Edges) != 1 || len(graph.Edges[0].Sections) != 1 {
		t.Fatalf("self-loop sections are incomplete")
	}
	section := graph.Edges[0].Sections[0]
	if len(section.BendPoints) != 4 {
		t.Fatalf("bend points = %d, want 4", len(section.BendPoints))
	}
	assertNear(t, "start x", section.StartPoint.X, 28.666666666666668)
	assertNear(t, "start y", section.StartPoint.Y, 22)
	assertNear(t, "bend 0 x", section.BendPoints[0].X, 28.666666666666668)
	assertNear(t, "bend 0 y", section.BendPoints[0].Y, 17)
	assertNear(t, "bend 1 x", section.BendPoints[1].X, 33.66666666666667)
	assertNear(t, "bend 1 y", section.BendPoints[1].Y, 12)
	assertNear(t, "bend 2 x", section.BendPoints[2].X, 40.333333333333336)
	assertNear(t, "bend 2 y", section.BendPoints[2].Y, 12)
	assertNear(t, "bend 3 x", section.BendPoints[3].X, 45.333333333333336)
	assertNear(t, "bend 3 y", section.BendPoints[3].Y, 17)
	assertNear(t, "end x", section.EndPoint.X, 45.333333333333336)
	assertNear(t, "end y", section.EndPoint.Y, 22)
}

func TestLayeredPolylineSelfLoopOracleParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_LAYERED_STRATEGIES") != "1" {
		t.Skip("set ELK_GO_ORACLE_LAYERED_STRATEGIES=1 to run Layered strategy differentials")
	}
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := oracle.Differential(ctx, runner, polylineSelfLoopFixture,
		elk.LayoutJSON, oracle.DefaultTolerance); err != nil {
		t.Fatalf("%v\ninput: %s", err, polylineSelfLoopFixture)
	}
}

var polylineSelfLoopFixture = []byte(`{
	"id":"root",
	"layoutOptions":{"elk.algorithm":"layered","elk.edgeRouting":"POLYLINE"},
	"children":[{"id":"a","width":50,"height":30}],
	"edges":[{"id":"e","sources":["a"],"targets":["a"]}]
}`)
