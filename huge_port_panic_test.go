// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package elk

import (
	"fmt"
	"math"
	"testing"
)

// hugePort is a minimized input: NETWORK_SIMPLEX node placement with a port
// of height MaxFloat64. The layers network simplex normalizes span more than
// an int holds; the original's make([]int, highest-lowest+1) panics with
// "makeslice: len out of range".
const hugePort = `{"id":"root","layoutOptions":{"elk.hierarchyHandling":"INCLUDE_CHILDREN",` +
	`"elk.layered.nodePlacement.strategy":"NETWORK_SIMPLEX"},"children":[{"id":"n37","width":21,"height":31},` +
	`{"id":"n49","x":32,"y":118,"width":6,"height":21,"ports":[{"id":"n49.p52","width":4,"height":1.7976931348623157e308}]}]}`

// The panic value of the original elk-go on that input (measured).
const hugePortPanic = "runtime.errorString: runtime error: makeslice: len out of range"

func panicText(call func()) (value string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			value = fmt.Sprintf("%T: %v", recovered, recovered)
		}
	}()
	call()
	return ""
}

// TestHugePortPanicsLikeOriginal checks LayoutJSON and Layout (on a
// Graph built in Go) on the minimized input panic with the original's value,
// repeatedly (pooled network simplex buffers of every size).
func TestHugePortPanicsLikeOriginal(t *testing.T) {
	for round := 0; round < 3; round++ {
		if got := panicText(func() { _, _ = LayoutJSON([]byte(hugePort)) }); got != hugePortPanic {
			t.Fatalf("LayoutJSON round %d: panic %q, want %q", round, got, hugePortPanic)
		}
		graph := &Graph{ID: StringID("root"), LayoutOptions: LayoutOptions{
			"elk.hierarchyHandling": "INCLUDE_CHILDREN", "elk.layered.nodePlacement.strategy": "NETWORK_SIMPLEX"}}
		graph.Children = []*Node{
			{ID: StringID("n37"), Width: Float64(21), Height: Float64(31)},
			{ID: StringID("n49"), X: Float64(32), Y: Float64(118), Width: Float64(6), Height: Float64(21),
				Ports: []*Port{{ID: StringID("n49.p52"), Width: Float64(4), Height: Float64(math.MaxFloat64)}}},
		}
		if got := panicText(func() { _ = Layout(graph) }); got != hugePortPanic {
			t.Fatalf("Layout round %d: panic %q, want %q", round, got, hugePortPanic)
		}
		// an ordinary layout in between keeps the pooled buffers in use
		if _, err := LayoutJSON([]byte(`{"id":"r","layoutOptions":{"elk.layered.nodePlacement.strategy":"NETWORK_SIMPLEX"},` +
			`"children":[{"id":"a","width":10,"height":10},{"id":"b","width":10,"height":10}],` +
			`"edges":[{"id":"e","sources":["a"],"targets":["b"]}]}`)); err != nil {
			t.Fatal(err)
		}
	}
}
