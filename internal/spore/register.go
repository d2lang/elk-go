// Copyright (c) 2017 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package spore

import "github.com/d2lang/elk-go/internal/core"

// Register installs both providers and captures the registry for ELK's
// optional underlyingLayoutAlgorithm delegation.
func Register(registry *core.AlgorithmRegistry) error {
	algorithms := []core.AlgorithmData{
		{
			ID:     OverlapAlgorithmID,
			Create: func() core.LayoutProvider { return &overlapProvider{algorithms: registry} },
			KnownOptions: []string{
				UnderlyingLayoutAlgorithm.ID,
				Padding.ID,
				SpacingNodeNode.ID,
				StructureExtraction.ID,
				MaxIterations.ID,
				RunScanline.ID,
				DebugMode.ID,
			},
		},
		{
			ID:     CompactionAlgorithmID,
			Create: func() core.LayoutProvider { return &compactionProvider{algorithms: registry} },
			KnownOptions: []string{
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
			},
		},
	}
	for _, algorithm := range algorithms {
		if err := registry.Register(algorithm); err != nil {
			return err
		}
	}
	return nil
}
