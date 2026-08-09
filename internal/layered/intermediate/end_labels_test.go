// Copyright (c) 2010, 2019 Kiel University and others.
// Copyright (c) 2012, 2020 Kiel University and others.
// Copyright (c) 2015, 2017 Kiel University and others.
// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2017, 2019 Kiel University and others.
// Copyright (c) 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestEndLabelPreAndPostprocessorPlaceHorizontalEndLabels(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	basegraph.Set(&arena.Graph(graphID).Properties, layered.DirectionOption, layered.DirectionRight)
	basegraph.Set(&arena.Graph(graphID).Properties, layered.SpacingEdgeLabel, 2.0)
	basegraph.Set(&arena.Graph(graphID).Properties, layered.SpacingLabelLabel, 3.0)

	source, target := arena.AddNode(graphID), arena.AddNode(graphID)
	arena.Node(source).Size = geometry.Point{X: 100, Y: 50}
	arena.Node(target).Size = geometry.Point{X: 100, Y: 50}
	sourceLayer, targetLayer := arena.AddLayerToGraph(graphID), arena.AddLayerToGraph(graphID)
	arena.SetNodeLayer(source, &sourceLayer)
	arena.SetNodeLayer(target, &targetLayer)

	sourcePort, targetPort := arena.AddPort(), arena.AddPort()
	arena.SetPortNode(sourcePort, &source)
	arena.SetPortNode(targetPort, &target)
	arena.SetPortSide(sourcePort, layered.EastSide)
	arena.SetPortSide(targetPort, layered.WestSide)
	arena.Port(sourcePort).Position = geometry.Point{X: 100, Y: 20}
	arena.Port(targetPort).Position = geometry.Point{X: 0, Y: 30}

	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &sourcePort)
	arena.SetEdgeTarget(edgeID, &targetPort)
	tailLabel, headLabel := arena.AddLabel("tail"), arena.AddLabel("head")
	arena.Label(tailLabel).Size = geometry.Point{X: 20, Y: 10}
	arena.Label(headLabel).Size = geometry.Point{X: 15, Y: 8}
	basegraph.Set(&arena.Label(tailLabel).Properties, layered.EdgeLabelsPlacement, layered.EdgeLabelPlacementTail)
	basegraph.Set(&arena.Label(headLabel).Properties, layered.EdgeLabelsPlacement, layered.EdgeLabelPlacementHead)
	basegraph.Set(&arena.Label(tailLabel).Properties, layered.InternalLabelSide, layered.LabelSideAbove)
	basegraph.Set(&arena.Label(headLabel).Properties, layered.InternalLabelSide, layered.LabelSideBelow)
	arena.Edge(edgeID).Labels = []layered.LLabelID{tailLabel, headLabel}

	PreprocessEndLabels(arena, graphID)

	if got := arena.Node(source).Margin; got != (geometry.Spacing{Right: 22}) {
		t.Fatalf("source margin = %+v, want right 22", got)
	}
	if got := arena.Node(target).Margin; got != (geometry.Spacing{Left: 17}) {
		t.Fatalf("target margin = %+v, want left 17", got)
	}
	if _, found := basegraph.TryGet(&arena.Node(source).Properties, endLabelCells); !found {
		t.Fatal("source has no end-label cell after preprocessing")
	}

	arena.Node(source).Position = geometry.Point{X: 10, Y: 20}
	arena.Node(target).Position = geometry.Point{X: 200, Y: 30}
	PostprocessEndLabels(arena, graphID)

	if got := arena.Label(tailLabel).Position; got != (geometry.Point{X: 112, Y: 27}) {
		t.Fatalf("tail label position = %v, want (112,27)", got)
	}
	if got := arena.Label(headLabel).Position; got != (geometry.Point{X: 183, Y: 63}) {
		t.Fatalf("head label position = %v, want (183,63)", got)
	}
	if basegraph.Has(&arena.Node(source).Properties, endLabelCells) {
		t.Fatal("postprocessor retained source end-label cells")
	}
}

func TestEndLabelPreAndPostprocessorUseVerticalLabelCells(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	basegraph.Set(&arena.Graph(graphID).Properties, layered.DirectionOption, layered.DirectionDown)
	basegraph.Set(&arena.Graph(graphID).Properties, layered.SpacingEdgeLabel, 2.0)
	basegraph.Set(&arena.Graph(graphID).Properties, layered.SpacingLabelLabel, 3.0)

	source, target := arena.AddNode(graphID), arena.AddNode(graphID)
	arena.Node(source).Size = geometry.Point{X: 100, Y: 50}
	sourceLayer, targetLayer := arena.AddLayerToGraph(graphID), arena.AddLayerToGraph(graphID)
	arena.SetNodeLayer(source, &sourceLayer)
	arena.SetNodeLayer(target, &targetLayer)
	sourcePort, targetPort := arena.AddPort(), arena.AddPort()
	arena.SetPortNode(sourcePort, &source)
	arena.SetPortNode(targetPort, &target)
	arena.SetPortSide(sourcePort, layered.SouthSide)
	arena.SetPortSide(targetPort, layered.NorthSide)
	arena.Port(sourcePort).Position = geometry.Point{X: 30, Y: 50}

	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &sourcePort)
	arena.SetEdgeTarget(edgeID, &targetPort)
	first, second := arena.AddLabel("first"), arena.AddLabel("second")
	arena.Label(first).Size = geometry.Point{X: 10, Y: 4}
	arena.Label(second).Size = geometry.Point{X: 20, Y: 6}
	for _, labelID := range []layered.LLabelID{first, second} {
		basegraph.Set(&arena.Label(labelID).Properties, layered.EdgeLabelsPlacement, layered.EdgeLabelPlacementTail)
		basegraph.Set(&arena.Label(labelID).Properties, layered.InternalLabelSide, layered.LabelSideAbove)
	}
	arena.Edge(edgeID).Labels = []layered.LLabelID{first, second}

	PreprocessEndLabels(arena, graphID)
	if got := arena.Node(source).Margin; got != (geometry.Spacing{Bottom: 8, Left: 6}) {
		t.Fatalf("source margin = %+v, want bottom 8 and left 6", got)
	}
	arena.Node(source).Position = geometry.Point{X: 10, Y: 20}
	PostprocessEndLabels(arena, graphID)

	if got := arena.Label(first).Position; got != (geometry.Point{X: 4, Y: 72}) {
		t.Fatalf("first label position = %v, want (4,72)", got)
	}
	if got := arena.Label(second).Position; got != (geometry.Point{X: 17, Y: 72}) {
		t.Fatalf("second label position = %v, want (17,72)", got)
	}
}

func TestEndLabelPreprocessorRemovesNorthSideOverlaps(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	basegraph.Set(&arena.Graph(graphID).Properties, layered.DirectionOption, layered.DirectionRight)
	basegraph.Set(&arena.Graph(graphID).Properties, layered.SpacingEdgeLabel, 2.0)
	basegraph.Set(&arena.Graph(graphID).Properties, layered.SpacingLabelLabel, 3.0)

	node := arena.AddNode(graphID)
	arena.Node(node).Size = geometry.Point{X: 100, Y: 50}
	layer := arena.AddLayerToGraph(graphID)
	arena.SetNodeLayer(node, &layer)
	firstPort, secondPort := arena.AddPort(), arena.AddPort()
	for _, portID := range []layered.LPortID{firstPort, secondPort} {
		arena.SetPortNode(portID, &node)
		arena.SetPortSide(portID, layered.NorthSide)
	}
	arena.Port(firstPort).Position.X = 30
	arena.Port(secondPort).Position.X = 35

	for index, portID := range []layered.LPortID{firstPort, secondPort} {
		remote := arena.AddNode(graphID)
		remotePort := arena.AddPort()
		arena.SetPortNode(remotePort, &remote)
		edgeID := arena.AddEdge()
		arena.SetEdgeSource(edgeID, &portID)
		arena.SetEdgeTarget(edgeID, &remotePort)
		labelID := arena.AddLabel("label")
		arena.Label(labelID).Size = geometry.Point{X: 20, Y: 10}
		basegraph.Set(&arena.Label(labelID).Properties, layered.EdgeLabelsPlacement, layered.EdgeLabelPlacementTail)
		basegraph.Set(&arena.Label(labelID).Properties, layered.InternalLabelSide, layered.LabelSideBelow)
		arena.Edge(edgeID).Labels = []layered.LLabelID{labelID}
		_ = index
	}

	PreprocessEndLabels(arena, graphID)
	cells, found := basegraph.TryGet(&arena.Node(node).Properties, endLabelCells)
	if !found {
		t.Fatal("node has no end-label cells")
	}
	if got := cells[firstPort].rect.Y; got != -12 {
		t.Fatalf("first cell y = %v, want -12", got)
	}
	if got := cells[secondPort].rect.Y; got != -28 {
		t.Fatalf("second cell y = %v, want -28", got)
	}
	if got := arena.Node(node).Margin.Top; got != 28 {
		t.Fatalf("node top margin = %v, want 28", got)
	}
}

func TestEndLabelSorterOrdersGroupsBySourcePort(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	basegraph.Set(&arena.Graph(graphID).Properties, layered.DirectionOption, layered.DirectionRight)

	firstSource, secondSource, target := arena.AddNode(graphID), arena.AddNode(graphID), arena.AddNode(graphID)
	sourceLayer, targetLayer := arena.AddLayerToGraph(graphID), arena.AddLayerToGraph(graphID)
	arena.SetNodeLayer(firstSource, &sourceLayer)
	arena.SetNodeLayer(secondSource, &sourceLayer)
	arena.SetNodeLayer(target, &targetLayer)
	firstSourcePort, secondSourcePort, targetPort := arena.AddPort(), arena.AddPort(), arena.AddPort()
	arena.SetPortNode(firstSourcePort, &firstSource)
	arena.SetPortNode(secondSourcePort, &secondSource)
	arena.SetPortNode(targetPort, &target)
	arena.SetPortSide(firstSourcePort, layered.EastSide)
	arena.SetPortSide(secondSourcePort, layered.EastSide)
	arena.SetPortSide(targetPort, layered.WestSide)

	// Insert the second source's edge first, so gathering and sorting orders
	// differ and the source-port comparator is observable.
	secondEdge := arena.AddEdge()
	arena.SetEdgeSource(secondEdge, &secondSourcePort)
	arena.SetEdgeTarget(secondEdge, &targetPort)
	secondLabel := arena.AddLabel("second")
	arena.Label(secondLabel).Size = geometry.Point{X: 20, Y: 10}
	basegraph.Set(&arena.Label(secondLabel).Properties, layered.EdgeLabelsPlacement, layered.EdgeLabelPlacementHead)
	basegraph.Set(&arena.Label(secondLabel).Properties, layered.InternalLabelSide, layered.LabelSideAbove)
	arena.Edge(secondEdge).Labels = []layered.LLabelID{secondLabel}

	firstEdge := arena.AddEdge()
	arena.SetEdgeSource(firstEdge, &firstSourcePort)
	arena.SetEdgeTarget(firstEdge, &targetPort)
	firstLabel := arena.AddLabel("first")
	arena.Label(firstLabel).Size = geometry.Point{X: 20, Y: 10}
	basegraph.Set(&arena.Label(firstLabel).Properties, layered.EdgeLabelsPlacement, layered.EdgeLabelPlacementHead)
	basegraph.Set(&arena.Label(firstLabel).Properties, layered.InternalLabelSide, layered.LabelSideAbove)
	arena.Edge(firstEdge).Labels = []layered.LLabelID{firstLabel}

	PreprocessEndLabels(arena, graphID)
	cells, _ := basegraph.TryGet(&arena.Node(target).Properties, endLabelCells)
	if got := cells[targetPort].labels; len(got) != 2 || got[0] != secondLabel || got[1] != firstLabel {
		t.Fatalf("gathered labels = %v, want [%v %v]", got, secondLabel, firstLabel)
	}

	SortEndLabels(arena, graphID)
	if got := cells[targetPort].labels; len(got) != 2 || got[0] != firstLabel || got[1] != secondLabel {
		t.Fatalf("sorted labels = %v, want [%v %v]", got, firstLabel, secondLabel)
	}
}

func TestManageEndNodePortLabelsWithoutJavaManagerIsNoop(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	ManageEndNodePortLabels(arena, graphID)
}

func TestCalculateInnermostNodeMarginsExcludesEndLabelsAndIncludesPortLabels(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	node := arena.AddNode(graphID)
	arena.Node(node).Position = geometry.Point{X: 10, Y: 20}
	arena.Node(node).Size = geometry.Point{X: 100, Y: 50}
	// Simulate the public-graph margin pass having included an edge end label.
	arena.Node(node).Margin = geometry.Spacing{Right: 25}
	layer := arena.AddLayerToGraph(graphID)
	arena.SetNodeLayer(node, &layer)

	port := arena.AddPort()
	arena.SetPortNode(port, &node)
	arena.Port(port).Position = geometry.Point{X: 100, Y: 15}
	portLabel := arena.AddLabel("port")
	arena.Label(portLabel).Position = geometry.Point{X: 2, Y: -5}
	arena.Label(portLabel).Size = geometry.Point{X: 20, Y: 10}
	arena.Port(port).Labels = []layered.LLabelID{portLabel}

	CalculateInnermostNodeMargins(arena, graphID)

	if got := arena.Node(node).Margin; got != (geometry.Spacing{Right: 22}) {
		t.Fatalf("node margin = %+v, want right 22", got)
	}
}
