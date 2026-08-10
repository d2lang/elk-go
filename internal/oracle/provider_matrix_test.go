// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package oracle

import (
	"context"
	"os"
	"testing"
	"time"

	elk "github.com/d2lang/elk-go"
)

// TestCandidateProviderMatrixParity exercises every algorithm registered by
// the official ELK.js 0.12.0 bundle through the public elk-go entry point. The
// shared graph is deliberately positioned, connected, and seeded so it is a
// valid deterministic input for utility, tree, force, compaction, packing,
// and Layered providers alike.
func TestCandidateProviderMatrixParity(t *testing.T) {
	if os.Getenv("ELK_GO_ORACLE_ALGORITHMS") != "1" {
		t.Skip("set ELK_GO_ORACLE_ALGORITHMS=1 to run all-provider differentials")
	}
	runner := DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Fatal(err)
	}
	for _, algorithm := range OfficialAlgorithmSuffixes {
		algorithm := algorithm
		t.Run(algorithm, func(t *testing.T) {
			input := providerMatrixInput(algorithm)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := Differential(ctx, runner, input, elk.LayoutJSON, ExactTolerance); err != nil {
				t.Fatalf("%v\ninput: %s", err, input)
			}
		})
	}
}
