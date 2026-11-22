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

// corruptLayers breaks the agreement between a graph's layer lists and its
// nodes' Layer fields the way ELK's hierarchical sweep can (it may write an
// external-port dummy into a second layer list, dropping the node it
// overwrites) and in the other ways the original code would read differently
// from arrays indexed by position. It returns a description of what it did.
func corruptLayers(arena *layered.LGraphArena, graphID layered.LGraphID, random *rand.Rand) string {
	layers := arena.Graph(graphID).Layers
	var listed []layered.LNodeID
	for _, layerID := range layers {
		listed = append(listed, arena.Layer(layerID).Nodes...)
	}
	if len(listed) == 0 {
		return "none"
	}
	nodeID := listed[random.Intn(len(listed))]
	node := arena.Node(nodeID)
	home := *node.Layer
	other := layers[random.Intn(len(layers))]
	insert := func(layerID layered.LayerID, nodeID layered.LNodeID, overwrite bool) {
		nodes := arena.Layer(layerID).Nodes
		index := random.Intn(len(nodes) + 1)
		if overwrite && len(nodes) > 0 {
			nodes[random.Intn(len(nodes))] = nodeID
			return
		}
		nodes = append(nodes, 0)
		copy(nodes[index+1:], nodes[index:])
		nodes[index] = nodeID
		arena.Layer(layerID).Nodes = nodes
	}
	remove := func(layerID layered.LayerID, nodeID layered.LNodeID) {
		nodes := arena.Layer(layerID).Nodes
		for index, listedID := range nodes {
			if listedID == nodeID {
				arena.Layer(layerID).Nodes = append(nodes[:index:index], nodes[index+1:]...)
				return
			}
		}
	}
	switch random.Intn(9) {
	case 8: // a layer of another graph (with other spacings) listed by this one
		otherGraph := arena.AddGraph()
		basegraph.Set(&arena.Graph(otherGraph).Properties, layered.SpacingNodeNode, float64(1+random.Intn(90)))
		basegraph.Set(&arena.Graph(otherGraph).Properties, layered.SpacingEdgeNode, float64(1+random.Intn(90)))
		basegraph.Set(&arena.Graph(otherGraph).Properties, layered.SpacingEdgeEdge, float64(1+random.Intn(90)))
		foreignLayer := arena.AddLayer(otherGraph)
		// moved consistently: listed by the foreign layer, Layer names it
		arena.SetNodeLayer(nodeID, &foreignLayer)
		for count := random.Intn(3); count > 0; count-- {
			if extra := listed[random.Intn(len(listed))]; *arena.Node(extra).Layer != foreignLayer {
				arena.SetNodeLayer(extra, &foreignLayer)
			}
		}
		position := random.Intn(len(layers) + 1)
		grown := append(append([]layered.LayerID(nil), layers[:position]...), foreignLayer)
		arena.Graph(graphID).Layers = append(grown, layers[position:]...)
		return fmt.Sprintf("layer of another graph listed at %d", position)
	case 0: // listed a second time in another (or the same) layer
		insert(other, nodeID, false)
		return fmt.Sprintf("node %d also listed in layer %d", nodeID, other)
	case 1: // written over another node of a layer (the sweep's case)
		insert(other, nodeID, true)
		return fmt.Sprintf("node %d written over a node of layer %d", nodeID, other)
	case 2: // listed in another layer than the one its Layer names
		remove(home, nodeID)
		insert(other, nodeID, false)
		return fmt.Sprintf("node %d moved to layer %d, Layer still %d", nodeID, other, home)
	case 3:
		node.Layer = nil
		return fmt.Sprintf("node %d Layer nil", nodeID)
	case 4:
		node.Layer = &other
		return fmt.Sprintf("node %d Layer %d, listed in %d", nodeID, other, home)
	case 5: // not listed at all (its edges remain)
		remove(home, nodeID)
		return fmt.Sprintf("node %d not listed", nodeID)
	case 6: // a layer listed twice
		arena.Graph(graphID).Layers = append(append([]layered.LayerID(nil), layers...), other)
		return fmt.Sprintf("layer %d listed twice", other)
	default: // Layer names a layer of another graph
		otherGraph := arena.AddGraph()
		foreignLayer := arena.AddLayer(otherGraph)
		arena.Graph(otherGraph).Layers = []layered.LayerID{foreignLayer}
		for count := random.Intn(3); count >= 0; count-- {
			foreign := arena.AddNode(otherGraph)
			arena.SetNodeLayer(foreign, &foreignLayer)
			arena.Node(foreign).ID = random.Intn(4)
		}
		arena.Layer(foreignLayer).ID = random.Intn(3)
		arena.Node(nodeID).Layer = &foreignLayer
		return fmt.Sprintf("node %d Layer in another graph", nodeID)
	}
}

// placeRecovering places the graph with the production paths (dense on,
// single-node shortcut on) or with the original code only, and returns the
// panic value, if any.
func placeRecovering(arena *layered.LGraphArena, graphID layered.LGraphID, original bool) (message string) {
	savedDense, savedSingle := bkDenseDisabled, bkSingleNodeDisabled
	bkDenseDisabled, bkSingleNodeDisabled = original, original
	defer func() {
		bkDenseDisabled, bkSingleNodeDisabled = savedDense, savedSingle
		if recovered := recover(); recovered != nil {
			message = fmt.Sprint(recovered)
		}
	}()
	PlaceBrandesKoepf(arena, graphID)
	return ""
}

// TestInconsistentLayersMatchOriginal places random layered graphs
// whose layer lists and Layer fields disagree (a node
// listed twice) with the production paths and with the original code, and
// requires the same positions, scratch IDs and panics.
func TestInconsistentLayersMatchOriginal(t *testing.T) {
	random := rand.New(rand.NewSource(59))
	rounds := 6000
	if testing.Short() {
		rounds = 1500
	}
	for round := 0; round < rounds; round++ {
		seed := random.Int63()
		corruption := random.Int63()
		productionArena, productionGraph := randomBKGraph(seed)
		originalArena, originalGraph := randomBKGraph(seed)
		what := corruptLayers(productionArena, productionGraph, rand.New(rand.NewSource(corruption)))
		corruptLayers(originalArena, originalGraph, rand.New(rand.NewSource(corruption)))
		productionPanic := placeRecovering(productionArena, productionGraph, false)
		originalPanic := placeRecovering(originalArena, originalGraph, true)
		if productionPanic != originalPanic {
			t.Fatalf("round %d (seed %d, %s): panic %q, original %q", round, seed, what, productionPanic, originalPanic)
		}
		sameNodes(t, fmt.Sprintf("round %d (seed %d, %s)", round, seed, what), productionArena, originalArena)
	}
}

// TestSingleNodeWithForeignLayer covers the single-node shortcut: a
// graph with one layer listing one node whose Layer field is nil or names
// another layer (with or without nodes, of this or another graph).
func TestSingleNodeWithForeignLayer(t *testing.T) {
	random := rand.New(rand.NewSource(5))
	for round := 0; round < 4000; round++ {
		options := random.Int63()
		kind := random.Intn(4)
		others := random.Intn(4)
		build := func() (*layered.LGraphArena, layered.LGraphID) {
			arena, graphID := singleNodeGraph(float64(round%7), float64(round%3), 1, options)
			nodeID := arena.Layer(arena.Graph(graphID).Layers[0]).Nodes[0]
			switch kind {
			case 0:
				arena.Node(nodeID).Layer = nil
			case 1, 2:
				owner := graphID
				if kind == 2 {
					owner = arena.AddGraph()
				}
				layerID := arena.AddLayer(owner)
				for count := 0; count < others; count++ {
					other := arena.AddNode(owner)
					arena.SetNodeLayer(other, &layerID)
					arena.Node(other).ID = count % 2
				}
				arena.Node(nodeID).Layer = &layerID
			default: // consistent: the shortcut applies
			}
			return arena, graphID
		}
		productionArena, productionGraph := build()
		originalArena, originalGraph := build()
		productionPanic := placeRecovering(productionArena, productionGraph, false)
		originalPanic := placeRecovering(originalArena, originalGraph, true)
		if productionPanic != originalPanic {
			t.Fatalf("round %d (kind %d, %d others): panic %q, original %q", round, kind, others, productionPanic, originalPanic)
		}
		sameNodes(t, fmt.Sprintf("round %d (kind %d, %d others)", round, kind, others), productionArena, originalArena)
	}
}

func sameNodes(t *testing.T, where string, production, original *layered.LGraphArena) {
	t.Helper()
	for index := range original.Nodes {
		got, want := production.Nodes[index], original.Nodes[index]
		if math.Float64bits(got.Position.Y) != math.Float64bits(want.Position.Y) || got.ID != want.ID {
			t.Fatalf("%s: node %d y %v id %d, original y %v id %d", where, index, got.Position.Y, got.ID, want.Position.Y, want.ID)
		}
	}
	for index := range original.Layers {
		if production.Layers[index].ID != original.Layers[index].ID {
			t.Fatalf("%s: layer %d id %d, original %d", where, index, production.Layers[index].ID, original.Layers[index].ID)
		}
	}
}
