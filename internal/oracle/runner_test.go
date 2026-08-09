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
	"path/filepath"
	"testing"
	"time"
)

func TestPinnedReferenceDeterminism(t *testing.T) {
	runner := DefaultRunner()
	if err := runner.Check(); err != nil {
		t.Skipf("elkjs differential oracle unavailable: %v", err)
	}
	paths, err := CuratedInputs()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			input, err := ReadInput(path)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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
				t.Fatalf("pinned elkjs output changed between identical runs")
			}
		})
	}
}
