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

// TestCandidateOptionBehaviorParity freezes reachable option behavior that
// used to be silently discarded by the native provider.
func TestCandidateOptionBehaviorParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_OPTIONS") != "1" {
		t.Skip("set ELK_GO_ORACLE_OPTIONS=1 to run option-behavior differentials")
	}
	runner := DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	fixtures := map[string]string{
		"merge-edges": `{
			"id":"root","layoutOptions":{"elk.algorithm":"layered","elk.layered.mergeEdges":true},
			"children":[{"id":"s","width":80,"height":60},{"id":"a","width":40,"height":30},
				{"id":"b","width":40,"height":30},{"id":"t","width":80,"height":60}],
			"edges":[{"id":"sa","sources":["s"],"targets":["a"]},{"id":"sb","sources":["s"],"targets":["b"]},
				{"id":"at","sources":["a"],"targets":["t"]},{"id":"bt","sources":["b"],"targets":["t"]}]
		}`,
		"node-label-placement": `{
			"id":"root",
			"layoutOptions":{"elk.algorithm":"layered"},
			"children":[{
				"id":"a","width":40,"height":40,
				"layoutOptions":{"org.eclipse.elk.nodeLabels.placement":"INSIDE H_CENTER V_CENTER"},
				"labels":[{"id":"l","text":"x","width":14,"height":6}]
			}]
		}`,
		"port-label-placement": `{
			"id":"root",
			"layoutOptions":{"elk.algorithm":"layered"},
			"children":[{
				"id":"a","width":100,"height":100,
				"layoutOptions":{"elk.portConstraints":"FIXED_SIDE"},
				"ports":[{
					"id":"p","width":10,"height":10,
					"layoutOptions":{"elk.port.side":"EAST"},
					"labels":[
						{"id":"l1","text":"one","width":20,"height":10},
						{"id":"l2","text":"two","width":30,"height":10}
					]
				}]
			}]
		}`,
		"port-label-grouping": `{
			"id":"root",
			"layoutOptions":{"elk.algorithm":"layered"},
			"children":[{
				"id":"a","width":100,"height":100,
				"layoutOptions":{
					"elk.portConstraints":"FIXED_SIDE",
					"org.eclipse.elk.portLabels.nextToPortIfPossible":"true",
					"org.eclipse.elk.portLabels.treatAsGroup":"true"
				},
				"ports":[{
					"id":"p","width":10,"height":10,
					"layoutOptions":{"elk.port.side":"EAST"},
					"labels":[
						{"id":"l1","text":"one","width":20,"height":10},
						{"id":"l2","text":"two","width":30,"height":10}
					]
				}]
			}]
		}`,
		"side-specific-port-alignment": `{
			"id":"root",
			"layoutOptions":{"elk.algorithm":"layered"},
			"children":[{
				"id":"a","width":100,"height":100,
				"layoutOptions":{
					"elk.portConstraints":"FIXED_SIDE",
					"org.eclipse.elk.portAlignment.east":"BEGIN"
				},
				"ports":[
					{"id":"p1","width":10,"height":10,"layoutOptions":{"elk.port.side":"EAST"}},
					{"id":"p2","width":10,"height":10,"layoutOptions":{"elk.port.side":"EAST"}},
					{"id":"p3","width":10,"height":10,"layoutOptions":{"elk.port.side":"EAST"}}
				]
			}]
		}`,
		"partitioning": `{
			"id":"root",
			"layoutOptions":{"elk.algorithm":"layered","org.eclipse.elk.partitioning.activate":"true"},
			"children":[
				{"id":"a","width":20,"height":20,"layoutOptions":{"org.eclipse.elk.partitioning.partition":"1"}},
				{"id":"b","width":20,"height":20,"layoutOptions":{"org.eclipse.elk.partitioning.partition":"0"}}
			],
			"edges":[{"id":"e","sources":["a"],"targets":["b"]}]
		}`,
		"inside-self-loop": `{
			"id":"root",
			"layoutOptions":{"elk.algorithm":"layered"},
			"children":[{
				"id":"a","width":100,"height":100,
				"layoutOptions":{
					"org.eclipse.elk.insideSelfLoops.activate":"true",
					"elk.portConstraints":"FIXED_POS"
				},
				"ports":[
					{"id":"p1","x":0,"y":20,"width":10,"height":10,
					 "layoutOptions":{"elk.port.side":"WEST"}},
					{"id":"p2","x":90,"y":70,"width":10,"height":10,
					 "layoutOptions":{"elk.port.side":"EAST"}}
				]
			}],
			"edges":[{
				"id":"e","sources":["p1"],"targets":["p2"],
				"layoutOptions":{"org.eclipse.elk.insideSelfLoops.yo":"true"}
			}]
		}`,
		"comment-box": `{
			"id":"root",
			"layoutOptions":{"elk.algorithm":"layered"},
			"children":[
				{"id":"n","width":100,"height":50},
				{"id":"c","width":40,"height":20,
				 "layoutOptions":{"org.eclipse.elk.commentBox":"true"}}
			],
			"edges":[{"id":"e","sources":["c"],"targets":["n"]}]
		}`,
		"hypernode": `{
			"id":"root",
			"layoutOptions":{"elk.algorithm":"layered"},
			"children":[
				{"id":"a","width":20,"height":20},
				{"id":"h","width":10,"height":10,
				 "layoutOptions":{"org.eclipse.elk.hypernode":"true"},
				 "ports":[{"id":"hi","width":0,"height":0},{"id":"ho","width":0,"height":0}]},
				{"id":"b","width":20,"height":20},
				{"id":"c","width":20,"height":20}
			],
			"edges":[
				{"id":"e1","sources":["a"],"targets":["hi"]},
				{"id":"e2","sources":["ho"],"targets":["b"]},
				{"id":"e3","sources":["ho"],"targets":["c"]}
			]
		}`,
		"comment-spacing": `{
			"id":"root",
			"layoutOptions":{
				"elk.algorithm":"layered",
				"org.eclipse.elk.spacing.commentComment":"7",
				"org.eclipse.elk.spacing.commentNode":"13"
			},
			"children":[
				{"id":"n","width":100,"height":50},
				{"id":"c1","width":20,"height":10,"layoutOptions":{"org.eclipse.elk.commentBox":"true"}},
				{"id":"c2","width":20,"height":10,"layoutOptions":{"org.eclipse.elk.commentBox":"true"}},
				{"id":"c3","width":20,"height":10,"layoutOptions":{"org.eclipse.elk.commentBox":"true"}}
			],
			"edges":[
				{"id":"e1","sources":["c1"],"targets":["n"]},
				{"id":"e2","sources":["c2"],"targets":["n"]},
				{"id":"e3","sources":["c3"],"targets":["n"]}
			]
		}`,
		"interactive-layout-constraints": `{
			"id":"root",
			"layoutOptions":{"elk.algorithm":"layered","org.eclipse.elk.interactiveLayout":"true"},
			"children":[{"id":"a","width":20,"height":20},{"id":"b","width":20,"height":20}],
			"edges":[{"id":"e","sources":["a"],"targets":["b"]}]
		}`,
	}
	for name, fixture := range fixtures {
		name, fixture := name, []byte(fixture)
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := Differential(ctx, runner, fixture, elk.LayoutJSON, ExactTolerance); err != nil {
				t.Fatalf("%v\ninput: %s", err, fixture)
			}
		})
	}
}
