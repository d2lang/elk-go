// Copyright (c) 2010, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import (
	"fmt"
	"math"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/javacompat"
	"github.com/d2lang/elk-go/internal/layered"
	"github.com/d2lang/elk-go/internal/layered/intermediate/preserveorder"
)

var (
	modelOrderCrossingCounterNodeInfluence = basegraph.PropertyWithDefault(
		"org.eclipse.elk.layered.considerModelOrder.crossingCounterNodeInfluence",
		func() float64 { return 0 },
	)
	modelOrderCrossingCounterPortInfluence = basegraph.PropertyWithDefault(
		"org.eclipse.elk.layered.considerModelOrder.crossingCounterPortInfluence",
		func() float64 { return 0 },
	)
)

// LayerSweepCrossingMinimizer is ELK Layered's phase-three layer sweep. Its
// zero value is the BARYCENTER phase implementation used by the provider.
type LayerSweepCrossingMinimizer struct {
	Type CrossMinType

	graphInfoHolders            []*GraphInfoHolder
	graphsWhoseNodeOrderChanged []int
	random                      *javacompat.Random
	randomSeed                  int64

	forsterScratch *forsterScratch // shared by the holders' constraint resolvers
	index          sweepIndex      // nodes of the holders (see sweepIndex)

	// ws holds everything ProcessHierarchy allocates for a BARYCENTER
	// minimization, from a pool; it is released when the call returns.
	ws *crossMinWorkspace
}

func NewLayerSweepCrossingMinimizer(crossMinType CrossMinType) *LayerSweepCrossingMinimizer {
	return &LayerSweepCrossingMinimizer{Type: crossMinType}
}

// HierarchyAware is a marker used by the layered executor. In an
// INCLUDE_CHILDREN layout this processor must be paused on child graphs and
// invoked once on the root after every graph has completed all processors
// preceding phase three.
func (*LayerSweepCrossingMinimizer) HierarchyAware() {}

func (minimizer *LayerSweepCrossingMinimizer) Process(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) error {
	return minimizer.ProcessHierarchy(arena, graphID)
}

// ProcessHierarchy is the source-shaped hierarchy-aware entry point. graphID
// must be the hierarchy root; initialization then discovers every child graph
// breadth-first and assigns GraphInfoHolder indices before sweeping bottom-up
// or through nested graphs as decided by LayerSweepTypeDecider.
func (minimizer *LayerSweepCrossingMinimizer) ProcessHierarchy(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) error {
	if arena.Graph(graphID).ParentNode != nil {
		return fmt.Errorf("p3order: hierarchy-aware layer sweep must be invoked on the root graph")
	}
	graph := arena.Graph(graphID)
	emptyGraph := len(graph.Layers) == 0
	if !emptyGraph {
		emptyGraph = true
		for _, layerID := range graph.Layers {
			if len(arena.Layer(layerID).Nodes) > 0 {
				emptyGraph = false
				break
			}
		}
	}
	singleNode := len(graph.Layers) == 1 && len(arena.Layer(graph.Layers[0]).Nodes) == 1
	hierarchical := basegraph.Get(&graph.Properties, layered.Hierarchy) == layered.HierarchyIncludeChildren
	if emptyGraph || singleNode && !hierarchical {
		return nil
	}
	if singleNode && !singleNodeDisabled && minimizer.processSingleNode(arena, graphID) {
		return nil
	}
	if minimizer.Type != CrossMinBarycenter {
		return minimizer.minimizeHierarchy(arena, graphID)
	}
	minimizer.ws = acquireCrossMinWorkspace()
	err := minimizer.minimizeHierarchy(arena, graphID)
	// The workspace goes back to the pool only from a call that returned: one
	// that panics (the original panics on some inputs, e.g. in the crossing
	// counter) may leave any of its scratch half-updated, so it is dropped
	// with the call instead.
	minimizer.releaseWorkspace()
	return err
}

// minimizeHierarchy is the rest of ProcessHierarchy, run with the minimizer's
// workspace (if any) acquired.
func (minimizer *LayerSweepCrossingMinimizer) minimizeHierarchy(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
) error {
	graphsToSweepOn, err := minimizer.initialize(arena, graphID)
	if err != nil {
		return err
	}
	if len(graphsToSweepOn) == 0 {
		return nil
	}
	methodHolder := minimizer.graphInfoHolders[graphsToSweepOn[0]]
	for _, graphIndex := range graphsToSweepOn {
		holder := minimizer.graphInfoHolders[graphIndex]
		if len(holder.CurrentNodeOrder) == 0 {
			continue
		}
		switch {
		case !methodHolder.CrossMinDeterministic():
			minimizer.compareDifferentRandomizedLayouts(arena, graphIndex)
		case methodHolder.CrossMinAlwaysImproves():
			minimizer.minimizeCrossingsNoCounter(arena, graphIndex)
		default:
			minimizer.minimizeCrossingsWithCounter(arena, graphIndex)
		}
		if holder.HasParent {
			minimizer.setPortOrderOnParentGraph(arena, holder)
		}
	}
	minimizer.transferNodeAndPortOrdersToGraph(arena)
	return nil
}

// releaseWorkspace returns the call's workspace to the pool; the holders
// and lists that live in it are dropped first.
func (minimizer *LayerSweepCrossingMinimizer) releaseWorkspace() {
	ws := minimizer.ws
	if ws == nil {
		return
	}
	ws.holderList = minimizer.graphInfoHolders[:0]
	ws.changedList = minimizer.graphsWhoseNodeOrderChanged[:0]
	minimizer.graphInfoHolders, minimizer.graphsWhoseNodeOrderChanged = nil, nil
	minimizer.ws = nil
	releaseCrossMinWorkspace(ws)
}

func (minimizer *LayerSweepCrossingMinimizer) minimizeCrossingsNoCounter(
	arena *layered.LGraphArena,
	graphIndex int,
) {
	holder := minimizer.graphInfoHolders[graphIndex]
	forward := minimizer.random.NextBoolean()
	improved := true
	for improved {
		improved = holder.CrossMinimizer.SetFirstLayerOrder(holder.CurrentNodeOrder, forward)
		improved = minimizer.sweepReducingCrossings(arena, graphIndex, forward, false) || improved
		forward = !forward
	}
	minimizer.setCurrentlyBestNodeOrders(arena)
}

func (minimizer *LayerSweepCrossingMinimizer) ProcessorConfiguration(
	_ *layered.LGraphArena,
	_ layered.LGraphID,
) layered.ProcessorConfiguration {
	var configuration layered.ProcessorConfiguration
	configuration.AddBefore(layered.PhaseNodeOrdering, layered.LongEdgeSplitter)
	configuration.AddBefore(layered.PhaseNodeOrdering, layered.PortListSorter)
	configuration.AddBefore(layered.PhaseNodePlacement, layered.InLayerConstraintProcessor)
	configuration.AddAfter(layered.PhaseEdgeRouting, layered.LongEdgeJoiner)
	return configuration
}

func (minimizer *LayerSweepCrossingMinimizer) initialize(
	arena *layered.LGraphArena,
	rootGraph layered.LGraphID,
) ([]int, error) {
	minimizer.graphInfoHolders = nil
	minimizer.graphsWhoseNodeOrderChanged = nil
	ws := minimizer.ws
	if ws != nil {
		minimizer.graphInfoHolders = ws.holderList[:0]
		minimizer.graphsWhoseNodeOrderChanged = ws.changedList[:0]
	}
	random, found := basegraph.TryGet(&arena.Graph(rootGraph).Properties, layered.Random)
	if !found || random == nil {
		seed := basegraph.Get(&arena.Graph(rootGraph).Properties, layered.RandomSeed)
		random = javacompat.NewRandom(int64(seed))
		basegraph.Set(&arena.Graph(rootGraph).Properties, layered.Random, random)
	}
	minimizer.random = random
	minimizer.randomSeed = random.NextLong()

	// The constraint resolvers' scratch is the workspace's (every use
	// empties it first) or the minimizer's own.
	scratch := minimizer.forsterScratch
	if ws != nil {
		scratch = &ws.forsterScratch
	} else if scratch == nil {
		scratch = &forsterScratch{}
		minimizer.forsterScratch = scratch
	}
	index := &minimizer.index
	if ws != nil {
		index = &ws.index
	}
	index.reset(len(arena.Nodes))
	// ELK prepends each graph not swept into; collect them in discovery order
	// and reverse once at the end, which gives the same list.
	var graphsToSweepOn []int
	var graphs []layered.LGraphID
	if ws != nil {
		graphsToSweepOn = ws.sweepOn[:0]
		graphs = append(ws.graphQueue[:0], rootGraph)
	} else {
		graphsToSweepOn = make([]int, 0)
		graphs = []layered.LGraphID{rootGraph}
	}
	for graphIndex := 0; graphIndex < len(graphs); graphIndex++ {
		currentGraph := graphs[graphIndex]
		arena.Graph(currentGraph).ID = graphIndex
		holder, err := newGraphInfoHolder(
			arena, currentGraph, minimizer.Type, minimizer.graphInfoHolders, random,
			holderContext{scratch: scratch, index: index, owner: int32(graphIndex + 1), ws: ws},
		)
		if err != nil {
			return nil, err
		}
		graphs = append(graphs, holder.ChildGraphs...)
		minimizer.graphInfoHolders = append(minimizer.graphInfoHolders, holder)
		if holder.DontSweepInto() {
			graphsToSweepOn = append(graphsToSweepOn, graphIndex)
		}
	}
	for left, right := 0, len(graphsToSweepOn)-1; left < right; left, right = left+1, right-1 {
		graphsToSweepOn[left], graphsToSweepOn[right] = graphsToSweepOn[right], graphsToSweepOn[left]
	}
	// A holder sweeps into a child graph only through one of its nodes'
	// nested graphs, all of which are in its ChildGraphs.
	for _, holder := range minimizer.graphInfoHolders {
		for _, child := range holder.ChildGraphs {
			if !minimizer.graphInfoHolders[arena.Graph(child).ID].DontSweepInto() {
				holder.sweepsIntoChildren = true
				break
			}
		}
	}
	if ws != nil {
		ws.sweepOn, ws.graphQueue = graphsToSweepOn[:0], graphs[:0]
		// Every scratch ID is final now (see portTable).
		if table := buildPortTable(ws, arena); table != nil {
			skipTrivial := portsStayWithTheirNodes(arena, minimizer.graphInfoHolders)
			for _, holder := range minimizer.graphInfoHolders {
				holder.usePortTable(table)
				if distributor, ok := holder.PortDistributor.(*BarycenterPortDistributor); ok {
					distributor.skipTrivial = skipTrivial
				}
			}
			buildSweepNodes(ws, arena, minimizer.graphInfoHolders)
		}
	}
	return graphsToSweepOn, nil
}

func (minimizer *LayerSweepCrossingMinimizer) compareDifferentRandomizedLayouts(
	arena *layered.LGraphArena,
	graphIndex int,
) {
	minimizer.random.SetSeed(minimizer.randomSeed)
	minimizer.graphsWhoseNodeOrderChanged = minimizer.graphsWhoseNodeOrderChanged[:0]
	holder := minimizer.graphInfoHolders[graphIndex]
	graph := arena.Graph(holder.LGraph)
	nodeInfluence := basegraph.Get(&graph.Properties, modelOrderCrossingCounterNodeInfluence)

	// ELK 0.8.1 checks the node influence twice here. Preserve the typo: the
	// port influence alone must not select the floating-point counter path.
	if nodeInfluence != 0 ||
		basegraph.Get(&graph.Properties, modelOrderCrossingCounterNodeInfluence) != 0 {
		bestCrossings := math.MaxFloat64
		if basegraph.Get(&graph.Properties, layered.ConsiderModelOrder) != layered.OrderingNone {
			holder.setTryWithInitialOrder(arena, layered.FirstTryWithInitialOrder, true)
		}
		for try := 0; try < basegraph.Get(&graph.Properties, layered.Thoroughness); try++ {
			crossings := minimizer.minimizeCrossingsNodePortOrderWithCounter(arena, graphIndex)
			if crossings < bestCrossings {
				bestCrossings = crossings
				minimizer.saveAllNodeOrdersOfChangedGraphs()
				if bestCrossings == 0 {
					break
				}
			}
		}
		return
	}

	bestCrossings := int(^uint(0) >> 1)
	if basegraph.Get(&graph.Properties, layered.ConsiderModelOrder) != layered.OrderingNone {
		holder.setTryWithInitialOrder(arena, layered.FirstTryWithInitialOrder, true)
	}
	for try := 0; try < basegraph.Get(&graph.Properties, layered.Thoroughness); try++ {
		crossings := minimizer.minimizeCrossingsWithCounter(arena, graphIndex)
		if crossings < bestCrossings {
			bestCrossings = crossings
			minimizer.saveAllNodeOrdersOfChangedGraphs()
			if bestCrossings == 0 {
				break
			}
		}
	}
}

func (minimizer *LayerSweepCrossingMinimizer) minimizeCrossingsWithCounter(
	arena *layered.LGraphArena,
	graphIndex int,
) int {
	holder := minimizer.graphInfoHolders[graphIndex]
	graph := arena.Graph(holder.LGraph)
	forward := minimizer.random.NextBoolean()

	// ELK 0.9 keeps a crossing-free model order completely unchanged. A
	// regular first sweep may otherwise shuffle unconnected ports even though
	// the input order is already optimal.
	if holder.tryWithInitialOrder(arena) &&
		minimizer.countCurrentNumberOfCrossings(arena, graphIndex) == 0 {
		return 0
	}
	if (!holder.tryWithInitialOrder(arena) && !holder.tryWithInitialOrder(arena)) ||
		basegraph.Get(&graph.Properties, layered.ConsiderModelOrder) == layered.OrderingNone {
		holder.CrossMinimizer.SetFirstLayerOrder(holder.CurrentNodeOrder, forward)
	} else {
		forward = holder.tryWithInitialOrder(arena)
	}
	minimizer.sweepReducingCrossings(arena, graphIndex, forward, true)

	// These two properties intentionally share one raw key in ELK 0.8.1.
	// Re-read after each mutation instead of caching both values.
	if holder.tryWithInitialOrder(arena) {
		holder.setTryWithInitialOrder(arena, layered.SecondTryWithInitialOrder, false)
	}
	if holder.tryWithInitialOrder(arena) {
		holder.setTryWithInitialOrder(arena, layered.FirstTryWithInitialOrder, false)
		holder.setTryWithInitialOrder(arena, layered.SecondTryWithInitialOrder, true)
	}

	crossingsInGraph := minimizer.countCurrentNumberOfCrossings(arena, graphIndex)
	oldCrossings := crossingsInGraph
	for {
		minimizer.setCurrentlyBestNodeOrders(arena)
		if crossingsInGraph == 0 {
			return 0
		}
		forward = !forward
		oldCrossings = crossingsInGraph
		minimizer.sweepReducingCrossings(arena, graphIndex, forward, false)
		crossingsInGraph = minimizer.countCurrentNumberOfCrossings(arena, graphIndex)
		if oldCrossings <= crossingsInGraph {
			return oldCrossings
		}
	}
}

func (minimizer *LayerSweepCrossingMinimizer) minimizeCrossingsNodePortOrderWithCounter(
	arena *layered.LGraphArena,
	graphIndex int,
) float64 {
	holder := minimizer.graphInfoHolders[graphIndex]
	graph := arena.Graph(holder.LGraph)
	forward := minimizer.random.NextBoolean()

	// See minimizeCrossingsWithCounter. Preserve the initial node and port
	// order when the weighted counter already reports an optimum.
	if holder.tryWithInitialOrder(arena) &&
		minimizer.countCurrentNumberOfCrossingsNodePortOrder(arena, graphIndex) == 0 {
		return 0
	}
	if (!holder.tryWithInitialOrder(arena) && !holder.tryWithInitialOrder(arena)) ||
		basegraph.Get(&graph.Properties, layered.ConsiderModelOrder) == layered.OrderingNone {
		holder.CrossMinimizer.SetFirstLayerOrder(holder.CurrentNodeOrder, forward)
	} else {
		forward = holder.tryWithInitialOrder(arena)
	}
	minimizer.sweepReducingCrossings(arena, graphIndex, forward, true)
	if holder.tryWithInitialOrder(arena) {
		holder.setTryWithInitialOrder(arena, layered.SecondTryWithInitialOrder, false)
	}
	if holder.tryWithInitialOrder(arena) {
		holder.setTryWithInitialOrder(arena, layered.FirstTryWithInitialOrder, false)
		holder.setTryWithInitialOrder(arena, layered.SecondTryWithInitialOrder, true)
	}

	crossingsInGraph := minimizer.countCurrentNumberOfCrossingsNodePortOrder(arena, graphIndex)
	oldCrossings := crossingsInGraph
	for {
		minimizer.setCurrentlyBestNodeOrders(arena)
		if crossingsInGraph == 0 {
			return 0
		}
		forward = !forward
		oldCrossings = crossingsInGraph
		minimizer.sweepReducingCrossings(arena, graphIndex, forward, false)
		crossingsInGraph = minimizer.countCurrentNumberOfCrossingsNodePortOrder(arena, graphIndex)
		if oldCrossings <= crossingsInGraph {
			return oldCrossings
		}
	}
}

func (minimizer *LayerSweepCrossingMinimizer) sweepReducingCrossings(
	arena *layered.LGraphArena,
	graphIndex int,
	forward, firstSweep bool,
) bool {
	holder := minimizer.graphInfoHolders[graphIndex]
	order := holder.CurrentNodeOrder
	length := len(order)
	first := sweepFirstIndex(forward, length)
	// Without a child graph to sweep into, sweepInHierarchicalNodes finds
	// no node to visit in any layer (see sweepsIntoChildren).
	sweepsInto := holder.sweepsIntoChildren
	improved := holder.PortDistributor.DistributePortsWhileSweeping(arena, order, first, forward)
	if sweepsInto {
		improved = minimizer.sweepInHierarchicalNodes(arena, order[first], forward, firstSweep) || improved
	}
	// The try flags are only written between sweeps, never during one (child
	// sweeps read their own graph's flags), so they are read once here.
	preordered := firstSweep && !holder.tryWithInitialOrder(arena) && !holder.tryWithInitialOrder(arena)
	for index := sweepFirstFree(forward, length); sweepNotAtEnd(forward, length, index); index += sweepNext(forward) {
		improved = holder.CrossMinimizer.MinimizeCrossings(order, index, forward, preordered) || improved
		improved = holder.PortDistributor.DistributePortsWhileSweeping(arena, order, index, forward) || improved
		if sweepsInto {
			improved = minimizer.sweepInHierarchicalNodes(arena, order[index], forward, firstSweep) || improved
		}
	}
	minimizer.markGraphChanged(graphIndex)
	return improved
}

func (minimizer *LayerSweepCrossingMinimizer) sweepInHierarchicalNodes(
	arena *layered.LGraphArena,
	layer []layered.LNodeID,
	forward, firstSweep bool,
) bool {
	improved := false
	for _, nodeID := range layer {
		nested := arena.Node(nodeID).NestedGraph
		if nested == nil {
			continue
		}
		childIndex := arena.Graph(*nested).ID
		if minimizer.graphInfoHolders[childIndex].DontSweepInto() {
			continue
		}
		improved = minimizer.sweepInHierarchicalNode(arena, nodeID, forward, firstSweep) || improved
	}
	return improved
}

func (minimizer *LayerSweepCrossingMinimizer) sweepInHierarchicalNode(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	forward, firstSweep bool,
) bool {
	nested := *arena.Node(nodeID).NestedGraph
	childIndex := arena.Graph(nested).ID
	holder := minimizer.graphInfoHolders[childIndex]
	start := sweepFirstIndex(forward, len(holder.CurrentNodeOrder))
	firstNode := holder.CurrentNodeOrder[start][0]
	if arena.Node(firstNode).Type == layered.ExternalPortNode {
		holder.CurrentNodeOrder[start] = minimizer.sortPortDummiesByPortPositions(
			arena, nodeID, holder.CurrentNodeOrder[start], sideOpposedSweep(forward),
		)
		// A node entering the holder's layers with a nested graph keeps the
		// hierarchical visits on (sweepsIntoChildren stays conservative).
		if !holder.sweepsIntoChildren {
			for _, layerNode := range holder.CurrentNodeOrder[start] {
				if arena.Node(layerNode).NestedGraph != nil {
					holder.sweepsIntoChildren = true
				}
			}
		}
	} else {
		holder.CrossMinimizer.SetFirstLayerOrder(holder.CurrentNodeOrder, forward)
	}
	improved := minimizer.sweepReducingCrossings(arena, childIndex, forward, firstSweep)
	minimizer.sortPortsByDummyPositionsInLastLayer(arena, holder.CurrentNodeOrder, *holder.Parent, forward)
	return improved
}

func (minimizer *LayerSweepCrossingMinimizer) sortPortDummiesByPortPositions(
	arena *layered.LGraphArena,
	parent layered.LNodeID,
	layerCloseToNodeEdge []layered.LNodeID,
	side layered.PortSide,
) []layered.LNodeID {
	result := minimizer.ws.nodeIDSlab().take(len(layerCloseToNodeEdge))[:0]
	// portsInNorthSouthEastWestOrder(parent, side) without the reversed copy:
	// the side view is read in place (nothing here changes it).
	view := arena.NodePortSideView(parent, side)
	reversed := side == layered.SouthSide || side == layered.WestSide
	if !reversed && side != layered.NorthSide && side != layered.EastSide {
		view = nil
	}
	for index := range view {
		portID := view[index]
		if reversed {
			portID = view[len(view)-1-index]
		}
		if !minimizer.insideConnections(arena, portID) {
			continue
		}
		if dummyID, found := minimizer.portDummy(arena, portID); found {
			result = append(result, dummyID)
		}
	}
	if len(result) < len(layerCloseToNodeEdge) {
		panic(fmt.Sprintf(
			"p3order: expected %d hierarchical ports, but found only %d",
			len(layerCloseToNodeEdge), len(result),
		))
	}
	return result
}

func (minimizer *LayerSweepCrossingMinimizer) sortPortsByDummyPositionsInLastLayer(
	arena *layered.LGraphArena,
	order [][]layered.LNodeID,
	parent layered.LNodeID,
	onRightMostLayer bool,
) {
	end := sweepEndIndex(onRightMostLayer, len(order))
	lastLayer := order[end]
	j := sweepFirstIndex(onRightMostLayer, len(lastLayer))
	if arena.Node(lastLayer[j]).Type != layered.ExternalPortNode {
		return
	}
	ports := arena.Node(parent).Ports
	for index, portID := range ports {
		port := arena.Port(portID)
		onEndSide := onRightMostLayer && port.Side == layered.EastSide ||
			!onRightMostLayer && port.Side == layered.WestSide
		if !onEndSide || !minimizer.insideConnections(arena, portID) {
			continue
		}
		if j < 0 || j >= len(lastLayer) {
			panic(fmt.Sprintf(
				"p3order: hierarchical port/dummy mismatch for parent %d graph %d side %v: port %d at parent index %d needs dummy index %d in layer %v",
				parent, arena.Node(parent).Graph, port.Side, portID, index, j, lastLayer,
			))
		}
		origin, isPort := minimizer.lportOrigin(arena, lastLayer[j])
		if !isPort {
			panic("p3order: external port dummy has no LPort origin")
		}
		ports[index] = origin
		j += sweepNext(onRightMostLayer)
	}
	arena.Node(parent).Ports = ports
	arena.InvalidatePortSideCache(parent)
}

func (minimizer *LayerSweepCrossingMinimizer) setPortOrderOnParentGraph(
	arena *layered.LGraphArena,
	holder *GraphInfoHolder,
) {
	if !holder.HasExternalPorts {
		return
	}
	best := holder.GetBestSweep()
	if best == nil {
		return
	}
	minimizer.sortPortsByDummyPositionsInLastLayer(arena, best.Nodes(), *holder.Parent, true)
	minimizer.sortPortsByDummyPositionsInLastLayer(arena, best.Nodes(), *holder.Parent, false)
	basegraph.Set(
		&arena.Node(*holder.Parent).Properties,
		layered.PortConstraintsOption,
		layered.PortConstraintsFixedOrder,
	)
	// The parent node belongs to the parent graph's holder, whose
	// distributor may have cached the old value.
	if parentData := holder.ParentGraphData; parentData != nil {
		if distributor, ok := parentData.PortDistributor.(*BarycenterPortDistributor); ok {
			distributor.forgetPortConstraints(*holder.Parent)
		}
	}
}

// portsStayWithTheirNodes reports whether no node can come to list a port
// of another node during the sweeps. Ports change lists only in
// sortPortsByDummyPositionsInLastLayer, which writes into a parent node's
// list the LPort origins of the nodes in its child graph's first or last
// layer; those layers hold their initial nodes, or the port dummies of the
// parent's ports after sortPortDummiesByPortPositions. So it suffices that
// every such node with an LPort origin has a port of the parent as origin.
func portsStayWithTheirNodes(arena *layered.LGraphArena, holders []*GraphInfoHolder) bool {
	originAtParent := func(nodeID layered.LNodeID, parent layered.LNodeID) bool {
		origin, found := basegraph.TryGet(&arena.Node(nodeID).Properties, layered.OriginalElement)
		if !found || origin.Kind != layered.OriginLPort {
			return true // never written (sortPortsByDummyPositionsInLastLayer panics instead)
		}
		if int(origin.LPort) >= len(arena.Ports) {
			return false
		}
		node := arena.Port(origin.LPort).Node
		return node != nil && *node == parent
	}
	for _, holder := range holders {
		if holder.Parent == nil {
			continue
		}
		parent := *holder.Parent
		order := holder.CurrentNodeOrder
		if len(order) > 0 {
			for _, layer := range [2][]layered.LNodeID{order[0], order[len(order)-1]} {
				for _, nodeID := range layer {
					if !originAtParent(nodeID, parent) {
						return false
					}
				}
			}
		}
		for _, portID := range arena.Node(parent).Ports {
			dummyID, found := basegraph.TryGet(&arena.Port(portID).Properties, layered.PortDummy)
			if found && (int(dummyID) >= len(arena.Nodes) || !originAtParent(dummyID, parent)) {
				return false
			}
		}
	}
	return true
}

func (minimizer *LayerSweepCrossingMinimizer) countCurrentNumberOfCrossings(
	arena *layered.LGraphArena,
	graphIndex int,
) int {
	holder := minimizer.graphInfoHolders[graphIndex]
	total := holder.CrossingsCounter.CountAllCrossings(arena, holder.CurrentNodeOrder)
	for _, childGraph := range holder.ChildGraphs {
		childIndex := arena.Graph(childGraph).ID
		if !minimizer.graphInfoHolders[childIndex].DontSweepInto() {
			total += minimizer.countCurrentNumberOfCrossings(arena, childIndex)
		}
	}
	return total
}

func (minimizer *LayerSweepCrossingMinimizer) countCurrentNumberOfCrossingsNodePortOrder(
	arena *layered.LGraphArena,
	graphIndex int,
) float64 {
	holder := minimizer.graphInfoHolders[graphIndex]
	graph := arena.Graph(holder.LGraph)
	modelOrderInfluence := 0.0
	strategy := basegraph.Get(&graph.Properties, layered.ConsiderModelOrder)
	if strategy != layered.OrderingNone {
		modelOrderInfluence += basegraph.Get(&graph.Properties, modelOrderCrossingCounterNodeInfluence) *
			float64(countModelOrderNodeChanges(arena, holder.CurrentNodeOrder, strategy))
		modelOrderInfluence += basegraph.Get(&graph.Properties, modelOrderCrossingCounterPortInfluence) *
			float64(countModelOrderPortChanges(arena, holder.CurrentNodeOrder, strategy,
				basegraph.Get(&graph.Properties, layered.PortModelOrder)))
	}
	total := float64(holder.CrossingsCounter.CountAllCrossings(arena, holder.CurrentNodeOrder)) + modelOrderInfluence
	for _, childGraph := range holder.ChildGraphs {
		childIndex := arena.Graph(childGraph).ID
		if !minimizer.graphInfoHolders[childIndex].DontSweepInto() {
			// ELK calls the integer counting method for children here.
			total += float64(minimizer.countCurrentNumberOfCrossings(arena, childIndex))
		}
	}
	return total
}

func (minimizer *LayerSweepCrossingMinimizer) setCurrentlyBestNodeOrders(
	arena *layered.LGraphArena,
) {
	// No one keeps a reference to a holder's sweep copies beyond the call that
	// reads them, so they are refilled in place instead of being replaced.
	for _, graphIndex := range minimizer.graphsWhoseNodeOrderChanged {
		holder := minimizer.graphInfoHolders[graphIndex]
		if holder.CurrentlyBestNodeAndPortOrder == nil {
			copy := minimizer.ws.sweepCopy()
			copy.fill(arena, holder.CurrentNodeOrder)
			holder.CurrentlyBestNodeAndPortOrder = copy
		} else {
			holder.CurrentlyBestNodeAndPortOrder.fill(arena, holder.CurrentNodeOrder)
		}
	}
}

func (minimizer *LayerSweepCrossingMinimizer) saveAllNodeOrdersOfChangedGraphs() {
	for _, graphIndex := range minimizer.graphsWhoseNodeOrderChanged {
		holder := minimizer.graphInfoHolders[graphIndex]
		if holder.CurrentlyBestNodeAndPortOrder != nil {
			if holder.BestNodeAndPortOrder == nil {
				copy := minimizer.ws.sweepCopy()
				copy.fillFrom(holder.CurrentlyBestNodeAndPortOrder)
				holder.BestNodeAndPortOrder = copy
			} else {
				holder.BestNodeAndPortOrder.fillFrom(holder.CurrentlyBestNodeAndPortOrder)
			}
		}
	}
}

func (minimizer *LayerSweepCrossingMinimizer) transferNodeAndPortOrdersToGraph(
	arena *layered.LGraphArena,
) {
	for _, holder := range minimizer.graphInfoHolders {
		if best := holder.GetBestSweep(); best != nil {
			best.TransferNodeAndPortOrdersToGraph(arena, holder.LGraph, true)
		}
	}
}

func (minimizer *LayerSweepCrossingMinimizer) markGraphChanged(graphIndex int) {
	for _, existing := range minimizer.graphsWhoseNodeOrderChanged {
		if existing == graphIndex {
			return
		}
	}
	minimizer.graphsWhoseNodeOrderChanged = append(minimizer.graphsWhoseNodeOrderChanged, graphIndex)
}

func countModelOrderNodeChanges(
	arena *layered.LGraphArena,
	order [][]layered.LNodeID,
	strategy layered.OrderingStrategy,
) int {
	wrong := 0
	previousLayer := -1
	for _, layer := range order {
		reference := order[0]
		if previousLayer >= 0 {
			reference = order[previousLayer]
		}
		comparator := preserveorder.NewNodeComparator(
			arena, reference, strategy, preserveorder.LongEdgeEqual, false)
		for first := 0; first < len(layer); first++ {
			for second := first + 1; second < len(layer); second++ {
				if basegraph.Has(&arena.Node(layer[first]).Properties, layered.ModelOrder) &&
					basegraph.Has(&arena.Node(layer[second]).Properties, layered.ModelOrder) &&
					comparator.Compare(layer[first], layer[second]) > 0 {
					wrong++
				}
			}
		}
		previousLayer++
	}
	return wrong
}

func countModelOrderPortChanges(
	arena *layered.LGraphArena,
	order [][]layered.LNodeID,
	strategy layered.OrderingStrategy,
	portModelOrder bool,
) int {
	wrong := 0
	previousLayer := -1
	for _, layer := range order {
		reference := order[0]
		if previousLayer >= 0 {
			reference = order[previousLayer]
		}
		for _, nodeID := range layer {
			node := arena.Node(nodeID)
			comparator := preserveorder.NewPortComparator(
				arena,
				reference,
				strategy,
				basegraph.Get(&node.Properties, layered.TargetNodeModelOrder),
				portModelOrder,
			)
			ports := arena.Node(nodeID).Ports
			for first := 0; first < len(ports); first++ {
				for second := first + 1; second < len(ports); second++ {
					if comparator.Compare(ports[first], ports[second]) > 0 {
						wrong++
					}
				}
			}
		}
		previousLayer++
	}
	return wrong
}

func sweepFirstIndex(forward bool, length int) int {
	if forward {
		return 0
	}
	return length - 1
}

func sweepEndIndex(forward bool, length int) int {
	if forward {
		return length - 1
	}
	return 0
}

func sweepFirstFree(forward bool, length int) int {
	if forward {
		return 1
	}
	return length - 2
}

func sweepNext(forward bool) int {
	if forward {
		return 1
	}
	return -1
}

func sweepNotAtEnd(forward bool, length, index int) bool {
	if forward {
		return index < length
	}
	return index >= 0
}

func sideOpposedSweep(forward bool) layered.PortSide {
	if forward {
		return layered.WestSide
	}
	return layered.EastSide
}
