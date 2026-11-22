// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
)

// singleNodeDisabled forces the general path for single-node roots (for
// tests).
var singleNodeDisabled bool

// processSingleNode is ProcessHierarchy for a hierarchy that is only its
// root graph, whose one layer holds a single node without ports and without
// a nested graph, minimized by the barycenter heuristic with a model-order
// strategy, at least one try and no node-influence counter. For any other
// input it does nothing and returns false (the caller has already returned
// for an empty graph or a non-hierarchical single node).
//
// The general path for such a root: initialize gives the root a Random when
// it has none (seeded from RandomSeed) and draws the minimizer's seed with
// NextLong; it numbers the root graph, its layer and its node 0 (graph
// discovery and the holder's initializers); the holder has no child graph
// and sweeps bottom-up (no parent node), so the root is the one graph swept
// on; compareDifferentRandomizedLayouts resets the Random to the drawn seed,
// sets FirstTryWithInitialOrder (model order is considered) and runs the
// first try, which draws the sweep direction with NextBoolean and returns 0
// because the order is tried as it is and a single node without ports has no
// crossing; zero crossings end the tries. No graph changed its order, so no
// best order exists and nothing is transferred. Everything else it does is
// in its workspace, and the properties it reads have non-materializing
// defaults.
func (minimizer *LayerSweepCrossingMinimizer) processSingleNode(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) bool {
	if minimizer.Type != CrossMinBarycenter {
		return false
	}
	graph := arena.Graph(graphID)
	layerID := graph.Layers[0]
	nodeID := arena.Layer(layerID).Nodes[0]
	node := arena.Node(nodeID)
	if len(node.Ports) != 0 || node.NestedGraph != nil ||
		basegraph.Get(&graph.Properties, modelOrderCrossingCounterNodeInfluence) != 0 ||
		basegraph.Get(&graph.Properties, layered.ConsiderModelOrder) == layered.OrderingNone ||
		basegraph.Get(&graph.Properties, layered.Thoroughness) < 1 {
		return false
	}
	random, found := basegraph.TryGet(&graph.Properties, layered.Random)
	if !found || random == nil {
		random = javacompat.NewRandom(int64(basegraph.Get(&graph.Properties, layered.RandomSeed)))
		basegraph.Set(&graph.Properties, layered.Random, random)
	}
	seed := random.NextLong()
	graph.ID = 0
	arena.Layer(layerID).ID = 0
	node.ID = 0
	random.SetSeed(seed)
	basegraph.Set(&graph.Properties, layered.FirstTryWithInitialOrder, true)
	random.NextBoolean()
	return true
}
