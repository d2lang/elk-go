// Copyright (c) 2010, 2015 Kiel University and others.
// Copyright (c) 2012, 2015 Kiel University and others.
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

type PortDistributorKind uint8

const (
	NodeRelativePortDistributor PortDistributorKind = iota
	LayerTotalPortDistributor
)

type BarycenterPortDistributor struct {
	Kind              PortDistributorKind
	PortRanks         []float32
	minimumBarycenter float32
	maximumBarycenter float32
	nodePositions     [][]int
	portBarycenter    []float32
	nPorts            int

	// own holds the distributor's buffers unless scratch points to shared
	// ones.
	own     distributorScratch
	scratch *distributorScratch

	// ranked records the layer whose port ranks the barycenter heuristic
	// computed last, until anything could change them: the sweep then asks
	// for the same ranks again (see DistributePortsWhileSweeping).
	ranked     []layered.LNodeID
	rankedType layered.PortType
	rankedSet  bool

	// With an index (set by the holder), the layers' position arrays are cut
	// in layer order from positionBacking, so a node of this distributor's
	// holder has its position at positionBacking[slot] (see sweepIndex), and
	// its port constraints are read once into constraints[slot] (forgotten
	// by forgetPortConstraints, which every write during the sweeps calls).
	index           *sweepIndex
	owner           int32
	positionBacking []int
	positionUsed    int
	constraints     []layered.PortConstraints
	constraintsRead []bool

	table *portTable // ports and edges, if the minimizer built a table

	// skipTrivial allows distributePorts to skip the barycenters of a node
	// without nested graph whose ports, at most one per side, are sorted by
	// side; the minimizer sets it when no node can come to list another
	// node's port (see portsStayWithTheirNodes).
	skipTrivial bool

	// nodes are the holder's sweepNodes by slot (set with the table).
	nodes []sweepNode
}

// distributorScratch is a distributor's working storage: sidePorts is
// distributePorts' reused NodePortsOnSide buffer, inLayerPorts the ports
// whose barycenters depend on in-layer edges.
type distributorScratch struct {
	sidePorts    []layered.LPortID
	inLayerPorts []layered.LPortID
	southPorts   []layered.LPortID // the one-pass collection's south list
	northPorts   []layered.LPortID // and north list
	freeSlots    []int32           // the slots of the sweep step's layers
	fixedSlots   []int32
}

func (distributor *BarycenterPortDistributor) buffers() *distributorScratch {
	if distributor.scratch != nil {
		return distributor.scratch
	}
	return &distributor.own
}

// useIndex makes the distributor look nodes of holder owner up through
// index; totalNodes is the number of nodes of the order it is made for.
func (distributor *BarycenterPortDistributor) useIndex(
	ws *crossMinWorkspace,
	index *sweepIndex,
	owner int32,
	totalNodes int,
) {
	if index == nil {
		return
	}
	distributor.index, distributor.owner = index, owner
	distributor.positionBacking = ws.intSlab().take(totalNodes)
	distributor.constraints = ws.constraintSlab().take(totalNodes)
	distributor.constraintsRead = ws.boolSlab().take(totalNodes)
}

// portConstraints is Get(node, PortConstraintsOption). The property's
// default is not materialized, so the read has no side effect and caching it
// is exact as long as every write forgets the cached value.
func (distributor *BarycenterPortDistributor) portConstraints(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
) layered.PortConstraints {
	slot, owned := distributor.index.slotOf(nodeID, distributor.owner)
	if !owned {
		return basegraph.Get(&arena.Node(nodeID).Properties, layered.PortConstraintsOption)
	}
	if !distributor.constraintsRead[slot] {
		distributor.constraints[slot] = basegraph.Get(&arena.Node(nodeID).Properties, layered.PortConstraintsOption)
		distributor.constraintsRead[slot] = true
	}
	return distributor.constraints[slot]
}

// orderFixed reports whether nodeID's remembered port constraints fix its
// port order (false when none are remembered).
func (distributor *BarycenterPortDistributor) orderFixed(nodeID layered.LNodeID) bool {
	slot, owned := distributor.index.slotOf(nodeID, distributor.owner)
	return owned && distributor.constraintsRead[slot] && distributor.constraints[slot].IsOrderFixed()
}

// forgetPortConstraints drops nodeID's cached port constraints.
func (distributor *BarycenterPortDistributor) forgetPortConstraints(nodeID layered.LNodeID) {
	if slot, owned := distributor.index.slotOf(nodeID, distributor.owner); owned {
		distributor.constraintsRead[slot] = false
	}
}

// noteHeuristicRanks records that CalculatePortRanks(layer, portType) has
// just been computed by the barycenter heuristic.
func (distributor *BarycenterPortDistributor) noteHeuristicRanks(layer []layered.LNodeID, portType layered.PortType) {
	distributor.ranked, distributor.rankedType, distributor.rankedSet = layer, portType, true
}

// takeHeuristicRanks reports whether layer's ranks for portType are the ones
// the heuristic computed last (same layer slice, same type), and forgets the
// record.
func (distributor *BarycenterPortDistributor) takeHeuristicRanks(layer []layered.LNodeID, portType layered.PortType) bool {
	same := distributor.rankedSet && distributor.rankedType == portType &&
		len(distributor.ranked) == len(layer) && len(layer) > 0 && &distributor.ranked[0] == &layer[0]
	distributor.ranked, distributor.rankedSet = nil, false
	return same
}

func NewBarycenterPortDistributor(kind PortDistributorKind, numberOfLayers int) *BarycenterPortDistributor {
	return newBarycenterPortDistributor(nil, kind, numberOfLayers)
}

// newBarycenterPortDistributor is NewBarycenterPortDistributor with its
// storage taken from ws (nil: allocated) and, with a workspace, the shared
// buffers.
func newBarycenterPortDistributor(
	ws *crossMinWorkspace,
	kind PortDistributorKind,
	numberOfLayers int,
) *BarycenterPortDistributor {
	distributor := ws.distributorSlab().one()
	distributor.Kind = kind
	distributor.nodePositions = ws.intListSlab().take(numberOfLayers)
	if ws != nil {
		distributor.scratch = &ws.distributorScratch
	}
	return distributor
}

func (distributor *BarycenterPortDistributor) DistributePortsWhileSweeping(
	arena *layered.LGraphArena,
	nodeOrder [][]layered.LNodeID,
	currentIndex int,
	forward bool,
) bool {
	if distributor.nodes != nil && distributor.distributeWhileSweepingBySlot(arena, nodeOrder, currentIndex, forward) {
		return false
	}
	distributor.updateNodePositions(arena, nodeOrder[currentIndex])
	// Neither port-rank calculation nor port distribution changes the node
	// order, so the layers are read in place (ELK iterates copies).
	freeLayer := nodeOrder[currentIndex]
	side := layered.EastSide
	if forward {
		side = layered.WestSide
	}
	if !isFirstSweepLayer(len(nodeOrder), currentIndex, forward) {
		fixedIndex := currentIndex + 1
		if forward {
			fixedIndex = currentIndex - 1
		}
		fixedLayer := nodeOrder[fixedIndex]
		// When the barycenter heuristic has just ranked this very layer for
		// this sweep direction, nothing has changed its ports since (it only
		// reorders the free layer), so recomputing would write the same
		// ranks.
		if !distributor.takeHeuristicRanks(fixedLayer, portTypeForSweep(forward)) {
			distributor.CalculatePortRanks(arena, fixedLayer, portTypeForSweep(forward))
		}
		// takeHeuristicRanks has reset the heuristic-ranks record and nothing
		// below sets it, so for a node whose remembered port constraints fix
		// its port order distributePorts would only reset it again and
		// return: such calls are skipped.
		for _, nodeID := range freeLayer {
			if !distributor.orderFixed(nodeID) {
				distributor.distributePorts(arena, nodeID, side)
			}
		}
		distributor.CalculatePortRanks(arena, freeLayer, portTypeForSweep(!forward))
		for _, nodeID := range fixedLayer {
			if arena.Node(nodeID).NestedGraph == nil && !distributor.orderFixed(nodeID) {
				distributor.distributePorts(arena, nodeID, side.Opposed())
			}
		}
	} else {
		for _, nodeID := range freeLayer {
			if distributor.orderFixed(nodeID) {
				// distributePorts would only reset the heuristic-ranks
				// record and return.
				distributor.ranked, distributor.rankedSet = nil, false
				continue
			}
			distributor.distributePorts(arena, nodeID, side)
		}
	}
	return false
}

// distributeWhileSweepingBySlot is DistributePortsWhileSweeping when every
// node of the free layer and of the fixed layer belongs to the distributor's
// holder (it reports false, having done nothing, otherwise): each node's
// position, port constraints and sweepNode are read by slot, looked up once
// per layer. The calls, reads and writes are the original ones, in the same
// order.
func (distributor *BarycenterPortDistributor) distributeWhileSweepingBySlot(
	arena *layered.LGraphArena,
	nodeOrder [][]layered.LNodeID,
	currentIndex int,
	forward bool,
) bool {
	buffers := distributor.buffers()
	freeLayer := nodeOrder[currentIndex]
	freeSlots, ok := distributor.index.slotsOf(freeLayer, distributor.owner, buffers.freeSlots[:0])
	buffers.freeSlots = freeSlots
	if !ok {
		return false
	}
	first := isFirstSweepLayer(len(nodeOrder), currentIndex, forward)
	var fixedLayer []layered.LNodeID
	var fixedSlots []int32
	if !first {
		fixedIndex := currentIndex + 1
		if forward {
			fixedIndex = currentIndex - 1
		}
		fixedLayer = nodeOrder[fixedIndex]
		fixedSlots, ok = distributor.index.slotsOf(fixedLayer, distributor.owner, buffers.fixedSlots[:0])
		buffers.fixedSlots = fixedSlots
		if !ok {
			return false
		}
	}
	positions := distributor.positionBacking
	for position, slot := range freeSlots {
		positions[slot] = position
	}
	side := layered.EastSide
	if forward {
		side = layered.WestSide
	}
	if first {
		for index, slot := range freeSlots {
			if distributor.constraintsRead[slot] && distributor.constraints[slot].IsOrderFixed() {
				// distributePorts would only reset the heuristic-ranks
				// record and return.
				distributor.ranked, distributor.rankedSet = nil, false
				continue
			}
			distributor.distributePortsBySlot(arena, freeLayer[index], slot, side)
		}
		return true
	}
	if !distributor.takeHeuristicRanks(fixedLayer, portTypeForSweep(forward)) {
		distributor.calculatePortRanksBySlot(fixedSlots, portTypeForSweep(forward))
	}
	// See DistributePortsWhileSweeping for the skipped calls.
	for index, slot := range freeSlots {
		if !(distributor.constraintsRead[slot] && distributor.constraints[slot].IsOrderFixed()) {
			distributor.distributePortsBySlot(arena, freeLayer[index], slot, side)
		}
	}
	distributor.calculatePortRanksBySlot(freeSlots, portTypeForSweep(!forward))
	opposed := side.Opposed()
	for index, slot := range fixedSlots {
		if !distributor.nodes[slot].nested &&
			!(distributor.constraintsRead[slot] && distributor.constraints[slot].IsOrderFixed()) {
			distributor.distributePortsBySlot(arena, fixedLayer[index], slot, opposed)
		}
	}
	return true
}

// distributePortsBySlot is distributePorts(nodeID) for the holder's node in
// slot, with a table.
func (distributor *BarycenterPortDistributor) distributePortsBySlot(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	slot int32,
	side layered.PortSide,
) {
	distributor.ranked, distributor.rankedSet = nil, false
	if !distributor.constraintsRead[slot] {
		distributor.constraints[slot] = basegraph.Get(&arena.Node(nodeID).Properties, layered.PortConstraintsOption)
		distributor.constraintsRead[slot] = true
	}
	if distributor.constraints[slot].IsOrderFixed() {
		return
	}
	table := distributor.table
	if node := &distributor.nodes[slot]; distributor.skipTrivial &&
		(side == layered.EastSide || side == layered.WestSide) {
		// A node without nested graph keeps its ports and their sides, and
		// only its own distributions change its list, which keep it sorted
		// by side; so once distributeBlocksFromTable has served it, its
		// counts per side hold for every later call. A node with at most one
		// port per side is never reordered: nothing is recomputed or
		// sorted, and (without a change of the order) the port side cache
		// needs no invalidation (it only drops ranges that NodePortSideView
		// recomputes before reading).
		switch node.blocks {
		case blocksTrivial:
			return
		case blocksCounted:
			distributor.distributeCountedBlocks(arena, table, nodeID, node.ports, side, &node.counts)
			return
		}
		if distributor.distributeBlocksFromTable(arena, table, nodeID, node.ports, side, node.nested) {
			if !node.nested {
				node.rememberBlocks(table)
			}
			return
		}
	}
	distributor.distributeSidesAndSortFromTable(arena, table, nodeID, side, distributor.buffers())
}

// rememberBlocks records the node's counts per side (its list, just served
// by distributeBlocksFromTable, is sorted by side).
func (node *sweepNode) rememberBlocks(table *portTable) {
	var counts [layered.WestSide + 1]int
	for _, portID := range node.ports {
		counts[table.ports[portID].side]++
	}
	trivial := true
	for side, count := range counts {
		if count > 255 {
			return
		}
		node.counts[side] = uint8(count)
		if count > 1 {
			trivial = false
		}
	}
	node.blocks = blocksCounted
	if trivial {
		node.blocks = blocksTrivial
	}
}

// distributeCountedBlocks is distributeBlocksFromTable for a node without
// nested graph whose list is sorted by side with the given counts per side.
func (distributor *BarycenterPortDistributor) distributeCountedBlocks(
	arena *layered.LGraphArena,
	table *portTable,
	nodeID layered.LNodeID,
	ports []layered.LPortID,
	side layered.PortSide,
	counts *[layered.WestSide + 1]uint8,
) {
	var starts [layered.WestSide + 1]int
	for portSide := layered.NorthSide; portSide <= layered.WestSide; portSide++ {
		starts[portSide] = starts[portSide-1] + int(counts[portSide-1])
	}
	blocks := [3]layered.PortSide{side, layered.SouthSide, layered.NorthSide}
	recomputed := false
	for _, blockSide := range blocks {
		if counts[blockSide] > 1 {
			distributor.distributePortList(arena, nodeID, ports[starts[blockSide]:starts[blockSide]+int(counts[blockSide])])
			recomputed = true
		}
	}
	if !recomputed {
		return
	}
	moved := false
	for _, blockSide := range blocks {
		if counts[blockSide] > 1 {
			moved = distributor.sortBlock(table, ports[starts[blockSide]:starts[blockSide]+int(counts[blockSide])]) || moved
		}
	}
	if moved {
		arena.InvalidatePortSideCache(nodeID)
	}
}

// calculatePortRanksOf is CalculatePortRanks(layer, portType), by slot when
// every node of layer is the holder's.
func (distributor *BarycenterPortDistributor) calculatePortRanksOf(
	arena *layered.LGraphArena,
	layer []layered.LNodeID,
	portType layered.PortType,
) {
	if distributor.nodes != nil {
		buffers := distributor.buffers()
		slots, ok := distributor.index.slotsOf(layer, distributor.owner, buffers.fixedSlots[:0])
		buffers.fixedSlots = slots
		if ok {
			distributor.calculatePortRanksBySlot(slots, portType)
			return
		}
	}
	distributor.CalculatePortRanks(arena, layer, portType)
}

// calculatePortRanksBySlot is CalculatePortRanks for the holder's nodes in
// slots, with a table.
func (distributor *BarycenterPortDistributor) calculatePortRanksBySlot(slots []int32, portType layered.PortType) {
	var consumed float32
	nodes := distributor.nodes
	for _, slot := range slots {
		consumed += distributor.calculateNodePortRanksFromTable(distributor.table, nodes[slot].ports, consumed, portType)
	}
}

func (distributor *BarycenterPortDistributor) CalculatePortRanks(
	arena *layered.LGraphArena,
	layer []layered.LNodeID,
	portType layered.PortType,
) {
	var consumed float32
	for _, nodeID := range layer {
		consumed += distributor.calculateNodePortRanks(arena, nodeID, consumed, portType)
	}
}

func (distributor *BarycenterPortDistributor) calculateNodePortRanks(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	rankSum float32,
	portType layered.PortType,
) float32 {
	ports := arena.Node(nodeID).Ports
	if table := distributor.table; table != nil {
		return distributor.calculateNodePortRanksFromTable(table, ports, rankSum, portType)
	}
	switch distributor.Kind {
	case NodeRelativePortDistributor:
		switch portType {
		case layered.PortTypeInput:
			inputCount, northInputCount := 0, 0
			for _, portID := range ports {
				if len(arena.Port(portID).IncomingEdges) > 0 {
					inputCount++
					if arena.Port(portID).Side == layered.NorthSide {
						northInputCount++
					}
				}
			}
			increment := float32(1) / float32(inputCount+1)
			northPosition := rankSum + float32(northInputCount)*increment
			restPosition := rankSum + 1 - increment
			for _, portID := range ports {
				if len(arena.Port(portID).IncomingEdges) == 0 {
					continue
				}
				if arena.Port(portID).Side == layered.NorthSide {
					distributor.PortRanks[arena.Port(portID).ID] = northPosition
					northPosition -= increment
				} else {
					distributor.PortRanks[arena.Port(portID).ID] = restPosition
					restPosition -= increment
				}
			}
		case layered.PortTypeOutput:
			outputCount := 0
			for _, portID := range ports {
				if len(arena.Port(portID).OutgoingEdges) > 0 {
					outputCount++
				}
			}
			increment := float32(1) / float32(outputCount+1)
			position := rankSum + increment
			for _, portID := range ports {
				if len(arena.Port(portID).OutgoingEdges) == 0 {
					continue
				}
				distributor.PortRanks[arena.Port(portID).ID] = position
				position += increment
			}
		default:
			panic("p3order: undefined port type")
		}
		return 1
	case LayerTotalPortDistributor:
		switch portType {
		case layered.PortTypeInput:
			inputCount, northInputCount := 0, 0
			for _, portID := range ports {
				if len(arena.Port(portID).IncomingEdges) > 0 {
					inputCount++
					if arena.Port(portID).Side == layered.NorthSide {
						northInputCount++
					}
				}
			}
			northPosition := rankSum + float32(northInputCount)
			restPosition := rankSum + float32(inputCount)
			for _, portID := range ports {
				if len(arena.Port(portID).IncomingEdges) == 0 {
					continue
				}
				if arena.Port(portID).Side == layered.NorthSide {
					distributor.PortRanks[arena.Port(portID).ID] = northPosition
					northPosition--
				} else {
					distributor.PortRanks[arena.Port(portID).ID] = restPosition
					restPosition--
				}
			}
			return float32(inputCount)
		case layered.PortTypeOutput:
			position := 0
			for _, portID := range ports {
				if len(arena.Port(portID).OutgoingEdges) == 0 {
					continue
				}
				position++
				distributor.PortRanks[arena.Port(portID).ID] = rankSum + float32(position)
			}
			return float32(position)
		default:
			panic("p3order: undefined port type")
		}
	default:
		panic("p3order: unknown port distributor")
	}
}

// calculateNodePortRanksFromTable is calculateNodePortRanks with each port's
// edge counts, side and ID read from the table (the table's entries and the
// ranks held in locals: the same reads and writes in the same order).
func (distributor *BarycenterPortDistributor) calculateNodePortRanksFromTable(
	table *portTable,
	ports []layered.LPortID,
	rankSum float32,
	portType layered.PortType,
) float32 {
	entries := table.ports
	ranks := distributor.PortRanks
	switch distributor.Kind {
	case NodeRelativePortDistributor:
		switch portType {
		case layered.PortTypeInput:
			inputCount, northInputCount := 0, 0
			for _, portID := range ports {
				if entry := &entries[portID]; entry.in > 0 {
					inputCount++
					if entry.side == layered.NorthSide {
						northInputCount++
					}
				}
			}
			increment := float32(1) / float32(inputCount+1)
			northPosition := rankSum + float32(northInputCount)*increment
			restPosition := rankSum + 1 - increment
			for _, portID := range ports {
				entry := &entries[portID]
				if entry.in == 0 {
					continue
				}
				if entry.side == layered.NorthSide {
					ranks[entry.id] = northPosition
					northPosition -= increment
				} else {
					ranks[entry.id] = restPosition
					restPosition -= increment
				}
			}
		case layered.PortTypeOutput:
			outputCount := 0
			for _, portID := range ports {
				if entries[portID].out > 0 {
					outputCount++
				}
			}
			increment := float32(1) / float32(outputCount+1)
			position := rankSum + increment
			for _, portID := range ports {
				entry := &entries[portID]
				if entry.out == 0 {
					continue
				}
				ranks[entry.id] = position
				position += increment
			}
		default:
			panic("p3order: undefined port type")
		}
		return 1
	case LayerTotalPortDistributor:
		switch portType {
		case layered.PortTypeInput:
			inputCount, northInputCount := 0, 0
			for _, portID := range ports {
				if entry := &entries[portID]; entry.in > 0 {
					inputCount++
					if entry.side == layered.NorthSide {
						northInputCount++
					}
				}
			}
			northPosition := rankSum + float32(northInputCount)
			restPosition := rankSum + float32(inputCount)
			for _, portID := range ports {
				entry := &entries[portID]
				if entry.in == 0 {
					continue
				}
				if entry.side == layered.NorthSide {
					ranks[entry.id] = northPosition
					northPosition--
				} else {
					ranks[entry.id] = restPosition
					restPosition--
				}
			}
			return float32(inputCount)
		case layered.PortTypeOutput:
			position := 0
			for _, portID := range ports {
				entry := &entries[portID]
				if entry.out == 0 {
					continue
				}
				position++
				ranks[entry.id] = rankSum + float32(position)
			}
			return float32(position)
		default:
			panic("p3order: undefined port type")
		}
	default:
		panic("p3order: unknown port distributor")
	}
}

func (distributor *BarycenterPortDistributor) distributePorts(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	side layered.PortSide,
) {
	distributor.ranked, distributor.rankedSet = nil, false
	constraints := distributor.portConstraints(arena, nodeID)
	if constraints.IsOrderFixed() {
		return
	}
	buffers := distributor.buffers()
	if table := distributor.table; table != nil {
		distributor.distributePortsFromTable(arena, table, nodeID, side, buffers)
		return
	}
	for _, portSide := range [3]layered.PortSide{side, layered.SouthSide, layered.NorthSide} {
		// arena.NodePortsOnSide into a reused buffer.
		sidePorts := buffers.sidePorts[:0]
		for _, portID := range arena.Node(nodeID).Ports {
			if arena.Port(portID).Side == portSide {
				sidePorts = append(sidePorts, portID)
			}
		}
		buffers.sidePorts = sidePorts
		distributor.distributePortList(arena, nodeID, sidePorts)
	}
	ports := arena.Node(nodeID).Ports
	javacompat.GoStableSort(ports, func(firstID, secondID layered.LPortID) bool {
		first, second := arena.Port(firstID), arena.Port(secondID)
		if first.Side != second.Side {
			return first.Side < second.Side
		}
		firstBarycenter := distributor.portBarycenter[first.ID]
		secondBarycenter := distributor.portBarycenter[second.ID]
		if firstBarycenter == 0 && secondBarycenter == 0 {
			return false
		}
		if firstBarycenter == 0 {
			return true
		}
		if secondBarycenter == 0 {
			return false
		}
		return firstBarycenter < secondBarycenter
	})
	arena.Node(nodeID).Ports = ports
	arena.InvalidatePortSideCache(nodeID)
}

// distributePortsFromTable is distributePorts after the port constraints
// check, with sides and IDs read from the table.
func (distributor *BarycenterPortDistributor) distributePortsFromTable(
	arena *layered.LGraphArena,
	table *portTable,
	nodeID layered.LNodeID,
	side layered.PortSide,
	buffers *distributorScratch,
) {
	if distributor.skipTrivial && (side == layered.EastSide || side == layered.WestSide) {
		if node := arena.Node(nodeID); distributor.distributeBlocksFromTable(
			arena, table, nodeID, node.Ports, side, node.NestedGraph != nil) {
			return
		}
	}
	distributor.distributeSidesAndSortFromTable(arena, table, nodeID, side, buffers)
}

// distributeSidesAndSortFromTable is distributePortsFromTable without the
// block shortcut.
func (distributor *BarycenterPortDistributor) distributeSidesAndSortFromTable(
	arena *layered.LGraphArena,
	table *portTable,
	nodeID layered.LNodeID,
	side layered.PortSide,
	buffers *distributorScratch,
) {
	if side != layered.EastSide && side != layered.WestSide {
		for _, portSide := range [3]layered.PortSide{side, layered.SouthSide, layered.NorthSide} {
			// arena.NodePortsOnSide into a reused buffer.
			sidePorts := buffers.sidePorts[:0]
			for _, portID := range arena.Node(nodeID).Ports {
				if table.ports[portID].side == portSide {
					sidePorts = append(sidePorts, portID)
				}
			}
			buffers.sidePorts = sidePorts
			distributor.distributePortList(arena, nodeID, sidePorts)
		}
	} else if !distributor.distributeSidesFromTable(arena, table, nodeID, side, buffers) {
		// Every side has at most one port and the list is sorted by side:
		// the sort below would only compare sides and keep the order, so no
		// barycenter computed here would ever be read (the node has no nested
		// graph, so no other code changes which ports it lists). Only the
		// cache invalidation of the end remains.
		arena.InvalidatePortSideCache(nodeID)
		return
	}
	ports := arena.Node(nodeID).Ports
	portBarycenter := distributor.portBarycenter
	javacompat.GoStableSort(ports, func(firstID, secondID layered.LPortID) bool {
		first, second := &table.ports[firstID], &table.ports[secondID]
		if first.side != second.side {
			return first.side < second.side
		}
		firstBarycenter := portBarycenter[first.id]
		secondBarycenter := portBarycenter[second.id]
		if firstBarycenter == 0 && secondBarycenter == 0 {
			return false
		}
		if firstBarycenter == 0 {
			return true
		}
		if secondBarycenter == 0 {
			return false
		}
		return firstBarycenter < secondBarycenter
	})
	arena.Node(nodeID).Ports = ports
	arena.InvalidatePortSideCache(nodeID)
}

// distributeSidesFromTable is distributePorts' loop over side (east or
// west), south and north: the three NodePortsOnSide lists are collected in
// one pass (distributing one list changes neither the port list nor any
// side, so the later lists are the same when collected first). It reports
// false, having distributed nothing, when the node has no nested graph and
// its ports, at most one per side, are sorted by side.
func (distributor *BarycenterPortDistributor) distributeSidesFromTable(
	arena *layered.LGraphArena,
	table *portTable,
	nodeID layered.LNodeID,
	side layered.PortSide,
	buffers *distributorScratch,
) bool {
	node := arena.Node(nodeID)
	sidePorts, south, north := buffers.sidePorts[:0], buffers.southPorts[:0], buffers.northPorts[:0]
	var seen [layered.WestSide + 1]bool
	trivial := node.NestedGraph == nil
	previous := layered.UndefinedSide
	for _, portID := range node.Ports {
		portSide := table.ports[portID].side
		switch portSide {
		case side:
			sidePorts = append(sidePorts, portID)
		case layered.SouthSide:
			south = append(south, portID)
		case layered.NorthSide:
			north = append(north, portID)
		}
		if trivial {
			if portSide > layered.WestSide || portSide < previous || seen[portSide] {
				trivial = false
			} else {
				seen[portSide] = true
				previous = portSide
			}
		}
	}
	buffers.sidePorts, buffers.southPorts, buffers.northPorts = sidePorts, south, north
	if trivial && distributor.skipTrivial {
		return false
	}
	distributor.distributePortList(arena, nodeID, sidePorts)
	distributor.distributePortList(arena, nodeID, south)
	distributor.distributePortList(arena, nodeID, north)
	return true
}

// distributeBlocksFromTable is distributePorts after the port constraints
// check (side east or west) for a node whose port list ports
// (arena.Node(nodeID).Ports) is sorted by side, called only when no node can
// come to list another node's port (skipTrivial); nested tells whether the
// node has a nested graph. It reports false, having done nothing, for a list
// not sorted by side.
//
// The comparator is a strict weak order (sides, then barycenters, which are
// never NaN, zero first), so the stable sort of a list sorted by side has a
// unique result: each side's block (contiguous) stably sorted on its own.
// Only the blocks whose barycenters this call recomputes (the side's, the
// south and the north ports) can be out of order for a node without nested
// graph: only this distributor's sorts change such a node's list
// (sortPortsByDummyPositionsInLastLayer writes parents' lists only), only
// calls for the node write its ports' barycenters, every call that writes
// them ends with a sort, and the barycenters start at zero, which the
// comparator treats as equal. A block of one port of such a node is neither
// recomputed nor sorted: its ports stay on their sides, so the barycenter of
// a port alone on its side is never compared (and the barycenter
// calculations have no other effect). A parent's list can also be rewritten
// by sortPortsByDummyPositionsInLastLayer, so for a parent every list is
// recomputed, as before, and every block is sorted.
func (distributor *BarycenterPortDistributor) distributeBlocksFromTable(
	arena *layered.LGraphArena,
	table *portTable,
	nodeID layered.LNodeID,
	ports []layered.LPortID,
	side layered.PortSide,
	nested bool,
) bool {
	var counts [layered.WestSide + 1]int
	previous := layered.UndefinedSide
	for _, portID := range ports {
		portSide := table.ports[portID].side
		if portSide < previous || portSide > layered.WestSide {
			return false
		}
		counts[portSide]++
		previous = portSide
	}
	var starts [layered.WestSide + 1]int
	for portSide := layered.NorthSide; portSide <= layered.WestSide; portSide++ {
		starts[portSide] = starts[portSide-1] + counts[portSide-1]
	}
	blocks := [3]layered.PortSide{side, layered.SouthSide, layered.NorthSide}
	smallest := 2 // the smallest block that is recomputed
	if nested {
		smallest = 1
	}
	recomputed := false
	for _, blockSide := range blocks {
		if counts[blockSide] >= smallest {
			distributor.distributePortList(arena, nodeID, ports[starts[blockSide]:starts[blockSide]+counts[blockSide]])
			recomputed = true
		}
	}
	if nested {
		for blockSide := layered.UndefinedSide; blockSide <= layered.WestSide; blockSide++ {
			if counts[blockSide] > 1 {
				distributor.sortBlock(table, ports[starts[blockSide]:starts[blockSide]+counts[blockSide]])
			}
		}
	} else if recomputed {
		for _, blockSide := range blocks {
			if counts[blockSide] > 1 {
				distributor.sortBlock(table, ports[starts[blockSide]:starts[blockSide]+counts[blockSide]])
			}
		}
	}
	arena.InvalidatePortSideCache(nodeID)
	return true
}

// sortBlock stably sorts ports, all on one side, by distributePorts'
// comparator (insertion sort; GoStableSort for long blocks). It reports
// whether the order may have changed.
func (distributor *BarycenterPortDistributor) sortBlock(table *portTable, ports []layered.LPortID) bool {
	portBarycenter := distributor.portBarycenter
	if len(ports) > 24 {
		javacompat.GoStableSort(ports, func(firstID, secondID layered.LPortID) bool {
			return barycenterBefore(portBarycenter[table.ports[firstID].id], portBarycenter[table.ports[secondID].id])
		})
		return true
	}
	moved := false
	for index := 1; index < len(ports); index++ {
		portID := ports[index]
		key := portBarycenter[table.ports[portID].id]
		position := index
		for position > 0 {
			before := ports[position-1]
			if !barycenterBefore(key, portBarycenter[table.ports[before].id]) {
				break
			}
			ports[position] = before
			position--
		}
		if position != index {
			ports[position] = portID
			moved = true
		}
	}
	return moved
}

// barycenterBefore is distributePorts' comparator for two ports on the same
// side: a zero barycenter comes first, others by value.
func barycenterBefore(first, second float32) bool {
	if first == 0 && second == 0 {
		return false
	}
	if first == 0 {
		return true
	}
	if second == 0 {
		return false
	}
	return first < second
}

func (distributor *BarycenterPortDistributor) distributePortList(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	ports []layered.LPortID,
) {
	buffers := distributor.buffers()
	buffers.inLayerPorts = buffers.inLayerPorts[:0]
	distributor.collectPortBarycenters(arena, nodeID, ports)
	if len(buffers.inLayerPorts) > 0 {
		distributor.calculateInLayerPortBarycenters(arena, nodeID)
	}
}

func (distributor *BarycenterPortDistributor) collectPortBarycenters(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	ports []layered.LPortID,
) {
	distributor.minimumBarycenter = 0
	distributor.maximumBarycenter = 0
	absurdlyLarge := float32(2*len(arena.Layer(*arena.Node(nodeID).Layer).Nodes) + 1)
	buffers := distributor.buffers()
	if table := distributor.table; table != nil {
		distributor.collectPortBarycentersFromTable(arena, table, nodeID, ports, absurdlyLarge, buffers)
		return
	}
	for _, portID := range ports {
		port := arena.Port(portID)
		northSouth := port.Side == layered.NorthSide || port.Side == layered.SouthSide
		var sum float32
		if northSouth {
			dummyID, found := basegraph.TryGet(&port.Properties, layered.PortDummy)
			if !found {
				continue
			}
			sum += distributor.northSouthPortValue(arena, absurdlyLarge, portID, dummyID)
		} else {
			inLayer := false
			for _, edgeID := range port.OutgoingEdges {
				connected := *arena.Edge(edgeID).Target
				if sameLayer(arena.Node(*arena.Port(connected).Node).Layer, arena.Node(nodeID).Layer) {
					buffers.inLayerPorts = append(buffers.inLayerPorts, portID)
					inLayer = true
					break
				}
				sum += distributor.PortRanks[arena.Port(connected).ID]
			}
			if inLayer {
				continue
			}
			for _, edgeID := range port.IncomingEdges {
				connected := *arena.Edge(edgeID).Source
				if sameLayer(arena.Node(*arena.Port(connected).Node).Layer, arena.Node(nodeID).Layer) {
					buffers.inLayerPorts = append(buffers.inLayerPorts, portID)
					inLayer = true
					break
				}
				sum -= distributor.PortRanks[arena.Port(connected).ID]
			}
			if inLayer {
				continue
			}
		}
		degree := portDegree(arena, portID)
		if degree > 0 {
			barycenter := sum / float32(degree)
			distributor.portBarycenter[port.ID] = barycenter
			if barycenter < distributor.minimumBarycenter {
				distributor.minimumBarycenter = barycenter
			}
			if barycenter > distributor.maximumBarycenter {
				distributor.maximumBarycenter = barycenter
			}
		} else if northSouth {
			distributor.portBarycenter[port.ID] = sum
		}
	}
}

// collectPortBarycentersFromTable is collectPortBarycenters with east and
// west ports read from the table: an edge end's connected port is in the
// node's layer when end.otherLayer is the node's layer key, its rank is at
// end.otherID, and the port's degree is its edge count.
func (distributor *BarycenterPortDistributor) collectPortBarycentersFromTable(
	arena *layered.LGraphArena,
	table *portTable,
	nodeID layered.LNodeID,
	ports []layered.LPortID,
	absurdlyLarge float32,
	buffers *distributorScratch,
) {
	nodeLayer := layerKey(arena.Node(nodeID).Layer)
	ranks := distributor.PortRanks
	for _, portID := range ports {
		entry := &table.ports[portID]
		northSouth := entry.side == layered.NorthSide || entry.side == layered.SouthSide
		var sum float32
		if northSouth {
			dummyID, found := basegraph.TryGet(&arena.Port(portID).Properties, layered.PortDummy)
			if !found {
				continue
			}
			sum += distributor.northSouthPortValue(arena, absurdlyLarge, portID, dummyID)
		} else {
			inLayer := false
			for _, end := range table.outgoing(entry) {
				if end.otherLayer == nodeLayer {
					buffers.inLayerPorts = append(buffers.inLayerPorts, portID)
					inLayer = true
					break
				}
				sum += ranks[end.otherID]
			}
			if inLayer {
				continue
			}
			for _, end := range table.incoming(entry) {
				if end.otherLayer == nodeLayer {
					buffers.inLayerPorts = append(buffers.inLayerPorts, portID)
					inLayer = true
					break
				}
				sum -= ranks[end.otherID]
			}
			if inLayer {
				continue
			}
		}
		degree := int(entry.in + entry.out)
		if degree > 0 {
			barycenter := sum / float32(degree)
			distributor.portBarycenter[entry.id] = barycenter
			if barycenter < distributor.minimumBarycenter {
				distributor.minimumBarycenter = barycenter
			}
			if barycenter > distributor.maximumBarycenter {
				distributor.maximumBarycenter = barycenter
			}
		} else if northSouth {
			distributor.portBarycenter[entry.id] = sum
		}
	}
}

func (distributor *BarycenterPortDistributor) calculateInLayerPortBarycenters(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
) {
	nodePosition := distributor.positionOf(arena, nodeID) + 1
	layerSize := len(arena.Layer(*arena.Node(nodeID).Layer).Nodes) + 1
	for _, portID := range distributor.buffers().inLayerPorts {
		sum, connections := 0, 0
		port := arena.Port(portID)
		for _, edges := range [2][]layered.LEdgeID{port.IncomingEdges, port.OutgoingEdges} {
			for _, edgeID := range edges {
				connected := arena.EdgeOtherPort(edgeID, portID)
				connectedNode := *arena.Port(connected).Node
				if sameLayer(arena.Node(connectedNode).Layer, arena.Node(nodeID).Layer) {
					sum += distributor.positionOf(arena, connectedNode) + 1
					connections++
				}
			}
		}
		barycenter := float32(sum) / float32(connections)
		if port.Side == layered.EastSide {
			if barycenter < float32(nodePosition) {
				distributor.portBarycenter[port.ID] = distributor.minimumBarycenter - barycenter
			} else {
				distributor.portBarycenter[port.ID] = distributor.maximumBarycenter + float32(layerSize) - barycenter
			}
		} else if port.Side == layered.WestSide {
			if barycenter < float32(nodePosition) {
				distributor.portBarycenter[port.ID] = distributor.maximumBarycenter + barycenter
			} else {
				distributor.portBarycenter[port.ID] = distributor.minimumBarycenter - (float32(layerSize) - barycenter)
			}
		}
	}
}

func (distributor *BarycenterPortDistributor) northSouthPortValue(
	arena *layered.LGraphArena,
	absurdlyLarge float32,
	portID layered.LPortID,
	dummyID layered.LNodeID,
) float32 {
	input, output := false, false
	for _, dummyPortID := range arena.Node(dummyID).Ports {
		origin, found := basegraph.TryGet(&arena.Port(dummyPortID).Properties, layered.OriginalElement)
		if !found || origin.Kind != layered.OriginLPort || origin.LPort != portID {
			continue
		}
		if len(arena.Port(dummyPortID).OutgoingEdges) > 0 {
			output = true
		} else if len(arena.Port(dummyPortID).IncomingEdges) > 0 {
			input = true
		}
	}
	position := float32(distributor.positionOf(arena, dummyID))
	if input != output {
		if input {
			if arena.Port(portID).Side == layered.NorthSide {
				return -position
			}
			return absurdlyLarge - position
		}
		return position + 1
	}
	if input && output && arena.Port(portID).Side == layered.SouthSide {
		return absurdlyLarge / 2
	}
	return 0
}

func (distributor *BarycenterPortDistributor) positionOf(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
) int {
	if slot, owned := distributor.index.slotOf(nodeID, distributor.owner); owned {
		return distributor.positionBacking[slot]
	}
	return distributor.nodePositions[arena.Layer(*arena.Node(nodeID).Layer).ID][arena.Node(nodeID).ID]
}

func (distributor *BarycenterPortDistributor) updateNodePositions(
	arena *layered.LGraphArena,
	layer []layered.LNodeID,
) {
	for position, nodeID := range layer {
		if slot, owned := distributor.index.slotOf(nodeID, distributor.owner); owned {
			distributor.positionBacking[slot] = position
			continue
		}
		distributor.nodePositions[arena.Layer(*arena.Node(nodeID).Layer).ID][arena.Node(nodeID).ID] = position
	}
}

func (distributor *BarycenterPortDistributor) InitAtLayerLevel(layerIndex int, order [][]layered.LNodeID) {
	n := len(order[layerIndex])
	if start := distributor.positionUsed; distributor.index != nil && start+n <= len(distributor.positionBacking) {
		distributor.nodePositions[layerIndex] = distributor.positionBacking[start : start+n : start+n]
		distributor.positionUsed += n
		return
	}
	distributor.nodePositions[layerIndex] = make([]int, n)
}

func (distributor *BarycenterPortDistributor) InitAtNodeLevel(
	arena *layered.LGraphArena,
	layerIndex, nodeIndex int,
	order [][]layered.LNodeID,
) {
	nodeID := order[layerIndex][nodeIndex]
	arena.Node(nodeID).ID = nodeIndex
	distributor.nodePositions[layerIndex][nodeIndex] = nodeIndex
}

func (distributor *BarycenterPortDistributor) InitAtPortLevel(
	arena *layered.LGraphArena,
	layerIndex, nodeIndex, portIndex int,
	order [][]layered.LNodeID,
) {
	portID := arena.Node(order[layerIndex][nodeIndex]).Ports[portIndex]
	arena.Port(portID).ID = distributor.nPorts
	distributor.nPorts++
}

func (distributor *BarycenterPortDistributor) InitAfterTraversal() {
	distributor.initAfterTraversal(nil)
}

// initAfterTraversal is InitAfterTraversal with storage from ws (nil:
// allocated).
func (distributor *BarycenterPortDistributor) initAfterTraversal(ws *crossMinWorkspace) {
	distributor.PortRanks = ws.float32Slab().take(distributor.nPorts)
	distributor.portBarycenter = ws.float32Slab().take(distributor.nPorts)
}

func InitializeBarycenterPortDistributor(
	distributor *BarycenterPortDistributor,
	arena *layered.LGraphArena,
	order [][]layered.LNodeID,
) {
	for layerIndex, nodes := range order {
		distributor.InitAtLayerLevel(layerIndex, order)
		if len(nodes) > 0 && arena.Node(nodes[0]).Layer != nil {
			arena.Layer(*arena.Node(nodes[0]).Layer).ID = layerIndex
		}
		for nodeIndex, nodeID := range nodes {
			distributor.InitAtNodeLevel(arena, layerIndex, nodeIndex, order)
			for portIndex := range arena.Node(nodeID).Ports {
				distributor.InitAtPortLevel(arena, layerIndex, nodeIndex, portIndex, order)
			}
		}
	}
	distributor.InitAfterTraversal()
}

func portTypeForSweep(forward bool) layered.PortType {
	if forward {
		return layered.PortTypeOutput
	}
	return layered.PortTypeInput
}

func isFirstSweepLayer(length, currentIndex int, forward bool) bool {
	if forward {
		return currentIndex == 0
	}
	return currentIndex == length-1
}
