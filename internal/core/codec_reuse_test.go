// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/graph"
)

// reuseTestGraph is a graph with nodes, ports, labels, nested children,
// edges and options, scaled by size.
func reuseTestGraph(size int) *graphjson.Graph {
	root := &graphjson.Graph{ID: graphjson.StringID("root"), LayoutOptions: graphjson.LayoutOptions{
		"elk.direction": "RIGHT", "elk.padding": "[top=1,left=2,bottom=3,right=4]", "elk.spacing.nodeNode": 12.0,
	}}
	for i := 0; i < size; i++ {
		id := "n" + strconv.Itoa(i)
		node := &graphjson.Node{
			ID: graphjson.StringID(id), Width: graphjson.Float64(float64(10 + i)), Height: graphjson.Float64(20),
			Labels:        []*graphjson.Label{{Text: graphjson.String("label " + id), Width: graphjson.Float64(5)}},
			LayoutOptions: graphjson.LayoutOptions{"elk.nodeSize.minimum": "(3, 4)", "elk.contentAlignment": "H_CENTER V_CENTER"},
		}
		if i%3 == 0 {
			node.Ports = []*graphjson.Port{{ID: graphjson.StringID(id + ".p"), LayoutOptions: graphjson.LayoutOptions{"elk.port.side": "EAST"}}}
		}
		if i%4 == 1 {
			node.Children = []*graphjson.Node{{ID: graphjson.StringID(id + ".c"), Width: graphjson.Float64(3), Height: graphjson.Float64(3)}}
		}
		root.Children = append(root.Children, node)
		if i > 0 {
			root.Edges = append(root.Edges, &graphjson.Edge{
				ID:      graphjson.StringID("e" + strconv.Itoa(i)),
				Sources: []graphjson.ID{graphjson.StringID("n" + strconv.Itoa(i-1))},
				Targets: []graphjson.ID{graphjson.StringID(id)},
				Labels:  []*graphjson.Label{{Text: graphjson.String("edge")}},
			})
		}
	}
	return root
}

// TestReleasedCodecImportsAsNew imports graphs of growing and shrinking
// sizes through one codec that is released and acquired again in between
// (the pool may or may not hand the same one back), and compares every model
// and every transferred graph with those of a new codec.
func TestReleasedCodecImportsAsNew(t *testing.T) {
	// Start without a released codec: one left by another test may hold
	// recycled storage (empty but not nil), which reflect.DeepEqual tells
	// apart from a new codec's nil although every value is the same.
	for releasedCodecs.Get() != nil {
	}
	registry := NewOptionRegistry()
	for _, size := range []int{40, 3, 0, 25, 60, 1, 60} {
		fresh := reuseTestGraph(size)
		want, err := NewJSONCodec(registry).Import(fresh)
		if err != nil {
			t.Fatal(err)
		}
		reused := AcquireJSONCodec(registry)
		input := reuseTestGraph(size)
		got, err := reused.Import(input)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("size %d: model of a reused codec differs", size)
		}
		freshCodec := NewJSONCodec(registry)
		freshModel, _ := freshCodec.Import(fresh)
		if err := freshCodec.TransferLayout(freshModel); err != nil {
			t.Fatal(err)
		}
		if err := reused.TransferLayout(got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(input, fresh) {
			t.Fatalf("size %d: transfer of a reused codec differs", size)
		}
		ReleaseJSONCodec(reused, got)
	}
}

// TestRememberedParsesMatchParseRaw checks that the codec's remembered parse
// results equal fresh parses, for the options marked pureParse and others,
// on first and repeated use, valid and invalid texts alike.
func TestRememberedParsesMatchParseRaw(t *testing.T) {
	registry := NewOptionRegistry()
	marked := 0
	for _, option := range registry.Options() {
		if option.pureParse {
			marked++
		}
	}
	if marked != 3 {
		t.Fatalf("%d options marked pureParse, want 3 (padding, port anchor, node size minimum)", marked)
	}
	codec := NewCodec(registry)
	texts := []any{"[top=50,left=50,bottom=50,right=50]", "(3, 4)", "(1.5,2)", "", "null", "[top=1]", "x", "(a, b)",
		"[left=-2.5e3,right=1e400]", " ( 7 , 8 ) ", 12.0, true, nil}
	for round := 0; round < 3; round++ {
		for _, option := range registry.Options() {
			for _, raw := range texts {
				gotValue, gotValid := codec.parse(option, raw)
				wantValue, wantValid := option.parseRaw(raw)
				if gotValid != wantValid || gotValid && !sameParsed(gotValue, wantValue) {
					t.Fatalf("%s %#v: parse = %#v %v, parseRaw = %#v %v", option.ID, raw, gotValue, gotValid, wantValue, wantValid)
				}
			}
		}
	}
}

// TestKnownOrdersFollowRegistrations checks that member orders remembered
// across imports are resolved again after a registration changes what a
// key resolves to, and that equal orders in other slices are recognised.
func TestKnownOrdersFollowRegistrations(t *testing.T) {
	registry := NewOptionRegistry()
	codec := NewJSONCodec(registry)
	importDirection := func() string {
		keys := []string{"direction", "spacing.nodeNode"} // a new slice each time
		root := &graphjson.Graph{ID: graphjson.StringID("root"),
			LayoutOptions: graphjson.LayoutOptions{"direction": "LEFT", "spacing.nodeNode": 3.0}}
		root.SetLayoutOptionKeys(keys)
		model, err := codec.Import(root)
		if err != nil {
			t.Fatal(err)
		}
		return graph.Get(&model.Nodes[0].Properties, Direction)
	}
	if got := importDirection(); got != "LEFT" {
		t.Fatalf("direction %q, want LEFT", got)
	}
	if got := importDirection(); got != "LEFT" {
		t.Fatalf("direction %q on the remembered order, want LEFT", got)
	}
	// "direction" becomes ambiguous: BySuffix resolves it to nil now.
	if err := registry.Register(OptionData{ID: "test.other.direction", Kind: StringOption}); err != nil {
		t.Fatal(err)
	}
	if registry.BySuffix("direction") != nil {
		t.Fatal("direction should be ambiguous")
	}
	if got := importDirection(); got != "UNDEFINED" {
		t.Fatalf("direction %q after the registration, want the default UNDEFINED", got)
	}
}
