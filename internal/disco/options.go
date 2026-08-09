// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package disco

import (
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

var (
	SpacingComponent = graph.PropertyWithDefault("org.eclipse.elk.spacing.componentComponent", func() float64 { return 20 })
	EdgeThickness    = graph.PropertyWithDefault("org.eclipse.elk.edge.thickness", func() float64 { return 1 })
	AspectRatio      = graph.NewProperty[float64]("org.eclipse.elk.aspectRatio")
	Padding          = graph.CloneablePropertyWithDefault("org.eclipse.elk.padding", func() geometry.Spacing { return geometry.UniformSpacing(12) })

	PolyLowSort              = graph.PropertyWithDefault("org.eclipse.elk.polyomino.lowLevelSort", func() string { return "BY_SIZE_AND_SHAPE" })
	PolyHighSort             = graph.PropertyWithDefault("org.eclipse.elk.polyomino.highLevelSort", func() string { return "NUM_OF_EXTERNAL_SIDES_THAN_NUM_OF_EXTENSIONS_LAST" })
	PolyTraversal            = graph.PropertyWithDefault("org.eclipse.elk.polyomino.traversalStrategy", func() string { return "QUADRANTS_LINE_BY_LINE" })
	PolyFill                 = graph.PropertyWithDefault("org.eclipse.elk.polyomino.fill", func() bool { return true })
	Strategy                 = graph.PropertyWithDefault("org.eclipse.elk.disco.componentCompaction.strategy", func() string { return "POLYOMINO" })
	ComponentLayoutAlgorithm = graph.PropertyWithDefault("org.eclipse.elk.disco.componentCompaction.componentLayoutAlgorithm", func() string { return "" })
	DebugGraph               = graph.NewProperty[any]("org.eclipse.elk.disco.debug.discoGraph")
	DebugPolyominoes         = graph.NewProperty[any]("org.eclipse.elk.disco.debug.discoPolys")
)

func RegisterOptions(registry *core.OptionRegistry) {
	if registry == nil {
		return
	}
	options := []core.OptionData{
		{ID: SpacingComponent.ID, Kind: core.DoubleOption, Targets: core.Parents},
		{ID: EdgeThickness.ID, Kind: core.DoubleOption, Targets: core.Edges},
		{ID: AspectRatio.ID, Kind: core.DoubleOption, Targets: core.Parents},
		{ID: Padding.ID, Kind: core.ObjectOption, Targets: core.Parents, Parse: func(value string) (any, error) { return geometry.ParseSpacing(value) }},
		{ID: PolyLowSort.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: []string{"BY_SIZE", "BY_SIZE_AND_SHAPE"}},
		{ID: PolyHighSort.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: []string{"CORNER_CASES_THAN_SINGLE_SIDE_LAST", "NUM_OF_EXTERNAL_SIDES_THAN_NUM_OF_EXTENSIONS_LAST"}},
		{ID: PolyTraversal.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: []string{"SPIRAL", "LINE_BY_LINE", "MANHATTAN", "JITTER", "QUADRANTS_MANHATTAN", "QUADRANTS_LINE_BY_LINE", "COMBINE_LINE_BY_LINE_MANHATTAN", "COMBINE_JITTER_MANHATTAN", "QUADRANTS_JITTER"}},
		{ID: PolyFill.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: Strategy.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: []string{"POLYOMINO"}},
		{ID: ComponentLayoutAlgorithm.ID, Kind: core.StringOption, Targets: core.Parents},
		{ID: DebugGraph.ID, Group: "debug", Kind: core.ObjectOption, Targets: core.Parents},
		{ID: DebugPolyominoes.ID, Group: "debug", Kind: core.ObjectOption, Targets: core.Parents},
	}
	for _, option := range options {
		if registry.ByID(option.ID) == nil {
			if err := registry.Register(option); err != nil {
				panic(err)
			}
		}
	}
}
