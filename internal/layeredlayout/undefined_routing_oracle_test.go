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

func TestLayeredUndefinedRoutingOracleParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_LAYERED_STRATEGIES") != "1" {
		t.Skip("set ELK_GO_ORACLE_LAYERED_STRATEGIES=1 to run Layered strategy differentials")
	}
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	input := []byte(`{
		"id":"root",
		"layoutOptions":{"elk.algorithm":"layered","elk.edgeRouting":"UNDEFINED"},
		"children":[
			{"id":"a","width":20,"height":20},
			{"id":"b","width":20,"height":20}
		],
		"edges":[{"id":"e","sources":["a"],"targets":["b"]}]
	}`)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := oracle.Differential(ctx, runner, input, elk.LayoutJSON, oracle.ExactTolerance); err != nil {
		t.Fatalf("%v\ninput: %s", err, input)
	}
}
