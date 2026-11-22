// Copyright (c) 2010, 2015 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// ProcessInLayerConstraints moves TOP nodes before ordinary nodes and BOTTOM
// nodes to the end while preserving the encounter order within each group.
// inLayerConstraintScratch holds ProcessInLayerConstraints' per-layer
// snapshot and bottom list (layered.ScratchInLayerConstraints), refilled for
// every layer.
type inLayerConstraintScratch struct {
	original []layered.LNodeID
	bottom   []layered.LNodeID
}

func ProcessInLayerConstraints(arena *layered.LGraphArena, graphID layered.LGraphID) {
	scratch, _ := arena.Scratch(layered.ScratchInLayerConstraints).(*inLayerConstraintScratch)
	if scratch == nil {
		scratch = &inLayerConstraintScratch{}
		arena.SetScratch(layered.ScratchInLayerConstraints, scratch)
	}
	for _, layerID := range arena.Graph(graphID).Layers {
		original := append(scratch.original[:0], arena.Layer(layerID).Nodes...)
		scratch.original = original
		topInsertionIndex := -1
		bottom := scratch.bottom[:0]
		for index, nodeID := range original {
			constraint := basegraph.Get(&arena.Node(nodeID).Properties, layered.InternalInLayerConstraint)
			if topInsertionIndex < 0 {
				if constraint != layered.InLayerConstraintTop {
					topInsertionIndex = index
				}
			} else if constraint == layered.InLayerConstraintTop {
				arena.SetNodeLayer(nodeID, nil)
				arena.SetNodeLayerAt(nodeID, &layerID, topInsertionIndex)
				topInsertionIndex++
			}
			if constraint == layered.InLayerConstraintBottom {
				bottom = append(bottom, nodeID)
			}
		}
		scratch.bottom = bottom
		for _, nodeID := range bottom {
			arena.SetNodeLayer(nodeID, nil)
			arena.SetNodeLayer(nodeID, &layerID)
		}
	}
}
