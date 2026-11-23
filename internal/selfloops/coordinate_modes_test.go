// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package selfloops_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	elk "github.com/d2lang/elk-go"
)

func TestNestedSuppliedLoopCoordinateModes(t *testing.T) {
	for _, algorithm := range []string{"force", "stress", "mrtree"} {
		for _, edgeMode := range []string{"ROOT", "PARENT", "CONTAINER"} {
			for _, shapeMode := range []string{"ROOT", "PARENT"} {
				t.Run(algorithm+"/"+edgeMode+"/"+shapeMode, func(t *testing.T) {
					// Declare the edge on the middle node. Its JSON parent,
					// actual container, and the root have distinct origins.
					input := fmt.Sprintf(`{"id":"root","layoutOptions":{"elk.algorithm":%q,"elkgo.routeSelfLoops":true,"elk.json.edgeCoords":%q,"elk.json.shapeCoords":%q},"children":[{"id":"sibling","width":50,"height":40},{"id":"middle","layoutOptions":{"elk.algorithm":%q},"children":[{"id":"inner","layoutOptions":{"elk.algorithm":%q},"children":[{"id":"owner","width":36,"height":24}]}],"edges":[{"id":"loop","sources":["owner"],"targets":["owner"],"labels":[{"text":"again","width":65,"height":16}]}]}]}`, algorithm, edgeMode, shapeMode, algorithm, algorithm)
					out, err := elk.LayoutJSON([]byte(input))
					if err != nil {
						t.Fatal(err)
					}
					var first elk.Graph
					if err := json.Unmarshal(out, &first); err != nil {
						t.Fatal(err)
					}
					edge := first.Children[1].Edges[0]
					if len(edge.Sections) != 1 {
						t.Fatal("missing native self-loop")
					}
					edge.JunctionPoints = []elk.Point{edge.Sections[0].BendPoints[0]}
					supplied, err := json.Marshal(&first)
					if err != nil {
						t.Fatal(err)
					}
					for i := 0; i < 2; i++ {
						out, err = elk.LayoutJSON(supplied)
						if err != nil {
							t.Fatal(err)
						}
						var next elk.Graph
						if err := json.Unmarshal(out, &next); err != nil {
							t.Fatal(err)
						}
						got := next.Children[1].Edges[0]
						if !reflect.DeepEqual(got.Sections, edge.Sections) || !reflect.DeepEqual(got.Labels, edge.Labels) || !reflect.DeepEqual(got.JunctionPoints, edge.JunctionPoints) {
							t.Fatalf("layout pass %d changed the supplied loop, label or junction\nfirst %s\n  got %s", i+2, supplied, out)
						}
						supplied = out
					}
				})
			}
		}
	}
}
