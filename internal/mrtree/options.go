// Copyright (c) 2015, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package mrtree is a native Go port of ELK Mr. Tree targeting ELK.js 0.12.0.
package mrtree

import (
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

const AlgorithmID = "org.eclipse.elk.mrtree"

var (
	Padding                     = graph.CloneablePropertyWithDefault("org.eclipse.elk.padding", func() geometry.Spacing { return geometry.UniformSpacing(20) })
	SpacingNodeNode             = graph.PropertyWithDefault("org.eclipse.elk.spacing.nodeNode", func() float64 { return 20 })
	AspectRatio                 = graph.PropertyWithDefault("org.eclipse.elk.aspectRatio", func() float64 { return 1.6 })
	Priority                    = graph.PropertyWithDefault("org.eclipse.elk.priority", func() int { return 1 })
	SeparateConnectedComponents = graph.PropertyWithDefault("org.eclipse.elk.separateConnectedComponents", func() bool { return true })
	OmitNodeMicroLayout         = graph.PropertyWithDefault("org.eclipse.elk.omitNodeMicroLayout", func() bool { return false })
	SpacingEdgeNode             = graph.PropertyWithDefault("org.eclipse.elk.spacing.edgeNode", func() float64 { return 3 })
	Direction                   = graph.PropertyWithDefault("org.eclipse.elk.direction", func() string { return "UNDEFINED" })
	Interactive                 = graph.PropertyWithDefault("org.eclipse.elk.interactive", func() bool { return false })
	Weighting                   = graph.PropertyWithDefault("org.eclipse.elk.mrtree.weighting", func() string { return "MODEL_ORDER" })
	SearchOrder                 = graph.PropertyWithDefault("org.eclipse.elk.mrtree.searchOrder", func() string { return "DFS" })
	PositionConstraint          = graph.PropertyWithDefault("org.eclipse.elk.mrtree.positionConstraint", func() int { return -1 })
	EdgeRoutingMode             = graph.PropertyWithDefault("org.eclipse.elk.mrtree.edgeRoutingMode", func() string { return "AVOID_OVERLAP" })
	TreeLevel                   = graph.PropertyWithDefault("org.eclipse.elk.mrtree.treeLevel", func() int { return 0 })
	Compaction                  = graph.PropertyWithDefault("org.eclipse.elk.mrtree.compaction", func() bool { return false })
	EdgeEndTextureLength        = graph.PropertyWithDefault("org.eclipse.elk.mrtree.edgeEndTextureLength", func() float64 { return 7 })
	NodeSizeFixedGraphSize      = graph.PropertyWithDefault("org.eclipse.elk.nodeSize.fixedGraphSize", func() bool { return false })
)

// RegisterOptions installs the metadata generated from Tree.melk in ELK
// 0.12.0. Re-exported core options are deliberately registered only when the
// shared registry does not already contain their identifier.
func RegisterOptions(registry *core.OptionRegistry) {
	if registry == nil {
		return
	}
	options := []core.OptionData{
		{ID: NodeSizeFixedGraphSize.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: AspectRatio.ID, Kind: core.DoubleOption, Targets: core.Parents},
		{ID: Priority.ID, Kind: core.IntegerOption, Targets: core.Nodes | core.Edges},
		{ID: SeparateConnectedComponents.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: OmitNodeMicroLayout.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: Weighting.ID, Kind: core.EnumOption, Targets: core.Parents,
			EnumValues: []string{"MODEL_ORDER", "DESCENDANTS", "FAN", "CONSTRAINT"}},
		{ID: SearchOrder.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: []string{"DFS", "BFS"}},
		{ID: PositionConstraint.ID, Kind: core.IntegerOption, Targets: core.Nodes},
		{ID: EdgeRoutingMode.ID, Kind: core.EnumOption, Targets: core.Parents,
			EnumValues: []string{"NONE", "MIDDLE_TO_MIDDLE", "AVOID_OVERLAP"}},
		{ID: TreeLevel.ID, Kind: core.IntegerOption, Targets: core.Nodes},
		{ID: Compaction.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: EdgeEndTextureLength.ID, Kind: core.DoubleOption, Targets: core.Parents},
	}
	for _, option := range options {
		if registry.ByID(option.ID) != nil {
			continue
		}
		if err := registry.Register(option); err != nil {
			panic(err)
		}
	}
}
