// Copyright (c) 2016 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"fmt"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
)

// CrossMinType is LayerSweepCrossingMinimizer.CrossMinType. Declaration
// order follows ELK 0.8.1 and is observable through the zero value used by the
// phase-three provider.
type CrossMinType uint8

const (
	CrossMinBarycenter CrossMinType = iota
	CrossMinOneSidedGreedySwitch
	CrossMinTwoSidedGreedySwitch
)

// GraphInfoHolder collects all state initialized by a single IInitializable
// traversal of one graph.
type GraphInfoHolder struct {
	LGraph layered.LGraphID

	CurrentNodeOrder              [][]layered.LNodeID
	CurrentlyBestNodeAndPortOrder *SweepCopy
	BestNodeAndPortOrder          *SweepCopy
	PortPositions                 []int
	UseBottomUp                   bool
	ChildGraphs                   []layered.LGraphID
	HasExternalPorts              bool
	HasParent                     bool
	Parent                        *layered.LNodeID
	ParentGraphData               *GraphInfoHolder
	CrossMinType                  CrossMinType
	CrossMinimizer                crossingMinimizationHeuristic
	PortDistributor               sweepPortDistributor
	CrossingsCounter              *AllCrossingsCounter
	nPorts                        int
}

func NewGraphInfoHolder(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	crossMinType CrossMinType,
	graphs []*GraphInfoHolder,
	random *javacompat.Random,
) (*GraphInfoHolder, error) {
	graph := arena.Graph(graphID)
	graphRandom, found := basegraph.TryGet(&graph.Properties, layered.Random)
	if !found || graphRandom == nil {
		graphRandom = random
		basegraph.Set(&graph.Properties, layered.Random, graphRandom)
	}
	order := make([][]layered.LNodeID, len(graph.Layers))
	for index, layerID := range graph.Layers {
		order[index] = append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...)
	}

	holder := &GraphInfoHolder{
		LGraph:           graphID,
		CurrentNodeOrder: order,
		HasParent:        graph.ParentNode != nil,
		Parent:           copyNodeID(graph.ParentNode),
		CrossMinType:     crossMinType,
		CrossingsCounter: NewAllCrossingsCounter(len(order)),
	}
	if holder.HasParent {
		parentGraphIndex := arena.Graph(arena.NodeGraph(*holder.Parent)).ID
		if parentGraphIndex < 0 || parentGraphIndex >= len(graphs) {
			return nil, fmt.Errorf("p3order: parent graph index %d is not initialized", parentGraphIndex)
		}
		holder.ParentGraphData = graphs[parentGraphIndex]
	}
	holder.HasExternalPorts = basegraph.Get(&graph.Properties, layered.GraphProperties)&
		layered.GraphPropertyExternalPorts != 0

	decider := NewLayerSweepTypeDecider(len(order))
	var resolver *ForsterConstraintResolver
	var barycenterDistributor *BarycenterPortDistributor
	var greedyDistributor *GreedyPortDistributor
	var greedyHeuristic *GreedySwitchHeuristic
	if crossMinType == CrossMinTwoSidedGreedySwitch {
		greedyDistributor = NewGreedyPortDistributor()
		holder.PortDistributor = greedyDistributor
	} else {
		// ISweepPortDistributor.create consumes one boolean for BARYCENTER and
		// ONE_SIDED_GREEDY_SWITCH, but none for TWO_SIDED_GREEDY_SWITCH.
		distributorKind := LayerTotalPortDistributor
		if graphRandom.NextBoolean() {
			distributorKind = NodeRelativePortDistributor
		}
		barycenterDistributor = NewBarycenterPortDistributor(distributorKind, len(order))
		holder.PortDistributor = barycenterDistributor
	}
	if crossMinType == CrossMinBarycenter {
		resolver = NewForsterConstraintResolver(arena, order)
		barycenter := NewBarycenterHeuristic(arena, resolver, graphRandom, barycenterDistributor)
		if basegraph.Get(&graph.Properties, layered.ForceNodeModelOrder) {
			barycenter.ModelOrder = NewModelOrderBarycenterHeuristic(true)
		}
		holder.CrossMinimizer = barycenter
	} else {
		greedyHeuristic = NewGreedySwitchHeuristic(arena, crossMinType, holder)
		holder.CrossMinimizer = greedyHeuristic
	}

	// IInitializable.init invokes every initializable at each level before
	// descending. Keep this flattened traversal source-shaped because node and
	// port scratch IDs are shared by several helpers.
	for layerIndex, nodes := range order {
		decider.InitAtLayerLevel(arena, layerIndex, order)
		if barycenterDistributor != nil {
			barycenterDistributor.InitAtLayerLevel(layerIndex, order)
		}
		if resolver != nil {
			resolver.InitAtLayerLevel(layerIndex, order)
		}
		layerID := *arena.Node(nodes[0]).Layer
		arena.Layer(layerID).ID = layerIndex
		if greedyHeuristic != nil {
			greedyHeuristic.InitAtLayerLevel(arena, layerIndex, order)
		}

		for nodeIndex, nodeID := range nodes {
			if nested := arena.Node(nodeID).NestedGraph; nested != nil {
				holder.ChildGraphs = append(holder.ChildGraphs, *nested)
			}
			holder.CrossingsCounter.InitAtNodeLevel(arena, layerIndex, nodeIndex, order)
			decider.InitAtNodeLevel(arena, layerIndex, nodeIndex, order)
			if barycenterDistributor != nil {
				barycenterDistributor.InitAtNodeLevel(arena, layerIndex, nodeIndex, order)
			} else {
				greedyDistributor.InitAtNodeLevel(arena, layerIndex, nodeIndex, order)
			}
			if resolver != nil {
				resolver.InitAtNodeLevel(arena, layerIndex, nodeIndex, order)
			}

			for portIndex, portID := range arena.Node(nodeID).Ports {
				holder.nPorts++
				holder.CrossingsCounter.InitAtPortLevel(arena, layerIndex, nodeIndex, portIndex, order)
				if barycenterDistributor != nil {
					barycenterDistributor.InitAtPortLevel(arena, layerIndex, nodeIndex, portIndex, order)
				}
				if greedyHeuristic != nil {
					greedyHeuristic.InitAtPortLevel()
				}
				for _, edgeID := range arena.PortConnectedEdges(portID) {
					holder.CrossingsCounter.InitAtEdgeLevel(
						arena, layerIndex, nodeIndex, portIndex, edgeID, order,
					)
				}
			}
		}
	}
	holder.PortPositions = make([]int, holder.nPorts)
	holder.CrossingsCounter.InitAfterTraversal()
	if barycenterDistributor != nil {
		barycenterDistributor.InitAfterTraversal()
	} else {
		greedyDistributor.InitAfterTraversal()
	}
	if greedyHeuristic != nil {
		greedyHeuristic.InitAfterTraversal()
	}
	holder.UseBottomUp = decider.UseBottomUp(
		arena, graphID, holder.Parent, holder.CrossMinDeterministic(), order,
	)
	return holder, nil
}

func (holder *GraphInfoHolder) DontSweepInto() bool { return holder.UseBottomUp }

func (holder *GraphInfoHolder) CrossMinDeterministic() bool {
	return holder.CrossMinType != CrossMinBarycenter
}

func (holder *GraphInfoHolder) CrossMinAlwaysImproves() bool {
	return holder.CrossMinType == CrossMinTwoSidedGreedySwitch
}

func (holder *GraphInfoHolder) GetBestSweep() *SweepCopy {
	if holder.CrossMinDeterministic() {
		return holder.CurrentlyBestNodeAndPortOrder
	}
	return holder.BestNodeAndPortOrder
}

func copyNodeID(nodeID *layered.LNodeID) *layered.LNodeID {
	if nodeID == nil {
		return nil
	}
	copy := *nodeID
	return &copy
}
