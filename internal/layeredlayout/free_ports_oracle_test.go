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
	"os"
	"testing"
	"time"

	elk "github.com/d2lang/elk-go"
	"github.com/d2lang/elk-go/internal/oracle"
)

// TestImplicitFreePortOrderOracleParity freezes ELK 0.8.1's observable
// assignment of model-order edges to implicit FREE ports. The two incoming
// ports on n1 deliberately exercise PortListSorter's short-range behavior;
// swapping them changes both n1's placement and the routed edge endpoints.
func TestImplicitFreePortOrderOracleParity(t *testing.T) {
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
			"elk.direction":"LEFT",
			"elk.layered.crossingMinimization.greedySwitch.type":"OFF",
			"elk.layered.crossingMinimization.strategy":"NONE",
			"elk.layered.nodePlacement.bk.edgeStraightening":"NONE",
			"elk.layered.nodePlacement.bk.fixedAlignment":"RIGHTDOWN"
		},
		"children":[
			{"id":"n0","width":46,"height":34},
			{"id":"n1","width":93,"height":19},
			{"id":"n3","width":23,"height":34}
		],
		"edges":[
			{"id":"e0","sources":["n3"],"targets":["n1"]},
			{"id":"e2","sources":["n0"],"targets":["n3"]},
			{"id":"e4","sources":["n0"],"targets":["n1"]}
		]
	}`)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := oracle.Differential(ctx, runner, input, elk.LayoutJSON, oracle.DefaultTolerance); err != nil {
		t.Fatalf("%v\ninput: %s", err, input)
	}
}
