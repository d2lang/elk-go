// Copyright (c) 2013, 2015 Kiel University and others.
// Copyright (c) 2015, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package mrtree

import (
	"testing"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/geometry"
)

func TestWalkerPlacementRetainsMrTreeChildOrder(t *testing.T) {
	g := &tGraph{spacing: 20}
	root := &tNode{id: 0, size: geometry.Point{X: 30, Y: 20}}
	first := &tNode{id: 1, size: geometry.Point{X: 40, Y: 10}}
	second := &tNode{id: 2, size: geometry.Point{X: 20, Y: 30}}
	g.nodes = []*tNode{root, first, second}
	g.addDummyChild(root, first)
	g.addDummyChild(root, second)

	computeFan(root)
	orderNodes(root, "FAN")
	computeLevelHeights(root, "DOWN")
	computeNeighbors(root)
	placeNodes(g, root)

	if children := root.children(); len(children) != 2 || children[0] != second || children[1] != first {
		t.Fatalf("equal-fan leaf order = %v; want reverse input order", children)
	}
	if root.position != (geometry.Point{X: 25, Y: 0}) || second.position != (geometry.Point{X: 0, Y: 45}) || first.position != (geometry.Point{X: 50, Y: 45}) {
		t.Fatalf("Walker positions = root %+v, second %+v, first %+v", root.position, second.position, first.position)
	}
}

func TestMrTreeMetadata(t *testing.T) {
	options := core.NewOptionRegistry()
	RegisterOptions(options)
	if got, ok := options.ByID(SearchOrder.ID).ParseValue("BFS"); !ok || got != "BFS" {
		t.Fatalf("searchOrder BFS = %#v, %v", got, ok)
	}
	algorithms := core.NewAlgorithmRegistry()
	if err := Register(algorithms); err != nil {
		t.Fatal(err)
	}
	algorithm := algorithms.ByID(AlgorithmID)
	if algorithm == nil || !algorithm.Features.Has(core.FeatureDisconnected) {
		t.Fatalf("Mr. Tree metadata = %#v", algorithm)
	}
}
