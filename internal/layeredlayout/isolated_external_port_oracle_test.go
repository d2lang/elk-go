// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package layeredlayout_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	elk "github.com/d2lang/elk-go"
	"github.com/d2lang/elk-go/internal/oracle"
)

// TestLayeredIsolatedFixedSidePortFrozen pins ELK 0.8.1's parent-node
// resizing behavior. An isolated fixed-side port does not become an external
// port dummy, so ElkGraphLayoutTransferrer scales its side position when the
// compound node changes size. Connected-component compaction must not affect
// this transfer step.
func TestLayeredIsolatedFixedSidePortFrozen(t *testing.T) {
	for _, compaction := range []bool{false, true} {
		t.Run(fmt.Sprintf("compaction-%t", compaction), func(t *testing.T) {
			var graph elk.Graph
			if err := json.Unmarshal(isolatedFixedSidePortFixture(compaction), &graph); err != nil {
				t.Fatal(err)
			}
			if err := elk.Layout(&graph); err != nil {
				t.Fatal(err)
			}
			if len(graph.Children) != 1 {
				t.Fatalf("root children = %d, want 1", len(graph.Children))
			}
			nested := graph.Children[0]
			assertNear(t, "graph width", elk.Number(nested.Width), 42)
			assertNear(t, "graph height", elk.Number(nested.Height), 42)
			if len(nested.Ports) != 1 {
				t.Fatalf("ports = %d, want 1", len(nested.Ports))
			}
			assertNear(t, "west port x", elk.Number(nested.Ports[0].X), 0)
			assertNear(t, "west port y", elk.Number(nested.Ports[0].Y), 24.5)
			if len(nested.Children) != 1 {
				t.Fatalf("nested children = %d, want 1", len(nested.Children))
			}
			assertNear(t, "child x", elk.Number(nested.Children[0].X), 12)
			assertNear(t, "child y", elk.Number(nested.Children[0].Y), 12)
		})
	}
}

// TestLayeredIsolatedFixedSidePortOracleParity compares both public
// connected-component compaction values with the frozen elkjs 0.8.2 / ELK
// 0.8.1 artifact.
func TestLayeredIsolatedFixedSidePortOracleParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_LAYERED_STRATEGIES") != "1" {
		t.Skip("set ELK_GO_ORACLE_LAYERED_STRATEGIES=1 to run Layered strategy differentials")
	}
	runner := oracle.DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	for _, compaction := range []bool{false, true} {
		t.Run(fmt.Sprintf("compaction-%t", compaction), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			fixture := isolatedFixedSidePortFixture(compaction)
			if err := oracle.Differential(ctx, runner, fixture, elk.LayoutJSON,
				oracle.DefaultTolerance); err != nil {
				t.Fatalf("%v\ninput: %s", err, fixture)
			}
		})
	}
}

func isolatedFixedSidePortFixture(compaction bool) []byte {
	return []byte(fmt.Sprintf(`{
		"id":"root",
		"layoutOptions":{"elk.algorithm":"fixed"},
		"children":[{
			"id":"g","x":0,"y":0,"width":160,"height":120,
			"layoutOptions":{
				"elk.algorithm":"layered",
				"elk.direction":"RIGHT",
				"elk.portConstraints":"FIXED_SIDE",
				"elk.spacing.componentComponent":"17",
				"elk.layered.compaction.connectedComponents":%t
			},
			"ports":[{
				"id":"pw","x":0,"y":70,"width":4,"height":4,
				"layoutOptions":{"elk.port.side":"WEST"}
			}],
			"children":[{"id":"u","width":18,"height":18}]
		}]
	}`, compaction))
}
