// Copyright (c) 2022-2024 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"strings"
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

type topdownProviderFunc func(*graph.Model, graph.NodeID, ProgressMonitor) error

func (provider topdownProviderFunc) Layout(model *graph.Model, parent graph.NodeID, monitor ProgressMonitor) error {
	return provider(model, parent, monitor)
}

func topdownAlgorithm(provider topdownProviderFunc) *AlgorithmData {
	return &AlgorithmData{ID: "org.eclipse.elk.test.topdown", Create: func() LayoutProvider { return provider }}
}

func TestTopdownRootSizing(t *testing.T) {
	model := graph.NewModel()
	root := model.Root
	child := model.AddNode(&root)
	model.Node(child).Shape.SetLocation(10, 20)
	model.Node(child).Shape.SetDimensions(30, 40)
	graph.Set(&model.Node(root).Properties, TopdownNodeType, "ROOT_NODE")
	graph.Set(&model.Node(root).Properties, Padding, geometry.Spacing{Top: 2, Right: 3, Bottom: 4, Left: 5})

	engine := NewEngine(nil, nil)
	if err := engine.layoutTopdown(model, root, topdownAlgorithm(func(*graph.Model, graph.NodeID, ProgressMonitor) error {
		return nil
	}), NullProgressMonitor); err != nil {
		t.Fatal(err)
	}
	if got := model.Node(root).Shape; got.Width != 38 || got.Height != 46 {
		t.Fatalf("root dimensions = (%v,%v), want (38,46)", got.Width, got.Height)
	}
	if got := graph.Get(&model.Node(root).Properties, ChildAreaWidth); got != 30 {
		t.Fatalf("child area width = %v, want 30", got)
	}
	if got := graph.Get(&model.Node(root).Properties, ChildAreaHeight); got != 40 {
		t.Fatalf("child area height = %v, want 40", got)
	}
}

func TestTopdownChildAreaComputedOnlyWhenNeitherDimensionIsSet(t *testing.T) {
	for _, test := range []struct {
		name       string
		setWidth   bool
		setHeight  bool
		wantWidth  float64
		wantHeight float64
	}{
		{name: "neither", wantWidth: 20, wantHeight: 30},
		{name: "width", setWidth: true, wantWidth: 80},
		{name: "height", setHeight: true, wantHeight: 70},
		{name: "both", setWidth: true, setHeight: true, wantWidth: 80, wantHeight: 70},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := graph.NewModel()
			root := model.Root
			child := model.AddNode(&root)
			model.Node(root).Shape.SetDimensions(100, 100)
			model.Node(child).Shape.SetLocation(5, 7)
			model.Node(child).Shape.SetDimensions(20, 30)
			graph.Set(&model.Node(root).Properties, TopdownNodeType, "HIERARCHICAL_NODE")
			graph.Set(&model.Node(root).Properties, Padding, geometry.Spacing{})
			if test.setWidth {
				graph.Set(&model.Node(root).Properties, ChildAreaWidth, 80)
			}
			if test.setHeight {
				graph.Set(&model.Node(root).Properties, ChildAreaHeight, 70)
			}

			engine := NewEngine(nil, nil)
			if err := engine.layoutTopdown(model, root, topdownAlgorithm(func(*graph.Model, graph.NodeID, ProgressMonitor) error {
				return nil
			}), NullProgressMonitor); err != nil {
				t.Fatal(err)
			}
			if got := graph.Get(&model.Node(root).Properties, ChildAreaWidth); got != test.wantWidth {
				t.Fatalf("child area width = %v, want %v", got, test.wantWidth)
			}
			if got := graph.Get(&model.Node(root).Properties, ChildAreaHeight); got != test.wantHeight {
				t.Fatalf("child area height = %v, want %v", got, test.wantHeight)
			}
		})
	}
}

func TestTopdownAlignmentTranslatesCompleteInnerLayout(t *testing.T) {
	tests := []struct {
		name      string
		alignment []string
		want      geometry.Point
	}{
		{name: "top left", alignment: []string{"H_LEFT", "V_TOP"}, want: geometry.Point{X: -5, Y: -5}},
		{name: "center", alignment: []string{"H_CENTER", "V_CENTER"}, want: geometry.Point{X: 5, Y: 0}},
		{name: "bottom right", alignment: []string{"H_RIGHT", "V_BOTTOM"}, want: geometry.Point{X: 15, Y: 5}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := graph.NewModel()
			root := model.Root
			child := model.AddNode(&root)
			model.Node(root).Shape.SetDimensions(100, 80)
			model.Node(child).Shape.SetLocation(1, 2)
			model.Node(child).Shape.SetDimensions(10, 10)
			graph.Set(&model.Node(root).Properties, TopdownNodeType, "HIERARCHICAL_NODE")
			graph.Set(&model.Node(root).Properties, Padding, geometry.UniformSpacing(10))
			graph.Set(&model.Node(root).Properties, ContentAlignment, test.alignment)
			graph.Set(&model.Node(root).Properties, TopdownScaleCap, 2.0)
			graph.Set(&model.Node(root).Properties, ChildAreaWidth, 20.0)
			graph.Set(&model.Node(root).Properties, ChildAreaHeight, 20.0)

			edge := model.AddEdge(&root)
			section := model.AddSection(edge)
			model.Section(section).Start = geometry.Point{X: 3, Y: 4}
			model.Section(section).End = geometry.Point{X: 5, Y: 6}
			model.Section(section).BendPoints = geometry.Chain{{X: 7, Y: 8}}
			label := model.AddLabel(graph.EdgeElementRef(edge), "edge")
			model.Label(label).Shape.SetLocation(9, 10)
			graph.Set(&model.Edge(edge).Properties, JunctionPoints, geometry.Chain{{X: 11, Y: 12}})

			engine := NewEngine(nil, nil)
			if err := engine.layoutTopdown(model, root, topdownAlgorithm(func(*graph.Model, graph.NodeID, ProgressMonitor) error {
				return nil
			}), NullProgressMonitor); err != nil {
				t.Fatal(err)
			}
			assertTopdownPoint(t, "node", model.Node(child).Shape.Position(), geometry.Point{X: 1, Y: 2}.Add(test.want))
			assertTopdownPoint(t, "section start", model.Section(section).Start, geometry.Point{X: 3, Y: 4}.Add(test.want))
			assertTopdownPoint(t, "section end", model.Section(section).End, geometry.Point{X: 5, Y: 6}.Add(test.want))
			assertTopdownPoint(t, "bend point", model.Section(section).BendPoints[0], geometry.Point{X: 7, Y: 8}.Add(test.want))
			assertTopdownPoint(t, "edge label", model.Label(label).Shape.Position(), geometry.Point{X: 9, Y: 10}.Add(test.want))
			junctions := graph.Get(&model.Edge(edge).Properties, JunctionPoints)
			assertTopdownPoint(t, "junction", junctions[0], geometry.Point{X: 11, Y: 12}.Add(test.want))
		})
	}
}

func TestTopdownValidationErrors(t *testing.T) {
	t.Run("missing node type", func(t *testing.T) {
		model := graph.NewModel()
		model.AddNode(&model.Root)
		err := NewEngine(nil, nil).layoutTopdown(model, model.Root, topdownAlgorithm(func(*graph.Model, graph.NodeID, ProgressMonitor) error {
			return nil
		}), NullProgressMonitor)
		if err == nil || !strings.Contains(err.Error(), "has not been assigned a top-down node type") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("negative available area", func(t *testing.T) {
		model := graph.NewModel()
		model.AddNode(&model.Root)
		model.Node(model.Root).Shape.SetDimensions(10, 10)
		graph.Set(&model.Node(model.Root).Properties, TopdownNodeType, "HIERARCHICAL_NODE")
		graph.Set(&model.Node(model.Root).Properties, Padding, geometry.UniformSpacing(6))
		err := NewEngine(nil, nil).layoutTopdown(model, model.Root, topdownAlgorithm(func(*graph.Model, graph.NodeID, ProgressMonitor) error {
			return nil
		}), NullProgressMonitor)
		if err == nil || !strings.Contains(err.Error(), "too small for the space provided by the paddings") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("include children", func(t *testing.T) {
		model := graph.NewModel()
		root := model.Root
		model.AddNode(&root)
		graph.Set(&model.Node(root).Properties, TopdownLayout, true)
		graph.Set(&model.Node(root).Properties, TopdownNodeType, "ROOT_NODE")
		graph.Set(&model.Node(root).Properties, HierarchyHandling, "INCLUDE_CHILDREN")
		algorithms := NewAlgorithmRegistry()
		err := algorithms.Register(AlgorithmData{
			ID: DefaultAlgorithmID, Features: FeatureCompound,
			Create: func() LayoutProvider {
				return topdownProviderFunc(func(*graph.Model, graph.NodeID, ProgressMonitor) error { return nil })
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		err = NewEngine(nil, algorithms).Layout(model)
		if err == nil || !strings.Contains(err.Error(), "Topdown layout cannot be used together with hierarchy handling.") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestTopdownRecursiveOrderAndScaleFactorState(t *testing.T) {
	model := graph.NewModel()
	root := model.Root
	compound := model.AddNode(&root)
	leaf := model.AddNode(&compound)
	model.Node(root).Shape.SetDimensions(100, 100)
	model.Node(compound).Shape.SetDimensions(40, 40)
	model.Node(leaf).Shape.SetDimensions(20, 20)
	for node, nodeType := range map[graph.NodeID]string{root: "ROOT_NODE", compound: "HIERARCHICAL_NODE"} {
		graph.Set(&model.Node(node).Properties, TopdownLayout, true)
		graph.Set(&model.Node(node).Properties, TopdownNodeType, nodeType)
		graph.Set(&model.Node(node).Properties, Padding, geometry.Spacing{})
	}
	graph.Set(&model.Node(compound).Properties, TopdownHierarchicalNodeWidth, 40.0)
	graph.Set(&model.Node(compound).Properties, TopdownHierarchicalNodeAspectRatio, 1.0)
	graph.Set(&model.Node(compound).Properties, TopdownScaleFactor, 99.0)

	var order []graph.NodeID
	provider := topdownProviderFunc(func(model *graph.Model, parent graph.NodeID, _ ProgressMonitor) error {
		order = append(order, parent)
		if parent == compound {
			model.Node(leaf).Shape.SetLocation(0, 0)
			graph.Set(&model.Node(parent).Properties, ChildAreaWidth, 80.0)
			graph.Set(&model.Node(parent).Properties, ChildAreaHeight, 80.0)
		}
		return nil
	})
	algorithms := NewAlgorithmRegistry()
	if err := algorithms.Register(AlgorithmData{ID: DefaultAlgorithmID, Create: func() LayoutProvider { return provider }}); err != nil {
		t.Fatal(err)
	}
	if err := NewEngine(nil, algorithms).Layout(model); err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != root || order[1] != compound {
		t.Fatalf("provider order = %v, want [%v %v]", order, root, compound)
	}
	if got := graph.Get(&model.Node(compound).Properties, TopdownScaleFactor); got != .5 {
		t.Fatalf("computed topdown scale factor = %v, want .5", got)
	}
	if got := model.Node(leaf).Shape; got.Width != 20 || got.Height != 20 {
		t.Fatalf("topdown scale factor changed geometry: %#v", got)
	}
}

func TestConfiguredNodeScalingStaysDistinctFromTopdownScaleFactor(t *testing.T) {
	model := graph.NewModel()
	root := model.Root
	nodeID := model.AddNode(&root)
	childID := model.AddNode(&nodeID)
	node := model.Node(nodeID)
	node.Shape.SetLocation(7, 8)
	node.Shape.SetDimensions(30, 40)
	model.Node(childID).Shape.SetLocation(11, 12)
	model.Node(childID).Shape.SetDimensions(13, 14)
	nodeLabelID := model.AddLabel(graph.NodeElementRef(nodeID), "node")
	model.Label(nodeLabelID).Shape = graph.Shape{X: 1, Y: 2, Width: 3, Height: 4}
	portID := model.AddPort(nodeID)
	port := model.Port(portID)
	port.Shape = graph.Shape{X: 5, Y: 6, Width: 7, Height: 8}
	graph.Set(&port.Properties, PortAnchor, geometry.Point{X: 2, Y: 3})
	portLabelID := model.AddLabel(graph.PortElementRef(portID), "port")
	model.Label(portLabelID).Shape = graph.Shape{X: 9, Y: 10, Width: 11, Height: 12}
	edgeID := model.AddEdge(&nodeID)
	sectionID := model.AddSection(edgeID)
	model.Section(sectionID).Start = geometry.Point{X: 15, Y: 16}

	graph.Set(&node.Properties, ScaleFactor, 2.0)
	graph.Set(&node.Properties, TopdownScaleFactor, 0.25)
	applyScale(model, nodeID)

	if node.Shape.X != 7 || node.Shape.Y != 8 || node.Shape.Width != 60 || node.Shape.Height != 80 {
		t.Fatalf("scaled node shape = %#v", node.Shape)
	}
	if got := model.Label(nodeLabelID).Shape; got != (graph.Shape{X: 2, Y: 4, Width: 6, Height: 8}) {
		t.Fatalf("scaled node label = %#v", got)
	}
	if got := port.Shape; got != (graph.Shape{X: 10, Y: 12, Width: 14, Height: 16}) {
		t.Fatalf("scaled port = %#v", got)
	}
	if got := graph.Get(&port.Properties, PortAnchor); got != (geometry.Point{X: 4, Y: 6}) {
		t.Fatalf("scaled port anchor = %#v", got)
	}
	if got := model.Label(portLabelID).Shape; got != (graph.Shape{X: 18, Y: 20, Width: 22, Height: 24}) {
		t.Fatalf("scaled port label = %#v", got)
	}
	if got := model.Node(childID).Shape; got.X != 11 || got.Y != 12 || got.Width != 13 || got.Height != 14 {
		t.Fatalf("child geometry was scaled = %#v", got)
	}
	if got := model.Section(sectionID).Start; got != (geometry.Point{X: 15, Y: 16}) {
		t.Fatalf("contained edge was scaled = %#v", got)
	}
	if got := graph.Get(&node.Properties, TopdownScaleFactor); got != .25 {
		t.Fatalf("topdown scale factor changed = %v", got)
	}
}

func assertTopdownPoint(t *testing.T, name string, got, want geometry.Point) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}
