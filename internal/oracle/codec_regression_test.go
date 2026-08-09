// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0/.
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

// TestCandidateCodecRegressionParity pins JSON member-order alias resolution
// and replacement of pre-layout edge sections to the frozen elkjs oracle.
func TestCandidateCodecRegressionParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_CODEC") != "1" {
		t.Skip("set ELK_GO_ORACLE_CODEC=1 to run codec regression differentials")
	}
	runner := DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	fixtures := map[string]string{
		"legacy-alias-last": `{
			"id":"root",
			"layoutOptions":{
				"elk.algorithm":"layered",
				"org.eclipse.elk.layered.spacing.nodeNodeBetweenLayers":"20",
				"spacing.nodeNodeBetweenLayers":"100"
			},
			"children":[{"id":"a","width":10,"height":10},{"id":"b","width":10,"height":10}],
			"edges":[{"id":"e","sources":["a"],"targets":["b"]}]
		}`,
		"canonical-alias-last": `{
			"id":"root",
			"layoutOptions":{
				"elk.algorithm":"layered",
				"spacing.nodeNodeBetweenLayers":"100",
				"org.eclipse.elk.layered.spacing.nodeNodeBetweenLayers":"20"
			},
			"children":[{"id":"a","width":10,"height":10},{"id":"b","width":10,"height":10}],
			"edges":[{"id":"e","sources":["a"],"targets":["b"]}]
		}`,
		"replace-stale-edge-sections": `{
			"id":"root",
			"layoutOptions":{"elk.algorithm":"layered"},
			"children":[{"id":"a","width":10,"height":10},{"id":"b","width":10,"height":10}],
			"edges":[{"id":"e","sources":["a"],"targets":["b"],"sections":[
				{"id":"old1","startPoint":{"x":1,"y":2},"endPoint":{"x":3,"y":4},"marker":"keep"},
				{"id":"old2","startPoint":{"x":5,"y":6},"endPoint":{"x":7,"y":8},"marker":"drop"}
			]}]
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
