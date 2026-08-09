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

func TestELK081EnumSurfaces(t *testing.T) {
	registry := core.NewOptionRegistry()
	RegisterOptions(registry)

	tests := []struct {
		id   string
		want []string
	}{
		{EdgeLabelsPlacement.ID, []string{"CENTER", "HEAD", "TAIL"}},
		{ConsiderModelOrder.ID, []string{"NONE", "NODES_AND_EDGES", "PREFER_EDGES"}},
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
	if _, ok := registry.ByID(ConsiderModelOrder.ID).ParseValue("PREFER_NODES"); ok {
		t.Error("model-order strategy accepted non-ELK-0.8.1 value PREFER_NODES")
	}
}
