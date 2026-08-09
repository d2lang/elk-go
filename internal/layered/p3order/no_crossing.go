// Copyright (c) 2022 Kiel University and others.
// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package p3order

import "github.com/d2lang/elk-go/internal/layered"

// NoCrossingMinimizer deliberately preserves the node and port order produced
// by layering and the required long-edge preprocessing.
type NoCrossingMinimizer struct{}

func (*NoCrossingMinimizer) ProcessorConfiguration(
	_ *layered.LGraphArena,
	_ layered.LGraphID,
) layered.ProcessorConfiguration {
	var configuration layered.ProcessorConfiguration
	configuration.
		AddBefore(layered.PhaseNodeOrdering, layered.LongEdgeSplitter).
		AddBefore(layered.PhaseNodeOrdering, layered.PortListSorter).
		AddBefore(layered.PhaseNodePlacement, layered.InLayerConstraintProcessor).
		AddAfter(layered.PhaseEdgeRouting, layered.LongEdgeJoiner)
	return configuration
}

func (*NoCrossingMinimizer) Process(*layered.LGraphArena, layered.LGraphID) error { return nil }
