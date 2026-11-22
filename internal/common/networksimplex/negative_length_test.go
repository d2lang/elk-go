// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package networksimplex

import (
	"fmt"
	"testing"
)

func panicValue(call func()) (value any) {
	defer func() { value = recover() }()
	call()
	return nil
}

// TestZeroedNegativeLengthPanicsLikeMake checks that the buffer
// helpers panic for a negative length exactly as make does (the original
// called make([]int, highest-lowest+1) in normalize), with or without a
// buffer to reuse.
func TestZeroedNegativeLengthPanicsLikeMake(t *testing.T) {
	for _, n := range []int{-1, -3, -9223372036854775808} {
		want := fmt.Sprintf("%T: %v", panicValue(func() { _ = make([]int, n) }), panicValue(func() { _ = make([]int, n) }))
		for _, capacity := range []int{0, 1, 64} {
			calls := map[string]func(){
				"ints":   func() { zeroedInts(make([]int, 0, capacity), n) },
				"floats": func() { zeroedFloats(make([]float64, 0, capacity), n) },
				"bools":  func() { zeroedBools(make([]bool, 0, capacity), n) },
			}
			for name, call := range calls {
				value := panicValue(call)
				if got := fmt.Sprintf("%T: %v", value, value); got != want {
					t.Errorf("%s(cap %d, %d): panic %s, want %s", name, capacity, n, got, want)
				}
			}
		}
	}
	if got := zeroedInts(make([]int, 3, 8), 5); len(got) != 5 || cap(got) != 8 {
		t.Errorf("reuse: len %d cap %d", len(got), cap(got))
	}
}
