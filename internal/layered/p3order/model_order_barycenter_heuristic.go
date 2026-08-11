// Copyright (c) 2021 Kiel University and others.
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
	"github.com/d2lang/elk-go/internal/layered/intermediate/preserveorder"
)

// ModelOrderBarycenterHeuristic is ELK 0.8.1's partial-order comparator used
// when model order is considered during crossing minimization.
type ModelOrderBarycenterHeuristic struct {
	ForceNodeModelOrder bool
	biggerThan          map[layered.LNodeID]map[layered.LNodeID]bool
	smallerThan         map[layered.LNodeID]map[layered.LNodeID]bool
}

func NewModelOrderBarycenterHeuristic(force bool) *ModelOrderBarycenterHeuristic {
	return &ModelOrderBarycenterHeuristic{
		ForceNodeModelOrder: force,
		biggerThan:          make(map[layered.LNodeID]map[layered.LNodeID]bool),
		smallerThan:         make(map[layered.LNodeID]map[layered.LNodeID]bool),
	}
}

func (modelOrder *ModelOrderBarycenterHeuristic) sort(
	heuristic *BarycenterHeuristic,
	layer []layered.LNodeID,
) {
	compare := func(first, second layered.LNodeID) int {
		return modelOrder.compare(heuristic, first, second)
	}
	if modelOrder.ForceNodeModelOrder {
		for index := 1; index < len(layer); index++ {
			insert := layer[index]
			position := index
			for position > 0 && compare(layer[position-1], insert) > 0 {
				layer[position] = layer[position-1]
				position--
			}
			layer[position] = insert
		}
		modelOrder.ClearTransitiveOrdering()
		return
	}
	javacompat.GWTStableSort(layer, compare)
}

func (modelOrder *ModelOrderBarycenterHeuristic) compare(
	heuristic *BarycenterHeuristic,
	first, second layered.LNodeID,
) int {
	if transitive := modelOrder.compareTransitive(first, second); transitive != 0 {
		return transitive
	}
	firstNode, secondNode := heuristic.Arena.Node(first), heuristic.Arena.Node(second)
	if basegraph.Has(&firstNode.Properties, layered.ModelOrder) && basegraph.Has(&secondNode.Properties, layered.ModelOrder) {
		graph := heuristic.Arena.Graph(heuristic.Arena.NodeGraph(first))
		offset := basegraph.Get(&graph.Properties, layered.MaximumModelOrderNodes)
		value := compareInt(
			preserveorder.CalculateModelOrderOrGroupModelOrder(
				graph, &firstNode.Properties, &secondNode.Properties, offset),
			preserveorder.CalculateModelOrderOrGroupModelOrder(
				graph, &secondNode.Properties, &firstNode.Properties, offset),
		)
		if basegraph.Get(&graph.Properties, layered.CrossingMinimizationGroupOrder) ==
			layered.GroupOrderOnlyWithinGroup &&
			basegraph.Get(&firstNode.Properties, layered.CrossingMinimizationGroupID) !=
				basegraph.Get(&secondNode.Properties, layered.CrossingMinimizationGroupID) {
			value = 0
		}
		if value < 0 {
			modelOrder.update(first, second)
			return value
		} else if value > 0 {
			modelOrder.update(second, first)
			return value
		}
		// Since ELK 0.11, equal model-order values no longer freeze the
		// relative order. Fall back to the barycenter comparison so the
		// connected port geometry can break the tie.
	}
	value := heuristic.compareBarycenters(first, second)
	if value < 0 {
		modelOrder.update(first, second)
	} else if value > 0 {
		modelOrder.update(second, first)
	}
	return value
}

func (modelOrder *ModelOrderBarycenterHeuristic) compareTransitive(first, second layered.LNodeID) int {
	modelOrder.ensure(first)
	if modelOrder.biggerThan[first][second] {
		return 1
	}
	modelOrder.ensure(second)
	if modelOrder.biggerThan[second][first] {
		return -1
	}
	if modelOrder.smallerThan[first][second] {
		return -1
	}
	if modelOrder.smallerThan[second][first] {
		return 1
	}
	return 0
}

// bigger/smaller names follow Java: update(bigger,smaller) records that the
// comparator found bigger < smaller in the final order.
func (modelOrder *ModelOrderBarycenterHeuristic) update(bigger, smaller layered.LNodeID) {
	modelOrder.ensure(bigger)
	modelOrder.ensure(smaller)
	smallerBiggerThan := keysOf(modelOrder.biggerThan[smaller])
	biggerSmallerThan := keysOf(modelOrder.smallerThan[bigger])
	modelOrder.biggerThan[bigger][smaller] = true
	modelOrder.smallerThan[smaller][bigger] = true
	for _, verySmall := range smallerBiggerThan {
		modelOrder.biggerThan[bigger][verySmall] = true
		modelOrder.smallerThan[verySmall][bigger] = true
		for _, transitive := range biggerSmallerThan {
			modelOrder.smallerThan[verySmall][transitive] = true
		}
	}
	for _, veryBig := range biggerSmallerThan {
		modelOrder.smallerThan[smaller][veryBig] = true
		modelOrder.biggerThan[veryBig][smaller] = true
		for _, transitive := range smallerBiggerThan {
			modelOrder.biggerThan[veryBig][transitive] = true
		}
	}
}

func (modelOrder *ModelOrderBarycenterHeuristic) ensure(nodeID layered.LNodeID) {
	if modelOrder.biggerThan[nodeID] == nil {
		modelOrder.biggerThan[nodeID] = make(map[layered.LNodeID]bool)
	}
	if modelOrder.smallerThan[nodeID] == nil {
		modelOrder.smallerThan[nodeID] = make(map[layered.LNodeID]bool)
	}
}

func (modelOrder *ModelOrderBarycenterHeuristic) ClearTransitiveOrdering() {
	modelOrder.biggerThan = make(map[layered.LNodeID]map[layered.LNodeID]bool)
	modelOrder.smallerThan = make(map[layered.LNodeID]map[layered.LNodeID]bool)
}

func keysOf(values map[layered.LNodeID]bool) []layered.LNodeID {
	result := make([]layered.LNodeID, 0, len(values))
	for nodeID := range values {
		result = append(result, nodeID)
	}
	return result
}

func compareInt(first, second int) int {
	if first < second {
		return -1
	}
	if first > second {
		return 1
	}
	return 0
}
