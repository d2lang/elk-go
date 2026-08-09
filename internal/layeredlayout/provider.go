// Copyright (c) 2010, 2020 Kiel University and others.
// Copyright (c) 2010, 2015 Kiel University and others.
// Copyright (c) 2015, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

// Package layeredlayout connects ELK Layered's internal five-phase pipeline
// to the core graph engine. It is separate from package layered to keep the
// phase subpackages acyclic.
package layeredlayout

import (
	"fmt"

	"github.com/d2lang/elk-go/internal/core"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/layered/components"
	"github.com/d2lang/elk-go/internal/layered/intermediate"
	"github.com/d2lang/elk-go/internal/layered/intermediate/wrapping"
	"github.com/d2lang/elk-go/internal/layered/p1cycles"
	"github.com/d2lang/elk-go/internal/layered/p2layers"
	"github.com/d2lang/elk-go/internal/layered/p3order"
	"github.com/d2lang/elk-go/internal/layered/p4nodes"
	"github.com/d2lang/elk-go/internal/layered/p5edges"
)

type Provider struct{}

// Register adds the ELK Layered provider and its complete graph-feature
// declaration to the algorithm registry.
func Register(registry *core.AlgorithmRegistry) error {
	return registry.Register(core.AlgorithmData{
		ID: "org.eclipse.elk.layered",
		Features: core.FeatureSelfLoops | core.FeatureInsideSelfLoops |
			core.FeatureMultiEdges | core.FeatureEdgeLabels | core.FeaturePorts |
			core.FeatureCompound | core.FeatureClusters | core.FeatureDisconnected,
		Create: func() core.LayoutProvider { return &Provider{} },
	})
}

func (*Provider) Layout(model *basegraph.Model, parent basegraph.NodeID, monitor core.ProgressMonitor) error {
	importer := layered.NewElkGraphImporter(model)
	arena, topGraph, err := importer.ImportGraph(parent)
	if err != nil {
		return err
	}
	if basegraph.Get(&arena.Graph(topGraph).Properties, layered.Hierarchy) == layered.HierarchyIncludeChildren {
		preprocessor := &layered.CompoundGraphPreprocessor{}
		if err := preprocessor.Process(arena, topGraph); err != nil {
			return err
		}
		if err := runHierarchy(arena, topGraph, monitor); err != nil {
			return err
		}
		postprocessor := &layered.CompoundGraphPostprocessor{}
		if err := postprocessor.Process(arena, topGraph); err != nil {
			return err
		}
	} else if err := runComponents(arena, topGraph, monitor); err != nil {
		return err
	}
	return layered.NewElkGraphLayoutTransferrer(model, importer.Maps).ApplyLayout(arena, topGraph)
}

// runComponents mirrors ElkLayered.doLayout: GraphConfigurator is run once
// before splitting, all components share its Random instance, and completed
// component layouts are made layerless before packing.
func runComponents(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	monitor core.ProgressMonitor,
) error {
	layered.ConfigureGraphProperties(arena, graphID)
	sharedRandom := basegraph.Get(&arena.Graph(graphID).Properties, layered.Random)
	processor := components.NewComponentsProcessor()
	componentIDs, err := processor.Split(arena, graphID)
	if err != nil {
		return err
	}
	for _, componentID := range componentIDs {
		run, err := prepareGraphRun(arena, componentID)
		if err != nil {
			return err
		}
		// prepareGraphRun invokes the configurator because hierarchy layout
		// needs that behavior. For components, restore the single Random object
		// created for the unsplit graph, as Java's copied properties do.
		basegraph.Set(&arena.Graph(componentID).Properties, layered.Random, sharedRandom)
		for slot := 0; slot < 6; slot++ {
			if err := run.runSlot(arena, slot, true, monitor); err != nil {
				return err
			}
		}
		components.MakeLayerless(arena, componentID)
	}
	return processor.Combine(arena, componentIDs, graphID)
}

func collectGraphsBottomUp(arena *layered.LGraphArena, root layered.LGraphID) []layered.LGraphID {
	queue := []layered.LGraphID{root}
	graphs := make([]layered.LGraphID, 0, len(arena.Graphs))
	for len(queue) > 0 {
		graphID := queue[0]
		queue = queue[1:]
		graphs = append(graphs, graphID)
		for _, nodeID := range arena.Graph(graphID).LayerlessNodes {
			if nested := arena.Node(nodeID).NestedGraph; nested != nil {
				queue = append(queue, *nested)
			}
		}
	}
	for left, right := 0, len(graphs)-1; left < right; left, right = left+1, right-1 {
		graphs[left], graphs[right] = graphs[right], graphs[left]
	}
	return graphs
}

func runGraph(arena *layered.LGraphArena, graphID layered.LGraphID, monitor core.ProgressMonitor) error {
	run, err := prepareGraphRun(arena, graphID)
	if err != nil {
		return err
	}
	for slot := 0; slot < 6; slot++ {
		if err := run.runSlot(arena, slot, true, monitor); err != nil {
			return err
		}
	}
	return nil
}

type graphRun struct {
	graphID       layered.LGraphID
	configuration layered.PipelineConfiguration
	phases        []layered.LayoutProcessor
}

func prepareGraphRun(arena *layered.LGraphArena, graphID layered.LGraphID) (*graphRun, error) {
	configuration := layered.BasePipelineConfiguration(arena, graphID)
	// GraphConfigurator makes this decision before phase 2 consumes
	// LayerlessNodes; the strict activation threshold is observable.
	if strategy, active := p3order.GreedySwitchIntermediateForGraph(arena, graphID); active {
		configuration.Intermediates.AddBefore(layered.PhaseNodePlacement, strategy)
	}
	phases, err := selectPhases(arena, graphID, configuration.Phases)
	if err != nil {
		return nil, err
	}
	for _, phase := range phases {
		if configured, ok := phase.(layered.ConfiguredProcessor); ok {
			configuration.Intermediates.AddAll(configured.ProcessorConfiguration(arena, graphID))
		}
	}

	return &graphRun{graphID: graphID, configuration: configuration, phases: phases}, nil
}

func (run *graphRun) runSlot(
	arena *layered.LGraphArena,
	slot int,
	runPhase bool,
	monitor core.ProgressMonitor,
) error {
	for _, strategy := range run.configuration.Intermediates.Slot(slot) {
		if err := runIntermediate(arena, run.graphID, strategy); err != nil {
			return err
		}
	}
	if runPhase && slot < len(run.phases) {
		if err := run.phases[slot].Process(arena, run.graphID); err != nil {
			return err
		}
		monitor.Worked(1)
	}
	return nil
}

// runHierarchy mirrors ElkLayered.hierarchicalLayout around phase 3, the
// hierarchy-aware layer-sweep processor. Every graph is prepared bottom-up,
// paused at the shared sweep, and resumed bottom-up afterwards.
func runHierarchy(arena *layered.LGraphArena, root layered.LGraphID, monitor core.ProgressMonitor) error {
	graphIDs := collectGraphsBottomUp(arena, root)
	runs := make([]*graphRun, 0, len(graphIDs))
	var rootRun *graphRun
	for _, graphID := range graphIDs {
		run, err := prepareGraphRun(arena, graphID)
		if err != nil {
			return err
		}
		runs = append(runs, run)
		if graphID == root {
			rootRun = run
		}
	}
	if rootRun == nil {
		return fmt.Errorf("layered: root graph %d is missing from its hierarchy", root)
	}

	// Layer sweep is the only hierarchy-aware phase in ELK 0.8.1. If the
	// root configuration selects a different crossing minimizer, the Java
	// hierarchy runner simply executes each graph's whole pipeline bottom-up.
	// In particular, it does not reject INTERACTIVE or NONE merely because
	// hierarchy handling is INCLUDE_CHILDREN.
	if _, hierarchyAware := rootRun.phases[layered.PhaseNodeOrdering].(*p3order.LayerSweepCrossingMinimizer); !hierarchyAware {
		for _, run := range runs {
			for slot := 0; slot < 6; slot++ {
				if err := run.runSlot(arena, slot, true, monitor); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for _, run := range runs {
		for slot := 0; slot < int(layered.PhaseNodeOrdering); slot++ {
			if err := run.runSlot(arena, slot, true, monitor); err != nil {
				return err
			}
		}
		if err := run.runSlot(arena, int(layered.PhaseNodeOrdering), false, monitor); err != nil {
			return err
		}
	}
	minimizer, ok := rootRun.phases[layered.PhaseNodeOrdering].(*p3order.LayerSweepCrossingMinimizer)
	if !ok {
		return fmt.Errorf("layered: hierarchy-aware crossing minimizer is not configured on root graph")
	}
	if err := minimizer.ProcessHierarchy(arena, root); err != nil {
		return err
	}
	monitor.Worked(1)

	for _, run := range runs {
		for slot := int(layered.PhaseNodePlacement); slot < 6; slot++ {
			if err := run.runSlot(arena, slot, true, monitor); err != nil {
				return err
			}
		}
	}
	return nil
}

func selectPhases(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	strategies layered.PhaseStrategies,
) ([]layered.LayoutProcessor, error) {
	var cycle layered.LayoutProcessor
	switch strategies.CycleBreaking {
	case layered.CycleBreakingGreedy:
		cycle = p1cycles.NewGreedyCycleBreaker()
	case layered.CycleBreakingDepthFirst:
		cycle = &p1cycles.DepthFirstCycleBreaker{}
	case layered.CycleBreakingInteractive:
		cycle = &p1cycles.InteractiveCycleBreaker{}
	case layered.CycleBreakingModelOrder:
		cycle = &p1cycles.ModelOrderCycleBreaker{}
	case layered.CycleBreakingGreedyModelOrder:
		cycle = p1cycles.NewGreedyModelOrderCycleBreaker()
	default:
		return nil, fmt.Errorf("layered: cycle-breaking strategy %v is not implemented", strategies.CycleBreaking)
	}
	var layerer layered.LayoutProcessor
	switch strategies.Layering {
	case layered.LayeringNetworkSimplex:
		layerer = &p2layers.NetworkSimplexLayerer{}
	case layered.LayeringLongestPath:
		layerer = &p2layers.LongestPathLayerer{}
	case layered.LayeringCoffmanGraham:
		layerer = &p2layers.CoffmanGrahamLayerer{}
	case layered.LayeringInteractive:
		layerer = &p2layers.InteractiveLayerer{}
	case layered.LayeringStretchWidth:
		layerer = &p2layers.StretchWidthLayerer{}
	case layered.LayeringMinWidth:
		layerer = &p2layers.MinWidthLayerer{}
	default:
		return nil, fmt.Errorf("layered: layering strategy %v is not implemented", strategies.Layering)
	}
	var crossing layered.LayoutProcessor
	switch strategies.CrossingMinimization {
	case layered.CrossingMinimizationLayerSweep:
		crossing = &p3order.LayerSweepCrossingMinimizer{}
	case layered.CrossingMinimizationInteractive:
		crossing = &p3order.InteractiveCrossingMinimizer{}
	case layered.CrossingMinimizationNone:
		crossing = &p3order.NoCrossingMinimizer{}
	default:
		return nil, fmt.Errorf("layered: crossing-minimization strategy %v is not implemented", strategies.CrossingMinimization)
	}
	var placer layered.LayoutProcessor
	switch strategies.NodePlacement {
	case layered.NodePlacementSimple:
		placer = &p4nodes.SimpleNodePlacer{}
	case layered.NodePlacementInteractive:
		placer = &p4nodes.InteractiveNodePlacer{}
	case layered.NodePlacementLinearSegments:
		placer = &p4nodes.LinearSegmentsNodePlacer{}
	case layered.NodePlacementBrandesKoepf:
		placer = p4nodes.BrandesKoepfPlacer{}
	case layered.NodePlacementNetworkSimplex:
		placer = &p4nodes.NetworkSimplexPlacer{}
	default:
		return nil, fmt.Errorf("layered: node-placement strategy %v is not implemented", strategies.NodePlacement)
	}
	var router layered.LayoutProcessor
	switch strategies.EdgeRouting {
	case layered.EdgeRoutingUndefined, layered.EdgeRoutingOrthogonal:
		// EdgeRouterFactory's default branch is OrthogonalEdgeRouter. Keep the
		// graph property itself undefined because other ELK processors observe
		// that distinction.
		router = &p5edges.OrthogonalEdgeRouter{}
	case layered.EdgeRoutingPolyline:
		router = &p5edges.PolylineEdgeRouter{}
	case layered.EdgeRoutingSplines:
		router = &p5edges.SplineEdgeRouter{}
	default:
		return nil, fmt.Errorf("layered: edge-routing strategy %v is not implemented", strategies.EdgeRouting)
	}
	return []layered.LayoutProcessor{
		cycle,
		layerer,
		crossing,
		placer,
		router,
	}, nil
}

func runIntermediate(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	strategy layered.IntermediateProcessorStrategy,
) error {
	switch strategy {
	case layered.DirectionPreprocessor:
		// ELK 0.8.1's IntermediateProcessorStrategy factory intentionally
		// associates the preprocessor with TO_INPUT_DIRECTION.
		intermediate.TransformGraph(arena, graphID, intermediate.ToInputDirection)
	case layered.DirectionPostprocessor:
		intermediate.TransformGraph(arena, graphID, intermediate.ToInternalLTR)
	case layered.EdgeAndLayerConstraintEdgeReverser:
		intermediate.ReverseConstraintEdges(arena, graphID)
	case layered.InLayerConstraintProcessor:
		intermediate.ProcessInLayerConstraints(arena, graphID)
	case layered.CommentPreprocessor:
		return intermediate.PreprocessComments(arena, graphID)
	case layered.InteractiveExternalPortPositioner:
		return intermediate.PositionInteractiveExternalPorts(arena, graphID)
	case layered.LabelDummyInserter:
		intermediate.InsertLabelDummies(arena, graphID)
	case layered.LayerConstraintPreprocessor:
		intermediate.ProcessLayerConstraintsBeforeLayering(arena, graphID)
	case layered.LayerConstraintPostprocessor:
		intermediate.ProcessLayerConstraintsAfterLayering(arena, graphID)
	case layered.LabelDummySwitcher:
		intermediate.SwitchLabelDummies(arena, graphID)
	case layered.LabelSideSelector:
		intermediate.SelectLabelSides(arena, graphID)
	case layered.LabelDummyRemover:
		intermediate.RemoveLabelDummies(arena, graphID)
	case layered.EndNodePortLabelManagementProcessor:
		intermediate.ManageEndNodePortLabels(arena, graphID)
	case layered.EndLabelPreprocessor:
		intermediate.PreprocessEndLabels(arena, graphID)
	case layered.EndLabelSorter:
		intermediate.SortEndLabels(arena, graphID)
	case layered.EndLabelPostprocessor:
		intermediate.PostprocessEndLabels(arena, graphID)
	case layered.LongEdgeSplitter:
		intermediate.SplitLongEdges(arena, graphID)
	case layered.LongEdgeJoiner:
		intermediate.JoinLongEdges(arena, graphID)
	case layered.PortSideProcessor:
		intermediate.ProcessPortSides(arena, graphID)
	case layered.PortListSorter:
		return intermediate.SortPortLists(arena, graphID)
	case layered.OneSidedGreedySwitch, layered.TwoSidedGreedySwitch:
		return p3order.ProcessGreedySwitchIntermediate(arena, graphID, strategy)
	case layered.HyperedgeDummyMerger:
		return intermediate.MergeHyperedgeDummies(arena, graphID)
	case layered.LabelAndNodeSizeProcessor:
		intermediate.ProcessLabelAndNodeSizes(arena, graphID)
	case layered.InnermostNodeMarginCalculator:
		intermediate.CalculateInnermostNodeMargins(arena, graphID)
	case layered.CommentNodeMarginCalculator:
		intermediate.CalculateCommentNodeMargins(arena, graphID)
	case layered.SortByInputOrderOfModel:
		return intermediate.SortByInputModel(arena, graphID)
	case layered.InvertedPortProcessor:
		return intermediate.ProcessInvertedPorts(arena, graphID)
	case layered.NorthSouthPortPreprocessor:
		intermediate.PreprocessNorthSouthPorts(arena, graphID)
	case layered.NorthSouthPortPostprocessor:
		intermediate.PostprocessNorthSouthPorts(arena, graphID)
	case layered.PartitionPreprocessor:
		intermediate.PreprocessPartitions(arena, graphID)
	case layered.PartitionMidprocessor:
		intermediate.ProcessPartitionsBeforeLayering(arena, graphID)
	case layered.HighDegreeNodeLayerProcessor:
		intermediate.ProcessHighDegreeNodes(arena, graphID)
	case layered.NodePromotion:
		intermediate.PromoteNodes(arena, graphID)
	case layered.PartitionPostprocessor:
		intermediate.PostprocessPartitions(arena, graphID)
	case layered.SemiInteractiveCrossMinProcessor:
		intermediate.ProcessSemiInteractiveCrossing(arena, graphID)
	case layered.BreakingPointInserter:
		wrapping.InsertBreakingPoints(arena, graphID)
	case layered.HierarchicalPortConstraintProcessor:
		return intermediate.ProcessHierarchicalPortConstraints(arena, graphID)
	case layered.HierarchicalPortDummySizeProcessor:
		return intermediate.SizeHierarchicalPortDummies(arena, graphID)
	case layered.HierarchicalPortPositionProcessor:
		return intermediate.PositionHierarchicalPorts(arena, graphID)
	case layered.HierarchicalPortOrthogonalEdgeRouter:
		return intermediate.RouteHierarchicalPorts(arena, graphID)
	case layered.HierarchicalNodeResizer:
		return intermediate.ResizeHierarchicalNode(arena, graphID)
	case layered.LayerSizeAndGraphHeightCalculator:
		intermediate.CalculateLayerSizesAndGraphHeight(arena, graphID)
	case layered.ReversedEdgeRestorer:
		intermediate.RestoreReversedEdges(arena, graphID)
	case layered.SelfLoopPreprocessor:
		return intermediate.PreprocessSelfLoops(arena, graphID)
	case layered.SelfLoopPortRestorer:
		return intermediate.RestoreSelfLoopPorts(arena, graphID)
	case layered.BreakingPointProcessor:
		wrapping.ProcessBreakingPoints(arena, graphID)
	case layered.SingleEdgeGraphWrapper:
		wrapping.WrapSingleEdgeGraph(arena, graphID)
	case layered.SelfLoopRouter:
		return intermediate.RouteSelfLoops(arena, graphID)
	case layered.SelfLoopPostprocessor:
		intermediate.PostprocessSelfLoops(arena, graphID)
	case layered.ConstraintsPostprocessor:
		intermediate.ProcessConstraints(arena, graphID)
	case layered.CommentPostprocessor:
		return intermediate.PostprocessComments(arena, graphID)
	case layered.HypernodeProcessor:
		intermediate.ProcessHypernodes(arena, graphID)
	case layered.BreakingPointRemover:
		wrapping.RemoveBreakingPoints(arena, graphID)
	case layered.FinalSplineBendpointsCalculator:
		p5edges.FinalizeSplineBendpoints(arena, graphID)
	case layered.HorizontalCompactor:
		return intermediate.CompactHorizontally(arena, graphID)
	default:
		return fmt.Errorf("layered: intermediate processor %d is required but not implemented", strategy)
	}
	return nil
}
