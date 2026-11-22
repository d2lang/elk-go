// Copyright (c) 2009, 2015 Kiel University and others.
// Copyright (c) 2010, 2015 Kiel University and others.
// Copyright (c) 2012, 2015 Kiel University and others.
// Copyright (c) 2015 Kiel University and others.
// Copyright (c) 2015, 2020 Kiel University and others.
// Copyright (c) 2016, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

var (
	Algorithm                                   = graph.PropertyWithDefault("org.eclipse.elk.algorithm", func() string { return "" })
	ResolvedAlgorithm                           = graph.NewProperty[*AlgorithmData]("org.eclipse.elk.resolvedAlgorithm")
	NoLayout                                    = graph.PropertyWithDefault("org.eclipse.elk.noLayout", func() bool { return false })
	Direction                                   = graph.PropertyWithDefault("org.eclipse.elk.direction", func() string { return "UNDEFINED" })
	HierarchyHandling                           = graph.PropertyWithDefault("org.eclipse.elk.hierarchyHandling", func() string { return "INHERIT" })
	Padding                                     = graph.CloneablePropertyWithDefault("org.eclipse.elk.padding", func() geometry.Spacing { return geometry.UniformSpacing(12) })
	ContentAlignment                            = graph.CloneablePropertyWithDefault("org.eclipse.elk.contentAlignment", func() []string { return []string{"V_TOP", "H_LEFT"} })
	AspectRatio                                 = graph.PropertyWithDefault("org.eclipse.elk.aspectRatio", func() float64 { return 1.6 })
	SpacingNodeNode                             = graph.PropertyWithDefault("org.eclipse.elk.spacing.nodeNode", func() float64 { return 20 })
	PortSide                                    = graph.PropertyWithDefault("org.eclipse.elk.port.side", func() string { return "UNDEFINED" })
	PortAnchor                                  = graph.NewProperty[geometry.Point]("org.eclipse.elk.port.anchor")
	PortConstraints                             = graph.PropertyWithDefault("org.eclipse.elk.portConstraints", func() string { return "UNDEFINED" })
	NodeSizeMinimum                             = graph.CloneablePropertyWithDefault("org.eclipse.elk.nodeSize.minimum", func() geometry.Point { return geometry.Point{} })
	NodeSizeConstraints                         = graph.CloneablePropertyWithDefault("org.eclipse.elk.nodeSize.constraints", func() []string { return nil })
	JunctionPoints                              = graph.NewProperty[geometry.Chain]("org.eclipse.elk.junctionPoints")
	ScaleFactor                                 = graph.PropertyWithDefault("org.eclipse.elk.scaleFactor", func() float64 { return 1 })
	SpacingIndividual                           = graph.NewProperty[*IndividualSpacings]("org.eclipse.elk.spacing.individual")
	SoftwrappingFuzziness                       = graph.PropertyWithDefault("org.eclipse.elk.softwrappingFuzziness", func() float64 { return 0 })
	ChildAreaWidth                              = graph.NewProperty[float64]("org.eclipse.elk.childAreaWidth")
	ChildAreaHeight                             = graph.NewProperty[float64]("org.eclipse.elk.childAreaHeight")
	JSONShapeCoords                             = graph.PropertyWithDefault("org.eclipse.elk.json.shapeCoords", func() string { return "INHERIT" })
	JSONEdgeCoords                              = graph.PropertyWithDefault("org.eclipse.elk.json.edgeCoords", func() string { return "INHERIT" })
	TopdownLayout                               = graph.PropertyWithDefault("org.eclipse.elk.topdownLayout", func() bool { return false })
	TopdownSizeCategories                       = graph.PropertyWithDefault("org.eclipse.elk.topdown.sizeCategories", func() int { return 3 })
	TopdownSizeCategoriesHierarchicalNodeWeight = graph.PropertyWithDefault(
		"org.eclipse.elk.topdown.sizeCategoriesHierarchicalNodeWeight", func() int { return 4 })
	TopdownScaleFactor           = graph.PropertyWithDefault("org.eclipse.elk.topdown.scaleFactor", func() float64 { return 1 })
	TopdownSizeApproximator      = graph.NewProperty[any]("org.eclipse.elk.topdown.sizeApproximator")
	TopdownHierarchicalNodeWidth = graph.PropertyWithDefault(
		"org.eclipse.elk.topdown.hierarchicalNodeWidth", func() float64 { return 150 })
	TopdownHierarchicalNodeAspectRatio = graph.PropertyWithDefault(
		"org.eclipse.elk.topdown.hierarchicalNodeAspectRatio", func() float64 { return 1.414 })
	TopdownNodeType = graph.NewProperty[string]("org.eclipse.elk.topdown.nodeType")
	TopdownScaleCap = graph.PropertyWithDefault("org.eclipse.elk.topdown.scaleCap", func() float64 { return 1 })

	// Core graph semantics consumed by ELK Layered and node micro-layout.
	CommentBox                     = graph.PropertyWithDefault("org.eclipse.elk.commentBox", func() bool { return false })
	Hypernode                      = graph.PropertyWithDefault("org.eclipse.elk.hypernode", func() bool { return false })
	InsideSelfLoopsActivate        = graph.PropertyWithDefault("org.eclipse.elk.insideSelfLoops.activate", func() bool { return false })
	InsideSelfLoopsEdge            = graph.PropertyWithDefault("org.eclipse.elk.insideSelfLoops.yo", func() bool { return false })
	InteractiveLayout              = graph.PropertyWithDefault("org.eclipse.elk.interactiveLayout", func() bool { return false })
	PartitioningActivate           = graph.PropertyWithDefault("org.eclipse.elk.partitioning.activate", func() bool { return false })
	PartitioningPartition          = graph.NewProperty[int]("org.eclipse.elk.partitioning.partition")
	PortAlignmentNorth             = graph.NewProperty[string]("org.eclipse.elk.portAlignment.north")
	PortAlignmentEast              = graph.NewProperty[string]("org.eclipse.elk.portAlignment.east")
	PortAlignmentSouth             = graph.NewProperty[string]("org.eclipse.elk.portAlignment.south")
	PortAlignmentWest              = graph.NewProperty[string]("org.eclipse.elk.portAlignment.west")
	PortLabelsNextToPortIfPossible = graph.PropertyWithDefault("org.eclipse.elk.portLabels.nextToPortIfPossible", func() bool { return false })
	PortLabelsTreatAsGroup         = graph.PropertyWithDefault("org.eclipse.elk.portLabels.treatAsGroup", func() bool { return true })
	SpacingCommentComment          = graph.PropertyWithDefault("org.eclipse.elk.spacing.commentComment", func() float64 { return 10 })
	SpacingCommentNode             = graph.PropertyWithDefault("org.eclipse.elk.spacing.commentNode", func() float64 { return 10 })

	// Headless elkjs exposes these UI and programmatic options through
	// knownLayoutOptions, but they do not affect native layout execution.
	Animate          = graph.PropertyWithDefault("org.eclipse.elk.animate", func() bool { return true })
	AnimationFactor  = graph.PropertyWithDefault("org.eclipse.elk.animTimeFactor", func() int { return 100 })
	EdgeType         = graph.PropertyWithDefault("org.eclipse.elk.edge.type", func() string { return "NONE" })
	FontName         = graph.NewProperty[string]("org.eclipse.elk.font.name")
	FontSize         = graph.NewProperty[int]("org.eclipse.elk.font.size")
	LabelManager     = graph.NewProperty[any]("org.eclipse.elk.labelManager")
	LayoutAncestors  = graph.PropertyWithDefault("org.eclipse.elk.layoutAncestors", func() bool { return false })
	MaxAnimationTime = graph.PropertyWithDefault("org.eclipse.elk.maxAnimTime", func() int { return 4000 })
	MinAnimationTime = graph.PropertyWithDefault("org.eclipse.elk.minAnimTime", func() int { return 400 })
	ProgressBar      = graph.PropertyWithDefault("org.eclipse.elk.progressBar", func() bool { return false })
	ValidateGraph    = graph.PropertyWithDefault("org.eclipse.elk.validateGraph", func() bool { return false })
	ValidateOptions  = graph.PropertyWithDefault("org.eclipse.elk.validateOptions", func() bool { return true })
	ZoomToFit        = graph.PropertyWithDefault("org.eclipse.elk.zoomToFit", func() bool { return false })
)

type IndividualSpacings struct{ Properties graph.PropertyMap }

func RegisterCoreOptions(registry *OptionRegistry) {
	options := []OptionData{
		{ID: Algorithm.ID, Kind: StringOption, Targets: Parents},
		{ID: NoLayout.ID, Kind: BooleanOption, Targets: Parents},
		{ID: Direction.ID, Kind: EnumOption, Targets: Parents, EnumValues: []string{"UNDEFINED", "RIGHT", "LEFT", "DOWN", "UP"}},
		{ID: HierarchyHandling.ID, Kind: EnumOption, Targets: Parents, EnumValues: []string{"INHERIT", "INCLUDE_CHILDREN", "SEPARATE_CHILDREN"}},
		{ID: Padding.ID, Kind: ObjectOption, Targets: Parents, Parse: func(value string) (any, error) { return geometry.ParseSpacing(value) }, pureParse: true},
		{ID: ContentAlignment.ID, Kind: EnumSetOption, Targets: Parents, EnumValues: []string{"V_TOP", "V_CENTER", "V_BOTTOM", "H_LEFT", "H_CENTER", "H_RIGHT"}},
		{ID: AspectRatio.ID, Kind: DoubleOption, Targets: Parents},
		{ID: PortSide.ID, Kind: EnumOption, Targets: Ports, EnumValues: []string{"UNDEFINED", "NORTH", "EAST", "SOUTH", "WEST"}},
		{ID: PortAnchor.ID, Kind: ObjectOption, Targets: Ports, Parse: func(value string) (any, error) { return geometry.ParsePoint(value) }, pureParse: true},
		{ID: PortConstraints.ID, Kind: EnumOption, Targets: Parents, EnumValues: []string{"UNDEFINED", "FREE", "FIXED_SIDE", "FIXED_ORDER", "FIXED_RATIO", "FIXED_POS"}},
		{ID: NodeSizeMinimum.ID, Kind: ObjectOption, Targets: Nodes, Parse: func(value string) (any, error) { return geometry.ParsePoint(value) }, pureParse: true},
		{ID: NodeSizeConstraints.ID, Kind: EnumSetOption, Targets: Nodes, EnumValues: []string{"PORTS", "PORT_LABELS", "NODE_LABELS", "MINIMUM_SIZE"}},
		{ID: ScaleFactor.ID, Kind: DoubleOption, Targets: Nodes},
		{ID: SoftwrappingFuzziness.ID, Kind: DoubleOption, Targets: Labels},
		{ID: ChildAreaWidth.ID, Kind: DoubleOption, Targets: Parents},
		{ID: ChildAreaHeight.ID, Kind: DoubleOption, Targets: Parents},
		{ID: JSONShapeCoords.ID, Group: "json", Kind: EnumOption, Targets: Parents,
			EnumValues: []string{"INHERIT", "PARENT", "ROOT"}},
		{ID: JSONEdgeCoords.ID, Group: "json", Kind: EnumOption, Targets: Parents,
			EnumValues: []string{"INHERIT", "CONTAINER", "PARENT", "ROOT"}},
		{ID: TopdownLayout.ID, Kind: BooleanOption, Targets: Parents},
		{ID: TopdownSizeCategories.ID, Group: "topdown", Kind: IntegerOption, Targets: Parents},
		{ID: TopdownSizeCategoriesHierarchicalNodeWeight.ID, Group: "topdown", Kind: IntegerOption, Targets: Parents},
		{ID: TopdownScaleFactor.ID, Group: "topdown", Kind: DoubleOption, Targets: Parents},
		{ID: TopdownSizeApproximator.ID, Group: "topdown", Kind: ObjectOption, Targets: Nodes},
		{ID: TopdownHierarchicalNodeWidth.ID, Group: "topdown", Kind: DoubleOption, Targets: Parents | Nodes},
		{ID: TopdownHierarchicalNodeAspectRatio.ID, Group: "topdown", Kind: DoubleOption, Targets: Parents | Nodes},
		{ID: TopdownNodeType.ID, Group: "topdown", Kind: EnumOption, Targets: Nodes,
			EnumValues: []string{"PARALLEL_NODE", "HIERARCHICAL_NODE", "ROOT_NODE"}},
		{ID: TopdownScaleCap.ID, Group: "topdown", Kind: DoubleOption, Targets: Parents},
		{ID: "org.eclipse.elk.edgeLabels.inline", Kind: BooleanOption, Targets: Edges},
		{ID: "org.eclipse.elk.spacing.edgeNode", Kind: DoubleOption, Targets: Parents},
		{ID: SpacingNodeNode.ID, Kind: DoubleOption, Targets: Parents},
		{ID: "org.eclipse.elk.spacing.nodeSelfLoop", Kind: DoubleOption, Targets: Parents},
		{ID: CommentBox.ID, Kind: BooleanOption, Targets: Nodes},
		{ID: Hypernode.ID, Kind: BooleanOption, Targets: Nodes},
		{ID: InsideSelfLoopsActivate.ID, Group: "insideSelfLoops", Kind: BooleanOption, Targets: Nodes},
		{ID: InsideSelfLoopsEdge.ID, Group: "insideSelfLoops", Kind: BooleanOption, Targets: Edges},
		{ID: InteractiveLayout.ID, Kind: BooleanOption, Targets: Parents},
		{ID: PartitioningActivate.ID, Group: "partitioning", Kind: BooleanOption, Targets: Parents},
		{ID: PartitioningPartition.ID, Group: "partitioning", Kind: IntegerOption, Targets: Parents | Nodes},
		{ID: PortAlignmentNorth.ID, Group: "portAlignment", Kind: EnumOption, Targets: Nodes,
			EnumValues: []string{"DISTRIBUTED", "JUSTIFIED", "BEGIN", "CENTER", "END"}},
		{ID: PortAlignmentEast.ID, Group: "portAlignment", Kind: EnumOption, Targets: Nodes,
			EnumValues: []string{"DISTRIBUTED", "JUSTIFIED", "BEGIN", "CENTER", "END"}},
		{ID: PortAlignmentSouth.ID, Group: "portAlignment", Kind: EnumOption, Targets: Nodes,
			EnumValues: []string{"DISTRIBUTED", "JUSTIFIED", "BEGIN", "CENTER", "END"}},
		{ID: PortAlignmentWest.ID, Group: "portAlignment", Kind: EnumOption, Targets: Nodes,
			EnumValues: []string{"DISTRIBUTED", "JUSTIFIED", "BEGIN", "CENTER", "END"}},
		{ID: PortLabelsNextToPortIfPossible.ID, Group: "portLabels", Kind: BooleanOption, Targets: Nodes},
		{ID: PortLabelsTreatAsGroup.ID, Group: "portLabels", Kind: BooleanOption, Targets: Nodes},
		{ID: SpacingCommentComment.ID, Group: "spacing", Kind: DoubleOption, Targets: Parents},
		{ID: SpacingCommentNode.ID, Group: "spacing", Kind: DoubleOption, Targets: Parents},

		{ID: AnimationFactor.ID, Kind: IntegerOption, Targets: Parents, MetadataOnly: true},
		{ID: Animate.ID, Kind: BooleanOption, Targets: Parents, MetadataOnly: true},
		{ID: EdgeType.ID, Group: "edge", Kind: EnumOption, Targets: Edges, MetadataOnly: true,
			EnumValues: []string{"NONE", "DIRECTED", "UNDIRECTED", "ASSOCIATION", "GENERALIZATION", "DEPENDENCY"}},
		{ID: FontName.ID, Group: "font", Kind: StringOption, Targets: Labels, MetadataOnly: true},
		{ID: FontSize.ID, Group: "font", Kind: IntegerOption, Targets: Labels, MetadataOnly: true},
		{ID: LabelManager.ID, Kind: ObjectOption, Targets: Parents | Labels, MetadataOnly: true},
		{ID: LayoutAncestors.ID, Kind: BooleanOption, Targets: Parents, MetadataOnly: true},
		{ID: MaxAnimationTime.ID, Kind: IntegerOption, Targets: Parents, MetadataOnly: true},
		{ID: MinAnimationTime.ID, Kind: IntegerOption, Targets: Parents, MetadataOnly: true},
		{ID: ProgressBar.ID, Kind: BooleanOption, Targets: Parents, MetadataOnly: true},
		{ID: ValidateGraph.ID, Kind: BooleanOption, Targets: Parents, MetadataOnly: true},
		{ID: ValidateOptions.ID, Kind: BooleanOption, Targets: Parents, MetadataOnly: true},
		{ID: ZoomToFit.ID, Kind: BooleanOption, Targets: Parents, MetadataOnly: true},
	}
	for _, option := range options {
		if err := registry.Register(option); err != nil {
			panic(err)
		}
	}
}
