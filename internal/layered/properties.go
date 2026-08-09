// Copyright (c) 2010, 2019 Kiel University and others.
// Copyright (c) 2015, 2020 Kiel University and others.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
)

// copyTypedProperties reproduces LGraphElement.copyProperties and then
// normalizes metadata-layer string enums to the strongly typed values used by
// the native pipeline.
func copyTypedProperties(target *basegraph.PropertyMap, source *basegraph.PropertyMap) {
	target.CopyFrom(source)
	setEnum(source, target, DirectionOption, ParseDirection)
	setEnum(source, target, PortSideOption, ParsePortSide)
	setEnum(source, target, Hierarchy, parseHierarchyHandling)
	setEnum(source, target, PortConstraintsOption, parsePortConstraints)
	setEnum(source, target, EdgeRoutingOption, parseEdgeRouting)
	setEnum(source, target, AlignmentOption, parseAlignment)
	setEnum(source, target, CycleBreaking, ParseCycleBreakingStrategy)
	setEnum(source, target, Layering, ParseLayeringStrategy)
	setEnum(source, target, CrossingMinimization, parseCrossingMinimization)
	setEnum(source, target, NodePlacement, parseNodePlacement)
	setEnum(source, target, NetworkSimplexNodeFlexibility, parseNodeFlexibility)
	setEnum(source, target, NetworkSimplexNodeFlexibilityDefault, parseNodeFlexibility)
	setEnum(source, target, BKFixedAlignment, parseFixedAlignment)
	setEnum(source, target, BKEdgeStraightening, parseEdgeStraightening)
	setEnum(source, target, SplineRoutingModeOption, parseSplineRoutingMode)
	setEnum(source, target, PostCompactionStrategy, parseGraphCompactionStrategy)
	setEnum(source, target, PostCompactionConstraints, parseConstraintCalculationStrategy)
	setEnum(source, target, NodePromotionStrategyOption, parseNodePromotionStrategy)
	setEnum(source, target, PortSortingStrategyOption, parsePortSortingStrategy)
	setEnum(source, target, WrappingStrategyOption, parseWrappingStrategy)
	setEnum(source, target, WrappingCuttingStrategy, parseCuttingStrategy)
	setEnum(source, target, WrappingValidifyStrategy, parseValidifyStrategy)
	setEnum(source, target, ConsiderModelOrder, parseOrderingStrategy)
	setEnum(source, target, GreedySwitch, parseGreedySwitch)
	setEnum(source, target, GreedySwitchHierarchical, parseGreedySwitch)
	setEnum(source, target, LayerConstraintOption, parseLayerConstraint)
	setEnum(source, target, InteractiveReferencePointOption, parseInteractiveReferencePoint)
	setEnum(source, target, DirectionCongruencyOption, parseDirectionCongruency)
	setEnum(source, target, EdgeLabelsPlacement, parseEdgeLabelPlacement)
	setEnum(source, target, LabelSideOption, parseLabelSide)
	setEnum(source, target, SelfLoopDistribution, parseSelfLoopDistribution)
	setEnum(source, target, SelfLoopOrdering, parseSelfLoopOrdering)
	setEnum(source, target, CenterEdgeLabelPlacement, parseCenterEdgeLabelPlacement)
	setEnum(source, target, EdgeLabelSideSelectionOption, parseEdgeLabelSideSelection)

	copyInt(source, target, RandomSeed)
	copyInt(source, target, Thoroughness)
	copyInt(source, target, CoffmanGrahamLayerBound)
	copyInt(source, target, MinWidthUpperBound)
	copyInt(source, target, MinWidthUpperLayerScaling)
	copyInt(source, target, GreedySwitchActivationThreshold)
	copyInt(source, target, PriorityDirection)
	copyInt(source, target, PriorityShortness)
	copyInt(source, target, PriorityStraightness)
	copyInt(source, target, PortIndex)
	copyInt(source, target, LayerIDOption)
	copyInt(source, target, PositionIDOption)
	copyInt(source, target, PositionChoiceConstraint)
	copyInt(source, target, LayerChoiceConstraint)
	copyInt(source, target, NodePromotionMaxIterations)
	copyInt(source, target, HighDegreeNodeThreshold)
	copyInt(source, target, HighDegreeNodeTreeHeight)
	copyInt(source, target, WrappingCuttingMSDFreedom)
	copyBool(source, target, NoLayout)
	copyBool(source, target, SeparateComponents)
	copyBool(source, target, EdgeLabelsInline)
	copyBool(source, target, FavorStraightEdges)
	copyBool(source, target, ForceNodeModelOrder)
	copyBool(source, target, SemiInteractive)
	copyBool(source, target, GeneratePositionAndLayerIDs)
	copyBool(source, target, FeedbackEdges)
	copyBool(source, target, MergeEdges)
	copyBool(source, target, MergeHierarchyEdges)
	copyBool(source, target, UnnecessaryBendpoints)
	copyBool(source, target, NoModelOrder)
	copyBool(source, target, HighDegreeNodeTreatment)
	copyBool(source, target, WrappingMultiEdgeImproveCuts)
	copyBool(source, target, WrappingMultiEdgeImproveWrappedEdges)

	for _, property := range []*basegraph.Property[float64]{
		&AspectRatio, &EdgeThickness, &PortBorderOffset,
		&LinearSegmentsDeflectionDampening, &HierarchicalSweepiness,
		&SpacingComponentComponent,
		&SpacingEdgeEdge, &SpacingEdgeLabel, &SpacingEdgeNode,
		&SpacingLabelLabel, &SpacingLabelNode, &SpacingLabelPortHorizontal,
		&SpacingLabelPortVertical, &SpacingNodeNode, &SpacingNodeSelfLoop,
		&SpacingPortPort, &SpacingBaseValue, &SpacingEdgeNodeBetweenLayers,
		&SpacingEdgeEdgeBetweenLayers, &SpacingNodeNodeBetweenLayers,
		&PolylineSlopedEdgeZoneWidth,
		&SplineSloppyLayerSpacingFactor,
		&WrappingAdditionalEdgeSpacing, &WrappingCorrectionFactor,
		&WrappingMultiEdgeDistancePenalty,
	} {
		copyFloat(source, target, *property)
	}

	// elkjs accepts these historical non-layered identifiers as unique
	// suffixes. The core codec preserves them exactly.
	copyFloatAlias(source, target, SpacingNodeNodeBetweenLayers,
		"org.eclipse.elk.spacing.nodeNodeBetweenLayers")
	copyFloatAlias(source, target, SpacingEdgeNodeBetweenLayers,
		"org.eclipse.elk.spacing.edgeNodeBetweenLayers")
	copyFloatAlias(source, target, SpacingEdgeEdgeBetweenLayers,
		"org.eclipse.elk.spacing.edgeEdgeBetweenLayers")
}

func setEnum[T any](
	source, target *basegraph.PropertyMap,
	property basegraph.Property[T],
	parse func(string) (T, bool),
) {
	value, found := source.Raw(property.ID)
	if !found {
		return
	}
	if typed, ok := value.(T); ok {
		basegraph.Set(target, property, typed)
		return
	}
	if parsed, ok := parse(fmt.Sprint(value)); ok {
		basegraph.Set(target, property, parsed)
	}
}

func copyInt(source, target *basegraph.PropertyMap, property basegraph.Property[int]) {
	value, found := source.Raw(property.ID)
	if !found {
		return
	}
	switch typed := value.(type) {
	case int:
		basegraph.Set(target, property, typed)
	case int32:
		basegraph.Set(target, property, int(typed))
	case int64:
		basegraph.Set(target, property, int(typed))
	case float64:
		basegraph.Set(target, property, int(typed))
	case string:
		if parsed, err := strconv.Atoi(typed); err == nil {
			basegraph.Set(target, property, parsed)
		}
	}
}

func copyBool(source, target *basegraph.PropertyMap, property basegraph.Property[bool]) {
	value, found := source.Raw(property.ID)
	if !found {
		return
	}
	switch typed := value.(type) {
	case bool:
		basegraph.Set(target, property, typed)
	case string:
		if parsed, err := strconv.ParseBool(typed); err == nil {
			basegraph.Set(target, property, parsed)
		}
	}
}

func copyFloat(source, target *basegraph.PropertyMap, property basegraph.Property[float64]) {
	copyFloatAlias(source, target, property, property.ID)
}

func copyFloatAlias(source, target *basegraph.PropertyMap, property basegraph.Property[float64], id string) {
	value, found := source.Raw(id)
	if !found {
		return
	}
	var result float64
	switch typed := value.(type) {
	case float64:
		result = typed
	case float32:
		result = float64(typed)
	case int:
		result = float64(typed)
	case int32:
		result = float64(typed)
	case int64:
		result = float64(typed)
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return
		}
		result = parsed
	default:
		return
	}
	basegraph.Set(target, property, result)
}

func parseHierarchyHandling(value string) (HierarchyHandling, bool) {
	switch strings.ToUpper(value) {
	case "INHERIT":
		return HierarchyInherit, true
	case "INCLUDE_CHILDREN":
		return HierarchyIncludeChildren, true
	case "SEPARATE_CHILDREN":
		return HierarchySeparateChildren, true
	default:
		return 0, false
	}
}

func parsePortConstraints(value string) (PortConstraints, bool) {
	switch strings.ToUpper(value) {
	case "UNDEFINED":
		return PortConstraintsUndefined, true
	case "FREE":
		return PortConstraintsFree, true
	case "FIXED_SIDE":
		return PortConstraintsFixedSide, true
	case "FIXED_ORDER":
		return PortConstraintsFixedOrder, true
	case "FIXED_RATIO":
		return PortConstraintsFixedRatio, true
	case "FIXED_POS":
		return PortConstraintsFixedPosition, true
	default:
		return 0, false
	}
}

func parseEdgeRouting(value string) (EdgeRouting, bool) {
	switch strings.ToUpper(value) {
	case "UNDEFINED":
		return EdgeRoutingUndefined, true
	case "POLYLINE":
		return EdgeRoutingPolyline, true
	case "ORTHOGONAL":
		return EdgeRoutingOrthogonal, true
	case "SPLINES":
		return EdgeRoutingSplines, true
	default:
		return 0, false
	}
}

func parseAlignment(value string) (Alignment, bool) {
	switch strings.ToUpper(value) {
	case "AUTOMATIC":
		return AlignmentAutomatic, true
	case "LEFT":
		return AlignmentLeft, true
	case "RIGHT":
		return AlignmentRight, true
	case "CENTER":
		return AlignmentCenter, true
	case "TOP":
		return AlignmentTop, true
	case "BOTTOM":
		return AlignmentBottom, true
	default:
		return 0, false
	}
}

func parseCrossingMinimization(value string) (CrossingMinimizationStrategy, bool) {
	switch strings.ToUpper(value) {
	case "LAYER_SWEEP":
		return CrossingMinimizationLayerSweep, true
	case "INTERACTIVE":
		return CrossingMinimizationInteractive, true
	case "NONE":
		return CrossingMinimizationNone, true
	default:
		return 0, false
	}
}

func parseNodePlacement(value string) (NodePlacementStrategy, bool) {
	switch strings.ToUpper(value) {
	case "SIMPLE":
		return NodePlacementSimple, true
	case "INTERACTIVE":
		return NodePlacementInteractive, true
	case "LINEAR_SEGMENTS":
		return NodePlacementLinearSegments, true
	case "BRANDES_KOEPF":
		return NodePlacementBrandesKoepf, true
	case "NETWORK_SIMPLEX":
		return NodePlacementNetworkSimplex, true
	default:
		return 0, false
	}
}

func parseNodeFlexibility(value string) (NodeFlexibility, bool) {
	switch strings.ToUpper(value) {
	case "NONE":
		return NodeFlexibilityNone, true
	case "PORT_POSITION":
		return NodeFlexibilityPortPosition, true
	case "NODE_SIZE_WHERE_SPACE_PERMITS":
		return NodeFlexibilitySizeWhereSpacePermits, true
	case "NODE_SIZE":
		return NodeFlexibilitySize, true
	default:
		return 0, false
	}
}

func parseInteractiveReferencePoint(value string) (InteractiveReferencePoint, bool) {
	switch strings.ToUpper(value) {
	case "CENTER":
		return InteractiveReferenceCenter, true
	case "TOP_LEFT":
		return InteractiveReferenceTopLeft, true
	default:
		return 0, false
	}
}

func parseFixedAlignment(value string) (FixedAlignment, bool) {
	switch strings.ToUpper(value) {
	case "NONE":
		return FixedAlignmentNone, true
	case "LEFTUP":
		return FixedAlignmentLeftUp, true
	case "RIGHTUP":
		return FixedAlignmentRightUp, true
	case "LEFTDOWN":
		return FixedAlignmentLeftDown, true
	case "RIGHTDOWN":
		return FixedAlignmentRightDown, true
	case "BALANCED":
		return FixedAlignmentBalanced, true
	default:
		return 0, false
	}
}

func parseEdgeStraightening(value string) (EdgeStraighteningStrategy, bool) {
	switch strings.ToUpper(value) {
	case "NONE":
		return EdgeStraighteningNone, true
	case "IMPROVE_STRAIGHTNESS":
		return EdgeStraighteningImprove, true
	default:
		return 0, false
	}
}

func parseSplineRoutingMode(value string) (SplineRoutingMode, bool) {
	switch strings.ToUpper(value) {
	case "SLOPPY":
		return SplineRoutingSloppy, true
	case "CONSERVATIVE":
		return SplineRoutingConservative, true
	default:
		return 0, false
	}
}

func parseGraphCompactionStrategy(value string) (GraphCompactionStrategy, bool) {
	values := []GraphCompactionStrategy{
		GraphCompactionNone, GraphCompactionLeft, GraphCompactionRight,
		GraphCompactionLeftRightConstraintLocking, GraphCompactionLeftRightConnectionLocking,
		GraphCompactionEdgeLength,
	}
	names := []string{
		"NONE", "LEFT", "RIGHT", "LEFT_RIGHT_CONSTRAINT_LOCKING",
		"LEFT_RIGHT_CONNECTION_LOCKING", "EDGE_LENGTH",
	}
	for index, name := range names {
		if strings.EqualFold(value, name) {
			return values[index], true
		}
	}
	return 0, false
}

func parseConstraintCalculationStrategy(value string) (ConstraintCalculationStrategy, bool) {
	switch strings.ToUpper(value) {
	case "QUADRATIC":
		return ConstraintCalculationQuadratic, true
	case "SCANLINE":
		return ConstraintCalculationScanline, true
	default:
		return 0, false
	}
}

func parseNodePromotionStrategy(value string) (NodePromotionStrategy, bool) {
	values := []NodePromotionStrategy{
		NodePromotionNone, NodePromotionNikolov, NodePromotionNikolovPixel,
		NodePromotionNikolovImproved, NodePromotionNikolovImprovedPixel,
		NodePromotionDummyNodePercentage, NodePromotionNodeCountPercentage,
		NodePromotionNoBoundary,
	}
	names := []string{
		"NONE", "NIKOLOV", "NIKOLOV_PIXEL", "NIKOLOV_IMPROVED",
		"NIKOLOV_IMPROVED_PIXEL", "DUMMYNODE_PERCENTAGE", "NODECOUNT_PERCENTAGE",
		"NO_BOUNDARY",
	}
	for index, name := range names {
		if strings.EqualFold(value, name) {
			return values[index], true
		}
	}
	return 0, false
}

func parsePortSortingStrategy(value string) (PortSortingStrategy, bool) {
	switch strings.ToUpper(value) {
	case "INPUT_ORDER":
		return PortSortingInputOrder, true
	case "PORT_DEGREE":
		return PortSortingPortDegree, true
	default:
		return 0, false
	}
}

func parseWrappingStrategy(value string) (WrappingStrategy, bool) {
	switch strings.ToUpper(value) {
	case "OFF":
		return WrappingOff, true
	case "SINGLE_EDGE":
		return WrappingSingleEdge, true
	case "MULTI_EDGE":
		return WrappingMultiEdge, true
	default:
		return 0, false
	}
}

func parseCuttingStrategy(value string) (CuttingStrategy, bool) {
	switch strings.ToUpper(value) {
	case "ARD":
		return CuttingARD, true
	case "MSD":
		return CuttingMSD, true
	case "MANUAL":
		return CuttingManual, true
	default:
		return 0, false
	}
}

func parseValidifyStrategy(value string) (ValidifyStrategy, bool) {
	switch strings.ToUpper(value) {
	case "NO":
		return ValidifyNo, true
	case "GREEDY":
		return ValidifyGreedy, true
	case "LOOK_BACK":
		return ValidifyLookBack, true
	default:
		return 0, false
	}
}

func parseOrderingStrategy(value string) (OrderingStrategy, bool) {
	switch strings.ToUpper(value) {
	case "NONE":
		return OrderingNone, true
	case "NODES_AND_EDGES":
		return OrderingNodesAndEdges, true
	case "PREFER_EDGES":
		return OrderingPreferEdges, true
	default:
		return 0, false
	}
}

func parseGreedySwitch(value string) (GreedySwitchType, bool) {
	switch strings.ToUpper(value) {
	case "ONE_SIDED":
		return GreedySwitchOneSided, true
	case "TWO_SIDED":
		return GreedySwitchTwoSided, true
	case "OFF":
		return GreedySwitchOff, true
	default:
		return 0, false
	}
}

func parseLayerConstraint(value string) (LayerConstraint, bool) {
	switch strings.ToUpper(value) {
	case "NONE":
		return LayerConstraintNone, true
	case "FIRST":
		return LayerConstraintFirst, true
	case "FIRST_SEPARATE":
		return LayerConstraintFirstSeparate, true
	case "LAST":
		return LayerConstraintLast, true
	case "LAST_SEPARATE":
		return LayerConstraintLastSeparate, true
	default:
		return 0, false
	}
}

func parseDirectionCongruency(value string) (DirectionCongruency, bool) {
	switch strings.ToUpper(value) {
	case "READING_DIRECTION":
		return DirectionCongruencyReading, true
	case "ROTATION":
		return DirectionCongruencyRotation, true
	default:
		return 0, false
	}
}

func parseEdgeLabelPlacement(value string) (EdgeLabelPlacement, bool) {
	switch strings.ToUpper(value) {
	case "CENTER":
		return EdgeLabelPlacementCenter, true
	case "HEAD":
		return EdgeLabelPlacementHead, true
	case "TAIL":
		return EdgeLabelPlacementTail, true
	default:
		return 0, false
	}
}

func parseLabelSide(value string) (LabelSide, bool) {
	switch strings.ToUpper(value) {
	case "UNKNOWN":
		return LabelSideUnknown, true
	case "ABOVE":
		return LabelSideAbove, true
	case "BELOW":
		return LabelSideBelow, true
	case "INLINE":
		return LabelSideInline, true
	default:
		return 0, false
	}
}

func parseSelfLoopDistribution(value string) (SelfLoopDistributionStrategy, bool) {
	switch strings.ToUpper(value) {
	case "EQUALLY":
		return SelfLoopDistributionEqually, true
	case "NORTH":
		return SelfLoopDistributionNorth, true
	case "NORTH_SOUTH":
		return SelfLoopDistributionNorthSouth, true
	default:
		return 0, false
	}
}

func parseSelfLoopOrdering(value string) (SelfLoopOrderingStrategy, bool) {
	switch strings.ToUpper(value) {
	case "STACKED":
		return SelfLoopOrderingStacked, true
	case "SEQUENCED":
		return SelfLoopOrderingSequenced, true
	default:
		return 0, false
	}
}

func parseCenterEdgeLabelPlacement(value string) (CenterEdgeLabelPlacementStrategy, bool) {
	values := []CenterEdgeLabelPlacementStrategy{
		CenterLabelMedianLayer, CenterLabelTailLayer, CenterLabelHeadLayer,
		CenterLabelSpaceEfficientLayer, CenterLabelWidestLayer, CenterLabelCenterLayer,
	}
	names := []string{"MEDIAN_LAYER", "TAIL_LAYER", "HEAD_LAYER", "SPACE_EFFICIENT_LAYER", "WIDEST_LAYER", "CENTER_LAYER"}
	for index, name := range names {
		if strings.EqualFold(value, name) {
			return values[index], true
		}
	}
	return 0, false
}

func parseEdgeLabelSideSelection(value string) (EdgeLabelSideSelection, bool) {
	values := []EdgeLabelSideSelection{
		EdgeLabelSideAlwaysUp, EdgeLabelSideAlwaysDown, EdgeLabelSideDirectionUp,
		EdgeLabelSideDirectionDown, EdgeLabelSideSmartUp, EdgeLabelSideSmartDown,
	}
	names := []string{"ALWAYS_UP", "ALWAYS_DOWN", "DIRECTION_UP", "DIRECTION_DOWN", "SMART_UP", "SMART_DOWN"}
	for index, name := range names {
		if strings.EqualFold(value, name) {
			return values[index], true
		}
	}
	return 0, false
}

func addSpacing(first, second geometry.Spacing) geometry.Spacing {
	return geometry.Spacing{
		Top: first.Top + second.Top, Right: first.Right + second.Right,
		Bottom: first.Bottom + second.Bottom, Left: first.Left + second.Left,
	}
}
