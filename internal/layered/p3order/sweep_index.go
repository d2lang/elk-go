// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import "github.com/d2lang/elk-go/internal/layered"

// sweepIndex maps every node of an initialized graph info holder to that
// holder and to the node's slot: its position in the holder's initial node
// order read layer by layer. Holder initialization sets the scratch IDs
// LNode.ID (index in the layer) and Layer.ID (index of the layer), and
// nothing changes them until the sweeps are over, so for a node of holder h
//
//	[Layer.ID][LNode.ID] in h's per-layer arrays == [slot] in their flat backing
//
// which is what the per-layer state lookups below use instead of the two
// indirections. A lookup for a node of any other holder (or of none) keeps
// the original two-level expression, so its result never changes.
type sweepIndex struct {
	nodes []nodeRef // by LNodeID
}

// nodeRef is a node's owner (holder index + 1; 0 = none) and slot.
type nodeRef struct {
	owner int32
	slot  int32
}

func (index *sweepIndex) reset(nodeCount int) {
	if cap(index.nodes) < nodeCount {
		index.nodes = make([]nodeRef, nodeCount)
		return
	}
	index.nodes = index.nodes[:nodeCount]
	clear(index.nodes)
}

func (index *sweepIndex) set(nodeID layered.LNodeID, owner int32, slot int) {
	if int(nodeID) < len(index.nodes) {
		index.nodes[nodeID] = nodeRef{owner: owner, slot: int32(slot)}
	}
}

// slotOf returns nodeID's slot if owner owns it.
func (index *sweepIndex) slotOf(nodeID layered.LNodeID, owner int32) (int, bool) {
	if index == nil || int(nodeID) >= len(index.nodes) {
		return 0, false
	}
	ref := index.nodes[nodeID]
	return int(ref.slot), ref.owner == owner && owner != 0
}
