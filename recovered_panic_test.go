// Copyright (c) 2026 The elk-go Authors.
// SPDX-License-Identifier: EPL-2.0

package elk

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// recoveredLayout runs LayoutJSON and turns a panic into an error, as d2's
// WebAssembly API does (d2js/d2wasm/api.go recovers panics and keeps the
// module alive for the next call).
func recoveredLayout(input []byte) (output []byte, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic: %v", recovered)
		}
	}()
	return LayoutJSON(input)
}

// TestRecoveredPanicLeavesNoState: a layout that panics (the
// original elk-go panics on these inputs too, with the same message) must not
// change the result of later layouts in the same process. Each victim is laid
// out once before any poison (the reference), then again right after each
// poison input.
func TestRecoveredPanicLeavesNoState(t *testing.T) {
	dir := filepath.Join("testdata", "poison")
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	victims := []string{"victim-563ac186a571393f.json", "victim-s192-c177.json"}
	poisons := []string{"poison-bit-a.json", "poison-bit-b.json", "poison-bit-c.json"}
	want := make(map[string][]byte)
	for _, victim := range victims {
		output, err := recoveredLayout(read(victim))
		if err != nil {
			t.Fatalf("%s before any poison: %v", victim, err)
		}
		want[victim] = output
	}
	for _, poison := range poisons {
		if _, err := recoveredLayout(read(poison)); err == nil {
			t.Fatalf("%s: expected the layout to panic", poison)
		}
		for _, victim := range victims {
			output, err := recoveredLayout(read(victim))
			if err != nil {
				t.Errorf("%s after %s: %v", victim, poison, err)
				continue
			}
			if !bytes.Equal(output, want[victim]) {
				t.Errorf("%s after %s: layout differs from the one before (%d vs %d bytes)",
					victim, poison, len(output), len(want[victim]))
			}
		}
	}
}
