// Copyright (c) 2026 The elk-go Authors.
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

// TestOfficialELKJS0120PostCompactionNonFinite pins the observable failure
// mode retained by official elkjs 0.12.0 when edge-aware scanline constraints form a cycle. The
// longest-path compactor leaves the cycle at infinity, and JSON.stringify
// serializes those non-finite numbers as null.
func TestOfficialELKJS0120PostCompactionNonFinite(t *testing.T) {
	want := []byte(`{"id":"root","layoutOptions":{"elk.algorithm":"layered","elk.layered.compaction.postCompaction.strategy":"LEFT"},"children":[{"id":"a","width":10,"height":10,"x":null,"y":23.686666666666667},{"id":"b","width":10,"height":10,"x":null,"y":22.02},{"id":"c","width":10,"height":10,"x":null,"y":52.019999999999996},{"id":"d","width":10,"height":10,"x":null,"y":52.019999999999996}],"edges":[{"id":"ab","sources":["a"],"targets":["b"],"sections":[{"id":"ab_s0","startPoint":{"x":null,"y":27.02},"endPoint":{"x":null,"y":27.02},"incomingShape":"a","outgoingShape":"b"}],"container":"root"},{"id":"bb","sources":["b"],"targets":["b"],"sections":[{"id":"bb_s0","startPoint":{"x":null,"y":22.02},"endPoint":{"x":null,"y":22.02},"bendPoints":[{"x":null,"y":12.02},{"x":null,"y":12.02}],"incomingShape":"b","outgoingShape":"b"}],"container":"root"},{"id":"ac","sources":["a"],"targets":["c"],"sections":[{"id":"ac_s0","startPoint":{"x":null,"y":30.35333333333333},"endPoint":{"x":null,"y":57.019999999999996},"bendPoints":[{"x":null,"y":30.353333333333335},{"x":null,"y":57.019999999999996}],"incomingShape":"a","outgoingShape":"c"}],"container":"root"},{"id":"cd","sources":["c"],"targets":["d"],"sections":[{"id":"cd_s0","startPoint":{"x":null,"y":57.019999999999996},"endPoint":{"x":null,"y":57.019999999999996},"incomingShape":"c","outgoingShape":"d"}],"container":"root"}],"x":0,"y":0,"width":null,"height":74.02}`)
	got, err := elk.LayoutJSON([]byte(postCompactionNonFiniteFixture))
	if err != nil {
		t.Fatal(err)
	}
	if err := oracle.CompareJSON(want, got, oracle.ExactTolerance); err != nil {
		t.Fatalf("%v\nwant: %s\ngot:  %s", err, want, got)
	}
}

func TestPostCompactionNonFiniteOracleParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_LAYERED_STRATEGIES") != "1" {
		t.Skip("set ELK_GO_ORACLE_LAYERED_STRATEGIES=1 to run Layered strategy differentials")
	}
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := oracle.Differential(ctx, runner, []byte(postCompactionNonFiniteFixture),
		elk.LayoutJSON, oracle.ExactTolerance); err != nil {
		t.Fatal(err)
	}
}

const postCompactionNonFiniteFixture = `{
  "id":"root",
  "layoutOptions":{
    "elk.algorithm":"layered",
    "elk.layered.compaction.postCompaction.strategy":"LEFT"
  },
  "children":[
    {"id":"a","width":10,"height":10},
    {"id":"b","width":10,"height":10},
    {"id":"c","width":10,"height":10},
    {"id":"d","width":10,"height":10}
  ],
  "edges":[
    {"id":"ab","sources":["a"],"targets":["b"]},
    {"id":"bb","sources":["b"],"targets":["b"]},
    {"id":"ac","sources":["a"],"targets":["c"]},
    {"id":"cd","sources":["c"],"targets":["d"]}
  ]
}`
