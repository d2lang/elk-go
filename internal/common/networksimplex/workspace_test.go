// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package networksimplex

import (
	"reflect"
	"runtime"
	"testing"
)

// drainLayerWorkspaces empties the workspace pool and returns
// how many workspaces it held.
func drainLayerWorkspaces() int {
	count := 0
	for layerWorkspaces.Get() != nil {
		count++
	}
	return count
}

// chainAdapter is a chain 0 -> 1 -> ... -> n-1; outgoing panics
// for node panicAt (none when it is negative).
func chainAdapter(n, panicAt int) Adapter[int, int] {
	nodes := make([]int, n)
	for index := range nodes {
		nodes[index] = index
	}
	return Adapter[int, int]{
		Nodes: nodes,
		Outgoing: func(node int) []int {
			if node == panicAt {
				panic("test: outgoing")
			}
			if node+1 < n {
				return []int{node}
			}
			return nil
		},
		Source:    func(edge int) int { return edge },
		Target:    func(edge int) int { return edge + 1 },
		Weight:    func(int) float64 { return 1 },
		MinLength: func(int) int { return 1 },
		Index:     func(node int) int { return node },
	}
}

// TestPanickingLayeringDropsItsWorkspace: a LayerInto call that
// panics after taking its workspace (here in a caller's callback) must not
// give the workspace back to the pool. A normal call gives it back.
func TestPanickingLayeringDropsItsWorkspace(t *testing.T) {
	drainLayerWorkspaces()
	panicked := func() (recovered any) {
		defer func() { recovered = recover() }()
		LayerInto(chainAdapter(5, 2), Options{}, func(int, int) {})
		return nil
	}()
	if panicked != "test: outgoing" {
		t.Fatalf("recovered %v, want the callback's panic", panicked)
	}
	if pooled := drainLayerWorkspaces(); pooled != 0 {
		t.Fatalf("a panicking layering pooled %d workspace(s)", pooled)
	}

	layers := make([]int, 0, 5)
	LayerInto(chainAdapter(5, -1), Options{}, func(_ int, layer int) { layers = append(layers, layer) })
	for index, layer := range layers {
		if layer != index {
			t.Fatalf("layers = %v, want 0..4", layers)
		}
	}
	if pooled := drainLayerWorkspaces(); pooled != 1 {
		t.Fatalf("a completed layering pooled %d workspaces, want 1", pooled)
	}
}

// TestLayerWorkspaceBytesBoundTheSlot: a reset node slot keeps
// edge lists of at most maxKeptEdgeList entries, a workspace with large
// buffers is dropped, and the byte sizes used are at least the types'.
func TestLayerWorkspaceBytesBoundTheSlot(t *testing.T) {
	var graph Graph
	small, large := graph.AddNode(), graph.AddNode()
	graph.Node(small).IncomingEdges = make([]EdgeID, 0, 32)
	graph.Node(large).OutgoingEdges = make([]EdgeID, 0, 1<<16)
	graph.reset()
	slots := graph.nodes[:2]
	if cap(slots[small].IncomingEdges) != 32 || slots[large].OutgoingEdges != nil {
		t.Fatalf("kept edge lists of %d and %d entries", cap(slots[small].IncomingEdges), cap(slots[large].OutgoingEdges))
	}

	drainLayerWorkspaces()
	workspace := &layerWorkspace{}
	workspace.simplex.cutvalue = make([]float64, 0, maxSlotLayerBytes/8+1)
	layerWorkspaces.Put(workspace, workspace.retainedBytes() <= maxSlotLayerBytes)
	// Only the slot survives collections: an oversized value may go to the
	// sync.Pool under gc, never to the slot.
	runtime.GC()
	runtime.GC()
	if pooled := drainLayerWorkspaces(); pooled != 0 {
		t.Fatalf("a workspace of %d bytes was kept", workspace.retainedBytes())
	}
	for _, size := range []struct {
		value any
		bytes int
	}{
		{Node{}, 120}, {Edge{}, 64}, {subtreeEntry{}, 8}, {walkFrame{}, 24},
	} {
		if actual := int(reflect.TypeOf(size.value).Size()); actual > size.bytes {
			t.Errorf("%T takes %d bytes, the bound counts %d", size.value, actual, size.bytes)
		}
	}
}
