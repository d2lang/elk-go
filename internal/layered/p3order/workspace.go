// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/reuse"
)

// slab hands out zeroed slices of T cut from a few large chunks. Every
// slice has its capacity capped at its length, so appending to it never
// writes into a neighbour. release zeroes everything handed out and makes
// the storage available again; the unused part of every chunk is always
// zero, so take never has to clear.
//
// A nil *slab allocates every slice with make, which is what the code did
// before slabs existed.
type slab[T any] struct {
	chunks [][]T
	chunk  int // chunk handing out slices
	used   int // elements of chunks[chunk] handed out
	total  int // elements of all chunks
}

const slabMinimumChunk = 16

func (s *slab[T]) take(n int) []T {
	if s == nil {
		return make([]T, n)
	}
	for s.chunk < len(s.chunks) {
		current := s.chunks[s.chunk]
		if s.used+n <= len(current) {
			out := current[s.used : s.used+n : s.used+n]
			s.used += n
			return out
		}
		if s.chunk+1 == len(s.chunks) {
			break
		}
		s.chunk++
		s.used = 0
	}
	size := slabMinimumChunk
	if len(s.chunks) > 0 {
		size = 2 * len(s.chunks[len(s.chunks)-1])
	}
	for size < n {
		size *= 2
	}
	s.chunks = append(s.chunks, make([]T, size))
	s.total += size
	s.chunk, s.used = len(s.chunks)-1, n
	return s.chunks[s.chunk][:n:n]
}

// recycle makes the slab's storage available again without clearing it:
// for a slab whose takers overwrite everything they take before reading it
// (take's contents are then unspecified).
func (s *slab[T]) recycle() {
	s.chunk, s.used = 0, 0
}

// elements is the number of elements the slab's chunks hold.
func (s *slab[T]) elements() int { return s.total }

// one returns a pointer to a zeroed T.
func (s *slab[T]) one() *T {
	if s == nil {
		return new(T)
	}
	return &s.take(1)[0]
}

func (s *slab[T]) release() {
	if s.chunk == 0 && s.used == 0 {
		return // nothing handed out since the last release
	}
	for index := 0; index < len(s.chunks) && index <= s.chunk; index++ {
		used := len(s.chunks[index])
		if index == s.chunk {
			used = s.used
		}
		clear(s.chunks[index][:used])
	}
	s.chunk, s.used = 0, 0
}

// crossMinWorkspace is the storage one ProcessHierarchy call uses for its
// graph info holders and sweeps. Everything taken from it is zeroed, exactly
// as a fresh allocation would be, and nothing taken from it is referenced
// once the call has returned; the workspace then goes back to a pool.
type crossMinWorkspace struct {
	holders      slab[GraphInfoHolder]
	allCounters  slab[AllCrossingsCounter]
	counters     slab[CrossingsCounter]
	deciders     slab[LayerSweepTypeDecider]
	distributors slab[BarycenterPortDistributor]
	resolvers    slab[ForsterConstraintResolver]
	heuristics   slab[BarycenterHeuristic]

	ints        slab[int]
	bools       slab[bool]
	float32s    slab[float32]
	nodeIDs     slab[layered.LNodeID]
	graphIDs    slab[layered.LGraphID]
	nodeLists   slab[[]layered.LNodeID]
	intLists    slab[[]int]
	states      slab[BarycenterState]
	stateLists  slab[[]BarycenterState]
	infos       slab[layerSweepNodeInfo]
	infoLists   slab[[]layerSweepNodeInfo]
	constraints slab[layered.PortConstraints]
	portEntries slab[portEntry]
	edgeEnds    slab[edgeEnd]
	sweepNodes  slab[sweepNode]
	portTable   portTable // of one call (see buildPortTable)

	// Scratch shared by all holders: each user empties, truncates or resets
	// it on entry and no call that uses one nests inside another call using
	// it (counting, minimizing one layer, distributing one node's ports).
	countScratch       countScratch
	heuristicScratch   heuristicScratch
	distributorScratch distributorScratch

	deciderDummies []layered.LNodeID

	// The constraint resolvers' shared scratch (see forsterScratch).
	forsterScratch forsterScratch

	// Remembered property reads of the hierarchical port helpers (see
	// portFact), by LPortID and LNodeID; taken on first use in a call.
	portFacts      slab[portFact]
	originFacts    slab[originFact]
	portFactList   []portFact
	originFactList []originFact

	// Storage of the sweep copies' arrays. Copies overwrite everything they
	// take before reading it, so these slabs are recycled without clearing
	// (their contents are unspecified); releasing the workspace drops every
	// copy's reference to them.
	copyLayers     slab[[]layered.LNodeID]
	copyPortLayers slab[[][]layered.LPortID]
	copyNodes      slab[layered.LNodeID]
	copyPortLists  slab[[]layered.LPortID]
	copyPorts      slab[layered.LPortID]

	// The node index of one call (reset by it).
	index sweepIndex

	// Lists of one call, kept for their capacity.
	holderList  []*GraphInfoHolder
	sweepOn     []int
	graphQueue  []layered.LGraphID
	changedList []int

	// Sweep copies of one call. A copy is only read after fill or fillFrom
	// has overwritten all of it, so a used one serves as a new one.
	copies     []*SweepCopy
	copiesUsed int
}

// sweepCopy returns an unused sweep copy (to be filled by the caller) whose
// arrays come from the workspace.
func (ws *crossMinWorkspace) sweepCopy() *SweepCopy {
	if ws == nil {
		return &SweepCopy{}
	}
	if ws.copiesUsed < len(ws.copies) {
		copy := ws.copies[ws.copiesUsed]
		ws.copiesUsed++
		return copy
	}
	copy := &SweepCopy{ws: ws}
	ws.copies = append(ws.copies, copy)
	ws.copiesUsed++
	return copy
}

// releasedCrossMinWorkspaces keeps released workspaces: one in a slot that
// garbage collection does not empty, so that sequential layouts always find
// it (a sync.Pool loses its values at collections and hides a value put on
// another processor's private slot), and more in a sync.Pool for concurrent
// layouts. Only workspaces of bounded size go to the slot; a larger one goes
// to the sync.Pool only (gc), which garbage collection empties, or is dropped
// (TinyGo, whose sync.Pool never frees what it holds; see
// reuse.OversizedToPool). Which workspace a call gets never matters:
// everything taken from one is reset to the state a new one has.
var releasedCrossMinWorkspaces reuse.Pool[crossMinWorkspace]

// maxSlotWorkspaceElements bounds the slab storage (in elements), and
// maxSlotWorkspaceBytes all the storage (see retainedBytes), of a workspace
// that goes to the slot (it may stay alive indefinitely); a larger one does
// not (see releasedCrossMinWorkspaces).
// The corpus's largest workspace takes 430 KB in 22512 slab elements.
const (
	maxSlotWorkspaceElements = 1 << 16
	maxSlotWorkspaceBytes    = 2 << 20
)

func acquireCrossMinWorkspace() *crossMinWorkspace {
	if ws := releasedCrossMinWorkspaces.Get(); ws != nil {
		return ws
	}
	return &crossMinWorkspace{}
}

func releaseCrossMinWorkspace(ws *crossMinWorkspace) {
	oversized := ws.elements() > maxSlotWorkspaceElements || ws.retainedBytes() > maxSlotWorkspaceBytes
	if oversized && !reuse.OversizedToPool {
		return // dropped (see releasedCrossMinWorkspaces)
	}
	ws.holders.release()
	ws.allCounters.release()
	ws.counters.release()
	ws.deciders.release()
	ws.distributors.release()
	ws.resolvers.release()
	ws.heuristics.release()
	ws.ints.release()
	ws.bools.release()
	ws.float32s.release()
	ws.nodeIDs.release()
	ws.graphIDs.release()
	ws.nodeLists.release()
	ws.intLists.release()
	ws.states.release()
	ws.stateLists.release()
	ws.infos.release()
	ws.infoLists.release()
	ws.constraints.release()
	// The table's entries and ends are all written by buildPortTable before
	// anything reads them (and hold no pointers).
	ws.portEntries.recycle()
	ws.edgeEnds.recycle()
	ws.sweepNodes.release()
	ws.portFacts.release()
	ws.originFacts.release()
	ws.portFactList, ws.originFactList = nil, nil
	ws.portTable = portTable{}
	// A count that panics (the pinned elk-go panics inside the north/south
	// counter for some graphs) leaves its pending ends behind. Every count
	// that returns pops all of them and a call that panicked is not
	// released; emptying them here keeps the next call's counts starting
	// with none, as a fresh counter does, without relying on either.
	ws.countScratch.ends = ws.countScratch.ends[:0]
	clear(ws.holderList[:cap(ws.holderList)])
	ws.holderList = ws.holderList[:0]
	ws.forsterScratch.release()
	for _, copy := range ws.copies[:ws.copiesUsed] {
		*copy = SweepCopy{ws: ws}
	}
	ws.copiesUsed = 0
	ws.copyLayers.recycle()
	ws.copyPortLayers.recycle()
	ws.copyNodes.recycle()
	ws.copyPortLists.recycle()
	ws.copyPorts.recycle()
	releasedCrossMinWorkspaces.Put(ws, !oversized)
}

// Bytes per element of the workspace's storage on a 64-bit platform (bounds
// checked by a test against the types' sizes).
const (
	bytesGraphInfoHolder       = 192
	bytesAllCrossingsCounter   = 88
	bytesCrossingsCounter      = 48
	bytesLayerSweepTypeDecider = 64
	bytesPortDistributor       = 416
	bytesConstraintResolver    = 112
	bytesBarycenterHeuristic   = 128
	bytesBarycenterState       = 104
	bytesNodeInfo              = 24
	bytesPortEntry             = 20
	bytesEdgeEnd               = 24
	bytesSweepNode             = 40
	bytesFact                  = 8
	bytesSweepCopy             = 128
	bytesConstraintGroup       = 96
	bytesNodeRef               = 8
	bytesSlice                 = 24 // a slice header
	bytesPointer               = 8
	bytesMapEntry              = 48 // a small-keyed map entry with its share of the map's overhead
)

// retainedBytes bounds the bytes of storage ws keeps on a 64-bit platform:
// its slabs' chunks, the scratch and lists it keeps for their capacity, its
// sweep copies, the constraint resolvers' scratch and group map.
func (ws *crossMinWorkspace) retainedBytes() int {
	bytes := ws.holders.elements()*bytesGraphInfoHolder +
		ws.allCounters.elements()*bytesAllCrossingsCounter +
		ws.counters.elements()*bytesCrossingsCounter +
		ws.deciders.elements()*bytesLayerSweepTypeDecider +
		ws.distributors.elements()*bytesPortDistributor +
		ws.resolvers.elements()*bytesConstraintResolver +
		ws.heuristics.elements()*bytesBarycenterHeuristic +
		ws.ints.elements()*8 + ws.bools.elements() + ws.float32s.elements()*4 +
		ws.nodeIDs.elements()*4 + ws.graphIDs.elements()*4 +
		(ws.nodeLists.elements()+ws.intLists.elements()+ws.stateLists.elements()+ws.infoLists.elements())*bytesSlice +
		ws.states.elements()*bytesBarycenterState + ws.infos.elements()*bytesNodeInfo +
		ws.constraints.elements() + ws.portEntries.elements()*bytesPortEntry + ws.edgeEnds.elements()*bytesEdgeEnd +
		ws.sweepNodes.elements()*bytesSweepNode + (ws.portFacts.elements()+ws.originFacts.elements())*bytesFact +
		(ws.copyLayers.elements()+ws.copyPortLayers.elements()+ws.copyPortLists.elements())*bytesSlice +
		(ws.copyNodes.elements()+ws.copyPorts.elements())*4
	scratch := &ws.countScratch
	bytes += cap(scratch.ports)*4 + (cap(scratch.ends)+cap(scratch.cardinalities))*8 +
		(cap(scratch.tree.binarySums)+cap(scratch.tree.numsPerIndex))*8
	heuristic := &ws.heuristicScratch
	bytes += (cap(heuristic.layerBuffer) + cap(heuristic.sortBuffer) + cap(heuristic.slots)) * 4
	distributor := &ws.distributorScratch
	bytes += (cap(distributor.sidePorts) + cap(distributor.inLayerPorts) + cap(distributor.southPorts) +
		cap(distributor.northPorts) + cap(distributor.freeSlots) + cap(distributor.fixedSlots)) * 4
	bytes += cap(ws.deciderDummies) * 4
	bytes += ws.forsterScratch.retainedBytes()
	bytes += cap(ws.index.nodes)*bytesNodeRef + cap(ws.holderList)*bytesPointer +
		(cap(ws.sweepOn)+cap(ws.changedList))*8 + cap(ws.graphQueue)*4 +
		cap(ws.copies)*bytesPointer + len(ws.copies)*bytesSweepCopy
	return bytes
}

// elements is the number of elements the workspace's slabs hold.
func (ws *crossMinWorkspace) elements() int {
	return ws.holders.elements() + ws.allCounters.elements() + ws.counters.elements() +
		ws.deciders.elements() + ws.distributors.elements() + ws.resolvers.elements() +
		ws.heuristics.elements() + ws.ints.elements() + ws.bools.elements() + ws.float32s.elements() +
		ws.nodeIDs.elements() + ws.graphIDs.elements() + ws.nodeLists.elements() + ws.intLists.elements() +
		ws.states.elements() + ws.stateLists.elements() + ws.infos.elements() + ws.infoLists.elements() +
		ws.constraints.elements() + ws.portEntries.elements() + ws.edgeEnds.elements() +
		ws.sweepNodes.elements() + ws.portFacts.elements() + ws.originFacts.elements() + ws.copyLayers.elements() + ws.copyPortLayers.elements() +
		ws.copyNodes.elements() + ws.copyPortLists.elements() + ws.copyPorts.elements()
}

// The accessors return nil for a nil workspace, so callers without one
// allocate as before.

func (ws *crossMinWorkspace) holderSlab() *slab[GraphInfoHolder] {
	if ws == nil {
		return nil
	}
	return &ws.holders
}

func (ws *crossMinWorkspace) allCounterSlab() *slab[AllCrossingsCounter] {
	if ws == nil {
		return nil
	}
	return &ws.allCounters
}

func (ws *crossMinWorkspace) counterSlab() *slab[CrossingsCounter] {
	if ws == nil {
		return nil
	}
	return &ws.counters
}

func (ws *crossMinWorkspace) deciderSlab() *slab[LayerSweepTypeDecider] {
	if ws == nil {
		return nil
	}
	return &ws.deciders
}

func (ws *crossMinWorkspace) distributorSlab() *slab[BarycenterPortDistributor] {
	if ws == nil {
		return nil
	}
	return &ws.distributors
}

func (ws *crossMinWorkspace) resolverSlab() *slab[ForsterConstraintResolver] {
	if ws == nil {
		return nil
	}
	return &ws.resolvers
}

func (ws *crossMinWorkspace) heuristicSlab() *slab[BarycenterHeuristic] {
	if ws == nil {
		return nil
	}
	return &ws.heuristics
}

func (ws *crossMinWorkspace) intSlab() *slab[int] {
	if ws == nil {
		return nil
	}
	return &ws.ints
}

func (ws *crossMinWorkspace) boolSlab() *slab[bool] {
	if ws == nil {
		return nil
	}
	return &ws.bools
}

func (ws *crossMinWorkspace) float32Slab() *slab[float32] {
	if ws == nil {
		return nil
	}
	return &ws.float32s
}

func (ws *crossMinWorkspace) nodeIDSlab() *slab[layered.LNodeID] {
	if ws == nil {
		return nil
	}
	return &ws.nodeIDs
}

func (ws *crossMinWorkspace) graphIDSlab() *slab[layered.LGraphID] {
	if ws == nil {
		return nil
	}
	return &ws.graphIDs
}

func (ws *crossMinWorkspace) nodeListSlab() *slab[[]layered.LNodeID] {
	if ws == nil {
		return nil
	}
	return &ws.nodeLists
}

func (ws *crossMinWorkspace) intListSlab() *slab[[]int] {
	if ws == nil {
		return nil
	}
	return &ws.intLists
}

func (ws *crossMinWorkspace) stateSlab() *slab[BarycenterState] {
	if ws == nil {
		return nil
	}
	return &ws.states
}

func (ws *crossMinWorkspace) stateListSlab() *slab[[]BarycenterState] {
	if ws == nil {
		return nil
	}
	return &ws.stateLists
}

func (ws *crossMinWorkspace) infoSlab() *slab[layerSweepNodeInfo] {
	if ws == nil {
		return nil
	}
	return &ws.infos
}

func (ws *crossMinWorkspace) infoListSlab() *slab[[]layerSweepNodeInfo] {
	if ws == nil {
		return nil
	}
	return &ws.infoLists
}

func (ws *crossMinWorkspace) constraintSlab() *slab[layered.PortConstraints] {
	if ws == nil {
		return nil
	}
	return &ws.constraints
}
