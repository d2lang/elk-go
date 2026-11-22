// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package core

import (
	"math/rand"
	"strings"
	"testing"
)

// revOriginalBySuffix is BySuffix of the original registry (elk-go harness),
// verbatim, over the registry's registered options and byID map.
func revOriginalBySuffix(registry *OptionRegistry, suffix string) *OptionData {
	if suffix == "" {
		return nil
	}
	if exact := registry.byID[suffix]; exact != nil {
		return exact
	}
	var match *OptionData
	for _, option := range registry.options {
		if dotSuffix(option.ID, suffix) {
			if match != nil && match != option {
				return nil
			}
			match = option
		}
	}
	if match != nil {
		return match
	}
	for _, option := range registry.options {
		for _, legacy := range option.LegacyIDs {
			if dotSuffix(legacy, suffix) {
				if match != nil && match != option {
					return nil
				}
				match = option
			}
		}
	}
	return match
}

// TestBySuffixRandomRegistries registers random IDs and legacy IDs
// over a tiny alphabet (so that tails, empty segments, leading and trailing
// dots, legacy IDs equal to other IDs, repeated legacy IDs and
// registration failures are common) into the default registry and a fresh one,
// and compares BySuffix with the original scan for every suffix of every ID
// and random probes, after every registration.
func TestBySuffixRandomRegistries(t *testing.T) {
	rng := rand.New(rand.NewSource(20261120))
	segments := []string{"a", "b", "ab", "", "x", "elk", "direction"}
	randomID := func() string {
		n := 1 + rng.Intn(4)
		parts := make([]string, n)
		for i := range parts {
			parts[i] = segments[rng.Intn(len(segments))]
		}
		return strings.Join(parts, ".")
	}
	for round := 0; round < 300; round++ {
		var registry *OptionRegistry
		if round%3 == 0 {
			registry = NewOptionRegistry()
		} else {
			registry = &OptionRegistry{byID: make(map[string]*OptionData)}
		}
		probes := []string{"", ".", "..", "...", "a", "a.", ".a", "elk.direction", "direction"}
		for step := 0; step < 30; step++ {
			option := OptionData{ID: randomID(), Kind: StringOption}
			for l := rng.Intn(3); l > 0; l-- {
				option.LegacyIDs = append(option.LegacyIDs, randomID())
			}
			if rng.Intn(10) == 0 && len(option.LegacyIDs) > 0 {
				option.LegacyIDs = append(option.LegacyIDs, option.LegacyIDs[0])
			}
			if rng.Intn(10) == 0 {
				option.LegacyIDs = append(option.LegacyIDs, option.ID)
			}
			_ = registry.Register(option) // failures (duplicate IDs) must change nothing
			for _, id := range append([]string{option.ID}, option.LegacyIDs...) {
				for start := 0; start <= len(id); start++ {
					probes = append(probes, id[start:], "."+id[start:], id[start:]+".")
				}
			}
			probes = append(probes, randomID(), "."+randomID(), randomID()+".")
			for _, probe := range probes {
				if got, want := registry.BySuffix(probe), revOriginalBySuffix(registry, probe); got != want {
					t.Fatalf("round %d step %d: BySuffix(%q) = %v, original = %v", round, step, probe, optionID(got), optionID(want))
				}
			}
		}
	}
}

// TestKnownOrderFollowsRegistry resolves one member order through a
// codec, registers an option that makes a key ambiguous (or resolves a key that
// did not resolve), and checks that the codec's remembered resolution follows.
func TestKnownOrderFollowsRegistry(t *testing.T) {
	registry := &OptionRegistry{byID: make(map[string]*OptionData)}
	if err := registry.Register(OptionData{ID: "p.q.size", Kind: StringOption}); err != nil {
		t.Fatal(err)
	}
	codec := NewCodec(registry)
	keys := []string{"size", "q.size", "other"}
	first := codec.PrepareOptions(keys, []any{"1", "2", "3"})
	if len(first.entries) != 2 {
		t.Fatalf("first preparation: %d entries, want 2", len(first.entries))
	}
	// "size" becomes ambiguous, "other" resolves.
	if err := registry.Register(OptionData{ID: "r.size", Kind: StringOption}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(OptionData{ID: "z.other", Kind: StringOption}); err != nil {
		t.Fatal(err)
	}
	second := codec.PrepareOptions(keys, []any{"1", "2", "3"})
	fresh := NewCodec(registry).PrepareOptions(keys, []any{"1", "2", "3"})
	if len(second.entries) != len(fresh.entries) {
		t.Fatalf("after registration: %d entries, fresh codec %d", len(second.entries), len(fresh.entries))
	}
	for i := range fresh.entries {
		if second.entries[i].key != fresh.entries[i].key || second.entries[i].value != fresh.entries[i].value {
			t.Fatalf("entry %d differs: %+v vs %+v", i, second.entries[i], fresh.entries[i])
		}
	}
}
