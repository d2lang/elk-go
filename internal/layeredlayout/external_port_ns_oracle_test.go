// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package layeredlayout_test

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"

	elk "github.com/d2lang/elk-go"
	"github.com/d2lang/elk-go/internal/oracle"
)

// TestLayeredExternalPortNorthWestComponentOfficial0120 pins the observable result
// of the external-port preprocessing and component-group path. The north/west
// component deliberately has two implicit ports: the peculiar ELK 0.8.1
// two-port fixed-side ordering retained by official 0.12 affects its routing
// and the final placement of every following component.
func TestLayeredExternalPortNorthWestComponentOfficial0120(t *testing.T) {
	var graph elk.Graph
	if err := json.Unmarshal([]byte(externalPortNorthWestComponentFixture), &graph); err != nil {
		t.Fatal(err)
	}
	if err := elk.Layout(&graph); err != nil {
		t.Fatal(err)
	}
	if len(graph.Children) != 1 {
		t.Fatalf("root children = %d, want 1", len(graph.Children))
	}
	nested := graph.Children[0]
	assertNear(t, "graph width", elk.Number(nested.Width), 187)
	assertNear(t, "graph height", elk.Number(nested.Height), 158)

	wantY := map[string]float64{
		"a": 12,
		"b": 49,
		"c": 116,
		"d": 49,
		"u": 81,
	}
	for _, child := range nested.Children {
		want, found := wantY[child.ID.String()]
		if !found {
			t.Fatalf("unexpected child %q", child.ID.String())
		}
		assertNear(t, child.ID.String()+" y", elk.Number(child.Y), want)
		delete(wantY, child.ID.String())
	}
	if len(wantY) != 0 {
		t.Fatalf("missing children: %v", wantY)
	}

	if len(nested.Edges) < 2 || len(nested.Edges[0].Sections) != 1 ||
		len(nested.Edges[1].Sections) != 1 {
		t.Fatalf("north/west routes are incomplete")
	}
	west := nested.Edges[0].Sections[0]
	north := nested.Edges[1].Sections[0]
	if len(west.BendPoints) != 2 || len(north.BendPoints) != 1 {
		t.Fatalf("north/west bend counts = %d/%d, want 1/2",
			len(north.BendPoints), len(west.BendPoints))
	}
	assertNear(t, "north bend y", north.BendPoints[0].Y, 18.333333333333332)
	assertNear(t, "west first bend y", west.BendPoints[0].Y, 28.333333333333332)
}

// TestLayeredExternalPortNorthWestComponentOracleParity checks the complete
// graph, including all port coordinates and routed sections, against the
// official elkjs 0.12.0 artifact.
func TestLayeredExternalPortNorthWestComponentOracleParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_LAYERED_STRATEGIES") != "1" {
		t.Skip("set ELK_GO_ORACLE_LAYERED_STRATEGIES=1 to run Layered strategy differentials")
	}
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := oracle.Differential(ctx, runner,
		[]byte(externalPortNorthWestComponentFixture), elk.LayoutJSON,
		oracle.ExactTolerance); err != nil {
		t.Fatalf("%v\ninput: %s", err, externalPortNorthWestComponentFixture)
	}
}

func assertNear(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("%s = %.12g, want %.12g", name, got, want)
	}
}

const externalPortNorthWestComponentFixture = `{
	"id":"root",
	"layoutOptions":{"elk.algorithm":"fixed"},
	"children":[{
		"id":"g","x":0,"y":0,"width":160,"height":120,
		"layoutOptions":{
			"elk.algorithm":"layered",
			"elk.direction":"RIGHT",
			"elk.portConstraints":"FIXED_SIDE",
			"elk.spacing.componentComponent":"17"
		},
		"ports":[
			{"id":"pn","x":70,"y":0,"width":4,"height":4,"layoutOptions":{"elk.port.side":"NORTH"}},
			{"id":"pe","x":156,"y":30,"width":4,"height":4,"layoutOptions":{"elk.port.side":"EAST"}},
			{"id":"ps","x":100,"y":116,"width":4,"height":4,"layoutOptions":{"elk.port.side":"SOUTH"}},
			{"id":"pw","x":0,"y":80,"width":4,"height":4,"layoutOptions":{"elk.port.side":"WEST"}}
		],
		"children":[
			{"id":"a","width":30,"height":20,"ports":[
				{"id":"aw","width":2,"height":2},
				{"id":"an","width":2,"height":2}
			]},
			{"id":"b","width":25,"height":25,"ports":[{"id":"be","width":2,"height":2}]},
			{"id":"c","width":20,"height":30,"ports":[{"id":"cs","width":2,"height":2}]},
			{"id":"d","width":35,"height":15},
			{"id":"u","width":18,"height":18}
		],
		"edges":[
			{"id":"wa","sources":["pw"],"targets":["aw"]},
			{"id":"na","sources":["pn"],"targets":["an"]},
			{"id":"beedge","sources":["be"],"targets":["pe"]},
			{"id":"csedge","sources":["cs"],"targets":["ps"]}
		]
	}]
}`
