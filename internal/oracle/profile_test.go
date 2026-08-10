// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package oracle

import (
	"encoding/json"
	"testing"
)

func TestPrepareD2ELK012Input(t *testing.T) {
	input := []byte(`{
		"id":"root",
		"layoutOptions":{
			"elk.layered.considerModelOrder.strategy":"NODES_AND_EDGES",
			"elk.layered.cycleBreaking.strategy":"GREEDY_MODEL_ORDER",
			"elk.layered.crossingMinimization.forceNodeModelOrder":true
		},
		"children":[{
			"id":"container",
			"layoutOptions":{
				"elk.layered.considerModelOrder.strategy":"NODES_AND_EDGES",
				"elk.layered.cycleBreaking.strategy":"GREEDY_MODEL_ORDER",
				"elk.layered.crossingMinimization.forceNodeModelOrder":true
			}
		}]
	}`)
	output, err := PrepareD2ELK012Input(input)
	if err != nil {
		t.Fatal(err)
	}
	var graph map[string]any
	if err := json.Unmarshal(output, &graph); err != nil {
		t.Fatal(err)
	}
	root := graph["layoutOptions"].(map[string]any)
	if got := root[considerModelOrderOption]; got != "NODES_AND_EDGES" {
		t.Fatalf("root model order = %v, want NODES_AND_EDGES", got)
	}
	if got := root[cycleBreakingOption]; got != "GREEDY_MODEL_ORDER" {
		t.Fatalf("root cycle breaking = %v, want GREEDY_MODEL_ORDER", got)
	}
	if _, found := root[forceNodeModelOrderOption]; found {
		t.Fatal("root forceNodeModelOrder was not removed")
	}
	child := graph["children"].([]any)[0].(map[string]any)["layoutOptions"].(map[string]any)
	if got := child[considerModelOrderOption]; got != "NONE" {
		t.Fatalf("nested model order = %v, want NONE", got)
	}
	if got := child[cycleBreakingOption]; got != "GREEDY" {
		t.Fatalf("nested cycle breaking = %v, want GREEDY", got)
	}
	if _, found := child[forceNodeModelOrderOption]; found {
		t.Fatal("nested forceNodeModelOrder was not removed")
	}
}

func TestD2GeneratedProfilesUseRootGreedyModelOrder(t *testing.T) {
	randomInput, err := (RandomCase{Seed: 1, NodeCount: 3, EdgeCount: 4, D2Options: true}).JSON()
	if err != nil {
		t.Fatal(err)
	}
	testCases := map[string][]byte{
		"provider matrix": providerMatrixInput("layered"),
		"random":          randomInput,
	}
	for name, input := range testCases {
		t.Run(name, func(t *testing.T) {
			var graph map[string]any
			if err := json.Unmarshal(input, &graph); err != nil {
				t.Fatal(err)
			}
			root := graph["layoutOptions"].(map[string]any)
			if got := root[considerModelOrderOption]; got != "NODES_AND_EDGES" {
				t.Fatalf("root model order = %v, want NODES_AND_EDGES", got)
			}
			if got := root[cycleBreakingOption]; got != "GREEDY_MODEL_ORDER" {
				t.Fatalf("root cycle breaking = %v, want GREEDY_MODEL_ORDER", got)
			}
			if _, found := root[forceNodeModelOrderOption]; found {
				t.Fatal("root forceNodeModelOrder must be absent")
			}
		})
	}
}
