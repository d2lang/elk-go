// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package p4nodes

import (
	"math"
	"math/rand"
	"testing"

	basegraph "github.com/d2lang/elk-go/internal/graph"
	"github.com/d2lang/elk-go/internal/layered"
)

// TestBKSingleNodeMatchesGeneral places random one-node graphs with
// placeSingleNode and with the general (dense) path and requires the same
// position bits, node and layer numbers, including sizes and margins at and
// beyond the accepted range (where placeSingleNode must decline).
func TestBKSingleNodeMatchesGeneral(t *testing.T) {
	values := []float64{
		0, math.Copysign(0, -1), 1, -1, 3, 0.5, -0.25, 17.75, 1e-300, -1e-300, 123456.789,
		1 << 40, -(1 << 40), 1<<60 - 1<<8, -(1<<60 - 1<<8), 1 << 60, -(1 << 60), 1e300, -1e300,
		math.MaxFloat64, -math.MaxFloat64, math.Inf(1), math.Inf(-1), math.NaN(),
	}
	random := rand.New(rand.NewSource(11))
	pick := func() float64 {
		if random.Intn(3) == 0 {
			return float64(random.Intn(200)) - 50 + float64(random.Intn(4))/4
		}
		return values[random.Intn(len(values))]
	}
	declined := 0
	for round := 0; round < 20000; round++ {
		sizeY, top, bottom := pick(), pick(), pick()
		options := random.Int63()
		fastArena, fastGraph := singleNodeGraph(sizeY, top, bottom, options)
		generalArena, generalGraph := singleNodeGraph(sizeY, top, bottom, options)
		if !placeSingleNode(fastArena, fastGraph) {
			declined++
			PlaceBrandesKoepf(fastArena, fastGraph)
		}
		saved := bkSingleNodeDisabled
		bkSingleNodeDisabled = true
		PlaceBrandesKoepf(generalArena, generalGraph)
		bkSingleNodeDisabled = saved
		fast, general := fastArena.Nodes[0], generalArena.Nodes[0]
		if math.Float64bits(fast.Position.Y) != math.Float64bits(general.Position.Y) ||
			fast.ID != general.ID || fastArena.Layers[0].ID != generalArena.Layers[0].ID {
			t.Fatalf("round %d (size %v, margins %v/%v, options %d): y %v (id %d, layer %d), general %v (id %d, layer %d)",
				round, sizeY, top, bottom, options, fast.Position.Y, fast.ID, fastArena.Layers[0].ID,
				general.Position.Y, general.ID, generalArena.Layers[0].ID)
		}
	}
	if declined == 0 {
		t.Fatal("placeSingleNode never declined")
	}
}

// singleNodeGraph is a graph with one layer holding one node without ports,
// with random BK options, node type and individual spacing taken from
// options' bits.
func singleNodeGraph(sizeY, top, bottom float64, options int64) (*layered.LGraphArena, layered.LGraphID) {
	random := rand.New(rand.NewSource(options))
	arena := layered.NewLGraphArena()
	graphID := arena.AddGraph()
	properties := &arena.Graph(graphID).Properties
	if random.Intn(3) != 0 {
		basegraph.Set(properties, layered.BKFixedAlignment, layered.FixedAlignment(random.Intn(6)))
	}
	if random.Intn(2) == 0 {
		basegraph.Set(properties, layered.FavorStraightEdges, random.Intn(2) == 0)
	}
	if random.Intn(2) == 0 {
		basegraph.Set(properties, layered.BKEdgeStraightening, layered.EdgeStraighteningStrategy(random.Intn(2)))
	}
	layerID := arena.AddLayer(graphID)
	arena.Layer(layerID).ID = 7
	arena.Graph(graphID).Layers = []layered.LayerID{layerID}
	nodeID := arena.AddNode(graphID)
	node := arena.Node(nodeID)
	node.ID = 5
	node.Size.Y, node.Margin.Top, node.Margin.Bottom = sizeY, top, bottom
	node.Type = layered.NodeType(random.Intn(4))
	if random.Intn(4) == 0 {
		var individual layered.IndividualSpacings
		basegraph.Set(&individual.Properties, layered.SpacingNodeNode, float64(random.Intn(40)))
		basegraph.Set(&node.Properties, layered.SpacingIndividual, individual)
	}
	arena.SetNodeLayer(nodeID, &layerID)
	return arena, graphID
}
