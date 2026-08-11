// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"reflect"
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
)

func TestBasePipelineConfigurationAddsPartitionProcessors(t *testing.T) {
	arena := NewLGraphArena()
	graphID := arena.AddGraph()
	basegraph.Set(&arena.Graph(graphID).Properties, GraphProperties, GraphPropertyPartitions)

	configuration := BasePipelineConfiguration(arena, graphID).Intermediates
	assertHasIntermediate := func(slot int, strategy IntermediateProcessorStrategy) {
		t.Helper()
		for _, candidate := range configuration.Slot(slot) {
			if candidate == strategy {
				return
			}
		}
		t.Fatalf("slot %d = %v, missing %v", slot, configuration.Slot(slot), strategy)
	}
	assertHasIntermediate(int(PhaseCycleBreaking), PartitionPreprocessor)
	assertHasIntermediate(int(PhaseLayering), PartitionMidprocessor)
	assertHasIntermediate(int(PhaseNodeOrdering), PartitionPostprocessor)
}

func TestBasePipelineConfigurationSchedulesNodePlacementRepeater(t *testing.T) {
	arena := NewLGraphArena()
	graphID := arena.AddGraph()
	nodeID := arena.AddNode(graphID)
	arena.Graph(graphID).LayerlessNodes = []LNodeID{nodeID}

	configuration := BasePipelineConfiguration(arena, graphID)
	withoutRecompute := configuration.Intermediates.Slot(int(PhaseEdgeRouting))
	if containsProcessor(withoutRecompute, NodePlacementRepeater) {
		t.Fatalf("unconfigured pipeline unexpectedly schedules node-placement repeater: %v", withoutRecompute)
	}

	basegraph.Set(
		&arena.Graph(graphID).Properties,
		NetworkSimplexRecomputeNodePlacement,
		NodePlacementSimple,
	)
	configuration = BasePipelineConfiguration(arena, graphID)
	withRecompute := configuration.Intermediates.Slot(int(PhaseEdgeRouting))
	want := []IntermediateProcessorStrategy{
		NodePlacementRepeater,
		LayerSizeAndGraphHeightCalculator,
	}
	if !reflect.DeepEqual(withRecompute, want) {
		t.Fatalf("before edge routing = %v, want %v", withRecompute, want)
	}
}

func containsProcessor(values []IntermediateProcessorStrategy, target IntermediateProcessorStrategy) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
