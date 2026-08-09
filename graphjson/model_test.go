// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package graphjson

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestGraphJSONPreservesPresenceAndOrder(t *testing.T) {
	input := `{"id":"","x":0,"children":[],"edges":[],"extension":{"a":1}}`
	var graph Graph
	if err := json.Unmarshal([]byte(input), &graph); err != nil {
		t.Fatal(err)
	}
	if graph.X == nil || *graph.X != 0 {
		t.Fatalf("explicit zero x was not preserved: %#v", graph.X)
	}
	if graph.Children == nil || len(graph.Children) != 0 {
		t.Fatalf("explicit empty children were not preserved")
	}
	if graph.Edges == nil || len(graph.Edges) != 0 {
		t.Fatalf("explicit empty edges were not preserved")
	}
	output, err := json.Marshal(&graph)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{`"id":""`, `"x":0`, `"children":[]`, `"edges":[]`, `"extension":{"a":1}`} {
		if !strings.Contains(string(output), fragment) {
			t.Errorf("output %s does not contain %s", output, fragment)
		}
	}
}

func TestGraphJSONUsesJavaScriptNonFiniteNumberSemantics(t *testing.T) {
	graph := Graph{
		ID: StringID("root"),
		X:  Float64(math.Inf(1)),
		Y:  Float64(math.NaN()),
		Edges: []*Edge{{
			ID:             StringID("e"),
			JunctionPoints: []Point{{X: math.Inf(-1), Y: 12}},
		}},
	}
	output, err := json.Marshal(&graph)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{`"x":null`, `"y":null`, `"junctionPoints":[{"x":null,"y":12}]`} {
		if !strings.Contains(string(output), fragment) {
			t.Errorf("output %s does not contain %s", output, fragment)
		}
	}
}

func TestGraphJSONCapturesLayoutOptionMemberOrder(t *testing.T) {
	input := `{
		"id":"root",
		"layoutOptions":{"second":"2","first":"1"},
		"individualSpacings":{"z":"3","a":"4"},
		"children":[{"id":"child","properties":{"legacy.second":"2","legacy.first":"1"}}]
	}`
	var graph Graph
	if err := json.Unmarshal([]byte(input), &graph); err != nil {
		t.Fatal(err)
	}
	if got, want := graph.LayoutOptionKeys(), []string{"second", "first"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("layout option order = %#v, want %#v", got, want)
	}
	if got, want := graph.IndividualSpacingKeys(), []string{"z", "a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("individual spacing order = %#v, want %#v", got, want)
	}
	if got, want := graph.Children[0].LayoutOptionKeys(), []string{"legacy.second", "legacy.first"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy property order = %#v, want %#v", got, want)
	}
	programmatic := &Node{LayoutOptions: LayoutOptions{"second": "2", "first": "1"}}
	if got := programmatic.LayoutOptionKeys(); got != nil {
		t.Fatalf("programmatic option order = %#v, want nil", got)
	}
}

func TestIDAcceptsStringsAndIntegralNumbers(t *testing.T) {
	for _, test := range []struct {
		input, text string
		numeric     bool
	}{{`"a"`, "a", false}, {`12`, "12", true}} {
		var id ID
		if err := json.Unmarshal([]byte(test.input), &id); err != nil {
			t.Fatal(err)
		}
		if id.String() != test.text || id.IsNumber() != test.numeric {
			t.Fatalf("decoded %#v", id)
		}
	}
	var id ID
	if err := json.Unmarshal([]byte(`1.5`), &id); err == nil {
		t.Fatal("fractional id was accepted")
	}
}
