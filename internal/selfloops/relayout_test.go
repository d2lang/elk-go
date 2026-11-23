// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package selfloops_test

import (
	"encoding/json"
	"fmt"
	"testing"

	elk "github.com/d2lang/elk-go"
)

func TestLayoutsCanConsumeTheirOwnLoopSections(t *testing.T) {
	for _, algorithm := range []string{"mrtree", "force", "stress"} {
		t.Run(algorithm, func(t *testing.T) {
			input := []byte(fmt.Sprintf(`{"id":"r","layoutOptions":{"elk.algorithm":"%s","elkgo.routeSelfLoops":true,"elk.spacing.nodeSelfLoop":50},"children":[{"id":"a","width":53,"height":66},{"id":"b","width":53,"height":66}],"edges":[{"id":"loop","sources":["b"],"targets":["b"],"labels":[{"text":"retry","width":130,"height":18}]}]}`, algorithm))
			var first []byte
			for pass := 0; pass < 3; pass++ {
				output, err := elk.LayoutJSON(input)
				if err != nil {
					t.Fatal(err)
				}
				var g elk.Graph
				if err := json.Unmarshal(output, &g); err != nil {
					t.Fatal(err)
				}
				if len(g.Edges[0].Sections) != 1 {
					t.Fatal("missing self-loop")
				}
				// These two deterministic single-node components have the same
				// footprints even when their loop sections came from the input.
				geometry, err := json.Marshal([]any{g.Width, g.Height, g.Children, g.Edges})
				if err != nil {
					t.Fatal(err)
				}
				if pass == 0 {
					first = geometry
				} else if string(first) != string(geometry) {
					t.Fatalf("pass %d changed loop, label, or packing geometry\nfirst %s\n  got %s", pass, first, geometry)
				}
				input = output
			}
		})
	}
}
