// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"reflect"
	"testing"

	"github.com/d2lang/elk-go/internal/core"
	basegraph "github.com/d2lang/elk-go/internal/graph"
)

func TestGreedySwitchOptionsNormalizeAndRegister(t *testing.T) {
	var source, target basegraph.PropertyMap
	source.SetRaw(GreedySwitchActivationThreshold.ID, "17")
	source.SetRaw(GreedySwitchHierarchical.ID, "ONE_SIDED")
	copyTypedProperties(&target, &source)
	if got := basegraph.Get(&target, GreedySwitchActivationThreshold); got != 17 {
		t.Fatalf("activation threshold = %d, want 17", got)
	}
	if got := basegraph.Get(&target, GreedySwitchHierarchical); got != GreedySwitchOneSided {
		t.Fatalf("hierarchical switch = %v, want ONE_SIDED", got)
	}

	registry := core.NewOptionRegistry()
	RegisterOptions(registry)
	if option := registry.ByID(GreedySwitchActivationThreshold.ID); option == nil || option.Kind != core.IntegerOption {
		t.Fatalf("activation-threshold metadata = %#v", option)
	}
	if option := registry.ByID(GreedySwitchHierarchical.ID); option == nil || option.Kind != core.EnumOption {
		t.Fatalf("hierarchical-switch metadata = %#v", option)
	}
}

func TestELK012EnumSurfaces(t *testing.T) {
	registry := core.NewOptionRegistry()
	RegisterOptions(registry)

	tests := []struct {
		id   string
		want []string
	}{
		{EdgeLabelsPlacement.ID, []string{"CENTER", "HEAD", "TAIL"}},
		{ConsiderModelOrder.ID, []string{"NONE", "NODES_AND_EDGES", "PREFER_EDGES", "PREFER_NODES"}},
		{CycleBreaking.ID, []string{"GREEDY", "DEPTH_FIRST", "INTERACTIVE", "MODEL_ORDER", "GREEDY_MODEL_ORDER", "SCC_CONNECTIVITY", "SCC_NODE_TYPE", "DFS_NODE_ORDER", "BFS_NODE_ORDER"}},
		{Layering.ID, []string{"NETWORK_SIMPLEX", "LONGEST_PATH", "LONGEST_PATH_SOURCE", "COFFMAN_GRAHAM", "INTERACTIVE", "STRETCH_WIDTH", "MIN_WIDTH", "BF_MODEL_ORDER", "DF_MODEL_ORDER"}},
		{CrossingMinimization.ID, []string{"LAYER_SWEEP", "MEDIAN_LAYER_SWEEP", "INTERACTIVE", "NONE"}},
		{ComponentOrdering.ID, []string{"NONE", "INSIDE_PORT_SIDE_GROUPS", "GROUP_MODEL_ORDER", "MODEL_ORDER"}},
		{SelfLoopOrdering.ID, []string{"STACKED", "REVERSE_STACKED", "SEQUENCED"}},
		{SplineRoutingModeOption.ID, []string{"SLOPPY", "CONSERVATIVE", "CONSERVATIVE_SOFT"}},
	}
	for _, test := range tests {
		option := registry.ByID(test.id)
		if option == nil {
			t.Fatalf("option %q is not registered", test.id)
		}
		if !reflect.DeepEqual(option.EnumValues, test.want) {
			t.Errorf("option %q enum values = %v, want %v", test.id, option.EnumValues, test.want)
		}
	}

	if _, ok := registry.ByID(EdgeLabelsPlacement.ID).ParseValue("UNDEFINED"); ok {
		t.Error("edge-label placement accepted non-ELK-0.8.1 value UNDEFINED")
	}
	if value, ok := registry.ByID(ConsiderModelOrder.ID).ParseValue("PREFER_NODES"); !ok || value != "PREFER_NODES" {
		t.Errorf("model-order strategy PREFER_NODES = %v, accepted=%v", value, ok)
	}
}

func TestELK012NewOptionsRegister(t *testing.T) {
	registry := core.NewOptionRegistry()
	RegisterOptions(registry)
	for _, id := range []string{
		InLayerPredecessorOf.ID,
		InLayerSuccessorOf.ID,
		NetworkSimplexRecomputeNodePlacement.ID,
		LayerUnzippingStrategyOption.ID,
		PortModelOrder.ID,
		CycleBreakingGroupID.ID,
		CrossingMinimizationGroupOrder.ID,
		CrossingMinimizationEnforcedGroups.ID,
	} {
		if registry.ByID(id) == nil {
			t.Errorf("ELK 0.12 option %q is not registered", id)
		}
	}
}
