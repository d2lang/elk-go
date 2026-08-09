// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package oracle

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	elk "github.com/d2lang/elk-go"
)

// TestCandidateProviderMatrixParity exercises every algorithm registered by
// the frozen D2 ELK.js 0.8.2 bundle through the public elk-go entry point. The
// shared graph is deliberately positioned, connected, and seeded so it is a
// valid deterministic input for utility, tree, force, compaction, packing,
// and Layered providers alike.
func TestCandidateProviderMatrixParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_ALGORITHMS") != "1" {
		t.Skip("set ELK_GO_ORACLE_ALGORITHMS=1 to run all-provider differentials")
	}
	runner := DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	algorithms := []string{
		"fixed",
		"box",
		"random",
		"layered",
		"stress",
		"mrtree",
		"radial",
		"force",
		"disco",
		"sporeOverlap",
		"sporeCompaction",
		"rectpacking",
	}
	for _, algorithm := range algorithms {
		algorithm := algorithm
		t.Run(algorithm, func(t *testing.T) {
			input := []byte(fmt.Sprintf(`{
				"id":"root",
				"layoutOptions":{
					"elk.algorithm":%q,
					"elk.randomSeed":"7",
					"elk.direction":"DOWN",
					"elk.hierarchyHandling":"INCLUDE_CHILDREN",
					"elk.spacing.edgeNode":"40",
					"elk.layered.nodePlacement.bk.fixedAlignment":"BALANCED",
					"elk.layered.thoroughness":"8",
					"elk.layered.spacing.edgeEdgeBetweenLayers":"50",
					"elk.edgeLabels.inline":"false",
					"elk.layered.considerModelOrder.strategy":"NODES_AND_EDGES",
					"elk.layered.cycleBreaking.strategy":"GREEDY_MODEL_ORDER",
					"elk.nodeSize.constraints":"MINIMUM_SIZE",
					"elk.contentAlignment":"H_CENTER V_CENTER",
					"spacing.nodeNodeBetweenLayers":"70",
					"spacing.edgeNodeBetweenLayers":"40",
					"elk.spacing.nodeSelfLoop":"50"
				},
				"children":[
					{"id":"a","x":0,"y":0,"width":30,"height":20},
					{"id":"b","x":100,"y":10,"width":40,"height":30},
					{"id":"c","x":45,"y":100,"width":20,"height":25}
				],
				"edges":[
					{"id":"e1","sources":["a"],"targets":["b"]},
					{"id":"e2","sources":["b"],"targets":["c"]}
				]
			}`, algorithm))
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := Differential(ctx, runner, input, elk.LayoutJSON, DefaultTolerance); err != nil {
				t.Fatalf("%v\ninput: %s", err, input)
			}
		})
	}
}
