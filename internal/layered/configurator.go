// Copyright (c) 2015, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"time"

	"github.com/d2lang/elk-go/internal/core"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
)

const minimumEdgeSpacing = 2.0

// ConfigureGraphProperties applies GraphConfigurator's non-pipeline setup.
func ConfigureGraphProperties(arena *LGraphArena, graphID LGraphID) {
	graph := arena.Graph(graphID)
	if spacing := basegraph.Get(&graph.Properties, SpacingEdgeEdge); spacing < minimumEdgeSpacing {
		basegraph.Set(&graph.Properties, SpacingEdgeEdge, minimumEdgeSpacing)
	}
	if basegraph.Get(&graph.Properties, DirectionOption) == DirectionUndefined {
		// LGraphUtil.getDirection derives this from interactive coordinates.
		// With no usable coordinates it returns RIGHT, which is the common and
		// D2-active path.
		basegraph.Set(&graph.Properties, DirectionOption, DirectionRight)
	}
	seed := basegraph.Get(&graph.Properties, RandomSeed)
	if seed == 0 {
		basegraph.Set(&graph.Properties, Random, javacompat.NewRandom(time.Now().UnixNano()))
	} else {
		basegraph.Set(&graph.Properties, Random, javacompat.NewRandom(int64(seed)))
	}
	if _, present := basegraph.TryGet(&graph.Properties, FavorStraightEdges); !present {
		basegraph.Set(&graph.Properties, FavorStraightEdges,
			basegraph.Get(&graph.Properties, EdgeRoutingOption) == EdgeRoutingOrthogonal)
	}
	copyOriginalPortConstraints(arena, graphID)
}

func copyOriginalPortConstraints(arena *LGraphArena, graphID LGraphID) {
	// GraphAllNodes' order (layerless nodes, then the layers' nodes), walked
	// in place: setting properties and recursing into nested graphs change
	// none of this graph's lists. One function, so that a level of nesting
	// costs one frame.
	graph := arena.Graph(graphID)
	for list := -1; list < len(graph.Layers); list++ {
		nodes := graph.LayerlessNodes
		if list >= 0 {
			nodes = arena.Layer(graph.Layers[list]).Nodes
		}
		for _, nodeID := range nodes {
			copyOriginalPortConstraint(arena, nodeID)
			if arena.Node(nodeID).NestedGraph != nil {
				copyOriginalPortConstraints(arena, *arena.Node(nodeID).NestedGraph)
			}
		}
	}
}

// copyOriginalPortConstraint records nodeID's port constraints as its
// original ones.
func copyOriginalPortConstraint(arena *LGraphArena, nodeID LNodeID) {
	constraints := basegraph.Get(&arena.Node(nodeID).Properties, PortConstraintsOption)
	basegraph.Set(&arena.Node(nodeID).Properties, OriginalPortConstraints, constraints)
}

func BasePipelineConfiguration(arena *LGraphArena, graphID LGraphID) PipelineConfiguration {
	ConfigureGraphProperties(arena, graphID)
	graph := arena.Graph(graphID)
	configuration := PipelineConfiguration{Phases: PhaseStrategies{
		CycleBreaking:        basegraph.Get(&graph.Properties, CycleBreaking),
		Layering:             basegraph.Get(&graph.Properties, Layering),
		CrossingMinimization: basegraph.Get(&graph.Properties, CrossingMinimization),
		NodePlacement:        basegraph.Get(&graph.Properties, NodePlacement),
		EdgeRouting:          basegraph.Get(&graph.Properties, EdgeRoutingOption),
	}}

	configuration.Intermediates.
		AddBefore(PhaseNodePlacement, InnermostNodeMarginCalculator).
		AddBefore(PhaseNodePlacement, LabelAndNodeSizeProcessor).
		AddBefore(PhaseEdgeRouting, LayerSizeAndGraphHeightCalculator).
		AddAfter(PhaseEdgeRouting, EndLabelSorter)

	if basegraph.Get(&graph.Properties, Hierarchy) == HierarchyIncludeChildren {
		configuration.Intermediates.AddAfter(PhaseEdgeRouting, HierarchicalNodeResizer)
	}
	if basegraph.Get(&graph.Properties, FeedbackEdges) {
		configuration.Intermediates.AddBefore(PhaseCycleBreaking, PortSideProcessor)
	} else {
		configuration.Intermediates.AddBefore(PhaseNodeOrdering, PortSideProcessor)
	}
	switch basegraph.Get(&graph.Properties, DirectionOption) {
	case DirectionLeft, DirectionDown, DirectionUp:
		configuration.Intermediates.
			AddBefore(PhaseCycleBreaking, DirectionPreprocessor).
			AddAfter(PhaseEdgeRouting, DirectionPostprocessor)
	}
	if basegraph.Get(&graph.Properties, ConsiderModelOrder) != OrderingNone {
		configuration.Intermediates.AddBefore(PhaseNodeOrdering, SortByInputOrderOfModel)
	}
	if basegraph.Get(&graph.Properties, NodePromotionStrategyOption) != NodePromotionNone {
		configuration.Intermediates.AddBefore(PhaseNodeOrdering, NodePromotion)
	}
	if basegraph.Get(&graph.Properties, PostCompactionStrategy) != GraphCompactionNone &&
		basegraph.Get(&graph.Properties, EdgeRoutingOption) != EdgeRoutingPolyline {
		configuration.Intermediates.AddAfter(PhaseEdgeRouting, HorizontalCompactor)
	}
	if basegraph.Get(&graph.Properties, HighDegreeNodeTreatment) {
		configuration.Intermediates.AddBefore(PhaseNodeOrdering, HighDegreeNodeLayerProcessor)
	}
	if basegraph.Get(&graph.Properties, SemiInteractive) {
		configuration.Intermediates.AddBefore(PhaseNodeOrdering, SemiInteractiveCrossMinProcessor)
	}
	if basegraph.Get(&graph.Properties, LayerUnzippingStrategyOption) == LayerUnzippingAlternating {
		configuration.Intermediates.AddBefore(PhaseNodePlacement, AlternatingLayerUnzipper)
	}
	// GraphConfigurator evaluates this while the nodes are still layerless.
	// NodeFlexibility.NONE is an enum value rather than Java null, so every
	// node satisfies the source's non-null predicate when the recomputation
	// option is explicitly present.
	if len(graph.LayerlessNodes) != 0 &&
		basegraph.Has(&graph.Properties, NetworkSimplexRecomputeNodePlacement) {
		configuration.Intermediates.AddAfter(PhaseNodePlacement, NodePlacementRepeater)
	}
	if basegraph.Get(&graph.Properties, GraphProperties)&GraphPropertyPartitions != 0 {
		configuration.Intermediates.
			AddBefore(PhaseCycleBreaking, PartitionPreprocessor).
			AddBefore(PhaseLayering, PartitionMidprocessor).
			AddBefore(PhaseNodeOrdering, PartitionPostprocessor)
	}
	if basegraph.Get(&graph.Properties, GraphProperties)&GraphPropertyComments != 0 {
		configuration.Intermediates.
			AddBefore(PhaseCycleBreaking, CommentPreprocessor).
			AddBefore(PhaseNodePlacement, CommentNodeMarginCalculator).
			AddAfter(PhaseEdgeRouting, CommentPostprocessor)
	}
	if basegraph.Get(&graph.Properties, core.InteractiveLayout) ||
		basegraph.Get(&graph.Properties, GeneratePositionAndLayerIDs) {
		configuration.Intermediates.AddAfter(PhaseEdgeRouting, ConstraintsPostprocessor)
	}
	switch basegraph.Get(&graph.Properties, WrappingStrategyOption) {
	case WrappingSingleEdge:
		configuration.Intermediates.AddBefore(PhaseNodePlacement, SingleEdgeGraphWrapper)
	case WrappingMultiEdge:
		configuration.Intermediates.
			AddBefore(PhaseNodeOrdering, BreakingPointInserter).
			AddBefore(PhaseNodePlacement, BreakingPointProcessor).
			AddAfter(PhaseEdgeRouting, BreakingPointRemover)
	}
	return configuration
}

func GreedyCycleConfiguration() ProcessorConfiguration {
	var configuration ProcessorConfiguration
	configuration.AddAfter(PhaseEdgeRouting, ReversedEdgeRestorer)
	return configuration
}

func NetworkSimplexLayeringConfiguration() ProcessorConfiguration {
	var configuration ProcessorConfiguration
	configuration.
		AddBefore(PhaseCycleBreaking, EdgeAndLayerConstraintEdgeReverser).
		AddBefore(PhaseLayering, LayerConstraintPreprocessor).
		AddBefore(PhaseNodeOrdering, LayerConstraintPostprocessor)
	return configuration
}
