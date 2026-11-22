// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// TestBKDenseMatchesOriginal places random layered graphs with the dense
// implementation and with the original one and requires bit-identical node
// positions (and the same panics, if any).
func TestBKDenseMatchesOriginal(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	for round := 0; round < 3000; round++ {
		seed := random.Int63()
		denseArena, denseGraph := randomBKGraph(seed)
		originalArena, originalGraph := randomBKGraph(seed)
		densePanic := placeCatching(denseArena, denseGraph, false)
		originalPanic := placeCatching(originalArena, originalGraph, true)
		if densePanic != originalPanic {
			t.Fatalf("round %d (seed %d): panic %q, original %q", round, seed, densePanic, originalPanic)
		}
		for index := range denseArena.Nodes {
			got, want := denseArena.Nodes[index].Position.Y, originalArena.Nodes[index].Position.Y
			if math.Float64bits(got) != math.Float64bits(want) {
				t.Fatalf("round %d (seed %d): node %d y = %v, original %v", round, seed, index, got, want)
			}
			if denseArena.Nodes[index].ID != originalArena.Nodes[index].ID {
				t.Fatalf("round %d (seed %d): node %d scratch ID differs", round, seed, index)
			}
		}
	}
}

func placeCatching(arena *layered.LGraphArena, graphID layered.LGraphID, original bool) (message string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			message = fmt.Sprint(recovered)
		}
	}()
	saved := bkDenseDisabled
	bkDenseDisabled = original
	defer func() { bkDenseDisabled = saved }()
	PlaceBrandesKoepf(arena, graphID)
	return ""
}

// randomBKGraph builds a layered graph: random layer sizes, node types,
// sizes, margins and port positions, edges between adjacent and distant
// layers, in-layer edges, self-loops, multi-edges, priorities, individual
// spacings and random BK options.
func randomBKGraph(seed int64) (*layered.LGraphArena, layered.LGraphID) {
	random := rand.New(rand.NewSource(seed))
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	properties := &arena.Graph(graphID).Properties
	switch random.Intn(6) {
	case 0:
		basegraph.Set(properties, layered.BKFixedAlignment, layered.FixedAlignmentBalanced)
	case 1:
		basegraph.Set(properties, layered.BKFixedAlignment, layered.FixedAlignment(random.Intn(6)))
	}
	if random.Intn(2) == 0 {
		basegraph.Set(properties, layered.FavorStraightEdges, random.Intn(2) == 0)
	}
	if random.Intn(3) == 0 {
		basegraph.Set(properties, layered.BKEdgeStraightening, layered.EdgeStraighteningStrategy(random.Intn(2)))
	}
	if random.Intn(2) == 0 {
		basegraph.Set(properties, layered.SpacingNodeNode, float64(random.Intn(30)))
	}
	if random.Intn(2) == 0 {
		basegraph.Set(properties, layered.SpacingEdgeNode, float64(random.Intn(20)))
	}
	if random.Intn(2) == 0 {
		basegraph.Set(properties, layered.SpacingEdgeEdge, float64(random.Intn(20)))
	}
	layerCount := 1 + random.Intn(6)
	layers := make([]layered.LayerID, layerCount)
	var nodes [][]layered.LNodeID
	for index := range layers {
		layers[index] = arena.AddLayer(graphID)
		size := 1 + random.Intn(5)
		var layerNodes []layered.LNodeID
		for k := 0; k < size; k++ {
			nodeID := arena.AddNode(graphID)
			node := arena.Node(nodeID)
			node.Size.Y = float64(random.Intn(40))
			if random.Intn(4) == 0 {
				node.Size.Y += 0.5
			}
			node.Margin.Top = float64(random.Intn(4))
			node.Margin.Bottom = float64(random.Intn(4))
			switch random.Intn(5) {
			case 0, 1:
				node.Type = layered.LongEdgeNode
			case 2:
				node.Type = layered.LabelNode
			}
			if random.Intn(400) == 0 {
				// no vertical spacing is defined for these: both paths panic
				node.Type = layered.NodeType(4 + random.Intn(4))
			}
			if random.Intn(12) == 0 {
				var individual layered.IndividualSpacings
				basegraph.Set(&individual.Properties, layered.SpacingNodeNode, float64(random.Intn(40)))
				basegraph.Set(&node.Properties, layered.SpacingIndividual, individual)
			}
			layerID := layers[index]
			arena.SetNodeLayer(nodeID, &layerID)
			layerNodes = append(layerNodes, nodeID)
		}
		nodes = append(nodes, layerNodes)
	}
	arena.Graph(graphID).Layers = append([]layered.LayerID(nil), layers...)
	port := func(nodeID layered.LNodeID, side layered.PortSide) layered.LPortID {
		if ports := arena.Node(nodeID).Ports; len(ports) > 0 && random.Intn(3) == 0 {
			return ports[random.Intn(len(ports))]
		}
		portID := arena.AddPort()
		arena.SetPortNode(portID, &nodeID)
		arena.Port(portID).Position.Y = float64(random.Intn(20))
		arena.Port(portID).Size.Y = float64(random.Intn(3))
		arena.SetPortSide(portID, side)
		return portID
	}
	if random.Intn(50) == 0 {
		// an edge to a node of another graph: the dense path declines
		otherGraph := arena.AddGraph()
		otherLayer := arena.AddLayer(otherGraph)
		arena.Graph(otherGraph).Layers = []layered.LayerID{otherLayer}
		foreign := arena.AddNode(otherGraph)
		arena.SetNodeLayer(foreign, &otherLayer)
		nodes = append(nodes, []layered.LNodeID{foreign})
		layerCount++
	}
	edgeCount := random.Intn(4 * len(arena.Nodes))
	for edge := 0; edge < edgeCount; edge++ {
		fromLayer := random.Intn(layerCount)
		toLayer := fromLayer + 1
		switch random.Intn(8) {
		case 0:
			toLayer = fromLayer
		case 1:
			toLayer = random.Intn(layerCount)
		}
		if toLayer >= layerCount {
			continue
		}
		source := nodes[fromLayer][random.Intn(len(nodes[fromLayer]))]
		target := nodes[toLayer][random.Intn(len(nodes[toLayer]))]
		sourcePort, targetPort := port(source, layered.EastSide), port(target, layered.WestSide)
		edgeID := arena.AddEdge()
		arena.SetEdgeSource(edgeID, &sourcePort)
		arena.SetEdgeTarget(edgeID, &targetPort)
		if random.Intn(6) == 0 {
			basegraph.Set(&arena.Edge(edgeID).Properties, layered.PriorityStraightness, random.Intn(3))
		}
	}
	return arena, graphID
}
