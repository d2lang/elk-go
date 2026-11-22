// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package elk

import (
	"encoding/json"
	"testing"
)

// TestEmptyDecodedOrder decodes a graph whose root has
// "layoutOptions": {}, adds two spellings of one option in Go and lays it out.
// The original importer read the member order through LayoutOptionKeys, which
// returns nil for an empty recorded order, so the keys were applied in
// spelling-rank order (legacy ID first, full ID last: the full ID wins). The
// result must equal the layout with only the full ID's value.
func TestEmptyDecodedOrder(t *testing.T) {
	layout := func(rootOptions string, add map[string]any) string {
		input := `{"id":"root","layoutOptions":` + rootOptions + `,"children":[{"id":"a","width":10,"height":10},` +
			`{"id":"b","width":10,"height":10}],"edges":[{"id":"e","sources":["a"],"targets":["b"]}]}`
		var g Graph
		if err := json.Unmarshal([]byte(input), &g); err != nil {
			t.Fatal(err)
		}
		for key, value := range add {
			g.LayoutOptions[key] = value
		}
		if err := Layout(&g); err != nil {
			t.Fatal(err)
		}
		out, err := json.Marshal(&g)
		if err != nil {
			t.Fatal(err)
		}
		return string(out)
	}
	want := layout(`{"org.eclipse.elk.layered.spacing.nodeNodeBetweenLayers":200}`, nil)
	got := layout(`{}`, map[string]any{
		"org.eclipse.elk.layered.spacing.nodeNodeBetweenLayers": 200.0,
		"spacing.nodeNodeBetweenLayers":                         5.0,
	})
	// The layoutOptions echo differs (two keys); compare the children.
	var wantGraph, gotGraph Graph
	if err := json.Unmarshal([]byte(want), &wantGraph); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(got), &gotGraph); err != nil {
		t.Fatal(err)
	}
	if *wantGraph.Children[1].X != *gotGraph.Children[1].X {
		t.Fatalf("b.x = %v, want %v (the full option ID must win over the legacy ID, as with a nil order)",
			*gotGraph.Children[1].X, *wantGraph.Children[1].X)
	}
}
