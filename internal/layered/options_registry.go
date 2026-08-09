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
	"fmt"
	"strconv"
	"strings"

	"github.com/d2lang/elk-go/internal/common/nodespacing"
	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
)

// RegisterOptions installs the ELK Layered 0.8.1 metadata used by the native
// provider. It is intentionally idempotent with the core registry: generated
// LayeredOptions re-export a number of CoreOptions under the same identifier.
func RegisterOptions(registry *core.OptionRegistry) {
	if registry == nil {
		return
	}
	parents := core.Parents
	allShapes := core.Parents | core.Nodes | core.Edges | core.Ports | core.Labels
	options := []core.OptionData{
		{ID: EdgeRoutingOption.ID, Kind: core.EnumOption, Targets: parents, EnumValues: []string{"UNDEFINED", "POLYLINE", "ORTHOGONAL", "SPLINES"}},
		{ID: RandomSeed.ID, Kind: core.IntegerOption, Targets: parents},
		{ID: SeparateComponents.ID, Kind: core.BooleanOption, Targets: parents},
		{ID: AspectRatio.ID, Kind: core.DoubleOption, Targets: parents},
		{ID: AlignmentOption.ID, Kind: core.EnumOption, Targets: core.Nodes, EnumValues: []string{"AUTOMATIC", "LEFT", "RIGHT", "CENTER", "TOP", "BOTTOM"}},
		{ID: PositionOption.ID, Kind: core.ObjectOption, Targets: allShapes, Parse: func(value string) (any, error) { return geometry.ParsePoint(value) }},
		{ID: PortBorderOffset.ID, Kind: core.DoubleOption, Targets: core.Ports},
		{ID: PortAnchor.ID, Kind: core.ObjectOption, Targets: core.Ports, Parse: func(value string) (any, error) { return geometry.ParsePoint(value) }},
		{ID: PortIndex.ID, Kind: core.IntegerOption, Targets: core.Ports},
		{ID: NodeLabelsPadding.ID, Kind: core.ObjectOption, Targets: core.Nodes, Parse: func(value string) (any, error) { return geometry.ParseSpacing(value) }},
		{ID: nodespacing.NodeLabelsPlacement.ID, Group: "nodeLabels", Kind: core.EnumSetOption, Targets: core.Nodes | core.Labels, EnumValues: []string{"H_LEFT", "H_CENTER", "H_RIGHT", "V_TOP", "V_CENTER", "V_BOTTOM", "INSIDE", "OUTSIDE", "H_PRIORITY"}},
		{ID: nodespacing.PortLabelsPlacement.ID, Group: "portLabels", Kind: core.EnumSetOption, Targets: core.Nodes, EnumValues: []string{"OUTSIDE", "INSIDE", "NEXT_TO_PORT_IF_POSSIBLE", "ALWAYS_SAME_SIDE", "ALWAYS_OTHER_SAME_SIDE", "SPACE_EFFICIENT"}},
		{ID: EdgeLabelsPlacement.ID, Kind: core.EnumOption, Targets: core.Labels, EnumValues: []string{"CENTER", "HEAD", "TAIL"}},
		{ID: EdgeThickness.ID, Kind: core.DoubleOption, Targets: core.Edges},
		{ID: JunctionPoints.ID, Kind: core.ObjectOption, Targets: core.Edges, Parse: func(value string) (any, error) { return geometry.ParseChain(value) }},

		{ID: CycleBreaking.ID, Kind: core.EnumOption, Targets: parents, EnumValues: []string{"GREEDY", "DEPTH_FIRST", "INTERACTIVE", "MODEL_ORDER", "GREEDY_MODEL_ORDER"}},
		{ID: Layering.ID, Kind: core.EnumOption, Targets: parents, EnumValues: []string{"NETWORK_SIMPLEX", "LONGEST_PATH", "COFFMAN_GRAHAM", "INTERACTIVE", "STRETCH_WIDTH", "MIN_WIDTH"}},
		{ID: CoffmanGrahamLayerBound.ID, Kind: core.IntegerOption, Targets: parents},
		{ID: MinWidthUpperBound.ID, Kind: core.IntegerOption, Targets: parents},
		{ID: MinWidthUpperLayerScaling.ID, Kind: core.IntegerOption, Targets: parents},
		{ID: CrossingMinimization.ID, Kind: core.EnumOption, Targets: parents, EnumValues: []string{"LAYER_SWEEP", "INTERACTIVE", "NONE"}},
		{ID: NodePlacement.ID, Kind: core.EnumOption, Targets: parents, EnumValues: []string{"SIMPLE", "INTERACTIVE", "LINEAR_SEGMENTS", "BRANDES_KOEPF", "NETWORK_SIMPLEX"}},
		{ID: FavorStraightEdges.ID, Kind: core.BooleanOption, Targets: parents},
		{ID: BKFixedAlignment.ID, Kind: core.EnumOption, Targets: parents, EnumValues: []string{"NONE", "LEFTUP", "RIGHTUP", "LEFTDOWN", "RIGHTDOWN", "BALANCED"}},
		{ID: BKEdgeStraightening.ID, Kind: core.EnumOption, Targets: parents, EnumValues: []string{"NONE", "IMPROVE_STRAIGHTNESS"}},
		{ID: LinearSegmentsDeflectionDampening.ID, Kind: core.DoubleOption, Targets: parents},
		{ID: NetworkSimplexNodeFlexibility.ID, Group: "nodePlacement.networkSimplex", Kind: core.EnumOption, Targets: core.Nodes, EnumValues: []string{"NONE", "PORT_POSITION", "NODE_SIZE_WHERE_SPACE_PERMITS", "NODE_SIZE"}},
		{ID: NetworkSimplexNodeFlexibilityDefault.ID, Group: "nodePlacement.networkSimplex.nodeFlexibility", Kind: core.EnumOption, Targets: parents, EnumValues: []string{"NONE", "PORT_POSITION", "NODE_SIZE_WHERE_SPACE_PERMITS", "NODE_SIZE"}},
		{ID: Thoroughness.ID, Kind: core.IntegerOption, Targets: parents},
		{ID: ForceNodeModelOrder.ID, Kind: core.BooleanOption, Targets: parents},
		{ID: ConsiderModelOrder.ID, Kind: core.EnumOption, Targets: parents, EnumValues: []string{"NONE", "NODES_AND_EDGES", "PREFER_EDGES"}},
		{ID: GreedySwitchActivationThreshold.ID, Kind: core.IntegerOption, Targets: parents},
		{ID: GreedySwitch.ID, Kind: core.EnumOption, Targets: parents, EnumValues: []string{"ONE_SIDED", "TWO_SIDED", "OFF"}},
		{ID: GreedySwitchHierarchical.ID, Kind: core.EnumOption, Targets: parents, EnumValues: []string{"ONE_SIDED", "TWO_SIDED", "OFF"}},
		{ID: SemiInteractive.ID, Kind: core.BooleanOption, Targets: parents},
		{ID: HierarchicalSweepiness.ID, Kind: core.DoubleOption, Targets: parents},
		{ID: LayerConstraintOption.ID, Kind: core.EnumOption, Targets: core.Nodes, EnumValues: []string{"NONE", "FIRST", "FIRST_SEPARATE", "LAST", "LAST_SEPARATE"}},
		{ID: LayerIDOption.ID, Kind: core.IntegerOption, Targets: core.Nodes},
		{ID: PositionIDOption.ID, Kind: core.IntegerOption, Targets: core.Nodes},
		{ID: GeneratePositionAndLayerIDs.ID, Kind: core.BooleanOption, Targets: parents},
		{ID: FeedbackEdges.ID, Kind: core.BooleanOption, Targets: parents},
		{ID: InteractiveReferencePointOption.ID, Kind: core.EnumOption, Targets: parents, EnumValues: []string{"CENTER", "TOP_LEFT"}},
		{ID: DirectionCongruencyOption.ID, Kind: core.EnumOption, Targets: parents, EnumValues: []string{"READING_DIRECTION", "ROTATION"}},
		{ID: MergeEdges.ID, Kind: core.BooleanOption, Targets: parents},
		{ID: MergeHierarchyEdges.ID, Kind: core.BooleanOption, Targets: parents},
		{ID: SelfLoopDistribution.ID, Kind: core.EnumOption, Targets: parents, EnumValues: []string{"EQUALLY", "NORTH", "NORTH_SOUTH"}},
		{ID: SelfLoopOrdering.ID, Kind: core.EnumOption, Targets: parents, EnumValues: []string{"STACKED", "SEQUENCED"}},
		{ID: PolylineSlopedEdgeZoneWidth.ID, Kind: core.DoubleOption, Targets: parents},
		{ID: SplineRoutingModeOption.ID, Group: "edgeRouting.splines", Kind: core.EnumOption, Targets: parents, EnumValues: []string{"SLOPPY", "CONSERVATIVE"}},
		{ID: SplineSloppyLayerSpacingFactor.ID, Group: "edgeRouting.splines.sloppy", Kind: core.DoubleOption, Targets: parents},
		{ID: UnnecessaryBendpoints.ID, Kind: core.BooleanOption, Targets: parents},
		{ID: CenterEdgeLabelPlacement.ID, Kind: core.EnumOption, Targets: parents, EnumValues: []string{"MEDIAN_LAYER", "TAIL_LAYER", "HEAD_LAYER", "SPACE_EFFICIENT_LAYER", "WIDEST_LAYER", "CENTER_LAYER"}},
		{ID: EdgeLabelSideSelectionOption.ID, Kind: core.EnumOption, Targets: parents, EnumValues: []string{"ALWAYS_UP", "ALWAYS_DOWN", "DIRECTION_UP", "DIRECTION_DOWN", "SMART_UP", "SMART_DOWN"}},

		{ID: PostCompactionStrategy.ID, Group: "compaction.postCompaction", Kind: core.EnumOption, Targets: parents,
			EnumValues: []string{"NONE", "LEFT", "RIGHT", "LEFT_RIGHT_CONSTRAINT_LOCKING", "LEFT_RIGHT_CONNECTION_LOCKING", "EDGE_LENGTH"}},
		{ID: PostCompactionConstraints.ID, Group: "compaction.postCompaction", Kind: core.EnumOption, Targets: parents,
			EnumValues: []string{"QUADRATIC", "SCANLINE"}},
		{ID: NoModelOrder.ID, Group: "considerModelOrder", Kind: core.BooleanOption, Targets: core.Nodes},
		{ID: PositionChoiceConstraint.ID, Group: "crossingMinimization", Kind: core.IntegerOption, Targets: core.Nodes},
		{ID: HighDegreeNodeTreatment.ID, Group: "highDegreeNodes", Kind: core.BooleanOption, Targets: parents},
		{ID: HighDegreeNodeThreshold.ID, Group: "highDegreeNodes", Kind: core.IntegerOption, Targets: parents},
		{ID: HighDegreeNodeTreeHeight.ID, Group: "highDegreeNodes", Kind: core.IntegerOption, Targets: parents},
		{ID: LayerChoiceConstraint.ID, Group: "layering", Kind: core.IntegerOption, Targets: core.Nodes},
		{ID: NodePromotionStrategyOption.ID, Group: "layering.nodePromotion", Kind: core.EnumOption, Targets: parents,
			EnumValues: []string{"NONE", "NIKOLOV", "NIKOLOV_PIXEL", "NIKOLOV_IMPROVED", "NIKOLOV_IMPROVED_PIXEL", "DUMMYNODE_PERCENTAGE", "NODECOUNT_PERCENTAGE", "NO_BOUNDARY"}},
		{ID: NodePromotionMaxIterations.ID, Group: "layering.nodePromotion", Kind: core.IntegerOption, Targets: parents},
		{ID: PortSortingStrategyOption.ID, Kind: core.EnumOption, Targets: parents,
			EnumValues: []string{"INPUT_ORDER", "PORT_DEGREE"}},
		{ID: WrappingStrategyOption.ID, Group: "wrapping", Kind: core.EnumOption, Targets: parents,
			EnumValues: []string{"OFF", "SINGLE_EDGE", "MULTI_EDGE"}},
		{ID: WrappingAdditionalEdgeSpacing.ID, Group: "wrapping", Kind: core.DoubleOption, Targets: parents},
		{ID: WrappingCorrectionFactor.ID, Group: "wrapping", Kind: core.DoubleOption, Targets: parents},
		{ID: WrappingCuttingStrategy.ID, Group: "wrapping.cutting", Kind: core.EnumOption, Targets: parents,
			EnumValues: []string{"ARD", "MSD", "MANUAL"}},
		{ID: WrappingCuttingCuts.ID, Group: "wrapping.cutting", Kind: core.ObjectOption, Targets: parents, Parse: parseIntegerList},
		{ID: WrappingCuttingMSDFreedom.ID, Group: "wrapping.cutting.msd", Kind: core.IntegerOption, Targets: parents},
		{ID: WrappingValidifyStrategy.ID, Group: "wrapping.validify", Kind: core.EnumOption, Targets: parents,
			EnumValues: []string{"NO", "GREEDY", "LOOK_BACK"}},
		{ID: WrappingValidifyForbiddenIndices.ID, Group: "wrapping.validify", Kind: core.ObjectOption, Targets: parents, Parse: parseIntegerList},
		{ID: WrappingMultiEdgeImproveCuts.ID, Group: "wrapping.multiEdge", Kind: core.BooleanOption, Targets: parents},
		{ID: WrappingMultiEdgeDistancePenalty.ID, Group: "wrapping.multiEdge", Kind: core.DoubleOption, Targets: parents},
		{ID: WrappingMultiEdgeImproveWrappedEdges.ID, Group: "wrapping.multiEdge", Kind: core.BooleanOption, Targets: parents},
		{ID: PriorityDirection.ID, Kind: core.IntegerOption, Targets: core.Edges},
		{ID: PriorityShortness.ID, Kind: core.IntegerOption, Targets: core.Edges},
		{ID: PriorityStraightness.ID, Kind: core.IntegerOption, Targets: core.Edges},

		{ID: SpacingComponentComponent.ID, Kind: core.DoubleOption, Targets: parents},
		{ID: SpacingEdgeEdge.ID, Kind: core.DoubleOption, Targets: parents},
		{ID: SpacingEdgeLabel.ID, Kind: core.DoubleOption, Targets: parents},
		{ID: SpacingEdgeNode.ID, Kind: core.DoubleOption, Targets: parents},
		{ID: SpacingLabelLabel.ID, Kind: core.DoubleOption, Targets: parents},
		{ID: SpacingLabelNode.ID, Kind: core.DoubleOption, Targets: parents},
		{ID: SpacingLabelPortHorizontal.ID, Kind: core.DoubleOption, Targets: parents},
		{ID: SpacingLabelPortVertical.ID, Kind: core.DoubleOption, Targets: parents},
		{ID: SpacingNodeNode.ID, Kind: core.DoubleOption, Targets: parents},
		{ID: SpacingNodeSelfLoop.ID, Kind: core.DoubleOption, Targets: parents},
		{ID: SpacingPortPort.ID, Kind: core.DoubleOption, Targets: parents},
		{ID: SpacingPortsSurrounding.ID, Kind: core.ObjectOption, Targets: parents, Parse: func(value string) (any, error) { return geometry.ParseSpacing(value) }},
		{ID: SpacingBaseValue.ID, Kind: core.DoubleOption, Targets: parents},
		{ID: SpacingEdgeNodeBetweenLayers.ID, Kind: core.DoubleOption, Targets: parents, LegacyIDs: []string{"spacing.edgeNodeBetweenLayers"}},
		{ID: SpacingEdgeEdgeBetweenLayers.ID, Kind: core.DoubleOption, Targets: parents, LegacyIDs: []string{"spacing.edgeEdgeBetweenLayers", "org.eclipse.elk.spacing.edgeEdgeBetweenLayers"}},
		{ID: SpacingNodeNodeBetweenLayers.ID, Kind: core.DoubleOption, Targets: parents, LegacyIDs: []string{"spacing.nodeNodeBetweenLayers"}},
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

func parseIntegerList(value string) (any, error) {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "[")
	trimmed = strings.TrimSuffix(trimmed, "]")
	if strings.TrimSpace(trimmed) == "" {
		return []int{}, nil
	}
	fields := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\r' || r == '\n'
	})
	result := make([]int, 0, len(fields))
	for _, field := range fields {
		parsed, err := strconv.Atoi(field)
		if err != nil {
			return nil, fmt.Errorf("invalid integer list %q: %w", value, err)
		}
		result = append(result, parsed)
	}
	return result, nil
}
