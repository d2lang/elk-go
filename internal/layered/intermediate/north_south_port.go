// Copyright (c) 2011, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

var allowNonFlowPortsToSwitchSides = basegraph.PropertyWithDefault(
	"org.eclipse.elk.layered.allowNonFlowPortsToSwitchSides",
	func() bool { return false },
)

// PreprocessNorthSouthPorts ports ELK 0.8.1's NorthSouthPortPreprocessor.
// Every connected north/south port of a side-fixed normal node is represented
// by an in-layer dummy whose west/east ports carry the original edges through
// crossing minimization and routing.
func PreprocessNorthSouthPorts(arena *layered.LGraphArena, graphID layered.LGraphID) {
	graph := arena.Graph(graphID)
	for _, layerID := range graph.Layers {
		pointer := -1
		nodeSnapshot := append([]layered.LNodeID(nil), arena.Layer(layerID).Nodes...)
		for _, nodeID := range nodeSnapshot {
			pointer++
			node := arena.Node(nodeID)
			constraints := basegraph.Get(&node.Properties, layered.PortConstraintsOption)
			if node.Type != layered.NormalNode || !constraints.IsSideFixed() {
				continue
			}
			if !constraints.IsOrderFixed() {
				sortNorthSouthPortList(arena, nodeID)
			}

			basegraph.Set(&node.Properties, layered.InLayerLayoutUnit, nodeID)
			northDummies := make([]layered.LNodeID, 0)
			southDummies := make([]layered.LNodeID, 0)
			barycenterAssociates := make([]layered.LNodeID, 0)

			northPorts := arena.NodePortsOnSide(nodeID, layered.NorthSide)
			createNorthSouthPortDummies(
				arena, graphID, northPorts, &northDummies, &southDummies, &barycenterAssociates)

			insertPoint := pointer
			for _, dummyID := range northDummies {
				arena.SetNodeLayerAt(dummyID, &layerID, insertPoint)
				pointer++
				basegraph.Set(&arena.Node(dummyID).Properties, layered.InLayerLayoutUnit, nodeID)
				originPort := firstNorthSouthDummyOriginPort(arena, dummyID)
				if !basegraph.Get(&arena.Port(originPort).Properties, allowNonFlowPortsToSwitchSides) {
					appendNodeProperty(&arena.Node(dummyID).Properties,
						layered.InLayerSuccessorConstraints, nodeID)
				}
			}

			southPorts := arena.NodePortsOnSide(nodeID, layered.SouthSide)
			for left, right := 0, len(southPorts)-1; left < right; left, right = left+1, right-1 {
				southPorts[left], southPorts[right] = southPorts[right], southPorts[left]
			}
			createNorthSouthPortDummies(
				arena, graphID, southPorts, &southDummies, nil, &barycenterAssociates)
			for _, dummyID := range southDummies {
				pointer++
				arena.SetNodeLayerAt(dummyID, &layerID, pointer)
				basegraph.Set(&arena.Node(dummyID).Properties, layered.InLayerLayoutUnit, nodeID)
				originPort := firstNorthSouthDummyOriginPort(arena, dummyID)
				if !basegraph.Get(&arena.Port(originPort).Properties, allowNonFlowPortsToSwitchSides) {
					appendNodeProperty(&arena.Node(nodeID).Properties,
						layered.InLayerSuccessorConstraints, dummyID)
				}
			}

			if len(barycenterAssociates) != 0 {
				basegraph.Set(&arena.Node(nodeID).Properties, layered.BarycenterAssociates, barycenterAssociates)
			}
		}
	}
}

func appendNodeProperty(
	properties *basegraph.PropertyMap,
	property basegraph.Property[[]layered.LNodeID],
	nodeID layered.LNodeID,
) {
	values := basegraph.Get(properties, property)
	values = append(values, nodeID)
	basegraph.Set(properties, property, values)
}

func firstNorthSouthDummyOriginPort(arena *layered.LGraphArena, dummyID layered.LNodeID) layered.LPortID {
	ports := arena.Node(dummyID).Ports
	if len(ports) == 0 {
		panic("layered: north/south port dummy has no ports")
	}
	origin, found := basegraph.TryGet(&arena.Port(ports[0]).Properties, layered.OriginalElement)
	if !found || origin.Kind != layered.OriginLPort {
		panic("layered: north/south dummy port has no LPort origin")
	}
	return origin.LPort
}

func sortNorthSouthPortList(arena *layered.LGraphArena, nodeID layered.LNodeID) {
	node := arena.Node(nodeID)
	portCount := len(node.Ports)
	nextInput, nextInputOutput, nextOutput := 0, portCount, 2*portCount
	for _, portID := range node.Ports {
		port := arena.Port(portID)
		switch port.Side {
		case layered.EastSide, layered.WestSide:
			port.ID = -1
		case layered.NorthSide, layered.SouthSide:
			hasIncoming := len(port.IncomingEdges) > 0
			hasOutgoing := len(port.OutgoingEdges) > 0
			switch {
			case hasIncoming && hasOutgoing:
				port.ID = nextInputOutput
				nextInputOutput++
			case hasIncoming || !hasOutgoing:
				port.ID = nextInput
				nextInput++
			default:
				port.ID = nextOutput
				nextOutput++
			}
		}
	}

	gwtStableSort(node.Ports, func(firstID, secondID layered.LPortID) int {
		first, second := arena.Port(firstID), arena.Port(secondID)
		if first.Side != second.Side {
			return int(first.Side) - int(second.Side)
		}
		if first.ID == second.ID {
			return 0
		}
		if first.Side == layered.NorthSide {
			return first.ID - second.ID
		}
		return second.ID - first.ID
	})
	arena.InvalidatePortSideCache(nodeID)
}

func createNorthSouthPortDummies(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	ports []layered.LPortID,
	dummyNodes *[]layered.LNodeID,
	opposingSideDummyNodes *[]layered.LNodeID,
	barycenterAssociates *[]layered.LNodeID,
) {
	inputPorts := make([]layered.LPortID, 0, len(ports))
	outputPorts := make([]layered.LPortID, 0, len(ports))
	inputOutputPorts := make([]layered.LPortID, 0, len(ports))
	sameSideLoops := make([]layered.LEdgeID, 0, len(ports))
	northSouthLoops := make([]layered.LEdgeID, 0, len(ports))

	for _, portID := range ports {
		port := arena.Port(portID)
		for _, edgeID := range append([]layered.LEdgeID(nil), port.OutgoingEdges...) {
			edge := arena.Edge(edgeID)
			if edge.Source == nil || edge.Target == nil || arena.EdgeSourceNode(edgeID) != arena.EdgeTargetNode(edgeID) {
				continue
			}
			targetSide := arena.Port(*edge.Target).Side
			if port.Side == targetSide {
				sameSideLoops = append(sameSideLoops, edgeID)
			} else if port.Side == layered.NorthSide && targetSide == layered.SouthSide {
				northSouthLoops = append(northSouthLoops, edgeID)
			}
		}
	}

	for _, edgeID := range northSouthLoops {
		if opposingSideDummyNodes == nil {
			panic("layered: north/south self-loop encountered while processing south ports")
		}
		createNorthSouthSelfLoopDummies(
			arena, graphID, edgeID, dummyNodes, opposingSideDummyNodes, layered.EastSide)
	}
	for _, edgeID := range sameSideLoops {
		createSameSideSelfLoopDummy(arena, graphID, edgeID, dummyNodes)
	}

	for _, portID := range ports {
		port := arena.Port(portID)
		hasIncoming := len(port.IncomingEdges) > 0
		hasOutgoing := len(port.OutgoingEdges) > 0
		switch {
		case hasIncoming && hasOutgoing:
			inputOutputPorts = append(inputOutputPorts, portID)
		case hasIncoming:
			inputPorts = append(inputPorts, portID)
		case hasOutgoing:
			outputPorts = append(outputPorts, portID)
		}
	}

	// ELK 0.8.1's USE_NEW_APPROACH is a compile-time true constant: each
	// remaining input/output port receives its own unconstrained dummy.
	for _, portID := range inputPorts {
		dummyID := createNorthSouthPortDummy(arena, graphID, &portID, nil, dummyNodes)
		*barycenterAssociates = append(*barycenterAssociates, dummyID)
	}
	for _, portID := range outputPorts {
		dummyID := createNorthSouthPortDummy(arena, graphID, nil, &portID, dummyNodes)
		*barycenterAssociates = append(*barycenterAssociates, dummyID)
	}
	for _, portID := range inputOutputPorts {
		dummyID := createNorthSouthPortDummy(arena, graphID, &portID, &portID, dummyNodes)
		*barycenterAssociates = append(*barycenterAssociates, dummyID)
	}
}

func createNorthSouthPortDummy(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	inputPort, outputPort *layered.LPortID,
	dummyNodes *[]layered.LNodeID,
) layered.LNodeID {
	dummyID := arena.AddNode(graphID)
	dummy := arena.Node(dummyID)
	dummy.Type = layered.NorthSouthPortNode
	basegraph.Set(&dummy.Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedPosition)
	crossingHint := 0

	if inputPort != nil {
		dummyPort := arena.AddPort()
		basegraph.Set(&arena.Port(dummyPort).Properties, layered.OriginalElement, layered.LPortOrigin(*inputPort))
		basegraph.Set(&dummy.Properties, layered.OriginalElement, layered.LNodeOrigin(*arena.Port(*inputPort).Node))
		arena.SetPortSide(dummyPort, layered.WestSide)
		arena.SetPortNode(dummyPort, &dummyID)
		for _, edgeID := range append([]layered.LEdgeID(nil), arena.Port(*inputPort).IncomingEdges...) {
			arena.SetEdgeTarget(edgeID, &dummyPort)
		}
		basegraph.Set(&arena.Port(*inputPort).Properties, layered.PortDummy, dummyID)
		crossingHint++
	}

	if outputPort != nil {
		dummyPort := arena.AddPort()
		basegraph.Set(&dummy.Properties, layered.OriginalElement, layered.LNodeOrigin(*arena.Port(*outputPort).Node))
		basegraph.Set(&arena.Port(dummyPort).Properties, layered.OriginalElement, layered.LPortOrigin(*outputPort))
		arena.SetPortSide(dummyPort, layered.EastSide)
		arena.SetPortNode(dummyPort, &dummyID)
		for _, edgeID := range append([]layered.LEdgeID(nil), arena.Port(*outputPort).OutgoingEdges...) {
			arena.SetEdgeSource(edgeID, &dummyPort)
		}
		basegraph.Set(&arena.Port(*outputPort).Properties, layered.PortDummy, dummyID)
		crossingHint++
	}

	basegraph.Set(&dummy.Properties, layered.CrossingHint, crossingHint)
	*dummyNodes = append(*dummyNodes, dummyID)
	return dummyID
}

func createSameSideSelfLoopDummy(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	edgeID layered.LEdgeID,
	dummyNodes *[]layered.LNodeID,
) {
	edge := arena.Edge(edgeID)
	dummyID := arena.AddNode(graphID)
	dummy := arena.Node(dummyID)
	dummy.Type = layered.NorthSouthPortNode
	basegraph.Set(&dummy.Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedPosition)
	basegraph.Set(&dummy.Properties, layered.OriginalElement, layered.LEdgeOrigin(edgeID))

	input := arena.AddPort()
	basegraph.Set(&arena.Port(input).Properties, layered.OriginalElement, layered.LPortOrigin(*edge.Target))
	arena.SetPortSide(input, layered.WestSide)
	arena.SetPortNode(input, &dummyID)
	output := arena.AddPort()
	basegraph.Set(&arena.Port(output).Properties, layered.OriginalElement, layered.LPortOrigin(*edge.Source))
	arena.SetPortSide(output, layered.EastSide)
	arena.SetPortNode(output, &dummyID)

	basegraph.Set(&arena.Port(*edge.Source).Properties, layered.PortDummy, dummyID)
	basegraph.Set(&arena.Port(*edge.Target).Properties, layered.PortDummy, dummyID)
	arena.SetEdgeSource(edgeID, nil)
	arena.SetEdgeTarget(edgeID, nil)
	*dummyNodes = append(*dummyNodes, dummyID)
	basegraph.Set(&dummy.Properties, layered.CrossingHint, 2)
}

func createNorthSouthSelfLoopDummies(
	arena *layered.LGraphArena,
	graphID layered.LGraphID,
	edgeID layered.LEdgeID,
	northDummies, southDummies *[]layered.LNodeID,
	portSide layered.PortSide,
) {
	edge := arena.Edge(edgeID)
	originalSource, originalTarget := *edge.Source, *edge.Target
	originalNode := *arena.Port(originalSource).Node

	northDummyID := arena.AddNode(graphID)
	northDummy := arena.Node(northDummyID)
	northDummy.Type = layered.NorthSouthPortNode
	basegraph.Set(&northDummy.Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedPosition)
	basegraph.Set(&northDummy.Properties, layered.OriginalElement, layered.LNodeOrigin(originalNode))
	northPort := arena.AddPort()
	basegraph.Set(&arena.Port(northPort).Properties, layered.OriginalElement, layered.LPortOrigin(originalSource))
	arena.SetPortSide(northPort, portSide)
	arena.SetPortNode(northPort, &northDummyID)
	basegraph.Set(&arena.Port(originalSource).Properties, layered.PortDummy, northDummyID)

	southDummyID := arena.AddNode(graphID)
	southDummy := arena.Node(southDummyID)
	southDummy.Type = layered.NorthSouthPortNode
	basegraph.Set(&southDummy.Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedPosition)
	basegraph.Set(&southDummy.Properties, layered.OriginalElement, layered.LNodeOrigin(originalNode))
	southPort := arena.AddPort()
	basegraph.Set(&arena.Port(southPort).Properties, layered.OriginalElement, layered.LPortOrigin(originalTarget))
	arena.SetPortSide(southPort, portSide)
	arena.SetPortNode(southPort, &southDummyID)
	basegraph.Set(&arena.Port(originalTarget).Properties, layered.PortDummy, southDummyID)

	arena.SetEdgeSource(edgeID, &northPort)
	arena.SetEdgeTarget(edgeID, &southPort)
	*northDummies = append([]layered.LNodeID{northDummyID}, *northDummies...)
	*southDummies = append(*southDummies, southDummyID)
	basegraph.Set(&arena.Node(northDummyID).Properties, layered.CrossingHint, 1)
	basegraph.Set(&southDummy.Properties, layered.CrossingHint, 1)
}
