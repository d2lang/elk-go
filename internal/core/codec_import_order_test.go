// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"testing"

	"github.com/d2lang/elk-go/graphjson"
)

// TestUnconstructableErrorsTakePrecedence checks that an unconstructable
// option anywhere in the tree is reported before any other import error, as
// when the whole tree was checked before importing.
func TestUnconstructableErrorsTakePrecedence(t *testing.T) {
	const reject = "Couldn't create new instance of property 'org.eclipse.elk.topdown.sizeApproximator'. Make sure its type is registered with the ElkReflect utility class."
	rejecting := func(id graphjson.ID) *graphjson.Node {
		return &graphjson.Node{ID: id, LayoutOptions: graphjson.LayoutOptions{
			"elk.direction": "DOWN", "topdown.sizeApproximator": "COUNT_CHILDREN"}}
	}
	tests := []struct {
		name  string
		graph *graphjson.Graph
		want  string
	}{
		{"later node rejected after a node without id", &graphjson.Graph{ID: graphjson.StringID("root"),
			Children: []*graphjson.Node{{}, rejecting(graphjson.StringID("b"))}}, reject},
		{"rejected node without id", &graphjson.Graph{ID: graphjson.StringID("root"),
			Children: []*graphjson.Node{rejecting(graphjson.ID{})}}, reject},
		{"nested rejection after a duplicate id", &graphjson.Graph{ID: graphjson.StringID("root"),
			Children: []*graphjson.Node{{ID: graphjson.StringID("a")}, {ID: graphjson.StringID("a")},
				{ID: graphjson.StringID("c"), Children: []*graphjson.Node{nil, rejecting(graphjson.StringID("d"))}}}}, reject},
		{"rejection after a bad port", &graphjson.Graph{ID: graphjson.StringID("root"),
			Children: []*graphjson.Node{{ID: graphjson.StringID("a"), Ports: []*graphjson.Port{{}}}, rejecting(graphjson.StringID("b"))}}, reject},
		{"no rejection keeps the import error", &graphjson.Graph{ID: graphjson.StringID("root"),
			Children: []*graphjson.Node{{ID: graphjson.StringID("a")}, {}}}, "every node must have an id"},
		{"null value is not rejected", &graphjson.Graph{ID: graphjson.StringID("root"),
			Children: []*graphjson.Node{{ID: graphjson.StringID("a"), LayoutOptions: graphjson.LayoutOptions{
				"topdown.sizeApproximator": nil}}, {ID: graphjson.StringID("a")}}}, "duplicate node id: a"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewCodec(NewOptionRegistry()).Import(test.graph)
			if err == nil || err.Error() != test.want {
				t.Fatalf("Import error = %v, want %q", err, test.want)
			}
		})
	}
}
