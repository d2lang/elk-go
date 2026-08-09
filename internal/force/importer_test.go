// Copyright (c) 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package force

import (
	"testing"

	"github.com/d2lang/elk-go/internal/graph"
)

// These cases are source-shaped ports of ELK 0.8.1's ForceImportTest.
func TestImport(t *testing.T) {
	model, parent := createSimpleGraph()
	fgraph, err := NewImporter(model, parent).ImportGraph()
	if err != nil {
		t.Fatal(err)
	}
	checkSimpleGraph(t, fgraph)
}

func TestSeparateConnectedComponents(t *testing.T) {
	model := graph.NewModel()
	parent := model.Root
	addSimpleGraph(model, parent)
	addSimpleGraph(model, parent)
	fgraph, err := NewImporter(model, parent).ImportGraph()
	if err != nil {
		t.Fatal(err)
	}
	fgraph.SeparateConnectedComponents = true
	components := Split(fgraph)
	if len(components) != 2 {
		t.Fatalf("got %d components, want 2", len(components))
	}
	for _, component := range components {
		checkSimpleGraph(t, component)
	}
}

func TestDoNotSeparateConnectedComponents(t *testing.T) {
	model := graph.NewModel()
	parent := model.Root
	addSimpleGraph(model, parent)
	addSimpleGraph(model, parent)
	fgraph, err := NewImporter(model, parent).ImportGraph()
	if err != nil {
		t.Fatal(err)
	}
	fgraph.SeparateConnectedComponents = false
	components := Split(fgraph)
	if len(components) != 1 {
		t.Fatalf("got %d components, want 1", len(components))
	}
	if got := len(components[0].Nodes); got != 6 {
		t.Fatalf("got %d nodes, want 6", got)
	}
	if got := len(components[0].Edges); got != 4 {
		t.Fatalf("got %d edges, want 4", got)
	}
	if got := len(components[0].Labels); got != 4 {
		t.Fatalf("got %d labels, want 4", got)
	}
}

func createSimpleGraph() (*graph.Model, graph.NodeID) {
	model := graph.NewModel()
	addSimpleGraph(model, model.Root)
	return model, model.Root
}

func addSimpleGraph(model *graph.Model, parent graph.NodeID) {
	nodes := []graph.NodeID{
		model.AddNode(&parent), model.AddNode(&parent), model.AddNode(&parent),
	}
	for _, endpoints := range [][2]int{{0, 1}, {0, 2}} {
		edge := model.AddEdge(&parent)
		model.AddSource(edge, graph.NodeRef(nodes[endpoints[0]]))
		model.AddTarget(edge, graph.NodeRef(nodes[endpoints[1]]))
		model.AddLabel(graph.EdgeElementRef(edge), "test")
	}
}

func checkSimpleGraph(t *testing.T, fgraph *FGraph) {
	t.Helper()
	if got := len(fgraph.Nodes); got != 3 {
		t.Fatalf("got %d nodes, want 3", got)
	}
	if got := len(fgraph.Edges); got != 2 {
		t.Fatalf("got %d edges, want 2", got)
	}
	if got := len(fgraph.Labels); got != 2 {
		t.Fatalf("got %d labels, want 2", got)
	}
}
