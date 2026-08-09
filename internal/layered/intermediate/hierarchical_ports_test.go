// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: EPL-2.0

package intermediate

import (
	"testing"

	"github.com/d2lang/elk-go/internal/geometry"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

func TestHierarchicalNorthDummyReplacementAndSizing(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	basegraph.Set(&arena.Graph(graphID).Properties, layered.PortConstraintsOption,
		layered.PortConstraintsFixedSide)
	first := arena.AddLayerToGraph(graphID)
	second := arena.AddLayerToGraph(graphID)
	original := arena.AddNode(graphID)
	arena.Node(original).Type = layered.ExternalPortNode
	basegraph.Set(&arena.Node(original).Properties, layered.ExternalPortSide, layered.NorthSide)
	basegraph.Set(&arena.Node(original).Properties, layered.OriginalElement, layered.LPortOrigin(99))
	originalPort := arena.AddPort()
	arena.SetPortNode(originalPort, &original)
	arena.SetPortSide(originalPort, layered.SouthSide)
	normal := arena.AddNode(graphID)
	normalPort := arena.AddPort()
	arena.SetPortNode(normalPort, &normal)
	arena.SetPortSide(normalPort, layered.WestSide)
	arena.SetNodeLayer(original, &first)
	arena.SetNodeLayer(normal, &second)
	edge := arena.AddEdge()
	arena.SetEdgeSource(edge, &originalPort)
	arena.SetEdgeTarget(edge, &normalPort)

	if err := ProcessHierarchicalPortConstraints(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if arena.Node(original).Layer != nil {
		t.Fatal("original north dummy remained in a layer")
	}
	replaced := basegraph.Get(&arena.Graph(graphID).Properties, layered.ExternalPortReplacedDummies)
	if len(replaced) != 1 || replaced[0] != original {
		t.Fatalf("replaced dummies = %v", replaced)
	}
	representative := arena.EdgeSourceNode(edge)
	if representative == original || arena.Node(representative).Type != layered.ExternalPortNode ||
		basegraph.Get(&arena.Node(representative).Properties, layered.ExternalPortReplacedDummy) != original {
		t.Fatalf("representative = %#v", arena.Node(representative))
	}
	if err := SizeHierarchicalPortDummies(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if got := basegraph.Get(&arena.Node(representative).Properties, layered.AlignmentOption); got != layered.AlignmentCenter {
		t.Fatalf("representative alignment = %v", got)
	}
}

func TestRouteHierarchicalEastPortFixesBorderAndBend(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	arena.Graph(graphID).Size = geometry.Point{X: 100, Y: 60}
	arena.Graph(graphID).Padding = geometry.Spacing{Left: 10, Right: 20, Top: 5, Bottom: 5}
	layerID := arena.AddLayerToGraph(graphID)
	normal := arena.AddNode(graphID)
	dummy := arena.AddNode(graphID)
	arena.Node(dummy).Type = layered.ExternalPortNode
	basegraph.Set(&arena.Node(dummy).Properties, layered.ExternalPortSide, layered.EastSide)
	basegraph.Set(&arena.Node(dummy).Properties, layered.ExternalPortSize, geometry.Point{Y: 8})
	normalPort := arena.AddPort()
	dummyPort := arena.AddPort()
	arena.SetPortNode(normalPort, &normal)
	arena.SetPortSide(normalPort, layered.EastSide)
	arena.SetPortNode(dummyPort, &dummy)
	arena.SetPortSide(dummyPort, layered.WestSide)
	arena.Port(dummyPort).Position.Y = 12
	arena.SetNodeLayer(normal, &layerID)
	arena.SetNodeLayer(dummy, &layerID)
	edge := arena.AddEdge()
	arena.SetEdgeSource(edge, &normalPort)
	arena.SetEdgeTarget(edge, &dummyPort)
	arena.Edge(edge).BendPoints = geometry.Chain{{X: 70, Y: 3}}

	if err := RouteHierarchicalPorts(arena, graphID); err != nil {
		t.Fatal(err)
	}
	if got := arena.Node(dummy).Position.X; got != 120 {
		t.Fatalf("east dummy x = %v", got)
	}
	if got := arena.Edge(edge).BendPoints[0].Y; got != 12 {
		t.Fatalf("edge terminal bend y = %v", got)
	}
}

func TestResizeHierarchicalNodeTransfersSizeAndPort(t *testing.T) {
	arena := layered.NewLGraphArena()
	outer := arena.AddGraph()
	parent := arena.AddNode(outer)
	arena.Node(parent).Size = geometry.Point{X: 50, Y: 40}
	arena.Graph(outer).LayerlessNodes = append(arena.Graph(outer).LayerlessNodes, parent)
	child := arena.AddGraph()
	arena.Graph(child).ParentNode = &parent
	arena.Node(parent).NestedGraph = &child
	arena.Graph(child).Size = geometry.Point{X: 100, Y: 60}
	arena.Graph(child).Padding = geometry.Spacing{Left: 10, Right: 10, Top: 5, Bottom: 5}
	basegraph.Set(&arena.Graph(child).Properties, layered.GraphProperties,
		layered.GraphPropertyExternalPorts)
	layerID := arena.AddLayerToGraph(child)
	boundary := arena.AddPort()
	arena.SetPortNode(boundary, &parent)
	dummy := arena.AddNode(child)
	arena.Node(dummy).Type = layered.ExternalPortNode
	basegraph.Set(&arena.Node(dummy).Properties, layered.OriginalElement, layered.LPortOrigin(boundary))
	basegraph.Set(&arena.Node(dummy).Properties, layered.ExternalPortSide, layered.EastSide)
	dummyPort := arena.AddPort()
	arena.SetPortNode(dummyPort, &dummy)
	arena.SetPortSide(dummyPort, layered.WestSide)
	arena.SetNodeLayer(dummy, &layerID)

	if err := ResizeHierarchicalNode(arena, child); err != nil {
		t.Fatal(err)
	}
	if len(arena.Graph(child).Layers) != 0 || len(arena.Graph(child).LayerlessNodes) != 1 {
		t.Fatalf("finished child graph = layers %v nodes %v", arena.Graph(child).Layers,
			arena.Graph(child).LayerlessNodes)
	}
	if got := arena.Node(parent).Size; got != (geometry.Point{X: 120, Y: 70}) {
		t.Fatalf("parent size = %+v", got)
	}
	if got := basegraph.Get(&arena.Node(parent).Properties, layered.PortConstraintsOption); got != layered.PortConstraintsFixedPosition {
		t.Fatalf("parent port constraints = %v", got)
	}
	if arena.Port(boundary).Side != layered.EastSide || arena.Port(boundary).Position.X != 120 {
		t.Fatalf("external parent port = side %v position %+v", arena.Port(boundary).Side,
			arena.Port(boundary).Position)
	}
}
