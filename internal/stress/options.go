// Copyright (c) 2016, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package stress ports org.eclipse.elk.stress targeting ELK.js 0.12.0 while
// retaining the historical ELK 0.8.1 source mapping for unchanged routines.
package stress

import (
	"math"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/graph"
)

const (
	DimensionXY = "XY"
	DimensionX  = "X"
	DimensionY  = "Y"
)

var (
	Fixed             = graph.PropertyWithDefault("org.eclipse.elk.stress.fixed", func() bool { return false })
	DesiredEdgeLength = graph.PropertyWithDefault("org.eclipse.elk.stress.desiredEdgeLength", func() float64 { return 100 })
	Dimension         = graph.PropertyWithDefault("org.eclipse.elk.stress.dimension", func() string { return DimensionXY })
	Epsilon           = graph.PropertyWithDefault("org.eclipse.elk.stress.epsilon", func() float64 { return 10e-4 })
	IterationLimit    = graph.PropertyWithDefault("org.eclipse.elk.stress.iterationLimit", func() int { return math.MaxInt32 })
)

func RegisterOptions(registry *core.OptionRegistry) {
	options := []core.OptionData{
		{ID: Fixed.ID, Kind: core.BooleanOption, Targets: core.Nodes},
		{ID: DesiredEdgeLength.ID, Kind: core.DoubleOption, Targets: core.Parents | core.Edges},
		{ID: Dimension.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: []string{DimensionXY, DimensionX, DimensionY}},
		{ID: Epsilon.ID, Kind: core.DoubleOption, Targets: core.Parents},
		{ID: IterationLimit.ID, Kind: core.IntegerOption, Targets: core.Parents},
	}
	for _, option := range options {
		if registry.ByID(option.ID) == nil {
			if err := registry.Register(option); err != nil {
				panic(err)
			}
		}
	}
}
