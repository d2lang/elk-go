// Copyright (c) 2016, 2020 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"sort"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// ProcessSemiInteractiveCrossing derives pairwise in-layer constraints from
// explicitly supplied node positions while leaving dummy nodes unconstrained.
func ProcessSemiInteractiveCrossing(arena *layered.LGraphArena, graphID layered.LGraphID) {
	addedConstraints := false
	for _, layerID := range arena.Graph(graphID).Layers {
		positioned := make([]layered.LNodeID, 0)
		for _, nodeID := range arena.Layer(layerID).Nodes {
			node := arena.Node(nodeID)
			if node.Type == layered.NormalNode && basegraph.Has(&node.Properties, layered.PositionOption) {
				positioned = append(positioned, nodeID)
			}
		}
		sort.SliceStable(positioned, func(first, second int) bool {
			firstPosition := basegraph.Get(&arena.Node(positioned[first]).Properties, layered.PositionOption)
			secondPosition := basegraph.Get(&arena.Node(positioned[second]).Properties, layered.PositionOption)
			return firstPosition.Y < secondPosition.Y
		})
		for index := 1; index < len(positioned); index++ {
			previous := positioned[index-1]
			successors := basegraph.Get(&arena.Node(previous).Properties, layered.InLayerSuccessorConstraints)
			successors = append(successors, positioned[index])
			basegraph.Set(&arena.Node(previous).Properties, layered.InLayerSuccessorConstraints, successors)
		}
		// Stream.reduce returns a present Optional even for a single node.
		addedConstraints = addedConstraints || len(positioned) > 0
	}
	if addedConstraints {
		basegraph.Set(&arena.Graph(graphID).Properties,
			layered.InLayerSuccessorConstraintsBetweenNonDummies, true)
	}
}
