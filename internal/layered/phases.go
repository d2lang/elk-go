// Copyright (c) 2010, 2020 Kiel University and others.
// Copyright (c) 2017 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import "math/bits"

type Phase uint8

const (
	PhaseCycleBreaking Phase = iota
	PhaseLayering
	PhaseNodeOrdering
	PhaseNodePlacement
	PhaseEdgeRouting
)

// IntermediateProcessorStrategy declaration order follows ELK 0.12.0.
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
	AlternatingLayerUnzipper
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
	NodePlacementRepeater
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
// one after edge routing. Each slot is Java's EnumSet of strategies, kept as
// a bit set: iterating its bits in ascending order is the EnumSet order.
type ProcessorConfiguration struct {
	slots [6]uint64
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
		configuration.slots[slot] |= other.slots[slot]
	}
	return configuration
}

// Slot returns the slot's strategies in ascending (EnumSet) order.
func (configuration *ProcessorConfiguration) Slot(slot int) []IntermediateProcessorStrategy {
	var result []IntermediateProcessorStrategy
	for set := configuration.slots[slot]; set != 0; set &= set - 1 {
		result = append(result, IntermediateProcessorStrategy(bits.TrailingZeros64(set)))
	}
	return result
}

// SlotSet returns the slot as a bit set; bit i is strategy i.
func (configuration *ProcessorConfiguration) SlotSet(slot int) uint64 {
	return configuration.slots[slot]
}

func (configuration *ProcessorConfiguration) add(slot int, processor IntermediateProcessorStrategy) {
	if processor >= 64 {
		panic("layered: intermediate processor strategy does not fit the slot bit set")
	}
	configuration.slots[slot] |= 1 << processor
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
