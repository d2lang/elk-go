// Copyright (c) 2025 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package preserveorder

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// CalculateModelOrderOrGroupModelOrder is ELK 0.12's
// CMGroupModelOrderCalculator. ENFORCED group order applies only when both
// compared elements belong to one of the configured enforced groups.
func CalculateModelOrderOrGroupModelOrder(
	graph *layered.LGraph,
	element, other *basegraph.PropertyMap,
	offset int,
) int {
	if !basegraph.Has(element, layered.ModelOrder) {
		return -1
	}
	if basegraph.Get(&graph.Properties, layered.CrossingMinimizationGroupOrder) ==
		layered.GroupOrderEnforced {
		enforced := basegraph.Get(
			&graph.Properties,
			layered.CrossingMinimizationEnforcedGroups,
		)
		elementGroup := basegraph.Get(element, layered.CrossingMinimizationGroupID)
		otherGroup := basegraph.Get(other, layered.CrossingMinimizationGroupID)
		if containsGroup(enforced, elementGroup) && containsGroup(enforced, otherGroup) {
			return offset*elementGroup + basegraph.Get(element, layered.ModelOrder)
		}
	}
	return basegraph.Get(element, layered.ModelOrder)
}

func containsGroup(groups []int, group int) bool {
	for _, candidate := range groups {
		if candidate == group {
			return true
		}
	}
	return false
}
