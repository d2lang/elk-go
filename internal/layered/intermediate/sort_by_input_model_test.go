// Copyright (c) 2020 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"reflect"
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestGWTStableSortMatchesElkjsComparatorCallOrder(t *testing.T) {
	values := []int{6, 5, 4, 3, 2, 1, 0}
	type comparison struct{ first, second int }
	var comparisons []comparison
	gwtStableSort(values, func(first, second int) int {
		comparisons = append(comparisons, comparison{first, second})
		return first - second
	})

	if want := []int{0, 1, 2, 3, 4, 5, 6}; !reflect.DeepEqual(values, want) {
		t.Fatalf("sorted values = %v, want %v", values, want)
	}
	wantComparisons := []comparison{
		{6, 5}, {6, 4}, {5, 4},
		{3, 2}, {3, 1}, {2, 1}, {3, 0}, {2, 0}, {1, 0},
		{6, 0}, {4, 0}, {4, 1}, {4, 2}, {4, 3},
	}
	if !reflect.DeepEqual(comparisons, wantComparisons) {
		t.Fatalf("comparison trace = %v, want elkjs GWT trace %v", comparisons, wantComparisons)
	}
}

func TestSortByInputModelGroupsPortsByUltimateTarget(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	graph := arena.Graph(graphID)
	basegraph.Set(&graph.Properties, layered.ConsiderModelOrder, layered.OrderingNodesAndEdges)
	firstLayer := arena.AddLayerToGraph(graphID)
	secondLayer := arena.AddLayerToGraph(graphID)
	source := arena.AddNode(graphID)
	targetOne := arena.AddNode(graphID)
	targetTwo := arena.AddNode(graphID)
	arena.SetNodeLayer(source, &firstLayer)
	// Start the target layer contrary to model order to test node sorting too.
	arena.SetNodeLayer(targetTwo, &secondLayer)
	arena.SetNodeLayer(targetOne, &secondLayer)
	basegraph.Set(&arena.Node(targetOne).Properties, layered.ModelOrder, 0)
	basegraph.Set(&arena.Node(targetTwo).Properties, layered.ModelOrder, 1)

	portTwo := addOrderedEdge(arena, source, targetTwo, 2)
	portOneHigh := addOrderedEdge(arena, source, targetOne, 5)
	portOneLow := addOrderedEdge(arena, source, targetOne, 1)
	if err := SortByInputModel(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if want := []layered.LPortID{portOneHigh, portOneLow, portTwo}; !reflect.DeepEqual(arena.Node(source).Ports, want) {
		t.Fatalf("source ports = %v, want %v", arena.Node(source).Ports, want)
	}
	if want := []layered.LNodeID{targetOne, targetTwo}; !reflect.DeepEqual(arena.Layer(secondLayer).Nodes, want) {
		t.Fatalf("target nodes = %v, want %v", arena.Layer(secondLayer).Nodes, want)
	}
	orders := basegraph.Get(&arena.Node(source).Properties, layered.TargetNodeModelOrder)
	if orders[targetOne] != 1 || orders[targetTwo] != 2 {
		t.Fatalf("target minimum orders = %v, want {%d:1, %d:2}", orders, targetOne, targetTwo)
	}
}

func TestLongEdgeTargetNodePreprocessingUsesDummyShortcutAndCaches(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	firstLayer := arena.AddLayerToGraph(graphID)
	secondLayer := arena.AddLayerToGraph(graphID)
	thirdLayer := arena.AddLayerToGraph(graphID)
	source := arena.AddNode(graphID)
	target := arena.AddNode(graphID)
	arena.SetNodeLayer(source, &firstLayer)
	arena.SetNodeLayer(target, &thirdLayer)
	sourcePort := addOrderedEdge(arena, source, target, 4)
	SplitLongEdges(arena, graphID)
	if len(arena.Layer(secondLayer).Nodes) != 1 || arena.Node(arena.Layer(secondLayer).Nodes[0]).Type != layered.LongEdgeNode {
		t.Fatal("long-edge splitter did not create the expected dummy")
	}
	orders, err := LongEdgeTargetNodePreprocessing(arena, source)
	if err != nil {
		t.Fatal(err)
	}
	if orders[target] != 4 {
		t.Fatalf("ultimate target orders = %v, want target order 4", orders)
	}
	if got, found := basegraph.TryGet(&arena.Port(sourcePort).Properties, layered.LongEdgeTargetNode); !found || got != target {
		t.Fatalf("source long-edge target = %v, %v; want %v, true", got, found, target)
	}
	basegraph.Set(&arena.Edge(arena.Port(sourcePort).OutgoingEdges[0]).Properties, layered.ModelOrder, 1)
	cached, err := LongEdgeTargetNodePreprocessing(arena, source)
	if err != nil {
		t.Fatal(err)
	}
	if cached[target] != 4 {
		t.Fatalf("cached target order = %d, want 4", cached[target])
	}
}

func TestSortByInputModelHonorsFixedPortOrder(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	layerID := arena.AddLayerToGraph(graphID)
	source := arena.AddNode(graphID)
	firstTarget := arena.AddNode(graphID)
	secondTarget := arena.AddNode(graphID)
	arena.SetNodeLayer(source, &layerID)
	arena.SetNodeLayer(firstTarget, &layerID)
	arena.SetNodeLayer(secondTarget, &layerID)
	basegraph.Set(&arena.Node(source).Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedOrder)
	first := addOrderedEdge(arena, source, firstTarget, 9)
	second := addOrderedEdge(arena, source, secondTarget, 1)
	if err := SortByInputModel(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if want := []layered.LPortID{first, second}; !reflect.DeepEqual(arena.Node(source).Ports, want) {
		t.Fatalf("fixed port order = %v, want %v", arena.Node(source).Ports, want)
	}
}

func addOrderedEdge(
	arena *layered.LGraphArena,
	source, target layered.LNodeID,
	modelOrder int,
) layered.LPortID {
	sourcePort := arena.AddPort()
	targetPort := arena.AddPort()
	arena.SetPortNode(sourcePort, &source)
	arena.SetPortNode(targetPort, &target)
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &sourcePort)
	arena.SetEdgeTarget(edgeID, &targetPort)
	basegraph.Set(&arena.Edge(edgeID).Properties, layered.ModelOrder, modelOrder)
	return sourcePort
}
