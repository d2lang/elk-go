// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"testing"

	"github.com/d2lang/elk-go/internal/common/nodespacing"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
)

func TestTransferrerAppliesNodePortAndEdgeLayout(t *testing.T) {
	model := basegraph.NewModel()
	root := model.Root
	first := model.AddNode(&root)
	second := model.AddNode(&root)
	model.Node(first).Shape.SetDimensions(20, 10)
	model.Node(second).Shape.SetDimensions(20, 10)
	source := model.AddPort(first)
	target := model.AddPort(second)
	edge := model.AddEdge(&root)
	model.AddSource(edge, basegraph.PortRef(source))
	model.AddTarget(edge, basegraph.PortRef(target))
	importer := NewElkGraphImporter(model)
	arena, graphID, err := importer.ImportGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	arena.Graph(graphID).Padding = geometry.Spacing{Left: 5, Top: 7}
	arena.Graph(graphID).Size = geometry.Point{X: 100, Y: 30}
	arena.Node(importer.Maps.Nodes.At(first)).Position = geometry.Point{X: 10, Y: 3}
	arena.Node(importer.Maps.Nodes.At(second)).Position = geometry.Point{X: 70, Y: 3}
	arena.Port(importer.Maps.Ports.At(source)).Anchor = geometry.Point{X: 2, Y: 1}
	arena.Port(importer.Maps.Ports.At(target)).Anchor = geometry.Point{X: 1, Y: 1}
	arena.Edge(importer.Maps.Edges.At(edge)).BendPoints = geometry.Chain{{X: 40, Y: 9}}

	if err := NewElkGraphLayoutTransferrer(model, importer.Maps).ApplyLayout(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if got := model.Node(first).Shape.Position(); got != (geometry.Point{X: 15, Y: 10}) {
		t.Fatalf("first position = %+v", got)
	}
	if model.Node(root).Shape.Width != 105 || model.Node(root).Shape.Height != 37 {
		t.Fatalf("root size = %v x %v", model.Node(root).Shape.Width, model.Node(root).Shape.Height)
	}
	section := model.Section(model.Edge(edge).Sections[0])
	if section.Start != (geometry.Point{X: 17, Y: 11}) || section.BendPoints[0] != (geometry.Point{X: 45, Y: 16}) {
		t.Fatalf("section = %+v", section)
	}
}

func TestTransferrerPreservesJavaEndpointEvaluationOrder(t *testing.T) {
	model := basegraph.NewModel()
	root := model.Root
	sourceNode := model.AddNode(&root)
	targetNode := model.AddNode(&root)
	model.Node(sourceNode).Shape.SetDimensions(103, 101)
	model.Node(targetNode).Shape.SetDimensions(101, 101)
	sourcePort := model.AddPort(sourceNode)
	targetPort := model.AddPort(targetNode)
	edgeID := model.AddEdge(&root)
	model.AddSource(edgeID, basegraph.PortRef(sourcePort))
	model.AddTarget(edgeID, basegraph.PortRef(targetPort))

	importer := NewElkGraphImporter(model)
	arena, graphID, err := importer.ImportGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	// These are the minimized operands from a D2 compound/component layout.
	// Summing graph offset + node + port produces the adjacent lower float;
	// ELK first computes the absolute port anchor, then adds the graph offset.
	arena.Graph(graphID).Padding = geometry.Spacing{}
	arena.Graph(graphID).Offset.X = 167.83333333333334
	arena.Node(importer.Maps.Nodes.At(sourceNode)).Position.X = 8.1666666666666714
	arena.Port(importer.Maps.Ports.At(sourcePort)).Position.X = 68.666666666666657

	if err := NewElkGraphLayoutTransferrer(model, importer.Maps).ApplyLayout(arena, graphID); err != nil {
		t.Fatal(err)
	}
	section := model.Section(model.Edge(edgeID).Sections[0])
	if got, want := section.Start.X, 244.66666666666669; got != want {
		t.Fatalf("source endpoint x = %.17g, want Java/GWT evaluation %.17g", got, want)
	}
}

func TestTransferrerOnlyAppliesNodeLabelLayoutWhenPlacementIsConfigured(t *testing.T) {
	model := basegraph.NewModel()
	root := model.Root
	nodeID := model.AddNode(&root)
	model.Node(nodeID).Shape.SetDimensions(20, 10)
	labelID := model.AddLabel(basegraph.NodeElementRef(nodeID), "label")
	model.Label(labelID).Shape.SetLocation(1, 2)
	model.Label(labelID).Shape.SetDimensions(4, 3)
	importer := NewElkGraphImporter(model)
	arena, graphID, err := importer.ImportGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	lNode := importer.Maps.Nodes.At(nodeID)
	lLabel := importer.Maps.Labels.At(labelID)
	arena.Label(lLabel).Position = geometry.Point{X: 100, Y: 200}

	transferrer := NewElkGraphLayoutTransferrer(model, importer.Maps)
	if err := transferrer.ApplyLayout(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if got := model.Label(labelID).Shape.Position(); got != (geometry.Point{X: 1, Y: 2}) {
		t.Fatalf("fixed label position = %+v, want input position", got)
	}

	basegraph.Set(&arena.Node(lNode).Properties, nodespacing.NodeLabelsPlacement, []string{"INSIDE"})
	if err := transferrer.ApplyLayout(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if got := model.Label(labelID).Shape.Position(); got != (geometry.Point{X: 100, Y: 200}) {
		t.Fatalf("managed label position = %+v", got)
	}
}

func TestTransferrerDoesNotDoubleApplyCrossHierarchyTargetOffset(t *testing.T) {
	model := basegraph.NewModel()
	root := model.Root
	basegraph.Set(&model.Node(root).Properties, core.HierarchyHandling, "INCLUDE_CHILDREN")
	sourceParent := model.AddNode(&root)
	targetParent := model.AddNode(&root)
	basegraph.Set(&model.Node(sourceParent).Properties, core.HierarchyHandling, "INCLUDE_CHILDREN")
	basegraph.Set(&model.Node(targetParent).Properties, core.HierarchyHandling, "INCLUDE_CHILDREN")
	sourceNode := model.AddNode(&sourceParent)
	targetNode := model.AddNode(&targetParent)
	model.Node(sourceNode).Shape.SetDimensions(10, 10)
	model.Node(targetNode).Shape.SetDimensions(10, 10)
	edge := model.AddEdge(&root)
	model.AddSource(edge, basegraph.NodeRef(sourceNode))
	model.AddTarget(edge, basegraph.NodeRef(targetNode))

	importer := NewElkGraphImporter(model)
	arena, graphID, err := importer.ImportGraph(root)
	if err != nil {
		t.Fatal(err)
	}
	arena.Graph(graphID).Padding = geometry.Spacing{}
	sourceParentNode := arena.Node(importer.Maps.Nodes.At(sourceParent))
	targetParentNode := arena.Node(importer.Maps.Nodes.At(targetParent))
	sourceParentNode.Position = geometry.Point{X: 10, Y: 20}
	targetParentNode.Position = geometry.Point{X: 200, Y: 300}
	sourceGraph := *sourceParentNode.NestedGraph
	targetGraph := *targetParentNode.NestedGraph
	arena.Graph(sourceGraph).Offset = geometry.Point{X: 3, Y: 4}
	arena.Graph(sourceGraph).Padding = geometry.Spacing{Left: 5, Top: 6}
	arena.Graph(targetGraph).Offset = geometry.Point{X: 7, Y: 8}
	arena.Graph(targetGraph).Padding = geometry.Spacing{Left: 9, Top: 10}
	arena.Node(importer.Maps.Nodes.At(sourceNode)).Position = geometry.Point{X: 11, Y: 12}
	arena.Node(importer.Maps.Nodes.At(targetNode)).Position = geometry.Point{X: 13, Y: 14}
	lEdge := importer.Maps.Edges.At(edge)
	// CompoundGraphPostprocessor computes TARGET_OFFSET by changing from the
	// target graph's coordinates into the source reference graph.
	basegraph.Set(&arena.Edge(lEdge).Properties, TargetOffset,
		graphCoordinateOffset(arena, targetGraph, sourceGraph))

	if err := NewElkGraphLayoutTransferrer(model, importer.Maps).ApplyLayout(arena, graphID); err != nil {
		t.Fatal(err)
	}
	section := model.Section(model.Edge(edge).Sections[0])
	// Parent position + nested offset/padding + child/port position.
	if section.End != (geometry.Point{X: 229, Y: 332}) {
		t.Fatalf("cross-hierarchy target = %+v, want the absolute target without TARGET_OFFSET", section.End)
	}
}
