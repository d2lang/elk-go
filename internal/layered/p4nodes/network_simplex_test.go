// Copyright (c) 2013, 2020 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"math"
	"testing"

	"github.com/d2lang/elk-go/internal/common/nodespacing"
	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// This is the arena-backed counterpart of BasicNodePlacementTest's
// testProperCoordinates assertion in the ELK 0.8.1 test suite.
func TestNetworkSimplexPlacerProducesProperCoordinates(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	for layerIndex := 0; layerIndex < 3; layerIndex++ {
		layerID := arena.AddLayer(graphID)
		arena.Graph(graphID).Layers = append(arena.Graph(graphID).Layers, layerID)
		for nodeIndex := 0; nodeIndex < 4; nodeIndex++ {
			nodeID := arena.AddNode(graphID)
			arena.SetNodeLayer(nodeID, &layerID)
			node := arena.Node(nodeID)
			node.Size.Y = float64(11 + layerIndex*3 + nodeIndex*7)
			node.Margin.Top = float64(nodeIndex) / 2
			node.Margin.Bottom = float64(layerIndex) / 3
		}
	}
	if err := (&NetworkSimplexPlacer{}).Process(arena, graphID); err != nil {
		t.Fatal(err)
	}
	for _, layerID := range arena.Graph(graphID).Layers {
		nodes := arena.Layer(layerID).Nodes
		for index, nodeID := range nodes {
			node := arena.Node(nodeID)
			if node.Position.Y != math.Floor(node.Position.Y) {
				t.Fatalf("node %d y = %v, want integral coordinate", nodeID, node.Position.Y)
			}
			if index == 0 {
				continue
			}
			previous := arena.Node(nodes[index-1])
			if previous.Position.Y+previous.Size.Y >= node.Position.Y {
				t.Fatalf("layer order overlaps: previous bottom %v, current y %v",
					previous.Position.Y+previous.Size.Y, node.Position.Y)
			}
		}
	}
}

func TestNetworkSimplexPlacerUsesFixedPortOffsets(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	leftLayer := arena.AddLayer(graphID)
	rightLayer := arena.AddLayer(graphID)
	arena.Graph(graphID).Layers = []layered.LayerID{leftLayer, rightLayer}
	source := arena.AddNode(graphID)
	target := arena.AddNode(graphID)
	arena.SetNodeLayer(source, &leftLayer)
	arena.SetNodeLayer(target, &rightLayer)
	arena.Node(source).Size.Y = 70
	arena.Node(target).Size.Y = 45
	basegraph.Set(&arena.Node(source).Properties, layered.PortConstraintsOption,
		layered.PortConstraintsFixedPosition)
	basegraph.Set(&arena.Node(target).Properties, layered.PortConstraintsOption,
		layered.PortConstraintsFixedPosition)

	sourcePort := fixedPlacementPort(arena, source, layered.EastSide, 10)
	targetPort := fixedPlacementPort(arena, target, layered.WestSide, 31)
	edgeID := arena.AddEdge()
	arena.SetEdgeSource(edgeID, &sourcePort)
	arena.SetEdgeTarget(edgeID, &targetPort)
	if err := (&NetworkSimplexPlacer{}).Process(arena, graphID); err != nil {
		t.Fatal(err)
	}
	sourceAnchor := arena.Node(source).Position.Y + arena.Port(sourcePort).Position.Y +
		arena.Port(sourcePort).Anchor.Y
	targetAnchor := arena.Node(target).Position.Y + arena.Port(targetPort).Position.Y +
		arena.Port(targetPort).Anchor.Y
	if sourceAnchor != targetAnchor {
		t.Fatalf("edge is not straight: source anchor %v, target anchor %v", sourceAnchor, targetAnchor)
	}
}

func TestNetworkSimplexPlacerRequestsHierarchicalPortPositioning(t *testing.T) {
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	basegraph.Set(&arena.Graph(graphID).Properties, layered.GraphProperties,
		layered.GraphPropertyExternalPorts)
	configuration := (&NetworkSimplexPlacer{}).ProcessorConfiguration(arena, graphID)
	processors := configuration.Slot(int(layered.PhaseEdgeRouting))
	if len(processors) != 1 || processors[0] != layered.HierarchicalPortPositionProcessor {
		t.Fatalf("edge-routing processors = %v, want hierarchical port position processor", processors)
	}
}

func TestNetworkSimplexPlacerAdjustsFlexibleNodeLabels(t *testing.T) {
	for _, test := range []struct {
		name      string
		placement []string
		want      float64
	}{
		{"bottom", []string{"INSIDE", "V_BOTTOM", "H_CENTER"}, 12},
		{"center", []string{"INSIDE", "V_CENTER", "H_CENTER"}, 7},
		{"top", []string{"INSIDE", "V_TOP", "H_CENTER"}, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			arena := layered.NewLGraphArena()
			graphID := arena.AddGraph()
			nodeID := arena.AddNode(graphID)
			labelID := arena.AddLabel("label")
			arena.Node(nodeID).Labels = append(arena.Node(nodeID).Labels, labelID)
			arena.Label(labelID).Position.Y = 2
			basegraph.Set(&arena.Node(nodeID).Properties, nodespacing.NodeLabelsPlacement, test.placement)
			placer := networkSimplexPlacement{arena: arena, graphID: graphID}
			placer.adjustLabelsAndSouthPorts(nodeID, 10, layered.NodeFlexibilitySize)
			if got := arena.Label(labelID).Position.Y; got != test.want {
				t.Fatalf("label y = %v, want %v", got, test.want)
			}
		})
	}
}

func fixedPlacementPort(
	arena *layered.LGraphArena,
	nodeID layered.LNodeID,
	side layered.PortSide,
	y float64,
) layered.LPortID {
	portID := arena.AddPort()
	port := arena.Port(portID)
	port.Size.X, port.Size.Y = 4, 4
	port.Position.Y = y
	arena.SetPortNode(portID, &nodeID)
	arena.SetPortSide(portID, side)
	return portID
}
