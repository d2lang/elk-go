//go:build audit

// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0/.
//
// SPDX-License-Identifier: EPL-2.0

package layeredlayout_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	elk "github.com/d2lang/elk-go"
	"github.com/d2lang/elk-go/internal/oracle"
)

type auditEnumCase struct {
	id       string
	values   []string
	target   string
	activate map[string]any
}

func TestAuditAllLayeredEnumValues(t *testing.T) {
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	cases := []auditEnumCase{
		{"elk.direction", []string{"UNDEFINED", "RIGHT", "LEFT", "DOWN", "UP"}, "parent", nil},
		{"elk.hierarchyHandling", []string{"INHERIT", "INCLUDE_CHILDREN", "SEPARATE_CHILDREN"}, "parent", nil},
		{"elk.edgeRouting", []string{"UNDEFINED", "POLYLINE", "ORTHOGONAL", "SPLINES"}, "parent", nil},
		{"elk.alignment", []string{"AUTOMATIC", "LEFT", "RIGHT", "CENTER", "TOP", "BOTTOM"}, "node", nil},
		{"elk.portConstraints", []string{"UNDEFINED", "FREE", "FIXED_SIDE", "FIXED_ORDER", "FIXED_RATIO", "FIXED_POS"}, "node", nil},
		{"elk.port.side", []string{"UNDEFINED", "NORTH", "EAST", "SOUTH", "WEST"}, "port", nil},
		{"elk.edgeLabels.placement", []string{"CENTER", "HEAD", "TAIL"}, "label", nil},
		{"elk.layered.directionCongruency", []string{"READING_DIRECTION", "ROTATION"}, "parent", nil},
		{"elk.layered.interactiveReferencePoint", []string{"CENTER", "TOP_LEFT"}, "parent", nil},
		{"elk.layered.portSortingStrategy", []string{"INPUT_ORDER", "PORT_DEGREE"}, "parent", nil},
		{"elk.layered.cycleBreaking.strategy", []string{"GREEDY", "DEPTH_FIRST", "INTERACTIVE", "MODEL_ORDER", "GREEDY_MODEL_ORDER"}, "parent", nil},
		{"elk.layered.layering.strategy", []string{"NETWORK_SIMPLEX", "LONGEST_PATH", "COFFMAN_GRAHAM", "INTERACTIVE", "STRETCH_WIDTH", "MIN_WIDTH"}, "parent", nil},
		{"elk.layered.layering.layerConstraint", []string{"NONE", "FIRST", "FIRST_SEPARATE", "LAST", "LAST_SEPARATE"}, "node", nil},
		{"elk.layered.layering.nodePromotion.strategy", []string{"NONE", "NIKOLOV", "NIKOLOV_PIXEL", "NIKOLOV_IMPROVED", "NIKOLOV_IMPROVED_PIXEL", "DUMMYNODE_PERCENTAGE", "NODECOUNT_PERCENTAGE", "NO_BOUNDARY"}, "parent", nil},
		{"elk.layered.crossingMinimization.strategy", []string{"LAYER_SWEEP", "INTERACTIVE", "NONE"}, "parent", nil},
		{"elk.layered.crossingMinimization.greedySwitch.type", []string{"ONE_SIDED", "TWO_SIDED", "OFF"}, "parent", map[string]any{"elk.layered.crossingMinimization.greedySwitch.activationThreshold": 0}},
		{"elk.layered.crossingMinimization.greedySwitchHierarchical.type", []string{"ONE_SIDED", "TWO_SIDED", "OFF"}, "parent", map[string]any{"elk.hierarchyHandling": "INCLUDE_CHILDREN"}},
		{"elk.layered.nodePlacement.strategy", []string{"SIMPLE", "INTERACTIVE", "LINEAR_SEGMENTS", "BRANDES_KOEPF", "NETWORK_SIMPLEX"}, "parent", nil},
		{"elk.layered.nodePlacement.bk.edgeStraightening", []string{"NONE", "IMPROVE_STRAIGHTNESS"}, "parent", map[string]any{"elk.layered.nodePlacement.strategy": "BRANDES_KOEPF"}},
		{"elk.layered.nodePlacement.bk.fixedAlignment", []string{"NONE", "LEFTUP", "RIGHTUP", "LEFTDOWN", "RIGHTDOWN", "BALANCED"}, "parent", map[string]any{"elk.layered.nodePlacement.strategy": "BRANDES_KOEPF"}},
		{"elk.layered.nodePlacement.networkSimplex.nodeFlexibility.default", []string{"NONE", "PORT_POSITION", "NODE_SIZE_WHERE_SPACE_PERMITS", "NODE_SIZE"}, "parent", map[string]any{"elk.layered.nodePlacement.strategy": "NETWORK_SIMPLEX"}},
		{"elk.layered.nodePlacement.networkSimplex.nodeFlexibility", []string{"NONE", "PORT_POSITION", "NODE_SIZE_WHERE_SPACE_PERMITS", "NODE_SIZE"}, "node", map[string]any{"elk.layered.nodePlacement.strategy": "NETWORK_SIMPLEX"}},
		{"elk.layered.edgeRouting.selfLoopDistribution", []string{"EQUALLY", "NORTH", "NORTH_SOUTH"}, "node-b", nil},
		{"elk.layered.edgeRouting.selfLoopOrdering", []string{"STACKED", "SEQUENCED"}, "node-b", nil},
		{"elk.layered.edgeRouting.splines.mode", []string{"SLOPPY", "CONSERVATIVE"}, "parent", map[string]any{"elk.edgeRouting": "SPLINES"}},
		{"elk.layered.compaction.postCompaction.strategy", []string{"NONE", "LEFT", "RIGHT", "LEFT_RIGHT_CONSTRAINT_LOCKING", "LEFT_RIGHT_CONNECTION_LOCKING", "EDGE_LENGTH"}, "parent", nil},
		{"elk.layered.compaction.postCompaction.constraints", []string{"QUADRATIC", "SCANLINE"}, "parent", map[string]any{"elk.layered.compaction.postCompaction.strategy": "LEFT_RIGHT_CONSTRAINT_LOCKING"}},
		{"elk.layered.wrapping.strategy", []string{"OFF", "SINGLE_EDGE", "MULTI_EDGE"}, "parent", map[string]any{"elk.aspectRatio": 1.3}},
		{"elk.layered.wrapping.cutting.strategy", []string{"ARD", "MSD", "MANUAL"}, "parent", map[string]any{"elk.layered.wrapping.strategy": "SINGLE_EDGE"}},
		{"elk.layered.wrapping.validify.strategy", []string{"NO", "GREEDY", "LOOK_BACK"}, "parent", map[string]any{"elk.layered.wrapping.strategy": "SINGLE_EDGE"}},
		{"elk.layered.edgeLabels.sideSelection", []string{"ALWAYS_UP", "ALWAYS_DOWN", "DIRECTION_UP", "DIRECTION_DOWN", "SMART_UP", "SMART_DOWN"}, "parent", nil},
		{"elk.layered.edgeLabels.centerLabelPlacementStrategy", []string{"MEDIAN_LAYER", "TAIL_LAYER", "HEAD_LAYER", "SPACE_EFFICIENT_LAYER", "WIDEST_LAYER", "CENTER_LAYER"}, "parent", nil},
		{"elk.layered.considerModelOrder.strategy", []string{"NONE", "NODES_AND_EDGES", "PREFER_EDGES"}, "parent", nil},
		{"elk.layered.considerModelOrder.components", []string{"NONE", "INSIDE_PORT_SIDE_GROUPS", "FORCE_MODEL_ORDER"}, "parent", nil},
		{"elk.layered.considerModelOrder.longEdgeStrategy", []string{"DUMMY_NODE_OVER", "DUMMY_NODE_UNDER", "EQUAL"}, "parent", nil},
	}
	for _, tc := range cases {
		for _, value := range tc.values {
			t.Run(tc.id+"/"+value, func(t *testing.T) {
				var graph map[string]any
				if err := json.Unmarshal([]byte(auditLayeredEnumGraph), &graph); err != nil {
					t.Fatal(err)
				}
				parent := graph["layoutOptions"].(map[string]any)
				if tc.id == "elk.port.side" && value == "UNDEFINED" {
					node := graph["children"].([]any)[0].(map[string]any)
					node["layoutOptions"].(map[string]any)["elk.portConstraints"] = "FREE"
				}
				// ELK 0.8.1 itself throws when non-default network-simplex
				// flexibility is combined with a self-loop. Exercise each public
				// enum value on the same graph without that unrelated oracle bug.
				if tc.id == "elk.layered.nodePlacement.networkSimplex.nodeFlexibility.default" && value != "NONE" {
					edges := graph["edges"].([]any)
					filtered := edges[:0]
					for _, rawEdge := range edges {
						edge := rawEdge.(map[string]any)
						if edge["id"] != "bb" {
							filtered = append(filtered, rawEdge)
						}
					}
					graph["edges"] = filtered
				}
				for key, active := range tc.activate {
					parent[key] = active
				}
				switch tc.target {
				case "parent":
					parent[tc.id] = value
				case "node", "node-b":
					index := 0
					if tc.target == "node-b" {
						index = 1
					}
					node := graph["children"].([]any)[index].(map[string]any)
					node["layoutOptions"].(map[string]any)[tc.id] = value
				case "port":
					node := graph["children"].([]any)[0].(map[string]any)
					port := node["ports"].([]any)[0].(map[string]any)
					port["layoutOptions"].(map[string]any)[tc.id] = value
				case "label":
					edge := graph["edges"].([]any)[0].(map[string]any)
					label := edge["labels"].([]any)[0].(map[string]any)
					label["layoutOptions"].(map[string]any)[tc.id] = value
				default:
					t.Fatalf("bad target %q", tc.target)
				}
				input, err := json.Marshal(graph)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := oracle.Differential(ctx, runner, input, elk.LayoutJSON, oracle.DefaultTolerance); err != nil {
					want, _ := runner.Layout(ctx, input)
					got, _ := elk.LayoutJSON(input)
					t.Fatalf("%v\ninput: %s\nwant: %s\ngot: %s", err, input, want, got)
				}
			})
		}
	}
}

const auditLayeredEnumGraph = `{
  "id":"root",
  "layoutOptions":{"elk.algorithm":"layered","elk.randomSeed":7},
  "children":[
    {"id":"a","x":3,"y":80,"width":34,"height":24,
      "layoutOptions":{"elk.portConstraints":"FIXED_SIDE","elk.position":"(3,80)"},
      "ports":[{"id":"ap","x":30,"y":8,"width":4,"height":4,
        "layoutOptions":{"elk.port.side":"EAST"}}]},
    {"id":"b","x":90,"y":10,"width":42,"height":28,"layoutOptions":{"elk.position":"(90,10)"}},
    {"id":"c","x":90,"y":100,"width":26,"height":35,"layoutOptions":{"elk.position":"(90,100)"}},
    {"id":"d","x":170,"y":50,"width":38,"height":18,"layoutOptions":{"elk.position":"(170,50)"}},
    {"id":"e","x":240,"y":20,"width":29,"height":31,"layoutOptions":{"elk.position":"(240,20)"}},
    {"id":"f","x":240,"y":110,"width":33,"height":22,"layoutOptions":{"elk.position":"(240,110)"}}
  ],
  "edges":[
    {"id":"ab","sources":["ap"],"targets":["b"],
      "labels":[{"id":"abl","text":"ab","width":15,"height":8,
        "layoutOptions":{"elk.edgeLabels.placement":"CENTER"}}]},
    {"id":"bb","sources":["b"],"targets":["b"]},
    {"id":"ac","sources":["a"],"targets":["c"]},
    {"id":"bd","sources":["b"],"targets":["d"]},
    {"id":"cd","sources":["c"],"targets":["d"]},
    {"id":"de","sources":["d"],"targets":["e"]},
    {"id":"df","sources":["d"],"targets":["f"]},
    {"id":"fa","sources":["f"],"targets":["a"]}
  ]
}`
