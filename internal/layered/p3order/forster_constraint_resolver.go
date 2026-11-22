// Copyright (c) 2016, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// BarycenterState is ELK's BarycenterState. Barycenter is meaningful only
// when HasBarycenter is set; an unset barycenter is Java's null Double. (ELK
// never shares or mutates a Double through an alias, so a value and a flag
// represent it exactly.)
type BarycenterState struct {
	Node          layered.LNodeID
	SummedWeight  float64
	Degree        int
	Barycenter    float64
	HasBarycenter bool
	Visited       bool

	// Node's barycenter associates and in-layer successor constraints, read
	// once. Only intermediate processors write these properties, and none
	// runs during crossing minimisation; the first read still happens where
	// ELK makes it, because it materialises the cloneable empty default into
	// the node's property map. A state can also be reached for another node
	// (a node of another graph whose Layer.ID and LNode.ID index this state,
	// as in ELK): such a node's lists are read every time, as ELK does, and
	// never cached here.
	associates      []layered.LNodeID
	associatesKnown bool
	successors      []layered.LNodeID
	successorsKnown bool
}

// barycenterAssociates is Get(node, BarycenterAssociates), read once for the
// state's own node.
func (state *BarycenterState) barycenterAssociates(arena *layered.LGraphArena, nodeID layered.LNodeID) []layered.LNodeID {
	if nodeID != state.Node {
		return basegraph.Get(&arena.Node(nodeID).Properties, layered.BarycenterAssociates)
	}
	if !state.associatesKnown {
		state.associates = basegraph.Get(&arena.Node(nodeID).Properties, layered.BarycenterAssociates)
		state.associatesKnown = true
	}
	return state.associates
}

// successorConstraints is Get(node, InLayerSuccessorConstraints), read once
// for the state's own node.
func (state *BarycenterState) successorConstraints(arena *layered.LGraphArena, nodeID layered.LNodeID) []layered.LNodeID {
	if nodeID != state.Node {
		return basegraph.Get(&arena.Node(nodeID).Properties, layered.InLayerSuccessorConstraints)
	}
	if !state.successorsKnown {
		state.successors = basegraph.Get(&arena.Node(nodeID).Properties, layered.InLayerSuccessorConstraints)
		state.successorsKnown = true
	}
	return state.successors
}

func (state *BarycenterState) setBarycenter(value float64) {
	state.Barycenter, state.HasBarycenter = value, true
}

func (state *BarycenterState) clearBarycenter() {
	state.Barycenter, state.HasBarycenter = 0, false
}

type ForsterConstraintResolver struct {
	constraintsBetweenNonDummies bool
	layoutUnits                  map[layered.LNodeID][]layered.LNodeID // made on first insertion
	BarycenterStates             [][]BarycenterState

	// The layers' states are cut, in layer order, from one array sized for
	// the order the resolver was made for; each part is handed out once.
	stateBacking []BarycenterState
	stateUsed    int

	scratch *forsterScratch

	// index and owner (set by the holder) give the state of a node of this
	// resolver's holder directly: stateBacking[slot] is
	// BarycenterStates[Layer.ID][LNode.ID] (see sweepIndex).
	index *sweepIndex
	owner int32

	// knownSuccessors counts the states whose successor constraints
	// readSuccessors has read (a state read otherwise is not counted), and
	// nonEmptySuccessors those of them with a constraint.
	knownSuccessors    int
	nonEmptySuccessors int
}

// readSuccessors is state.successorConstraints(arena, nodeID), counting a
// first read of the state's own node.
func (resolver *ForsterConstraintResolver) readSuccessors(
	state *BarycenterState,
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
) []layered.LNodeID {
	if nodeID != state.Node {
		return state.successorConstraints(arena, nodeID) // not cached, not counted
	}
	if state.successorsKnown {
		return state.successors
	}
	successors := state.successorConstraints(arena, nodeID)
	resolver.knownSuccessors++
	if len(successors) > 0 {
		resolver.nonEmptySuccessors++
	}
	return successors
}

// forsterScratch is processConstraintStage's working storage. Every call
// starts by truncating or emptying all of it, and nothing in it is referenced
// once the call has returned, so the resolvers of one crossing minimisation
// share one (calls never nest).
type forsterScratch struct {
	groups    []constraintGroup
	list      []int
	result    []layered.LNodeID
	singleton []layered.LNodeID
	groupOf   map[layered.LNodeID]int

	// groupOfLargest is the most entries groupOf has held (a map's storage
	// never shrinks).
	groupOfLargest int
}

// maxKeptGroupOfEntries bounds the group map a released workspace keeps.
const maxKeptGroupOfEntries = 1 << 10

// release prepares the scratch of a released workspace: the groups' edge
// lists are dropped (each group gets new ones when it is used again) and a
// group map that has held many entries is dropped.
func (scratch *forsterScratch) release() {
	clear(scratch.groups[:cap(scratch.groups)])
	if scratch.groupOfLargest > maxKeptGroupOfEntries {
		scratch.groupOf, scratch.groupOfLargest = nil, 0
	}
}

// retainedBytes bounds the bytes of storage scratch keeps (see
// crossMinWorkspace.retainedBytes); the groups' edge lists are dropped on
// release.
func (scratch *forsterScratch) retainedBytes() int {
	return cap(scratch.groups)*bytesConstraintGroup + cap(scratch.list)*8 +
		(cap(scratch.result)+cap(scratch.singleton))*4 + scratch.groupOfLargest*bytesMapEntry
}

func NewForsterConstraintResolver(
	arena *layered.LGraphArena,
	order [][]layered.LNodeID,
) *ForsterConstraintResolver {
	return newForsterConstraintResolver(nil, arena, order)
}

// newForsterConstraintResolver is NewForsterConstraintResolver with its
// storage taken from ws (nil: allocated).
func newForsterConstraintResolver(
	ws *crossMinWorkspace,
	arena *layered.LGraphArena,
	order [][]layered.LNodeID,
) *ForsterConstraintResolver {
	constraints := false
	if len(order) > 0 && len(order[0]) > 0 {
		graphID := arena.NodeGraph(order[0][0])
		constraints = basegraph.Get(&arena.Graph(graphID).Properties, layered.InLayerSuccessorConstraintsBetweenNonDummies)
	}
	total := 0
	for _, nodes := range order {
		total += len(nodes)
	}
	resolver := ws.resolverSlab().one()
	resolver.constraintsBetweenNonDummies = constraints
	resolver.BarycenterStates = ws.stateListSlab().take(len(order))
	resolver.stateBacking = ws.stateSlab().take(total)
	return resolver
}

// InitAtLayerLevel gives the layer zeroed states (fresh, never handed out
// before), as make did.
func (resolver *ForsterConstraintResolver) InitAtLayerLevel(layerIndex int, order [][]layered.LNodeID) {
	n := len(order[layerIndex])
	if start := resolver.stateUsed; start+n <= len(resolver.stateBacking) {
		resolver.BarycenterStates[layerIndex] = resolver.stateBacking[start : start+n : start+n]
		resolver.stateUsed += n
		return
	}
	resolver.BarycenterStates[layerIndex] = make([]BarycenterState, n)
}

func (resolver *ForsterConstraintResolver) InitAtNodeLevel(
	arena *layered.LGraphArena,
	layerIndex, nodeIndex int,
	order [][]layered.LNodeID,
) {
	nodeID := order[layerIndex][nodeIndex]
	resolver.BarycenterStates[layerIndex][nodeIndex] = BarycenterState{Node: nodeID}
	if layoutUnit, found := basegraph.TryGet(&arena.Node(nodeID).Properties, layered.InLayerLayoutUnit); found {
		if resolver.layoutUnits == nil {
			resolver.layoutUnits = make(map[layered.LNodeID][]layered.LNodeID)
		}
		resolver.layoutUnits[layoutUnit] = append(resolver.layoutUnits[layoutUnit], nodeID)
	}
}

func InitializeForsterConstraintResolver(
	resolver *ForsterConstraintResolver,
	arena *layered.LGraphArena,
	order [][]layered.LNodeID,
) {
	for layerIndex, nodes := range order {
		resolver.InitAtLayerLevel(layerIndex, order)
		if len(nodes) > 0 {
			layerID := *arena.Node(nodes[0]).Layer
			arena.Layer(layerID).ID = layerIndex
		}
		for nodeIndex, nodeID := range nodes {
			arena.Node(nodeID).ID = nodeIndex
			resolver.InitAtNodeLevel(arena, layerIndex, nodeIndex, order)
		}
	}
}

type constraintGroup struct {
	summedWeight             float64
	degree                   int
	nodes                    []layered.LNodeID
	outgoing                 []int
	incoming                 []int
	incomingConstraintsCount int
}

func (resolver *ForsterConstraintResolver) ProcessConstraints(
	arena *layered.LGraphArena,
	nodes *[]layered.LNodeID,
) {
	if resolver.constraintsBetweenNonDummies {
		resolver.processConstraintStage(arena, nodes, true)
	}
	resolver.processConstraintStage(arena, nodes, false)
}

func (resolver *ForsterConstraintResolver) processConstraintStage(
	arena *layered.LGraphArena,
	nodes *[]layered.LNodeID,
	onlyNormal bool,
) {
	if resolver.stageAddsNoConstraint(arena, *nodes, onlyNormal) {
		// With no constraint every node stays its own group, no constraint
		// is violated, and the stage writes back the same order and every
		// node's own barycenter: nothing changes.
		return
	}
	count := len(*nodes)
	scratch := resolver.scratch
	if scratch == nil {
		scratch = &forsterScratch{}
		resolver.scratch = scratch
	}
	// Each node starts as its own group. The singleton node lists share one
	// backing array; capacity 1 makes any append copy, as before.
	singletons := append(scratch.singleton[:0], *nodes...)
	scratch.singleton = singletons
	groups := scratch.groups[:0]
	list := scratch.list[:0]
	groupOf := scratch.groupOf
	if groupOf == nil {
		groupOf = make(map[layered.LNodeID]int, count)
		scratch.groupOf = groupOf
	} else {
		for nodeID := range groupOf {
			delete(groupOf, nodeID)
		}
	}
	for index, nodeID := range singletons {
		groups = append(groups, constraintGroup{nodes: singletons[index : index+1 : index+1]})
		groupOf[nodeID] = index
		list = append(list, index)
	}
	scratch.groupOfLargest = max(scratch.groupOfLargest, len(groupOf))
	resolver.buildConstraintGraph(arena, groups, groupOf, list, onlyNormal)
	for {
		first, second, found := resolver.findViolatedConstraint(arena, groups, list)
		if !found {
			break
		}
		resolver.handleViolatedConstraint(arena, &groups, &list, first, second)
	}
	result := scratch.result[:0]
	for _, groupID := range list {
		barycenter, hasBarycenter := resolver.groupBarycenter(arena, groups, groupID)
		for _, nodeID := range groups[groupID].nodes {
			result = append(result, nodeID)
			state := resolver.state(arena, nodeID)
			state.Barycenter, state.HasBarycenter = barycenter, hasBarycenter
		}
	}
	scratch.groups, scratch.list, scratch.result = groups, list, result
	// Every node is in exactly one listed group, so result has the length of
	// *nodes; write it in place (ELK assigns the new list).
	if len(result) == len(*nodes) {
		copy(*nodes, result)
	} else {
		*nodes = append([]layered.LNodeID(nil), result...)
	}
}

// stageAddsNoConstraint reads the successor constraints of the stage's
// nodes in the order buildConstraintGraph reads them (the first read of a
// node materializes the empty default into its property map, so it must
// happen here, exactly when the full stage would make it) and reports
// whether buildConstraintGraph would add no edge: no read node has a
// successor constraint, and the stage either ignores layout units
// (onlyNormal) or there are none.
func (resolver *ForsterConstraintResolver) stageAddsNoConstraint(
	arena *layered.LGraphArena,
	nodes []layered.LNodeID,
	onlyNormal bool,
) bool {
	none := onlyNormal || len(resolver.layoutUnits) == 0
	for _, nodeID := range nodes {
		if onlyNormal && arena.Node(nodeID).Type != layered.NormalNode {
			continue
		}
		if len(resolver.readSuccessors(resolver.state(arena, nodeID), arena, nodeID)) > 0 {
			none = false
		}
	}
	return none
}

func (resolver *ForsterConstraintResolver) buildConstraintGraph(
	arena *layered.LGraphArena,
	groups []constraintGroup,
	groupOf map[layered.LNodeID]int,
	list []int,
	onlyNormal bool,
) {
	for _, groupID := range list {
		groups[groupID].outgoing = nil
		groups[groupID].incomingConstraintsCount = 0
	}
	var lastNormal layered.LNodeID
	hasLastNormal := false
	for _, groupID := range list {
		nodeID := groups[groupID].nodes[0]
		if onlyNormal && arena.Node(nodeID).Type != layered.NormalNode {
			continue
		}
		for _, successor := range resolver.state(arena, nodeID).successorConstraints(arena, nodeID) {
			if onlyNormal && arena.Node(successor).Type != layered.NormalNode {
				continue
			}
			successorGroup, found := groupOf[successor]
			if !found {
				continue
			}
			groups[groupID].outgoing = append(groups[groupID].outgoing, successorGroup)
			groups[successorGroup].incomingConstraintsCount++
		}
		if !onlyNormal && arena.Node(nodeID).Type == layered.NormalNode {
			if hasLastNormal {
				for _, previousUnitNode := range resolver.layoutUnits[lastNormal] {
					for _, currentUnitNode := range resolver.layoutUnits[nodeID] {
						previousGroup, previousFound := groupOf[previousUnitNode]
						currentGroup, currentFound := groupOf[currentUnitNode]
						if previousFound && currentFound {
							groups[previousGroup].outgoing = append(groups[previousGroup].outgoing, currentGroup)
							groups[currentGroup].incomingConstraintsCount++
						}
					}
				}
			}
			lastNormal, hasLastNormal = nodeID, true
		}
	}
}

func (resolver *ForsterConstraintResolver) findViolatedConstraint(
	arena *layered.LGraphArena,
	groups []constraintGroup,
	list []int,
) (int, int, bool) {
	active := make([]int, 0)
	for _, groupID := range list {
		groups[groupID].incoming = nil
		if len(groups[groupID].outgoing) > 0 && groups[groupID].incomingConstraintsCount == 0 {
			active = append(active, groupID)
		}
	}
	for len(active) > 0 {
		groupID := active[0]
		active = active[1:]
		groupBarycenter, groupHas := resolver.groupBarycenter(arena, groups, groupID)
		for _, predecessor := range groups[groupID].incoming {
			predecessorBarycenter, predecessorHas := resolver.groupBarycenter(arena, groups, predecessor)
			if !predecessorHas || !groupHas {
				continue
			}
			if float32(predecessorBarycenter) == float32(groupBarycenter) {
				if groupListIndex(list, predecessor) > groupListIndex(list, groupID) {
					return predecessor, groupID, true
				}
			} else if predecessorBarycenter > groupBarycenter {
				return predecessor, groupID, true
			}
		}
		for _, successor := range groups[groupID].outgoing {
			groups[successor].incoming = append([]int{groupID}, groups[successor].incoming...)
			if groups[successor].incomingConstraintsCount == len(groups[successor].incoming) {
				active = append(active, successor)
			}
		}
	}
	return 0, 0, false
}

func (resolver *ForsterConstraintResolver) handleViolatedConstraint(
	arena *layered.LGraphArena,
	groups *[]constraintGroup,
	list *[]int,
	first, second int,
) {
	newGroup := resolver.mergeGroups(arena, groups, first, second)
	newBarycenter, newHas := resolver.groupBarycenter(arena, *groups, newGroup)
	inserted := false
	for index := 0; index < len(*list); {
		groupID := (*list)[index]
		if groupID == first || groupID == second {
			*list = append((*list)[:index], (*list)[index+1:]...)
			continue
		}
		barycenter, has := resolver.groupBarycenter(arena, *groups, groupID)
		if !inserted && has && newHas && barycenter > newBarycenter {
			*list = append(*list, 0)
			copy((*list)[index+1:], (*list)[index:])
			(*list)[index] = newGroup
			inserted = true
			index++
			continue
		}
		outgoing := &(*groups)[groupID].outgoing
		firstRemoved := removeFirstGroup(outgoing, first)
		secondRemoved := removeFirstGroup(outgoing, second)
		if firstRemoved || secondRemoved {
			*outgoing = append(*outgoing, newGroup)
			(*groups)[newGroup].incomingConstraintsCount++
		}
		index++
	}
	if !inserted {
		*list = append(*list, newGroup)
	}
}

func (resolver *ForsterConstraintResolver) mergeGroups(
	arena *layered.LGraphArena,
	groups *[]constraintGroup,
	first, second int,
) int {
	firstGroup, secondGroup := (*groups)[first], (*groups)[second]
	nodes := append(append([]layered.LNodeID(nil), firstGroup.nodes...), secondGroup.nodes...)
	var outgoing []int
	if firstGroup.outgoing != nil {
		outgoing = append([]int(nil), firstGroup.outgoing...)
		removeFirstGroup(&outgoing, second)
		for _, candidate := range secondGroup.outgoing {
			if candidate == first {
				continue
			}
			if containsGroup(outgoing, candidate) {
				(*groups)[candidate].incomingConstraintsCount--
			} else {
				outgoing = append(outgoing, candidate)
			}
		}
	} else if secondGroup.outgoing != nil {
		outgoing = append([]int(nil), secondGroup.outgoing...)
		removeFirstGroup(&outgoing, first)
	}
	summedWeight := firstGroup.summedWeight + secondGroup.summedWeight
	degree := firstGroup.degree + secondGroup.degree
	firstBarycenter, firstHas := resolver.groupBarycenter(arena, *groups, first)
	secondBarycenter, secondHas := resolver.groupBarycenter(arena, *groups, second)
	newID := len(*groups)
	*groups = append(*groups, constraintGroup{summedWeight: summedWeight, degree: degree, nodes: nodes, outgoing: outgoing})
	var barycenter float64
	hasBarycenter := true
	switch {
	case degree > 0:
		barycenter = summedWeight / float64(degree)
	case firstHas && secondHas:
		barycenter = (firstBarycenter + secondBarycenter) / 2
	case firstHas:
		barycenter = firstBarycenter
	case secondHas:
		barycenter = secondBarycenter
	default:
		hasBarycenter = false
	}
	if hasBarycenter {
		for _, nodeID := range nodes {
			resolver.state(arena, nodeID).setBarycenter(barycenter)
		}
	}
	return newID
}

func (resolver *ForsterConstraintResolver) groupBarycenter(
	arena *layered.LGraphArena,
	groups []constraintGroup,
	groupID int,
) (float64, bool) {
	state := resolver.state(arena, groups[groupID].nodes[0])
	return state.Barycenter, state.HasBarycenter
}

func (resolver *ForsterConstraintResolver) state(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
) *BarycenterState {
	if slot, owned := resolver.index.slotOf(nodeID, resolver.owner); owned {
		return &resolver.stateBacking[slot]
	}
	layerIndex := arena.Layer(*arena.Node(nodeID).Layer).ID
	return &resolver.BarycenterStates[layerIndex][arena.Node(nodeID).ID]
}

func groupListIndex(list []int, groupID int) int {
	for index, candidate := range list {
		if candidate == groupID {
			return index
		}
	}
	return -1
}

func removeFirstGroup(values *[]int, remove int) bool {
	for index, value := range *values {
		if value == remove {
			*values = append((*values)[:index], (*values)[index+1:]...)
			return true
		}
	}
	return false
}

func containsGroup(values []int, find int) bool {
	for _, value := range values {
		if value == find {
			return true
		}
	}
	return false
}
