// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"reflect"
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
)

func TestAdvancedLayeringProcessorsAreScheduledInELKOrder(t *testing.T) {
	arena := NewLGraphArena()
	graphID := arena.AddGraph()
	graph := arena.Graph(graphID)
	basegraph.Set(&graph.Properties, HighDegreeNodeTreatment, true)
	basegraph.Set(&graph.Properties, NodePromotionStrategyOption, NodePromotionNikolov)
	basegraph.Set(&graph.Properties, PostCompactionStrategy, GraphCompactionLeft)
	basegraph.Set(&graph.Properties, EdgeRoutingOption, EdgeRoutingOrthogonal)

	configuration := BasePipelineConfiguration(arena, graphID).Intermediates
	if got, want := configuration.Slot(int(PhaseNodeOrdering)),
		[]IntermediateProcessorStrategy{HighDegreeNodeLayerProcessor, NodePromotion, PortSideProcessor}; !reflect.DeepEqual(got, want) {
		t.Fatalf("before node ordering = %v, want %v", got, want)
	}
	postRouting := configuration.Slot(int(PhaseEdgeRouting) + 1)
	if !containsIntermediate(postRouting, HorizontalCompactor) {
		t.Fatalf("after edge routing = %v, missing horizontal compactor", postRouting)
	}
}

func TestPostCompactionIsNotScheduledForPolylineRouting(t *testing.T) {
	arena := NewLGraphArena()
	graphID := arena.AddGraph()
	graph := arena.Graph(graphID)
	basegraph.Set(&graph.Properties, PostCompactionStrategy, GraphCompactionRight)
	basegraph.Set(&graph.Properties, EdgeRoutingOption, EdgeRoutingPolyline)

	configuration := BasePipelineConfiguration(arena, graphID)
	postRouting := configuration.Intermediates.Slot(int(PhaseEdgeRouting) + 1)
	if containsIntermediate(postRouting, HorizontalCompactor) {
		t.Fatalf("polyline pipeline unexpectedly schedules horizontal compactor: %v", postRouting)
	}
}

func containsIntermediate(values []IntermediateProcessorStrategy, target IntermediateProcessorStrategy) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
