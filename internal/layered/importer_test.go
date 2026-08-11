// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"testing"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
)

func TestImporterPreservesNodePortEdgeOrderAndTypedOptions(t *testing.T) {
	model := basegraph.NewModel()
	root := model.Root
	basegraph.Set(&model.Node(root).Properties, core.Direction, "DOWN")
	model.Node(root).Properties.SetRaw(CycleBreaking.ID, "GREEDY_MODEL_ORDER")
	model.Node(root).Properties.SetRaw(SpacingNodeNodeBetweenLayers.ID, 70.0)
	first := model.AddNode(&root)
	second := model.AddNode(&root)
	model.Node(first).Shape.SetDimensions(30, 20)
	model.Node(second).Shape.SetDimensions(40, 25)
	source := model.AddPort(first)
	target := model.AddPort(second)
	basegraph.Set(&model.Port(source).Properties, core.PortSide, "SOUTH")
	basegraph.Set(&model.Port(target).Properties, core.PortSide, "NORTH")
	edge := model.AddEdge(&root)
	model.AddSource(edge, basegraph.PortRef(source))
	model.AddTarget(edge, basegraph.PortRef(target))

	importer := NewElkGraphImporter(model)
	arena, graphID, err := importer.ImportGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := arena.Graph(graphID).LayerlessNodes; len(got) != 2 || got[0] != importer.Maps.Nodes[first] || got[1] != importer.Maps.Nodes[second] {
		t.Fatalf("layerless nodes = %v", got)
	}
	if got := basegraph.Get(&arena.Graph(graphID).Properties, DirectionOption); got != DirectionDown {
		t.Fatalf("direction = %v", got)
	}
	if got := basegraph.Get(&arena.Graph(graphID).Properties, CycleBreaking); got != CycleBreakingGreedyModelOrder {
		t.Fatalf("cycle strategy = %v", got)
	}
	if got := basegraph.Get(&arena.Graph(graphID).Properties, SpacingNodeNodeBetweenLayers); got != 70 {
		t.Fatalf("layer spacing = %v", got)
	}
	if got := arena.Port(importer.Maps.Ports[source]).Side; got != SouthSide {
		t.Fatalf("source side = %v", got)
	}
	if got := arena.Edge(importer.Maps.Edges[edge]); *got.Source != importer.Maps.Ports[source] || *got.Target != importer.Maps.Ports[target] {
		t.Fatalf("edge endpoints = %v -> %v", got.Source, got.Target)
	}
}

func TestImporterLeavesImplicitPortPositionsAtOrigin(t *testing.T) {
	for _, test := range []struct {
		direction Direction
	}{
		{DirectionRight},
		{DirectionLeft},
		{DirectionDown},
		{DirectionUp},
	} {
		arena := NewLGraphArena()
		graphID := arena.AddGraph()
		basegraph.Set(&arena.Graph(graphID).Properties, DirectionOption, test.direction)
		nodeID := arena.AddNode(graphID)
		arena.Node(nodeID).Size = geometry.Point{X: 80, Y: 60}
		importer := &ElkGraphImporter{}
		output := importer.createDefaultPort(arena, graphID, nodeID, PortTypeOutput)
		input := importer.createDefaultPort(arena, graphID, nodeID, PortTypeInput)
		if got := arena.Port(output).Position; got != (geometry.Point{}) {
			t.Errorf("%v output position = %+v, want zero until sizing", test.direction, got)
		}
		if got := arena.Port(input).Position; got != (geometry.Point{}) {
			t.Errorf("%v input position = %+v, want zero until sizing", test.direction, got)
		}
	}
}

func TestImporterPreservesRawNodeSizeBeforeDirectionTransform(t *testing.T) {
	model := basegraph.NewModel()
	root := model.Root
	basegraph.Set(&model.Node(root).Properties, core.Direction, "DOWN")
	nodeID := model.AddNode(&root)
	model.Node(nodeID).Shape.SetDimensions(357, 81)
	basegraph.Set(&model.Node(nodeID).Properties, NodeSizeConstraints, []string{"MINIMUM_SIZE"})
	basegraph.Set(&model.Node(nodeID).Properties, NodeSizeMinimum, geometry.Point{X: 81, Y: 357})

	importer := NewElkGraphImporter(model)
	arena, _, err := importer.ImportGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := arena.Node(importer.Maps.Nodes[nodeID]).Size; got != (geometry.Point{X: 357, Y: 81}) {
		t.Fatalf("imported size = %+v, want raw 357x81 before DOWN transforms", got)
	}
	if model.Node(nodeID).Shape.Width != 357 || model.Node(nodeID).Shape.Height != 81 {
		t.Fatalf("import mutated source size = %vx%v", model.Node(nodeID).Shape.Width, model.Node(nodeID).Shape.Height)
	}
}

func TestImporterOnlyCopiesExplicitIndividualSpacings(t *testing.T) {
	model := basegraph.NewModel()
	root := model.Root
	nodeID := model.AddNode(&root)
	model.Node(nodeID).Properties.SetRaw(SpacingNodeSelfLoop.ID, 0.0)
	explicit := &core.IndividualSpacings{}
	basegraph.Set(&explicit.Properties, SpacingPortPort, 17.0)
	basegraph.Set(&model.Node(nodeID).Properties, core.SpacingIndividual, explicit)

	importer := NewElkGraphImporter(model)
	arena, _, err := importer.ImportGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	individual, found := basegraph.TryGet(
		&arena.Node(importer.Maps.Nodes[nodeID]).Properties, SpacingIndividual)
	if !found {
		t.Fatal("explicit individual spacings were not copied")
	}
	if got, present := basegraph.TryGet(&individual.Properties, SpacingNodeSelfLoop); present {
		t.Fatalf("direct nodeSelfLoop was incorrectly wrapped: %v", got)
	}
	if got, present := basegraph.TryGet(&individual.Properties, SpacingPortPort); !present || got != 17 {
		t.Fatalf("individual portPort = %v, explicit=%v", got, present)
	}
}

func TestImporterBuildsNestedGraphsForIncludeChildren(t *testing.T) {
	model := basegraph.NewModel()
	root := model.Root
	basegraph.Set(&model.Node(root).Properties, core.HierarchyHandling, "INCLUDE_CHILDREN")
	container := model.AddNode(&root)
	basegraph.Set(&model.Node(container).Properties, core.HierarchyHandling, "INCLUDE_CHILDREN")
	child := model.AddNode(&container)
	sibling := model.AddNode(&root)
	edge := model.AddEdge(&root)
	model.AddSource(edge, basegraph.NodeRef(child))
	model.AddTarget(edge, basegraph.NodeRef(sibling))

	importer := NewElkGraphImporter(model)
	arena, graphID, err := importer.ImportGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	nested := arena.Node(importer.Maps.Nodes[container]).NestedGraph
	if nested == nil || *nested != importer.Maps.Graphs[container] {
		t.Fatalf("nested graph = %v", nested)
	}
	if arena.NodeGraph(importer.Maps.Nodes[child]) != *nested {
		t.Fatal("child was not imported into its container graph")
	}
	if arena.Graph(*nested).ParentNode == nil || *arena.Graph(*nested).ParentNode != importer.Maps.Nodes[container] {
		t.Fatal("nested graph parent relation missing")
	}
	if _, found := basegraph.TryGet(&arena.Edge(importer.Maps.Edges[edge]).Properties, CoordinateSystemOrigin); !found {
		t.Fatal("cross-hierarchy edge lacks coordinate-system origin")
	}
	_ = graphID
}

func TestImporterAssignsHierarchicalEdgeModelOrderBreadthFirst(t *testing.T) {
	model := basegraph.NewModel()
	root := model.Root
	basegraph.Set(&model.Node(root).Properties, core.HierarchyHandling, "INCLUDE_CHILDREN")
	model.Node(root).Properties.SetRaw(ConsiderModelOrder.ID, "NODES_AND_EDGES")
	compound := model.AddNode(&root)
	basegraph.Set(&model.Node(compound).Properties, core.HierarchyHandling, "INCLUDE_CHILDREN")
	first := model.AddNode(&compound)
	second := model.AddNode(&compound)
	sibling := model.AddNode(&root)

	// Arena order intentionally disagrees with the breadth-first containment
	// order used by ElkGraphImporter.importHierarchicalGraph.
	deep := model.AddEdge(&compound)
	model.AddSource(deep, basegraph.NodeRef(first))
	model.AddTarget(deep, basegraph.NodeRef(second))
	shallow := model.AddEdge(&root)
	model.AddSource(shallow, basegraph.NodeRef(first))
	model.AddTarget(shallow, basegraph.NodeRef(sibling))

	importer := NewElkGraphImporter(model)
	arena, _, err := importer.ImportGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := basegraph.Get(&arena.Edge(importer.Maps.Edges[shallow]).Properties, ModelOrder); got != 0 {
		t.Fatalf("shallow edge model order = %d, want 0", got)
	}
	if got := basegraph.Get(&arena.Edge(importer.Maps.Edges[deep]).Properties, ModelOrder); got != 1 {
		t.Fatalf("deep edge model order = %d, want 1", got)
	}
}

func TestImporterSkipsModelOrderForOptedOutNodes(t *testing.T) {
	model := basegraph.NewModel()
	root := model.Root
	model.Node(root).Properties.SetRaw(ConsiderModelOrder.ID, "NODES_AND_EDGES")
	first := model.AddNode(&root)
	optedOut := model.AddNode(&root)
	last := model.AddNode(&root)
	basegraph.Set(&model.Node(optedOut).Properties, NoModelOrder, true)

	importer := NewElkGraphImporter(model)
	arena, _, err := importer.ImportGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := basegraph.Get(&arena.Node(importer.Maps.Nodes[first]).Properties, ModelOrder); got != 0 {
		t.Fatalf("first model order = %d, want 0", got)
	}
	if _, found := basegraph.TryGet(&arena.Node(importer.Maps.Nodes[optedOut]).Properties, ModelOrder); found {
		t.Fatal("opted-out node received a model order")
	}
	if got := basegraph.Get(&arena.Node(importer.Maps.Nodes[last]).Properties, ModelOrder); got != 1 {
		t.Fatalf("last model order = %d, want 1", got)
	}
}

func TestImporterAssignsPortModelOrder(t *testing.T) {
	model := basegraph.NewModel()
	root := model.Root
	model.Node(root).Properties.SetRaw(ConsiderModelOrder.ID, "NODES_AND_EDGES")
	nodeID := model.AddNode(&root)
	first := model.AddPort(nodeID)
	skipped := model.AddPort(nodeID)
	last := model.AddPort(nodeID)
	basegraph.Set(&model.Port(skipped).Properties, core.NoLayout, true)

	importer := NewElkGraphImporter(model)
	arena, _, err := importer.ImportGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := basegraph.Get(&arena.Port(importer.Maps.Ports[first]).Properties, ModelOrder); got != 0 {
		t.Fatalf("first port model order = %d, want 0", got)
	}
	if _, found := importer.Maps.Ports[skipped]; found {
		t.Fatal("no-layout port was imported")
	}
	if got := basegraph.Get(&arena.Port(importer.Maps.Ports[last]).Properties, ModelOrder); got != 2 {
		t.Fatalf("last port model order = %d, want 2 including skipped port", got)
	}
}

func TestImporterUsesImmediateParentModelOrderProfile(t *testing.T) {
	model := basegraph.NewModel()
	root := model.Root
	basegraph.Set(&model.Node(root).Properties, core.HierarchyHandling, "INCLUDE_CHILDREN")
	model.Node(root).Properties.SetRaw(ConsiderModelOrder.ID, "NODES_AND_EDGES")
	container := model.AddNode(&root)
	basegraph.Set(&model.Node(container).Properties, core.HierarchyHandling, "INCLUDE_CHILDREN")
	model.Node(container).Properties.SetRaw(ConsiderModelOrder.ID, "NONE")
	child := model.AddNode(&container)
	childPort := model.AddPort(child)

	importer := NewElkGraphImporter(model)
	arena, _, err := importer.ImportGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := basegraph.TryGet(&arena.Node(importer.Maps.Nodes[child]).Properties, ModelOrder); found {
		t.Fatal("nested child inherited the root model-order profile")
	}
	if _, found := basegraph.TryGet(&arena.Port(importer.Maps.Ports[childPort]).Properties, ModelOrder); found {
		t.Fatal("nested child port inherited the root model-order profile")
	}
}

func TestImporterActivatesPartitionGraphProperty(t *testing.T) {
	model := basegraph.NewModel()
	root := model.Root
	basegraph.Set(&model.Node(root).Properties, core.PartitioningActivate, true)

	arena, graphID, err := NewElkGraphImporter(model).ImportGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := basegraph.Get(&arena.Graph(graphID).Properties, GraphProperties); got&GraphPropertyPartitions == 0 {
		t.Fatalf("graph properties = %v, want PARTITIONS", got)
	}
}
