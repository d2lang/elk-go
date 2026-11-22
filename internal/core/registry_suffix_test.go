// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"testing"

	"github.com/d2lang/elk-go/internal/graph"
)

// suffixProbes returns every suffix (at any position, not only dot
// boundaries) of every registered ID and legacy ID, plus variants that must
// miss or be ambiguous.
func suffixProbes(registry *OptionRegistry, extra ...string) []string {
	probes := append([]string{"", ".", "..", "x", "x.", ".x"}, extra...)
	for _, option := range registry.Options() {
		ids := append([]string{option.ID}, option.LegacyIDs...)
		for _, id := range ids {
			for start := 0; start <= len(id); start++ {
				probes = append(probes, id[start:], "."+id[start:], "z"+id[start:], id[start:]+".")
			}
		}
	}
	return probes
}

func TestBySuffixIndexMatchesScan(t *testing.T) {
	registry := NewOptionRegistry()
	synthetic := []OptionData{
		// Shared tails create ambiguity between IDs, between legacy IDs and
		// between an ID and a legacy ID.
		{ID: "a.b.shared", Kind: StringOption},
		{ID: "c.d.shared", Kind: StringOption},
		{ID: "only.id.tail", Kind: StringOption, LegacyIDs: []string{"old.legacy.tail", "older.legacy.tail"}},
		{ID: "other.id.thing", Kind: StringOption, LegacyIDs: []string{"x.legacy.tail"}},
		{ID: "third.id.solo", Kind: StringOption, LegacyIDs: []string{"some.solo"}},
		{ID: "dots..in..id", Kind: StringOption},
		{ID: "trailing.dot.", Kind: StringOption},
		{ID: ".leading.dot", Kind: StringOption},
		{ID: "nodots", Kind: StringOption, LegacyIDs: []string{"legacy.nodots.alias", "a.b.shared.alias"}},
		{ID: "org.eclipse.elk.layered.spacing.edgeNode.extra", Kind: DoubleOption},
	}
	for _, option := range synthetic {
		if err := registry.Register(option); err != nil {
			t.Fatal(err)
		}
	}
	probes := suffixProbes(registry, "shared", "b.shared", "tail", "legacy.tail", "solo", "id..in..id", "in..id",
		".in..id", "dot.", "dot", "leading.dot", "alias", "shared.alias", "edgeNode", "spacing.edgeNode")
	for _, probe := range probes {
		got, want := registry.BySuffix(probe), registry.bySuffixScan(probe)
		if got != want {
			t.Fatalf("BySuffix(%q) = %v, scan = %v", probe, optionID(got), optionID(want))
		}
	}
}

func optionID(option *OptionData) string {
	if option == nil {
		return "<nil>"
	}
	return option.ID
}

// TestRegisterRecordsPropertyKey checks that every registered option carries
// the interned key of its ID, which import uses instead of interning the ID.
func TestRegisterRecordsPropertyKey(t *testing.T) {
	registry := NewOptionRegistry()
	if err := registry.Register(OptionData{ID: "test.registered.key", Kind: StringOption, LegacyIDs: []string{"test.legacy.key"}}); err != nil {
		t.Fatal(err)
	}
	for _, option := range registry.Options() {
		if option.key == 0 || option.key != graph.InternPropertyKey(option.ID) || option.propertyKey() != option.key {
			t.Fatalf("%s: key %d, interned %d", option.ID, option.key, graph.InternPropertyKey(option.ID))
		}
	}
	unregistered := OptionData{ID: "test.unregistered.key"}
	if unregistered.propertyKey() != graph.InternPropertyKey("test.unregistered.key") {
		t.Fatal("unregistered option key differs from the interned ID")
	}
}
