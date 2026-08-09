// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package spore

import (
	"fmt"

	"github.com/d2lang/elk-go/internal/core"
	"github.com/d2lang/elk-go/internal/graph"
)

type overlapProvider struct {
	algorithms *core.AlgorithmRegistry
}

func (provider *overlapProvider) Layout(model *graph.Model, parent graph.NodeID, monitor core.ProgressMonitor) error {
	if err := runUnderlying(model, parent, monitor, provider.algorithms); err != nil {
		return err
	}
	properties := &model.Node(parent).Properties
	graph.Set(properties, RootSelection, RootCenter)
	graph.Set(properties, SpanningTreeCost, CostInvertedOverlap)
	graph.Set(properties, TreeConstruction, TreeMinimum)
	maxIterations := graph.Get(properties, MaxIterations)

	monitor.Begin("Overlap removal", 1)
	defer monitor.Done()
	graphImporter := newImporter(model, parent)
	internal, err := graphImporter.importGraph()
	if err != nil {
		return err
	}
	overlapsExisted := true
	for iteration := 0; iteration < maxIterations && overlapsExisted; iteration++ {
		if graph.Get(properties, RunScanline) {
			internal.edges = scanlineOverlaps(internal.vertices)
			if len(internal.edges) == 0 {
				break
			}
		}
		if err := executeStructureAndTree(internal); err != nil {
			return err
		}
		overlapsExisted = growTree(internal.tree)
		graphImporter.updateGraph()
	}
	graphImporter.applyPositions()
	return nil
}

type compactionProvider struct {
	algorithms *core.AlgorithmRegistry
}

func (provider *compactionProvider) Layout(model *graph.Model, parent graph.NodeID, monitor core.ProgressMonitor) error {
	if err := runUnderlying(model, parent, monitor, provider.algorithms); err != nil {
		return err
	}
	graphImporter := newImporter(model, parent)
	internal, err := graphImporter.importGraph()
	if err != nil {
		return err
	}
	monitor.Begin("Compaction by shrinking a tree", 3)
	if len(internal.vertices) > 1 {
		if err := executeStructureAndTree(internal); err != nil {
			return err
		}
		switch internal.compaction {
		case CompactionDepthFirst:
			compactTree(internal.tree, internal.orthogonal)
		default:
			return fmt.Errorf("no SPOrE compaction implementation for %q", internal.compaction)
		}
	}
	monitor.Done()
	graphImporter.applyPositions()
	return nil
}

func executeStructureAndTree(internal *sporeGraph) error {
	points := make([]*vertex, len(internal.vertices))
	for index, candidate := range internal.vertices {
		points[index] = candidate.original
	}
	for _, edge := range triangulate(points) {
		internal.edges = addEdge(internal.edges, edge)
	}
	if len(internal.vertices) == 0 {
		return nil
	}
	cost := internal.cost
	if internal.treeConstruction == TreeMaximum {
		cost = func(edge *tEdge) float64 { return -internal.cost(edge) }
	} else if internal.treeConstruction != TreeMinimum {
		return fmt.Errorf("no SPOrE tree construction implementation for %q", internal.treeConstruction)
	}
	root := internal.vertices[0].original
	if internal.preferredRoot != nil {
		root = internal.preferredRoot.original
	}
	internal.tree = createSpanningTreeForNodes(internal.edges, root, cost, internal.vertices)
	return nil
}
