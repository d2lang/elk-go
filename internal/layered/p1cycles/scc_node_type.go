// Copyright (c) 2025 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p1cycles

import (
	"math"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// SCCNodeTypeCycleBreaker ports ELK 0.12's SCC_NODE_TYPE strategy. It first
// finds strongly connected components, then chooses reversals from each
// component using constrained model order and the preferred source/target
// group IDs.
type SCCNodeTypeCycleBreaker struct{}

func (*SCCNodeTypeCycleBreaker) ProcessorConfiguration(
	_ *layered.LGraphArena,
	_ layered.LGraphID,
) layered.ProcessorConfiguration {
	return layered.GreedyCycleConfiguration()
}

func (*SCCNodeTypeCycleBreaker) Process(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) error {
	graph := arena.Graph(graphID)
	offset := max(len(graph.LayerlessNodes),
		basegraph.Get(&graph.Properties, layered.MaximumModelOrderNodes))
	groupCount := cycleBreakingGroupCount(arena, graphID)
	bigOffset := offset * groupCount

	for {
		components := stronglyConnectedComponents(arena, graphID)
		if len(components) == 0 {
			return nil
		}

		toReverse := make([]layered.LEdgeID, 0)
		for _, component := range components {
			toReverse = append(toReverse,
				findSCCNodeTypeReversals(arena, graphID, component, offset, bigOffset)...)
		}
		for _, edgeID := range toReverse {
			arena.ReverseEdge(graphID, edgeID, false)
			newSource := arena.EdgeSourceNode(edgeID)
			basegraph.Set(
				&arena.Node(newSource).Properties,
				layered.LayerIDOption,
				basegraph.Get(&arena.Node(newSource).Properties, layered.LayerIDOption)+1,
			)
			basegraph.Set(&graph.Properties, layered.Cyclic, true)
		}
	}
}

func findSCCNodeTypeReversals(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	component []layered.LNodeID,
	offset, bigOffset int,
) []layered.LEdgeID {
	if len(component) <= 1 {
		return nil
	}
	graph := arena.Graph(graphID)
	calculator := groupModelOrderCalculator{}
	enforceGroups := basegraph.Get(&graph.Properties, layered.CycleBreakingGroupOrder) ==
		layered.GroupOrderEnforced
	componentSet := make(map[layered.LNodeID]struct{}, len(component))
	for _, nodeID := range component {
		componentSet[nodeID] = struct{}{}
	}

	var minimum, maximum layered.LNodeID
	minimumOrder, maximumOrder := math.MaxInt, math.MinInt
	for index, nodeID := range component {
		order := calculator.constraintModelOrder(
			arena,
			nodeID,
			offset,
			bigOffset,
			enforceGroups,
		)
		if index == 0 || minimumOrder > order {
			minimum, minimumOrder = nodeID, order
		}
		if index == 0 || maximumOrder < order {
			maximum, maximumOrder = nodeID, order
		}
	}

	preferredSource := basegraph.Get(
		&graph.Properties, layered.CycleBreakingPreferredSourceID)
	preferredTarget := basegraph.Get(
		&graph.Properties, layered.CycleBreakingPreferredTargetID)
	if basegraph.Get(&arena.Node(minimum).Properties, layered.CycleBreakingGroupID) ==
		preferredSource {
		return incomingEdgesWithin(arena, minimum, componentSet)
	}
	if basegraph.Get(&arena.Node(maximum).Properties, layered.CycleBreakingGroupID) ==
		preferredTarget {
		// Preserve SCCNodeTypeCycleBreaker's source-membership check: the
		// maximum node is itself in the component, so every outgoing edge is
		// selected here, including one that leaves the SCC.
		return append([]layered.LEdgeID(nil), arena.NodeOutgoingEdges(maximum)...)
	}
	if len(arena.NodeIncomingEdges(minimum)) > len(arena.NodeOutgoingEdges(maximum)) {
		return incomingEdgesWithin(arena, minimum, componentSet)
	}
	return outgoingEdgesWithin(arena, maximum, componentSet)
}

func incomingEdgesWithin(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	component map[layered.LNodeID]struct{},
) []layered.LEdgeID {
	edges := make([]layered.LEdgeID, 0)
	for _, edgeID := range arena.NodeIncomingEdges(nodeID) {
		if _, inside := component[arena.EdgeSourceNode(edgeID)]; inside {
			edges = append(edges, edgeID)
		}
	}
	return edges
}

func outgoingEdgesWithin(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	component map[layered.LNodeID]struct{},
) []layered.LEdgeID {
	edges := make([]layered.LEdgeID, 0)
	for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
		if _, inside := component[arena.EdgeTargetNode(edgeID)]; inside {
			edges = append(edges, edgeID)
		}
	}
	return edges
}

type groupModelOrderCalculator struct {
	firstSeparate int
	lastSeparate  int
}

func (calculator *groupModelOrderCalculator) constraintModelOrder(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	smallOffset, bigOffset int,
	enforceGroups bool,
) int {
	node := arena.Node(nodeID)
	offset := smallOffset
	if enforceGroups {
		offset = bigOffset
	}
	order := 0
	switch basegraph.Get(&node.Properties, layered.LayerConstraintOption) {
	case layered.LayerConstraintFirstSeparate:
		order = -2*offset + calculator.firstSeparate
		calculator.firstSeparate++
	case layered.LayerConstraintFirst:
		order = -offset
	case layered.LayerConstraintLast:
		order = offset
	case layered.LayerConstraintLastSeparate:
		order = 2*offset + calculator.lastSeparate
		calculator.lastSeparate++
	}
	if modelOrder, present := basegraph.TryGet(&node.Properties, layered.ModelOrder); present {
		if enforceGroups {
			order += basegraph.Get(&node.Properties, layered.CycleBreakingGroupID)*smallOffset +
				modelOrder
		} else {
			order += modelOrder
		}
	}
	return order
}

func cycleBreakingGroupCount(arena *layered.LGraphArena, graphID layered.LGraphID) int {
	groups := make(map[int]struct{})
	for _, nodeID := range arena.Graph(graphID).LayerlessNodes {
		if !basegraph.Has(&arena.Node(nodeID).Properties, layered.ModelOrder) ||
			!basegraph.Has(&arena.Node(nodeID).Properties, layered.CycleBreakingGroupID) {
			continue
		}
		groups[basegraph.Get(&arena.Node(nodeID).Properties, layered.CycleBreakingGroupID)] = struct{}{}
	}
	return len(groups)
}

func stronglyConnectedComponents(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) [][]layered.LNodeID {
	nodes := arena.Graph(graphID).LayerlessNodes
	nextIndex := 0
	indices := make(map[layered.LNodeID]int, len(nodes))
	lowlinks := make(map[layered.LNodeID]int, len(nodes))
	onStack := make(map[layered.LNodeID]bool, len(nodes))
	stack := make([]layered.LNodeID, 0, len(nodes))
	components := make([][]layered.LNodeID, 0)

	var connect func(layered.LNodeID)
	connect = func(nodeID layered.LNodeID) {
		indices[nodeID] = nextIndex
		lowlinks[nodeID] = nextIndex
		nextIndex++
		stack = append(stack, nodeID)
		onStack[nodeID] = true

		for _, edgeID := range arena.NodeOutgoingEdges(nodeID) {
			target := arena.EdgeTargetNode(edgeID)
			targetIndex, visited := indices[target]
			if !visited {
				connect(target)
				lowlinks[nodeID] = min(lowlinks[nodeID], lowlinks[target])
			} else if onStack[target] {
				lowlinks[nodeID] = min(lowlinks[nodeID], targetIndex)
			}
		}

		if lowlinks[nodeID] != indices[nodeID] {
			return
		}
		component := make([]layered.LNodeID, 0)
		for {
			last := len(stack) - 1
			member := stack[last]
			stack = stack[:last]
			onStack[member] = false
			component = append(component, member)
			if member == nodeID {
				break
			}
		}
		if len(component) > 1 {
			components = append(components, component)
		}
	}

	for _, nodeID := range nodes {
		if _, visited := indices[nodeID]; !visited {
			connect(nodeID)
		}
	}
	return components
}
