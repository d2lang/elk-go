// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package spore

import (
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

const (
	OverlapAlgorithmID    = "org.eclipse.elk.sporeOverlap"
	CompactionAlgorithmID = "org.eclipse.elk.sporeCompaction"

	StructureDelaunay = "DELAUNAY_TRIANGULATION"

	TreeMinimum = "MINIMUM_SPANNING_TREE"
	TreeMaximum = "MAXIMUM_SPANNING_TREE"

	CostCenterDistance      = "CENTER_DISTANCE"
	CostCircleUnderlap      = "CIRCLE_UNDERLAP"
	CostRectangleUnderlap   = "RECTANGLE_UNDERLAP"
	CostInvertedOverlap     = "INVERTED_OVERLAP"
	CostMinimumRootDistance = "MINIMUM_ROOT_DISTANCE"

	RootFixed  = "FIXED"
	RootCenter = "CENTER_NODE"

	CompactionDepthFirst = "DEPTH_FIRST"
)

var (
	UnderlyingLayoutAlgorithm = graph.NewProperty[string]("org.eclipse.elk.underlyingLayoutAlgorithm")
	Padding                   = graph.CloneablePropertyWithDefault("org.eclipse.elk.padding", func() geometry.Spacing { return geometry.UniformSpacing(8) })
	SpacingNodeNode           = graph.PropertyWithDefault("org.eclipse.elk.spacing.nodeNode", func() float64 { return 8 })
	StructureExtraction       = graph.PropertyWithDefault("org.eclipse.elk.structure.structureExtractionStrategy", func() string { return StructureDelaunay })
	TreeConstruction          = graph.PropertyWithDefault("org.eclipse.elk.processingOrder.treeConstruction", func() string { return TreeMinimum })
	SpanningTreeCost          = graph.PropertyWithDefault("org.eclipse.elk.processingOrder.spanningTreeCostFunction", func() string { return CostCircleUnderlap })
	PreferredRoot             = graph.NewProperty[string]("org.eclipse.elk.processingOrder.preferredRoot")
	RootSelection             = graph.PropertyWithDefault("org.eclipse.elk.processingOrder.rootSelection", func() string { return RootCenter })
	Compaction                = graph.PropertyWithDefault("org.eclipse.elk.compaction.compactionStrategy", func() string { return CompactionDepthFirst })
	Orthogonal                = graph.PropertyWithDefault("org.eclipse.elk.compaction.orthogonal", func() bool { return false })
	MaxIterations             = graph.PropertyWithDefault("org.eclipse.elk.overlapRemoval.maxIterations", func() int { return 64 })
	RunScanline               = graph.PropertyWithDefault("org.eclipse.elk.overlapRemoval.runScanline", func() bool { return true })
	DebugMode                 = graph.PropertyWithDefault("org.eclipse.elk.debugMode", func() bool { return false })
)

// optionSpecs declares the options RegisterOptions registers.
var optionSpecs = [...]core.OptionSpec{
	{ID: UnderlyingLayoutAlgorithm.ID, Kind: core.StringOption, Targets: core.Parents},
	{ID: StructureExtraction.ID, Group: "structure", Kind: core.EnumOption, Targets: core.Parents, EnumValues: StructureDelaunay},
	{ID: TreeConstruction.ID, Group: "processingOrder", Kind: core.EnumOption, Targets: core.Parents, EnumValues: TreeMinimum + " " + TreeMaximum},
	{ID: SpanningTreeCost.ID, Group: "processingOrder", Kind: core.EnumOption, Targets: core.Parents, EnumValues: CostCenterDistance + " " + CostCircleUnderlap + " " + CostRectangleUnderlap + " " + CostInvertedOverlap + " " + CostMinimumRootDistance},
	{ID: PreferredRoot.ID, Group: "processingOrder", Kind: core.StringOption, Targets: core.Parents},
	{ID: RootSelection.ID, Group: "processingOrder", Kind: core.EnumOption, Targets: core.Parents, EnumValues: RootFixed + " " + RootCenter},
	{ID: Compaction.ID, Group: "compaction", Kind: core.EnumOption, Targets: core.Parents, EnumValues: CompactionDepthFirst},
	{ID: Orthogonal.ID, Group: "compaction", Kind: core.BooleanOption, Targets: core.Parents},
	{ID: MaxIterations.ID, Group: "overlapRemoval", Kind: core.IntegerOption, Targets: core.Parents},
	{ID: RunScanline.ID, Group: "overlapRemoval", Kind: core.BooleanOption, Targets: core.Parents},
	{ID: DebugMode.ID, Kind: core.BooleanOption, Targets: core.Parents},
}

// RegisterOptions installs the metadata generated from SPOrE.melk in ELK
// 0.8.1. Shared core options are registered only when absent.
func RegisterOptions(registry *core.OptionRegistry) {
	if registry == nil {
		return
	}
	for index := range optionSpecs {
		registry.RegisterMissing(optionSpecs[index])
	}
}
