// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package radial

import (
	"fmt"

	"github.com/d2lang/elk-go/internal/basicoptions"
	"github.com/d2lang/elk-go/internal/box"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

// Radial normally starts at a single root. If a projected hierarchy edge is
// in a different tree, laying out only that first root leaves its siblings
// overlapping. Lay out the actual trees independently and pack their bounds
// with the native Box provider. Each tree keeps its Radial geometry.
func layoutForest(model *graph.Model, parent graph.NodeID, trees [][]graph.NodeID, monitor core.ProgressMonitor) error {
	node := model.Node(parent)
	children, shape := node.Children, node.Shape
	defer func() { node.Children = children }()
	padding := graph.Get(&node.Properties, Padding)
	radius := graph.Get(&node.Properties, Radius)
	if radius == 0 {
		graph.Set(&node.Properties, Radius, largestNode(model, parent))
	}
	pack := graph.NewModel()
	props := &pack.Node(pack.Root).Properties
	graph.Set(props, basicoptions.BoxPadding, padding)
	graph.Set(props, basicoptions.BoxSpacingNodeNode, graph.Get(&node.Properties, SpacingNodeNode))
	graph.Set(props, basicoptions.PackingMode, "SIMPLE")
	placements := make([]graph.NodeID, 0, len(trees))
	for _, tree := range trees {
		node.Children = tree
		node.Shape = shape
		if err := layoutTree(model, parent, monitor); err != nil {
			return err
		}
		id := pack.AddNode(&pack.Root)
		pack.Node(id).Shape.SetDimensions(graph.Get(&node.Properties, core.ChildAreaWidth), graph.Get(&node.Properties, core.ChildAreaHeight))
		placements = append(placements, id)
	}
	if err := new(box.Provider).Layout(pack, pack.Root, monitor); err != nil {
		return err
	}
	size := pack.Node(pack.Root).Shape
	if graph.Get(&node.Properties, basicoptions.NodeSizeFixedGraphSize) {
		if size.Width > shape.Width+1e-8 || size.Height > shape.Height+1e-8 {
			return fmt.Errorf("radial tree components require %.2f x %.2f, exceeding fixed parent %q (%.2f x %.2f)", size.Width, size.Height, node.Identifier, shape.Width, shape.Height)
		}
	} else {
		node.Shape.SetDimensions(size.Width, size.Height)
	}
	graph.Set(&node.Properties, core.ChildAreaWidth, size.Width-padding.Horizontal())
	graph.Set(&node.Properties, core.ChildAreaHeight, size.Height-padding.Vertical())
	offsets := make(map[graph.NodeID]geometry.Point, len(children))
	for i, tree := range trees {
		at := pack.Node(placements[i]).Shape.Position().Sub(geometry.Point{X: padding.Left, Y: padding.Top})
		for _, child := range tree {
			s := &model.Node(child).Shape
			s.SetLocation(s.X+at.X, s.Y+at.Y)
			offsets[child] = at
		}
	}
	// Child-internal geometry is already relative to its owner. Only
	// peer-edge sections and labels use the packed parent's coordinates.
	for _, id := range node.ContainedEdges {
		edge := model.Edge(id)
		if len(edge.Sources) != 1 {
			continue
		}
		at, ok := offsets[model.ShapeNode(edge.Sources[0])]
		if !ok {
			continue
		}
		for _, section := range edge.Sections {
			core.TranslateSection(model, section, at)
		}
		for _, label := range edge.Labels {
			s := &model.Label(label).Shape
			if s.Presence&(graph.HasX|graph.HasY) != 0 {
				s.SetLocation(s.X+at.X, s.Y+at.Y)
			}
		}
	}
	return nil
}
