// Copyright (c) 2026 The elk-go Authors.
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

// TestConnectedComponentsCompactionOracleParity freezes the supported
// no-external-port path through ELK 0.8.1's ComponentsCompactor. The fixture
// deliberately creates irregular component hulls with routed bend points so a
// bounding-box-only packer cannot satisfy it.
func TestConnectedComponentsCompactionOracleParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_LAYERED_STRATEGIES") != "1" {
		t.Skip("set ELK_GO_ORACLE_LAYERED_STRATEGIES=1 to run Layered strategy differentials")
	}
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	fixtures := []struct {
		name  string
		input string
	}{
		{name: "right", input: connectedComponentsCompactionFixture},
		{name: "down", input: strings.Replace(
			connectedComponentsCompactionFixture, `"RIGHT"`, `"DOWN"`, 1)},
	}
	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := oracle.Differential(ctx, runner, []byte(fixture.input),
				elk.LayoutJSON, oracle.DefaultTolerance); err != nil {
				t.Fatalf("%v\ninput: %s", err, fixture.input)
			}
		})
	}
}

const connectedComponentsCompactionFixture = `{
  "id": "root",
  "layoutOptions": {
    "elk.algorithm": "layered",
    "elk.direction": "RIGHT",
    "elk.edgeRouting": "ORTHOGONAL",
    "elk.randomSeed": "17",
    "elk.spacing.componentComponent": "27",
    "elk.layered.compaction.connectedComponents": "true"
  },
  "children": [
    {"id":"a0","width":55,"height":22},
    {"id":"a1","width":18,"height":48},
    {"id":"a2","width":46,"height":18},
    {"id":"a3","width":24,"height":26},
    {"id":"b0","width":20,"height":60},
    {"id":"b1","width":58,"height":20},
    {"id":"b2","width":22,"height":38},
    {"id":"c0","width":64,"height":16},
    {"id":"c1","width":16,"height":64},
    {"id":"c2","width":34,"height":24},
    {"id":"d0","width":37,"height":31},
    {"id":"d1","width":21,"height":53},
    {"id":"e0","width":43,"height":29}
  ],
  "edges": [
    {"id":"a-e0","sources":["a0"],"targets":["a1"]},
    {"id":"a-e1","sources":["a1"],"targets":["a2"]},
    {"id":"a-e2","sources":["a2"],"targets":["a3"]},
    {"id":"b-e0","sources":["b0"],"targets":["b1"]},
    {"id":"b-e1","sources":["b1"],"targets":["b2"]},
    {"id":"c-e0","sources":["c0"],"targets":["c1"]},
    {"id":"c-e1","sources":["c0"],"targets":["c2"]},
    {"id":"d-e0","sources":["d0"],"targets":["d1"]}
  ]
}`
