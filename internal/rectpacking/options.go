// Copyright (c) 2018, 2020 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package rectpacking

import (
	"github.com/d2lang/elk-go/internal/basicoptions"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

var (
	AspectRatio = graph.PropertyWithDefault("org.eclipse.elk.aspectRatio", func() float64 { return 1.3 })
	ExpandNodes = graph.PropertyWithDefault("org.eclipse.elk.expandNodes", func() bool { return false })
	Padding     = graph.CloneablePropertyWithDefault("org.eclipse.elk.padding", func() geometry.Spacing {
		return geometry.UniformSpacing(15)
	})
	SpacingNodeNode = graph.PropertyWithDefault("org.eclipse.elk.spacing.nodeNode", func() float64 { return 15 })
	Interactive     = graph.PropertyWithDefault("org.eclipse.elk.interactive", func() bool { return false })

	OptimizationGoal   = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.optimizationGoal", func() string { return "MAX_SCALE_DRIVEN" })
	LastPlaceShift     = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.lastPlaceShift", func() bool { return true })
	CurrentPosition    = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.currentPosition", func() int { return -1 })
	DesiredPosition    = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.desiredPosition", func() int { return -1 })
	OnlyFirstIteration = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.onlyFirstIteration", func() bool { return false })
	RowCompaction      = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.rowCompaction", func() bool { return true })
	ExpandToAspect     = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.expandToAspectRatio", func() bool { return false })
	TargetWidth        = graph.PropertyWithDefault("org.eclipse.elk.rectpacking.targetWidth", func() float64 { return -1 })
)

// RegisterOptions registers the metadata declared by RectPacking.melk in ELK
// 0.8.1. Shared core identifiers are deliberately registered idempotently.
func RegisterOptions(registry *core.OptionRegistry) {
	if registry == nil {
		return
	}
	options := []core.OptionData{
		{ID: AspectRatio.ID, Kind: core.DoubleOption, Targets: core.Parents},
		{ID: ExpandNodes.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: Padding.ID, Kind: core.ObjectOption, Targets: core.Parents, Parse: func(value string) (any, error) { return geometry.ParseSpacing(value) }},
		{ID: SpacingNodeNode.ID, Kind: core.DoubleOption, Targets: core.Parents},
		{ID: Interactive.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: OptimizationGoal.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: []string{"MAX_SCALE_DRIVEN", "ASPECT_RATIO_DRIVEN", "AREA_DRIVEN"}},
		{ID: LastPlaceShift.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: CurrentPosition.ID, Kind: core.IntegerOption, Targets: core.Nodes},
		{ID: DesiredPosition.ID, Kind: core.IntegerOption, Targets: core.Nodes},
		{ID: OnlyFirstIteration.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: RowCompaction.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: ExpandToAspect.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: TargetWidth.ID, Kind: core.DoubleOption, Targets: core.Parents},
	}
	for _, option := range options {
		if registry.ByID(option.ID) == nil {
			if err := registry.Register(option); err != nil {
				panic(err)
			}
		}
	}
	// The node-size option is shared with the utility providers and is needed
	// by effectiveMinSizeConstraintFor.
	basicoptions.RegisterOptions(registry)
}
