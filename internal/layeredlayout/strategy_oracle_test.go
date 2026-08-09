// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package layeredlayout_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	elk "github.com/d2lang/elk-go"
	"github.com/d2lang/elk-go/internal/oracle"
)

// TestLayeredStrategyOracleParity pins every translated alternate phase to
// the frozen elkjs 0.8.2 / ELK 0.8.1 oracle. It is opt-in because normal
// module consumers do not have the frozen JavaScript artifact or Node.js.
func TestLayeredStrategyOracleParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_LAYERED_STRATEGIES") != "1" {
		t.Skip("set ELK_GO_ORACLE_LAYERED_STRATEGIES=1 to run Layered strategy differentials")
	}
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		option string
		value  string
		body   string
	}{
		{"cycle-depth-first", "cycleBreaking.strategy", "DEPTH_FIRST", cycleStrategyBody},
		{"cycle-interactive", "cycleBreaking.strategy", "INTERACTIVE", cycleStrategyBody},
		{"cycle-model-order", "cycleBreaking.strategy", "MODEL_ORDER", cycleStrategyBody},
		{"layering-longest-path", "layering.strategy", "LONGEST_PATH", twoLayerStrategyBody},
		{"layering-coffman-graham", "layering.strategy", "COFFMAN_GRAHAM", crossingStrategyBody},
		{"layering-interactive", "layering.strategy", "INTERACTIVE", interactiveLayeringBody},
		{"layering-stretch-width", "layering.strategy", "STRETCH_WIDTH", crossingStrategyBody},
		{"layering-min-width", "layering.strategy", "MIN_WIDTH", crossingStrategyBody},
		{"crossing-interactive", "crossingMinimization.strategy", "INTERACTIVE", crossingStrategyBody},
		{"crossing-none", "crossingMinimization.strategy", "NONE", crossingStrategyBody},
		{"placement-simple", "nodePlacement.strategy", "SIMPLE", crossingStrategyBody},
		{"placement-interactive", "nodePlacement.strategy", "INTERACTIVE", crossingStrategyBody},
		{"placement-network-simplex", "nodePlacement.strategy", "NETWORK_SIMPLEX", crossingStrategyBody},
		{"placement-linear-segments", "nodePlacement.strategy", "LINEAR_SEGMENTS", crossingStrategyBody},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			input := []byte(fmt.Sprintf(`{
				"id":"root",
				"layoutOptions":{
					"elk.algorithm":"layered",
					"elk.randomSeed":"7",
					"elk.layered.%s":%q
				},
				%s
			}`, test.option, test.value, test.body))
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := oracle.Differential(ctx, runner, input, elk.LayoutJSON, oracle.DefaultTolerance); err != nil {
				t.Fatalf("%v\ninput: %s", err, input)
			}
		})
	}
}

func TestLayeredSplineOracleParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_LAYERED_STRATEGIES") != "1" {
		t.Skip("set ELK_GO_ORACLE_LAYERED_STRATEGIES=1 to run Layered strategy differentials")
	}
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"SLOPPY", "CONSERVATIVE"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			input := []byte(fmt.Sprintf(`{
				"id":"root",
				"layoutOptions":{
					"elk.algorithm":"layered",
					"elk.edgeRouting":"SPLINES",
					"elk.layered.edgeRouting.splines.mode":%q
				},
				%s
			}`, mode, crossingStrategyBody))
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := oracle.Differential(ctx, runner, input, elk.LayoutJSON, oracle.DefaultTolerance); err != nil {
				t.Fatalf("%v\ninput: %s", err, input)
			}
		})
	}
}

func TestLayeredWrappingOracleParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_LAYERED_STRATEGIES") != "1" {
		t.Skip("set ELK_GO_ORACLE_LAYERED_STRATEGIES=1 to run Layered strategy differentials")
	}
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, strategy, cutting, routing string
	}{
		{"single-msd-orthogonal", "SINGLE_EDGE", "MSD", "ORTHOGONAL"},
		{"single-ard-polyline", "SINGLE_EDGE", "ARD", "POLYLINE"},
		{"single-manual-splines", "SINGLE_EDGE", "MANUAL", "SPLINES"},
		{"multi-msd-orthogonal", "MULTI_EDGE", "MSD", "ORTHOGONAL"},
		{"multi-ard-polyline", "MULTI_EDGE", "ARD", "POLYLINE"},
		{"multi-manual-splines", "MULTI_EDGE", "MANUAL", "SPLINES"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			input := []byte(fmt.Sprintf(`{
				"id":"root",
				"layoutOptions":{
					"elk.algorithm":"layered",
					"elk.randomSeed":"7",
					"elk.aspectRatio":"1.6",
					"elk.edgeRouting":%q,
					"elk.layered.layering.strategy":"LONGEST_PATH",
					"elk.layered.wrapping.strategy":%q,
					"elk.layered.wrapping.cutting.strategy":%q
				},
				%s
			}`, test.routing, test.strategy, test.cutting, wrappingStrategyBody))
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := oracle.Differential(ctx, runner, input, elk.LayoutJSON, oracle.DefaultTolerance); err != nil {
				t.Fatalf("%v\ninput: %s", err, input)
			}
		})
	}
}

func TestLayeredSemiInteractiveOracleParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_LAYERED_STRATEGIES") != "1" {
		t.Skip("set ELK_GO_ORACLE_LAYERED_STRATEGIES=1 to run Layered strategy differentials")
	}
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	input := []byte(`{
		"id":"root",
		"layoutOptions":{
			"elk.algorithm":"layered",
			"elk.randomSeed":"7",
			"elk.layered.crossingMinimization.semiInteractive":true
		},
		"children":[
			{"id":"a","width":30,"height":20,"layoutOptions":{"elk.position":"(0,100)"}},
			{"id":"b","width":30,"height":20,"layoutOptions":{"elk.position":"(0,0)"}},
			{"id":"c","width":30,"height":20,"layoutOptions":{"elk.position":"(100,0)"}},
			{"id":"d","width":30,"height":20,"layoutOptions":{"elk.position":"(100,100)"}}
		],
		"edges":[
			{"id":"ac","sources":["a"],"targets":["c"]},
			{"id":"bd","sources":["b"],"targets":["d"]}
		]
	}`)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := oracle.Differential(ctx, runner, input, elk.LayoutJSON, oracle.DefaultTolerance); err != nil {
		t.Fatalf("%v\ninput: %s", err, input)
	}
}

func TestLayeredExternalPortComponentGroupOracleParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_LAYERED_STRATEGIES") != "1" {
		t.Skip("set ELK_GO_ORACLE_LAYERED_STRATEGIES=1 to run Layered strategy differentials")
	}
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, input string
	}{
		{"sector-none", externalPortSectorBody},
		{"sector-inside-port-side-groups", externalPortSectorModelOrderBody},
		{"force-model-order-right", externalPortForceModelOrderRightBody},
		{"force-model-order-down", externalPortForceModelOrderDownBody},
		{"north-south", externalPortNorthSouthCompactionBody},
		{"north-west", externalPortNorthWestCompactionBody},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			input := []byte(test.input)
			if err := oracle.Differential(ctx, runner, input, elk.LayoutJSON, oracle.DefaultTolerance); err != nil {
				t.Fatalf("%v\ninput: %s", err, input)
			}
		})
	}
}

func TestLayeredExternalPortComponentCompactionOracleParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_LAYERED_STRATEGIES") != "1" {
		t.Skip("set ELK_GO_ORACLE_LAYERED_STRATEGIES=1 to run Layered strategy differentials")
	}
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, input string
	}{
		{"sector-none", externalPortSectorBody},
		{"sector-inside-port-side-groups", externalPortSectorModelOrderBody},
		{"force-model-order-right", externalPortForceModelOrderRightBody},
		{"force-model-order-down", externalPortForceModelOrderDownBody},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			var graph map[string]any
			if err := json.Unmarshal([]byte(test.input), &graph); err != nil {
				t.Fatal(err)
			}
			children := graph["children"].([]any)
			nested := children[0].(map[string]any)
			options := nested["layoutOptions"].(map[string]any)
			options["elk.layered.compaction.connectedComponents"] = true
			input, err := json.Marshal(graph)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := oracle.Differential(
				ctx, runner, input, elk.LayoutJSON, oracle.DefaultTolerance,
			); err != nil {
				t.Fatalf("%v\ninput: %s", err, input)
			}
		})
	}
}

// TestLayeredStrategyRandomOracleParity broadens the source-shaped matrix with
// deterministic generated topology. The environment value is the seed count
// to run for every completed alternate phase.
func TestLayeredStrategyRandomOracleParity(t *testing.T) {
	value := os.Getenv("ELK_GO_ORACLE_LAYERED_STRATEGY_RANDOM")
	if value == "" {
		t.Skip("set ELK_GO_ORACLE_LAYERED_STRATEGY_RANDOM to a positive seed count")
	}
	count, err := strconv.Atoi(value)
	if err != nil || count < 1 {
		t.Fatalf("ELK_GO_ORACLE_LAYERED_STRATEGY_RANDOM must be positive, got %q", value)
	}
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	strategies := []struct {
		name, option, value string
	}{
		{"cycle-depth-first", "elk.layered.cycleBreaking.strategy", "DEPTH_FIRST"},
		{"cycle-interactive", "elk.layered.cycleBreaking.strategy", "INTERACTIVE"},
		{"cycle-model-order", "elk.layered.cycleBreaking.strategy", "MODEL_ORDER"},
		{"layering-longest-path", "elk.layered.layering.strategy", "LONGEST_PATH"},
		{"layering-coffman-graham", "elk.layered.layering.strategy", "COFFMAN_GRAHAM"},
		{"layering-interactive", "elk.layered.layering.strategy", "INTERACTIVE"},
		{"layering-stretch-width", "elk.layered.layering.strategy", "STRETCH_WIDTH"},
		{"layering-min-width", "elk.layered.layering.strategy", "MIN_WIDTH"},
		{"crossing-interactive", "elk.layered.crossingMinimization.strategy", "INTERACTIVE"},
		{"crossing-none", "elk.layered.crossingMinimization.strategy", "NONE"},
		{"placement-simple", "elk.layered.nodePlacement.strategy", "SIMPLE"},
		{"placement-interactive", "elk.layered.nodePlacement.strategy", "INTERACTIVE"},
		{"placement-network-simplex", "elk.layered.nodePlacement.strategy", "NETWORK_SIMPLEX"},
		{"placement-linear-segments", "elk.layered.nodePlacement.strategy", "LINEAR_SEGMENTS"},
		{"routing-splines", "elk.edgeRouting", "SPLINES"},
	}
	for _, strategy := range strategies {
		strategy := strategy
		t.Run(strategy.name, func(t *testing.T) {
			for seed := 0; seed < count; seed++ {
				input, err := (oracle.RandomCase{
					Seed:      uint64(seed),
					NodeCount: uint8(seed*7 + 3),
					EdgeCount: uint8(seed*11 + 5),
					D2Options: true,
				}).JSON()
				if err != nil {
					t.Fatal(err)
				}
				var graph map[string]any
				if err := json.Unmarshal(input, &graph); err != nil {
					t.Fatal(err)
				}
				options := graph["layoutOptions"].(map[string]any)
				options[strategy.option] = strategy.value
				input, err = json.Marshal(graph)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				err = oracle.Differential(ctx, runner, input, elk.LayoutJSON, oracle.DefaultTolerance)
				cancel()
				if err != nil {
					t.Fatalf("seed %d: %v\ninput: %s", seed, err, input)
				}
			}
		})
	}
}

const cycleStrategyBody = `
	"children":[
		{"id":"a","x":0,"y":50,"width":30,"height":20},
		{"id":"b","x":100,"y":0,"width":30,"height":20},
		{"id":"c","x":200,"y":80,"width":30,"height":20}
	],
	"edges":[
		{"id":"e1","sources":["a"],"targets":["b"]},
		{"id":"e2","sources":["b"],"targets":["c"]},
		{"id":"e3","sources":["c"],"targets":["a"]}
	]`

const twoLayerStrategyBody = `
	"children":[
		{"id":"a","x":0,"y":60,"width":30,"height":20},
		{"id":"b","x":0,"y":0,"width":45,"height":30},
		{"id":"c","x":120,"y":25,"width":30,"height":20}
	],
	"edges":[
		{"id":"e1","sources":["a"],"targets":["c"]},
		{"id":"e2","sources":["b"],"targets":["c"]}
	]`

const interactiveLayeringBody = `
	"children":[
		{"id":"a","x":0,"y":60,"width":30,"height":20},
		{"id":"b","x":15,"y":0,"width":30,"height":20},
		{"id":"c","x":100,"y":25,"width":30,"height":20}
	],
	"edges":[
		{"id":"e1","sources":["a"],"targets":["c"]},
		{"id":"e2","sources":["b"],"targets":["c"]}
	]`

const crossingStrategyBody = `
	"children":[
		{"id":"a","x":0,"y":100,"width":30,"height":20},
		{"id":"b","x":0,"y":0,"width":45,"height":30},
		{"id":"c","x":120,"y":100,"width":30,"height":20},
		{"id":"d","x":120,"y":0,"width":35,"height":25}
	],
	"edges":[
		{"id":"e1","sources":["a"],"targets":["d"]},
		{"id":"e2","sources":["b"],"targets":["c"]}
	]`

const wrappingStrategyBody = `
	"children":[
		{"id":"n0","width":30,"height":20},
		{"id":"n1","width":30,"height":20},
		{"id":"n2","width":30,"height":20},
		{"id":"n3","width":30,"height":20},
		{"id":"n4","width":30,"height":20},
		{"id":"n5","width":30,"height":20},
		{"id":"n6","width":30,"height":20},
		{"id":"n7","width":30,"height":20},
		{"id":"n8","width":30,"height":20},
		{"id":"n9","width":30,"height":20},
		{"id":"n10","width":30,"height":20},
		{"id":"n11","width":30,"height":20}
	],
	"edges":[
		{"id":"e0","sources":["n0"],"targets":["n1"]},
		{"id":"e1","sources":["n1"],"targets":["n2"]},
		{"id":"e2","sources":["n2"],"targets":["n3"]},
		{"id":"e3","sources":["n3"],"targets":["n4"]},
		{"id":"e4","sources":["n4"],"targets":["n5"]},
		{"id":"e5","sources":["n5"],"targets":["n6"]},
		{"id":"e6","sources":["n6"],"targets":["n7"]},
		{"id":"e7","sources":["n7"],"targets":["n8"]},
		{"id":"e8","sources":["n8"],"targets":["n9"]},
		{"id":"e9","sources":["n9"],"targets":["n10"]},
		{"id":"e10","sources":["n10"],"targets":["n11"]}
	]`

const externalPortSectorBody = `{
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
			{"id":"pe","x":156,"y":30,"width":4,"height":4,"layoutOptions":{"elk.port.side":"EAST"}},
			{"id":"pw","x":0,"y":80,"width":4,"height":4,"layoutOptions":{"elk.port.side":"WEST"}}
		],
		"children":[
			{"id":"a","width":30,"height":20,"ports":[{"id":"aw","width":2,"height":2}]},
			{"id":"b","width":25,"height":25,"ports":[{"id":"be","width":2,"height":2}]},
			{"id":"d","width":35,"height":15},
			{"id":"u","width":18,"height":18}
		],
		"edges":[
			{"id":"wa","sources":["pw"],"targets":["aw"]},
			{"id":"beedge","sources":["be"],"targets":["pe"]}
		]
	}]
}`

const externalPortSectorModelOrderBody = `{
	"id":"root",
	"layoutOptions":{"elk.algorithm":"fixed"},
	"children":[{
		"id":"g","x":0,"y":0,"width":160,"height":120,
		"layoutOptions":{
			"elk.algorithm":"layered",
			"elk.direction":"RIGHT",
			"elk.portConstraints":"FIXED_SIDE",
			"elk.spacing.componentComponent":"17",
			"elk.layered.considerModelOrder.components":"INSIDE_PORT_SIDE_GROUPS"
		},
		"ports":[
			{"id":"pe","x":156,"y":30,"width":4,"height":4,"layoutOptions":{"elk.port.side":"EAST"}},
			{"id":"pw","x":0,"y":80,"width":4,"height":4,"layoutOptions":{"elk.port.side":"WEST"}}
		],
		"children":[
			{"id":"a","width":30,"height":20,"ports":[{"id":"aw","width":2,"height":2}]},
			{"id":"b","width":25,"height":25,"ports":[{"id":"be","width":2,"height":2}]},
			{"id":"d","width":35,"height":15},
			{"id":"u","width":18,"height":18}
		],
		"edges":[
			{"id":"wa","sources":["pw"],"targets":["aw"]},
			{"id":"beedge","sources":["be"],"targets":["pe"]}
		]
	}]
}`

const externalPortForceModelOrderRightBody = `{
	"id":"root",
	"layoutOptions":{"elk.algorithm":"fixed"},
	"children":[{
		"id":"g","x":0,"y":0,"width":140,"height":100,
		"layoutOptions":{
			"elk.algorithm":"layered",
			"elk.direction":"RIGHT",
			"elk.portConstraints":"FIXED_SIDE",
			"elk.spacing.componentComponent":"13",
			"elk.layered.considerModelOrder.components":"FORCE_MODEL_ORDER"
		},
		"ports":[{"id":"pw","x":0,"y":40,"width":4,"height":4,"layoutOptions":{"elk.port.side":"WEST"}}],
		"children":[
			{"id":"c0","width":30,"height":18},
			{"id":"w","width":24,"height":20,"ports":[{"id":"wp","width":2,"height":2}]},
			{"id":"c1","width":20,"height":16},
			{"id":"c2","width":28,"height":14}
		],
		"edges":[{"id":"we","sources":["pw"],"targets":["wp"]}]
	}]
}`

const externalPortForceModelOrderDownBody = `{
	"id":"root",
	"layoutOptions":{"elk.algorithm":"fixed"},
	"children":[{
		"id":"g","x":0,"y":0,"width":140,"height":100,
		"layoutOptions":{
			"elk.algorithm":"layered",
			"elk.direction":"DOWN",
			"elk.portConstraints":"FIXED_SIDE",
			"elk.spacing.componentComponent":"13",
			"elk.layered.considerModelOrder.components":"FORCE_MODEL_ORDER"
		},
		"ports":[{"id":"pw","x":0,"y":40,"width":4,"height":4,"layoutOptions":{"elk.port.side":"WEST"}}],
		"children":[
			{"id":"c0","width":30,"height":18},
			{"id":"w","width":24,"height":20,"ports":[{"id":"wp","width":2,"height":2}]},
			{"id":"c1","width":20,"height":16},
			{"id":"c2","width":28,"height":14}
		],
		"edges":[{"id":"we","sources":["pw"],"targets":["wp"]}]
	}]
}`

const externalPortNorthSouthCompactionBody = `{
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
			{"id":"pn","x":40,"y":0,"width":4,"height":4,"layoutOptions":{"elk.port.side":"NORTH"}},
			{"id":"ps","x":100,"y":116,"width":4,"height":4,"layoutOptions":{"elk.port.side":"SOUTH"}}
		],
		"children":[
			{"id":"a","width":30,"height":20,"ports":[{"id":"an","width":2,"height":2}]},
			{"id":"b","width":25,"height":25,"ports":[{"id":"bs","width":2,"height":2}]},
			{"id":"d","width":35,"height":15},
			{"id":"u","width":18,"height":18}
		],
		"edges":[
			{"id":"na","sources":["pn"],"targets":["an"]},
			{"id":"bsedge","sources":["bs"],"targets":["ps"]}
		]
	}]
}`

const externalPortNorthWestCompactionBody = `{
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
			{"id":"pn","x":60,"y":0,"width":4,"height":4,"layoutOptions":{"elk.port.side":"NORTH"}},
			{"id":"pw","x":0,"y":70,"width":4,"height":4,"layoutOptions":{"elk.port.side":"WEST"}}
		],
		"children":[
			{"id":"a","width":30,"height":20,"ports":[
				{"id":"an","width":2,"height":2},
				{"id":"aw","width":2,"height":2}
			]},
			{"id":"d","width":35,"height":15},
			{"id":"u","width":18,"height":18}
		],
		"edges":[
			{"id":"na","sources":["pn"],"targets":["an"]},
			{"id":"wa","sources":["pw"],"targets":["aw"]}
		]
	}]
}`
