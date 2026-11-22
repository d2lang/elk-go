// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
)

// drainArenaPool empties the arena pool and returns how many
// arenas it held.
func drainArenaPool() int {
	count := 0
	for arenaPool.Get() != nil {
		count++
	}
	return count
}

// TestLargeArenaIsDropped: an arena above maxSlotArenaElements
// never goes to the slot: under TinyGo it is dropped (TinyGo's sync.Pool never
// frees what it holds), under gc it may
// go to the sync.Pool, which collections empty; a
// bounded one is kept in the slot.
func TestLargeArenaIsDropped(t *testing.T) {
	drainArenaPool()
	for _, nodes := range []int{maxSlotArenaElements / 2, 2 * maxSlotArenaElements} {
		arena := AcquireLGraphArena()
		graphID := arena.AddGraph()
		for index := 0; index < nodes; index++ {
			arena.AddNode(graphID)
		}
		ReleaseLGraphArena(arena)
		want := 1
		if cap(arena.Graphs)+cap(arena.Nodes) > maxSlotArenaElements {
			want = 0
		}
		// Only the slot survives collections: an oversized value may go to the
		// sync.Pool under gc, never to the slot.
		runtime.GC()
		runtime.GC()
		if pooled := drainArenaPool(); pooled != want {
			t.Fatalf("arena of %d nodes: %d kept, want %d", nodes, pooled, want)
		}
	}
}

// recursiveCompoundInsideTraversalKey is compoundInsideTraversalKey as it
// was before its recursion over nesting levels became a loop.
func recursiveCompoundInsideTraversalKey(arena *LGraphArena, graphID LGraphID, portID LPortID) ([]int, int) {
	port := arena.Port(portID)
	if port.Node == nil {
		return nil, math.MaxInt
	}
	nodeID := *port.Node
	path := []int{graphNodeIndex(arena, graphID, nodeID)}
	node := arena.Node(nodeID)
	if node.NestedGraph != nil {
		if dummyID, found := basegraph.TryGet(&port.Properties, PortDummy); found {
			if nextPort, _, connected := compoundDummyInsideEndpoint(arena, dummyID); connected {
				nestedPath, terminalPort := recursiveCompoundInsideTraversalKey(arena, *node.NestedGraph, nextPort)
				return append(path, nestedPath...), terminalPort
			}
		}
	}
	return path, nodePortIndex(arena, nodeID, portID)
}

// TestTraversalKeyLoopMatchesRecursion compares the loop with
// the recursion on random nesting chains: compound nodes whose ports lead,
// through port dummies and their edges, to ports of nodes one level deeper,
// with dummies missing, unconnected or leading to ports without nodes.
func TestTraversalKeyLoopMatchesRecursion(t *testing.T) {
	random := rand.New(rand.NewSource(1))
	for round := 0; round < 3000; round++ {
		arena := NewLGraphArena()
		graphID := arena.AddGraph()
		addNodes := func(graphID LGraphID) []LNodeID {
			var nodes []LNodeID
			for count := 1 + random.Intn(3); count > 0; count-- {
				nodeID := arena.AddNode(graphID)
				arena.Graph(graphID).LayerlessNodes = append(arena.Graph(graphID).LayerlessNodes, nodeID)
				for ports := random.Intn(3); ports > 0; ports-- {
					portID := arena.AddPort()
					arena.SetPortNode(portID, &nodeID)
				}
				nodes = append(nodes, nodeID)
			}
			return nodes
		}
		nodes := addNodes(graphID)
		start := arena.AddPort()
		first := nodes[random.Intn(len(nodes))]
		arena.SetPortNode(start, &first)
		port, node := start, first
		for depth := random.Intn(7); depth > 0; depth-- {
			nested := arena.AddGraph()
			arena.Graph(nested).ParentNode = &node
			arena.Node(node).NestedGraph = &nested
			inner := addNodes(nested)
			if random.Intn(8) == 0 {
				break // no port dummy
			}
			dummy := arena.AddNode(nested)
			basegraph.Set(&arena.Port(port).Properties, PortDummy, dummy)
			dummyPort := arena.AddPort()
			arena.SetPortNode(dummyPort, &dummy)
			if random.Intn(8) == 0 {
				break // the dummy is not connected
			}
			next := arena.AddPort()
			target := inner[random.Intn(len(inner))]
			if random.Intn(10) != 0 {
				arena.SetPortNode(next, &target) // else a port without node
			}
			edgeID := arena.AddEdge()
			if random.Intn(2) == 0 {
				arena.SetEdgeSource(edgeID, &dummyPort)
				arena.SetEdgeTarget(edgeID, &next)
			} else {
				arena.SetEdgeSource(edgeID, &next)
				arena.SetEdgeTarget(edgeID, &dummyPort)
			}
			if arena.Port(next).Node == nil {
				break
			}
			port, node = next, target
		}
		wantPath, wantIndex := recursiveCompoundInsideTraversalKey(arena, graphID, start)
		gotPath, gotIndex := compoundInsideTraversalKey(arena, graphID, start)
		if fmt.Sprint(wantPath, wantIndex, wantPath == nil) != fmt.Sprint(gotPath, gotIndex, gotPath == nil) {
			t.Fatalf("round %d: recursion %v %d, loop %v %d", round, wantPath, wantIndex, gotPath, gotIndex)
		}
	}
}

// TestReleasedArenaKeepsNoCallerData: a released arena keeps
// its elements' slots and property storage for reuse, but nothing they
// referred to that may be the caller's: property values and label texts are
// cleared, and so are the boxes of the last stored point and spacing; a large
// origin box table is dropped.
func TestReleasedArenaKeepsNoCallerData(t *testing.T) {
	drainArenaPool()
	arena := AcquireLGraphArena()
	graphID := arena.AddGraph()
	node := arena.AddNode(graphID)
	caller := []string{"caller's value"}
	basegraph.Set(&arena.Node(node).Properties, ContentAlignment, caller)
	label := arena.AddLabel("caller's text")
	basegraph.Set(&arena.Label(label).Properties, ContentAlignment, caller)
	arena.SetOrigin(&arena.Node(node).Properties, NodeOrigin(basegraph.NodeID(5000)))
	arena.SetOrigin(&arena.Node(node).Properties, PortOrigin(1))
	arena.SetPoint(&arena.Graph(graphID).Properties, PositionOption, geometry.Point{X: 1})
	ReleaseLGraphArena(arena)
	if pooled := arenaPool.Get(); pooled != arena {
		t.Fatalf("the arena was not kept")
	}
	if nodes := arena.Nodes[:1]; nodes[0].Properties.Len() != 0 {
		t.Fatalf("a released node kept %d properties", nodes[0].Properties.Len())
	}
	if labels := arena.Labels[:1]; labels[0].Text != "" || labels[0].Properties.Len() != 0 {
		t.Fatalf("a released label kept text %q and %d properties", labels[0].Text, labels[0].Properties.Len())
	}
	if arena.origins[OriginNode] != nil || len(arena.origins[OriginPort]) == 0 {
		t.Fatalf("origin boxes: node table %d (want dropped), port table %d (want kept)",
			cap(arena.origins[OriginNode]), len(arena.origins[OriginPort]))
	}
	if arena.lastPointBox != nil || arena.lastSpacingBox != nil {
		t.Fatal("the last point or spacing box was kept")
	}
	// The kept storage serves the next layout like new storage.
	graphID = arena.AddGraph()
	node = arena.AddNode(graphID)
	if arena.Node(node).Properties.Len() != 0 || basegraph.Has(&arena.Node(node).Properties, ContentAlignment) {
		t.Fatal("a reused slot's property map is not empty")
	}
}

// TestReleasedArenaStorageIsBounded: a kept slot keeps its
// property storage only up to maxKeptPropertyEntries entries, and origin box
// tables up to maxKeptOriginBoxes.
func TestReleasedArenaStorageIsBounded(t *testing.T) {
	drainArenaPool()
	arena := AcquireLGraphArena()
	graphID := arena.AddGraph()
	small, large := arena.AddNode(graphID), arena.AddNode(graphID)
	basegraph.Set(&arena.Node(small).Properties, ContentAlignment, []string{"x"})
	for index := 0; index <= maxKeptPropertyEntries; index++ {
		arena.Node(large).Properties.SetRaw(fmt.Sprintf("test.property.%d", index), index)
	}
	// A table for ID i grows to capacity 2i+2.
	arena.SetOrigin(&arena.Node(small).Properties, EdgeOrigin(basegraph.EdgeID(maxKeptOriginBoxes/2-1)))
	arena.SetOrigin(&arena.Node(small).Properties, LabelOrigin(basegraph.LabelID(maxKeptOriginBoxes/2)))
	ReleaseLGraphArena(arena)
	if pooled := arenaPool.Get(); pooled != arena {
		t.Fatalf("the arena was not kept")
	}
	nodes := arena.Nodes[:2]
	if nodes[small].Properties.EntryCapacity() == 0 || nodes[large].Properties.EntryCapacity() != 0 {
		t.Fatalf("property storage kept: %d (want some) and %d (want none)",
			nodes[small].Properties.EntryCapacity(), nodes[large].Properties.EntryCapacity())
	}
	if arena.origins[OriginEdge] == nil || arena.origins[OriginLabel] != nil {
		t.Fatalf("origin tables of %d and %d boxes kept, want the first only",
			cap(arena.origins[OriginEdge]), cap(arena.origins[OriginLabel]))
	}
}
