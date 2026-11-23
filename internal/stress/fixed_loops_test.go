// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package stress_test

import (
	"encoding/json"
	"strings"
	"testing"

	elk "github.com/d2lang/elk-go"
)

func TestFixedLoopClearance(t *testing.T) {
	// Each wall blocks a side of the pinned owner. Node-to-node spacing
	// alone is not an obstacle: ten pixels of free space remain between the
	// bodies, but a normally-sized loop would pass through a wall.
	node := func(id string, x, y int) map[string]any {
		return map[string]any{"id": id, "x": x, "y": y, "width": 20, "height": 20,
			"layoutOptions": map[string]any{"elk.stress.fixed": true}}
	}
	nodes := []map[string]any{node("owner", 50, 50), node("left", 20, 50), node("right", 80, 50),
		node("top", 50, 20), node("bottom", 50, 80)}
	for _, tc := range []struct {
		count     int
		enabled   bool
		wantError bool
	}{{4, true, false}, {5, true, true}, {5, false, false}} {
		input := map[string]any{
			"id": "root", "children": nodes[:tc.count],
			"layoutOptions": map[string]any{"elk.algorithm": "stress", "elk.interactive": true,
				"elk.separateConnectedComponents": false, "elkgo.routeSelfLoops": tc.enabled, "elk.spacing.nodeSelfLoop": 50},
			"edges": []map[string]any{{"id": "loop", "sources": []string{"owner"}, "targets": []string{"owner"}}},
		}
		data, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		_, err = elk.LayoutJSON(data)
		if tc.wantError {
			if err == nil || !strings.Contains(err.Error(), "fixed node") {
				t.Fatalf("with four fixed walls, got %v", err)
			}
		} else if err != nil {
			t.Fatalf("count=%d enabled=%v: %v", tc.count, tc.enabled, err)
		}
	}
}
