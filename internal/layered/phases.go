// Copyright (c) 2010, 2020 Kiel University and others.
// Copyright (c) 2017 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import "sort"

type Phase uint8

const (
	PhaseCycleBreaking Phase = iota
	PhaseLayering
	PhaseNodeOrdering
	PhaseNodePlacement
	PhaseEdgeRouting
)

// IntermediateProcessorStrategy declaration order is the exact ELK 0.8.1
// enum order. LayoutProcessorConfiguration uses an EnumSet, so this order is
// observable when processors from multiple phase configurations share a slot.
type IntermediateProcessorStrategy uint8

const (
	DirectionPreprocessor IntermediateProcessorStrategy = iota
	CommentPreprocessor
	EdgeAndLayerConstraintEdgeReverser
	InteractiveExternalPortPositioner
	PartitionPreprocessor
	LabelDummyInserter
	SelfLoopPreprocessor
	LayerConstraintPreprocessor
	PartitionMidprocessor
	HighDegreeNodeLayerProcessor
	NodePromotion
	LayerConstraintPostprocessor
	PartitionPostprocessor
	HierarchicalPortConstraintProcessor
	SemiInteractiveCrossMinProcessor
	BreakingPointInserter
	LongEdgeSplitter
	PortSideProcessor
	InvertedPortProcessor
	PortListSorter
	SortByInputOrderOfModel
	NorthSouthPortPreprocessor
	BreakingPointProcessor
	OneSidedGreedySwitch
	TwoSidedGreedySwitch
	SelfLoopPortRestorer
	SingleEdgeGraphWrapper
	InLayerConstraintProcessor
	EndNodePortLabelManagementProcessor
	LabelAndNodeSizeProcessor
	InnermostNodeMarginCalculator
	SelfLoopRouter
	CommentNodeMarginCalculator
	EndLabelPreprocessor
	LabelDummySwitcher
	CenterLabelManagementProcessor
	LabelSideSelector
	HyperedgeDummyMerger
	HierarchicalPortDummySizeProcessor
	LayerSizeAndGraphHeightCalculator
	HierarchicalPortPositionProcessor
	ConstraintsPostprocessor
	CommentPostprocessor
	HypernodeProcessor
	HierarchicalPortOrthogonalEdgeRouter
	LongEdgeJoiner
	SelfLoopPostprocessor
	BreakingPointRemover
	NorthSouthPortPostprocessor
	HorizontalCompactor
	LabelDummyRemover
	FinalSplineBendpointsCalculator
	EndLabelSorter
	ReversedEdgeRestorer
	EndLabelPostprocessor
	HierarchicalNodeResizer
	DirectionPostprocessor
)

// LayoutProcessor is implemented by phases and intermediate processors.
type LayoutProcessor interface {
	Process(arena *LGraphArena, graphID LGraphID) error
}

// ConfiguredProcessor contributes its own intermediate dependencies.
type ConfiguredProcessor interface {
	LayoutProcessor
	ProcessorConfiguration(arena *LGraphArena, graphID LGraphID) ProcessorConfiguration
}

// ProcessorConfiguration has six slots: before each of the five phases and
// one after edge routing.
type ProcessorConfiguration struct {
	slots [6][]IntermediateProcessorStrategy
}

func (configuration *ProcessorConfiguration) AddBefore(phase Phase, processor IntermediateProcessorStrategy) *ProcessorConfiguration {
	configuration.add(int(phase), processor)
	return configuration
}

func (configuration *ProcessorConfiguration) AddAfter(phase Phase, processor IntermediateProcessorStrategy) *ProcessorConfiguration {
	configuration.add(int(phase)+1, processor)
	return configuration
}

func (configuration *ProcessorConfiguration) AddAll(other ProcessorConfiguration) *ProcessorConfiguration {
	for slot := range other.slots {
		for _, processor := range other.slots[slot] {
			configuration.add(slot, processor)
		}
	}
	return configuration
}

func (configuration *ProcessorConfiguration) Slot(slot int) []IntermediateProcessorStrategy {
	return append([]IntermediateProcessorStrategy(nil), configuration.slots[slot]...)
}

func (configuration *ProcessorConfiguration) add(slot int, processor IntermediateProcessorStrategy) {
	for _, existing := range configuration.slots[slot] {
		if existing == processor {
			return
		}
	}
	configuration.slots[slot] = append(configuration.slots[slot], processor)
	sort.Slice(configuration.slots[slot], func(first, second int) bool {
		return configuration.slots[slot][first] < configuration.slots[slot][second]
	})
}

type PhaseStrategies struct {
	CycleBreaking        CycleBreakingStrategy
	Layering             LayeringStrategy
	CrossingMinimization CrossingMinimizationStrategy
	NodePlacement        NodePlacementStrategy
	EdgeRouting          EdgeRouting
}

type PipelineConfiguration struct {
	Phases        PhaseStrategies
	Intermediates ProcessorConfiguration
}
