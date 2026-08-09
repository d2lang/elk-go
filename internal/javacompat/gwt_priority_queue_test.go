// Copyright (c) 2026 The elk-go Authors.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package javacompat

import (
	"reflect"
	"testing"
)

func TestGWTPriorityQueueBulkHeapAndEqualSiftOrder(t *testing.T) {
	type item struct {
		priority int
		name     string
	}
	queue := NewGWTPriorityQueue(func(first, second item) int {
		return intComparator(first.priority, second.priority)
	})
	queue.AddAll([]item{{3, "a"}, {1, "b"}, {1, "c"}, {2, "d"}, {1, "e"}})
	var got []string
	for !queue.IsEmpty() {
		value, _ := queue.Poll()
		got = append(got, value.name)
	}
	// Frozen from the GWT heap's recursive makeHeap and strict mergeHeaps
	// comparison; OpenJDK's equal-element order is different.
	want := []string{"e", "b", "c", "d", "a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("poll order = %v, want %v", got, want)
	}
}
