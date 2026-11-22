// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"fmt"
	"math/rand"
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
)

// TestSingleNodeMatchesGeneral runs ProcessHierarchy on random single-node
// roots with and without processSingleNode and requires the same graph,
// layer and node numbers, the same root properties (keys, values, order),
// the same Random state and the same node and port lists.
func TestSingleNodeMatchesGeneral(t *testing.T) {
	random := rand.New(rand.NewSource(3))
	fast := 0
	for round := 0; round < 4000; round++ {
		seed := random.Int63()
		fastArena, fastRoot := singleNodeRoot(seed)
		generalArena, generalRoot := singleNodeRoot(seed)
		minimizer := &LayerSweepCrossingMinimizer{}
		if minimizer.processSingleNode(fastArena, fastRoot) {
			fast++
			fastArena, fastRoot = singleNodeRoot(seed)
		}
		if err := (&LayerSweepCrossingMinimizer{}).ProcessHierarchy(fastArena, fastRoot); err != nil {
			t.Fatal(err)
		}
		singleNodeDisabled = true
		err := (&LayerSweepCrossingMinimizer{}).ProcessHierarchy(generalArena, generalRoot)
		singleNodeDisabled = false
		if err != nil {
			t.Fatal(err)
		}
		if got, want := singleNodeState(fastArena, fastRoot), singleNodeState(generalArena, generalRoot); got != want {
			t.Fatalf("round %d (seed %d):\n got %s\nwant %s", round, seed, got, want)
		}
	}
	if fast == 0 || fast == 4000 {
		t.Fatalf("processSingleNode applied in %d of 4000 rounds", fast)
	}
}

func singleNodeRoot(seed int64) (*layered.LGraphArena, layered.LGraphID) {
	random := rand.New(rand.NewSource(seed))
	arena := layered.NewLGraphArena()
	root := arena.AddGraph()
	properties := &arena.Graph(root).Properties
	basegraph.Set(properties, layered.Hierarchy, layered.HierarchyIncludeChildren)
	switch random.Intn(4) {
	case 0:
	case 1:
		basegraph.Set(properties, layered.ConsiderModelOrder, layered.OrderingNone)
	default:
		basegraph.Set(properties, layered.ConsiderModelOrder, layered.OrderingStrategy(1+random.Intn(3)))
	}
	switch random.Intn(5) {
	case 0:
	case 1:
		basegraph.Set(properties, layered.Thoroughness, 0)
	default:
		basegraph.Set(properties, layered.Thoroughness, 1+random.Intn(9))
	}
	if random.Intn(5) == 0 {
		basegraph.Set(properties, modelOrderCrossingCounterNodeInfluence, float64(random.Intn(2))*0.5)
	}
	if random.Intn(3) != 0 {
		basegraph.Set(properties, layered.RandomSeed, random.Intn(100))
	}
	if random.Intn(4) != 0 {
		basegraph.Set(properties, layered.Random, javacompat.NewRandom(random.Int63()))
	}
	if random.Intn(3) == 0 {
		basegraph.Set(properties, layered.FirstTryWithInitialOrder, random.Intn(2) == 0)
	}
	arena.Graph(root).ID = 9
	layerID := arena.AddLayerToGraph(root)
	arena.Layer(layerID).ID = 4
	nodeID := arena.AddNode(root)
	arena.Node(nodeID).ID = 6
	arena.Node(nodeID).Type = layered.NodeType(random.Intn(3))
	if random.Intn(3) == 0 {
		basegraph.Set(&arena.Node(nodeID).Properties, layered.ModelOrder, random.Intn(5))
	}
	arena.SetNodeLayer(nodeID, &layerID)
	return arena, root
}

func singleNodeState(arena *layered.LGraphArena, root layered.LGraphID) string {
	graph := arena.Graph(root)
	state := fmt.Sprintf("graph %d layers %v", graph.ID, graph.Layers)
	for _, entry := range graph.Properties.Entries() {
		if random, ok := entry.Value.(*javacompat.Random); ok {
			state += fmt.Sprintf(" %s=%+v", entry.ID, *random)
			continue
		}
		state += fmt.Sprintf(" %s=%v", entry.ID, entry.Value)
	}
	for _, layerID := range graph.Layers {
		state += fmt.Sprintf(" | layer %d nodes %v", arena.Layer(layerID).ID, arena.Layer(layerID).Nodes)
		for _, nodeID := range arena.Layer(layerID).Nodes {
			node := arena.Node(nodeID)
			state += fmt.Sprintf(" node %d ports %#v props %v", node.ID, node.Ports, node.Properties.Entries())
		}
	}
	return state
}
