// Copyright (c) 2010, 2015 Kiel University and others.
// Copyright (c) 2012, 2015 Kiel University and others.
// Copyright (c) 2015, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package basicoptions contains the core options whose defaults are
// specialized by ELK's fixed, box, and random utility algorithms.
package basicoptions

import (
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

var (
	FixedPadding = graph.CloneablePropertyWithDefault("org.eclipse.elk.padding", func() geometry.Spacing {
		return geometry.UniformSpacing(15)
	})
	BoxPadding = graph.CloneablePropertyWithDefault("org.eclipse.elk.padding", func() geometry.Spacing {
		return geometry.UniformSpacing(15)
	})
	RandomPadding = graph.CloneablePropertyWithDefault("org.eclipse.elk.padding", func() geometry.Spacing {
		return geometry.UniformSpacing(15)
	})

	Position    = graph.NewProperty[geometry.Point]("org.eclipse.elk.position")
	BendPoints  = graph.NewProperty[geometry.Chain]("org.eclipse.elk.bendPoints")
	EdgeRouting = graph.PropertyWithDefault("org.eclipse.elk.edgeRouting", func() string { return "UNDEFINED" })

	NodeSizeFixedGraphSize = graph.PropertyWithDefault("org.eclipse.elk.nodeSize.fixedGraphSize", func() bool { return false })
	NodeSizeOptions        = graph.CloneablePropertyWithDefault("org.eclipse.elk.nodeSize.options", func() []string {
		return []string{"DEFAULT_MINIMUM_SIZE"}
	})
	ExpandNodes = graph.PropertyWithDefault("org.eclipse.elk.expandNodes", func() bool { return false })
	Interactive = graph.PropertyWithDefault("org.eclipse.elk.interactive", func() bool { return false })
	Priority    = graph.PropertyWithDefault("org.eclipse.elk.priority", func() int { return 0 })
	PackingMode = graph.PropertyWithDefault("org.eclipse.elk.box.packingMode", func() string { return "SIMPLE" })

	// The source metadata spells these defaults as float literals. The value is
	// widened to double only after Java has rounded it to float.
	BoxAspectRatio = graph.PropertyWithDefault("org.eclipse.elk.aspectRatio", func() float64 {
		return float64(float32(1.3))
	})
	RandomAspectRatio = graph.PropertyWithDefault("org.eclipse.elk.aspectRatio", func() float64 {
		return float64(float32(1.6))
	})
	BoxSpacingNodeNode    = graph.PropertyWithDefault("org.eclipse.elk.spacing.nodeNode", func() float64 { return 15 })
	RandomSpacingNodeNode = graph.PropertyWithDefault("org.eclipse.elk.spacing.nodeNode", func() float64 { return 15 })
	RandomSeed            = graph.PropertyWithDefault("org.eclipse.elk.randomSeed", func() int { return 0 })
)

// RegisterOptions adds the metadata consumed by the three utility providers.
// Core and Layered may already have registered shared identifiers, so this is
// deliberately idempotent by canonical option ID.
func RegisterOptions(registry *core.OptionRegistry) {
	if registry == nil {
		return
	}
	options := []core.OptionData{
		{ID: Position.ID, Kind: core.ObjectOption, Targets: core.Nodes | core.Ports | core.Labels,
			Parse: func(value string) (any, error) { return geometry.ParsePoint(value) }},
		{ID: BendPoints.ID, Kind: core.ObjectOption, Targets: core.Edges,
			Parse: func(value string) (any, error) { return geometry.ParseChain(value) }},
		{ID: EdgeRouting.ID, Kind: core.EnumOption, Targets: core.Parents,
			EnumValues: []string{"UNDEFINED", "POLYLINE", "ORTHOGONAL", "SPLINES"}},
		{ID: NodeSizeFixedGraphSize.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: NodeSizeOptions.ID, Kind: core.EnumSetOption, Targets: core.Nodes,
			EnumValues: []string{"DEFAULT_MINIMUM_SIZE", "MINIMUM_SIZE_ACCOUNTS_FOR_PADDING", "COMPUTE_PADDING", "OUTSIDE_NODE_LABELS_OVERHANG", "PORTS_OVERHANG", "UNIFORM_PORT_SPACING", "SPACE_EFFICIENT_PORT_LABELS", "FORCE_TABULAR_NODE_LABELS", "ASYMMETRICAL"}},
		{ID: ExpandNodes.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: Interactive.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: Priority.ID, Kind: core.IntegerOption, Targets: core.Nodes | core.Edges},
		{ID: PackingMode.ID, Kind: core.EnumOption, Targets: core.Parents,
			EnumValues: []string{"SIMPLE", "GROUP_DEC", "GROUP_MIXED", "GROUP_INC"}},
		{ID: BoxAspectRatio.ID, Kind: core.DoubleOption, Targets: core.Parents},
		{ID: RandomSeed.ID, Kind: core.IntegerOption, Targets: core.Parents},
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
