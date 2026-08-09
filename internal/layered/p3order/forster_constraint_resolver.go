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

type BarycenterState struct {
	Node         layered.LNodeID
	SummedWeight float64
	Degree       int
	Barycenter   *float64
	Visited      bool
}

type ForsterConstraintResolver struct {
	constraintsBetweenNonDummies bool
	layoutUnits                  map[layered.LNodeID][]layered.LNodeID
	BarycenterStates             [][]BarycenterState
}

func NewForsterConstraintResolver(
	arena *layered.LGraphArena,
	order [][]layered.LNodeID,
) *ForsterConstraintResolver {
	constraints := false
	if len(order) > 0 && len(order[0]) > 0 {
		graphID := arena.NodeGraph(order[0][0])
		constraints = basegraph.Get(&arena.Graph(graphID).Properties, layered.InLayerSuccessorConstraintsBetweenNonDummies)
	}
	return &ForsterConstraintResolver{
		constraintsBetweenNonDummies: constraints,
		layoutUnits:                  make(map[layered.LNodeID][]layered.LNodeID),
		BarycenterStates:             make([][]BarycenterState, len(order)),
	}
}

func (resolver *ForsterConstraintResolver) InitAtLayerLevel(layerIndex int, order [][]layered.LNodeID) {
	resolver.BarycenterStates[layerIndex] = make([]BarycenterState, len(order[layerIndex]))
}

func (resolver *ForsterConstraintResolver) InitAtNodeLevel(
	arena *layered.LGraphArena,
	layerIndex, nodeIndex int,
	order [][]layered.LNodeID,
) {
	nodeID := order[layerIndex][nodeIndex]
	resolver.BarycenterStates[layerIndex][nodeIndex] = BarycenterState{Node: nodeID}
	if layoutUnit, found := basegraph.TryGet(&arena.Node(nodeID).Properties, layered.InLayerLayoutUnit); found {
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
	groups := make([]constraintGroup, len(*nodes))
	groupOf := make(map[layered.LNodeID]int, len(*nodes))
	list := make([]int, len(*nodes))
	for index, nodeID := range *nodes {
		groups[index] = constraintGroup{nodes: []layered.LNodeID{nodeID}}
		groupOf[nodeID] = index
		list[index] = index
	}
	resolver.buildConstraintGraph(arena, groups, groupOf, list, onlyNormal)
	for {
		first, second, found := resolver.findViolatedConstraint(arena, groups, list)
		if !found {
			break
		}
		resolver.handleViolatedConstraint(arena, &groups, &list, first, second)
	}
	result := make([]layered.LNodeID, 0, len(*nodes))
	for _, groupID := range list {
		barycenter := resolver.groupBarycenter(arena, groups, groupID)
		for _, nodeID := range groups[groupID].nodes {
			result = append(result, nodeID)
			resolver.state(arena, nodeID).Barycenter = cloneFloat(barycenter)
		}
	}
	*nodes = result
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
	var lastNormal *layered.LNodeID
	for _, groupID := range list {
		nodeID := groups[groupID].nodes[0]
		if onlyNormal && arena.Node(nodeID).Type != layered.NormalNode {
			continue
		}
		for _, successor := range basegraph.Get(&arena.Node(nodeID).Properties, layered.InLayerSuccessorConstraints) {
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
			if lastNormal != nil {
				for _, previousUnitNode := range resolver.layoutUnits[*lastNormal] {
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
			copy := nodeID
			lastNormal = &copy
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
		groupBarycenter := resolver.groupBarycenter(arena, groups, groupID)
		for _, predecessor := range groups[groupID].incoming {
			predecessorBarycenter := resolver.groupBarycenter(arena, groups, predecessor)
			if predecessorBarycenter == nil || groupBarycenter == nil {
				continue
			}
			if float32(*predecessorBarycenter) == float32(*groupBarycenter) {
				if groupListIndex(list, predecessor) > groupListIndex(list, groupID) {
					return predecessor, groupID, true
				}
			} else if *predecessorBarycenter > *groupBarycenter {
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
	newBarycenter := resolver.groupBarycenter(arena, *groups, newGroup)
	inserted := false
	for index := 0; index < len(*list); {
		groupID := (*list)[index]
		if groupID == first || groupID == second {
			*list = append((*list)[:index], (*list)[index+1:]...)
			continue
		}
		barycenter := resolver.groupBarycenter(arena, *groups, groupID)
		if !inserted && barycenter != nil && newBarycenter != nil && *barycenter > *newBarycenter {
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
	firstBarycenter := resolver.groupBarycenter(arena, *groups, first)
	secondBarycenter := resolver.groupBarycenter(arena, *groups, second)
	newID := len(*groups)
	*groups = append(*groups, constraintGroup{summedWeight: summedWeight, degree: degree, nodes: nodes, outgoing: outgoing})
	var barycenter *float64
	switch {
	case degree > 0:
		barycenter = floatPtr(summedWeight / float64(degree))
	case firstBarycenter != nil && secondBarycenter != nil:
		barycenter = floatPtr((*firstBarycenter + *secondBarycenter) / 2)
	case firstBarycenter != nil:
		barycenter = floatPtr(*firstBarycenter)
	case secondBarycenter != nil:
		barycenter = floatPtr(*secondBarycenter)
	}
	if barycenter != nil {
		for _, nodeID := range nodes {
			resolver.state(arena, nodeID).Barycenter = floatPtr(*barycenter)
		}
	}
	return newID
}

func (resolver *ForsterConstraintResolver) groupBarycenter(
	arena *layered.LGraphArena,
	groups []constraintGroup,
	groupID int,
) *float64 {
	return resolver.state(arena, groups[groupID].nodes[0]).Barycenter
}

func (resolver *ForsterConstraintResolver) state(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
) *BarycenterState {
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

func floatPtr(value float64) *float64 { return &value }
func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	return floatPtr(*value)
}
