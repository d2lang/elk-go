// Copyright (c) 2009, 2015 Kiel University and others.
// Copyright (c) 2009, 2020 Kiel University and others.
// Copyright (c) 2010, 2015 Kiel University and others.
// Copyright (c) 2010, 2017 Kiel University and others.
// Copyright (c) 2011, 2015 Kiel University and others.
// Copyright (c) 2012, 2015 Kiel University and others.
// Copyright (c) 2012, 2017 Kiel University and others.
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
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"fmt"
	"strings"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
)

type Direction uint8

const (
	DirectionUndefined Direction = iota
	DirectionRight
	DirectionLeft
	DirectionDown
	DirectionUp
)

type HierarchyHandling uint8

const (
	HierarchyInherit HierarchyHandling = iota
	HierarchyIncludeChildren
	HierarchySeparateChildren
)

type PortConstraints uint8

const (
	PortConstraintsUndefined PortConstraints = iota
	PortConstraintsFree
	PortConstraintsFixedSide
	PortConstraintsFixedOrder
	PortConstraintsFixedRatio
	PortConstraintsFixedPosition
)

func (constraints PortConstraints) IsSideFixed() bool {
	return constraints >= PortConstraintsFixedSide
}

func (constraints PortConstraints) IsOrderFixed() bool {
	return constraints >= PortConstraintsFixedOrder
}

func (constraints PortConstraints) IsRatioFixed() bool {
	return constraints == PortConstraintsFixedRatio
}

func (constraints PortConstraints) IsPositionFixed() bool {
	return constraints >= PortConstraintsFixedPosition
}

type EdgeRouting uint8

const (
	EdgeRoutingUndefined EdgeRouting = iota
	EdgeRoutingPolyline
	EdgeRoutingOrthogonal
	EdgeRoutingSplines
)

// InteractiveReferencePoint selects the position compared by ELK's
// interactive cycle-breaking, crossing-minimization, and node-placement
// phases. The declaration order matches ELK 0.8.1.
type InteractiveReferencePoint uint8

const (
	InteractiveReferenceCenter InteractiveReferencePoint = iota
	InteractiveReferenceTopLeft
)

type Alignment uint8

const (
	AlignmentAutomatic Alignment = iota
	AlignmentLeft
	AlignmentRight
	AlignmentCenter
	AlignmentTop
	AlignmentBottom
)

type CycleBreakingStrategy uint8

const (
	CycleBreakingGreedy CycleBreakingStrategy = iota
	CycleBreakingDepthFirst
	CycleBreakingInteractive
	CycleBreakingModelOrder
	CycleBreakingGreedyModelOrder
	CycleBreakingSCCConnectivity
	CycleBreakingSCCNodeType
	CycleBreakingDFSNodeOrder
	CycleBreakingBFSNodeOrder
)

type LayeringStrategy uint8

const (
	LayeringNetworkSimplex LayeringStrategy = iota
	LayeringLongestPath
	LayeringLongestPathSource
	LayeringCoffmanGraham
	LayeringInteractive
	LayeringStretchWidth
	LayeringMinWidth
	LayeringBreadthFirstModelOrder
	LayeringDepthFirstModelOrder
)

type CrossingMinimizationStrategy uint8

const (
	CrossingMinimizationLayerSweep CrossingMinimizationStrategy = iota
	CrossingMinimizationMedianLayerSweep
	CrossingMinimizationInteractive
	CrossingMinimizationNone
)

type NodePlacementStrategy uint8

const (
	NodePlacementSimple NodePlacementStrategy = iota
	NodePlacementInteractive
	NodePlacementLinearSegments
	NodePlacementBrandesKoepf
	NodePlacementNetworkSimplex
)

// NodeFlexibility matches ELK 0.8.1's NetworkSimplexPlacer flexibility
// levels. Declaration order is source-significant.
type NodeFlexibility uint8

const (
	NodeFlexibilityNone NodeFlexibility = iota
	NodeFlexibilityPortPosition
	NodeFlexibilitySizeWhereSpacePermits
	NodeFlexibilitySize
)

func (flexibility NodeFlexibility) IsFlexibleSize() bool {
	return flexibility == NodeFlexibilitySize
}

func (flexibility NodeFlexibility) IsFlexibleSizeWhereSpacePermits() bool {
	return flexibility == NodeFlexibilitySizeWhereSpacePermits ||
		flexibility == NodeFlexibilitySize
}

func (flexibility NodeFlexibility) IsFlexiblePorts() bool {
	return flexibility == NodeFlexibilityPortPosition ||
		flexibility == NodeFlexibilitySizeWhereSpacePermits ||
		flexibility == NodeFlexibilitySize
}

type FixedAlignment uint8

const (
	FixedAlignmentNone FixedAlignment = iota
	FixedAlignmentLeftUp
	FixedAlignmentRightUp
	FixedAlignmentLeftDown
	FixedAlignmentRightDown
	FixedAlignmentBalanced
)

type EdgeStraighteningStrategy uint8

const (
	EdgeStraighteningNone EdgeStraighteningStrategy = iota
	EdgeStraighteningImprove
)

type SplineRoutingMode uint8

const (
	SplineRoutingSloppy SplineRoutingMode = iota
	SplineRoutingConservative
	SplineRoutingConservativeSoft
)

type GraphCompactionStrategy uint8

const (
	GraphCompactionNone GraphCompactionStrategy = iota
	GraphCompactionLeft
	GraphCompactionRight
	GraphCompactionLeftRightConstraintLocking
	GraphCompactionLeftRightConnectionLocking
	GraphCompactionEdgeLength
)

type ConstraintCalculationStrategy uint8

const (
	ConstraintCalculationQuadratic ConstraintCalculationStrategy = iota
	ConstraintCalculationScanline
)

type NodePromotionStrategy uint8

const (
	NodePromotionNone NodePromotionStrategy = iota
	NodePromotionNikolov
	NodePromotionNikolovPixel
	NodePromotionNikolovImproved
	NodePromotionNikolovImprovedPixel
	NodePromotionDummyNodePercentage
	NodePromotionNodeCountPercentage
	NodePromotionNoBoundary
	NodePromotionModelOrderLeftToRight
	NodePromotionModelOrderRightToLeft
)

type PortSortingStrategy uint8

const (
	PortSortingInputOrder PortSortingStrategy = iota
	PortSortingPortDegree
)

type WrappingStrategy uint8

const (
	WrappingOff WrappingStrategy = iota
	WrappingSingleEdge
	WrappingMultiEdge
)

type CuttingStrategy uint8

const (
	CuttingARD CuttingStrategy = iota
	CuttingMSD
	CuttingManual
)

type ValidifyStrategy uint8

const (
	ValidifyNo ValidifyStrategy = iota
	ValidifyGreedy
	ValidifyLookBack
)

type DirectionCongruency uint8

const (
	DirectionCongruencyReading DirectionCongruency = iota
	DirectionCongruencyRotation
)

type OrderingStrategy uint8

const (
	OrderingNone OrderingStrategy = iota
	OrderingNodesAndEdges
	OrderingPreferEdges
	OrderingPreferNodes
)

type LongEdgeOrderingStrategy uint8

const (
	LongEdgeDummyNodeOver LongEdgeOrderingStrategy = iota
	LongEdgeDummyNodeUnder
	LongEdgeEqual
)

type ComponentOrderingStrategy uint8

const (
	ComponentOrderingNone ComponentOrderingStrategy = iota
	ComponentOrderingInsidePortSideGroups
	ComponentOrderingGroupModelOrder
	ComponentOrderingModelOrder
)

type GroupOrderStrategy uint8

const (
	GroupOrderOnlyWithinGroup GroupOrderStrategy = iota
	GroupOrderModelOrder
	GroupOrderEnforced
)

type LayerUnzippingStrategy uint8

const (
	LayerUnzippingNone LayerUnzippingStrategy = iota
	LayerUnzippingAlternating
)

type GreedySwitchType uint8

const (
	GreedySwitchOneSided GreedySwitchType = iota
	GreedySwitchTwoSided
	GreedySwitchOff
)

type LayerConstraint uint8

const (
	LayerConstraintNone LayerConstraint = iota
	LayerConstraintFirst
	LayerConstraintFirstSeparate
	LayerConstraintLast
	LayerConstraintLastSeparate
)

type InLayerConstraint uint8

const (
	InLayerConstraintNone InLayerConstraint = iota
	InLayerConstraintTop
	InLayerConstraintBottom
)

type EdgeConstraint uint8

const (
	EdgeConstraintNone EdgeConstraint = iota
	EdgeConstraintIncomingOnly
	EdgeConstraintOutgoingOnly
)

type GraphProperty uint16

const (
	GraphPropertyComments GraphProperty = 1 << iota
	GraphPropertyExternalPorts
	GraphPropertyHyperedges
	GraphPropertyHypernodes
	GraphPropertyNonFreePorts
	GraphPropertyNorthSouthPorts
	GraphPropertySelfLoops
	GraphPropertyCenterLabels
	GraphPropertyEndLabels
	GraphPropertyPartitions
)

type PortType uint8

const (
	PortTypeUndefined PortType = iota
	PortTypeInput
	PortTypeOutput
)

type SelfLoopDistributionStrategy uint8

const (
	SelfLoopDistributionEqually SelfLoopDistributionStrategy = iota
	SelfLoopDistributionNorth
	SelfLoopDistributionNorthSouth
)

type SelfLoopOrderingStrategy uint8

const (
	SelfLoopOrderingStacked SelfLoopOrderingStrategy = iota
	SelfLoopOrderingReverseStacked
	SelfLoopOrderingSequenced
)

type EdgeLabelPlacement uint8

const (
	EdgeLabelPlacementCenter EdgeLabelPlacement = iota
	EdgeLabelPlacementHead
	EdgeLabelPlacementTail
)

type LabelSide uint8

const (
	LabelSideUnknown LabelSide = iota
	LabelSideAbove
	LabelSideBelow
	LabelSideInline
)

type CenterEdgeLabelPlacementStrategy uint8

const (
	CenterLabelMedianLayer CenterEdgeLabelPlacementStrategy = iota
	CenterLabelTailLayer
	CenterLabelHeadLayer
	CenterLabelSpaceEfficientLayer
	CenterLabelWidestLayer
	CenterLabelCenterLayer
)

type EdgeLabelSideSelection uint8

const (
	EdgeLabelSideAlwaysUp EdgeLabelSideSelection = iota
	EdgeLabelSideAlwaysDown
	EdgeLabelSideDirectionUp
	EdgeLabelSideDirectionDown
	EdgeLabelSideSmartUp
	EdgeLabelSideSmartDown
)

// IndividualSpacings is the property-holder value used by
// org.eclipse.elk.spacing.individual.
type IndividualSpacings struct{ Properties basegraph.PropertyMap }

func (spacings IndividualSpacings) Clone() any {
	return IndividualSpacings{Properties: spacings.Properties.Clone()}
}

var (
	// Core options inherited by ELK Layered. Defaults are the effective
	// Layered 0.8.1 defaults, not the generic core metadata defaults.
	Algorithm             = basegraph.PropertyWithDefault("org.eclipse.elk.algorithm", func() string { return "layered" })
	DirectionOption       = basegraph.PropertyWithDefault("org.eclipse.elk.direction", func() Direction { return DirectionUndefined })
	Hierarchy             = basegraph.PropertyWithDefault("org.eclipse.elk.hierarchyHandling", func() HierarchyHandling { return HierarchySeparateChildren })
	Padding               = basegraph.CloneablePropertyWithConstantDefault("org.eclipse.elk.padding", func() geometry.Spacing { return geometry.UniformSpacing(12) })
	PortSideOption        = basegraph.PropertyWithDefault("org.eclipse.elk.port.side", func() PortSide { return UndefinedSide })
	PortConstraintsOption = basegraph.PropertyWithDefault("org.eclipse.elk.portConstraints", func() PortConstraints { return PortConstraintsUndefined })
	PortBorderOffset      = basegraph.PropertyWithDefault("org.eclipse.elk.port.borderOffset", func() float64 { return 0 })
	PortAnchor            = basegraph.NewProperty[geometry.Point]("org.eclipse.elk.port.anchor")
	PortIndex             = basegraph.NewProperty[int]("org.eclipse.elk.port.index")
	EdgeRoutingOption     = basegraph.PropertyWithDefault("org.eclipse.elk.edgeRouting", func() EdgeRouting { return EdgeRoutingOrthogonal })
	NoLayout              = basegraph.PropertyWithDefault("org.eclipse.elk.noLayout", func() bool { return false })
	RandomSeed            = basegraph.PropertyWithDefault("org.eclipse.elk.randomSeed", func() int { return 1 })
	SeparateComponents    = basegraph.PropertyWithDefault("org.eclipse.elk.separateConnectedComponents", func() bool { return true })
	AspectRatio           = basegraph.PropertyWithDefault("org.eclipse.elk.aspectRatio", func() float64 { return 1.6 })
	AlignmentOption       = basegraph.PropertyWithDefault("org.eclipse.elk.alignment", func() Alignment { return AlignmentAutomatic })
	PositionOption        = basegraph.NewProperty[geometry.Point]("org.eclipse.elk.position")
	NodeSizeMinimum       = basegraph.CloneablePropertyWithConstantDefault("org.eclipse.elk.nodeSize.minimum", func() geometry.Point { return geometry.Point{} })
	NodeSizeConstraints   = basegraph.CloneablePropertyWithConstantDefault("org.eclipse.elk.nodeSize.constraints", func() []string { return nil })
	ContentAlignment      = basegraph.CloneablePropertyWithConstantDefault("org.eclipse.elk.contentAlignment", func() []string { return []string{"V_TOP", "H_LEFT"} })
	NodeLabelsPadding     = basegraph.CloneablePropertyWithConstantDefault("org.eclipse.elk.nodeLabels.padding", func() geometry.Spacing { return geometry.UniformSpacing(5) })
	EdgeLabelsInline      = basegraph.PropertyWithDefault("org.eclipse.elk.edgeLabels.inline", func() bool { return false })
	EdgeLabelsPlacement   = basegraph.PropertyWithDefault("org.eclipse.elk.edgeLabels.placement", func() EdgeLabelPlacement { return EdgeLabelPlacementCenter })
	LabelSideOption       = basegraph.PropertyWithDefault("org.eclipse.elk.labelSide", func() LabelSide { return LabelSideUnknown })
	EdgeThickness         = basegraph.PropertyWithDefault("org.eclipse.elk.edge.thickness", func() float64 { return 1 })
	JunctionPoints        = basegraph.CloneablePropertyWithDefault("org.eclipse.elk.junctionPoints", func() geometry.Chain { return nil })

	// Layered strategy options.
	CycleBreaking                        = basegraph.PropertyWithDefault("org.eclipse.elk.layered.cycleBreaking.strategy", func() CycleBreakingStrategy { return CycleBreakingGreedy })
	Layering                             = basegraph.PropertyWithDefault("org.eclipse.elk.layered.layering.strategy", func() LayeringStrategy { return LayeringNetworkSimplex })
	CoffmanGrahamLayerBound              = basegraph.PropertyWithDefault("org.eclipse.elk.layered.layering.coffmanGraham.layerBound", func() int { return int(^uint32(0) >> 1) })
	MinWidthUpperBound                   = basegraph.PropertyWithDefault("org.eclipse.elk.layered.layering.minWidth.upperBoundOnWidth", func() int { return 4 })
	MinWidthUpperLayerScaling            = basegraph.PropertyWithDefault("org.eclipse.elk.layered.layering.minWidth.upperLayerEstimationScalingFactor", func() int { return 2 })
	CrossingMinimization                 = basegraph.PropertyWithDefault("org.eclipse.elk.layered.crossingMinimization.strategy", func() CrossingMinimizationStrategy { return CrossingMinimizationLayerSweep })
	NodePlacement                        = basegraph.PropertyWithDefault("org.eclipse.elk.layered.nodePlacement.strategy", func() NodePlacementStrategy { return NodePlacementBrandesKoepf })
	FavorStraightEdges                   = basegraph.NewProperty[bool]("org.eclipse.elk.layered.nodePlacement.favorStraightEdges")
	BKFixedAlignment                     = basegraph.PropertyWithDefault("org.eclipse.elk.layered.nodePlacement.bk.fixedAlignment", func() FixedAlignment { return FixedAlignmentNone })
	BKEdgeStraightening                  = basegraph.PropertyWithDefault("org.eclipse.elk.layered.nodePlacement.bk.edgeStraightening", func() EdgeStraighteningStrategy { return EdgeStraighteningImprove })
	LinearSegmentsDeflectionDampening    = basegraph.PropertyWithDefault("org.eclipse.elk.layered.nodePlacement.linearSegments.deflectionDampening", func() float64 { return 0.3 })
	NetworkSimplexNodeFlexibility        = basegraph.NewProperty[NodeFlexibility]("org.eclipse.elk.layered.nodePlacement.networkSimplex.nodeFlexibility")
	NetworkSimplexNodeFlexibilityDefault = basegraph.PropertyWithDefault("org.eclipse.elk.layered.nodePlacement.networkSimplex.nodeFlexibility.default", func() NodeFlexibility { return NodeFlexibilityNone })
	NetworkSimplexRecomputeNodePlacement = basegraph.NewProperty[NodePlacementStrategy]("org.eclipse.elk.layered.nodePlacement.networkSimplex.nodeFlexibility.recomputeNodePlacement")
	LayerChoiceConstraint                = basegraph.NewProperty[int]("org.eclipse.elk.layered.layering.layerChoiceConstraint")
	NodePromotionStrategyOption          = basegraph.PropertyWithDefault("org.eclipse.elk.layered.layering.nodePromotion.strategy", func() NodePromotionStrategy { return NodePromotionNone })
	NodePromotionMaxIterations           = basegraph.PropertyWithDefault("org.eclipse.elk.layered.layering.nodePromotion.maxIterations", func() int { return 0 })
	Thoroughness                         = basegraph.PropertyWithDefault("org.eclipse.elk.layered.thoroughness", func() int { return 7 })
	ForceNodeModelOrder                  = basegraph.PropertyWithDefault("org.eclipse.elk.layered.crossingMinimization.forceNodeModelOrder", func() bool { return false })
	ConsiderModelOrder                   = basegraph.PropertyWithDefault("org.eclipse.elk.layered.considerModelOrder.strategy", func() OrderingStrategy { return OrderingNone })
	NoModelOrder                         = basegraph.PropertyWithDefault("org.eclipse.elk.layered.considerModelOrder.noModelOrder", func() bool { return false })
	PortModelOrder                       = basegraph.PropertyWithDefault("org.eclipse.elk.layered.considerModelOrder.portModelOrder", func() bool { return false })
	ComponentOrdering                    = basegraph.PropertyWithDefault("org.eclipse.elk.layered.considerModelOrder.components", func() ComponentOrderingStrategy { return ComponentOrderingNone })
	LongEdgeOrdering                     = basegraph.PropertyWithDefault("org.eclipse.elk.layered.considerModelOrder.longEdgeStrategy", func() LongEdgeOrderingStrategy { return LongEdgeDummyNodeOver })
	CrossingCounterNodeInfluence         = basegraph.PropertyWithDefault("org.eclipse.elk.layered.considerModelOrder.crossingCounterNodeInfluence", func() float64 { return 0 })
	CrossingCounterPortInfluence         = basegraph.PropertyWithDefault("org.eclipse.elk.layered.considerModelOrder.crossingCounterPortInfluence", func() float64 { return 0 })
	CycleBreakingGroupID                 = basegraph.PropertyWithDefault("org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cycleBreakingId", func() int { return 0 })
	CrossingMinimizationGroupID          = basegraph.PropertyWithDefault("org.eclipse.elk.layered.considerModelOrder.groupModelOrder.crossingMinimizationId", func() int { return 0 })
	ComponentGroupID                     = basegraph.PropertyWithDefault("org.eclipse.elk.layered.considerModelOrder.groupModelOrder.componentGroupId", func() int { return 0 })
	CycleBreakingGroupOrder              = basegraph.PropertyWithDefault("org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cbGroupOrderStrategy", func() GroupOrderStrategy { return GroupOrderOnlyWithinGroup })
	CycleBreakingPreferredSourceID       = basegraph.NewProperty[int]("org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cbPreferredSourceId")
	CycleBreakingPreferredTargetID       = basegraph.NewProperty[int]("org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cbPreferredTargetId")
	CrossingMinimizationGroupOrder       = basegraph.PropertyWithDefault("org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cmGroupOrderStrategy", func() GroupOrderStrategy { return GroupOrderOnlyWithinGroup })
	CrossingMinimizationEnforcedGroups   = basegraph.CloneablePropertyWithDefault("org.eclipse.elk.layered.considerModelOrder.groupModelOrder.cmEnforcedGroupOrders", func() []int { return []int{1, 2, 6, 7, 10, 11} })
	GreedySwitchActivationThreshold      = basegraph.PropertyWithDefault("org.eclipse.elk.layered.crossingMinimization.greedySwitch.activationThreshold", func() int { return 40 })
	GreedySwitch                         = basegraph.PropertyWithDefault("org.eclipse.elk.layered.crossingMinimization.greedySwitch.type", func() GreedySwitchType { return GreedySwitchTwoSided })
	GreedySwitchHierarchical             = basegraph.PropertyWithDefault("org.eclipse.elk.layered.crossingMinimization.greedySwitchHierarchical.type", func() GreedySwitchType { return GreedySwitchOff })
	SemiInteractive                      = basegraph.PropertyWithDefault("org.eclipse.elk.layered.crossingMinimization.semiInteractive", func() bool { return false })
	HierarchicalSweepiness               = basegraph.PropertyWithDefault("org.eclipse.elk.layered.crossingMinimization.hierarchicalSweepiness", func() float64 { return 0.1 })
	LayerConstraintOption                = basegraph.PropertyWithDefault("org.eclipse.elk.layered.layering.layerConstraint", func() LayerConstraint { return LayerConstraintNone })
	LayerIDOption                        = basegraph.PropertyWithDefault("org.eclipse.elk.layered.layering.layerId", func() int { return -1 })
	PositionIDOption                     = basegraph.PropertyWithDefault("org.eclipse.elk.layered.crossingMinimization.positionId", func() int { return -1 })
	PositionChoiceConstraint             = basegraph.NewProperty[int]("org.eclipse.elk.layered.crossingMinimization.positionChoiceConstraint")
	InLayerPredecessorOf                 = basegraph.NewProperty[string]("org.eclipse.elk.layered.crossingMinimization.inLayerPredOf")
	InLayerSuccessorOf                   = basegraph.NewProperty[string]("org.eclipse.elk.layered.crossingMinimization.inLayerSuccOf")
	GeneratePositionAndLayerIDs          = basegraph.PropertyWithDefault("org.eclipse.elk.layered.generatePositionAndLayerIds", func() bool { return false })
	FeedbackEdges                        = basegraph.PropertyWithDefault("org.eclipse.elk.layered.feedbackEdges", func() bool { return false })
	InteractiveReferencePointOption      = basegraph.PropertyWithDefault("org.eclipse.elk.layered.interactiveReferencePoint", func() InteractiveReferencePoint { return InteractiveReferenceCenter })
	DirectionCongruencyOption            = basegraph.PropertyWithDefault("org.eclipse.elk.layered.directionCongruency", func() DirectionCongruency { return DirectionCongruencyReading })
	MergeEdges                           = basegraph.PropertyWithDefault("org.eclipse.elk.layered.mergeEdges", func() bool { return false })
	MergeHierarchyEdges                  = basegraph.PropertyWithDefault("org.eclipse.elk.layered.mergeHierarchyEdges", func() bool { return true })
	SelfLoopDistribution                 = basegraph.PropertyWithDefault("org.eclipse.elk.layered.edgeRouting.selfLoopDistribution", func() SelfLoopDistributionStrategy { return SelfLoopDistributionNorth })
	SelfLoopOrdering                     = basegraph.PropertyWithDefault("org.eclipse.elk.layered.edgeRouting.selfLoopOrdering", func() SelfLoopOrderingStrategy { return SelfLoopOrderingStacked })
	PolylineSlopedEdgeZoneWidth          = basegraph.PropertyWithDefault("org.eclipse.elk.layered.edgeRouting.polyline.slopedEdgeZoneWidth", func() float64 { return 2 })
	SplineRoutingModeOption              = basegraph.PropertyWithDefault("org.eclipse.elk.layered.edgeRouting.splines.mode", func() SplineRoutingMode { return SplineRoutingSloppy })
	SplineSloppyLayerSpacingFactor       = basegraph.PropertyWithDefault("org.eclipse.elk.layered.edgeRouting.splines.sloppy.layerSpacingFactor", func() float64 { return 0.2 })
	PostCompactionStrategy               = basegraph.PropertyWithDefault("org.eclipse.elk.layered.compaction.postCompaction.strategy", func() GraphCompactionStrategy { return GraphCompactionNone })
	PostCompactionConstraints            = basegraph.PropertyWithDefault("org.eclipse.elk.layered.compaction.postCompaction.constraints", func() ConstraintCalculationStrategy { return ConstraintCalculationScanline })
	HighDegreeNodeTreatment              = basegraph.PropertyWithDefault("org.eclipse.elk.layered.highDegreeNodes.treatment", func() bool { return false })
	HighDegreeNodeThreshold              = basegraph.PropertyWithDefault("org.eclipse.elk.layered.highDegreeNodes.threshold", func() int { return 16 })
	HighDegreeNodeTreeHeight             = basegraph.PropertyWithDefault("org.eclipse.elk.layered.highDegreeNodes.treeHeight", func() int { return 5 })
	PortSortingStrategyOption            = basegraph.PropertyWithDefault("org.eclipse.elk.layered.portSortingStrategy", func() PortSortingStrategy { return PortSortingInputOrder })
	WrappingStrategyOption               = basegraph.PropertyWithDefault("org.eclipse.elk.layered.wrapping.strategy", func() WrappingStrategy { return WrappingOff })
	WrappingAdditionalEdgeSpacing        = basegraph.PropertyWithDefault("org.eclipse.elk.layered.wrapping.additionalEdgeSpacing", func() float64 { return 10 })
	WrappingCorrectionFactor             = basegraph.PropertyWithDefault("org.eclipse.elk.layered.wrapping.correctionFactor", func() float64 { return 1 })
	WrappingCuttingStrategy              = basegraph.PropertyWithDefault("org.eclipse.elk.layered.wrapping.cutting.strategy", func() CuttingStrategy { return CuttingMSD })
	WrappingCuttingCuts                  = basegraph.NewProperty[[]int]("org.eclipse.elk.layered.wrapping.cutting.cuts")
	WrappingCuttingMSDFreedom            = basegraph.PropertyWithDefault("org.eclipse.elk.layered.wrapping.cutting.msd.freedom", func() int { return 1 })
	WrappingValidifyStrategy             = basegraph.PropertyWithDefault("org.eclipse.elk.layered.wrapping.validify.strategy", func() ValidifyStrategy { return ValidifyGreedy })
	WrappingValidifyForbiddenIndices     = basegraph.NewProperty[[]int]("org.eclipse.elk.layered.wrapping.validify.forbiddenIndices")
	WrappingMultiEdgeImproveCuts         = basegraph.PropertyWithDefault("org.eclipse.elk.layered.wrapping.multiEdge.improveCuts", func() bool { return true })
	WrappingMultiEdgeDistancePenalty     = basegraph.PropertyWithDefault("org.eclipse.elk.layered.wrapping.multiEdge.distancePenalty", func() float64 { return 2 })
	WrappingMultiEdgeImproveWrappedEdges = basegraph.PropertyWithDefault("org.eclipse.elk.layered.wrapping.multiEdge.improveWrappedEdges", func() bool { return true })
	LayerUnzippingStrategyOption         = basegraph.PropertyWithDefault("org.eclipse.elk.layered.layerUnzipping.strategy", func() LayerUnzippingStrategy { return LayerUnzippingNone })
	LayerUnzippingMinimizeEdgeLength     = basegraph.PropertyWithDefault("org.eclipse.elk.layered.layerUnzipping.minimizeEdgeLength", func() bool { return false })
	LayerUnzippingLayerSplit             = basegraph.PropertyWithDefault("org.eclipse.elk.layered.layerUnzipping.layerSplit", func() int { return 2 })
	LayerUnzippingResetOnLongEdges       = basegraph.PropertyWithDefault("org.eclipse.elk.layered.layerUnzipping.resetOnLongEdges", func() bool { return true })
	UnnecessaryBendpoints                = basegraph.PropertyWithDefault("org.eclipse.elk.layered.unnecessaryBendpoints", func() bool { return false })
	CenterEdgeLabelPlacement             = basegraph.PropertyWithDefault("org.eclipse.elk.layered.edgeLabels.centerLabelPlacementStrategy", func() CenterEdgeLabelPlacementStrategy { return CenterLabelMedianLayer })
	EdgeLabelSideSelectionOption         = basegraph.PropertyWithDefault("org.eclipse.elk.layered.edgeLabels.sideSelection", func() EdgeLabelSideSelection { return EdgeLabelSideSmartDown })

	// Priorities.
	PriorityDirection    = basegraph.PropertyWithDefault("org.eclipse.elk.layered.priority.direction", func() int { return 0 })
	PriorityShortness    = basegraph.PropertyWithDefault("org.eclipse.elk.layered.priority.shortness", func() int { return 0 })
	PriorityStraightness = basegraph.PropertyWithDefault("org.eclipse.elk.layered.priority.straightness", func() int { return 0 })

	// Spacing options. The `spacing.*BetweenLayers` aliases accepted by
	// elkjs are resolved by the metadata registry before import.
	SpacingIndividual            = basegraph.NewProperty[IndividualSpacings]("org.eclipse.elk.spacing.individual")
	SpacingComponentComponent    = basegraph.PropertyWithDefault("org.eclipse.elk.spacing.componentComponent", func() float64 { return 20 })
	SpacingEdgeEdge              = basegraph.PropertyWithDefault("org.eclipse.elk.spacing.edgeEdge", func() float64 { return 10 })
	SpacingEdgeLabel             = basegraph.PropertyWithDefault("org.eclipse.elk.spacing.edgeLabel", func() float64 { return 2 })
	SpacingEdgeNode              = basegraph.PropertyWithDefault("org.eclipse.elk.spacing.edgeNode", func() float64 { return 10 })
	SpacingLabelLabel            = basegraph.PropertyWithDefault("org.eclipse.elk.spacing.labelLabel", func() float64 { return 0 })
	SpacingLabelNode             = basegraph.PropertyWithDefault("org.eclipse.elk.spacing.labelNode", func() float64 { return 5 })
	SpacingLabelPortHorizontal   = basegraph.PropertyWithDefault("org.eclipse.elk.spacing.labelPortHorizontal", func() float64 { return 1 })
	SpacingLabelPortVertical     = basegraph.PropertyWithDefault("org.eclipse.elk.spacing.labelPortVertical", func() float64 { return 1 })
	SpacingNodeNode              = basegraph.PropertyWithDefault("org.eclipse.elk.spacing.nodeNode", func() float64 { return 20 })
	SpacingNodeSelfLoop          = basegraph.PropertyWithDefault("org.eclipse.elk.spacing.nodeSelfLoop", func() float64 { return 10 })
	SpacingPortPort              = basegraph.PropertyWithDefault("org.eclipse.elk.spacing.portPort", func() float64 { return 10 })
	SpacingPortsSurrounding      = basegraph.CloneablePropertyWithConstantDefault("org.eclipse.elk.spacing.portsSurrounding", func() geometry.Spacing { return geometry.Spacing{} })
	SpacingBaseValue             = basegraph.NewProperty[float64]("org.eclipse.elk.layered.spacing.baseValue")
	SpacingEdgeNodeBetweenLayers = basegraph.PropertyWithDefault("org.eclipse.elk.layered.spacing.edgeNodeBetweenLayers", func() float64 { return 10 })
	SpacingEdgeEdgeBetweenLayers = basegraph.PropertyWithDefault("org.eclipse.elk.layered.spacing.edgeEdgeBetweenLayers", func() float64 { return 10 })
	SpacingNodeNodeBetweenLayers = basegraph.PropertyWithDefault("org.eclipse.elk.layered.spacing.nodeNodeBetweenLayers", func() float64 { return 20 })
)

// Parse helpers keep the importer explicit and avoid relying on Go map or
// reflection behavior for option identity.
func ParseDirection(value string) (Direction, bool) {
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := ParseDirectionUpper(value); ok {
		return parsed, true
	}
	return ParseDirectionUpper(strings.ToUpper(value))
}

func ParseDirectionUpper(value string) (Direction, bool) {
	switch value {
	case "UNDEFINED":
		return DirectionUndefined, true
	case "RIGHT":
		return DirectionRight, true
	case "LEFT":
		return DirectionLeft, true
	case "DOWN":
		return DirectionDown, true
	case "UP":
		return DirectionUp, true
	default:
		return 0, false
	}
}

func ParsePortSide(value string) (PortSide, bool) {
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := ParsePortSideUpper(value); ok {
		return parsed, true
	}
	return ParsePortSideUpper(strings.ToUpper(value))
}

func ParsePortSideUpper(value string) (PortSide, bool) {
	switch value {
	case "UNDEFINED":
		return UndefinedSide, true
	case "NORTH":
		return NorthSide, true
	case "EAST":
		return EastSide, true
	case "SOUTH":
		return SouthSide, true
	case "WEST":
		return WestSide, true
	default:
		return 0, false
	}
}

func ParseCycleBreakingStrategy(value string) (CycleBreakingStrategy, bool) {
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := ParseCycleBreakingStrategyUpper(value); ok {
		return parsed, true
	}
	return ParseCycleBreakingStrategyUpper(strings.ToUpper(value))
}

func ParseCycleBreakingStrategyUpper(value string) (CycleBreakingStrategy, bool) {
	switch value {
	case "GREEDY":
		return CycleBreakingGreedy, true
	case "DEPTH_FIRST":
		return CycleBreakingDepthFirst, true
	case "INTERACTIVE":
		return CycleBreakingInteractive, true
	case "MODEL_ORDER":
		return CycleBreakingModelOrder, true
	case "GREEDY_MODEL_ORDER":
		return CycleBreakingGreedyModelOrder, true
	case "SCC_CONNECTIVITY":
		return CycleBreakingSCCConnectivity, true
	case "SCC_NODE_TYPE":
		return CycleBreakingSCCNodeType, true
	case "DFS_NODE_ORDER":
		return CycleBreakingDFSNodeOrder, true
	case "BFS_NODE_ORDER":
		return CycleBreakingBFSNodeOrder, true
	default:
		return 0, false
	}
}

func ParseLayeringStrategy(value string) (LayeringStrategy, bool) {
	// An upper-case spelling matches as it is (strings.ToUpper would
	// return it unchanged); anything else is upper-cased first.
	if parsed, ok := ParseLayeringStrategyUpper(value); ok {
		return parsed, true
	}
	return ParseLayeringStrategyUpper(strings.ToUpper(value))
}

func ParseLayeringStrategyUpper(value string) (LayeringStrategy, bool) {
	switch value {
	case "NETWORK_SIMPLEX":
		return LayeringNetworkSimplex, true
	case "LONGEST_PATH":
		return LayeringLongestPath, true
	case "LONGEST_PATH_SOURCE":
		return LayeringLongestPathSource, true
	case "COFFMAN_GRAHAM":
		return LayeringCoffmanGraham, true
	case "INTERACTIVE":
		return LayeringInteractive, true
	case "STRETCH_WIDTH":
		return LayeringStretchWidth, true
	case "MIN_WIDTH":
		return LayeringMinWidth, true
	case "BF_MODEL_ORDER":
		return LayeringBreadthFirstModelOrder, true
	case "DF_MODEL_ORDER":
		return LayeringDepthFirstModelOrder, true
	default:
		return 0, false
	}
}

func DirectionPortSide(direction Direction) PortSide {
	switch direction {
	case DirectionRight:
		return EastSide
	case DirectionLeft:
		return WestSide
	case DirectionDown:
		return SouthSide
	case DirectionUp:
		return NorthSide
	default:
		return UndefinedSide
	}
}

func enumError(kind, value string) error {
	return fmt.Errorf("layered: invalid %s value %q", kind, value)
}
