// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package graph

import "testing"

func TestPropertyMapOrderAndDefaults(t *testing.T) {
	spacing := PropertyWithDefault("spacing", func() float64 { return 20 })
	chain := CloneablePropertyWithDefault("chain", func() []string { return []string{"x"} })
	var properties PropertyMap
	if Get(&properties, spacing) != 20 || properties.Has(spacing.ID) {
		t.Fatal("scalar default was materialized")
	}
	if len(Get(&properties, chain)) != 1 || !properties.Has(chain.ID) {
		t.Fatal("cloneable default was not materialized")
	}
	properties.SetRaw("a", 1)
	properties.SetRaw("b", 2)
	properties.SetRaw("a", 3)
	entries := properties.Entries()
	if entries[1].ID != "a" || entries[2].ID != "b" {
		t.Fatalf("entry order = %#v", entries)
	}
}

func TestBestEdgeContainment(t *testing.T) {
	model := NewModel()
	root := model.Root
	a := model.AddNode(&root)
	a1 := model.AddNode(&a)
	a2 := model.AddNode(&a)
	edge := model.AddEdge(nil)
	model.AddSource(edge, NodeRef(a1))
	model.AddTarget(edge, NodeRef(a2))
	model.UpdateContainment(edge)
	if model.Edge(edge).ContainingNode == nil || *model.Edge(edge).ContainingNode != a {
		t.Fatalf("containment = %#v", model.Edge(edge).ContainingNode)
	}
	b := model.AddNode(&root)
	cross := model.AddEdge(nil)
	model.AddSource(cross, NodeRef(a1))
	model.AddTarget(cross, NodeRef(b))
	model.UpdateContainment(cross)
	if model.Edge(cross).ContainingNode == nil || *model.Edge(cross).ContainingNode != root {
		t.Fatalf("cross containment = %#v", model.Edge(cross).ContainingNode)
	}
	loop := model.AddEdge(nil)
	model.AddSource(loop, NodeRef(a1))
	model.AddTarget(loop, NodeRef(a1))
	model.UpdateContainment(loop)
	if model.Edge(loop).ContainingNode == nil || *model.Edge(loop).ContainingNode != a {
		t.Fatalf("self-loop containment = %#v, want parent %d", model.Edge(loop).ContainingNode, a)
	}
}

func TestFirstSectionResetReplacesExistingRoute(t *testing.T) {
	model := NewModel()
	edge := model.AddEdge(&model.Root)
	first := model.AddSection(edge)
	model.Section(first).Identifier = "first"
	model.Section(first).IdentifierPresent = true
	model.Section(first).StartPresent = true
	model.AddSection(edge)

	if got := model.FirstSection(edge, true); got != first {
		t.Fatalf("first section = %d, want %d", got, first)
	}
	if got := len(model.Edge(edge).Sections); got != 1 {
		t.Fatalf("section count = %d, want 1", got)
	}
	section := model.Section(first)
	if section.Identifier != "first" || !section.IdentifierPresent {
		t.Fatalf("section identity was not preserved: %#v", section)
	}
	if section.StartPresent {
		t.Fatalf("section geometry was not reset: %#v", section)
	}
}
