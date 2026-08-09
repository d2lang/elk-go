// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"testing"

	"github.com/d2lang/elk-go/graphjson"
	"github.com/d2lang/elk-go/internal/graph"
)

func TestLayoutOptionSuffixAndParsing(t *testing.T) {
	registry := NewOptionRegistry()
	if option := registry.BySuffix("direction"); option == nil || option.ID != Direction.ID {
		t.Fatalf("direction lookup = %#v", option)
	}
	if value, ok := registry.BySuffix("direction").ParseValue("2"); !ok || value != "LEFT" {
		t.Fatalf("enum ordinal = %#v, %v", value, ok)
	}
	if value, ok := registry.BySuffix("contentAlignment").ParseValue("H_CENTER V_CENTER"); !ok || len(value.([]string)) != 2 {
		t.Fatalf("enum set = %#v, %v", value, ok)
	}
}

func TestJSONImportAndLayoutTransfer(t *testing.T) {
	zero := 0.0
	width := 20.0
	height := 10.0
	input := &graphjson.Graph{ID: graphjson.StringID("root"), Children: []*graphjson.Node{
		{ID: graphjson.StringID("a"), X: &zero, Width: &width, Height: &height},
		{ID: graphjson.StringID("b"), Width: &width, Height: &height},
	}, Edges: []*graphjson.Edge{{ID: graphjson.StringID("e"), Sources: []graphjson.ID{graphjson.StringID("a")}, Targets: []graphjson.ID{graphjson.StringID("b")}}}}
	codec := NewCodec(NewOptionRegistry())
	model, err := codec.Import(input)
	if err != nil {
		t.Fatal(err)
	}
	if got := model.Node(model.Root).Children; len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("child order = %#v", got)
	}
	model.Node(1).Shape.SetLocation(7, 8)
	sectionID := model.AddSection(0)
	section := model.Section(sectionID)
	section.Start.X = 1
	section.End.X = 2
	if err := codec.TransferLayout(model); err != nil {
		t.Fatal(err)
	}
	if *input.Children[0].X != 7 || *input.Children[0].Y != 8 {
		t.Fatalf("layout not transferred: %#v", input.Children[0])
	}
	if len(input.Edges[0].Sections) != 1 || input.Edges[0].Sections[0].ID.String() != "e_s0" {
		t.Fatalf("sections = %#v", input.Edges[0].Sections)
	}
}

type placingProvider struct{}

func (placingProvider) Layout(model *graph.Model, parent graph.NodeID, _ ProgressMonitor) error {
	for index, child := range model.Node(parent).Children {
		model.Node(child).Shape.SetLocation(float64(index*10), 5)
	}
	return nil
}

func TestRecursiveGraphLayoutEngine(t *testing.T) {
	options := NewOptionRegistry()
	algorithms := NewAlgorithmRegistry()
	if err := algorithms.Register(AlgorithmData{ID: DefaultAlgorithmID, Create: func() LayoutProvider { return placingProvider{} }}); err != nil {
		t.Fatal(err)
	}
	model := graph.NewModel()
	root := model.Root
	model.AddNode(&root)
	model.AddNode(&root)
	if err := NewEngine(options, algorithms).Layout(model); err != nil {
		t.Fatal(err)
	}
	if model.Node(2).Shape.X != 10 || model.Node(2).Shape.Y != 5 {
		t.Fatalf("position = %#v", model.Node(2).Shape)
	}
}
