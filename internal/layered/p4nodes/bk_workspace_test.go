// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/d2lang/elk-go/internal/layered"
)

// drainBKWorkspaces empties the BK workspace pool and returns
// how many workspaces it held.
func drainBKWorkspaces() int {
	count := 0
	for bkWorkspaces.Get() != nil {
		count++
	}
	return count
}

// TestPanickingPlacementDropsItsWorkspace: a Brandes-Koepf
// placement that panics after taking its workspace (here on a layer listing a
// node the arena does not have) must not give the workspace back to the
// pool: its storage may be half-updated. A normal placement gives it back.
func TestPanickingPlacementDropsItsWorkspace(t *testing.T) {
	drainBKWorkspaces()
	arena, graphID, _ := buildPlacementDiamond()
	layers := arena.Graph(graphID).Layers
	last := layers[len(layers)-1]
	arena.Layer(last).Nodes = append(arena.Layer(last).Nodes, layered.LNodeID(len(arena.Nodes)+100))
	panicked := func() (recovered any) {
		defer func() { recovered = recover() }()
		PlaceBrandesKoepf(arena, graphID)
		return nil
	}()
	if panicked == nil {
		t.Fatal("the malformed arena did not make the placement panic")
	}
	if pooled := drainBKWorkspaces(); pooled != 0 {
		t.Fatalf("a panicking placement pooled %d workspace(s)", pooled)
	}

	arena, graphID, _ = buildPlacementDiamond()
	PlaceBrandesKoepf(arena, graphID)
	if pooled := drainBKWorkspaces(); pooled != 1 {
		t.Fatalf("a completed placement pooled %d workspaces, want 1", pooled)
	}
}

// TestBKWorkspaceBytesBoundTheSlot: a workspace with a large
// buffer not sized by the node count (here the conflict marks, indexed by
// arena edge ID) is dropped; the byte sizes used are at least the types'.
func TestBKWorkspaceBytesBoundTheSlot(t *testing.T) {
	drainBKWorkspaces()
	workspace := acquireBKWorkspace()
	workspace.marked.mark(layered.LEdgeID(1 << 21)) // 2 MB of marks for a graph of no nodes
	workspace.marked.clear()
	releaseBKWorkspace(workspace)
	// Only the slot survives collections: an oversized value may go to the
	// sync.Pool under gc, never to the slot.
	runtime.GC()
	runtime.GC()
	if pooled := drainBKWorkspaces(); pooled != 0 {
		t.Fatal("a workspace holding 2 MB of edge marks was kept")
	}
	for _, size := range []struct {
		value any
		bytes int
	}{
		{optNode{}, 8}, {optFloat{}, 16}, {Neighbor{}, 8}, {classNode{}, 56}, {classEdge{}, 16},
		{postprocessable{}, 16}, {bkPost{}, 16}, {bkNeighbor{}, 8}, {bkEdgeEnd{}, 12},
		{[]int(nil), 24}, {(*BKAlignedLayout)(nil), 8}, {layered.NodeType(0), 1},
	} {
		if actual := int(reflect.TypeOf(size.value).Size()); actual > size.bytes {
			t.Errorf("%T takes %d bytes, the bound counts %d", size.value, actual, size.bytes)
		}
	}
}
