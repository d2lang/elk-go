// Copyright (c) 2009, 2015 Kiel University and others.
// Copyright (c) 2009, 2020 Kiel University and others.
// Copyright (c) 2010, 2015 Kiel University and others.
// Copyright (c) 2010, 2017 Kiel University and others.
// Copyright (c) 2011, 2015 Kiel University and others.
// Copyright (c) 2012, 2015 Kiel University and others.
// Copyright (c) 2012, 2017 Kiel University and others.
// Copyright (c) 2012, 2020 Kiel University and others.
// Copyright (c) 2014, 2015 Kiel University and others.
// Copyright (c) 2014, 2017 Kiel University and others.
// Copyright (c) 2014, 2020 Kiel University and others.
// Copyright (c) 2015 Kiel University and others.
// Copyright (c) 2015, 2020 Kiel University and others.
// Copyright (c) 2016 Kiel University and others.
// Copyright (c) 2016, 2017 Kiel University and others.
// Copyright (c) 2016, 2020 Kiel University and others.
// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2018 Kiel University and others.
// Copyright (c) 2020 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"github.com/d2lang/elk-go/internal/common/nodespacing"
	"github.com/d2lang/elk-go/internal/core"
)

// optionSpecs declares the options RegisterOptions registers.
var optionSpecs = [...]core.OptionSpec{
	{ID: EdgeRoutingOption.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: "UNDEFINED POLYLINE ORTHOGONAL SPLINES"},
	{ID: RandomSeed.ID, Kind: core.IntegerOption, Targets: core.Parents},
	{ID: SeparateComponents.ID, Kind: core.BooleanOption, Targets: core.Parents},
	{ID: AspectRatio.ID, Kind: core.DoubleOption, Targets: core.Parents},
	{ID: AlignmentOption.ID, Kind: core.EnumOption, Targets: core.Nodes, EnumValues: "AUTOMATIC LEFT RIGHT CENTER TOP BOTTOM"},
	{ID: PositionOption.ID, Kind: core.ObjectOption, Targets: core.Parents | core.Nodes | core.Edges | core.Ports | core.Labels, Parse: core.PointParser},
	{ID: PortBorderOffset.ID, Kind: core.DoubleOption, Targets: core.Ports},
	{ID: PortAnchor.ID, Kind: core.ObjectOption, Targets: core.Ports, Parse: core.PointParser},
	{ID: PortIndex.ID, Kind: core.IntegerOption, Targets: core.Ports},
	{ID: NodeLabelsPadding.ID, Kind: core.ObjectOption, Targets: core.Nodes, Parse: core.SpacingParser},
	{ID: nodespacing.NodeLabelsPlacement.ID, Group: "nodeLabels", Kind: core.EnumSetOption, Targets: core.Nodes | core.Labels, EnumValues: "H_LEFT H_CENTER H_RIGHT V_TOP V_CENTER V_BOTTOM INSIDE OUTSIDE H_PRIORITY"},
	{ID: nodespacing.PortLabelsPlacement.ID, Group: "portLabels", Kind: core.EnumSetOption, Targets: core.Nodes, EnumValues: "OUTSIDE INSIDE NEXT_TO_PORT_IF_POSSIBLE ALWAYS_SAME_SIDE ALWAYS_OTHER_SAME_SIDE SPACE_EFFICIENT"},
	{ID: EdgeLabelsPlacement.ID, Kind: core.EnumOption, Targets: core.Labels, EnumValues: "CENTER HEAD TAIL"},
	{ID: EdgeThickness.ID, Kind: core.DoubleOption, Targets: core.Edges},
	{ID: JunctionPoints.ID, Kind: core.ObjectOption, Targets: core.Edges, Parse: core.ChainParser},

	{ID: CycleBreaking.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: "GREEDY DEPTH_FIRST INTERACTIVE MODEL_ORDER GREEDY_MODEL_ORDER SCC_CONNECTIVITY SCC_NODE_TYPE DFS_NODE_ORDER BFS_NODE_ORDER"},
	{ID: Layering.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: "NETWORK_SIMPLEX LONGEST_PATH LONGEST_PATH_SOURCE COFFMAN_GRAHAM INTERACTIVE STRETCH_WIDTH MIN_WIDTH BF_MODEL_ORDER DF_MODEL_ORDER"},
	{ID: CoffmanGrahamLayerBound.ID, Kind: core.IntegerOption, Targets: core.Parents},
	{ID: MinWidthUpperBound.ID, Kind: core.IntegerOption, Targets: core.Parents},
	{ID: MinWidthUpperLayerScaling.ID, Kind: core.IntegerOption, Targets: core.Parents},
	{ID: CrossingMinimization.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: "LAYER_SWEEP MEDIAN_LAYER_SWEEP INTERACTIVE NONE"},
	{ID: NodePlacement.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: "SIMPLE INTERACTIVE LINEAR_SEGMENTS BRANDES_KOEPF NETWORK_SIMPLEX"},
	{ID: FavorStraightEdges.ID, Kind: core.BooleanOption, Targets: core.Parents},
	{ID: BKFixedAlignment.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: "NONE LEFTUP RIGHTUP LEFTDOWN RIGHTDOWN BALANCED"},
	{ID: BKEdgeStraightening.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: "NONE IMPROVE_STRAIGHTNESS"},
	{ID: LinearSegmentsDeflectionDampening.ID, Kind: core.DoubleOption, Targets: core.Parents},
	{ID: NetworkSimplexNodeFlexibility.ID, Group: "nodePlacement.networkSimplex", Kind: core.EnumOption, Targets: core.Nodes, EnumValues: "NONE PORT_POSITION NODE_SIZE_WHERE_SPACE_PERMITS NODE_SIZE"},
	{ID: NetworkSimplexNodeFlexibilityDefault.ID, Group: "nodePlacement.networkSimplex.nodeFlexibility", Kind: core.EnumOption, Targets: core.Parents, EnumValues: "NONE PORT_POSITION NODE_SIZE_WHERE_SPACE_PERMITS NODE_SIZE"},
	{ID: NetworkSimplexRecomputeNodePlacement.ID, Group: "nodePlacement.networkSimplex.nodeFlexibility", Kind: core.EnumOption, Targets: core.Parents, EnumValues: "SIMPLE INTERACTIVE LINEAR_SEGMENTS BRANDES_KOEPF NETWORK_SIMPLEX"},
	{ID: Thoroughness.ID, Kind: core.IntegerOption, Targets: core.Parents},
	{ID: ForceNodeModelOrder.ID, Kind: core.BooleanOption, Targets: core.Parents},
	{ID: ConsiderModelOrder.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: "NONE NODES_AND_EDGES PREFER_EDGES PREFER_NODES"},
	{ID: PortModelOrder.ID, Group: "considerModelOrder", Kind: core.BooleanOption, Targets: core.Parents},
	{ID: ComponentOrdering.ID, Group: "considerModelOrder", Kind: core.EnumOption, Targets: core.Parents, EnumValues: "NONE INSIDE_PORT_SIDE_GROUPS GROUP_MODEL_ORDER MODEL_ORDER"},
	{ID: LongEdgeOrdering.ID, Group: "considerModelOrder", Kind: core.EnumOption, Targets: core.Parents, EnumValues: "DUMMY_NODE_OVER DUMMY_NODE_UNDER EQUAL"},
	{ID: CrossingCounterNodeInfluence.ID, Group: "considerModelOrder", Kind: core.DoubleOption, Targets: core.Parents},
	{ID: CrossingCounterPortInfluence.ID, Group: "considerModelOrder", Kind: core.DoubleOption, Targets: core.Parents},
	{ID: CycleBreakingGroupID.ID, Group: "considerModelOrder.groupModelOrder", Kind: core.IntegerOption, Targets: core.Nodes},
	{ID: CrossingMinimizationGroupID.ID, Group: "considerModelOrder.groupModelOrder", Kind: core.IntegerOption, Targets: core.Nodes | core.Edges | core.Ports},
	{ID: ComponentGroupID.ID, Group: "considerModelOrder.groupModelOrder", Kind: core.IntegerOption, Targets: core.Nodes | core.Edges | core.Ports},
	{ID: CycleBreakingGroupOrder.ID, Group: "considerModelOrder.groupModelOrder", Kind: core.EnumOption, Targets: core.Parents, EnumValues: "ONLY_WITHIN_GROUP MODEL_ORDER ENFORCED"},
	{ID: CycleBreakingPreferredSourceID.ID, Group: "considerModelOrder.groupModelOrder", Kind: core.IntegerOption, Targets: core.Parents},
	{ID: CycleBreakingPreferredTargetID.ID, Group: "considerModelOrder.groupModelOrder", Kind: core.IntegerOption, Targets: core.Parents},
	{ID: CrossingMinimizationGroupOrder.ID, Group: "considerModelOrder.groupModelOrder", Kind: core.EnumOption, Targets: core.Parents, EnumValues: "ONLY_WITHIN_GROUP MODEL_ORDER ENFORCED"},
	{ID: CrossingMinimizationEnforcedGroups.ID, Group: "considerModelOrder.groupModelOrder", Kind: core.ObjectOption, Targets: core.Parents, Parse: core.IntegerListParser},
	{ID: GreedySwitchActivationThreshold.ID, Kind: core.IntegerOption, Targets: core.Parents},
	{ID: GreedySwitch.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: "ONE_SIDED TWO_SIDED OFF"},
	{ID: GreedySwitchHierarchical.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: "ONE_SIDED TWO_SIDED OFF"},
	{ID: SemiInteractive.ID, Kind: core.BooleanOption, Targets: core.Parents},
	{ID: HierarchicalSweepiness.ID, Kind: core.DoubleOption, Targets: core.Parents},
	{ID: LayerConstraintOption.ID, Kind: core.EnumOption, Targets: core.Nodes, EnumValues: "NONE FIRST FIRST_SEPARATE LAST LAST_SEPARATE"},
	{ID: LayerIDOption.ID, Kind: core.IntegerOption, Targets: core.Nodes},
	{ID: PositionIDOption.ID, Kind: core.IntegerOption, Targets: core.Nodes},
	{ID: GeneratePositionAndLayerIDs.ID, Kind: core.BooleanOption, Targets: core.Parents},
	{ID: FeedbackEdges.ID, Kind: core.BooleanOption, Targets: core.Parents},
	{ID: InteractiveReferencePointOption.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: "CENTER TOP_LEFT"},
	{ID: DirectionCongruencyOption.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: "READING_DIRECTION ROTATION"},
	{ID: MergeEdges.ID, Kind: core.BooleanOption, Targets: core.Parents},
	{ID: MergeHierarchyEdges.ID, Kind: core.BooleanOption, Targets: core.Parents},
	{ID: SelfLoopDistribution.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: "EQUALLY NORTH NORTH_SOUTH"},
	{ID: SelfLoopOrdering.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: "STACKED REVERSE_STACKED SEQUENCED"},
	{ID: PolylineSlopedEdgeZoneWidth.ID, Kind: core.DoubleOption, Targets: core.Parents},
	{ID: SplineRoutingModeOption.ID, Group: "edgeRouting.splines", Kind: core.EnumOption, Targets: core.Parents, EnumValues: "SLOPPY CONSERVATIVE CONSERVATIVE_SOFT"},
	{ID: SplineSloppyLayerSpacingFactor.ID, Group: "edgeRouting.splines.sloppy", Kind: core.DoubleOption, Targets: core.Parents},
	{ID: UnnecessaryBendpoints.ID, Kind: core.BooleanOption, Targets: core.Parents},
	{ID: CenterEdgeLabelPlacement.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: "MEDIAN_LAYER TAIL_LAYER HEAD_LAYER SPACE_EFFICIENT_LAYER WIDEST_LAYER CENTER_LAYER"},
	{ID: EdgeLabelSideSelectionOption.ID, Kind: core.EnumOption, Targets: core.Parents, EnumValues: "ALWAYS_UP ALWAYS_DOWN DIRECTION_UP DIRECTION_DOWN SMART_UP SMART_DOWN"},

	{ID: PostCompactionStrategy.ID, Group: "compaction.postCompaction", Kind: core.EnumOption, Targets: core.Parents,
		EnumValues: "NONE LEFT RIGHT LEFT_RIGHT_CONSTRAINT_LOCKING LEFT_RIGHT_CONNECTION_LOCKING EDGE_LENGTH"},
	{ID: PostCompactionConstraints.ID, Group: "compaction.postCompaction", Kind: core.EnumOption, Targets: core.Parents,
		EnumValues: "QUADRATIC SCANLINE"},
	{ID: NoModelOrder.ID, Group: "considerModelOrder", Kind: core.BooleanOption, Targets: core.Nodes},
	{ID: PositionChoiceConstraint.ID, Group: "crossingMinimization", Kind: core.IntegerOption, Targets: core.Nodes},
	{ID: InLayerPredecessorOf.ID, Group: "crossingMinimization", Kind: core.StringOption, Targets: core.Nodes},
	{ID: InLayerSuccessorOf.ID, Group: "crossingMinimization", Kind: core.StringOption, Targets: core.Nodes},
	{ID: HighDegreeNodeTreatment.ID, Group: "highDegreeNodes", Kind: core.BooleanOption, Targets: core.Parents},
	{ID: HighDegreeNodeThreshold.ID, Group: "highDegreeNodes", Kind: core.IntegerOption, Targets: core.Parents},
	{ID: HighDegreeNodeTreeHeight.ID, Group: "highDegreeNodes", Kind: core.IntegerOption, Targets: core.Parents},
	{ID: LayerChoiceConstraint.ID, Group: "layering", Kind: core.IntegerOption, Targets: core.Nodes},
	{ID: NodePromotionStrategyOption.ID, Group: "layering.nodePromotion", Kind: core.EnumOption, Targets: core.Parents,
		EnumValues: "NONE NIKOLOV NIKOLOV_PIXEL NIKOLOV_IMPROVED NIKOLOV_IMPROVED_PIXEL DUMMYNODE_PERCENTAGE NODECOUNT_PERCENTAGE NO_BOUNDARY MODEL_ORDER_LEFT_TO_RIGHT MODEL_ORDER_RIGHT_TO_LEFT"},
	{ID: NodePromotionMaxIterations.ID, Group: "layering.nodePromotion", Kind: core.IntegerOption, Targets: core.Parents},
	{ID: PortSortingStrategyOption.ID, Kind: core.EnumOption, Targets: core.Parents,
		EnumValues: "INPUT_ORDER PORT_DEGREE"},
	{ID: WrappingStrategyOption.ID, Group: "wrapping", Kind: core.EnumOption, Targets: core.Parents,
		EnumValues: "OFF SINGLE_EDGE MULTI_EDGE"},
	{ID: WrappingAdditionalEdgeSpacing.ID, Group: "wrapping", Kind: core.DoubleOption, Targets: core.Parents},
	{ID: WrappingCorrectionFactor.ID, Group: "wrapping", Kind: core.DoubleOption, Targets: core.Parents},
	{ID: WrappingCuttingStrategy.ID, Group: "wrapping.cutting", Kind: core.EnumOption, Targets: core.Parents,
		EnumValues: "ARD MSD MANUAL"},
	{ID: WrappingCuttingCuts.ID, Group: "wrapping.cutting", Kind: core.ObjectOption, Targets: core.Parents, Parse: core.IntegerListParser},
	{ID: WrappingCuttingMSDFreedom.ID, Group: "wrapping.cutting.msd", Kind: core.IntegerOption, Targets: core.Parents},
	{ID: WrappingValidifyStrategy.ID, Group: "wrapping.validify", Kind: core.EnumOption, Targets: core.Parents,
		EnumValues: "NO GREEDY LOOK_BACK"},
	{ID: WrappingValidifyForbiddenIndices.ID, Group: "wrapping.validify", Kind: core.ObjectOption, Targets: core.Parents, Parse: core.IntegerListParser},
	{ID: WrappingMultiEdgeImproveCuts.ID, Group: "wrapping.multiEdge", Kind: core.BooleanOption, Targets: core.Parents},
	{ID: WrappingMultiEdgeDistancePenalty.ID, Group: "wrapping.multiEdge", Kind: core.DoubleOption, Targets: core.Parents},
	{ID: WrappingMultiEdgeImproveWrappedEdges.ID, Group: "wrapping.multiEdge", Kind: core.BooleanOption, Targets: core.Parents},
	{ID: LayerUnzippingStrategyOption.ID, Group: "layerUnzipping", Kind: core.EnumOption, Targets: core.Parents, EnumValues: "NONE ALTERNATING"},
	{ID: LayerUnzippingMinimizeEdgeLength.ID, Group: "layerUnzipping", Kind: core.BooleanOption, Targets: core.Nodes},
	{ID: LayerUnzippingLayerSplit.ID, Group: "layerUnzipping", Kind: core.IntegerOption, Targets: core.Nodes},
	{ID: LayerUnzippingResetOnLongEdges.ID, Group: "layerUnzipping", Kind: core.BooleanOption, Targets: core.Nodes},
	{ID: PriorityDirection.ID, Kind: core.IntegerOption, Targets: core.Edges},
	{ID: PriorityShortness.ID, Kind: core.IntegerOption, Targets: core.Edges},
	{ID: PriorityStraightness.ID, Kind: core.IntegerOption, Targets: core.Edges},

	{ID: SpacingComponentComponent.ID, Kind: core.DoubleOption, Targets: core.Parents},
	{ID: SpacingEdgeEdge.ID, Kind: core.DoubleOption, Targets: core.Parents},
	{ID: SpacingEdgeLabel.ID, Kind: core.DoubleOption, Targets: core.Parents},
	{ID: SpacingEdgeNode.ID, Kind: core.DoubleOption, Targets: core.Parents},
	{ID: SpacingLabelLabel.ID, Kind: core.DoubleOption, Targets: core.Parents},
	{ID: SpacingLabelNode.ID, Kind: core.DoubleOption, Targets: core.Parents},
	{ID: SpacingLabelPortHorizontal.ID, Kind: core.DoubleOption, Targets: core.Parents},
	{ID: SpacingLabelPortVertical.ID, Kind: core.DoubleOption, Targets: core.Parents},
	{ID: SpacingNodeNode.ID, Kind: core.DoubleOption, Targets: core.Parents},
	{ID: SpacingNodeSelfLoop.ID, Kind: core.DoubleOption, Targets: core.Parents},
	{ID: SpacingPortPort.ID, Kind: core.DoubleOption, Targets: core.Parents},
	{ID: SpacingPortsSurrounding.ID, Kind: core.ObjectOption, Targets: core.Parents, Parse: core.SpacingParser},
	{ID: SpacingBaseValue.ID, Kind: core.DoubleOption, Targets: core.Parents},
	{ID: SpacingEdgeNodeBetweenLayers.ID, Kind: core.DoubleOption, Targets: core.Parents, LegacyIDs: "spacing.edgeNodeBetweenLayers"},
	{ID: SpacingEdgeEdgeBetweenLayers.ID, Kind: core.DoubleOption, Targets: core.Parents, LegacyIDs: "spacing.edgeEdgeBetweenLayers org.eclipse.elk.spacing.edgeEdgeBetweenLayers"},
	{ID: SpacingNodeNodeBetweenLayers.ID, Kind: core.DoubleOption, Targets: core.Parents, LegacyIDs: "spacing.nodeNodeBetweenLayers"},
}

// RegisterOptions installs the ELK Layered 0.8.1 metadata used by the native
// provider. It is intentionally idempotent with the core registry: generated
// LayeredOptions re-export a number of CoreOptions under the same identifier.
func RegisterOptions(registry *core.OptionRegistry) {
	if registry == nil {
		return
	}
	for index := range optionSpecs {
		registry.RegisterMissing(optionSpecs[index])
	}
}
