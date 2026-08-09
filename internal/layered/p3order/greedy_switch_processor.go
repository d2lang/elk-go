// Copyright (c) 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"fmt"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// GreedySwitchIntermediateForGraph implements GraphConfigurator's exact
// activation decision. It must be called while the graph is still layerless,
// since the threshold is compared to LGraph.layerlessNodes.size().
func GreedySwitchIntermediateForGraph(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) (layered.IntermediateProcessorStrategy, bool) {
	graph := arena.Graph(graphID)
	hierarchical := basegraph.Get(&graph.Properties, layered.Hierarchy) == layered.HierarchyIncludeChildren
	var switchType layered.GreedySwitchType
	if hierarchical {
		switchType = basegraph.Get(&graph.Properties, layered.GreedySwitchHierarchical)
		if graph.ParentNode != nil || switchType == layered.GreedySwitchOff {
			return 0, false
		}
	} else {
		switchType = basegraph.Get(&graph.Properties, layered.GreedySwitch)
		interactive := basegraph.Get(&graph.Properties, layered.SemiInteractive) ||
			basegraph.Get(&graph.Properties, layered.CrossingMinimization) == layered.CrossingMinimizationInteractive
		threshold := basegraph.Get(&graph.Properties, layered.GreedySwitchActivationThreshold)
		if interactive || switchType == layered.GreedySwitchOff ||
			threshold != 0 && threshold <= len(graph.LayerlessNodes) {
			return 0, false
		}
	}
	if switchType == layered.GreedySwitchOneSided {
		return layered.OneSidedGreedySwitch, true
	}
	return layered.TwoSidedGreedySwitch, true
}

// NewGreedySwitchPostprocessor constructs IntermediateProcessorStrategy's
// exact LayerSweepCrossingMinimizer variant.
func NewGreedySwitchPostprocessor(
	strategy layered.IntermediateProcessorStrategy,
) (*LayerSweepCrossingMinimizer, error) {
	switch strategy {
	case layered.OneSidedGreedySwitch:
		return NewLayerSweepCrossingMinimizer(CrossMinOneSidedGreedySwitch), nil
	case layered.TwoSidedGreedySwitch:
		return NewLayerSweepCrossingMinimizer(CrossMinTwoSidedGreedySwitch), nil
	default:
		return nil, fmt.Errorf("p3order: strategy %d is not a greedy-switch processor", strategy)
	}
}

func ProcessGreedySwitchIntermediate(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	strategy layered.IntermediateProcessorStrategy,
) error {
	processor, err := NewGreedySwitchPostprocessor(strategy)
	if err != nil {
		return err
	}
	return processor.ProcessHierarchy(arena, graphID)
}
