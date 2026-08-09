// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import (
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
