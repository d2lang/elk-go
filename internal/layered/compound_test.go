// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package layered

import (
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
)

func TestExternalPortDummyMatchesSideGeometry(t *testing.T) {
	arena := NewLGraphArena()
	graphID := arena.AddGraph()
	var holder basegraph.PropertyMap
	basegraph.Set(&holder, PortBorderOffset, -2.0)
	dummy := arena.CreateExternalPortDummy(&holder, PortConstraintsFixedSide, EastSide,
		1, nil, nil, geometry.Point{X: 8, Y: 6}, DirectionRight, graphID)
	port := arena.Port(arena.Node(dummy).Ports[0])

	if arena.Node(dummy).Type != ExternalPortNode || arena.Node(dummy).Size != (geometry.Point{X: 2, Y: 6}) {
		t.Fatalf("dummy type/size = %v/%+v", arena.Node(dummy).Type, arena.Node(dummy).Size)
	}
	if port.Side != WestSide || port.Position != (geometry.Point{Y: 3}) {
		t.Fatalf("dummy port side/position = %v/%+v", port.Side, port.Position)
	}
	if got := basegraph.Get(&arena.Node(dummy).Properties, LayerConstraintOption); got != LayerConstraintLastSeparate {
		t.Fatalf("layer constraint = %v", got)
	}
	if got := basegraph.Get(&arena.Node(dummy).Properties, ExternalPortSide); got != EastSide {
		t.Fatalf("external side = %v", got)
	}
}

func TestExternalDummyForBoundarySurvivesPortSliceReallocation(t *testing.T) {
	arena := NewLGraphArena()
	parentGraph := arena.AddGraph()
	childGraph := arena.AddGraph()
	parent := arena.AddNode(parentGraph)
	basegraph.Set(&arena.Graph(childGraph).Properties, DirectionOption, DirectionRight)

	boundary := arena.AddPort()
	arena.SetPortNode(boundary, &parent)
	basegraph.Set(&arena.Port(boundary).Properties, PortBorderOffset, 3.0)
	// Force the dummy port append to replace the backing array. This catches
	// stale pointers into arena.Ports deterministically.
	ports := make([]LPort, len(arena.Ports))
	copy(ports, arena.Ports)
	arena.Ports = ports

	edge := arena.AddEdge()
	otherNode := arena.AddNode(parentGraph)
	otherPort := arena.AddPort()
	arena.SetPortNode(otherPort, &otherNode)
	arena.SetEdgeSource(edge, &boundary)
	arena.SetEdgeTarget(edge, &otherPort)
	dummy := externalDummyForBoundary(arena, childGraph, parent, boundary, PortTypeOutput, edge)
	port := arena.Port(boundary)
	if got, found := basegraph.TryGet(&port.Properties, PortDummy); !found || got != dummy {
		t.Fatalf("boundary port dummy = %v, present=%v; want %v", got, found, dummy)
	}
	if !basegraph.Get(&port.Properties, InsideConnections) {
		t.Fatal("boundary port lost inside-connections marker after reallocation")
	}
	if got := basegraph.Get(&port.Properties, PortSideOption); got != EastSide {
		t.Fatalf("boundary property side = %v, want EAST", got)
	}
	if port.Side != EastSide {
		t.Fatalf("boundary field side = %v, want EAST", port.Side)
	}
}

func TestCompoundParentEndPortUsesConnectionNetFlow(t *testing.T) {
	arena := NewLGraphArena()
	top := arena.AddGraph()
	basegraph.Set(&arena.Graph(top).Properties, DirectionOption, DirectionDown)
	basegraph.Set(&arena.Graph(top).Properties, Hierarchy, HierarchyIncludeChildren)

	ancestor := arena.AddNode(top)
	arena.Graph(top).LayerlessNodes = append(arena.Graph(top).LayerlessNodes, ancestor)
	childGraph := arena.AddGraph()
	arena.Node(ancestor).NestedGraph = &childGraph
	arena.Graph(childGraph).ParentNode = &ancestor
	basegraph.Set(&arena.Graph(childGraph).Properties, DirectionOption, DirectionDown)

	intermediate := arena.AddNode(childGraph)
	arena.Graph(childGraph).LayerlessNodes = append(arena.Graph(childGraph).LayerlessNodes, intermediate)
	grandchildGraph := arena.AddGraph()
	arena.Node(intermediate).NestedGraph = &grandchildGraph
	arena.Graph(grandchildGraph).ParentNode = &intermediate
	basegraph.Set(&arena.Graph(grandchildGraph).Properties, DirectionOption, DirectionDown)

	descendant := arena.AddNode(grandchildGraph)
	arena.Graph(grandchildGraph).LayerlessNodes = append(arena.Graph(grandchildGraph).LayerlessNodes, descendant)
	sourcePort := arena.AddPort()
	targetPort := arena.AddPort()
	arena.SetPortNode(sourcePort, &ancestor)
	arena.SetPortNode(targetPort, &descendant)
	original := arena.AddEdge()
	arena.SetEdgeSource(original, &sourcePort)
	arena.SetEdgeTarget(original, &targetPort)

	if got := compoundPortNetFlow(arena, sourcePort); got != 1 {
		t.Fatalf("ancestor source net flow = %d, want 1", got)
	}
	if err := (&CompoundGraphPreprocessor{}).Process(arena, top); err != nil {
		t.Fatal(err)
	}
	dummy, found := basegraph.TryGet(&arena.Port(sourcePort).Properties, PortDummy)
	if !found {
		t.Fatal("ancestor source port has no hierarchy dummy")
	}
	if got := basegraph.Get(&arena.Node(dummy).Properties, ExternalPortSide); got != SouthSide {
		t.Fatalf("ancestor source dummy side = %v, want SOUTH", got)
	}
	if got := arena.Port(sourcePort).Side; got != SouthSide {
		t.Fatalf("ancestor source side = %v, want SOUTH", got)
	}
}

func TestCompoundPreprocessorCreatesDummiesForEveryCompoundPort(t *testing.T) {
	arena := NewLGraphArena()
	top := arena.AddGraph()
	parent := arena.AddNode(top)
	arena.Graph(top).LayerlessNodes = append(arena.Graph(top).LayerlessNodes, parent)
	child := arena.AddGraph()
	arena.Graph(child).ParentNode = &parent
	arena.Node(parent).NestedGraph = &child
	basegraph.Set(&arena.Graph(child).Properties, DirectionOption, DirectionDown)
	basegraph.Set(&arena.Graph(child).Properties, GraphProperties, GraphPropertyExternalPorts)

	connected := arena.AddPort()
	arena.SetPortNode(connected, &parent)
	arena.SetPortSide(connected, SouthSide)
	target := arena.AddNode(top)
	arena.Graph(top).LayerlessNodes = append(arena.Graph(top).LayerlessNodes, target)
	targetPort := arena.AddPort()
	arena.SetPortNode(targetPort, &target)
	edge := arena.AddEdge()
	arena.SetEdgeSource(edge, &connected)
	arena.SetEdgeTarget(edge, &targetPort)
	unconnected := arena.AddPort()
	arena.SetPortNode(unconnected, &parent)
	arena.SetPortSide(unconnected, SouthSide)
	label := arena.AddLabel("port label")
	arena.Label(label).Size = geometry.Point{X: 12, Y: 7}
	arena.Port(unconnected).Labels = append(arena.Port(unconnected).Labels, label)

	ensureCompoundPortDummies(arena, top)
	if got := len(arena.Graph(child).LayerlessNodes); got != 2 {
		t.Fatalf("child dummies = %d, want one for each compound port", got)
	}
	for _, portID := range []LPortID{connected, unconnected} {
		dummy, found := basegraph.TryGet(&arena.Port(portID).Properties, PortDummy)
		if !found || arena.Node(dummy).Type != ExternalPortNode {
			t.Fatalf("port %d dummy = %v, present=%v", portID, dummy, found)
		}
		if got := basegraph.Get(&arena.Node(dummy).Properties, ExternalPortSide); got != SouthSide {
			t.Fatalf("port %d dummy side = %v, want SOUTH", portID, got)
		}
	}
	dummy := basegraph.Get(&arena.Port(unconnected).Properties, PortDummy)
	dummyPort := arena.Node(dummy).Ports[0]
	if len(arena.Port(dummyPort).Labels) != 1 ||
		arena.Label(arena.Port(dummyPort).Labels[0]).Size != (geometry.Point{X: 12, Y: 7}) {
		t.Fatalf("dummy labels = %+v", arena.Port(dummyPort).Labels)
	}
}

func TestCompareCompoundTraversalPathsUsesRecursivePostOrder(t *testing.T) {
	tests := []struct {
		name          string
		first, second []int
		want          int
	}{
		{name: "descendant before parent", first: []int{1, 2}, second: []int{1}, want: -1},
		{name: "parent after descendant", first: []int{1}, second: []int{1, 2}, want: 1},
		{name: "earlier sibling first", first: []int{1, 2}, second: []int{1, 3}, want: -1},
		{name: "same path", first: []int{1, 2}, second: []int{1, 2}, want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := compareCompoundTraversalPaths(test.first, test.second); got != test.want {
				t.Fatalf("compare(%v, %v) = %d, want %d", test.first, test.second, got, test.want)
			}
		})
	}
}

func TestCompoundGraphSplitAndJoinSiblingGraphs(t *testing.T) {
	arena := NewLGraphArena()
	top := arena.AddGraph()
	basegraph.Set(&arena.Graph(top).Properties, DirectionOption, DirectionDown)
	basegraph.Set(&arena.Graph(top).Properties, Hierarchy, HierarchyIncludeChildren)

	sourceParent := arena.AddNode(top)
	targetParent := arena.AddNode(top)
	arena.Graph(top).LayerlessNodes = append(arena.Graph(top).LayerlessNodes, sourceParent, targetParent)
	arena.Node(sourceParent).Position = geometry.Point{X: 100, Y: 20}
	arena.Node(targetParent).Position = geometry.Point{X: 300, Y: 30}

	sourceGraph := arena.AddGraph()
	targetGraph := arena.AddGraph()
	arena.Graph(sourceGraph).ParentNode = &sourceParent
	arena.Graph(targetGraph).ParentNode = &targetParent
	arena.Node(sourceParent).NestedGraph = &sourceGraph
	arena.Node(targetParent).NestedGraph = &targetGraph
	basegraph.Set(&arena.Graph(sourceGraph).Properties, DirectionOption, DirectionDown)
	basegraph.Set(&arena.Graph(targetGraph).Properties, DirectionOption, DirectionDown)

	sourceNode := arena.AddNode(sourceGraph)
	targetNode := arena.AddNode(targetGraph)
	arena.Graph(sourceGraph).LayerlessNodes = append(arena.Graph(sourceGraph).LayerlessNodes, sourceNode)
	arena.Graph(targetGraph).LayerlessNodes = append(arena.Graph(targetGraph).LayerlessNodes, targetNode)
	sourcePort := arena.AddPort()
	targetPort := arena.AddPort()
	arena.SetPortNode(sourcePort, &sourceNode)
	arena.SetPortNode(targetPort, &targetNode)

	original := arena.AddEdge()
	arena.SetEdgeSource(original, &sourcePort)
	arena.SetEdgeTarget(original, &targetPort)
	label := arena.AddLabel("center")
	arena.Edge(original).Labels = append(arena.Edge(original).Labels, label)
	basegraph.Set(&arena.Label(label).Properties, EdgeLabelsPlacement, EdgeLabelPlacementCenter)

	if err := (&CompoundGraphPreprocessor{}).Process(arena, top); err != nil {
		t.Fatal(err)
	}
	if arena.Edge(original).Source != nil || arena.Edge(original).Target != nil {
		t.Fatal("original edge was not detached")
	}
	crossings, found := basegraph.TryGet(&arena.Graph(top).Properties, CrossHierarchyEdges)
	if !found || len(crossings.Entries) != 1 || len(crossings.Entries[0].Segments) != 3 {
		t.Fatalf("cross hierarchy map = %#v", crossings)
	}
	if got := len(arena.Graph(sourceGraph).LayerlessNodes); got != 2 {
		t.Fatalf("source graph nodes = %d, want child plus one external dummy", got)
	}
	if got := len(arena.Graph(targetGraph).LayerlessNodes); got != 2 {
		t.Fatalf("target graph nodes = %d, want child plus one external dummy", got)
	}
	if arena.Port(arena.Node(sourceParent).Ports[0]).Side != SouthSide {
		t.Fatalf("source boundary side = %v", arena.Port(arena.Node(sourceParent).Ports[0]).Side)
	}
	if arena.Port(arena.Node(targetParent).Ports[0]).Side != NorthSide {
		t.Fatalf("target boundary side = %v", arena.Port(arena.Node(targetParent).Ports[0]).Side)
	}

	var central LEdgeID
	for _, segment := range crossings.Entries[0].Segments {
		arena.Edge(segment.Edge).BendPoints = geometry.Chain{{X: 1, Y: 2}}
		if segment.Graph == top {
			central = segment.Edge
			basegraph.Set(&arena.Edge(segment.Edge).Properties, JunctionPoints,
				geometry.Chain{{X: 5, Y: 6}})
			arena.Label(label).Position = geometry.Point{}
		}
	}
	if central == 0 {
		t.Fatal("central segment was not found")
	}

	if err := (&CompoundGraphPostprocessor{}).Process(arena, top); err != nil {
		t.Fatal(err)
	}
	if arena.Edge(original).Source == nil || *arena.Edge(original).Source != sourcePort ||
		arena.Edge(original).Target == nil || *arena.Edge(original).Target != targetPort {
		t.Fatalf("restored endpoints = %v -> %v", arena.Edge(original).Source, arena.Edge(original).Target)
	}
	if len(arena.Edge(original).Labels) != 1 || arena.Edge(original).Labels[0] != label {
		t.Fatalf("restored labels = %v", arena.Edge(original).Labels)
	}
	if got := arena.Label(label).Position; got != (geometry.Point{X: -100, Y: -20}) {
		t.Fatalf("translated center label = %+v", got)
	}
	if got := basegraph.Get(&arena.Edge(original).Properties, TargetOffset); got != (geometry.Point{X: 200, Y: 10}) {
		t.Fatalf("target offset = %+v", got)
	}
	if points, ok := basegraph.TryGet(&arena.Edge(original).Properties, JunctionPoints); !ok || len(points) != 1 || points[0] != (geometry.Point{X: -95, Y: -14}) {
		t.Fatalf("junction points = %+v, present=%v", points, ok)
	}
	if len(arena.Edge(original).BendPoints) < 3 {
		t.Fatalf("joined bend points = %+v", arena.Edge(original).BendPoints)
	}
	for _, segment := range crossings.Entries[0].Segments {
		if arena.Edge(segment.Edge).Source != nil || arena.Edge(segment.Edge).Target != nil {
			t.Fatalf("dummy edge %d was not detached", segment.Edge)
		}
	}
}
