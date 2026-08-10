// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package oracle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

const (
	considerModelOrderOption  = "elk.layered.considerModelOrder.strategy"
	cycleBreakingOption       = "elk.layered.cycleBreaking.strategy"
	forceNodeModelOrderOption = "elk.layered.crossingMinimization.forceNodeModelOrder"
)

// PrepareD2ELK012Input migrates the former D2 0.8.2 model-order profile to the
// compatibility profile used with elkjs 0.12.0. It preserves the checked-in
// source fixtures while ensuring the oracle and candidate receive identical,
// valid input:
//
//   - GREEDY replaces GREEDY_MODEL_ORDER for cycle breaking at every level;
//   - forceNodeModelOrder is removed at every level; and
//   - nested graphs use the explicit NONE model-order strategy. The root keeps
//     NODES_AND_EDGES, matching D2's 0.12 adapter.
//
// The migration is deliberately limited to existing option keys. Generic ELK
// fixtures that do not opt into D2's profile retain normal ELK defaults.
func PrepareD2ELK012Input(input []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	var graph map[string]any
	if err := decoder.Decode(&graph); err != nil {
		return nil, fmt.Errorf("decode D2 ELK input: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode D2 ELK input: multiple JSON values")
		}
		return nil, fmt.Errorf("decode D2 ELK input: %w", err)
	}
	normalizeD2ELK012Node(graph, 0)
	output, err := json.Marshal(graph)
	if err != nil {
		return nil, fmt.Errorf("encode D2 ELK 0.12 input: %w", err)
	}
	return output, nil
}

func normalizeD2ELK012Node(node map[string]any, depth int) {
	if options, ok := node["layoutOptions"].(map[string]any); ok {
		delete(options, forceNodeModelOrderOption)
		if value, exists := options[cycleBreakingOption]; exists && value == "GREEDY_MODEL_ORDER" {
			options[cycleBreakingOption] = "GREEDY"
		}
		if depth > 0 {
			if _, exists := options[considerModelOrderOption]; exists {
				options[considerModelOrderOption] = "NONE"
			}
		}
	}
	children, _ := node["children"].([]any)
	for _, child := range children {
		if child, ok := child.(map[string]any); ok {
			normalizeD2ELK012Node(child, depth+1)
		}
	}
}
