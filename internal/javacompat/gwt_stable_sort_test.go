// Copyright 2008 Google Inc.
// Copyright (c) 2026 The elk-go Authors.
//
// SPDX-License-Identifier: Apache-2.0

package javacompat

import (
	"reflect"
	"testing"
)

func TestGWTStableSortComparatorTrace(t *testing.T) {
	values := []int{6, 5, 4, 3, 2, 1, 0}
	type comparison struct{ first, second int }
	var comparisons []comparison
	GWTStableSort(values, func(first, second int) int {
		comparisons = append(comparisons, comparison{first, second})
		return first - second
	})

	if want := []int{0, 1, 2, 3, 4, 5, 6}; !reflect.DeepEqual(values, want) {
		t.Fatalf("sorted values = %v, want %v", values, want)
	}
	// This is the exact GWT 2.11.0 legacy merge-sort call trace. It is
	// unchanged from 2.9.0 and differs from Go's sorting implementations.
	wantComparisons := []comparison{
		{6, 5}, {6, 4}, {5, 4},
		{3, 2}, {3, 1}, {2, 1}, {3, 0}, {2, 0}, {1, 0},
		{6, 0}, {4, 0}, {4, 1}, {4, 2}, {4, 3},
	}
	if !reflect.DeepEqual(comparisons, wantComparisons) {
		t.Fatalf("comparison trace = %v, want GWT 2.11.0 trace %v", comparisons, wantComparisons)
	}
}

func TestGWTStableSortIsStable(t *testing.T) {
	type item struct {
		key  int
		name string
	}
	values := []item{{2, "a"}, {1, "b"}, {2, "c"}, {1, "d"}, {2, "e"}, {1, "f"}, {2, "g"}, {1, "h"}}
	GWTStableSort(values, func(first, second item) int { return first.key - second.key })
	want := []item{{1, "b"}, {1, "d"}, {1, "f"}, {1, "h"}, {2, "a"}, {2, "c"}, {2, "e"}, {2, "g"}}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("stable order = %v, want %v", values, want)
	}
}
