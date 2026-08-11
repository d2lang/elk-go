// Copyright (c) 2017, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package radial is a native Go port of ELK Radial targeting ELK.js 0.12.0.
package radial

import (
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

const AlgorithmID = "org.eclipse.elk.radial"

var (
	Padding                             = graph.CloneablePropertyWithDefault("org.eclipse.elk.padding", func() geometry.Spacing { return geometry.UniformSpacing(12) })
	SpacingNodeNode                     = graph.PropertyWithDefault("org.eclipse.elk.spacing.nodeNode", func() float64 { return 20 })
	OmitNodeMicroLayout                 = graph.PropertyWithDefault("org.eclipse.elk.omitNodeMicroLayout", func() bool { return false })
	Position                            = graph.NewProperty[geometry.Point]("org.eclipse.elk.position")
	OrderID                             = graph.PropertyWithDefault("org.eclipse.elk.radial.orderId", func() int { return 0 })
	Radius                              = graph.PropertyWithDefault("org.eclipse.elk.radial.radius", func() float64 { return 0 })
	Compactor                           = graph.PropertyWithDefault("org.eclipse.elk.radial.compactor", func() string { return "NONE" })
	CompactionStepSize                  = graph.PropertyWithDefault("org.eclipse.elk.radial.compactionStepSize", func() int { return 1 })
	Sorter                              = graph.PropertyWithDefault("org.eclipse.elk.radial.sorter", func() string { return "NONE" })
	WedgeCriteria                       = graph.PropertyWithDefault("org.eclipse.elk.radial.wedgeCriteria", func() string { return "NODE_SIZE" })
	OptimizationCriteria                = graph.PropertyWithDefault("org.eclipse.elk.radial.optimizationCriteria", func() string { return "NONE" })
	CenterOnRoot                        = graph.PropertyWithDefault("org.eclipse.elk.radial.centerOnRoot", func() bool { return false })
	Rotate                              = graph.PropertyWithDefault("org.eclipse.elk.radial.rotate", func() bool { return false })
	RotationTargetAngle                 = graph.PropertyWithDefault("org.eclipse.elk.radial.rotation.targetAngle", func() float64 { return 0 })
	RotationComputeAdditionalWedgeSpace = graph.PropertyWithDefault(
		"org.eclipse.elk.radial.rotation.computeAdditionalWedgeSpace", func() bool { return false })
	RotationOutgoingEdgeAngles = graph.PropertyWithDefault(
		"org.eclipse.elk.radial.rotation.outgoingEdgeAngles", func() bool { return false })
)

// RegisterOptions installs the metadata generated from Radial.melk in ELK
// 0.12.0.
func RegisterOptions(registry *core.OptionRegistry) {
	if registry == nil {
		return
	}
	options := []core.OptionData{
		{ID: Position.ID, Kind: core.ObjectOption, Targets: core.Nodes | core.Ports | core.Labels, Parse: func(value string) (any, error) { return geometry.ParsePoint(value) }},
		{ID: OmitNodeMicroLayout.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: OrderID.ID, Kind: core.IntegerOption, Targets: core.Nodes},
		{ID: Radius.ID, Kind: core.DoubleOption, Targets: core.Parents},
		{ID: Compactor.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: []string{"NONE", "RADIAL_COMPACTION", "WEDGE_COMPACTION"}},
		{ID: CompactionStepSize.ID, Kind: core.IntegerOption, Targets: core.Parents},
		{ID: Sorter.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: []string{"NONE", "POLAR_COORDINATE", "ID"}},
		{ID: WedgeCriteria.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: []string{"LEAF_NUMBER", "NODE_SIZE"}},
		{ID: OptimizationCriteria.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: []string{"NONE", "EDGE_LENGTH", "EDGE_LENGTH_BY_POSITION", "CROSSING_MINIMIZATION_BY_POSITION"}},
		{ID: CenterOnRoot.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: Rotate.ID, Kind: core.BooleanOption, Targets: core.Parents},
		{ID: RotationTargetAngle.ID, Group: "rotation", Kind: core.DoubleOption, Targets: core.Parents},
		{ID: RotationComputeAdditionalWedgeSpace.ID, Group: "rotation", Kind: core.BooleanOption, Targets: core.Parents},
		{ID: RotationOutgoingEdgeAngles.ID, Group: "rotation", Kind: core.BooleanOption, Targets: core.Parents},
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
