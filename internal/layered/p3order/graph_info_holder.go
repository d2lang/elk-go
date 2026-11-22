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

	// sweepsIntoChildren is false when no node of the holder's layers can
	// have a nested graph that is swept into (see the minimizer's
	// initialize and sweepInHierarchicalNode).
	sweepsIntoChildren bool

	// tryFlag is Get(graph, FirstTryWithInitialOrder) of the holder's graph
	// (SecondTryWithInitialOrder shares its key) once tryFlagKnown.
	tryFlag, tryFlagKnown bool
}

// tryWithInitialOrder is Get(graph, FirstTryWithInitialOrder) of the
// holder's graph. The value is read once and then kept up to date by
// setTryWithInitialOrder, the only writer of the key while the minimizer
// runs (the default is not materialized, so the read has no effect).
func (holder *GraphInfoHolder) tryWithInitialOrder(arena *layered.LGraphArena) bool {
	if !holder.tryFlagKnown {
		holder.tryFlag = basegraph.Get(&arena.Graph(holder.LGraph).Properties, layered.FirstTryWithInitialOrder)
		holder.tryFlagKnown = true
	}
	return holder.tryFlag
}

// setTryWithInitialOrder is Set(graph, property, value) for
// FirstTryWithInitialOrder or SecondTryWithInitialOrder (one key).
func (holder *GraphInfoHolder) setTryWithInitialOrder(
	arena *layered.LGraphArena,
	property basegraph.Property[bool],
	value bool,
) {
	basegraph.Set(&arena.Graph(holder.LGraph).Properties, property, value)
	holder.tryFlag, holder.tryFlagKnown = value, true
}

func NewGraphInfoHolder(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	crossMinType CrossMinType,
	graphs []*GraphInfoHolder,
	random *javacompat.Random,
) (*GraphInfoHolder, error) {
	return newGraphInfoHolder(arena, graphID, crossMinType, graphs, random, holderContext{})
}

// holderContext is the minimizer's shared storage for a holder: the
// constraint resolvers' scratch (when not nil) and the node index with the
// holder's owner number (index + 1; unused when index is nil).
type holderContext struct {
	scratch *forsterScratch
	index   *sweepIndex
	owner   int32
	ws      *crossMinWorkspace // storage for the holder (nil: allocate)
}

// newGraphInfoHolder is NewGraphInfoHolder with the minimizer's shared
// storage.
func newGraphInfoHolder(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	crossMinType CrossMinType,
	graphs []*GraphInfoHolder,
	random *javacompat.Random,
	context holderContext,
) (*GraphInfoHolder, error) {
	graph := arena.Graph(graphID)
	graphRandom, found := basegraph.TryGet(&graph.Properties, layered.Random)
	if !found || graphRandom == nil {
		graphRandom = random
		basegraph.Set(&graph.Properties, layered.Random, graphRandom)
	}
	ws := context.ws
	// The layers' copies are cut from one array (capacity capped at length;
	// nothing appends to them). An empty layer stays nil, as before.
	order := ws.nodeListSlab().take(len(graph.Layers))
	totalNodes := 0
	for _, layerID := range graph.Layers {
		totalNodes += len(arena.Layer(layerID).Nodes)
	}
	orderBacking := ws.nodeIDSlab().take(totalNodes)
	for index, layerID := range graph.Layers {
		nodes := arena.Layer(layerID).Nodes
		if len(nodes) == 0 {
			continue
		}
		layer := orderBacking[:len(nodes):len(nodes)]
		orderBacking = orderBacking[len(nodes):]
		copy(layer, nodes)
		order[index] = layer
	}

	holder := ws.holderSlab().one()
	*holder = GraphInfoHolder{
		LGraph:           graphID,
		CurrentNodeOrder: order,
		HasParent:        graph.ParentNode != nil,
		Parent:           copyNodeIDInto(ws.nodeIDSlab(), graph.ParentNode),
		CrossMinType:     crossMinType,
		CrossingsCounter: newAllCrossingsCounter(ws, len(order)),
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

	decider := newLayerSweepTypeDecider(ws, len(order), totalNodes)
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
		barycenterDistributor = newBarycenterPortDistributor(ws, distributorKind, len(order))
		barycenterDistributor.useIndex(ws, context.index, context.owner, totalNodes)
		holder.PortDistributor = barycenterDistributor
	}
	if crossMinType == CrossMinBarycenter {
		resolver = newForsterConstraintResolver(ws, arena, order)
		if context.scratch != nil {
			resolver.scratch = context.scratch
		}
		resolver.index, resolver.owner = context.index, context.owner
		barycenter := ws.heuristicSlab().one()
		*barycenter = BarycenterHeuristic{Arena: arena, Resolver: resolver, Random: graphRandom, Distributor: barycenterDistributor}
		if ws != nil {
			barycenter.scratch = &ws.heuristicScratch
		}
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
	if ws != nil {
		// The child graphs are collected into a list of the final size (nil
		// when there are none, as append left it).
		children := 0
		for _, nodes := range order {
			for _, nodeID := range nodes {
				if arena.Node(nodeID).NestedGraph != nil {
					children++
				}
			}
		}
		if children > 0 {
			holder.ChildGraphs = ws.graphIDSlab().take(children)[:0]
		}
	}
	slot := 0
	traversal := order
	if barycenterDistributor != nil && resolver != nil && greedyHeuristic == nil && context.index != nil {
		initializeBarycenterHolder(arena, holder, order, decider, barycenterDistributor, resolver, context)
		traversal = nil // done
	}
	for layerIndex, nodes := range traversal {
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
			if context.index != nil {
				context.index.set(nodeID, context.owner, slot)
			}
			slot++
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
				// InitAtEdgeLevel only reads, so the lists are walked in place.
				for _, edges := range arena.PortEdgeLists(portID) {
					for _, edgeID := range edges {
						holder.CrossingsCounter.InitAtEdgeLevel(
							arena, layerIndex, nodeIndex, portIndex, edgeID, order,
						)
					}
				}
			}
		}
	}
	holder.PortPositions = ws.intSlab().take(holder.nPorts)
	holder.CrossingsCounter.initAfterTraversal(ws)
	if barycenterDistributor != nil {
		barycenterDistributor.initAfterTraversal(ws)
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

// initializeBarycenterHolder is the IInitializable traversal of
// newGraphInfoHolder for a barycenter holder with a node index, fused into
// one loop: every initializer's writes at each layer, node, port and edge,
// in the same order (the initializers read no scratch ID another one
// writes, and the node and port IDs they write are the same values).
// AllCrossingsCounter.InitAtEdgeLevel only counts in-layer edges into a list
// nothing reads, so it is called only where it would panic (an outgoing edge
// whose endpoints lack nodes), which it then does as before.
func initializeBarycenterHolder(
	arena *layered.LGraphArena,
	holder *GraphInfoHolder,
	order [][]layered.LNodeID,
	decider *LayerSweepTypeDecider,
	distributor *BarycenterPortDistributor,
	resolver *ForsterConstraintResolver,
	context holderContext,
) {
	counter := holder.CrossingsCounter
	index, owner := context.index, context.owner
	slot := 0
	for layerIndex, nodes := range order {
		decider.InitAtLayerLevel(arena, layerIndex, order)
		distributor.InitAtLayerLevel(layerIndex, order)
		resolver.InitAtLayerLevel(layerIndex, order)
		layerID := *arena.Node(nodes[0]).Layer
		arena.Layer(layerID).ID = layerIndex
		positions := distributor.nodePositions[layerIndex]
		states := resolver.BarycenterStates[layerIndex]
		infos := decider.NodeInfo[layerIndex]
		for nodeIndex, nodeID := range nodes {
			index.set(nodeID, owner, slot)
			slot++
			node := arena.Node(nodeID)
			if nested := node.NestedGraph; nested != nil {
				holder.ChildGraphs = append(holder.ChildGraphs, *nested)
			}
			counter.hasNorthSouthPorts[layerIndex] = counter.hasNorthSouthPorts[layerIndex] ||
				node.Type == layered.NorthSouthPortNode
			node.ID = nodeIndex
			infos[nodeIndex] = layerSweepNodeInfo{}
			positions[nodeIndex] = nodeIndex
			states[nodeIndex] = BarycenterState{Node: nodeID}
			if layoutUnit, found := basegraph.TryGet(&node.Properties, layered.InLayerLayoutUnit); found {
				if resolver.layoutUnits == nil {
					resolver.layoutUnits = make(map[layered.LNodeID][]layered.LNodeID)
				}
				resolver.layoutUnits[layoutUnit] = append(resolver.layoutUnits[layoutUnit], nodeID)
			}
			for portIndex, portID := range node.Ports {
				holder.nPorts++
				port := arena.Port(portID)
				port.ID = counter.nPorts
				counter.nPorts++
				distributor.nPorts++
				if len(port.OutgoingEdges)+len(port.IncomingEdges) > 1 {
					if port.Side == layered.EastSide {
						counter.hasHyperEdgesEastOfIndex[layerIndex] = true
					} else if port.Side == layered.WestSide && layerIndex > 0 {
						counter.hasHyperEdgesEastOfIndex[layerIndex-1] = true
					}
				}
				for _, edges := range [2][]layered.LEdgeID{port.IncomingEdges, port.OutgoingEdges} {
					for _, edgeID := range edges {
						edge := arena.Edge(edgeID)
						if edge.Source == nil || *edge.Source != portID {
							continue
						}
						if arena.Port(*edge.Source).Node == nil || edge.Target == nil ||
							arena.Port(*edge.Target).Node == nil {
							counter.InitAtEdgeLevel(arena, layerIndex, nodeIndex, portIndex, edgeID, order)
						}
					}
				}
			}
		}
	}
}

// usePortTable makes the holder's barycenter heuristic, port distributor and
// crossings counter read ports and edges through table.
func (holder *GraphInfoHolder) usePortTable(table *portTable) {
	if heuristic, ok := holder.CrossMinimizer.(*BarycenterHeuristic); ok {
		heuristic.table = table
	}
	if distributor, ok := holder.PortDistributor.(*BarycenterPortDistributor); ok {
		distributor.table = table
	}
	if counter := holder.CrossingsCounter.crossingCounter; counter != nil {
		counter.table = table
	}
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

// copyNodeIDInto returns a copy of *nodeID (nil for nil) taken from nodeIDs.
func copyNodeIDInto(nodeIDs *slab[layered.LNodeID], nodeID *layered.LNodeID) *layered.LNodeID {
	if nodeID == nil {
		return nil
	}
	copy := nodeIDs.one()
	*copy = *nodeID
	return copy
}
