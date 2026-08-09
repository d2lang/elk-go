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
	"slices"
	"testing"

	"github.com/d2lang/elk-go/internal/core"
)

func TestRegistrationMatchesSPOrEMetadata(t *testing.T) {
	options := core.NewOptionRegistry()
	RegisterOptions(options)
	for _, id := range []string{
		UnderlyingLayoutAlgorithm.ID,
		StructureExtraction.ID,
		TreeConstruction.ID,
		SpanningTreeCost.ID,
		PreferredRoot.ID,
		RootSelection.ID,
		Compaction.ID,
		Orthogonal.ID,
		MaxIterations.ID,
		RunScanline.ID,
		DebugMode.ID,
	} {
		if options.ByID(id) == nil {
			t.Fatalf("option %q was not registered", id)
		}
	}
	algorithms := core.NewAlgorithmRegistry()
	if err := Register(algorithms); err != nil {
		t.Fatal(err)
	}
	overlap := algorithms.ByID(OverlapAlgorithmID)
	compaction := algorithms.ByID(CompactionAlgorithmID)
	if overlap == nil || compaction == nil {
		t.Fatal("both SPOrE providers must be registered")
	}
	if !slices.Equal(overlap.KnownOptions, []string{
		UnderlyingLayoutAlgorithm.ID,
		Padding.ID,
		SpacingNodeNode.ID,
		StructureExtraction.ID,
		MaxIterations.ID,
		RunScanline.ID,
		DebugMode.ID,
	}) {
		t.Fatalf("overlap known options = %v", overlap.KnownOptions)
	}
	if !slices.Equal(compaction.KnownOptions, []string{
		UnderlyingLayoutAlgorithm.ID,
		TreeConstruction.ID,
		SpanningTreeCost.ID,
		PreferredRoot.ID,
		RootSelection.ID,
		Padding.ID,
		SpacingNodeNode.ID,
		StructureExtraction.ID,
		Compaction.ID,
		Orthogonal.ID,
		DebugMode.ID,
	}) {
		t.Fatalf("compaction known options = %v", compaction.KnownOptions)
	}
}
