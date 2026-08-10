// Copyright (c) 2011, 2015 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"reflect"
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestNorthSouthPortPreAndPostprocessOutputPort(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	basegraph.Set(&arena.Graph(graphID).Properties, layered.EdgeRoutingOption, layered.EdgeRoutingOrthogonal)
	source, target := arena.AddNode(graphID), arena.AddNode(graphID)
	basegraph.Set(&arena.Node(source).Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedPosition)
	sourceLayer, targetLayer := arena.AddLayerToGraph(graphID), arena.AddLayerToGraph(graphID)
	arena.SetNodeLayer(source, &sourceLayer)
	arena.SetNodeLayer(target, &targetLayer)
	sourcePort, targetPort := arena.AddPort(), arena.AddPort()
	arena.SetPortNode(sourcePort, &source)
	arena.SetPortNode(targetPort, &target)
	arena.SetPortSide(sourcePort, layered.SouthSide)
	arena.SetPortSide(targetPort, layered.WestSide)
	arena.Node(source).Position = geometry.Point{X: 10, Y: 20}
	arena.Port(sourcePort).Position = geometry.Point{X: 40, Y: 50}
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &sourcePort)
	arena.SetEdgeTarget(edgeID, &targetPort)

	PreprocessNorthSouthPorts(arena, graphID)

	if got := arena.Layer(sourceLayer).Nodes; len(got) != 2 || got[0] != source {
		t.Fatalf("source layer = %v, want source followed by dummy", got)
	}
	dummyID := arena.Layer(sourceLayer).Nodes[1]
	if arena.Node(dummyID).Type != layered.NorthSouthPortNode {
		t.Fatalf("dummy type = %v", arena.Node(dummyID).Type)
	}
	if arena.Edge(edgeID).Source == nil || arena.Port(*arena.Edge(edgeID).Source).Node == nil ||
		*arena.Port(*arena.Edge(edgeID).Source).Node != dummyID {
		t.Fatalf("edge source was not rerouted to dummy: %v", arena.Edge(edgeID).Source)
	}
	if got := basegraph.Get(&arena.Port(sourcePort).Properties, layered.PortDummy); got != dummyID {
		t.Fatalf("source port dummy = %v, want %v", got, dummyID)
	}
	if got := basegraph.Get(&arena.Node(source).Properties, layered.InLayerSuccessorConstraints); len(got) != 1 || got[0] != dummyID {
		t.Fatalf("source successor constraints = %v, want [%v]", got, dummyID)
	}
	if got := basegraph.Get(&arena.Node(source).Properties, layered.BarycenterAssociates); len(got) != 1 || got[0] != dummyID {
		t.Fatalf("barycenter associates = %v, want [%v]", got, dummyID)
	}

	arena.Node(dummyID).Position = geometry.Point{X: 30, Y: 70}
	arena.Edge(edgeID).BendPoints = geometry.Chain{{X: 20, Y: 30}}
	PostprocessNorthSouthPorts(arena, graphID)

	if arena.Edge(edgeID).Source == nil || *arena.Edge(edgeID).Source != sourcePort {
		t.Fatalf("restored source = %v, want %v", arena.Edge(edgeID).Source, sourcePort)
	}
	wantBends := geometry.Chain{{X: 50, Y: 70}, {X: 20, Y: 30}}
	if got := arena.Edge(edgeID).BendPoints; !reflect.DeepEqual(got, wantBends) {
		t.Fatalf("bend points = %v, want %v", got, wantBends)
	}
	if arena.Node(dummyID).Layer != nil || len(arena.Layer(sourceLayer).Nodes) != 1 {
		t.Fatalf("dummy remains in source layer: layer=%v nodes=%v",
			arena.Node(dummyID).Layer, arena.Layer(sourceLayer).Nodes)
	}
}

func TestNorthSouthPortInOutDummyAddsJunctionPoints(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	basegraph.Set(&arena.Graph(graphID).Properties, layered.EdgeRoutingOption, layered.EdgeRoutingOrthogonal)
	upper, node, lower := arena.AddNode(graphID), arena.AddNode(graphID), arena.AddNode(graphID)
	basegraph.Set(&arena.Node(node).Properties, layered.PortConstraintsOption, layered.PortConstraintsFixedPosition)
	upperLayer, middleLayer, lowerLayer := arena.AddLayerToGraph(graphID), arena.AddLayerToGraph(graphID), arena.AddLayerToGraph(graphID)
	arena.SetNodeLayer(upper, &upperLayer)
	arena.SetNodeLayer(node, &middleLayer)
	arena.SetNodeLayer(lower, &lowerLayer)
	upperPort, northPort, lowerPort := arena.AddPort(), arena.AddPort(), arena.AddPort()
	arena.SetPortNode(upperPort, &upper)
	arena.SetPortNode(northPort, &node)
	arena.SetPortNode(lowerPort, &lower)
	arena.SetPortSide(northPort, layered.NorthSide)
	arena.Node(node).Position.X = 10
	arena.Port(northPort).Position.X = 50
	inEdge, outEdge := arena.AddEdge(), arena.AddEdge()
	arena.SetEdgeSource(inEdge, &upperPort)
	arena.SetEdgeTarget(inEdge, &northPort)
	arena.SetEdgeSource(outEdge, &northPort)
	arena.SetEdgeTarget(outEdge, &lowerPort)

	PreprocessNorthSouthPorts(arena, graphID)
	if got := arena.Layer(middleLayer).Nodes; len(got) != 2 || got[1] != node {
		t.Fatalf("middle layer = %v, want dummy followed by node", got)
	}
	dummyID := arena.Layer(middleLayer).Nodes[0]
	if len(arena.Node(dummyID).Ports) != 2 {
		t.Fatalf("dummy ports = %v, want two", arena.Node(dummyID).Ports)
	}
	arena.Node(dummyID).Position.Y = 5
	arena.Edge(inEdge).BendPoints = geometry.Chain{{X: 1, Y: 2}}
	arena.Edge(outEdge).BendPoints = geometry.Chain{{X: 3, Y: 4}}

	PostprocessNorthSouthPorts(arena, graphID)

	if arena.Edge(inEdge).Target == nil || *arena.Edge(inEdge).Target != northPort ||
		arena.Edge(outEdge).Source == nil || *arena.Edge(outEdge).Source != northPort {
		t.Fatalf("edges were not restored: in target=%v out source=%v",
			arena.Edge(inEdge).Target, arena.Edge(outEdge).Source)
	}
	point := geometry.Point{X: 60, Y: 5}
	if got := arena.Edge(inEdge).BendPoints; !reflect.DeepEqual(got, geometry.Chain{{X: 1, Y: 2}, point}) {
		t.Fatalf("incoming bends = %v", got)
	}
	if got := arena.Edge(outEdge).BendPoints; !reflect.DeepEqual(got, geometry.Chain{point, {X: 3, Y: 4}}) {
		t.Fatalf("outgoing bends = %v", got)
	}
	for _, edgeID := range []layered.LEdgeID{inEdge, outEdge} {
		junctions, found := basegraph.TryGet(&arena.Edge(edgeID).Properties, layered.JunctionPoints)
		if !found || !reflect.DeepEqual(junctions, geometry.Chain{point}) {
			t.Fatalf("edge %v junctions = %v, found=%v", edgeID, junctions, found)
		}
	}
}

func TestNorthSouthPortSorterUsesFlowGroups(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	node := arena.AddNode(graphID)
	input, output, inputOutput, unconnected, east := arena.AddPort(), arena.AddPort(), arena.AddPort(), arena.AddPort(), arena.AddPort()
	for _, portID := range []layered.LPortID{output, inputOutput, east, unconnected, input} {
		arena.SetPortNode(portID, &node)
	}
	for _, portID := range []layered.LPortID{input, output, inputOutput, unconnected} {
		arena.SetPortSide(portID, layered.NorthSide)
	}
	arena.SetPortSide(east, layered.EastSide)
	remote := arena.AddNode(graphID)
	remoteIn, remoteOut := arena.AddPort(), arena.AddPort()
	arena.SetPortNode(remoteIn, &remote)
	arena.SetPortNode(remoteOut, &remote)
	first, second, third, fourth := arena.AddEdge(), arena.AddEdge(), arena.AddEdge(), arena.AddEdge()
	arena.SetEdgeSource(first, &remoteOut)
	arena.SetEdgeTarget(first, &input)
	arena.SetEdgeSource(second, &output)
	arena.SetEdgeTarget(second, &remoteIn)
	arena.SetEdgeSource(third, &remoteOut)
	arena.SetEdgeTarget(third, &inputOutput)
	arena.SetEdgeSource(fourth, &inputOutput)
	arena.SetEdgeTarget(fourth, &remoteIn)

	sortNorthSouthPortList(arena, node)

	want := []layered.LPortID{unconnected, input, inputOutput, output, east}
	if got := arena.Node(node).Ports; !reflect.DeepEqual(got, want) {
		t.Fatalf("sorted ports = %v, want %v", got, want)
	}
}

func TestModelOrderNorthSouthInputReversingMatchesGuavaReverseView(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	node := arena.AddNode(graphID)
	remote := arena.AddNode(graphID)
	in1, out1, in2, out2, unconnected :=
		arena.AddPort(), arena.AddPort(), arena.AddPort(), arena.AddPort(), arena.AddPort()
	for _, portID := range []layered.LPortID{in1, out1, in2, out2, unconnected} {
		arena.SetPortNode(portID, &node)
	}
	remoteIn1, remoteOut1, remoteIn2, remoteOut2 :=
		arena.AddPort(), arena.AddPort(), arena.AddPort(), arena.AddPort()
	for _, portID := range []layered.LPortID{remoteIn1, remoteOut1, remoteIn2, remoteOut2} {
		arena.SetPortNode(portID, &remote)
	}
	for _, endpoints := range [][2]layered.LPortID{
		{remoteOut1, in1}, {out1, remoteIn1},
		{remoteOut2, in2}, {out2, remoteIn2},
	} {
		edgeID := arena.AddEdge()
		arena.SetEdgeSource(edgeID, &endpoints[0])
		arena.SetEdgeTarget(edgeID, &endpoints[1])
	}

	input := []layered.LPortID{in1, out1, in2, out2, unconnected}
	want := []layered.LPortID{unconnected, out2, out1, in1, in2}
	if got := modelOrderNorthSouthInputReversing(arena, input); !reflect.DeepEqual(got, want) {
		t.Fatalf("model-order north/south ports = %v, want %v", got, want)
	}
}
