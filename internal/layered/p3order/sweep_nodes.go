// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import "github.com/d2lang/elk-go/internal/layered"

// sweepNode is what the sweeps read about one node of a holder that does not
// change while they run, indexed by the node's slot (see sweepIndex): the
// slice header of its port list, its layer key and whether it has a nested
// graph. During the sweeps port lists are only permuted in place
// (distributePorts and sortPortsByDummyPositionsInLastLayer write through
// the header they read and store it back unchanged; only the final transfer
// assigns new lists), and nodes keep their layers and nested graphs (see
// portTable), so each field equals the arena expression it replaces.
type sweepNode struct {
	ports  []layered.LPortID
	layer  int64
	nested bool

	// blocks and counts remember, for a node without nested graph, what
	// distributeBlocksFromTable found when it first served the node (see
	// distributePortsBySlot): the number of its ports on each side.
	blocks blockState
	counts [layered.WestSide + 1]uint8
}

// blockState is what a sweepNode remembers about its port list.
type blockState uint8

const (
	blocksUnknown blockState = iota
	blocksTrivial            // at most one port per side, sorted by side
	blocksCounted            // sorted by side, with the counts per side
)

// buildSweepNodes gives every holder its nodes' sweepNodes by slot. It runs
// with the port table, after every holder has been initialized; the
// holders' node orders are still their initial orders, in which the slots
// were numbered.
func buildSweepNodes(ws *crossMinWorkspace, arena *layered.LGraphArena, holders []*GraphInfoHolder) {
	for _, holder := range holders {
		total := 0
		for _, layer := range holder.CurrentNodeOrder {
			total += len(layer)
		}
		nodes := ws.sweepNodes.take(total)
		slot := 0
		for _, layer := range holder.CurrentNodeOrder {
			for _, nodeID := range layer {
				node := arena.Node(nodeID)
				nodes[slot] = sweepNode{ports: node.Ports, layer: layerKey(node.Layer), nested: node.NestedGraph != nil}
				slot++
			}
		}
		if distributor, ok := holder.PortDistributor.(*BarycenterPortDistributor); ok && distributor.index != nil {
			distributor.nodes = nodes
		}
	}
}

// slotsOf appends the slots of nodes, all owned by owner, to slots; false
// when some node is not owner's.
func (index *sweepIndex) slotsOf(nodes []layered.LNodeID, owner int32, slots []int32) ([]int32, bool) {
	if index == nil || owner == 0 {
		return slots, false
	}
	refs := index.nodes
	for _, nodeID := range nodes {
		if int(nodeID) >= len(refs) {
			return slots, false
		}
		ref := refs[nodeID]
		if ref.owner != owner {
			return slots, false
		}
		slots = append(slots, ref.slot)
	}
	return slots, true
}
