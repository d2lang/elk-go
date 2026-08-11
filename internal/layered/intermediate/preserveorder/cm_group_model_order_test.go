// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package preserveorder

import (
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestCalculateModelOrderOrGroupModelOrderEnforcesConfiguredGroups(t *testing.T) {
	graph := &layered.LGraph{}
	basegraph.Set(
		&graph.Properties,
		layered.CrossingMinimizationGroupOrder,
		layered.GroupOrderEnforced,
	)
	var first, second basegraph.PropertyMap
	basegraph.Set(&first, layered.ModelOrder, 1)
	basegraph.Set(&first, layered.CrossingMinimizationGroupID, 2)
	basegraph.Set(&second, layered.ModelOrder, 7)
	basegraph.Set(&second, layered.CrossingMinimizationGroupID, 1)

	if got := CalculateModelOrderOrGroupModelOrder(graph, &first, &second, 8); got != 17 {
		t.Fatalf("first enforced order = %d, want 17", got)
	}
	if got := CalculateModelOrderOrGroupModelOrder(graph, &second, &first, 8); got != 15 {
		t.Fatalf("second enforced order = %d, want 15", got)
	}

	basegraph.Set(&second, layered.CrossingMinimizationGroupID, 3)
	if got := CalculateModelOrderOrGroupModelOrder(graph, &first, &second, 8); got != 1 {
		t.Fatalf("order with a non-enforced peer = %d, want model order 1", got)
	}
}
