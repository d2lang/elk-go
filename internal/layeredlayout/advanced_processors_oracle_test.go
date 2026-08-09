// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package layeredlayout_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	elk "github.com/d2lang/elk-go"
	"github.com/d2lang/elk-go/internal/oracle"
)

// TestAdvancedLayeredProcessorOracleParity pins the optional post-layering
// and post-routing processors to ELK 0.8.1, including every public strategy.
func TestAdvancedLayeredProcessorOracleParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_LAYERED_STRATEGIES") != "1" {
		t.Skip("set ELK_GO_ORACLE_LAYERED_STRATEGIES=1 to run Layered strategy differentials")
	}
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		options string
		body    string
	}{
		{
			"high-degree-bounded-trees",
			`"elk.layered.highDegreeNodes.treatment":"true",
			 "elk.layered.highDegreeNodes.threshold":"4",
			 "elk.layered.highDegreeNodes.treeHeight":"2"`,
			highDegreeProcessorBody,
		},
	}
	for _, strategy := range []string{
		"NIKOLOV", "NIKOLOV_PIXEL", "NIKOLOV_IMPROVED", "NIKOLOV_IMPROVED_PIXEL",
		"DUMMYNODE_PERCENTAGE", "NODECOUNT_PERCENTAGE", "NO_BOUNDARY",
	} {
		tests = append(tests, struct {
			name    string
			options string
			body    string
		}{
			name: "node-promotion-" + strategy,
			options: fmt.Sprintf(`"elk.layered.layering.strategy":"LONGEST_PATH",
				"elk.layered.layering.nodePromotion.strategy":%q,
				"elk.layered.layering.nodePromotion.maxIterations":"50"`, strategy),
			body: nodePromotionProcessorBody,
		})
	}
	for _, constraints := range []string{"SCANLINE", "QUADRATIC"} {
		for _, strategy := range []string{
			"LEFT", "RIGHT", "LEFT_RIGHT_CONSTRAINT_LOCKING",
			"LEFT_RIGHT_CONNECTION_LOCKING", "EDGE_LENGTH",
		} {
			tests = append(tests, struct {
				name    string
				options string
				body    string
			}{
				name: "post-compaction-" + constraints + "-" + strategy,
				options: fmt.Sprintf(`"elk.edgeRouting":"ORTHOGONAL",
					"elk.layered.compaction.postCompaction.constraints":%q,
					"elk.layered.compaction.postCompaction.strategy":%q`, constraints, strategy),
				body: postCompactionProcessorBody,
			})
		}
	}
	for _, strategy := range []string{"LEFT", "EDGE_LENGTH"} {
		tests = append(tests, struct {
			name    string
			options string
			body    string
		}{
			name: "post-compaction-SPLINES-" + strategy,
			options: fmt.Sprintf(`"elk.edgeRouting":"SPLINES",
				"elk.layered.compaction.postCompaction.constraints":"SCANLINE",
				"elk.layered.compaction.postCompaction.strategy":%q`, strategy),
			body: postCompactionProcessorBody,
		})
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			input := []byte(fmt.Sprintf(`{
				"id":"root",
				"layoutOptions":{
					"elk.algorithm":"layered",
					"elk.randomSeed":"7",
					%s
				},
				%s
			}`, test.options, test.body))
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := oracle.Differential(ctx, runner, input, elk.LayoutJSON, oracle.DefaultTolerance); err != nil {
				t.Fatalf("%v\ninput: %s", err, input)
			}
		})
	}
}

func TestAdvancedLayeredProcessorRandomOracleParity(t *testing.T) {
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
	profiles := []struct {
		name    string
		options map[string]any
	}{
		{"high-degree", map[string]any{
			"elk.layered.highDegreeNodes.treatment":  "true",
			"elk.layered.highDegreeNodes.threshold":  "4",
			"elk.layered.highDegreeNodes.treeHeight": "3",
		}},
		{"node-promotion", map[string]any{
			"elk.layered.layering.nodePromotion.strategy": "NO_BOUNDARY",
		}},
		{"post-left-scanline", map[string]any{
			"elk.layered.compaction.postCompaction.strategy":    "LEFT",
			"elk.layered.compaction.postCompaction.constraints": "SCANLINE",
		}},
		{"post-right-quadratic", map[string]any{
			"elk.layered.compaction.postCompaction.strategy":    "RIGHT",
			"elk.layered.compaction.postCompaction.constraints": "QUADRATIC",
		}},
		{"post-edge-length", map[string]any{
			"elk.layered.compaction.postCompaction.strategy": "EDGE_LENGTH",
		}},
	}
	for _, profile := range profiles {
		profile := profile
		t.Run(profile.name, func(t *testing.T) {
			for seed := 0; seed < count; seed++ {
				input, err := (oracle.RandomCase{
					Seed: uint64(seed), NodeCount: uint8(seed*7 + 3), EdgeCount: uint8(seed*11 + 5),
					D2Options: true,
				}).JSON()
				if err != nil {
					t.Fatal(err)
				}
				var graph map[string]any
				if err := json.Unmarshal(input, &graph); err != nil {
					t.Fatal(err)
				}
				normalizeAdvancedRandomDAG(graph)
				options := graph["layoutOptions"].(map[string]any)
				for key, optionValue := range profile.options {
					options[key] = optionValue
				}
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

func normalizeAdvancedRandomDAG(graph map[string]any) {
	edges, _ := graph["edges"].([]any)
	result := make([]any, 0, len(edges))
	seen := make(map[string]struct{})
	for _, raw := range edges {
		edge, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		sources, sourceOK := edge["sources"].([]any)
		targets, targetOK := edge["targets"].([]any)
		if !sourceOK || !targetOK || len(sources) == 0 || len(targets) == 0 {
			continue
		}
		source, sourceOK := sources[0].(string)
		target, targetOK := targets[0].(string)
		if !sourceOK || !targetOK || source == target {
			continue
		}
		sourceIndex, sourceErr := strconv.Atoi(strings.TrimPrefix(source, "n"))
		targetIndex, targetErr := strconv.Atoi(strings.TrimPrefix(target, "n"))
		if sourceErr != nil || targetErr != nil {
			continue
		}
		if sourceIndex > targetIndex {
			source, target = target, source
		}
		key := source + "->" + target
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		edge["sources"] = []any{source}
		edge["targets"] = []any{target}
		result = append(result, edge)
	}
	graph["edges"] = result
}

const highDegreeProcessorBody = `
	"children":[
		{"id":"in1","width":24,"height":18},{"id":"in2","width":26,"height":20},
		{"id":"hub1","width":40,"height":35},{"id":"hub2","width":42,"height":32},
		{"id":"a1","width":20,"height":16},{"id":"a2","width":22,"height":18},
		{"id":"a3","width":18,"height":14},{"id":"a4","width":25,"height":17},
		{"id":"tail","width":30,"height":20}
	],
	"edges":[
		{"id":"e1","sources":["in1"],"targets":["hub1"]},
		{"id":"e2","sources":["in2"],"targets":["hub1"]},
		{"id":"e3","sources":["hub1"],"targets":["a1"]},
		{"id":"e4","sources":["hub1"],"targets":["a2"]},
		{"id":"e5","sources":["hub1"],"targets":["hub2"]},
		{"id":"e6","sources":["hub2"],"targets":["a3"]},
		{"id":"e7","sources":["hub2"],"targets":["a4"]},
		{"id":"e8","sources":["hub2"],"targets":["tail"]}
	]`

const nodePromotionProcessorBody = `
	"children":[
		{"id":"a","width":30,"height":22},{"id":"b","width":45,"height":20},
		{"id":"c","width":24,"height":35},{"id":"d","width":40,"height":18},
		{"id":"e","width":26,"height":28},{"id":"f","width":32,"height":24},
		{"id":"g","width":38,"height":30},{"id":"h","width":20,"height":20}
	],
	"edges":[
		{"id":"e1","sources":["a"],"targets":["d"]},
		{"id":"e2","sources":["a"],"targets":["e"]},
		{"id":"e3","sources":["b"],"targets":["e"]},
		{"id":"e4","sources":["b"],"targets":["f"]},
		{"id":"e5","sources":["c"],"targets":["f"]},
		{"id":"e6","sources":["d"],"targets":["g"]},
		{"id":"e7","sources":["e"],"targets":["g"]},
		{"id":"e8","sources":["f"],"targets":["g"]},
		{"id":"e9","sources":["a"],"targets":["h"]},
		{"id":"e10","sources":["h"],"targets":["g"]}
	]`

const postCompactionProcessorBody = `
	"children":[
		{"id":"a","width":35,"height":24},{"id":"b","width":25,"height":36},
		{"id":"c","width":42,"height":20},{"id":"d","width":30,"height":30},
		{"id":"e","width":38,"height":22},{"id":"f","width":28,"height":34}
	],
	"edges":[
		{"id":"e1","sources":["a"],"targets":["d"]},
		{"id":"e2","sources":["a"],"targets":["e"]},
		{"id":"e3","sources":["b"],"targets":["d"]},
		{"id":"e4","sources":["b"],"targets":["f"]},
		{"id":"e5","sources":["c"],"targets":["e"]},
		{"id":"e6","sources":["c"],"targets":["f"]},
		{"id":"e7","sources":["d"],"targets":["f"]}
	]`
