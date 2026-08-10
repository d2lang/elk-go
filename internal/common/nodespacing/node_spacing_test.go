// Copyright (c) 2010, 2020 Kiel University and others.
// Copyright (c) 2014, 2015 Kiel University and others.
// Copyright (c) 2017, 2019 Kiel University and others.
// SPDX-License-Identifier: EPL-2.0

package nodespacing

import (
	"testing"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

func TestFixedSizePortPlacementAndMargins(t *testing.T) {
	model := graph.NewModel()
	root := model.Root
	nodeID := model.AddNode(&root)
	node := model.Node(nodeID)
	node.Shape.SetLocation(10, 20)
	node.Shape.SetDimensions(30, 30)
	graph.Set(&node.Properties, core.PortConstraints, "FIXED_POS")

	west := model.AddPort(nodeID)
	east := model.AddPort(nodeID)
	graph.Set(&model.Port(west).Properties, core.PortSide, "WEST")
	graph.Set(&model.Port(east).Properties, core.PortSide, "EAST")
	model.Port(west).Shape.Y = 15
	model.Port(east).Shape.Y = 15

	got := ProcessNodeSize(model, root, nodeID, true, false)
	if got != (geometry.Point{X: 30, Y: 30}) {
		t.Fatalf("size = %v", got)
	}
	if model.Port(west).Shape.X != 0 || model.Port(east).Shape.X != 30 || model.Port(west).Shape.Y != 15 || model.Port(east).Shape.Y != 15 {
		t.Fatalf("ports = west:%+v east:%+v", model.Port(west).Shape, model.Port(east).Shape)
	}
	CalculateNodeMargins(model, root, true)
	if margin := graph.Get(&node.Properties, Margins); margin != (geometry.Spacing{}) {
		t.Fatalf("margin = %+v", margin)
	}
}

func TestMinimumSizeAndCenteredLabel(t *testing.T) {
	model := graph.NewModel()
	root := model.Root
	nodeID := model.AddNode(&root)
	node := model.Node(nodeID)
	node.Shape.SetDimensions(100, 80)
	graph.Set(&node.Properties, core.NodeSizeConstraints, []string{"MINIMUM_SIZE"})
	graph.Set(&node.Properties, core.NodeSizeMinimum, geometry.Point{X: 40, Y: 20})
	graph.Set(&node.Properties, NodeLabelsPlacement, []string{"INSIDE", "H_CENTER", "V_CENTER"})
	labelID := model.AddLabel(graph.NodeElementRef(nodeID), "label")
	model.Label(labelID).Shape.SetDimensions(10, 4)
	if got := ProcessNodeSize(model, root, nodeID, true, false); got != (geometry.Point{X: 40, Y: 20}) {
		t.Fatalf("size = %v", got)
	}
	if got := model.Label(labelID).Shape.Position(); got != (geometry.Point{X: 15, Y: 8}) {
		t.Fatalf("label position = %v", got)
	}
}

func TestNodeLabelConstraintSizesAndPlacesInsideLabel(t *testing.T) {
	model := graph.NewModel()
	root := model.Root
	nodeID := model.AddNode(&root)
	node := model.Node(nodeID)
	node.Shape.SetDimensions(10, 10)
	graph.Set(&node.Properties, core.NodeSizeConstraints, []string{"NODE_LABELS"})
	graph.Set(&node.Properties, NodeLabelsPlacement, []string{"INSIDE", "H_CENTER", "V_CENTER"})
	labelID := model.AddLabel(graph.NodeElementRef(nodeID), "wide")
	model.Label(labelID).Shape.SetDimensions(88, 18)

	if got := ProcessNodeSize(model, root, nodeID, true, false); got != (geometry.Point{X: 98, Y: 28}) {
		t.Fatalf("size = %v", got)
	}
	if got := model.Label(labelID).Shape.Position(); got != (geometry.Point{X: 5, Y: 5}) {
		t.Fatalf("label position = %v", got)
	}
}

func TestMarginIncludesOutsideElements(t *testing.T) {
	model := graph.NewModel()
	root := model.Root
	nodeID := model.AddNode(&root)
	node := model.Node(nodeID)
	node.Shape.SetLocation(10, 20)
	node.Shape.SetDimensions(30, 20)
	labelID := model.AddLabel(graph.NodeElementRef(nodeID), "outside")
	label := model.Label(labelID)
	label.Shape.SetLocation(-7, -3)
	label.Shape.SetDimensions(5, 2)
	portID := model.AddPort(nodeID)
	port := model.Port(portID)
	port.Shape.SetLocation(30, 8)
	port.Shape.SetDimensions(4, 4)

	NewNodeMarginCalculator().ExcludeEdgeHeadTailLabels().Process(model, root)
	if got, want := graph.Get(&node.Properties, Margins), (geometry.Spacing{Top: 3, Right: 4, Left: 7}); got != want {
		t.Fatalf("margin = %+v, want %+v", got, want)
	}
}
