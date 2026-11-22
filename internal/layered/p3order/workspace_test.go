// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"reflect"
	"runtime"
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
)

// drainWorkspaces empties the workspace slot and pool and
// returns how many workspaces they held.
func drainWorkspaces() int {
	count := 0
	for releasedCrossMinWorkspaces.Get() != nil {
		count++
	}
	return count
}

// crossingArena is two layers of two nodes whose edges cross.
func crossingArena() (*layered.LGraphArena, layered.LGraphID, layered.LayerID) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	leftLayer, rightLayer := arena.AddLayer(graphID), arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = []layered.LayerID{leftLayer, rightLayer}
	leftTop := addCountingNode(arena, graphID, leftLayer, layered.EastSide)
	leftBottom := addCountingNode(arena, graphID, leftLayer, layered.EastSide)
	rightTop := addCountingNode(arena, graphID, rightLayer, layered.WestSide)
	rightBottom := addCountingNode(arena, graphID, rightLayer, layered.WestSide)
	connectCountingPorts(arena, arena.Node(leftTop).Ports[0], arena.Node(rightBottom).Ports[0])
	connectCountingPorts(arena, arena.Node(leftBottom).Ports[0], arena.Node(rightTop).Ports[0])
	basegraph.Set(&arena.Graph(graphID).Properties, layered.Random, javacompat.NewRandom(1))
	return arena, graphID, leftLayer
}

// TestPanickingSweepDropsItsWorkspace: a crossing minimization
// that panics after taking its workspace (here on a layer listing a node the
// arena does not have) must not give the workspace back to the pool, whose
// scratch it may have left half-updated. A normal call gives it back.
func TestPanickingSweepDropsItsWorkspace(t *testing.T) {
	drainWorkspaces()
	arena, graphID, leftLayer := crossingArena()
	arena.Layer(leftLayer).Nodes = append(arena.Layer(leftLayer).Nodes, layered.LNodeID(len(arena.Nodes)+100))
	panicked := func() (recovered any) {
		defer func() { recovered = recover() }()
		_ = NewLayerSweepCrossingMinimizer(CrossMinBarycenter).ProcessHierarchy(arena, graphID)
		return nil
	}()
	if panicked == nil {
		t.Fatal("the malformed arena did not make the minimizer panic")
	}
	if pooled := drainWorkspaces(); pooled != 0 {
		t.Fatalf("a panicking minimization pooled %d workspace(s)", pooled)
	}

	arena, graphID, _ = crossingArena()
	if err := NewLayerSweepCrossingMinimizer(CrossMinBarycenter).ProcessHierarchy(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if pooled := drainWorkspaces(); pooled != 1 {
		t.Fatalf("a completed minimization pooled %d workspaces, want 1", pooled)
	}
}

// TestOversizedWorkspaceIsDropped: a workspace whose slabs
// exceed maxSlotWorkspaceElements is dropped, not put into the sync.Pool
// (TinyGo's sync.Pool never frees what it holds); a bounded one is kept.
func TestOversizedWorkspaceIsDropped(t *testing.T) {
	drainWorkspaces()
	for _, size := range []int{maxSlotWorkspaceElements / 2, maxSlotWorkspaceElements + 1} {
		ws := acquireCrossMinWorkspace()
		ws.ints.take(size)
		releaseCrossMinWorkspace(ws)
		want := 1
		if size > maxSlotWorkspaceElements {
			want = 0
		}
		// Only the slot survives collections: an oversized value may go to the
		// sync.Pool under gc, never to the slot.
		runtime.GC()
		runtime.GC()
		if pooled := drainWorkspaces(); pooled != want {
			t.Fatalf("workspace of %d slab elements: %d kept, want %d", size, pooled, want)
		}
	}
}

// TestStateCachesOnlyItsOwnNode: a BarycenterState caches the
// successor constraints and barycenter associates of its own node only. A
// read for another node through the state (a node of another graph whose
// Layer.ID/LNode.ID index this state, as ELK's lookup does) reads that
// node's property every time, as ELK does, and neither fills the cache nor
// counts as a first read.
func TestStateCachesOnlyItsOwnNode(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	own, other := arena.AddNode(graphID), arena.AddNode(graphID)
	basegraph.Set(&arena.Node(other).Properties, layered.InLayerSuccessorConstraints, []layered.LNodeID{own})
	basegraph.Set(&arena.Node(other).Properties, layered.BarycenterAssociates, []layered.LNodeID{own})
	state := BarycenterState{Node: own}
	resolver := &ForsterConstraintResolver{}
	for round := 0; round < 2; round++ {
		if got := resolver.readSuccessors(&state, arena, other); len(got) != 1 || got[0] != own {
			t.Fatalf("round %d: successors read for the other node = %v", round, got)
		}
		if got := state.barycenterAssociates(arena, other); len(got) != 1 || got[0] != own {
			t.Fatalf("round %d: associates read for the other node = %v", round, got)
		}
	}
	if state.successorsKnown || state.associatesKnown || resolver.knownSuccessors != 0 {
		t.Fatalf("reads for another node were cached: successors %v, associates %v, counted %d",
			state.successorsKnown, state.associatesKnown, resolver.knownSuccessors)
	}
	// The state's own node: empty lists, and the first reads materialize the
	// empty defaults into its property map, as ELK's first reads do.
	if got := resolver.readSuccessors(&state, arena, own); len(got) != 0 {
		t.Fatalf("own successors = %v", got)
	}
	if got := state.barycenterAssociates(arena, own); len(got) != 0 {
		t.Fatalf("own associates = %v", got)
	}
	properties := &arena.Node(own).Properties
	if !basegraph.Has(properties, layered.InLayerSuccessorConstraints) || !basegraph.Has(properties, layered.BarycenterAssociates) {
		t.Fatal("the own node's first reads did not materialize the defaults")
	}
	if resolver.knownSuccessors != 1 || resolver.nonEmptySuccessors != 0 {
		t.Fatalf("counted %d first reads (%d non-empty), want 1 (0)", resolver.knownSuccessors, resolver.nonEmptySuccessors)
	}
}

// TestForeignPortDummyKeepsOwnReads uses an arena found by a randomized
// aliasing test: a parent port whose PortDummy is a node of the parent
// graph (A), which sortPortDummiesByPortPositions puts into the child's first
// layer, where its state is the child dummy D1's. The original reads D1's
// own successor constraints and so materializes the empty default into D1's
// map; with the state caches filled by A's read it did not. (Both then panic
// on A, which has no LPort origin, with the same value; the property maps are
// compared after the panic.)
func TestForeignPortDummyKeepsOwnReads(t *testing.T) {
	arena := layered.NewLGraphArena()
	root, child := arena.AddGraph(), arena.AddGraph()
	// 0 A (root, layer 0), 1 P (root, layer 1, holds child), 2 D1 (child,
	// layer 0, external port dummy), 3 Y (child, layer 0), 4 X (child,
	// layer 1), 5 B (root, layer 0), 6 Q (child, layer 1).
	graphs := []layered.LGraphID{root, root, child, child, child, root, child}
	types := []layered.NodeType{0, 0, layered.ExternalPortNode, 0, 0, 0, 0}
	for index, graphID := range graphs {
		nodeID := arena.AddNode(graphID)
		arena.Node(nodeID).Type = types[index]
	}
	parent := layered.LNodeID(1)
	arena.Graph(child).ParentNode = &parent
	arena.Node(parent).NestedGraph = &child
	for graphID, layers := range [][][]int{{{0, 5}, {1}}, {{2, 3}, {4, 6}}} {
		for _, nodes := range layers {
			layerID := arena.AddLayer(layered.LGraphID(graphID))
			arena.Graph(layered.LGraphID(graphID)).Layers = append(arena.Graph(layered.LGraphID(graphID)).Layers, layerID)
			for _, node := range nodes {
				layer := layerID
				arena.SetNodeLayer(layered.LNodeID(node), &layer)
			}
		}
	}
	thoroughness := 5
	rootProperties := &arena.Graph(root).Properties
	basegraph.Set(rootProperties, layered.Hierarchy, layered.HierarchyIncludeChildren)
	basegraph.Set(rootProperties, layered.RandomSeed, 2)
	basegraph.Set(rootProperties, layered.Thoroughness, thoroughness)
	childProperties := &arena.Graph(child).Properties
	basegraph.Set(childProperties, layered.Hierarchy, layered.HierarchyIncludeChildren)
	basegraph.Set(childProperties, layered.HierarchicalSweepiness, 1.5)
	basegraph.Set(childProperties, layered.GraphProperties, layered.GraphPropertyExternalPorts)
	type portDesc struct {
		node      int
		side      layered.PortSide
		portDummy int // node, or -1
	}
	for _, desc := range []portDesc{
		{1, layered.WestSide, 2}, {1, layered.WestSide, 0}, // P's ports: dummies D1 and (foreign) A
		{0, layered.EastSide, -1}, {5, layered.EastSide, -1}, {2, layered.EastSide, -1}, {3, layered.EastSide, -1},
		{4, layered.WestSide, -1}, {6, layered.WestSide, -1}, {1, layered.EastSide, -1}, {1, layered.EastSide, -1},
	} {
		portID := arena.AddPort()
		node := layered.LNodeID(desc.node)
		arena.SetPortNode(portID, &node)
		arena.SetPortSide(portID, desc.side)
		if desc.portDummy >= 0 {
			basegraph.Set(&arena.Port(portID).Properties, layered.InsideConnections, true)
			basegraph.Set(&arena.Port(portID).Properties, layered.PortDummy, layered.LNodeID(desc.portDummy))
		}
	}
	basegraph.Set(&arena.Node(2).Properties, layered.OriginalElement, layered.Origin{Kind: layered.OriginLPort, LPort: 0})
	basegraph.Set(&arena.Node(0).Properties, layered.InLayerSuccessorConstraints, []layered.LNodeID{3})
	for _, ends := range [][2]layered.LPortID{{2, 0}, {3, 1}, {4, 7}, {5, 6}, {4, 6}, {2, 1}} {
		edgeID := arena.AddEdge()
		source, target := ends[0], ends[1]
		arena.SetEdgeSource(edgeID, &source)
		arena.SetEdgeTarget(edgeID, &target)
	}
	panicked := func() (recovered any) {
		defer func() { recovered = recover() }()
		_ = NewLayerSweepCrossingMinimizer(CrossMinBarycenter).ProcessHierarchy(arena, root)
		return nil
	}()
	if panicked != "p3order: external port dummy has no LPort origin" {
		t.Fatalf("recovered %v, want the original's panic", panicked)
	}
	if !basegraph.Has(&arena.Node(2).Properties, layered.InLayerSuccessorConstraints) {
		t.Fatal("D1's own successor constraints were never read (its default is not materialized)")
	}
}

// TestWorkspaceBytesBoundTheSlot: a workspace whose storage
// outside its slabs is large (here a counting buffer) is dropped although
// its slabs are small, and a group map that has held many entries is not
// kept with a workspace; the byte sizes used are at least the types' sizes.
func TestWorkspaceBytesBoundTheSlot(t *testing.T) {
	drainWorkspaces()
	ws := acquireCrossMinWorkspace()
	ws.countScratch.ends = make([]int, 0, 1<<18+1) // over 2 MB outside the slabs
	releaseCrossMinWorkspace(ws)
	// Only the slot survives collections: an oversized value may go to the
	// sync.Pool under gc, never to the slot.
	runtime.GC()
	runtime.GC()
	if pooled := drainWorkspaces(); pooled != 0 {
		t.Fatal("a workspace holding a counting buffer of over 2 MB was kept")
	}

	ws = acquireCrossMinWorkspace()
	ws.forsterScratch.groupOf = map[layered.LNodeID]int{1: 0}
	ws.forsterScratch.groupOfLargest = maxKeptGroupOfEntries + 1
	ws.forsterScratch.groups = append(ws.forsterScratch.groups, constraintGroup{outgoing: []int{1}})[:0]
	releaseCrossMinWorkspace(ws)
	if pooled := drainWorkspaces(); pooled != 1 {
		t.Fatalf("%d workspaces kept, want 1", pooled)
	}
	if ws.forsterScratch.groupOf != nil {
		t.Fatal("a group map that held many entries was kept")
	}
	if groups := ws.forsterScratch.groups[:cap(ws.forsterScratch.groups)]; groups[0].outgoing != nil {
		t.Fatal("a group's edge list was kept")
	}

	for _, size := range []struct {
		value any
		bytes int
	}{
		{GraphInfoHolder{}, bytesGraphInfoHolder}, {AllCrossingsCounter{}, bytesAllCrossingsCounter},
		{CrossingsCounter{}, bytesCrossingsCounter}, {LayerSweepTypeDecider{}, bytesLayerSweepTypeDecider},
		{BarycenterPortDistributor{}, bytesPortDistributor}, {ForsterConstraintResolver{}, bytesConstraintResolver},
		{BarycenterHeuristic{}, bytesBarycenterHeuristic}, {BarycenterState{}, bytesBarycenterState},
		{layerSweepNodeInfo{}, bytesNodeInfo}, {portEntry{}, bytesPortEntry}, {edgeEnd{}, bytesEdgeEnd},
		{sweepNode{}, bytesSweepNode}, {portFact{}, bytesFact}, {originFact{}, bytesFact},
		{SweepCopy{}, bytesSweepCopy}, {constraintGroup{}, bytesConstraintGroup}, {nodeRef{}, bytesNodeRef},
		{[]int(nil), bytesSlice}, {layered.PortConstraints(0), 1},
	} {
		if actual := int(reflect.TypeOf(size.value).Size()); actual > size.bytes {
			t.Errorf("%T takes %d bytes, the bound counts %d", size.value, actual, size.bytes)
		}
	}
}
