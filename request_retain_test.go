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
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/jsonorder"
)

// TestEmptyOrdersAsNil checks that the importer's order
// accessors answer what the exported LayoutOptionKeys and
// IndividualSpacingKeys methods (the original importer's reading) answer, nil
// included, for every element kind and every way an order is recorded.
func TestEmptyOrdersAsNil(t *testing.T) {
	same := func(what string, got, want []string) {
		t.Helper()
		if (got == nil) != (want == nil) || !reflect.DeepEqual(append([]string{}, got...), append([]string{}, want...)) {
			t.Fatalf("%s: accessor %#v, exported method %#v", what, got, want)
		}
	}
	members := []string{``, `"layoutOptions":null`, `"layoutOptions":{}`, `"layoutOptions":{"b":1,"a":2}`,
		`"properties":{}`, `"properties":{"x":true}`}
	for _, member := range members {
		object := "{" + member + "}"
		if member == "" {
			object = "{}"
		}
		var node graphjson.Node
		var port graphjson.Port
		var label graphjson.Label
		var edge graphjson.Edge
		var section graphjson.EdgeSection
		for _, target := range []any{&node, &port, &label, &edge, &section} {
			if err := json.Unmarshal([]byte(object), target); err != nil {
				t.Fatal(err)
			}
		}
		same("node "+object, jsonorder.LayoutOptionKeys(&node), node.LayoutOptionKeys())
		same("port "+object, jsonorder.LayoutOptionKeys(&port), port.LayoutOptionKeys())
		same("label "+object, jsonorder.LayoutOptionKeys(&label), label.LayoutOptionKeys())
		same("edge "+object, jsonorder.LayoutOptionKeys(&edge), edge.LayoutOptionKeys())
		same("section "+object, jsonorder.LayoutOptionKeys(&section), section.LayoutOptionKeys())
	}
	for _, member := range []string{`{}`, `{"individualSpacings":null}`, `{"individualSpacings":{}}`, `{"individualSpacings":{"a":1}}`} {
		var node graphjson.Node
		if err := json.Unmarshal([]byte(member), &node); err != nil {
			t.Fatal(err)
		}
		same("individual spacings "+member, jsonorder.IndividualSpacingKeys(&node), node.IndividualSpacingKeys())
	}
	for _, keys := range [][]string{nil, {}, {"a"}, {"a", "a"}} {
		node, port, label, edge, section := &graphjson.Node{}, &graphjson.Port{}, &graphjson.Label{}, &graphjson.Edge{}, &graphjson.EdgeSection{}
		node.SetLayoutOptionKeys(keys)
		node.SetIndividualSpacingKeys(keys)
		port.SetLayoutOptionKeys(keys)
		label.SetLayoutOptionKeys(keys)
		edge.SetLayoutOptionKeys(keys)
		section.SetLayoutOptionKeys(keys)
		what := fmt.Sprintf("set %#v", keys)
		same(what, jsonorder.LayoutOptionKeys(node), node.LayoutOptionKeys())
		same(what, jsonorder.IndividualSpacingKeys(node), node.IndividualSpacingKeys())
		same(what, jsonorder.LayoutOptionKeys(port), port.LayoutOptionKeys())
		same(what, jsonorder.LayoutOptionKeys(label), label.LayoutOptionKeys())
		same(what, jsonorder.LayoutOptionKeys(edge), edge.LayoutOptionKeys())
		same(what, jsonorder.LayoutOptionKeys(section), section.LayoutOptionKeys())
	}
}

// orderGraph is a graph with every element kind that has
// layout options (root, child, port, port label, node label, edge, edge
// label, preset section) and individual spacings, decoded from JSON whose
// option objects are all `{}` when decoded is set, else built in Go (no
// recorded orders); then the same two spellings of one option are added to
// every options map in Go.
func orderGraph(t *testing.T, decoded bool) *Graph {
	t.Helper()
	const input = `{"id":"root","layoutOptions":{},"individualSpacings":{},"children":[` +
		`{"id":"a","width":10,"height":10,"layoutOptions":{},"individualSpacings":{},` +
		`"ports":[{"id":"p","width":1,"height":1,"layoutOptions":{},"labels":[{"text":"pl","layoutOptions":{}}]}],` +
		`"labels":[{"text":"al","width":3,"height":3,"layoutOptions":{}}]},` +
		`{"id":"b","width":10,"height":10}],` +
		`"edges":[{"id":"e","sources":["p"],"targets":["b"],"layoutOptions":{},"labels":[{"text":"el","layoutOptions":{}}],` +
		`"sections":[{"id":"s","startPoint":{"x":0,"y":0},"endPoint":{"x":5,"y":5},"layoutOptions":{}}]}]}`
	var g Graph
	if err := json.Unmarshal([]byte(input), &g); err != nil {
		t.Fatal(err)
	}
	if !decoded {
		// The same graph without recorded orders: re-decode the elements'
		// shapes but build every options map in Go.
		g.SetLayoutOptionKeys(nil)
		g.SetIndividualSpacingKeys(nil)
		a := g.Children[0]
		a.SetLayoutOptionKeys(nil)
		a.SetIndividualSpacingKeys(nil)
		a.Ports[0].SetLayoutOptionKeys(nil)
		a.Ports[0].Labels[0].SetLayoutOptionKeys(nil)
		a.Labels[0].SetLayoutOptionKeys(nil)
		e := g.Edges[0]
		e.SetLayoutOptionKeys(nil)
		e.Labels[0].SetLayoutOptionKeys(nil)
		e.Sections[0].SetLayoutOptionKeys(nil)
	}
	// Two spellings of one option only: with spellings of several options
	// the nil-order comparator is not transitive, and the result depends on
	// map iteration order (in the original too).
	add := func(options LayoutOptions) {
		options["org.eclipse.elk.layered.spacing.nodeNodeBetweenLayers"] = 200.0
		options["spacing.nodeNodeBetweenLayers"] = 5.0
	}
	a, e := g.Children[0], g.Edges[0]
	for _, options := range []LayoutOptions{g.LayoutOptions, g.IndividualSpacings, a.LayoutOptions, a.IndividualSpacings,
		a.Ports[0].LayoutOptions, a.Ports[0].Labels[0].LayoutOptions, a.Labels[0].LayoutOptions,
		e.LayoutOptions, e.Labels[0].LayoutOptions, e.Sections[0].LayoutOptions} {
		if options == nil {
			t.Fatal("an options map was not decoded")
		}
		add(options)
	}
	return &g
}

// TestEmptyOrderImports imports a decoded graph whose option
// objects were `{}` and got two spellings of an option in Go, and the same
// graph built without recorded orders: every element must get the same
// properties (the full option ID applied last, as for no order), and the
// layouts must be equal.
func TestEmptyOrderImports(t *testing.T) {
	options, _ := sharedRegistries()
	entries := func(model *graph.Model) []any {
		var out []any
		for i := range model.Nodes {
			out = append(out, model.Nodes[i].Properties.Entries())
		}
		for i := range model.Ports {
			out = append(out, model.Ports[i].Properties.Entries())
		}
		for i := range model.Labels {
			out = append(out, model.Labels[i].Properties.Entries())
		}
		for i := range model.Edges {
			out = append(out, model.Edges[i].Properties.Entries())
		}
		for i := range model.Sections {
			out = append(out, model.Sections[i].Properties.Entries())
		}
		return out
	}
	want, err := core.NewCodec(options).Import(orderGraph(t, false))
	if err != nil {
		t.Fatal(err)
	}
	got, err := core.NewCodec(options).Import(orderGraph(t, true))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(entries(got), entries(want)) {
		t.Fatalf("properties of the decoded graph differ from those without orders:\n got %v\nwant %v", entries(got), entries(want))
	}
	layout := func(decoded bool) string {
		g := orderGraph(t, decoded)
		if err := Layout(g); err != nil {
			return "error: " + err.Error()
		}
		out, err := json.Marshal(g)
		if err != nil {
			t.Fatal(err)
		}
		return string(out)
	}
	if got, want := layout(true), layout(false); got != want {
		t.Fatalf("layout of the decoded graph differs:\n got %s\nwant %s", got, want)
	}
}

// liveHeap is the live heap after collections.
func liveHeap() uint64 {
	var stats runtime.MemStats
	for i := 0; i < 3; i++ {
		runtime.GC()
	}
	runtime.ReadMemStats(&stats)
	return stats.HeapAlloc
}

// wideJSON is a graph of nodes nodes chained by edges, the
// first with ports ports.
func wideJSON(nodes, ports int) []byte {
	var b strings.Builder
	b.WriteString(`{"id":"root","layoutOptions":{"elk.direction":"DOWN"},"children":[`)
	for i := 0; i < nodes; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"id":"n%d","width":10,"height":10`, i)
		if i == 0 && ports > 0 {
			b.WriteString(`,"ports":[`)
			for p := 0; p < ports; p++ {
				if p > 0 {
					b.WriteByte(',')
				}
				fmt.Fprintf(&b, `{"id":"p%d","width":1,"height":1}`, p)
			}
			b.WriteString(`]`)
		}
		b.WriteString(`}`)
	}
	b.WriteString(`],"edges":[`)
	for i := 1; i < nodes; i++ {
		if i > 1 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"id":"e%d","sources":["n%d"],"targets":["n%d"]}`, i, i-1, i)
	}
	b.WriteString(`]}`)
	return []byte(b.String())
}

// TestLargeLayoutNotRetained lays out a large graph and then
// small ones: the storage kept for reuse must stay within its bound (2 MB
// for the codec) instead of growing with the large layout (an earlier
// version kept 14 MB more here, for good).
func TestLargeLayoutNotRetained(t *testing.T) {
	if runtime.Compiler == "tinygo" {
		t.Skip("heap statistics differ under TinyGo")
	}
	small := wideJSON(3, 0)
	if _, err := LayoutJSON(small); err != nil {
		t.Fatal(err)
	}
	before := liveHeap()
	if _, err := LayoutJSON(wideJSON(500, 120000)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := LayoutJSON(small); err != nil {
			t.Fatal(err)
		}
	}
	if after := liveHeap(); after > before+3<<20 {
		t.Fatalf("live heap %d -> %d bytes after one large layout", before, after)
	}
}

// TestCallerTextsNotRetained lays out graphs (through Layout and
// a Request) whose option keys, option values and IDs are parts of one large
// string, and checks that nothing kept for reuse refers to it afterwards
// (an earlier version's parse cache, remembered orders and their memos
// kept such texts, and with them the whole string).
func TestCallerTextsNotRetained(t *testing.T) {
	if runtime.Compiler == "tinygo" {
		t.Skip("heap statistics differ under TinyGo")
	}
	const size = 64 << 20
	run := func() {
		var builder strings.Builder
		builder.Grow(size)
		builder.WriteString("elk.directionDOWNelk.nodeSize.minimum(10, 20)rootab")
		for builder.Len() < size {
			builder.WriteString("..........")
		}
		big := builder.String()
		key, value := big[0:13], big[13:17]
		minimumKey, minimum := big[17:37], big[37:45]
		root, a, b := big[45:49], big[49:50], big[50:51]
		keys := []string{key, minimumKey}

		// Layout of a Graph built in Go, with recorded orders.
		g := &Graph{ID: StringID(root), LayoutOptions: LayoutOptions{key: value}}
		g.SetLayoutOptionKeys(keys[:1])
		for _, id := range []string{a, b} {
			child := &Node{ID: StringID(id), Width: Float64(10), Height: Float64(10), LayoutOptions: LayoutOptions{minimumKey: minimum}}
			child.SetLayoutOptionKeys(keys[1:])
			g.Children = append(g.Children, child)
		}
		g.Edges = []*Edge{{ID: StringID(root + a), Sources: []ID{StringID(a)}, Targets: []ID{StringID(b)}}}
		if err := Layout(g); err != nil {
			t.Fatal(err)
		}

		// A Request with options prepared from the same texts.
		request := NewRequest(3, 0, 0, 1)
		rootOptions := request.PrepareOptions(keys[:1], []any{value})
		childOptions := request.PrepareOptions(keys[1:], []any{minimum})
		ref := request.Root(root, rootOptions)
		request.Node(ref, a, 0, 0, 10, 10, childOptions)
		request.Node(ref, b, 0, 0, 10, 10, childOptions)
		request.Edge(ref, root+a, []string{a}, []string{b}, nil)
		if err := request.Layout(); err != nil {
			t.Fatal(err)
		}
		request.Release()
		shared := PrepareOptions(keys, []any{value, minimum})
		_ = shared
	}
	before := liveHeap()
	run()
	if after := liveHeap(); after > before+size/2 {
		t.Fatalf("live heap %d -> %d bytes: the caller's string is still referenced", before, after)
	}
}

// TestPanickedRequestDropped lays out random requests (the
// generator of TestRequestMatchesImport) until the engine panics in
// one, releasing it from a deferred call while the panic unwinds, as a
// caller with `defer request.Release()` does: the request must be dropped,
// not offered for reuse, and later requests must lay out as before.
func TestPanickedRequestDropped(t *testing.T) {
	keys := make([]string, 0, len(revOptionPool))
	for key := range revOptionPool {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	found := false
	for seed := int64(0); seed < 3000 && !found; seed++ {
		gen := &revGen{rng: rand.New(rand.NewSource(seed)), keys: keys}
		input := gen.graph()
		b := buildRequest(t, decodeLikeLayoutJSON(t, input))
		if b.req.Err() != nil {
			b.req.Release()
			continue
		}
		request := b.req
		var recovered any
		func() {
			defer func() {
				if recovered = recover(); recovered != nil {
					request.Release()
				}
			}()
			defer func() {
				if r := recover(); r != nil {
					// Release while the panic unwinds, then let it go on.
					request.Release()
					panic(r)
				}
			}()
			_ = request.Layout()
		}()
		if recovered == nil {
			request.Release()
			continue
		}
		found = true
		releasedRequests.Lock()
		kept := releasedRequests.slot == request
		releasedRequests.Unlock()
		if kept || request.codec != nil || request.model != nil {
			t.Fatalf("seed %d: a request whose layout panicked was offered for reuse", seed)
		}
		// The JSON path panics the same way on the same graph.
		again := func() (value any) {
			defer func() { value = recover() }()
			_, _ = LayoutJSON(input)
			return nil
		}()
		if fmt.Sprint(again) != fmt.Sprint(recovered) {
			t.Fatalf("seed %d: LayoutJSON ends with %v, the request panicked with %v", seed, again, recovered)
		}
	}
	if !found {
		t.Skip("no random request panicked")
	}
	small := wideJSON(3, 0)
	first, err := LayoutJSON(small)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LayoutJSON(small)
	if err != nil || string(first) != string(second) {
		t.Fatalf("layouts after a panicked request differ: %v", err)
	}
}
