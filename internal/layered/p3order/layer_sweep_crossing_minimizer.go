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
	random, found := basegraph.TryGet(&arena.Graph(rootGraph).Properties, layered.Random)
	if !found || random == nil {
		seed := basegraph.Get(&arena.Graph(rootGraph).Properties, layered.RandomSeed)
		random = javacompat.NewRandom(int64(seed))
		basegraph.Set(&arena.Graph(rootGraph).Properties, layered.Random, random)
	}
	minimizer.random = random
	minimizer.randomSeed = random.NextLong()

	graphsToSweepOn := make([]int, 0)
	graphs := []layered.LGraphID{rootGraph}
	for index := 0; index < len(graphs); index++ {
		currentGraph := graphs[index]
		arena.Graph(currentGraph).ID = index
		holder, err := NewGraphInfoHolder(
			arena, currentGraph, minimizer.Type, minimizer.graphInfoHolders, random,
		)
		if err != nil {
			return nil, err
		}
		graphs = append(graphs, holder.ChildGraphs...)
		minimizer.graphInfoHolders = append(minimizer.graphInfoHolders, holder)
		if holder.DontSweepInto() {
			graphsToSweepOn = append([]int{index}, graphsToSweepOn...)
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
			basegraph.Set(&graph.Properties, layered.FirstTryWithInitialOrder, true)
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
		basegraph.Set(&graph.Properties, layered.FirstTryWithInitialOrder, true)
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
	if basegraph.Get(&graph.Properties, layered.FirstTryWithInitialOrder) &&
		minimizer.countCurrentNumberOfCrossings(arena, graphIndex) == 0 {
		return 0
	}
	if (!basegraph.Get(&graph.Properties, layered.FirstTryWithInitialOrder) &&
		!basegraph.Get(&graph.Properties, layered.SecondTryWithInitialOrder)) ||
		basegraph.Get(&graph.Properties, layered.ConsiderModelOrder) == layered.OrderingNone {
		holder.CrossMinimizer.SetFirstLayerOrder(holder.CurrentNodeOrder, forward)
	} else {
		forward = basegraph.Get(&graph.Properties, layered.FirstTryWithInitialOrder)
	}
	minimizer.sweepReducingCrossings(arena, graphIndex, forward, true)

	// These two properties intentionally share one raw key in ELK 0.8.1.
	// Re-read after each mutation instead of caching both values.
	if basegraph.Get(&graph.Properties, layered.SecondTryWithInitialOrder) {
		basegraph.Set(&graph.Properties, layered.SecondTryWithInitialOrder, false)
	}
	if basegraph.Get(&graph.Properties, layered.FirstTryWithInitialOrder) {
		basegraph.Set(&graph.Properties, layered.FirstTryWithInitialOrder, false)
		basegraph.Set(&graph.Properties, layered.SecondTryWithInitialOrder, true)
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
	if basegraph.Get(&graph.Properties, layered.FirstTryWithInitialOrder) &&
		minimizer.countCurrentNumberOfCrossingsNodePortOrder(arena, graphIndex) == 0 {
		return 0
	}
	if (!basegraph.Get(&graph.Properties, layered.FirstTryWithInitialOrder) &&
		!basegraph.Get(&graph.Properties, layered.SecondTryWithInitialOrder)) ||
		basegraph.Get(&graph.Properties, layered.ConsiderModelOrder) == layered.OrderingNone {
		holder.CrossMinimizer.SetFirstLayerOrder(holder.CurrentNodeOrder, forward)
	} else {
		forward = basegraph.Get(&graph.Properties, layered.FirstTryWithInitialOrder)
	}
	minimizer.sweepReducingCrossings(arena, graphIndex, forward, true)
	if basegraph.Get(&graph.Properties, layered.SecondTryWithInitialOrder) {
		basegraph.Set(&graph.Properties, layered.SecondTryWithInitialOrder, false)
	}
	if basegraph.Get(&graph.Properties, layered.FirstTryWithInitialOrder) {
		basegraph.Set(&graph.Properties, layered.FirstTryWithInitialOrder, false)
		basegraph.Set(&graph.Properties, layered.SecondTryWithInitialOrder, true)
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
	improved := holder.PortDistributor.DistributePortsWhileSweeping(arena, order, first, forward)
	improved = minimizer.sweepInHierarchicalNodes(arena, order[first], forward, firstSweep) || improved
	for index := sweepFirstFree(forward, length); sweepNotAtEnd(forward, length, index); index += sweepNext(forward) {
		graph := arena.Graph(holder.LGraph)
		preordered := firstSweep &&
			!basegraph.Get(&graph.Properties, layered.FirstTryWithInitialOrder) &&
			!basegraph.Get(&graph.Properties, layered.SecondTryWithInitialOrder)
		improved = holder.CrossMinimizer.MinimizeCrossings(order, index, forward, preordered) || improved
		improved = holder.PortDistributor.DistributePortsWhileSweeping(arena, order, index, forward) || improved
		improved = minimizer.sweepInHierarchicalNodes(arena, order[index], forward, firstSweep) || improved
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
	result := make([]layered.LNodeID, 0, len(layerCloseToNodeEdge))
	for _, portID := range portsInNorthSouthEastWestOrder(arena, parent, side) {
		if !basegraph.Get(&arena.Port(portID).Properties, layered.InsideConnections) {
			continue
		}
		dummyID, found := basegraph.TryGet(&arena.Port(portID).Properties, layered.PortDummy)
		if found {
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
		if !onEndSide || !basegraph.Get(&port.Properties, layered.InsideConnections) {
			continue
		}
		if j < 0 || j >= len(lastLayer) {
			panic(fmt.Sprintf(
				"p3order: hierarchical port/dummy mismatch for parent %d graph %d side %v: port %d at parent index %d needs dummy index %d in layer %v",
				parent, arena.Node(parent).Graph, port.Side, portID, index, j, lastLayer,
			))
		}
		origin, found := basegraph.TryGet(&arena.Node(lastLayer[j]).Properties, layered.OriginalElement)
		if !found || origin.Kind != layered.OriginLPort {
			panic("p3order: external port dummy has no LPort origin")
		}
		ports[index] = origin.LPort
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
	for _, graphIndex := range minimizer.graphsWhoseNodeOrderChanged {
		holder := minimizer.graphInfoHolders[graphIndex]
		holder.CurrentlyBestNodeAndPortOrder = NewSweepCopy(arena, holder.CurrentNodeOrder)
	}
}

func (minimizer *LayerSweepCrossingMinimizer) saveAllNodeOrdersOfChangedGraphs() {
	for _, graphIndex := range minimizer.graphsWhoseNodeOrderChanged {
		holder := minimizer.graphInfoHolders[graphIndex]
		if holder.CurrentlyBestNodeAndPortOrder != nil {
			holder.BestNodeAndPortOrder = holder.CurrentlyBestNodeAndPortOrder.Clone()
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
