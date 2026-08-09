// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0/.
//
// SPDX-License-Identifier: EPL-2.0

package oracle

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestRandomCaseIsDeterministic(t *testing.T) {
	t.Parallel()
	testCase := RandomCase{Seed: 0x0123456789abcdef, NodeCount: 7, EdgeCount: 15}
	first, err := testCase.JSON()
	if err != nil {
		t.Fatal(err)
	}
	second, err := testCase.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("same scalar random case produced different JSON:\n%s\n%s", first, second)
	}
}

func FuzzReferenceRandomDeterminism(f *testing.F) {
	runner := DefaultRunner()
	if err := runner.Check(); err != nil {
		f.Skipf("elkjs differential oracle unavailable: %v", err)
	}
	f.Add(uint64(1), uint8(3), uint8(4))
	f.Fuzz(func(t *testing.T, seed uint64, nodes, edges uint8) {
		input, err := (RandomCase{Seed: seed, NodeCount: nodes, EdgeCount: edges}).JSON()
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		first, err := runner.Layout(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		second, err := runner.Layout(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("elkjs 0.8.2 is not byte-deterministic for seed=%d nodes=%d edges=%d", seed, nodes, edges)
		}
	})
}
