// Copyright (c) 2015, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package force is a native Go port of ELK 0.8.1's org.eclipse.elk.force
// provider. The source-shaped names and iteration order in this package are
// intentional: both are observable in the layouts emitted by elkjs 0.8.2.
package force

import (
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

const (
	ModelFruchtermanReingold = "FRUCHTERMAN_REINGOLD"
	ModelEades               = "EADES"
)

var (
	Model          = graph.PropertyWithDefault("org.eclipse.elk.force.model", func() string { return ModelFruchtermanReingold })
	Iterations     = graph.PropertyWithDefault("org.eclipse.elk.force.iterations", func() int { return 300 })
	RepulsivePower = graph.PropertyWithDefault("org.eclipse.elk.force.repulsivePower", func() int { return 0 })
	Temperature    = graph.PropertyWithDefault("org.eclipse.elk.force.temperature", func() float64 { return 0.001 })
	Repulsion      = graph.PropertyWithDefault("org.eclipse.elk.force.repulsion", func() float64 { return 5 })

	Priority                    = graph.PropertyWithDefault("org.eclipse.elk.priority", func() int { return 1 })
	SpacingNodeNode             = graph.PropertyWithDefault("org.eclipse.elk.spacing.nodeNode", func() float64 { return 80 })
	SpacingEdgeLabel            = graph.PropertyWithDefault("org.eclipse.elk.spacing.edgeLabel", func() float64 { return 5 })
	AspectRatio                 = graph.PropertyWithDefault("org.eclipse.elk.aspectRatio", func() float64 { return 1.6 })
	RandomSeed                  = graph.PropertyWithDefault("org.eclipse.elk.randomSeed", func() int { return 1 })
	SeparateConnectedComponents = graph.PropertyWithDefault("org.eclipse.elk.separateConnectedComponents", func() bool { return true })
	Interactive                 = graph.PropertyWithDefault("org.eclipse.elk.interactive", func() bool { return false })
	OmitNodeMicroLayout         = graph.PropertyWithDefault("org.eclipse.elk.omitNodeMicroLayout", func() bool { return false })
	EdgeLabelsInline            = graph.PropertyWithDefault("org.eclipse.elk.edgeLabels.inline", func() bool { return false })
	Padding                     = graph.CloneablePropertyWithDefault("org.eclipse.elk.padding", func() geometry.Spacing {
		return geometry.UniformSpacing(50)
	})
)

// RegisterOptions adds the generated ForceOptions metadata from ELK 0.8.1.
// Shared core options may already be present, so registration is idempotent.
func RegisterOptions(registry *core.OptionRegistry) {
	options := []core.OptionData{
		{ID: Model.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: []string{ModelEades, ModelFruchtermanReingold}},
		{ID: Iterations.ID, Kind: core.IntegerOption, Targets: core.Parents},
		{ID: RepulsivePower.ID, Kind: core.IntegerOption, Targets: core.Edges},
		{ID: Temperature.ID, Kind: core.DoubleOption, Targets: core.Parents},
		{ID: Repulsion.ID, Kind: core.DoubleOption, Targets: core.Parents},
		{ID: Priority.ID, Kind: core.IntegerOption, Targets: core.Nodes | core.Edges},
		{ID: SpacingNodeNode.ID, Kind: core.DoubleOption, Targets: core.Parents},
		{ID: SpacingEdgeLabel.ID, Kind: core.DoubleOption, Targets: core.Parents},
		{ID: AspectRatio.ID, Kind: core.DoubleOption, Targets: core.Parents},
		{ID: RandomSeed.ID, Kind: core.IntegerOption, Targets: core.Parents},
		{ID: SeparateConnectedComponents.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: Interactive.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: OmitNodeMicroLayout.ID, Kind: core.BooleanOption, Targets: core.Parents},
	}
	for _, option := range options {
		if registry.ByID(option.ID) == nil {
			if err := registry.Register(option); err != nil {
				panic(err)
			}
		}
	}
}
