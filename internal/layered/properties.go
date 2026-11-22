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
//
// It is one pass over the source entries: an entry whose key has a typed
// converter is stored converted (or raw when the conversion fails), any other
// entry is stored cloned. This equals copying every entry and then re-setting
// each converted property in place: entry positions are fixed by the order in
// which keys are first inserted, every key is converted independently, and
// every converted type is a value type whose clone is itself. The aliases are
// applied last, as before, because they write a different key.
func copyTypedProperties(target *basegraph.PropertyMap, source *basegraph.PropertyMap) {
	target.CopyFromConverting(source, typedPropertyConverters)

	// elkjs accepts these historical non-layered identifiers as unique
	// suffixes. The core codec preserves them exactly.
	for _, alias := range typedFloatAliases {
		if value, found := source.RawKey(alias.key); found {
			if converted, ok := convertFloat(value); ok {
				target.SetRawKey(alias.target, converted)
			}
		}
	}
}

type propertyConverter = basegraph.PropertyConverter

type floatAlias struct {
	key, target basegraph.PropertyKey
}

var (
	typedPropertyConverters = buildTypedPropertyConverters()
	typedFloatAliases       = []floatAlias{
		{basegraph.InternPropertyKey("org.eclipse.elk.spacing.nodeNodeBetweenLayers"), SpacingNodeNodeBetweenLayers.Key()},
		{basegraph.InternPropertyKey("org.eclipse.elk.spacing.edgeNodeBetweenLayers"), SpacingEdgeNodeBetweenLayers.Key()},
		{basegraph.InternPropertyKey("org.eclipse.elk.spacing.edgeEdgeBetweenLayers"), SpacingEdgeEdgeBetweenLayers.Key()},
	}
)

// buildTypedPropertyConverters indexes the converters by PropertyKey. Every
// key listed here was interned by its property's declaration, so keys interned
// later can never need a converter.
func buildTypedPropertyConverters() []propertyConverter {
	var converters []propertyConverter
	add := func(key basegraph.PropertyKey, convert propertyConverter) {
		for int(key) >= len(converters) {
			converters = append(converters, nil)
		}
		if converters[key] != nil {
			panic("layered: duplicate typed property converter")
		}
		converters[key] = convert
	}
	addEnum(add, DirectionOption, ParseDirection)
	addEnum(add, PortSideOption, ParsePortSide)
	addEnum(add, Hierarchy, parseHierarchyHandling)
	addEnum(add, PortConstraintsOption, parsePortConstraints)
	addEnum(add, EdgeRoutingOption, parseEdgeRouting)
	addEnum(add, AlignmentOption, parseAlignment)
	addEnum(add, CycleBreaking, ParseCycleBreakingStrategy)
	addEnum(add, Layering, ParseLayeringStrategy)
	addEnum(add, CrossingMinimization, parseCrossingMinimization)
	addEnum(add, NodePlacement, parseNodePlacement)
	addEnum(add, NetworkSimplexRecomputeNodePlacement, parseNodePlacement)
	addEnum(add, NetworkSimplexNodeFlexibility, parseNodeFlexibility)
	addEnum(add, NetworkSimplexNodeFlexibilityDefault, parseNodeFlexibility)
	addEnum(add, BKFixedAlignment, parseFixedAlignment)
	addEnum(add, BKEdgeStraightening, parseEdgeStraightening)
	addEnum(add, SplineRoutingModeOption, parseSplineRoutingMode)
	addEnum(add, PostCompactionStrategy, parseGraphCompactionStrategy)
	addEnum(add, PostCompactionConstraints, parseConstraintCalculationStrategy)
	addEnum(add, NodePromotionStrategyOption, parseNodePromotionStrategy)
	addEnum(add, PortSortingStrategyOption, parsePortSortingStrategy)
	addEnum(add, WrappingStrategyOption, parseWrappingStrategy)
	addEnum(add, WrappingCuttingStrategy, parseCuttingStrategy)
	addEnum(add, WrappingValidifyStrategy, parseValidifyStrategy)
	addEnum(add, ConsiderModelOrder, parseOrderingStrategy)
	addEnum(add, ComponentOrdering, parseComponentOrderingStrategy)
	addEnum(add, LongEdgeOrdering, parseLongEdgeOrderingStrategy)
	addEnum(add, CycleBreakingGroupOrder, parseGroupOrderStrategy)
	addEnum(add, CrossingMinimizationGroupOrder, parseGroupOrderStrategy)
	addEnum(add, LayerUnzippingStrategyOption, parseLayerUnzippingStrategy)
	addEnum(add, GreedySwitch, parseGreedySwitch)
	addEnum(add, GreedySwitchHierarchical, parseGreedySwitch)
	addEnum(add, LayerConstraintOption, parseLayerConstraint)
	addEnum(add, InteractiveReferencePointOption, parseInteractiveReferencePoint)
	addEnum(add, DirectionCongruencyOption, parseDirectionCongruency)
	addEnum(add, EdgeLabelsPlacement, parseEdgeLabelPlacement)
	addEnum(add, LabelSideOption, parseLabelSide)
	addEnum(add, SelfLoopDistribution, parseSelfLoopDistribution)
	addEnum(add, SelfLoopOrdering, parseSelfLoopOrdering)
	addEnum(add, CenterEdgeLabelPlacement, parseCenterEdgeLabelPlacement)
	addEnum(add, EdgeLabelSideSelectionOption, parseEdgeLabelSideSelection)

	for _, property := range []*basegraph.Property[int]{
		&RandomSeed, &Thoroughness, &CoffmanGrahamLayerBound, &MinWidthUpperBound,
		&MinWidthUpperLayerScaling, &GreedySwitchActivationThreshold, &PriorityDirection,
		&PriorityShortness, &PriorityStraightness, &PortIndex, &LayerIDOption,
		&PositionIDOption, &PositionChoiceConstraint, &LayerChoiceConstraint,
		&NodePromotionMaxIterations, &HighDegreeNodeThreshold, &HighDegreeNodeTreeHeight,
		&LayerUnzippingLayerSplit, &CycleBreakingGroupID, &CrossingMinimizationGroupID,
		&ComponentGroupID, &CycleBreakingPreferredSourceID, &CycleBreakingPreferredTargetID,
		&WrappingCuttingMSDFreedom,
	} {
		add(property.Key(), convertInt)
	}
	for _, property := range []*basegraph.Property[bool]{
		&NoLayout, &SeparateComponents, &EdgeLabelsInline, &FavorStraightEdges,
		&ForceNodeModelOrder, &SemiInteractive, &GeneratePositionAndLayerIDs, &FeedbackEdges,
		&MergeEdges, &MergeHierarchyEdges, &UnnecessaryBendpoints, &NoModelOrder,
		&PortModelOrder, &LayerUnzippingMinimizeEdgeLength, &LayerUnzippingResetOnLongEdges,
		&HighDegreeNodeTreatment, &WrappingMultiEdgeImproveCuts, &WrappingMultiEdgeImproveWrappedEdges,
	} {
		add(property.Key(), convertBool)
	}
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
		&CrossingCounterNodeInfluence, &CrossingCounterPortInfluence,
	} {
		add(property.Key(), convertFloat)
	}
	return converters
}

// addEnum registers setEnum's conversion: a value of the enum type is kept,
// anything else is parsed from its fmt.Sprint spelling.
func addEnum[T any](
	add func(basegraph.PropertyKey, propertyConverter),
	property basegraph.Property[T],
	parse func(string) (T, bool),
) {
	add(property.Key(), func(value any) (any, bool) {
		if _, ok := value.(T); ok {
			return value, true
		}
		text, isString := value.(string)
		if !isString {
			text = fmt.Sprint(value)
		}
		if parsed, ok := parse(text); ok {
			return parsed, true
		}
		return nil, false
	})
}

// convertInt is copyInt's conversion.
func convertInt(value any) (any, bool) {
	switch typed := value.(type) {
	case int:
		return value, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case string:
		if parsed, err := strconv.Atoi(typed); err == nil {
			return parsed, true
		}
	}
	return nil, false
}

// convertBool is copyBool's conversion.
func convertBool(value any) (any, bool) {
	switch typed := value.(type) {
	case bool:
		return value, true
	case string:
		if parsed, err := strconv.ParseBool(typed); err == nil {
			return parsed, true
		}
	}
	return nil, false
}

// convertFloat is copyFloatAlias's conversion.
func convertFloat(value any) (any, bool) {
	switch typed := value.(type) {
	case float64:
		return value, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return nil, false
		}
		return parsed, true
	}
	return nil, false
}

func parseHierarchyHandling(value string) (HierarchyHandling, bool) {
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseHierarchyHandlingUpper(value); ok {
		return parsed, true
	}
	return parseHierarchyHandlingUpper(strings.ToUpper(value))
}

func parseHierarchyHandlingUpper(value string) (HierarchyHandling, bool) {
	switch value {
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
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parsePortConstraintsUpper(value); ok {
		return parsed, true
	}
	return parsePortConstraintsUpper(strings.ToUpper(value))
}

func parsePortConstraintsUpper(value string) (PortConstraints, bool) {
	switch value {
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
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseEdgeRoutingUpper(value); ok {
		return parsed, true
	}
	return parseEdgeRoutingUpper(strings.ToUpper(value))
}

func parseEdgeRoutingUpper(value string) (EdgeRouting, bool) {
	switch value {
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
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseAlignmentUpper(value); ok {
		return parsed, true
	}
	return parseAlignmentUpper(strings.ToUpper(value))
}

func parseAlignmentUpper(value string) (Alignment, bool) {
	switch value {
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
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseCrossingMinimizationUpper(value); ok {
		return parsed, true
	}
	return parseCrossingMinimizationUpper(strings.ToUpper(value))
}

func parseCrossingMinimizationUpper(value string) (CrossingMinimizationStrategy, bool) {
	switch value {
	case "LAYER_SWEEP":
		return CrossingMinimizationLayerSweep, true
	case "MEDIAN_LAYER_SWEEP":
		return CrossingMinimizationMedianLayerSweep, true
	case "INTERACTIVE":
		return CrossingMinimizationInteractive, true
	case "NONE":
		return CrossingMinimizationNone, true
	default:
		return 0, false
	}
}

func parseNodePlacement(value string) (NodePlacementStrategy, bool) {
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseNodePlacementUpper(value); ok {
		return parsed, true
	}
	return parseNodePlacementUpper(strings.ToUpper(value))
}

func parseNodePlacementUpper(value string) (NodePlacementStrategy, bool) {
	switch value {
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
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseNodeFlexibilityUpper(value); ok {
		return parsed, true
	}
	return parseNodeFlexibilityUpper(strings.ToUpper(value))
}

func parseNodeFlexibilityUpper(value string) (NodeFlexibility, bool) {
	switch value {
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
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseInteractiveReferencePointUpper(value); ok {
		return parsed, true
	}
	return parseInteractiveReferencePointUpper(strings.ToUpper(value))
}

func parseInteractiveReferencePointUpper(value string) (InteractiveReferencePoint, bool) {
	switch value {
	case "CENTER":
		return InteractiveReferenceCenter, true
	case "TOP_LEFT":
		return InteractiveReferenceTopLeft, true
	default:
		return 0, false
	}
}

func parseFixedAlignment(value string) (FixedAlignment, bool) {
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseFixedAlignmentUpper(value); ok {
		return parsed, true
	}
	return parseFixedAlignmentUpper(strings.ToUpper(value))
}

func parseFixedAlignmentUpper(value string) (FixedAlignment, bool) {
	switch value {
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
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseEdgeStraighteningUpper(value); ok {
		return parsed, true
	}
	return parseEdgeStraighteningUpper(strings.ToUpper(value))
}

func parseEdgeStraighteningUpper(value string) (EdgeStraighteningStrategy, bool) {
	switch value {
	case "NONE":
		return EdgeStraighteningNone, true
	case "IMPROVE_STRAIGHTNESS":
		return EdgeStraighteningImprove, true
	default:
		return 0, false
	}
}

func parseSplineRoutingMode(value string) (SplineRoutingMode, bool) {
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseSplineRoutingModeUpper(value); ok {
		return parsed, true
	}
	return parseSplineRoutingModeUpper(strings.ToUpper(value))
}

func parseSplineRoutingModeUpper(value string) (SplineRoutingMode, bool) {
	switch value {
	case "SLOPPY":
		return SplineRoutingSloppy, true
	case "CONSERVATIVE":
		return SplineRoutingConservative, true
	case "CONSERVATIVE_SOFT":
		return SplineRoutingConservativeSoft, true
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
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseConstraintCalculationStrategyUpper(value); ok {
		return parsed, true
	}
	return parseConstraintCalculationStrategyUpper(strings.ToUpper(value))
}

func parseConstraintCalculationStrategyUpper(value string) (ConstraintCalculationStrategy, bool) {
	switch value {
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
		NodePromotionNoBoundary, NodePromotionModelOrderLeftToRight,
		NodePromotionModelOrderRightToLeft,
	}
	names := []string{
		"NONE", "NIKOLOV", "NIKOLOV_PIXEL", "NIKOLOV_IMPROVED",
		"NIKOLOV_IMPROVED_PIXEL", "DUMMYNODE_PERCENTAGE", "NODECOUNT_PERCENTAGE",
		"NO_BOUNDARY", "MODEL_ORDER_LEFT_TO_RIGHT", "MODEL_ORDER_RIGHT_TO_LEFT",
	}
	for index, name := range names {
		if strings.EqualFold(value, name) {
			return values[index], true
		}
	}
	return 0, false
}

func parsePortSortingStrategy(value string) (PortSortingStrategy, bool) {
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parsePortSortingStrategyUpper(value); ok {
		return parsed, true
	}
	return parsePortSortingStrategyUpper(strings.ToUpper(value))
}

func parsePortSortingStrategyUpper(value string) (PortSortingStrategy, bool) {
	switch value {
	case "INPUT_ORDER":
		return PortSortingInputOrder, true
	case "PORT_DEGREE":
		return PortSortingPortDegree, true
	default:
		return 0, false
	}
}

func parseWrappingStrategy(value string) (WrappingStrategy, bool) {
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseWrappingStrategyUpper(value); ok {
		return parsed, true
	}
	return parseWrappingStrategyUpper(strings.ToUpper(value))
}

func parseWrappingStrategyUpper(value string) (WrappingStrategy, bool) {
	switch value {
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
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseCuttingStrategyUpper(value); ok {
		return parsed, true
	}
	return parseCuttingStrategyUpper(strings.ToUpper(value))
}

func parseCuttingStrategyUpper(value string) (CuttingStrategy, bool) {
	switch value {
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
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseValidifyStrategyUpper(value); ok {
		return parsed, true
	}
	return parseValidifyStrategyUpper(strings.ToUpper(value))
}

func parseValidifyStrategyUpper(value string) (ValidifyStrategy, bool) {
	switch value {
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
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseOrderingStrategyUpper(value); ok {
		return parsed, true
	}
	return parseOrderingStrategyUpper(strings.ToUpper(value))
}

func parseOrderingStrategyUpper(value string) (OrderingStrategy, bool) {
	switch value {
	case "NONE":
		return OrderingNone, true
	case "NODES_AND_EDGES":
		return OrderingNodesAndEdges, true
	case "PREFER_EDGES":
		return OrderingPreferEdges, true
	case "PREFER_NODES":
		return OrderingPreferNodes, true
	default:
		return 0, false
	}
}

func parseLongEdgeOrderingStrategy(value string) (LongEdgeOrderingStrategy, bool) {
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseLongEdgeOrderingStrategyUpper(value); ok {
		return parsed, true
	}
	return parseLongEdgeOrderingStrategyUpper(strings.ToUpper(value))
}

func parseLongEdgeOrderingStrategyUpper(value string) (LongEdgeOrderingStrategy, bool) {
	switch value {
	case "DUMMY_NODE_OVER":
		return LongEdgeDummyNodeOver, true
	case "DUMMY_NODE_UNDER":
		return LongEdgeDummyNodeUnder, true
	case "EQUAL":
		return LongEdgeEqual, true
	default:
		return 0, false
	}
}

func parseComponentOrderingStrategy(value string) (ComponentOrderingStrategy, bool) {
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseComponentOrderingStrategyUpper(value); ok {
		return parsed, true
	}
	return parseComponentOrderingStrategyUpper(strings.ToUpper(value))
}

func parseComponentOrderingStrategyUpper(value string) (ComponentOrderingStrategy, bool) {
	switch value {
	case "NONE":
		return ComponentOrderingNone, true
	case "INSIDE_PORT_SIDE_GROUPS":
		return ComponentOrderingInsidePortSideGroups, true
	case "GROUP_MODEL_ORDER":
		return ComponentOrderingGroupModelOrder, true
	case "MODEL_ORDER":
		return ComponentOrderingModelOrder, true
	default:
		return 0, false
	}
}

func parseGroupOrderStrategy(value string) (GroupOrderStrategy, bool) {
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseGroupOrderStrategyUpper(value); ok {
		return parsed, true
	}
	return parseGroupOrderStrategyUpper(strings.ToUpper(value))
}

func parseGroupOrderStrategyUpper(value string) (GroupOrderStrategy, bool) {
	switch value {
	case "ONLY_WITHIN_GROUP":
		return GroupOrderOnlyWithinGroup, true
	case "MODEL_ORDER":
		return GroupOrderModelOrder, true
	case "ENFORCED":
		return GroupOrderEnforced, true
	default:
		return 0, false
	}
}

func parseLayerUnzippingStrategy(value string) (LayerUnzippingStrategy, bool) {
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseLayerUnzippingStrategyUpper(value); ok {
		return parsed, true
	}
	return parseLayerUnzippingStrategyUpper(strings.ToUpper(value))
}

func parseLayerUnzippingStrategyUpper(value string) (LayerUnzippingStrategy, bool) {
	switch value {
	case "NONE":
		return LayerUnzippingNone, true
	case "ALTERNATING":
		return LayerUnzippingAlternating, true
	default:
		return 0, false
	}
}

func parseGreedySwitch(value string) (GreedySwitchType, bool) {
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseGreedySwitchUpper(value); ok {
		return parsed, true
	}
	return parseGreedySwitchUpper(strings.ToUpper(value))
}

func parseGreedySwitchUpper(value string) (GreedySwitchType, bool) {
	switch value {
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
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseLayerConstraintUpper(value); ok {
		return parsed, true
	}
	return parseLayerConstraintUpper(strings.ToUpper(value))
}

func parseLayerConstraintUpper(value string) (LayerConstraint, bool) {
	switch value {
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
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseDirectionCongruencyUpper(value); ok {
		return parsed, true
	}
	return parseDirectionCongruencyUpper(strings.ToUpper(value))
}

func parseDirectionCongruencyUpper(value string) (DirectionCongruency, bool) {
	switch value {
	case "READING_DIRECTION":
		return DirectionCongruencyReading, true
	case "ROTATION":
		return DirectionCongruencyRotation, true
	default:
		return 0, false
	}
}

func parseEdgeLabelPlacement(value string) (EdgeLabelPlacement, bool) {
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseEdgeLabelPlacementUpper(value); ok {
		return parsed, true
	}
	return parseEdgeLabelPlacementUpper(strings.ToUpper(value))
}

func parseEdgeLabelPlacementUpper(value string) (EdgeLabelPlacement, bool) {
	switch value {
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
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseLabelSideUpper(value); ok {
		return parsed, true
	}
	return parseLabelSideUpper(strings.ToUpper(value))
}

func parseLabelSideUpper(value string) (LabelSide, bool) {
	switch value {
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
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseSelfLoopDistributionUpper(value); ok {
		return parsed, true
	}
	return parseSelfLoopDistributionUpper(strings.ToUpper(value))
}

func parseSelfLoopDistributionUpper(value string) (SelfLoopDistributionStrategy, bool) {
	switch value {
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
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := parseSelfLoopOrderingUpper(value); ok {
		return parsed, true
	}
	return parseSelfLoopOrderingUpper(strings.ToUpper(value))
}

func parseSelfLoopOrderingUpper(value string) (SelfLoopOrderingStrategy, bool) {
	switch value {
	case "STACKED":
		return SelfLoopOrderingStacked, true
	case "REVERSE_STACKED":
		return SelfLoopOrderingReverseStacked, true
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
