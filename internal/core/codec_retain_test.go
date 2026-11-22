// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"encoding/json"
	"reflect"
	"runtime"
	"strconv"
	"testing"

	"github.com/d2lang/elk-go/internal/reuse"

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

// TestRetainedByteBounds checks that the per-element byte bounds
// retainedBytes uses are at least the elements' sizes. codec.go cites it by
// this name.
func TestRetainedByteBounds(t *testing.T) {
	for _, c := range []struct {
		value any
		bound int
	}{
		{graph.Node{}, nodeBytes}, {graph.Port{}, portBytes}, {graph.Label{}, labelBytes}, {graph.Edge{}, edgeBytes},
		{graph.PropertyMap{}, propertyMapBytes}, {PreparedOptions{}, preparedOptionsBytes},
		{preparedEntry{}, preparedEntryBytes}, {preparedKey{}, preparedKeyBytes}, {graph.ShapeID{}, shapeIDBytes},
		{graph.LabelID(0), labelIDBytes}, {graph.NodeID(0), nodeIDBytes}, {geometry.Point{}, pointBytes},
		{"", stringHeaderBytes}, {(*graphjson.Node)(nil), pointerBytes},
	} {
		if size := int(reflect.TypeOf(c.value).Size()); size > c.bound {
			t.Errorf("%T takes %d bytes, bound %d", c.value, size, c.bound)
		}
	}
	// A map binding holds a key and a value with their share of the tables.
	if size := int(reflect.TypeOf("").Size() + reflect.TypeOf(graph.NodeID(0)).Size()); 2*size > textBindingBytes {
		t.Errorf("text binding bound %d too small for %d-byte entries", textBindingBytes, size)
	}
}

// wideGraph is a graph with n nodes, each with ports ports,
// chained by edges, every element with an option.
func wideGraph(n, ports int) *graphjson.Graph {
	root := &graphjson.Graph{ID: graphjson.StringID("root"), LayoutOptions: graphjson.LayoutOptions{"elk.direction": "RIGHT"}}
	for i := 0; i < n; i++ {
		id := "n" + strconv.Itoa(i)
		node := &graphjson.Node{ID: graphjson.StringID(id), Width: graphjson.Float64(10), Height: graphjson.Float64(10),
			LayoutOptions: graphjson.LayoutOptions{"elk.nodeSize.minimum": "(3, " + strconv.Itoa(i) + ")"},
			Labels:        []*graphjson.Label{{Text: graphjson.String(id)}}}
		for p := 0; p < ports; p++ {
			node.Ports = append(node.Ports, &graphjson.Port{ID: graphjson.StringID(id + "." + strconv.Itoa(p))})
		}
		root.Children = append(root.Children, node)
		if i > 0 {
			root.Edges = append(root.Edges, &graphjson.Edge{ID: graphjson.StringID("e" + id),
				Sources: []graphjson.ID{graphjson.StringID("n" + strconv.Itoa(i-1))}, Targets: []graphjson.ID{graphjson.StringID(id)}})
		}
	}
	return root
}

// TestReleasedCodecBounded imports graphs of growing size
// through acquired codecs and checks what the slot keeps after each release:
// at most maxRetainedBytes, ID maps of at most maxRetainedIDs bindings, and
// a codec that imports as a new one does.
func TestReleasedCodecBounded(t *testing.T) {
	registry := NewOptionRegistry()
	drain := func() {
		for releasedCodecs.Get() != nil {
		}
	}
	for _, size := range []struct{ nodes, ports int }{{10, 0}, {3000, 0}, {20, 0}, {200, 100}, {5000, 2}, {10, 1}} {
		drain()
		codec := AcquireCodec(registry)
		model, err := codec.Import(wideGraph(size.nodes, size.ports))
		if err != nil {
			t.Fatal(err)
		}
		ReleaseCodec(codec, model)
		// Only the slot survives collections: an oversized codec may go to the
		// sync.Pool under gc, never to the slot.
		runtime.GC()
		runtime.GC()
		kept := releasedCodecs.Get()
		if kept == nil {
			// Not in the slot: it must have been too large. Under TinyGo it is
			// emptied and dropped; under gc it went to the sync.Pool, which
			// collections empty.
			if !reuse.OversizedToPool && !reflect.DeepEqual(*codec, Codec{}) {
				t.Fatalf("%v: a dropped codec was not emptied", size)
			}
			if reuse.OversizedToPool && codec.retainedBytes() <= maxRetainedBytes {
				t.Fatalf("%v: a codec of %d bytes was not kept in the slot", size, codec.retainedBytes())
			}
			continue
		}
		if bytes := kept.retainedBytes(); bytes > maxRetainedBytes {
			t.Fatalf("%v: the slot keeps %d bytes", size, bytes)
		}
		if kept.nodeIDs.textSize > maxRetainedIDs || kept.portIDs.textSize > maxRetainedIDs {
			t.Fatalf("%v: the slot keeps ID maps sized for %d and %d bindings", size, kept.nodeIDs.textSize, kept.portIDs.textSize)
		}
		if size.nodes+size.nodes*size.ports > maxRetainedIDs && (kept.nodeIDs.text != nil && size.nodes > maxRetainedIDs ||
			kept.portIDs.text != nil && size.nodes*size.ports > maxRetainedIDs) {
			t.Fatalf("%v: an oversized ID map was kept", size)
		}
		// The kept codec imports as a new one.
		fresh, err := NewCodec(registry).Import(wideGraph(7, 2))
		if err != nil {
			t.Fatal(err)
		}
		reused := AcquireCodec(registry)
		got, err := reused.Import(wideGraph(7, 2))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(modelValues(got), modelValues(fresh)) {
			t.Fatalf("%v: a reused codec imports differently", size)
		}
		ReleaseCodec(reused, got)
	}
}

// modelValues is what an import put into the model, comparable.
func modelValues(model *graph.Model) []any {
	var values []any
	for i := range model.Nodes {
		node := &model.Nodes[i]
		values = append(values, node.Identifier, node.Shape, node.Children, node.Ports, node.Labels, node.Properties.Entries())
	}
	for i := range model.Ports {
		values = append(values, model.Ports[i].Identifier, model.Ports[i].Properties.Entries())
	}
	for i := range model.Labels {
		values = append(values, model.Labels[i].Text, model.Labels[i].Properties.Entries())
	}
	for i := range model.Edges {
		values = append(values, model.Edges[i].Identifier, model.Edges[i].Sources, model.Edges[i].Targets)
	}
	return values
}

// TestCachesOwnTheirTexts checks that the caches a codec keeps
// across imports (remembered orders, their memos, parse results) hold
// texts of their own, at most maxOwnedText long, and not other values.
func TestCachesOwnTheirTexts(t *testing.T) {
	registry := NewOptionRegistry()
	codec := NewCodec(registry)
	long := string(make([]byte, 4*maxOwnedText))
	keys := []string{"elk.direction", "elk.nodeSize.minimum", "elk.padding", "elk.spacing.nodeNode", "elk.nodeLabels.placement"}
	for _, values := range [][]any{
		{"DOWN", "(1, 2)", "[top=1,left=2,bottom=3,right=4]", 12.0, "[H_CENTER, V_TOP]"},
		{"RIGHT", "(1, 2)" + long, "[top=1]", json.Number("7"), []any{"x"}},
		{"DOWN", "(3, 4)", "[top=1,left=2,bottom=3,right=4]", true, "[H_CENTER, V_TOP]"},
	} {
		want := NewCodec(registry).PrepareOptions(keys, values)
		got := codec.PrepareOptions(keys, values)
		if !reflect.DeepEqual(got.entries, want.entries) || !reflect.DeepEqual(got.nodeErr, want.nodeErr) {
			t.Fatalf("preparation of %v differs from a new codec's", values)
		}
	}
	for _, known := range codec.knownOrders {
		for _, memo := range known.memo {
			if !memo.set {
				continue
			}
			switch raw := memo.raw.(type) {
			case string:
				if len(raw) > maxOwnedText {
					t.Fatalf("the memo keeps a text of %d bytes", len(raw))
				}
			case json.Number, bool, float64:
			default:
				t.Fatalf("the memo keeps a %T", raw)
			}
		}
	}
	for key := range codec.parsed {
		if len(key.text) > maxOwnedText {
			t.Fatalf("the parse cache keeps a text of %d bytes", len(key.text))
		}
	}
	if _, ok := ownedRaw([]any{"x"}); ok {
		t.Fatal("ownedRaw keeps a slice")
	}
	if owned, ok := ownedRaw("abc"); !ok || owned != "abc" {
		t.Fatalf("ownedRaw(abc) = %v, %v", owned, ok)
	}
}
