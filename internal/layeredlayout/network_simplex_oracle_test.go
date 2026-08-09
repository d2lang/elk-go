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
	"os"
	"strings"
	"testing"
	"time"

	elk "github.com/d2lang/elk-go"
	"github.com/d2lang/elk-go/internal/oracle"
)

// TestNetworkSimplexPlacementOracleParity freezes source-shaped placement
// behaviors against the elkjs 0.8.2 / ELK 0.8.1 artifact.
func TestNetworkSimplexPlacementOracleParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_LAYERED_STRATEGIES") != "1" {
		t.Skip("set ELK_GO_ORACLE_LAYERED_STRATEGIES=1 to run Layered strategy differentials")
	}
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		input string
	}{
		{"straight-paths", networkSimplexStraightPaths},
		{"disconnected-components", networkSimplexDisconnected},
		{"fixed-ports-and-priority", networkSimplexFixedPorts},
		{"north-south-ports", networkSimplexNorthSouthPorts},
		{"inverted-ports", networkSimplexInvertedPorts},
		{"flexible-port-position", networkSimplexFlexiblePorts},
		{"flexible-node-size", networkSimplexFlexibleSize},
		{"flexible-where-space-permits", strings.Replace(
			networkSimplexFlexibleSize, `"NODE_SIZE"`, `"NODE_SIZE_WHERE_SPACE_PERMITS"`, 1,
		)},
		{"self-loop-port-cache", networkSimplexSelfLoopPortCache},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := oracle.Differential(
				ctx, runner, []byte(test.input), elk.LayoutJSON, oracle.DefaultTolerance,
			); err != nil {
				want, _ := runner.Layout(ctx, []byte(test.input))
				got, _ := elk.LayoutJSON([]byte(test.input))
				t.Fatalf("%v\nwant: %s\ngot:  %s", err, want, got)
			}
		})
	}
}

func TestFrozenELKJS082NetworkSimplexSelfLoopPortCache(t *testing.T) {
	want := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"layered","elk.layered.nodePlacement.strategy":"NETWORK_SIMPLEX"},"children":[{"id":"a","width":34,"height":24,"layoutOptions":{"elk.portConstraints":"FIXED_SIDE"},"ports":[{"id":"ap","x":34,"y":13,"width":4,"height":4,"layoutOptions":{"elk.port.side":"EAST"}}],"x":12,"y":23},{"id":"b","width":42,"height":28,"x":105,"y":127},{"id":"c","width":26,"height":35,"x":113,"y":12},{"id":"d","width":38,"height":18,"x":167,"y":24},{"id":"e","width":29,"height":31,"x":225,"y":17}],"edges":[{"id":"ab","sources":["ap"],"targets":["b"],"labels":[{"id":"abl","text":"ab","width":15,"height":8,"layoutOptions":{"elk.edgeLabels.placement":"CENTER"},"x":70,"y":94}],"sections":[{"id":"ab_s0","startPoint":{"x":50,"y":38},"endPoint":{"x":105,"y":141},"bendPoints":[{"x":60,"y":38},{"x":60,"y":91},{"x":95,"y":91},{"x":95,"y":141}],"incomingShape":"ap","outgoingShape":"b"}],"container":"root"},{"id":"bb","sources":["b"],"targets":["b"],"sections":[{"id":"bb_s0","startPoint":{"x":119,"y":127},"endPoint":{"x":133,"y":127},"bendPoints":[{"x":119,"y":117},{"x":133,"y":117}],"incomingShape":"b","outgoingShape":"b"}],"container":"root"},{"id":"ac","sources":["a"],"targets":["c"],"sections":[{"id":"ac_s0","startPoint":{"x":46,"y":30},"endPoint":{"x":113,"y":30},"incomingShape":"a","outgoingShape":"c"}],"container":"root"},{"id":"bd","sources":["b"],"targets":["d"],"sections":[{"id":"bd_s0","startPoint":{"x":147,"y":141},"endPoint":{"x":167,"y":36},"bendPoints":[{"x":157,"y":141},{"x":157,"y":36}],"incomingShape":"b","outgoingShape":"d"}],"container":"root"},{"id":"cd","sources":["c"],"targets":["d"],"sections":[{"id":"cd_s0","startPoint":{"x":139,"y":30},"endPoint":{"x":167,"y":30},"incomingShape":"c","outgoingShape":"d"}],"container":"root"},{"id":"de","sources":["d"],"targets":["e"],"sections":[{"id":"de_s0","startPoint":{"x":205,"y":33},"endPoint":{"x":225,"y":33},"incomingShape":"d","outgoingShape":"e"}],"container":"root"}],"x":0,"y":0,"width":266,"height":167}`)
	got, err := elk.LayoutJSON([]byte(networkSimplexSelfLoopPortCache))
	if err != nil {
		t.Fatal(err)
	}
	if err := oracle.CompareJSON(want, got, oracle.DefaultTolerance); err != nil {
		t.Fatal(err)
	}
}

func TestFrozenELKJS082NetworkSimplexFlexibleSelfLoopError(t *testing.T) {
	_, err := elk.LayoutJSON([]byte(networkSimplexFlexibleSelfLoopError))
	if err == nil {
		t.Fatal("layout succeeded, want ELK 0.8.1 auxiliary-edge error")
	}
	if !strings.Contains(err.Error(), "NEdge must have a source and target NNode specified") {
		t.Fatalf("layout error = %q, want ELK 0.8.1 auxiliary-edge error", err)
	}
}

func TestNetworkSimplexFlexibleSelfLoopErrorOracleParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_LAYERED_STRATEGIES") != "1" {
		t.Skip("set ELK_GO_ORACLE_LAYERED_STRATEGIES=1 to run Layered strategy differentials")
	}
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, oracleErr := runner.Layout(ctx, []byte(networkSimplexFlexibleSelfLoopError))
	_, candidateErr := elk.LayoutJSON([]byte(networkSimplexFlexibleSelfLoopError))
	for name, err := range map[string]error{"oracle": oracleErr, "candidate": candidateErr} {
		if err == nil {
			t.Errorf("%s layout succeeded, want auxiliary-edge error", name)
		} else if !strings.Contains(err.Error(), "NEdge must have a source and target NNode specified") {
			t.Errorf("%s layout error = %q, want ELK 0.8.1 auxiliary-edge error", name, err)
		}
	}
}

const networkSimplexFlexibleSelfLoopError = `{
  "id":"root",
  "layoutOptions":{
    "elk.algorithm":"layered",
    "elk.layered.nodePlacement.strategy":"NETWORK_SIMPLEX",
    "elk.layered.nodePlacement.networkSimplex.nodeFlexibility.default":"PORT_POSITION"
  },
  "children":[
    {"id":"b","width":30,"height":20},
    {"id":"c","width":30,"height":20}
  ],
  "edges":[
    {"id":"bb","sources":["b"],"targets":["b"]},
    {"id":"bc","sources":["b"],"targets":["c"]}
  ]
}`

const networkSimplexSelfLoopPortCache = `{
  "id":"root",
  "layoutOptions":{
    "elk.algorithm":"layered",
    "elk.layered.nodePlacement.strategy":"NETWORK_SIMPLEX"
  },
  "children":[
    {"id":"a","width":34,"height":24,"layoutOptions":{"elk.portConstraints":"FIXED_SIDE"},
      "ports":[{"id":"ap","x":30,"y":8,"width":4,"height":4,"layoutOptions":{"elk.port.side":"EAST"}}]},
    {"id":"b","width":42,"height":28},
    {"id":"c","width":26,"height":35},
    {"id":"d","width":38,"height":18},
    {"id":"e","width":29,"height":31}
  ],
  "edges":[
    {"id":"ab","sources":["ap"],"targets":["b"],
      "labels":[{"id":"abl","text":"ab","width":15,"height":8,
        "layoutOptions":{"elk.edgeLabels.placement":"CENTER"}}]},
    {"id":"bb","sources":["b"],"targets":["b"]},
    {"id":"ac","sources":["a"],"targets":["c"]},
    {"id":"bd","sources":["b"],"targets":["d"]},
    {"id":"cd","sources":["c"],"targets":["d"]},
    {"id":"de","sources":["d"],"targets":["e"]}
  ]
}`

const networkSimplexStraightPaths = `{
  "id":"root",
  "layoutOptions":{
    "elk.algorithm":"layered",
    "elk.layered.nodePlacement.strategy":"NETWORK_SIMPLEX",
    "elk.layered.nodePlacement.favorStraightEdges":"true",
    "elk.edgeRouting":"ORTHOGONAL",
    "elk.randomSeed":"7"
  },
  "children":[
    {"id":"a","width":34,"height":22},
    {"id":"b","width":42,"height":28},
    {"id":"c","width":30,"height":20},
    {"id":"d","width":45,"height":25},
    {"id":"e","width":36,"height":24},
    {"id":"f","width":38,"height":26}
  ],
  "edges":[
    {"id":"ab","sources":["a"],"targets":["b"]},
    {"id":"bc","sources":["b"],"targets":["c"]},
    {"id":"cd","sources":["c"],"targets":["d"]},
    {"id":"ae","sources":["a"],"targets":["e"]},
    {"id":"ef","sources":["e"],"targets":["f"]},
    {"id":"fd","sources":["f"],"targets":["d"]},
    {"id":"ad","sources":["a"],"targets":["d"]}
  ]
}`

const networkSimplexDisconnected = `{
  "id":"root",
  "layoutOptions":{
    "elk.algorithm":"layered",
    "elk.layered.nodePlacement.strategy":"NETWORK_SIMPLEX",
    "elk.randomSeed":"3"
  },
  "children":[
    {"id":"a","width":30,"height":21},
    {"id":"b","width":40,"height":31},
    {"id":"c","width":25,"height":18},
    {"id":"d","width":35,"height":29},
    {"id":"isolated","width":27,"height":19}
  ],
  "edges":[
    {"id":"ab","sources":["a"],"targets":["b"]},
    {"id":"cd","sources":["c"],"targets":["d"]}
  ]
}`

const networkSimplexFixedPorts = `{
  "id":"root",
  "layoutOptions":{
    "elk.algorithm":"layered",
    "elk.layered.nodePlacement.strategy":"NETWORK_SIMPLEX",
    "elk.edgeRouting":"ORTHOGONAL",
    "elk.randomSeed":"11"
  },
  "children":[
    {"id":"a","width":50,"height":70,"layoutOptions":{"elk.portConstraints":"FIXED_POS"},
      "ports":[
        {"id":"a1","x":50,"y":10,"width":4,"height":4,"layoutOptions":{"elk.port.side":"EAST"}},
        {"id":"a2","x":50,"y":50,"width":4,"height":4,"layoutOptions":{"elk.port.side":"EAST"}}
      ]},
    {"id":"b","width":45,"height":45,"layoutOptions":{"elk.portConstraints":"FIXED_POS"},
      "ports":[
        {"id":"b1","x":-4,"y":8,"width":4,"height":4,"layoutOptions":{"elk.port.side":"WEST"}},
        {"id":"b2","x":-4,"y":31,"width":4,"height":4,"layoutOptions":{"elk.port.side":"WEST"}},
        {"id":"b3","x":45,"y":20,"width":4,"height":4,"layoutOptions":{"elk.port.side":"EAST"}}
      ]},
    {"id":"c","width":40,"height":35,"layoutOptions":{"elk.portConstraints":"FIXED_POS"},
      "ports":[
        {"id":"c1","x":-4,"y":15,"width":4,"height":4,"layoutOptions":{"elk.port.side":"WEST"}}
      ]}
  ],
  "edges":[
    {"id":"e1","sources":["a1"],"targets":["b2"],"layoutOptions":{"elk.layered.priority.straightness":"3"}},
    {"id":"e2","sources":["a2"],"targets":["b1"]},
    {"id":"e3","sources":["b3"],"targets":["c1"]}
  ]
}`

const networkSimplexFlexiblePorts = `{
  "id":"root",
  "layoutOptions":{
    "elk.algorithm":"layered",
    "elk.layered.nodePlacement.strategy":"NETWORK_SIMPLEX",
    "elk.layered.nodePlacement.networkSimplex.nodeFlexibility.default":"PORT_POSITION",
    "elk.spacing.portsSurrounding":"[top=7,left=0,bottom=9,right=0]",
    "elk.edgeRouting":"ORTHOGONAL",
    "elk.randomSeed":"13"
  },
  "children":[
    {"id":"a","width":50,"height":90,"layoutOptions":{"elk.portConstraints":"FIXED_ORDER"},
      "ports":[
        {"id":"a1","width":4,"height":4,"layoutOptions":{"elk.port.side":"EAST"}},
        {"id":"a2","width":4,"height":4,"layoutOptions":{"elk.port.side":"EAST"}}
      ]},
    {"id":"b","width":45,"height":75,"layoutOptions":{"elk.portConstraints":"FIXED_ORDER"},
      "ports":[
        {"id":"b1","width":4,"height":4,"layoutOptions":{"elk.port.side":"WEST"}},
        {"id":"b2","width":4,"height":4,"layoutOptions":{"elk.port.side":"WEST"}}
      ]}
  ],
  "edges":[
    {"id":"e1","sources":["a1"],"targets":["b2"]},
    {"id":"e2","sources":["a2"],"targets":["b1"]}
  ]
}`

const networkSimplexNorthSouthPorts = `{
  "id":"root",
  "layoutOptions":{
    "elk.algorithm":"layered",
    "elk.layered.nodePlacement.strategy":"NETWORK_SIMPLEX",
    "elk.edgeRouting":"ORTHOGONAL",
    "elk.randomSeed":"19"
  },
  "children":[
    {"id":"a","width":60,"height":45,"layoutOptions":{"elk.portConstraints":"FIXED_SIDE"},
      "ports":[
        {"id":"as","width":6,"height":4,"layoutOptions":{"elk.port.side":"SOUTH"}},
        {"id":"ae","width":4,"height":4,"layoutOptions":{"elk.port.side":"EAST"}}
      ]},
    {"id":"b","width":55,"height":50,"layoutOptions":{"elk.portConstraints":"FIXED_SIDE"},
      "ports":[
        {"id":"bn","width":6,"height":4,"layoutOptions":{"elk.port.side":"NORTH"}},
        {"id":"be","width":4,"height":4,"layoutOptions":{"elk.port.side":"EAST"}}
      ]},
    {"id":"c","width":45,"height":35,"layoutOptions":{"elk.portConstraints":"FIXED_SIDE"},
      "ports":[
        {"id":"cw1","width":4,"height":4,"layoutOptions":{"elk.port.side":"WEST"}},
        {"id":"cw2","width":4,"height":4,"layoutOptions":{"elk.port.side":"WEST"}}
      ]}
  ],
  "edges":[
    {"id":"e1","sources":["as"],"targets":["bn"]},
    {"id":"e2","sources":["ae"],"targets":["cw1"]},
    {"id":"e3","sources":["be"],"targets":["cw2"]}
  ]
}`

const networkSimplexInvertedPorts = `{
  "id":"root",
  "layoutOptions":{
    "elk.algorithm":"layered",
    "elk.layered.nodePlacement.strategy":"NETWORK_SIMPLEX",
    "elk.edgeRouting":"ORTHOGONAL",
    "elk.randomSeed":"23"
  },
  "children":[
    {"id":"a","width":50,"height":40,"layoutOptions":{"elk.portConstraints":"FIXED_SIDE"},
      "ports":[
        {"id":"aw","width":4,"height":4,"layoutOptions":{"elk.port.side":"WEST"}},
        {"id":"ae","width":4,"height":4,"layoutOptions":{"elk.port.side":"EAST"}}
      ]},
    {"id":"b","width":55,"height":45,"layoutOptions":{"elk.portConstraints":"FIXED_SIDE"},
      "ports":[
        {"id":"bw","width":4,"height":4,"layoutOptions":{"elk.port.side":"WEST"}},
        {"id":"be","width":4,"height":4,"layoutOptions":{"elk.port.side":"EAST"}}
      ]},
    {"id":"c","width":45,"height":35,"layoutOptions":{"elk.portConstraints":"FIXED_SIDE"},
      "ports":[
        {"id":"cw","width":4,"height":4,"layoutOptions":{"elk.port.side":"WEST"}}
      ]}
  ],
  "edges":[
    {"id":"e1","sources":["aw"],"targets":["be"]},
    {"id":"e2","sources":["ae"],"targets":["cw"]},
    {"id":"e3","sources":["be"],"targets":["cw"]}
  ]
}`

const networkSimplexFlexibleSize = `{
  "id":"root",
  "layoutOptions":{
    "elk.algorithm":"layered",
    "elk.layered.nodePlacement.strategy":"NETWORK_SIMPLEX",
    "elk.layered.nodePlacement.networkSimplex.nodeFlexibility.default":"NODE_SIZE",
    "elk.edgeRouting":"ORTHOGONAL",
    "elk.randomSeed":"17"
  },
  "children":[
    {"id":"a","width":50,"height":30,"layoutOptions":{"elk.portConstraints":"FIXED_ORDER"},
      "ports":[
        {"id":"a1","width":4,"height":4,"layoutOptions":{"elk.port.side":"EAST"}},
        {"id":"a2","width":4,"height":4,"layoutOptions":{"elk.port.side":"EAST"}},
        {"id":"a3","width":4,"height":4,"layoutOptions":{"elk.port.side":"EAST"}}
      ]},
    {"id":"b","width":45,"height":80,"layoutOptions":{"elk.portConstraints":"FIXED_ORDER"},
      "ports":[
        {"id":"b1","width":4,"height":4,"layoutOptions":{"elk.port.side":"WEST"}},
        {"id":"b2","width":4,"height":4,"layoutOptions":{"elk.port.side":"WEST"}},
        {"id":"b3","width":4,"height":4,"layoutOptions":{"elk.port.side":"WEST"}}
      ]}
  ],
  "edges":[
    {"id":"e1","sources":["a1"],"targets":["b3"]},
    {"id":"e2","sources":["a2"],"targets":["b2"]},
    {"id":"e3","sources":["a3"],"targets":["b1"]}
  ]
}`
