// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package core_test

import (
	"testing"

	"github.com/d2lang/elk-go/internal/builtins"
	"github.com/d2lang/elk-go/internal/core"
)

// TestBySuffixMatchesScanOnDefaultRegistry checks BySuffix against the
// linear scan on every option the default registries hold: every suffix (at
// any position) of every ID and legacy ID, variants of them, and every key
// spelling D2 sends.
func TestBySuffixMatchesScanOnDefaultRegistry(t *testing.T) {
	registry := core.NewOptionRegistry()
	builtins.RegisterOptions(registry)
	probes := []string{"", ".", "..", "x", "x.", ".x", "strategy", "spacing", "elk.direction", "direction",
		"spacing.nodeNodeBetweenLayers", "spacing.edgeNodeBetweenLayers", "spacing.edgeEdgeBetweenLayers",
		"elk.spacing.nodeSelfLoop", "elk.padding", "padding", "elk.algorithm", "algorithm", "layered"}
	for _, option := range registry.Options() {
		for _, id := range append([]string{option.ID}, option.LegacyIDs...) {
			for start := 0; start <= len(id); start++ {
				probes = append(probes, id[start:], "."+id[start:], "z"+id[start:], id[start:]+".", "x."+id[start:])
			}
		}
	}
	checked := 0
	for _, probe := range probes {
		got, want := registry.BySuffix(probe), registry.BySuffixScan(probe)
		if got != want {
			t.Fatalf("BySuffix(%q) = %v, scan = %v", probe, got, want)
		}
		if got != nil {
			checked++
		}
	}
	if checked < 1000 {
		t.Fatalf("only %d probes resolved", checked)
	}
}
