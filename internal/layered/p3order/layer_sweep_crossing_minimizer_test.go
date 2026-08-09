// Copyright (c) 2010, 2015 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestLayerSweepCrossingMinimizerRemovesSimpleCrossing(t *testing.T) {
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

	processor := NewLayerSweepCrossingMinimizer(CrossMinBarycenter)
	if err := processor.Process(arena, graphID); err != nil {
		t.Fatal(err)
	}
	order := [][]layered.LNodeID{
		append([]layered.LNodeID(nil), arena.Layer(leftLayer).Nodes...),
		append([]layered.LNodeID(nil), arena.Layer(rightLayer).Nodes...),
	}
	counter := NewAllCrossingsCounter(2)
	InitializeAllCrossingsCounter(counter, arena, order)
	if got := counter.CountAllCrossings(arena, order); got != 0 {
		t.Fatalf("crossings after layer sweep = %d, want 0; order = %v", got, order)
	}
	for _, nodeID := range append(order[0], order[1]...) {
		if got := basegraph.Get(&arena.Node(nodeID).Properties, layered.PortConstraintsOption); got != layered.PortConstraintsFixedOrder {
			t.Fatalf("node %d port constraints = %v, want FIXED_ORDER", nodeID, got)
		}
	}
}

func TestLayerSweepCrossingMinimizerProcessesHierarchyFromRoot(t *testing.T) {
	arena := layered.NewLGraphArena()
	root := arena.AddGraph()
	rootLayer := arena.AddLayer(root)
	arena.Graph(root).Layers = []layered.LayerID{rootLayer}
	parent := arena.AddNode(root)
	arena.SetNodeLayer(parent, &rootLayer)
	child := arena.AddGraph()
	arena.Graph(child).ParentNode = &parent
	arena.Node(parent).NestedGraph = &child
	leftLayer, rightLayer := arena.AddLayer(child), arena.AddLayer(child)
	arena.Graph(child).Layers = []layered.LayerID{leftLayer, rightLayer}
	leftTop := addCountingNode(arena, child, leftLayer, layered.EastSide)
	leftBottom := addCountingNode(arena, child, leftLayer, layered.EastSide)
	rightTop := addCountingNode(arena, child, rightLayer, layered.WestSide)
	rightBottom := addCountingNode(arena, child, rightLayer, layered.WestSide)
	connectCountingPorts(arena, arena.Node(leftTop).Ports[0], arena.Node(rightBottom).Ports[0])
	connectCountingPorts(arena, arena.Node(leftBottom).Ports[0], arena.Node(rightTop).Ports[0])
	basegraph.Set(&arena.Graph(root).Properties, layered.Hierarchy, layered.HierarchyIncludeChildren)
	basegraph.Set(&arena.Graph(root).Properties, layered.Random, javacompat.NewRandom(1))
	basegraph.Set(&arena.Graph(child).Properties, layered.Random, javacompat.NewRandom(1))

	processor := NewLayerSweepCrossingMinimizer(CrossMinBarycenter)
	if err := processor.ProcessHierarchy(arena, root); err != nil {
		t.Fatal(err)
	}
	order := [][]layered.LNodeID{
		append([]layered.LNodeID(nil), arena.Layer(leftLayer).Nodes...),
		append([]layered.LNodeID(nil), arena.Layer(rightLayer).Nodes...),
	}
	counter := NewAllCrossingsCounter(2)
	InitializeAllCrossingsCounter(counter, arena, order)
	if got := counter.CountAllCrossings(arena, order); got != 0 {
		t.Fatalf("child crossings after hierarchy sweep = %d, want 0", got)
	}
}

func TestLayerSweepCrossingMinimizerRejectsChildEntry(t *testing.T) {
	arena := layered.NewLGraphArena()
	root, child := arena.AddGraph(), arena.AddGraph()
	parent := arena.AddNode(root)
	arena.Graph(child).ParentNode = &parent
	if err := (&LayerSweepCrossingMinimizer{}).ProcessHierarchy(arena, child); err == nil {
		t.Fatal("expected non-root hierarchy entry to fail")
	}
}

func TestLayerSweepCrossingMinimizerConfiguration(t *testing.T) {
	configuration := (&LayerSweepCrossingMinimizer{}).ProcessorConfiguration(nil, 0)
	assertProcessors := func(slot int, want ...layered.IntermediateProcessorStrategy) {
		t.Helper()
		got := configuration.Slot(slot)
		if len(got) != len(want) {
			t.Fatalf("slot %d = %v, want %v", slot, got, want)
		}
		for index := range want {
			if got[index] != want[index] {
				t.Fatalf("slot %d = %v, want %v", slot, got, want)
			}
		}
	}
	assertProcessors(int(layered.PhaseNodeOrdering), layered.LongEdgeSplitter, layered.PortListSorter)
	assertProcessors(int(layered.PhaseNodePlacement), layered.InLayerConstraintProcessor)
	assertProcessors(int(layered.PhaseEdgeRouting)+1, layered.LongEdgeJoiner)
}

func TestInitialOrderPropertiesRetainELK081Alias(t *testing.T) {
	var properties basegraph.PropertyMap
	basegraph.Set(&properties, layered.FirstTryWithInitialOrder, true)
	if !basegraph.Get(&properties, layered.SecondTryWithInitialOrder) {
		t.Fatal("second initial-order property must read the first property's raw value")
	}
	basegraph.Set(&properties, layered.SecondTryWithInitialOrder, false)
	if basegraph.Get(&properties, layered.FirstTryWithInitialOrder) {
		t.Fatal("clearing the second property must also clear the first property")
	}
}
