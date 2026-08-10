// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package layeredlayout_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	elk "github.com/d2lang/elk-go"
	"github.com/d2lang/elk-go/internal/oracle"
)

// TestLayeredAncestorToGrandchildEdgeOfficial0120 pins official elkjs 0.12.0's handling of an
// edge from a compound node to a non-direct descendant. The source exits the
// compound's south side and routes outside the intermediate compound before
// entering it from the north.
func TestLayeredAncestorToGrandchildEdgeOfficial0120(t *testing.T) {
	var graph elk.Graph
	if err := json.Unmarshal([]byte(ancestorToGrandchildFixture), &graph); err != nil {
		t.Fatal(err)
	}
	if err := elk.Layout(&graph); err != nil {
		t.Fatal(err)
	}
	if len(graph.Children) != 1 || len(graph.Children[0].Children) != 1 ||
		len(graph.Edges) != 1 || len(graph.Edges[0].Sections) != 1 {
		t.Fatalf("incomplete ancestor fixture layout")
	}

	ancestor := graph.Children[0]
	intermediate := ancestor.Children[0]
	section := graph.Edges[0].Sections[0]
	if len(section.BendPoints) != 2 {
		t.Fatalf("ancestor route bend points = %d, want 2", len(section.BendPoints))
	}
	assertNear(t, "ancestor width", elk.Number(ancestor.Width), 320)
	assertNear(t, "ancestor height", elk.Number(ancestor.Height), 305)
	assertNear(t, "source x", section.StartPoint.X, 270)
	assertNear(t, "source y", section.StartPoint.Y, 305)
	assertNear(t, "first bend x", section.BendPoints[0].X, 270)
	assertNear(t, "first bend y", section.BendPoints[0].Y, 50)
	assertNear(t, "second bend x", section.BendPoints[1].X, 140)
	assertNear(t, "second bend y", section.BendPoints[1].Y, 50)
	assertNear(t, "target x", section.EndPoint.X, 140)
	assertNear(t, "target y", section.EndPoint.Y, 145)

	if section.BendPoints[0].X <= elk.Number(intermediate.X)+elk.Number(intermediate.Width) {
		t.Fatalf("source leg x = %g, want it outside intermediate right edge %g",
			section.BendPoints[0].X,
			elk.Number(intermediate.X)+elk.Number(intermediate.Width))
	}
	if section.BendPoints[1].Y >= elk.Number(intermediate.Y) {
		t.Fatalf("entry bend y = %g, want it above intermediate top %g",
			section.BendPoints[1].Y, elk.Number(intermediate.Y))
	}
}

// TestLayeredAncestorToGrandchildEdgeOracleParity compares the complete graph
// with official elkjs 0.12.0.
func TestLayeredAncestorToGrandchildEdgeOracleParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_LAYERED_STRATEGIES") != "1" {
		t.Skip("set ELK_GO_ORACLE_LAYERED_STRATEGIES=1 to run Layered strategy differentials")
	}
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := oracle.Differential(ctx, runner, []byte(ancestorToGrandchildFixture),
		elk.LayoutJSON, oracle.ExactTolerance); err != nil {
		t.Fatalf("%v\ninput: %s", err, ancestorToGrandchildFixture)
	}
}

// TestLayeredAncestorSharedPortOutsideSelfLoopOfficial0120 pins the parent-port
// net flow retained by official elkjs 0.12.0. An outside self-loop contributes one output and one
// input vote, so it must not change the side selected by the remaining edges.
func TestLayeredAncestorSharedPortOutsideSelfLoopOfficial0120(t *testing.T) {
	tests := []struct {
		direction             string
		rootWidth, rootHeight float64
		ancestorX, ancestorY  float64
		ancestorWidth         float64
		ancestorHeight        float64
		portX, portY          float64
		sourceX, sourceY      float64
		targetX, targetY      float64
	}{
		{
			direction: "DOWN", rootWidth: 154, rootHeight: 199,
			ancestorX: 12, ancestorY: 62, ancestorWidth: 130, ancestorHeight: 125,
			portX: 65, portY: 0, sourceX: 65, sourceY: 0, targetX: 65, targetY: 55,
		},
		{
			direction: "RIGHT", rootWidth: 219, rootHeight: 144,
			ancestorX: 72, ancestorY: 12, ancestorWidth: 135, ancestorHeight: 120,
			portX: 0, portY: 60, sourceX: 0, sourceY: 60, targetX: 55, targetY: 60,
		},
	}
	for _, test := range tests {
		t.Run(test.direction, func(t *testing.T) {
			var graph elk.Graph
			fixture := ancestorSharedPortOutsideSelfLoopFixture(test.direction)
			if err := json.Unmarshal(fixture, &graph); err != nil {
				t.Fatal(err)
			}
			if err := elk.Layout(&graph); err != nil {
				t.Fatal(err)
			}
			if len(graph.Children) != 3 || graph.Children[2].ID.String() != "a" ||
				len(graph.Children[2].Ports) != 1 || len(graph.Edges) != 4 ||
				graph.Edges[0].ID.String() != "ancestor" || len(graph.Edges[0].Sections) != 1 {
				t.Fatalf("incomplete shared-port fixture layout")
			}

			ancestor := graph.Children[2]
			port := ancestor.Ports[0]
			section := graph.Edges[0].Sections[0]
			if section.StartPoint == nil || section.EndPoint == nil {
				t.Fatal("ancestor route has no endpoints")
			}
			assertNear(t, "root width", elk.Number(graph.Width), test.rootWidth)
			assertNear(t, "root height", elk.Number(graph.Height), test.rootHeight)
			assertNear(t, "ancestor x", elk.Number(ancestor.X), test.ancestorX)
			assertNear(t, "ancestor y", elk.Number(ancestor.Y), test.ancestorY)
			assertNear(t, "ancestor width", elk.Number(ancestor.Width), test.ancestorWidth)
			assertNear(t, "ancestor height", elk.Number(ancestor.Height), test.ancestorHeight)
			assertNear(t, "shared port x", elk.Number(port.X), test.portX)
			assertNear(t, "shared port y", elk.Number(port.Y), test.portY)
			assertNear(t, "source x", section.StartPoint.X, test.sourceX)
			assertNear(t, "source y", section.StartPoint.Y, test.sourceY)
			assertNear(t, "target x", section.EndPoint.X, test.targetX)
			assertNear(t, "target y", section.EndPoint.Y, test.targetY)
		})
	}
}

func TestLayeredAncestorSharedPortOutsideSelfLoopOracleParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_LAYERED_STRATEGIES") != "1" {
		t.Skip("set ELK_GO_ORACLE_LAYERED_STRATEGIES=1 to run Layered strategy differentials")
	}
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	for _, direction := range []string{"DOWN", "RIGHT"} {
		t.Run(direction, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			fixture := ancestorSharedPortOutsideSelfLoopFixture(direction)
			if err := oracle.Differential(ctx, runner, fixture, elk.LayoutJSON,
				oracle.ExactTolerance); err != nil {
				t.Fatalf("%v\ninput: %s", err, fixture)
			}
		})
	}
}

func ancestorSharedPortOutsideSelfLoopFixture(direction string) []byte {
	return []byte(fmt.Sprintf(`{
		"id":"root",
		"layoutOptions":{
			"elk.algorithm":"layered",
			"elk.direction":"%s",
			"elk.hierarchyHandling":"INCLUDE_CHILDREN"
		},
		"children":[
			{"id":"x","width":30,"height":20},
			{"id":"y","width":30,"height":20},
			{
				"id":"a","width":80,"height":60,
				"ports":[{"id":"p","width":0,"height":0}],
				"layoutOptions":{
					"elk.hierarchyHandling":"INCLUDE_CHILDREN",
					"elk.padding":"[top=30,left=30,bottom=30,right=30]"
				},
				"children":[{
					"id":"b","width":60,"height":40,
					"layoutOptions":{
						"elk.hierarchyHandling":"INCLUDE_CHILDREN",
						"elk.padding":"[top=20,left=20,bottom=20,right=20]"
					},
					"children":[{"id":"c","width":30,"height":20}]
				}]
			}
		],
		"edges":[
			{"id":"ancestor","sources":["p"],"targets":["c"]},
			{"id":"loop","sources":["p"],"targets":["p"]},
			{"id":"in-x","sources":["x"],"targets":["p"]},
			{"id":"in-y","sources":["y"],"targets":["p"]}
		]
	}`, direction))
}

const ancestorToGrandchildFixture = `{
	"id":"",
	"layoutOptions":{
		"elk.algorithm":"layered",
		"elk.direction":"DOWN",
		"elk.hierarchyHandling":"INCLUDE_CHILDREN",
		"elk.padding":"[top=50,left=50,bottom=50,right=50]",
		"elk.spacing.edgeNode":40,
		"spacing.edgeNodeBetweenLayers":40,
		"spacing.nodeNodeBetweenLayers":70
	},
	"children":[{
		"id":"a","width":80,"height":60,
		"layoutOptions":{
			"elk.hierarchyHandling":"INCLUDE_CHILDREN",
			"elk.nodeSize.constraints":"MINIMUM_SIZE",
			"elk.nodeSize.minimum":"(80, 60)",
			"elk.padding":"[top=50,left=50,bottom=50,right=50]",
			"elk.spacing.edgeNode":40,
			"spacing.edgeNodeBetweenLayers":40,
			"spacing.nodeNodeBetweenLayers":70
		},
		"children":[{
			"id":"a.b","width":80,"height":60,
			"layoutOptions":{
				"elk.hierarchyHandling":"INCLUDE_CHILDREN",
				"elk.nodeSize.constraints":"MINIMUM_SIZE",
				"elk.nodeSize.minimum":"(80, 60)",
				"elk.padding":"[top=50,left=50,bottom=50,right=50]",
				"elk.spacing.edgeNode":40,
				"spacing.edgeNodeBetweenLayers":40,
				"spacing.nodeNodeBetweenLayers":70
			},
			"children":[{"id":"a.b.c","width":80,"height":60}]
		}]
	}],
	"edges":[{"id":"e","sources":["a"],"targets":["a.b.c"]}]
}`
