// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2017, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package radial

import (
	"math"
	"testing"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
	"github.com/d2lang/elk-go/internal/graph"
)

func TestEadesPlacementUsesAnnulusWedges(t *testing.T) {
	model := graph.NewModel()
	root := model.AddNode(&model.Root)
	first := model.AddNode(&model.Root)
	second := model.AddNode(&model.Root)
	for _, node := range []graph.NodeID{root, first, second} {
		model.Node(node).Shape.SetDimensions(20, 20)
	}
	for index, target := range []graph.NodeID{first, second} {
		edge := model.AddEdge(&model.Root)
		model.Edge(edge).Identifier = string(rune('a' + index))
		model.AddSource(edge, graph.NodeRef(root))
		model.AddTarget(edge, graph.NodeRef(target))
	}

	positionGraph(model, root, 40, "NONE", "NODE_SIZE", "NONE")
	center := func(node graph.NodeID) geometry.Point {
		shape := model.Node(node).Shape
		return geometry.Point{X: shape.X + shape.Width/2, Y: shape.Y + shape.Height/2}
	}
	if got := center(root); math.Abs(got.X) > 1e-12 || math.Abs(got.Y) > 1e-12 {
		t.Fatalf("root center = %+v", got)
	}
	if got := center(first); math.Abs(got.X) > 1e-12 || math.Abs(got.Y-40) > 1e-12 {
		t.Fatalf("first child center = %+v", got)
	}
	if got := center(second); math.Abs(got.X) > 1e-12 || math.Abs(got.Y+40) > 1e-12 {
		t.Fatalf("second child center = %+v", got)
	}
}

func TestRadialMetadata(t *testing.T) {
	options := core.NewOptionRegistry()
	RegisterOptions(options)
	if got, ok := options.ByID(Compactor.ID).ParseValue("WEDGE_COMPACTION"); !ok || got != "WEDGE_COMPACTION" {
		t.Fatalf("compactor = %#v, %v", got, ok)
	}
	algorithms := core.NewAlgorithmRegistry()
	if err := Register(algorithms); err != nil {
		t.Fatal(err)
	}
	if algorithm := algorithms.ByID(AlgorithmID); algorithm == nil || algorithm.Features != 0 {
		t.Fatalf("Radial metadata = %#v", algorithm)
	}
}

func TestEadesEdgeLengthOptimizationUsesSourceRounding(t *testing.T) {
	model := graph.NewModel()
	root := model.AddNode(&model.Root)
	first := model.AddNode(&model.Root)
	second := model.AddNode(&model.Root)
	grandchild := model.AddNode(&model.Root)
	for index, node := range []graph.NodeID{root, first, second, grandchild} {
		sizes := []geometry.Point{{X: 23, Y: 17}, {X: 31, Y: 11}, {X: 13, Y: 29}, {X: 41, Y: 7}}
		model.Node(node).Shape.SetDimensions(sizes[index].X, sizes[index].Y)
	}
	for index, endpoints := range [][2]graph.NodeID{{root, first}, {root, second}, {first, grandchild}} {
		edge := model.AddEdge(&model.Root)
		model.Edge(edge).Identifier = string(rune('a' + index))
		model.AddSource(edge, graph.NodeRef(endpoints[0]))
		model.AddTarget(edge, graph.NodeRef(endpoints[1]))
	}
	best := math.MaxFloat64
	bestDegree := -1
	for degree := 0; degree < 360; degree++ {
		positionNodes(model, root, root, 42, "NONE", "NODE_SIZE", 0, 0, 2*math.Pi, float64(degree)*math.Pi/180)
		value := evaluateEdgeLength(model, root)
		if value < best {
			best = value
			bestDegree = degree
		}
	}
	if bestDegree != 277 {
		t.Fatalf("best edge-length offset = %d degrees, want 277", bestDegree)
	}
}
