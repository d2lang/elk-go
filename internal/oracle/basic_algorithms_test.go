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
	"os"
	"testing"
	"time"

	elk "github.com/d2lang/elk-go"
)

// TestCandidateBasicAlgorithmsParity pins one source-shaped fixture for each
// utility provider to the frozen elkjs 0.8.2 / ELK 0.8.1 oracle. Random uses a
// nonzero seed because zero intentionally requests a nondeterministic seed.
func TestCandidateBasicAlgorithmsParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_BASIC") != "1" {
		t.Skip("set ELK_GO_ORACLE_BASIC=1 to run utility-provider differentials")
	}
	runner := DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	fixtures := map[string]string{
		"fixed": `{
			"id":"root",
			"layoutOptions":{"elk.algorithm":"fixed","elk.padding":"[top=1,left=2,bottom=3,right=4]"},
			"children":[
				{"id":"a","width":10,"height":10,"layoutOptions":{"elk.position":"(5,6)"}},
				{"id":"b","width":20,"height":20,"layoutOptions":{"elk.position":"(30,40)"},
				 "labels":[{"id":"bl","width":7,"height":8,"layoutOptions":{"elk.position":"(11,12)"}}],
				 "ports":[{"id":"p","width":3,"height":4,"layoutOptions":{"elk.position":"(7,8)"},
				   "labels":[{"id":"pl","width":2,"height":2,"layoutOptions":{"elk.position":"(5,6)"}}]}]}
			],
			"edges":[{"id":"e","sources":["a"],"targets":["b"],
			 "layoutOptions":{"elk.bendPoints":"(10,11; 20,21; 30,31)"},
			 "labels":[{"id":"el","width":5,"height":6,"layoutOptions":{"elk.position":"(7,8)"}}]}]
		}`,
		"box": `{
			"id":"root",
			"layoutOptions":{"elk.algorithm":"box","elk.padding":"[top=3,left=5,bottom=7,right=11]","elk.spacing.nodeNode":"9"},
			"children":[
				{"id":"a","x":80,"y":70,"width":10,"height":20,"layoutOptions":{"elk.priority":"2"}},
				{"id":"b","x":20,"y":10,"width":30,"height":10},
				{"id":"c","x":10,"y":20,"width":20,"height":40,"layoutOptions":{"elk.priority":"1"}},
				{"id":"d","x":5,"y":5,"width":12,"height":12}
			]
		}`,
		"random": `{
			"id":"root",
			"layoutOptions":{"elk.algorithm":"random","elk.randomSeed":"42"},
			"children":[{"id":"a","width":10,"height":20},{"id":"b","width":30,"height":15}],
			"edges":[{"id":"e","sources":["a"],"targets":["b"]}]
		}`,
	}
	for name, fixture := range fixtures {
		name, fixture := name, []byte(fixture)
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := Differential(ctx, runner, fixture, elk.LayoutJSON, DefaultTolerance); err != nil {
				t.Fatalf("%v\ninput: %s", err, fixture)
			}
		})
	}
}

func TestCandidateBoxPackingModesParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_BASIC") != "1" {
		t.Skip("set ELK_GO_ORACLE_BASIC=1 to run utility-provider differentials")
	}
	runner := DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"SIMPLE", "GROUP_DEC", "GROUP_MIXED", "GROUP_INC"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			fixture := []byte(`{
				"id":"root",
				"layoutOptions":{"elk.algorithm":"box","elk.box.packingMode":"` + mode + `","elk.expandNodes":"true","elk.nodeSize.minimum":"(140,120)"},
				"children":[
					{"id":"a","width":80,"height":50,"children":[{"id":"a1","x":0,"y":0,"width":5,"height":5}]},
					{"id":"b","width":10,"height":10},
					{"id":"c","width":12,"height":11},
					{"id":"d","width":15,"height":9},
					{"id":"e","width":20,"height":17},
					{"id":"f","width":9,"height":8}
				]
			}`)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := Differential(ctx, runner, fixture, elk.LayoutJSON, DefaultTolerance); err != nil {
				t.Fatalf("%v\ninput: %s", err, fixture)
			}
		})
	}
}
