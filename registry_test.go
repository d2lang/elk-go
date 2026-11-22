// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package elk

import (
	"strings"
	"testing"

	"github.com/d2lang/elk-go/internal/core"
)

// scanBySuffix is ELK's suffix lookup as a linear scan over the registry.
func scanBySuffix(registry *core.OptionRegistry, suffix string) *core.OptionData {
	if suffix == "" {
		return nil
	}
	if exact := registry.ByID(suffix); exact != nil {
		return exact
	}
	dot := func(id string) bool {
		return strings.HasSuffix(id, suffix) && (len(id) == len(suffix) || id[len(id)-len(suffix)-1] == '.')
	}
	var match *core.OptionData
	for _, option := range registry.Options() {
		if dot(option.ID) {
			if match != nil && match != option {
				return nil
			}
			match = option
		}
	}
	if match != nil {
		return match
	}
	for _, option := range registry.Options() {
		for _, legacy := range option.LegacyIDs {
			if dot(legacy) {
				if match != nil && match != option {
					return nil
				}
				match = option
			}
		}
	}
	return match
}

// TestSharedRegistryBySuffix checks the indexed suffix lookup against the scan
// for every suffix of every option ID and legacy ID in the default registry.
func TestSharedRegistryBySuffix(t *testing.T) {
	options, _ := sharedRegistries()
	fresh, _ := defaultRegistries()
	probes := []string{"", ".", "..", "x.", "layered", "spacing.nodeNodeBetweenLayers",
		"elk.direction", "direction", "spacing", "elk.spacing.nodeSelfLoop", "nodeSelfLoop"}
	for _, option := range options.Options() {
		for _, id := range append([]string{option.ID}, option.LegacyIDs...) {
			for start := 0; start <= len(id); start++ {
				probes = append(probes, id[start:], "."+id[start:], "q"+id[start:])
			}
		}
	}
	for _, probe := range probes {
		want := scanBySuffix(options, probe)
		if got := options.BySuffix(probe); got != want {
			t.Fatalf("BySuffix(%q) = %v, scan = %v", probe, got, want)
		}
		// A registry built for one call resolves to the same option ID.
		freshGot := fresh.BySuffix(probe)
		if (freshGot == nil) != (want == nil) || freshGot != nil && freshGot.ID != want.ID {
			t.Fatalf("fresh BySuffix(%q) differs", probe)
		}
	}
}
